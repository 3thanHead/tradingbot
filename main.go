package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// TradingView webhook payload
type TradingViewEvent struct {
	Ticker   string `json:"ticker"`
	Exchange string `json:"exchange"`
	Interval string `json:"interval"`
	Close    string `json:"close"`
	Open     string `json:"open"`
	High     string `json:"high"`
	Low      string `json:"low"`
	Volume   string `json:"volume"`
	Time     string `json:"time"`
	TimeNow  string `json:"timenow"`
}

// ============================================================================
// STRATEGY SYSTEM TYPES
// ============================================================================
//
// NAMING CONVENTION:
// - Entry uses "steps" (can be sequential: step 1 → step 2 → step 3)
// - Exit uses "conditions" (usually independent triggers)
// Both are essentially webhook triggers, just named differently for clarity.
// ============================================================================

// EntryStep represents a single webhook trigger for entering a position
// Called "step" because entries can be sequential (wait for step 1, then step 2, etc.)
type EntryStep struct {
	Webhook string `json:"webhook"` // Webhook URL path, e.g., "/webhook/rsi/crossed-down"
	Comment string `json:"comment"` // Optional human-readable description
}

// EntryConditions defines how entry steps combine to trigger a trade
type EntryConditions struct {
	Combination string      `json:"combination"` // How to combine: "all", "all_sequential", or "any"
	Steps       []EntryStep `json:"steps"`       // Array of webhook triggers for entry
}

// ExitCondition represents a single webhook trigger for exiting a position
// Called "condition" because exits are usually independent triggers (any can close)
// ExitCondition defines a webhook that can trigger position exit
// Position direction (LONG/SHORT) is determined from the actual OANDA position
type ExitCondition struct {
	Webhook string `json:"webhook"` // Webhook URL path, e.g., "/webhook/macd/cross-down"
	Comment string `json:"comment"` // Optional human-readable description
}

// ExitConditions defines how exit conditions combine to close a position
type ExitConditions struct {
	Combination string          `json:"combination"` // How to combine: "any" (typical) or "all" (rare)
	Conditions  []ExitCondition `json:"conditions"`  // Array of webhook triggers for exit
}

// Strategy defines complete trading strategy
// Supports either unified entry/exit OR separate long/short configurations
type Strategy struct {
	Name        string `json:"name"`
	Description string `json:"description"`

	// Option 1: Unified entry/exit (applies to both LONG and SHORT)
	Entry *EntryConditions `json:"entry,omitempty"`
	Exit  *ExitConditions  `json:"exit,omitempty"`

	// Option 2: Separate LONG and SHORT configurations
	Long  *PositionStrategy `json:"long,omitempty"`
	Short *PositionStrategy `json:"short,omitempty"`
}

// PositionStrategy defines entry/exit for a specific position direction
type PositionStrategy struct {
	Entry EntryConditions `json:"entry"`
	Exit  ExitConditions  `json:"exit"`
}

// Position state for each symbol
type PositionState struct {
	Symbol           string
	PositionOpen     bool
	Position         string // "long" or "short"
	TradeID          string
	RSICrossedUp     bool    // Tracks if RSI > 70 condition met
	RSICrossedDown   bool    // Tracks if RSI < 30 condition met
	MACDCrossedUp    bool    // Tracks if MACD crossed up
	MACDCrossedDown  bool    // Tracks if MACD crossed down
	SwingHigh        float64 // Latest swing high price level
	SwingLow         float64 // Latest swing low price level
	RSICrossedCenter bool    // Tracks if RSI crossed center (first warning)

	// Track which entry steps have been completed
	EntryStepsCompleted map[string]bool // Maps step index to completion status

	// Track MA ribbon state
	MARibbonBullish bool // Fast > Mid > Slow
	MARibbonBearish bool // Fast < Mid < Slow
}

// Global state management
var (
	positions = make(map[string]*PositionState)
	mu        sync.RWMutex

	oandaAPIKey       = os.Getenv("OANDA_API_KEY")
	oandaAccountID    = os.Getenv("OANDA_ACCOUNT_ID")
	oandaBaseURL      = "https://api-fxpractice.oanda.com" // Change to api-fxtrade.oanda.com for live
	tradeUnits        string                               // Trading units (fixed amount)
	tradeUSDAmount    string                               // USD notional amount (calculates units from price)
	tradeMargin       string                               // Margin amount (OANDA calculates position size based on leverage)
	takeProfitPips    string                               // Take profit in pips (e.g., "50")
	takeProfitPct     string                               // Take profit in percentage (e.g., "2.5" for 2.5%)
	takeProfitDollars string                               // Take profit in dollar amount (e.g., "100" for $100 gain)

	// Strategy system
	activeStrategy Strategy // Currently loaded strategy
	strategyName   string   // Name of strategy file to load
)

// Get or create position state for a symbol
func getPositionState(symbol string) *PositionState {
	mu.Lock()
	defer mu.Unlock()

	if _, exists := positions[symbol]; !exists {
		positions[symbol] = &PositionState{
			Symbol:              symbol,
			PositionOpen:        false,
			Position:            "none",
			EntryStepsCompleted: make(map[string]bool),
		}
	}
	return positions[symbol]
}

// ============================================================================
// STRATEGY SYSTEM FUNCTIONS
// ============================================================================

// Load strategy from JSON file
func loadStrategy(name string) (*Strategy, error) {
	// Default to "default" if not specified
	if name == "" {
		name = "default"
	}

	// Build file path
	filename := filepath.Join("strategies", name+".json")

	log.Printf("🎯 [STRATEGY] Loading: %s", filename)

	// Read file
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read strategy file: %v", err)
	}

	// Parse JSON
	var strategy Strategy
	if err := json.Unmarshal(data, &strategy); err != nil {
		return nil, fmt.Errorf("failed to parse strategy JSON: %v", err)
	}

	// Validate strategy
	if err := validateStrategy(&strategy); err != nil {
		return nil, fmt.Errorf("invalid strategy: %v", err)
	}

	// Print prominent strategy header
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Printf("⭐ STRATEGY LOADED: %s", strings.ToUpper(strategy.Name))
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Printf("📖 %s", strategy.Description)
	log.Println("")

	// Log based on strategy format
	if strategy.Entry != nil && strategy.Exit != nil {
		// Unified format
		log.Printf("📊 Format: Unified (same logic for LONG and SHORT)")
		log.Println("")

		// Entry steps
		log.Printf("� ENTRY (%s - %d steps):", strings.ToUpper(strategy.Entry.Combination), len(strategy.Entry.Steps))
		for i, step := range strategy.Entry.Steps {
			comment := step.Comment
			if comment == "" {
				comment = "No description"
			}
			log.Printf("   %d. %s → %s", i+1, step.Webhook, comment)
		}
		log.Println("")

		// Exit conditions
		log.Printf("🔴 EXIT (%s - %d conditions):", strings.ToUpper(strategy.Exit.Combination), len(strategy.Exit.Conditions))
		for i, condition := range strategy.Exit.Conditions {
			comment := condition.Comment
			if comment == "" {
				comment = "No description"
			}
			log.Printf("   %d. %s → %s", i+1, condition.Webhook, comment)
		}

	} else if strategy.Long != nil && strategy.Short != nil {
		// Separate LONG/SHORT format
		log.Printf("📊 Format: Separate LONG/SHORT configurations")
		log.Println("")

		// LONG entry/exit
		log.Printf("🟢 LONG ENTRY (%s - %d steps):", strings.ToUpper(strategy.Long.Entry.Combination), len(strategy.Long.Entry.Steps))
		for i, step := range strategy.Long.Entry.Steps {
			comment := step.Comment
			if comment == "" {
				comment = "No description"
			}
			log.Printf("   %d. %s → %s", i+1, step.Webhook, comment)
		}
		log.Println("")

		log.Printf("🔴 LONG EXIT (%s - %d conditions):", strings.ToUpper(strategy.Long.Exit.Combination), len(strategy.Long.Exit.Conditions))
		for i, condition := range strategy.Long.Exit.Conditions {
			comment := condition.Comment
			if comment == "" {
				comment = "No description"
			}
			log.Printf("   %d. %s → %s", i+1, condition.Webhook, comment)
		}
		log.Println("")

		// SHORT entry/exit
		log.Printf("🟠 SHORT ENTRY (%s - %d steps):", strings.ToUpper(strategy.Short.Entry.Combination), len(strategy.Short.Entry.Steps))
		for i, step := range strategy.Short.Entry.Steps {
			comment := step.Comment
			if comment == "" {
				comment = "No description"
			}
			log.Printf("   %d. %s → %s", i+1, step.Webhook, comment)
		}
		log.Println("")

		log.Printf("🔴 SHORT EXIT (%s - %d conditions):", strings.ToUpper(strategy.Short.Exit.Combination), len(strategy.Short.Exit.Conditions))
		for i, condition := range strategy.Short.Exit.Conditions {
			comment := condition.Comment
			if comment == "" {
				comment = "No description"
			}
			log.Printf("   %d. %s → %s", i+1, condition.Webhook, comment)
		}
	}

	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	return &strategy, nil
}

// Validate strategy structure
func validateStrategy(s *Strategy) error {
	if s.Name == "" {
		return fmt.Errorf("strategy name is required")
	}

	// Check which format is being used
	hasUnified := s.Entry != nil && s.Exit != nil
	hasSeparate := s.Long != nil && s.Short != nil

	if !hasUnified && !hasSeparate {
		return fmt.Errorf("strategy must have either unified entry/exit OR separate long/short configurations")
	}

	if hasUnified && hasSeparate {
		return fmt.Errorf("strategy cannot have both unified and separate configurations - choose one")
	}

	// Validate based on format
	if hasUnified {
		// Validate unified entry/exit
		if err := validateEntryConditions(s.Entry); err != nil {
			return fmt.Errorf("entry: %v", err)
		}
		if err := validateExitConditions(s.Exit); err != nil {
			return fmt.Errorf("exit: %v", err)
		}
	} else {
		// Validate separate LONG/SHORT
		if err := validateEntryConditions(&s.Long.Entry); err != nil {
			return fmt.Errorf("long entry: %v", err)
		}
		if err := validateExitConditions(&s.Long.Exit); err != nil {
			return fmt.Errorf("long exit: %v", err)
		}
		if err := validateEntryConditions(&s.Short.Entry); err != nil {
			return fmt.Errorf("short entry: %v", err)
		}
		if err := validateExitConditions(&s.Short.Exit); err != nil {
			return fmt.Errorf("short exit: %v", err)
		}
	}

	log.Printf("✅ [STRATEGY] Validation passed")
	return nil
}

// Helper: validate entry conditions
func validateEntryConditions(entry *EntryConditions) error {
	if len(entry.Steps) == 0 {
		return fmt.Errorf("must have at least one step")
	}

	validCombination := map[string]bool{"all": true, "all_sequential": true, "any": true}
	if !validCombination[entry.Combination] {
		return fmt.Errorf("invalid combination: %s (must be 'all', 'all_sequential', or 'any')", entry.Combination)
	}

	for i, step := range entry.Steps {
		if step.Webhook == "" {
			return fmt.Errorf("step %d is missing webhook path", i+1)
		}
	}

	return nil
}

// Helper: validate exit conditions
func validateExitConditions(exit *ExitConditions) error {
	if len(exit.Conditions) == 0 {
		return fmt.Errorf("must have at least one condition")
	}

	validCombination := map[string]bool{"any": true, "all": true}
	if !validCombination[exit.Combination] {
		return fmt.Errorf("invalid combination: %s (must be 'any' or 'all')", exit.Combination)
	}

	for i, condition := range exit.Conditions {
		if condition.Webhook == "" {
			return fmt.Errorf("condition %d is missing webhook path", i+1)
		}
	}

	return nil
}

// Check if all entry conditions are met for opening a position
func shouldOpenPosition(symbol string, isLong bool, r *http.Request) bool {
	state := getPositionState(symbol)

	// Get the appropriate entry conditions based on strategy format
	var entryConditions *EntryConditions
	if activeStrategy.Entry != nil {
		// Unified format - same entry for both LONG and SHORT
		entryConditions = activeStrategy.Entry
	} else if isLong && activeStrategy.Long != nil {
		// Separate format - use LONG entry
		entryConditions = &activeStrategy.Long.Entry
	} else if !isLong && activeStrategy.Short != nil {
		// Separate format - use SHORT entry
		entryConditions = &activeStrategy.Short.Entry
	} else {
		log.Printf("⚠️  [STRATEGY] No entry conditions found for %s position", map[bool]string{true: "LONG", false: "SHORT"}[isLong])
		return false
	}

	switch entryConditions.Combination {
	case "all_sequential":
		// Sequential: steps must be completed in exact order
		for i, step := range entryConditions.Steps {
			stepKey := fmt.Sprintf("step_%d", i)

			// If this webhook matches the current step
			if step.Webhook == r.URL.Path {
				// Mark step completed
				mu.Lock()
				state.EntryStepsCompleted[stepKey] = true
				mu.Unlock()

				log.Printf("✅ [STRATEGY] Entry step %d/%d completed: %s",
					i+1, len(entryConditions.Steps), step.Comment)

				// Check if ALL steps are now completed
				allComplete := true
				for j := 0; j < len(entryConditions.Steps); j++ {
					key := fmt.Sprintf("step_%d", j)
					if !state.EntryStepsCompleted[key] {
						allComplete = false
						break
					}
				}

				if allComplete {
					log.Printf("🎯 [STRATEGY] All entry conditions met!")
					// Reset steps for next trade
					mu.Lock()
					state.EntryStepsCompleted = make(map[string]bool)
					mu.Unlock()
					return true
				}
			}
		}
		return false

	case "all":
		// All conditions must be met (order doesn't matter)
		// Mark this step as completed
		for i, step := range entryConditions.Steps {
			if step.Webhook == r.URL.Path {
				stepKey := fmt.Sprintf("step_%d", i)
				mu.Lock()
				state.EntryStepsCompleted[stepKey] = true
				mu.Unlock()
				log.Printf("✅ [STRATEGY] Entry condition met: %s", step.Comment)
			}
		}

		// Check if all are completed
		allComplete := true
		for i := 0; i < len(entryConditions.Steps); i++ {
			key := fmt.Sprintf("step_%d", i)
			if !state.EntryStepsCompleted[key] {
				allComplete = false
				break
			}
		}

		if allComplete {
			log.Printf("🎯 [STRATEGY] All simultaneous entry conditions met!")
			mu.Lock()
			state.EntryStepsCompleted = make(map[string]bool)
			mu.Unlock()
			return true
		}
		return false

	case "any":
		// Any condition triggers entry
		for _, step := range entryConditions.Steps {
			if step.Webhook == r.URL.Path {
				log.Printf("🎯 [STRATEGY] Entry condition met: %s", step.Comment)
				return true
			}
		}
		return false
	}

	return false
}

// Check if any exit condition is met
func shouldExitPosition(symbol string, isLong bool, r *http.Request) (bool, string) {
	// Get the appropriate exit conditions based on strategy format
	var exitConditions *ExitConditions
	if activeStrategy.Exit != nil {
		// Unified format - same exit for both LONG and SHORT
		exitConditions = activeStrategy.Exit
	} else if isLong && activeStrategy.Long != nil {
		// Separate format - use LONG exit
		exitConditions = &activeStrategy.Long.Exit
	} else if !isLong && activeStrategy.Short != nil {
		// Separate format - use SHORT exit
		exitConditions = &activeStrategy.Short.Exit
	} else {
		log.Printf("⚠️  [STRATEGY] No exit conditions found for %s position", map[bool]string{true: "LONG", false: "SHORT"}[isLong])
		return false, ""
	}

	// Check each exit condition
	// Note: Position direction (isLong) comes from OANDA, not the strategy JSON
	for _, condition := range exitConditions.Conditions {
		// Check if this webhook matches the exit condition
		if condition.Webhook == r.URL.Path {
			reason := condition.Comment
			if reason == "" {
				reason = fmt.Sprintf("exit condition: %s", r.URL.Path)
			}
			return true, reason
		}
	}

	return false, ""
}

// ============================================================================
// RSI EVENT HANDLERS
// ============================================================================

// POST /webhook/rsi/crossed-up
func handleRSICrossedUp(w http.ResponseWriter, r *http.Request) {
	log.Printf("🔔 [WEBHOOK] Received RSI Crossed Up event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON in RSI Crossed Up: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Log the full request
	eventJSON, _ := json.MarshalIndent(event, "", "  ")
	log.Printf("📥 [REQUEST] %s", string(eventJSON))

	log.Printf("📥 [DATA] Ticker: %s, Exchange: %s, Close: %s", event.Ticker, event.Exchange, event.Close)

	symbol := normalizeSymbol(event.Ticker)
	log.Printf("🔄 [CONVERT] Normalized %s → %s", event.Ticker, symbol)

	state := getPositionState(symbol)

	log.Printf("📊 RSI > 70 for %s (price: %s)", symbol, event.Close)

	// Set the condition flag
	mu.Lock()
	state.RSICrossedUp = true
	state.RSICrossedDown = false // Reset opposite condition
	mu.Unlock()

	// Check if we should exit LONG position
	if state.PositionOpen && state.Position == "long" {
		shouldExit, reason := shouldExitPosition(symbol, true, r)
		if shouldExit {
			log.Printf("⚠️ [EXIT] %s → closing LONG position", reason)
			closePosition(symbol)
			respondSuccess(w, fmt.Sprintf("RSI > 70 exit: %s → closed LONG", reason))
			return
		}
	}

	// Check if we should open SHORT position
	if shouldOpenPosition(symbol, false, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening SHORT position")
		openShortPosition(symbol, event.Close)
		respondSuccess(w, "RSI > 70 + strategy → SHORT opened")
		return
	}

	respondSuccess(w, "RSI > 70 condition set")
}

// POST /webhook/rsi/crossed-down
func handleRSICrossedDown(w http.ResponseWriter, r *http.Request) {
	log.Printf("🔔 [WEBHOOK] Received RSI Crossed Down event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON in RSI Crossed Down: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Log the full request
	eventJSON, _ := json.MarshalIndent(event, "", "  ")
	log.Printf("📥 [REQUEST] %s", string(eventJSON))

	log.Printf("📥 [DATA] Ticker: %s, Exchange: %s, Close: %s", event.Ticker, event.Exchange, event.Close)

	symbol := normalizeSymbol(event.Ticker)
	log.Printf("🔄 [CONVERT] Normalized %s → %s", event.Ticker, symbol)

	state := getPositionState(symbol)

	log.Printf("📊 RSI < 30 for %s (price: %s)", symbol, event.Close)

	// Set the condition flag
	mu.Lock()
	state.RSICrossedDown = true
	state.RSICrossedUp = false // Reset opposite condition
	mu.Unlock()

	// Check if we should exit SHORT position
	if state.PositionOpen && state.Position == "short" {
		shouldExit, reason := shouldExitPosition(symbol, false, r)
		if shouldExit {
			log.Printf("⚠️ [EXIT] %s → closing SHORT position", reason)
			closePosition(symbol)
			respondSuccess(w, fmt.Sprintf("RSI < 30 exit: %s → closed SHORT", reason))
			return
		}
	}

	// Check if we should open LONG position
	if shouldOpenPosition(symbol, true, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening LONG position")
		openLongPosition(symbol, event.Close)
		respondSuccess(w, "RSI < 30 + strategy → LONG opened")
		return
	}

	respondSuccess(w, "RSI < 30 condition set")
}

// ============================================================================
// MA RIBBON EVENT HANDLERS
// ============================================================================

// POST /webhook/ma/ribbon-bullish
func handleMARibbonBullish(w http.ResponseWriter, r *http.Request) {
	log.Printf("🔔 [WEBHOOK] Received MA Ribbon Bullish event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON in MA Ribbon Bullish: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	eventJSON, _ := json.MarshalIndent(event, "", "  ")
	log.Printf("📥 [REQUEST] %s", string(eventJSON))

	symbol := normalizeSymbol(event.Ticker)
	log.Printf("🔄 [CONVERT] Normalized %s → %s", event.Ticker, symbol)

	state := getPositionState(symbol)

	log.Printf("📊 MA Ribbon BULLISH for %s (Fast > Mid > Slow)", symbol)

	mu.Lock()
	state.MARibbonBullish = true
	state.MARibbonBearish = false
	mu.Unlock()

	// Check if should open LONG position
	if shouldOpenPosition(symbol, true, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening LONG position")
		openLongPosition(symbol, event.Close)
		respondSuccess(w, "MA Ribbon bullish → LONG opened")
		return
	}

	// Check if should close SHORT position (bearish to bullish reversal)
	if state.PositionOpen && state.Position == "short" {
		shouldExit, reason := shouldExitPosition(symbol, false, r)
		if shouldExit {
			log.Printf("⚠️ [EXIT] %s → closing SHORT position", reason)
			closePosition(symbol)
			respondSuccess(w, fmt.Sprintf("MA Ribbon reversal: %s → closed SHORT", reason))
			return
		}
	}

	respondSuccess(w, "MA Ribbon bullish signal recorded")
}

// POST /webhook/ma/ribbon-bearish
func handleMARibbonBearish(w http.ResponseWriter, r *http.Request) {
	log.Printf("🔔 [WEBHOOK] Received MA Ribbon Bearish event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON in MA Ribbon Bearish: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	eventJSON, _ := json.MarshalIndent(event, "", "  ")
	log.Printf("📥 [REQUEST] %s", string(eventJSON))

	symbol := normalizeSymbol(event.Ticker)
	log.Printf("🔄 [CONVERT] Normalized %s → %s", event.Ticker, symbol)

	state := getPositionState(symbol)

	log.Printf("📊 MA Ribbon BEARISH for %s (Fast < Mid < Slow)", symbol)

	mu.Lock()
	state.MARibbonBearish = true
	state.MARibbonBullish = false
	mu.Unlock()

	// Check if should open SHORT position
	if shouldOpenPosition(symbol, false, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening SHORT position")
		openShortPosition(symbol, event.Close)
		respondSuccess(w, "MA Ribbon bearish → SHORT opened")
		return
	}

	// Check if should close LONG position (bullish to bearish reversal)
	if state.PositionOpen && state.Position == "long" {
		shouldExit, reason := shouldExitPosition(symbol, true, r)
		if shouldExit {
			log.Printf("⚠️ [EXIT] %s → closing LONG position", reason)
			closePosition(symbol)
			respondSuccess(w, fmt.Sprintf("MA Ribbon reversal: %s → closed LONG", reason))
			return
		}
	}

	respondSuccess(w, "MA Ribbon bearish signal recorded")
}

// ============================================================================
// MACD EVENT HANDLERS
// ============================================================================

// POST /webhook/macd/cross-up
func handleMACDCrossUp(w http.ResponseWriter, r *http.Request) {
	log.Printf("🔔 [WEBHOOK] Received MACD Cross Up event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON in MACD Cross Up: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Log the full request
	eventJSON, _ := json.MarshalIndent(event, "", "  ")
	log.Printf("📥 [REQUEST] %s", string(eventJSON))

	symbol := normalizeSymbol(event.Ticker)
	state := getPositionState(symbol)

	log.Printf("📊 MACD Cross Up for %s", symbol)

	mu.Lock()
	state.MACDCrossedUp = true
	state.MACDCrossedDown = false // Reset opposite condition
	mu.Unlock()

	// Check if we should exit SHORT position (reversal)
	if state.PositionOpen && state.Position == "short" {
		shouldExit, reason := shouldExitPosition(symbol, false, r)
		if shouldExit {
			log.Printf("⚠️ [EXIT] %s → closing SHORT position", reason)
			closePosition(symbol)
			respondSuccess(w, fmt.Sprintf("MACD cross up exit: %s → closed SHORT", reason))
			return
		}
	}

	// Check if we should open LONG position
	if shouldOpenPosition(symbol, true, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening LONG position")
		openLongPosition(symbol, event.Close)
		respondSuccess(w, "MACD cross up + strategy → LONG opened")
		return
	}

	respondSuccess(w, "MACD cross up condition set")
}

// POST /webhook/macd/cross-down
func handleMACDCrossDown(w http.ResponseWriter, r *http.Request) {
	log.Printf("🔔 [WEBHOOK] Received MACD Cross Down event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON in MACD Cross Down: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Log the full request
	eventJSON, _ := json.MarshalIndent(event, "", "  ")
	log.Printf("📥 [REQUEST] %s", string(eventJSON))

	symbol := normalizeSymbol(event.Ticker)
	state := getPositionState(symbol)

	log.Printf("📊 MACD Cross Down for %s", symbol)

	mu.Lock()
	state.MACDCrossedDown = true
	state.MACDCrossedUp = false // Reset opposite condition
	mu.Unlock()

	// Check if we should exit LONG position (reversal)
	if state.PositionOpen && state.Position == "long" {
		shouldExit, reason := shouldExitPosition(symbol, true, r)
		if shouldExit {
			log.Printf("⚠️ [EXIT] %s → closing LONG position", reason)
			closePosition(symbol)
			respondSuccess(w, fmt.Sprintf("MACD cross down exit: %s → closed LONG", reason))
			return
		}
	}

	// Check if we should open SHORT position
	if shouldOpenPosition(symbol, false, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening SHORT position")
		openShortPosition(symbol, event.Close)
		respondSuccess(w, "MACD cross down + strategy → SHORT opened")
		return
	}

	respondSuccess(w, "MACD cross down condition set")
}

// ============================================================================
// RSI CENTERLINE EXIT HANDLER
// ============================================================================

// POST /webhook/rsi/crossed-center
func handleRSICrossedCenter(w http.ResponseWriter, r *http.Request) {
	log.Printf("🔔 [WEBHOOK] Received RSI Crossed Center event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON in RSI Crossed Center: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Log the full request
	eventJSON, _ := json.MarshalIndent(event, "", "  ")
	log.Printf("📥 [REQUEST] %s", string(eventJSON))

	log.Printf("📥 [DATA] Ticker: %s, Exchange: %s, Close: %s", event.Ticker, event.Exchange, event.Close)

	symbol := normalizeSymbol(event.Ticker)
	log.Printf("🔄 [CONVERT] Normalized %s → %s", event.Ticker, symbol)

	state := getPositionState(symbol)

	log.Printf("⚖️  [RSI CENTER] RSI crossed centerline (50) for %s", symbol)

	// Track center cross for exit conditions
	mu.Lock()
	if state.PositionOpen {
		// First center cross → Set flag and wait
		if !state.RSICrossedCenter {
			state.RSICrossedCenter = true
			mu.Unlock()
			log.Printf("⚠️  [WARNING] First RSI center cross - position remains open, waiting for confirmation")
			respondSuccess(w, "RSI crossed center (1st warning - waiting for 2nd cross or RSI extreme)")
			return
		}
	}
	mu.Unlock()

	// Check if we should exit position (for LONG or SHORT)
	if state.PositionOpen {
		isLong := state.Position == "long"
		shouldExit, reason := shouldExitPosition(symbol, isLong, r)
		if shouldExit {
			log.Printf("⚠️ [EXIT] %s → closing %s position", reason, strings.ToUpper(state.Position))
			closePosition(symbol)
			respondSuccess(w, fmt.Sprintf("RSI center exit: %s → closed %s", reason, strings.ToUpper(state.Position)))
			return
		}
	}

	respondSuccess(w, "RSI crossed center (no exit triggered)")
} // ============================================================================
// OANDA TRADING FUNCTIONS
// ============================================================================

// Get instrument-specific leverage from OANDA
func getInstrumentLeverage(symbol string) (float64, error) {
	url := fmt.Sprintf("%s/v3/accounts/%s/instruments", oandaBaseURL, oandaAccountID)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+oandaAPIKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	instruments, ok := result["instruments"].([]interface{})
	if !ok {
		return 0, fmt.Errorf("invalid instruments data")
	}

	// Find the specific instrument
	for _, inst := range instruments {
		instrument, ok := inst.(map[string]interface{})
		if !ok {
			continue
		}

		name, _ := instrument["name"].(string)
		if name != symbol {
			continue
		}

		marginRateStr, ok := instrument["marginRate"].(string)
		if !ok {
			return 0, fmt.Errorf("marginRate not found for %s", symbol)
		}

		marginRate := 0.0
		fmt.Sscanf(marginRateStr, "%f", &marginRate)

		if marginRate <= 0 {
			return 0, fmt.Errorf("invalid margin rate: %f", marginRate)
		}

		// Leverage = 1 / marginRate
		// e.g., marginRate 0.02 = 50:1, 0.05 = 20:1, 0.03 = 33:1
		leverage := 1.0 / marginRate
		return leverage, nil
	}

	return 0, fmt.Errorf("instrument %s not found", symbol)
}

// Get current price for a symbol from OANDA
func getCurrentPrice(symbol string) (float64, error) {
	url := fmt.Sprintf("%s/v3/accounts/%s/pricing?instruments=%s", oandaBaseURL, oandaAccountID, symbol)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+oandaAPIKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	prices, ok := result["prices"].([]interface{})
	if !ok || len(prices) == 0 {
		return 0, fmt.Errorf("no pricing data")
	}

	priceData := prices[0].(map[string]interface{})

	// Get ask price for buying (long), bid for selling (short)
	// Using closeout ask as a reasonable midpoint
	asks, ok := priceData["asks"].([]interface{})
	if !ok || len(asks) == 0 {
		return 0, fmt.Errorf("no ask price")
	}

	askData := asks[0].(map[string]interface{})
	priceStr, ok := askData["price"].(string)
	if !ok {
		return 0, fmt.Errorf("invalid price format")
	}

	price := 0.0
	fmt.Sscanf(priceStr, "%f", &price)
	return price, nil
}

// Calculate units from USD amount
func calculateUnitsFromUSD(symbol string, usdAmount float64) (string, error) {
	log.Printf("💱 [CALCULATE] Converting $%.2f USD to units for %s", usdAmount, symbol)

	// Get current price
	price, err := getCurrentPrice(symbol)
	if err != nil {
		log.Printf("❌ [ERROR] Failed to get price for %s: %v", symbol, err)
		return "", err
	}

	log.Printf("📊 [PRICE] Current price for %s: %.5f", symbol, price)

	// For forex pairs like EUR_USD:
	// If we want to trade $1000 USD worth and EUR/USD = 1.0850
	// Units = USD Amount / Price = 1000 / 1.0850 ≈ 921.66 units
	units := usdAmount / price

	// Round to nearest integer
	unitsInt := int(units)

	log.Printf("✅ [CALCULATE] $%.2f USD = %d units at price %.5f", usdAmount, unitsInt, price)

	return fmt.Sprintf("%d", unitsInt), nil
}

// Calculate take profit price based on pips or percentage
func calculateTakeProfitPrice(symbol string, entryPrice float64, isLong bool) (string, error) {
	if takeProfitPips == "" && takeProfitPct == "" {
		return "", nil // No take profit set
	}

	var tpPrice float64

	if takeProfitPips != "" {
		// Calculate based on pips
		pips := 0.0
		fmt.Sscanf(takeProfitPips, "%f", &pips)

		// Determine pip value based on instrument
		// For JPY pairs (e.g., USD_JPY), 1 pip = 0.01
		// For most other pairs, 1 pip = 0.0001
		pipValue := 0.0001
		if strings.Contains(symbol, "JPY") {
			pipValue = 0.01
		}

		pipDistance := pips * pipValue

		if isLong {
			tpPrice = entryPrice + pipDistance
		} else {
			tpPrice = entryPrice - pipDistance
		}

		log.Printf("🎯 [TP CALC] %.0f pips = %.5f price distance", pips, pipDistance)
		log.Printf("🎯 [TP CALC] Entry: %.5f → TP: %.5f (%s)", entryPrice, tpPrice, map[bool]string{true: "LONG", false: "SHORT"}[isLong])

	} else if takeProfitPct != "" {
		// Calculate based on percentage
		pct := 0.0
		fmt.Sscanf(takeProfitPct, "%f", &pct)

		priceMove := entryPrice * (pct / 100.0)

		if isLong {
			tpPrice = entryPrice + priceMove
		} else {
			tpPrice = entryPrice - priceMove
		}

		log.Printf("🎯 [TP CALC] %.2f%% = %.5f price distance", pct, priceMove)
		log.Printf("🎯 [TP CALC] Entry: %.5f → TP: %.5f (%s)", entryPrice, tpPrice, map[bool]string{true: "LONG", false: "SHORT"}[isLong])
	}

	// Format price with appropriate precision
	precision := 5
	if strings.Contains(symbol, "JPY") {
		precision = 3
	}

	return fmt.Sprintf("%.*f", precision, tpPrice), nil
}

// Get trade specification for OANDA order (units or margin-based)
func getTradeSpec(symbol string, isLong bool) map[string]interface{} {
	orderSpec := map[string]interface{}{
		"instrument":   symbol,
		"timeInForce":  "FOK",
		"type":         "MARKET",
		"positionFill": "DEFAULT",
	}

	// Priority: MARGIN_AMOUNT > TRADE_USD_AMOUNT > TRADE_UNITS
	if tradeMargin != "" {
		// For margin-based trading, we need to calculate the USD position size
		// then convert to units based on the instrument
		marginAmount := 0.0
		fmt.Sscanf(tradeMargin, "%f", &marginAmount)

		if marginAmount > 0 {
			log.Printf("💰 [MARGIN] Using margin amount: $%.2f", marginAmount)

			// Get current price
			price, err := getCurrentPrice(symbol)
			if err != nil {
				log.Printf("❌ [ERROR] Failed to get price for margin calculation: %v", err)
				log.Printf("⚠️  [FALLBACK] Using default units: %s", tradeUnits)
				orderSpec["units"] = tradeUnits
				if !isLong {
					orderSpec["units"] = "-" + tradeUnits
				}
				return orderSpec
			}

			// Get instrument-specific leverage from OANDA
			leverage, err := getInstrumentLeverage(symbol)
			if err != nil {
				log.Printf("❌ [ERROR] Failed to get leverage for %s: %v", symbol, err)
				log.Printf("⚠️  [FALLBACK] Using default 50:1 leverage")
				leverage = 50.0
			}

			// Calculate units based on margin and leverage
			// Formula: margin_per_unit = price / leverage
			//          units = margin_amount / margin_per_unit
			marginPerUnit := price / leverage
			units := int(marginAmount / marginPerUnit)

			log.Printf("💱 [MARGIN CALC] Price: %.5f, Leverage: %.0f:1", price, leverage)
			log.Printf("� [MARGIN CALC] Margin per unit: $%.6f", marginPerUnit)
			log.Printf("💱 [MARGIN CALC] Margin: $%.2f ÷ $%.6f = %d units", marginAmount, marginPerUnit, units)
			log.Printf("💱 [POSITION SIZE] %d units × $%.5f = $%.2f position", units, price, float64(units)*price)

			unitsStr := fmt.Sprintf("%d", units)
			if !isLong {
				unitsStr = "-" + unitsStr
			}
			orderSpec["units"] = unitsStr

			// Add take profit if configured
			if takeProfitPips != "" || takeProfitPct != "" {
				tpPrice, err := calculateTakeProfitPrice(symbol, price, isLong)
				if err != nil {
					log.Printf("⚠️  [WARNING] Failed to calculate take profit: %v", err)
				} else if tpPrice != "" {
					orderSpec["takeProfitOnFill"] = map[string]interface{}{
						"price":       tpPrice,
						"timeInForce": "GTC",
					}
					log.Printf("🎯 [TP] Take profit set at %s", tpPrice)
				}
			}

			return orderSpec
		}
	}

	// Fallback to units-based ordering
	var units string
	if tradeUSDAmount != "" {
		// Calculate units from USD amount
		usdAmount := 0.0
		fmt.Sscanf(tradeUSDAmount, "%f", &usdAmount)

		if usdAmount > 0 {
			calculatedUnits, err := calculateUnitsFromUSD(symbol, usdAmount)
			if err != nil {
				log.Printf("⚠️  [WARNING] Failed to calculate units from USD, using default: %s", tradeUnits)
				units = tradeUnits
			} else {
				units = calculatedUnits
			}
		} else {
			units = tradeUnits
		}
	} else {
		// Use fixed units
		units = tradeUnits
	}

	// For SHORT positions, units should be negative
	if !isLong {
		units = "-" + units
	}

	orderSpec["units"] = units

	// Add take profit if configured
	if takeProfitPips != "" || takeProfitPct != "" {
		// Get current price for TP calculation
		price, err := getCurrentPrice(symbol)
		if err != nil {
			log.Printf("⚠️  [WARNING] Failed to get price for TP calculation: %v", err)
		} else {
			tpPrice, err := calculateTakeProfitPrice(symbol, price, isLong)
			if err != nil {
				log.Printf("⚠️  [WARNING] Failed to calculate take profit: %v", err)
			} else if tpPrice != "" {
				orderSpec["takeProfitOnFill"] = map[string]interface{}{
					"price":       tpPrice,
					"timeInForce": "GTC",
				}
				log.Printf("🎯 [TP] Take profit set at %s", tpPrice)
			}
		}
	}

	return orderSpec
}

// Fetch existing open positions from OANDA on startup
func syncPositionsFromOanda() error {
	log.Printf("🔄 [SYNC] Fetching open positions from OANDA...")

	url := fmt.Sprintf("%s/v3/accounts/%s/openTrades", oandaBaseURL, oandaAccountID)
	log.Printf("📤 [OANDA] GET %s", url)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+oandaAPIKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("❌ [ERROR] Failed to fetch open trades: %v", err)
		return err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("❌ [ERROR] Failed to parse OANDA response: %v", err)
		return err
	}

	log.Printf("📥 [OANDA] Response status: %d", resp.StatusCode)

	if resp.StatusCode != 200 {
		log.Printf("❌ [ERROR] Failed to fetch positions (status %d)", resp.StatusCode)
		log.Printf("📄 [RESPONSE] %+v", result)
		return fmt.Errorf("failed to fetch positions: status %d", resp.StatusCode)
	}

	trades, ok := result["trades"].([]interface{})
	if !ok {
		log.Printf("✅ [SYNC] No open trades found")
		return nil
	}

	log.Printf("📊 [SYNC] Found %d open trade(s)", len(trades))

	mu.Lock()
	defer mu.Unlock()

	for _, trade := range trades {
		tradeMap, ok := trade.(map[string]interface{})
		if !ok {
			continue
		}

		instrument, _ := tradeMap["instrument"].(string)
		tradeID, _ := tradeMap["id"].(string)
		currentUnits, _ := tradeMap["currentUnits"].(string)

		if instrument == "" || tradeID == "" {
			continue
		}

		// Determine if long or short based on units
		var positionType string
		if currentUnits[0] == '-' {
			positionType = "short"
		} else {
			positionType = "long"
		}

		// Create or update position state
		if _, exists := positions[instrument]; !exists {
			positions[instrument] = &PositionState{
				Symbol: instrument,
			}
		}

		positions[instrument].PositionOpen = true
		positions[instrument].Position = positionType
		positions[instrument].TradeID = tradeID

		log.Printf("💾 [SYNC] Loaded %s position: %s (ID: %s, Units: %s)",
			positionType, instrument, tradeID, currentUnits)
	}

	log.Printf("✅ [SYNC] Position sync complete")
	return nil
}

func openLongPosition(symbol string, price string) {
	log.Printf("🟢 Opening LONG position for %s at %s", symbol, price)
	log.Printf("📤 [OANDA] Preparing market order request...")

	orderSpec := getTradeSpec(symbol, true) // true = LONG

	orderData := map[string]interface{}{
		"order": orderSpec,
	}

	log.Printf("📋 [ORDER] %+v", orderData)

	tradeID, err := sendOandaOrder(orderData)
	if err != nil {
		log.Printf("❌ Failed to open LONG position: %v", err)
		return
	}

	mu.Lock()
	state := positions[symbol]
	state.PositionOpen = true
	state.Position = "long"
	state.TradeID = tradeID
	mu.Unlock()

	log.Printf("✅ LONG position opened: %s (ID: %s)", symbol, tradeID)
	log.Printf("💾 [STATE] Position updated - Open=%v, Type=%s, TradeID=%s",
		state.PositionOpen, state.Position, state.TradeID)
}

func openShortPosition(symbol string, price string) {
	log.Printf("🔴 Opening SHORT position for %s at %s", symbol, price)
	log.Printf("📤 [OANDA] Preparing market order request...")

	orderSpec := getTradeSpec(symbol, false) // false = SHORT

	orderData := map[string]interface{}{
		"order": orderSpec,
	}

	log.Printf("📋 [ORDER] %+v", orderData)

	tradeID, err := sendOandaOrder(orderData)
	if err != nil {
		log.Printf("❌ Failed to open SHORT position: %v", err)
		return
	}

	mu.Lock()
	state := positions[symbol]
	state.PositionOpen = true
	state.Position = "short"
	state.TradeID = tradeID
	mu.Unlock()

	log.Printf("✅ SHORT position opened: %s (ID: %s)", symbol, tradeID)
	log.Printf("💾 [STATE] Position updated - Open=%v, Type=%s, TradeID=%s",
		state.PositionOpen, state.Position, state.TradeID)
}

func closePosition(symbol string) {
	mu.RLock()
	state := positions[symbol]
	tradeID := state.TradeID
	position := state.Position
	mu.RUnlock()

	if tradeID == "" {
		log.Printf("⚠️  No trade ID found for %s", symbol)
		return
	}

	log.Printf("🔵 Closing %s position for %s (ID: %s)", position, symbol, tradeID)

	url := fmt.Sprintf("%s/v3/accounts/%s/trades/%s/close", oandaBaseURL, oandaAccountID, tradeID)
	log.Printf("📤 [OANDA] PUT %s", url)

	req, _ := http.NewRequest("PUT", url, nil)
	req.Header.Set("Authorization", "Bearer "+oandaAPIKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("❌ Failed to close position: %v", err)
		return
	}
	defer resp.Body.Close()

	log.Printf("📥 [OANDA] Response status: %d", resp.StatusCode)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		mu.Lock()
		state.PositionOpen = false
		state.Position = "none"
		state.TradeID = ""
		// Reset all state machine flags
		state.RSICrossedUp = false
		state.RSICrossedDown = false
		state.MACDCrossedUp = false
		state.MACDCrossedDown = false
		state.RSICrossedCenter = false
		mu.Unlock()

		log.Printf("✅ Position closed: %s", symbol)
		log.Printf("💾 [STATE] Position updated - Open=%v, Type=%s, All flags reset", state.PositionOpen, state.Position)
	} else {
		// Read response body for error details
		var responseBody map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&responseBody)
		log.Printf("❌ Failed to close position (status %d)", resp.StatusCode)
		log.Printf("📄 [RESPONSE] %+v", responseBody)
	}
}

func sendOandaOrder(orderData map[string]interface{}) (string, error) {
	url := fmt.Sprintf("%s/v3/accounts/%s/orders", oandaBaseURL, oandaAccountID)
	log.Printf("📤 [OANDA] POST %s", url)

	jsonData, _ := json.MarshalIndent(orderData, "", "  ")
	log.Printf("📋 [REQUEST] %s", string(jsonData))

	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	req.Header.Set("Authorization", "Bearer "+oandaAPIKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("❌ [HTTP ERROR] %v", err)
		return "", err
	}
	defer resp.Body.Close()

	log.Printf("📥 [OANDA] Response status: %d", resp.StatusCode)

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	// Log response
	responseBytes, _ := json.MarshalIndent(result, "", "  ")
	log.Printf("📄 [RESPONSE] %s", string(responseBytes))

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		log.Printf("✅ [SUCCESS] Order accepted by OANDA")

		// Extract trade ID from response
		if orderFill, ok := result["orderFillTransaction"].(map[string]interface{}); ok {
			log.Printf("🔍 [PARSE] Found orderFillTransaction")
			if tradeOpened, ok := orderFill["tradeOpened"].(map[string]interface{}); ok {
				log.Printf("🔍 [PARSE] Found tradeOpened")
				if tradeID, ok := tradeOpened["tradeID"].(string); ok {
					log.Printf("✅ [TRADE ID] %s", tradeID)
					return tradeID, nil
				}
			}
		}

		log.Printf("⚠️  [WARNING] Could not extract trade ID from response, using 'unknown'")
		return "unknown", nil
	}

	log.Printf("❌ [OANDA ERROR] Order rejected")
	log.Printf("📄 [ERROR DETAILS] %+v", result)
	return "", fmt.Errorf("order failed with status %d", resp.StatusCode)
}

// ============================================================================
// UTILITY FUNCTIONS
// ============================================================================

// Get ngrok tunnel URL
func getNgrokURL() string {
	resp, err := http.Get("http://ngrok:4040/api/tunnels")
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return ""
	}

	tunnels, ok := result["tunnels"].([]interface{})
	if !ok || len(tunnels) == 0 {
		return ""
	}

	tunnel := tunnels[0].(map[string]interface{})
	publicURL, _ := tunnel["public_url"].(string)
	return publicURL
}

// Convert ticker format (e.g., "EURUSD" or "EUR/USD" to "EUR_USD")
func normalizeSymbol(ticker string) string {
	// Simple conversion - adjust as needed
	if len(ticker) == 6 {
		return ticker[:3] + "_" + ticker[3:]
	}
	return ticker
}

func respondSuccess(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": message,
	})
}

// GET /health
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "healthy",
		"service": "tradingview-webhook-bot",
		"version": "1.0.0",
	})
}

// GET /status
func statusHandler(w http.ResponseWriter, r *http.Request) {
	mu.RLock()
	defer mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"positions": positions,
	})
}

// ============================================================================
// MAIN
// ============================================================================

func main() {
	// Validate environment variables
	if oandaAPIKey == "" || oandaAccountID == "" {
		log.Fatal("❌ OANDA_API_KEY and OANDA_ACCOUNT_ID must be set")
	}

	// Set trade configuration (priority: MARGIN_AMOUNT > TRADE_USD_AMOUNT > TRADE_UNITS)
	tradeMargin = os.Getenv("MARGIN_AMOUNT")
	tradeUSDAmount = os.Getenv("TRADE_USD_AMOUNT")
	tradeUnits = os.Getenv("TRADE_UNITS")

	// Take profit configuration
	takeProfitPips = os.Getenv("TAKE_PROFIT_PIPS")
	takeProfitPct = os.Getenv("TAKE_PROFIT_PCT")
	takeProfitDollars = os.Getenv("TAKE_PROFIT_DOLLARS")

	// Strategy configuration
	strategyName = os.Getenv("STRATEGY")
	if strategyName == "" {
		strategyName = "default" // Use default strategy if not specified
	}

	// Default to 100 units if nothing specified
	if tradeUnits == "" && tradeUSDAmount == "" && tradeMargin == "" {
		tradeUnits = "100"
	}

	log.Println("🚀 TradingView Webhook Trading Bot Starting...")
	log.Printf("📡 OANDA Account: %s", oandaAccountID)
	log.Printf("🌐 OANDA API: %s", oandaBaseURL)

	// Load trading strategy
	strategy, err := loadStrategy(strategyName)
	if err != nil {
		log.Fatalf("❌ Failed to load strategy '%s': %v", strategyName, err)
	}
	activeStrategy = *strategy
	log.Printf("🚀 [STRATEGY] Active: %s", activeStrategy.Name)

	// Show active trading mode
	if tradeMargin != "" {
		log.Printf("💰 Margin Amount: $%s (OANDA calculates position size from leverage)", tradeMargin)
		log.Printf("   ⚡ With 50:1 leverage, $%s margin = ~$%s position", tradeMargin, func() string {
			margin := 0.0
			fmt.Sscanf(tradeMargin, "%f", &margin)
			return fmt.Sprintf("%.0f", margin*50)
		}())
	} else if tradeUSDAmount != "" {
		log.Printf("💵 Trade Amount: $%s USD (units calculated per trade)", tradeUSDAmount)
	} else {
		log.Printf("💰 Trade Units: %s (fixed)", tradeUnits)
	}

	// Show take profit settings
	if takeProfitPips != "" {
		log.Printf("🎯 Take Profit: %s pips", takeProfitPips)
	} else if takeProfitPct != "" {
		log.Printf("🎯 Take Profit: %s%%", takeProfitPct)
	} else {
		log.Printf("🎯 Take Profit: None (manual exit only)")
	}

	// Sync existing positions from OANDA on startup
	if err := syncPositionsFromOanda(); err != nil {
		log.Printf("⚠️  [WARNING] Could not sync positions from OANDA: %v", err)
		log.Printf("⚠️  [WARNING] Continuing with empty state - be careful of duplicate positions!")
	}

	// Health & Status
	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/status", statusHandler)

	// RSI Webhooks
	http.HandleFunc("/webhook/rsi/crossed-up", handleRSICrossedUp)
	http.HandleFunc("/webhook/rsi/crossed-down", handleRSICrossedDown)
	http.HandleFunc("/webhook/rsi/crossed-center", handleRSICrossedCenter)

	// MA Ribbon Webhooks
	http.HandleFunc("/webhook/ma/ribbon-bullish", handleMARibbonBullish)
	http.HandleFunc("/webhook/ma/ribbon-bearish", handleMARibbonBearish)

	// MACD Webhooks
	http.HandleFunc("/webhook/macd/cross-up", handleMACDCrossUp)
	http.HandleFunc("/webhook/macd/cross-down", handleMACDCrossDown)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("✅ Server listening on port %s", port)

	// Fetch and display ngrok tunnel URL
	log.Println("\n🌐 Fetching ngrok tunnel URL...")
	time.Sleep(2 * time.Second) // Give ngrok time to start
	ngrokURL := getNgrokURL()

	if ngrokURL != "" {
		log.Printf("✅ Public ngrok URL: %s", ngrokURL)
		log.Println("\n📱 TradingView Webhook URLs:")
		log.Printf("   POST %s/webhook/rsi/crossed-up        (RSI > 70)", ngrokURL)
		log.Printf("   POST %s/webhook/rsi/crossed-down      (RSI < 30)", ngrokURL)
		log.Printf("   POST %s/webhook/rsi/crossed-center    (RSI crosses 50)", ngrokURL)
		log.Printf("   POST %s/webhook/ma/ribbon-bullish     (MA ribbon bullish)", ngrokURL)
		log.Printf("   POST %s/webhook/ma/ribbon-bearish     (MA ribbon bearish)", ngrokURL)
		log.Printf("   POST %s/webhook/macd/cross-up         (MACD crosses up)", ngrokURL)
		log.Printf("   POST %s/webhook/macd/cross-down       (MACD crosses down)", ngrokURL)
		log.Println("\n📊 Monitoring:")
		log.Printf("   GET %s/health", ngrokURL)
		log.Printf("   GET %s/status", ngrokURL)
		log.Println("\n🖥️  ngrok Web Interface: http://localhost:4040")
	} else {
		log.Println("⚠️  Could not fetch ngrok URL (ngrok may not be running)")
		log.Println("   Run 'docker-compose up' to start ngrok automatically")
		log.Println("\n� Local Webhook Endpoints:")
		log.Println("   POST /webhook/rsi/crossed-up        (RSI > 70)")
		log.Println("   POST /webhook/rsi/crossed-down      (RSI < 30)")
		log.Println("   POST /webhook/rsi/crossed-center    (RSI crosses 50)")
		log.Println("   POST /webhook/ma/ribbon-bullish     (MA ribbon bullish)")
		log.Println("   POST /webhook/ma/ribbon-bearish     (MA ribbon bearish)")
		log.Println("   POST /webhook/macd/cross-up         (MACD crosses up)")
		log.Println("   POST /webhook/macd/cross-down       (MACD crosses down)")
		log.Println("\n� Monitoring:")
		log.Println("   GET /health")
		log.Println("   GET /status")
	}
	log.Println("")

	log.Fatal(http.ListenAndServe(":"+port, nil))
}
