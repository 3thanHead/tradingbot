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
	"sort"
	"strconv"
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
// COMBINATION MODES:
// - "any": Any single condition triggers the action
// - "all": All conditions must be met (order doesn't matter)
// - "sequential": All conditions must be met IN SEQUENTIAL ORDER (1 → 2 → 3)
// ============================================================================

// ConditionNode represents either a single condition or a group of conditions
// This allows for nested condition groups with different combination modes
type ConditionNode struct {
	Type        string          `json:"type"`                  // "condition" or "group"
	Webhook     string          `json:"webhook,omitempty"`     // For type="condition"
	Comment     string          `json:"comment,omitempty"`     // Description (legacy)
	Description string          `json:"description,omitempty"` // Description (preferred)
	Combination string          `json:"combination,omitempty"` // For type="group": "any", "all", or "sequential"
	Conditions  []ConditionNode `json:"conditions,omitempty"`  // For type="group": nested conditions
}

// EntryConditions defines how entry conditions combine to trigger a trade
// Supports both simple flat arrays (backward compatible) and nested groups
type EntryConditions struct {
	Type        string          `json:"type,omitempty"` // "group" for nested, empty for simple
	Combination string          `json:"combination"`    // "any", "all", or "sequential"
	Conditions  []ConditionNode `json:"conditions"`     // Can be simple conditions or nested groups
}

// ExitConditions defines how exit conditions combine to close a position
// Supports both simple flat arrays (backward compatible) and nested groups
type ExitConditions struct {
	Type        string          `json:"type,omitempty"` // "group" for nested, empty for simple
	Combination string          `json:"combination"`    // "any", "all", or "sequential"
	Conditions  []ConditionNode `json:"conditions"`     // Can be simple conditions or nested groups
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
	Symbol             string
	PositionOpen       bool
	Position           string // "long" or "short"
	TradeID            string
	Exchange           string  // Exchange name (OANDA, NYSE, NASDAQ, etc.)
	IsSimulated        bool    // True if this is a simulated trade (non-OANDA)
	SimulatedEntry     string  // Entry time for simulated trade
	SimulatedExit      string  // Exit time for simulated trade
	SimulatedPrice     string  // Entry price for simulated trade
	SimulatedExitPrice string  // Exit price for simulated trade
	SimulatedPL        string  // Final P/L for simulated trade (stored on close)
	LatestPrice        string  // Latest price from webhook (for P/L calculation)
	MACDCrossedUp      bool    // Tracks if MACD crossed up
	MACDCrossedDown    bool    // Tracks if MACD crossed down
	SwingHigh          float64 // Latest swing high price level
	SwingLow           float64 // Latest swing low price level

	// Stochastic indicator tracking
	StochInOversold   bool // Both K and D lines are in oversold (<20)
	StochInOverbought bool // Both K and D lines are in overbought (>80)

	// Stochastic RSI indicator tracking
	StochRSICrossedUp20   bool // Stochastic RSI crossed up above 20
	StochRSICrossedDown20 bool // Stochastic RSI crossed down below 20
	StochRSICrossedUp50   bool // Stochastic RSI crossed up above 50
	StochRSICrossedDown50 bool // Stochastic RSI crossed down below 50
	StochRSICrossedUp80   bool // Stochastic RSI crossed up above 80
	StochRSICrossedDown80 bool // Stochastic RSI crossed down below 80

	// RSI trend tracking
	RSIAbove50 bool // RSI is above 50 (uptrend)
	RSIBelow50 bool // RSI is below 50 (downtrend)

	// RSI directional crosses (used by fast_scalp_1min)
	RSICrossedUp25   bool // RSI crossed up from oversold at 25
	RSICrossedDown25 bool // RSI crossed down through 25
	RSICrossedUp30   bool // RSI crossed up from oversold at 30
	RSICrossedDown30 bool // RSI crossed down through 30
	RSICrossedUp40   bool // RSI crossed up through 40
	RSICrossedDown40 bool // RSI crossed down through 40
	RSICrossedUp60   bool // RSI crossed up through 60
	RSICrossedDown60 bool // RSI crossed down through 60
	RSICrossedUp70   bool // RSI crossed up through 70
	RSICrossedDown70 bool // RSI crossed down from overbought at 70
	RSICrossedUp75   bool // RSI crossed up through 75
	RSICrossedDown75 bool // RSI crossed down from overbought at 75

	// EMA trend tracking (higher timeframe)
	PriceAboveEMA9   bool // Price is above EMA 9 (very short-term)
	PriceBelowEMA9   bool // Price is below EMA 9 (very short-term)
	PriceAboveEMA50  bool // Price is above EMA 50 (uptrend)
	PriceBelowEMA50  bool // Price is below EMA 50 (downtrend)
	PriceAboveEMA20  bool // Price is above EMA 20 (short-term trend)
	PriceBelowEMA20  bool // Price is below EMA 20 (short-term trend)
	PriceAboveEMA200 bool // Price is above EMA 200 (long-term trend)
	PriceBelowEMA200 bool // Price is below EMA 200 (long-term trend)

	// EMA crossover tracking
	EMA9CrossedUpEMA21   bool // EMA 9 crossed above EMA 21 (bullish)
	EMA9CrossedDownEMA21 bool // EMA 9 crossed below EMA 21 (bearish)

	// EMA position tracking (for detecting crosses internally)
	EMA9AboveEMA21 bool // EMA 9 is currently above EMA 21
	EMA9BelowEMA21 bool // EMA 9 is currently below EMA 21

	// MA Ribbon position tracking (for MA1/MA2/MA3 alignment)
	MA2AboveMA3 bool // MA2 is currently above MA3
	MA2BelowMA3 bool // MA2 is currently below MA3

	// ATR volatility tracking
	ATRAboveAverage bool // ATR is above its 20-period average (high volatility)
	ATRBelowAverage bool // ATR is below its 20-period average (low volatility)

	// MACD histogram tracking
	MACDHistIncreasing bool // MACD histogram is increasing
	MACDHistDecreasing bool // MACD histogram is decreasing
	MACDHistAboveZero  bool // MACD histogram is above 0
	MACDHistBelowZero  bool // MACD histogram is below 0

	// MA Ribbon tracking
	MARibbonBullish bool // MA ribbon aligned bullish (5>10>20>50>100)
	MARibbonBearish bool // MA ribbon aligned bearish (5<10<20<50<100)

	// SMC (Smart Money Concept) structure tracking
	SMCLowLow   bool // SMC Lower Low (LL) detected
	SMCHighLow  bool // SMC Higher Low (HL) detected
	SMCLowHigh  bool // SMC Lower High (LH) detected
	SMCHighHigh bool // SMC Higher High (HH) detected

	// Cross detection initialization tracking
	EMA9EMA21StateInitialized bool   // True once we've seen at least one position state (prevents false crosses on startup)
	LastClosedDirection       string // Track last closed position direction ("long", "short", or "")
	OppositeDirectionOccurred bool   // Track if opposite MA cross occurred after position close

	// Track which entry conditions have been completed
	EntryConditionsCompleted map[string]bool // Maps condition index to completion status

	// Track which exit conditions have been completed
	ExitConditionsCompleted map[string]bool // Maps condition index to completion status
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
	stopLossPips      string                               // Stop loss in pips (e.g., "30")
	stopLossPct       string                               // Stop loss in percentage (e.g., "1.5" for 1.5%)
	stopLossDollars   string                               // Stop loss in dollar amount (e.g., "50" for $50 loss)

	// Strategy system
	activeStrategy          Strategy  // Currently loaded strategy
	strategyName            string    // Name of strategy file to load
	firstWebhookStatusShown bool      // Track if first webhook status has been shown
	lastStatusReportTime    time.Time // Track last time status was reported
	timezoneOffset          int       // Timezone offset in hours (e.g., -5 for UTC-5, 0 for UTC)
)

// Get or create position state for a symbol
func getPositionState(symbol string) *PositionState {
	mu.Lock()
	defer mu.Unlock()

	if _, exists := positions[symbol]; !exists {
		positions[symbol] = &PositionState{
			Symbol:                   symbol,
			PositionOpen:             false,
			Position:                 "none",
			EntryConditionsCompleted: make(map[string]bool),
			ExitConditionsCompleted:  make(map[string]bool),
		}
	}

	// Safety check: ensure maps are always initialized (in case state was created elsewhere)
	state := positions[symbol]
	if state.EntryConditionsCompleted == nil {
		state.EntryConditionsCompleted = make(map[string]bool)
	}
	if state.ExitConditionsCompleted == nil {
		state.ExitConditionsCompleted = make(map[string]bool)
	}

	return state
}

// updateLatestPrice updates the latest price for a symbol from webhook data
func updateLatestPrice(symbol string, price string) {
	if price == "" {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	if state, exists := positions[symbol]; exists {
		state.LatestPrice = price

		// Display P/L update if position is open and simulated
		if state.PositionOpen && state.IsSimulated && state.SimulatedPrice != "" {
			entryPrice, _ := strconv.ParseFloat(state.SimulatedPrice, 64)
			currentPrice, _ := strconv.ParseFloat(price, 64)

			var plDollars, plPercent float64
			if state.Position == "long" {
				plDollars = currentPrice - entryPrice
			} else {
				plDollars = entryPrice - currentPrice
			}
			plPercent = (plDollars / entryPrice) * 100

			plColor := "🟢"
			plSign := "+"
			if plDollars < 0 {
				plColor = "🔴"
				plSign = ""
			}

			log.Printf("💰 [%s] %s %s @ %s | P/L: %s %s$%.5f / %s%.2f%%",
				symbol, strings.ToUpper(state.Position), state.Exchange, price,
				plColor, plSign, plDollars, plSign, plPercent)
		}
	}
}

// ============================================================================
// STRATEGY SYSTEM FUNCTIONS
// ============================================================================

// Check if webhook path is a LONG entry condition in the current strategy
func isLongEntryCondition(webhookPath string) bool {
	if activeStrategy.Long == nil {
		return false
	}
	for _, condition := range activeStrategy.Long.Entry.Conditions {
		if condition.Webhook == webhookPath {
			return true
		}
	}
	return false
}

// Check if webhook path is a SHORT entry condition in the current strategy
func isShortEntryCondition(webhookPath string) bool {
	if activeStrategy.Short == nil {
		return false
	}
	for _, condition := range activeStrategy.Short.Entry.Conditions {
		if condition.Webhook == webhookPath {
			return true
		}
	}
	return false
}

// Track that opposite direction condition occurred (for reversal tracking)
func trackOppositeDirection(symbol string, webhookPath string) {
	state := getPositionState(symbol)

	// If last close was LONG and this is a SHORT entry condition, mark opposite occurred
	if state.LastClosedDirection == "long" && isShortEntryCondition(webhookPath) {
		mu.Lock()
		state.OppositeDirectionOccurred = true
		mu.Unlock()
		log.Printf("✅ [REVERSAL] SHORT entry condition triggered after LONG close - reversal confirmed")
	}

	// If last close was SHORT and this is a LONG entry condition, mark opposite occurred
	if state.LastClosedDirection == "short" && isLongEntryCondition(webhookPath) {
		mu.Lock()
		state.OppositeDirectionOccurred = true
		mu.Unlock()
		log.Printf("✅ [REVERSAL] LONG entry condition triggered after SHORT close - reversal confirmed")
	}
}

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

		// Entry conditions
		if strategy.Entry.Combination == "sequential" {
			log.Printf("🟢 ENTRY (%s - %d conditions IN ORDER):", strings.ToUpper(strategy.Entry.Combination), len(strategy.Entry.Conditions))
		} else {
			log.Printf("🟢 ENTRY (%s - %d conditions):", strings.ToUpper(strategy.Entry.Combination), len(strategy.Entry.Conditions))
		}
		for i, condition := range strategy.Entry.Conditions {
			comment := condition.Description
			if comment == "" {
				comment = condition.Comment // Fallback to legacy field
			}
			if comment == "" {
				comment = "No description"
			}
			log.Printf("   %d. %s → %s", i+1, condition.Webhook, comment)
		}
		log.Println("")

		// Exit conditions
		if len(strategy.Exit.Conditions) > 0 {
			if strategy.Exit.Combination == "sequential" {
				log.Printf("🔴 EXIT (%s - %d conditions IN ORDER):", strings.ToUpper(strategy.Exit.Combination), len(strategy.Exit.Conditions))
			} else {
				log.Printf("🔴 EXIT (%s - %d conditions):", strings.ToUpper(strategy.Exit.Combination), len(strategy.Exit.Conditions))
			}
			for i, condition := range strategy.Exit.Conditions {
				comment := condition.Description
				if comment == "" {
					comment = condition.Comment // Fallback to legacy field
				}
				if comment == "" {
					comment = "No description"
				}
				log.Printf("   %d. %s → %s", i+1, condition.Webhook, comment)
			}
		} else {
			log.Printf("🔴 EXIT (%s - %d conditions):", strings.ToUpper(strategy.Exit.Combination), len(strategy.Exit.Conditions))
			for i, condition := range strategy.Exit.Conditions {
				comment := condition.Description
				if comment == "" {
					comment = condition.Comment // Fallback to legacy field
				}
				if comment == "" {
					comment = "No description"
				}
				log.Printf("   %d. %s → %s", i+1, condition.Webhook, comment)
			}
		}

	} else if strategy.Long != nil && strategy.Short != nil {
		// Separate LONG/SHORT format
		log.Printf("📊 Format: Separate LONG/SHORT configurations")
		log.Println("")

		// LONG entry/exit
		if strategy.Long.Entry.Combination == "sequential" {
			log.Printf("🟢 LONG ENTRY (%s - %d conditions IN ORDER):", strings.ToUpper(strategy.Long.Entry.Combination), len(strategy.Long.Entry.Conditions))
		} else {
			log.Printf("🟢 LONG ENTRY (%s - %d conditions):", strings.ToUpper(strategy.Long.Entry.Combination), len(strategy.Long.Entry.Conditions))
		}
		for i, condition := range strategy.Long.Entry.Conditions {
			comment := condition.Description
			if comment == "" {
				comment = condition.Comment // Fallback to legacy field
			}
			if comment == "" {
				comment = "No description"
			}
			log.Printf("   %d. %s → %s", i+1, condition.Webhook, comment)
		}
		log.Println("")

		log.Printf("🔴 LONG EXIT (%s - %d %s%s):", strings.ToUpper(strategy.Long.Exit.Combination),
			func() int {
				if len(strategy.Long.Exit.Conditions) > 0 {
					return len(strategy.Long.Exit.Conditions)
				}
				return len(strategy.Long.Exit.Conditions)
			}(),
			func() string {
				if len(strategy.Long.Exit.Conditions) > 0 {
					return "steps"
				}
				return "conditions"
			}(),
			func() string {
				if len(strategy.Long.Exit.Conditions) > 0 && strategy.Long.Exit.Combination == "sequential" {
					return " IN ORDER"
				}
				return ""
			}())
		if len(strategy.Long.Exit.Conditions) > 0 {
			for i, condition := range strategy.Long.Exit.Conditions {
				comment := condition.Description
				if comment == "" {
					comment = condition.Comment // Fallback to legacy field
				}
				if comment == "" {
					comment = "No description"
				}
				log.Printf("   %d. %s → %s", i+1, condition.Webhook, comment)
			}
		} else {
			for i, condition := range strategy.Long.Exit.Conditions {
				comment := condition.Description
				if comment == "" {
					comment = condition.Comment // Fallback to legacy field
				}
				if comment == "" {
					comment = "No description"
				}
				log.Printf("   %d. %s → %s", i+1, condition.Webhook, comment)
			}
		}
		log.Println("")

		// SHORT entry/exit
		if strategy.Short.Entry.Combination == "sequential" {
			log.Printf("🟠 SHORT ENTRY (%s - %d conditions IN ORDER):", strings.ToUpper(strategy.Short.Entry.Combination), len(strategy.Short.Entry.Conditions))
		} else {
			log.Printf("🟠 SHORT ENTRY (%s - %d conditions):", strings.ToUpper(strategy.Short.Entry.Combination), len(strategy.Short.Entry.Conditions))
		}
		for i, condition := range strategy.Short.Entry.Conditions {
			comment := condition.Description
			if comment == "" {
				comment = condition.Comment // Fallback to legacy field
			}
			if comment == "" {
				comment = "No description"
			}
			log.Printf("   %d. %s → %s", i+1, condition.Webhook, comment)
		}
		log.Println("")

		log.Printf("🔴 SHORT EXIT (%s - %d %s%s):", strings.ToUpper(strategy.Short.Exit.Combination),
			func() int {
				if len(strategy.Short.Exit.Conditions) > 0 {
					return len(strategy.Short.Exit.Conditions)
				}
				return len(strategy.Short.Exit.Conditions)
			}(),
			func() string {
				if len(strategy.Short.Exit.Conditions) > 0 {
					return "steps"
				}
				return "conditions"
			}(),
			func() string {
				if len(strategy.Short.Exit.Conditions) > 0 && strategy.Short.Exit.Combination == "sequential" {
					return " IN ORDER"
				}
				return ""
			}())
		if len(strategy.Short.Exit.Conditions) > 0 {
			for i, condition := range strategy.Short.Exit.Conditions {
				comment := condition.Description
				if comment == "" {
					comment = condition.Comment // Fallback to legacy field
				}
				if comment == "" {
					comment = "No description"
				}
				log.Printf("   %d. %s → %s", i+1, condition.Webhook, comment)
			}
		} else {
			for i, condition := range strategy.Short.Exit.Conditions {
				comment := condition.Description
				if comment == "" {
					comment = condition.Comment // Fallback to legacy field
				}
				if comment == "" {
					comment = "No description"
				}
				log.Printf("   %d. %s → %s", i+1, condition.Webhook, comment)
			}
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

// Helper: validate entry conditions (supports both simple and nested formats)
func validateEntryConditions(entry *EntryConditions) error {
	if len(entry.Conditions) == 0 {
		return fmt.Errorf("must have at least one condition")
	}

	validCombination := map[string]bool{"all": true, "sequential": true, "any": true}
	if !validCombination[entry.Combination] {
		return fmt.Errorf("invalid combination: %s (must be 'all', 'sequential', or 'any')", entry.Combination)
	}

	// Validate each condition node (recursive for groups)
	for i := range entry.Conditions {
		if err := validateConditionNode(&entry.Conditions[i], i+1); err != nil {
			return err
		}
	}

	return nil
}

// Helper: validate exit conditions (supports both simple and nested formats)
func validateExitConditions(exit *ExitConditions) error {
	if len(exit.Conditions) == 0 {
		return fmt.Errorf("must have at least one condition")
	}

	validCombination := map[string]bool{"any": true, "all": true, "sequential": true}
	if !validCombination[exit.Combination] {
		return fmt.Errorf("invalid combination: %s (must be 'any', 'all', or 'sequential')", exit.Combination)
	}

	// Validate each condition node (recursive for groups)
	for i := range exit.Conditions {
		if err := validateConditionNode(&exit.Conditions[i], i+1); err != nil {
			return err
		}
	}

	return nil
}

// Helper: Get description from node (prefer Description over Comment)
func getNodeDescription(node *ConditionNode) string {
	if node.Description != "" {
		return node.Description
	}
	if node.Comment != "" {
		return node.Comment
	}
	if node.Type == "group" {
		return "group"
	}
	return node.Webhook
}

// Helper: validate a single condition node (recursive for groups)
func validateConditionNode(node *ConditionNode, index int) error {
	// Default to "condition" if type not specified (backward compatibility)
	nodeType := node.Type
	if nodeType == "" {
		nodeType = "condition"
	}

	if nodeType == "condition" {
		// Simple condition - must have webhook
		if node.Webhook == "" {
			return fmt.Errorf("condition %d is missing webhook path", index)
		}
		return nil
	} else if nodeType == "group" {
		// Group - must have combination and nested conditions
		if node.Combination == "" {
			return fmt.Errorf("group %d is missing combination mode", index)
		}
		validCombination := map[string]bool{"all": true, "sequential": true, "any": true}
		if !validCombination[node.Combination] {
			return fmt.Errorf("group %d has invalid combination: %s", index, node.Combination)
		}
		if len(node.Conditions) == 0 {
			return fmt.Errorf("group %d has no conditions", index)
		}
		// Recursively validate nested conditions
		for i := range node.Conditions {
			if err := validateConditionNode(&node.Conditions[i], i+1); err != nil {
				return fmt.Errorf("group %d, %v", index, err)
			}
		}
		return nil
	} else {
		return fmt.Errorf("condition %d has invalid type: %s (must be 'condition' or 'group')", index, nodeType)
	}
}

// Helper: Check if a condition node matches the current webhook
// Supports both simple conditions and one-level groups
func conditionNodeMatches(node *ConditionNode, currentPath string, state map[string]bool, nodeKey string) (matched bool, groupComplete bool) {
	nodeType := node.Type
	if nodeType == "" {
		nodeType = "condition"
	}

	if nodeType == "condition" {
		// Simple condition - direct match
		return node.Webhook == currentPath, node.Webhook == currentPath
	}

	// It's a group - check if any child conditions match
	if nodeType == "group" {
		matched = false
		groupKey := nodeKey

		// Check each child condition in the group
		for i, childNode := range node.Conditions {
			childKey := fmt.Sprintf("%s_child_%d", groupKey, i)

			// Simple child (groups can't be nested in one-level approach)
			if childNode.Webhook == currentPath {
				matched = true
				mu.Lock()
				state[childKey] = true
				mu.Unlock()
				log.Printf("✅ [GROUP] Condition met in group: %s", getNodeDescription(&childNode))
				break
			}
		}

		// Check if the group is complete based on its combination mode
		switch node.Combination {
		case "any":
			// Any child complete = group complete
			for i := range node.Conditions {
				childKey := fmt.Sprintf("%s_child_%d", groupKey, i)
				if state[childKey] {
					groupComplete = true
					break
				}
			}
		case "all", "sequential":
			// All children must be complete (for sequential, order is enforced by parent)
			groupComplete = true
			for i := range node.Conditions {
				childKey := fmt.Sprintf("%s_child_%d", groupKey, i)
				if !state[childKey] {
					groupComplete = false
					break
				}
			}
		}

		return matched, groupComplete
	}

	return false, false
}

// isConditionCurrentlyMet checks if a condition reflects the current state (not just if it was triggered)
func isConditionCurrentlyMet(webhookPath string, state *PositionState) bool {
	switch webhookPath {
	// RSI conditions
	case "/webhook/rsi/cross-up-50":
		return state.RSIAbove50
	case "/webhook/rsi/cross-down-50":
		return state.RSIBelow50
	case "/webhook/rsi/above-50":
		return state.RSIAbove50
	case "/webhook/rsi/below-50":
		return state.RSIBelow50
	case "/webhook/rsi/cross-up-30":
		return state.RSICrossedUp30
	case "/webhook/rsi/cross-down-30":
		return state.RSICrossedDown30
	case "/webhook/rsi/cross-up-40":
		return state.RSICrossedUp40
	case "/webhook/rsi/cross-down-40":
		return state.RSICrossedDown40
	case "/webhook/rsi/cross-up-60":
		return state.RSICrossedUp60
	case "/webhook/rsi/cross-down-60":
		return state.RSICrossedDown60
	case "/webhook/rsi/cross-up-70":
		return state.RSICrossedUp70
	case "/webhook/rsi/cross-down-70":
		return state.RSICrossedDown70
	case "/webhook/rsi/cross-up-75":
		return state.RSICrossedUp75
	case "/webhook/rsi/cross-down-75":
		return state.RSICrossedDown75
	case "/webhook/rsi/cross-up-25", "/webhook/rsi/cross-up-oversell-25":
		return state.RSICrossedUp25
	case "/webhook/rsi/cross-down-25":
		return state.RSICrossedDown25
	case "/webhook/rsi/cross-down-overbuy-75":
		return state.RSICrossedDown75
	case "/webhook/rsi/cross-down-overbuy":
		return state.RSICrossedDown70 // Generic overbuy uses 70 level
	case "/webhook/rsi/cross-up-oversell":
		return state.RSICrossedUp30 // Generic oversell uses 30 level

	// EMA price position conditions
	case "/webhook/ema/price-above-ema50":
		return state.PriceAboveEMA50
	case "/webhook/ema/price-below-ema50":
		return state.PriceBelowEMA50
	case "/webhook/ema/price-above-ema200":
		return state.PriceAboveEMA200
	case "/webhook/ema/price-below-ema200":
		return state.PriceBelowEMA200
	case "/webhook/ema/price-above-ema20":
		return state.PriceAboveEMA20
	case "/webhook/ema/price-below-ema20":
		return state.PriceBelowEMA20
	case "/webhook/ema/price-above-ema9":
		return state.PriceAboveEMA9
	case "/webhook/ema/price-below-ema9":
		return state.PriceBelowEMA9

	// MA Ribbon conditions (generic MA#1-4)
	case "/webhook/ma/price-above-ma4":
		return state.PriceAboveEMA200 // Reuse EMA200 state for MA#4
	case "/webhook/ma/price-below-ma4":
		return state.PriceBelowEMA200 // Reuse EMA200 state for MA#4
	case "/webhook/ma/price-above-ma2":
		return state.PriceAboveEMA20 // Reuse EMA20 state for MA#2
	case "/webhook/ma/price-below-ma2":
		return state.PriceBelowEMA20 // Reuse EMA20 state for MA#2
	case "/webhook/ma/price-cross-up-ma2":
		return state.PriceAboveEMA20 // Crossing up means now above
	case "/webhook/ma/price-cross-down-ma2":
		return state.PriceBelowEMA20 // Crossing down means now below
	case "/webhook/ma/ma1-cross-up-ma2":
		return state.EMA9CrossedUpEMA21 // Reuse EMA 9/21 state for MA#1/MA#2
	case "/webhook/ma/ma1-cross-down-ma2":
		return state.EMA9CrossedDownEMA21 // Reuse EMA 9/21 state for MA#1/MA#2
	case "/webhook/ma/ma1-above-ma2":
		return state.EMA9AboveEMA21
	case "/webhook/ma/ma1-below-ma2":
		return state.EMA9BelowEMA21
	case "/webhook/ma/ma2-above-ma3":
		return state.MA2AboveMA3
	case "/webhook/ma/ma2-below-ma3":
		return state.MA2BelowMA3

	// EMA crossover conditions
	case "/webhook/ema/9-cross-up-21":
		return state.EMA9CrossedUpEMA21
	case "/webhook/ema/9-cross-down-21":
		return state.EMA9CrossedDownEMA21
	case "/webhook/ema/9-above-21":
		return state.EMA9CrossedUpEMA21
	case "/webhook/ema/9-below-21":
		return state.EMA9CrossedDownEMA21
	case "/webhook/ema/price-cross-down-50":
		return state.PriceBelowEMA50 // Crossing down means now below
	case "/webhook/ema/price-cross-up-50":
		return state.PriceAboveEMA50 // Crossing up means now above

	// MACD conditions
	case "/webhook/macd/cross-up":
		return state.MACDCrossedUp
	case "/webhook/macd/cross-down":
		return state.MACDCrossedDown
	case "/webhook/macd/histogram-cross-up-0":
		return state.MACDHistAboveZero
	case "/webhook/macd/histogram-cross-down-0":
		return state.MACDHistBelowZero
	case "/webhook/macd/histogram-above-0":
		return state.MACDHistAboveZero
	case "/webhook/macd/histogram-below-0":
		return state.MACDHistBelowZero

	// Stochastic conditions
	case "/webhook/stochastic/oversold":
		return state.StochInOversold
	case "/webhook/stochastic/overbought":
		return state.StochInOverbought

	// Stochastic RSI conditions
	case "/webhook/stochastic-rsi/cross-up-20":
		return state.StochRSICrossedUp20
	case "/webhook/stochastic-rsi/cross-down-20":
		return state.StochRSICrossedDown20
	case "/webhook/stochastic-rsi/cross-up-50":
		return state.StochRSICrossedUp50
	case "/webhook/stochastic-rsi/cross-down-50":
		return state.StochRSICrossedDown50
	case "/webhook/stochastic-rsi/cross-up-80":
		return state.StochRSICrossedUp80
	case "/webhook/stochastic-rsi/cross-down-80":
		return state.StochRSICrossedDown80
	case "/webhook/stochastic-rsi/oversold":
		return state.StochInOversold
	case "/webhook/stochastic-rsi/overbought":
		return state.StochInOverbought

	// ATR conditions
	case "/webhook/atr/above-average":
		return state.ATRAboveAverage
	case "/webhook/atr/below-average":
		return state.ATRBelowAverage

	// SMC (Smart Money Concept) structure conditions
	case "/webhook/smc/low-low":
		return state.SMCLowLow
	case "/webhook/smc/high-low":
		return state.SMCHighLow
	case "/webhook/smc/low-high":
		return state.SMCLowHigh
	case "/webhook/smc/high-high":
		return state.SMCHighHigh

	default:
		// For unknown webhooks, fall back to checking if condition was completed
		// This maintains backward compatibility
		return false
	}
}

// Check if all entry conditions are met for opening a position
func shouldOpenPosition(symbol string, isLong bool, r *http.Request) bool {
	state := getPositionState(symbol)

	// Determine prefix for condition keys to avoid conflicts between LONG and SHORT
	var conditionPrefix string

	// Get the appropriate entry conditions based on strategy format
	var entryConditions *EntryConditions
	if activeStrategy.Entry != nil {
		// Unified format - same entry for both LONG and SHORT
		entryConditions = activeStrategy.Entry
		conditionPrefix = "" // No prefix needed for unified
	} else if isLong && activeStrategy.Long != nil {
		// Separate format - use LONG entry
		entryConditions = &activeStrategy.Long.Entry
		conditionPrefix = "long_" // Prefix to distinguish from SHORT
	} else if !isLong && activeStrategy.Short != nil {
		// Separate format - use SHORT entry
		entryConditions = &activeStrategy.Short.Entry
		conditionPrefix = "short_" // Prefix to distinguish from LONG
	} else {
		log.Printf("⚠️  [STRATEGY] No entry conditions found for %s position", map[bool]string{true: "LONG", false: "SHORT"}[isLong])
		return false
	}

	currentPath := ""
	if r != nil {
		currentPath = r.URL.Path
	}

	switch entryConditions.Combination {
	case "sequential":
		// Sequential: conditions/groups must be completed IN EXACT ORDER
		// Find the next expected condition (first incomplete condition)
		nextConditionIndex := -1
		for i := 0; i < len(entryConditions.Conditions); i++ {
			key := fmt.Sprintf("%scondition_%d", conditionPrefix, i)
			if !state.EntryConditionsCompleted[key] {
				nextConditionIndex = i
				break
			}
		}

		// If all conditions already completed, shouldn't happen but handle gracefully
		if nextConditionIndex == -1 {
			log.Printf("⚠️  [STRATEGY] All entry conditions already completed")
			return false
		}

		nextNode := &entryConditions.Conditions[nextConditionIndex]
		conditionKey := fmt.Sprintf("%scondition_%d", conditionPrefix, nextConditionIndex)

		// Check if this node (condition or group) matches
		matched, complete := conditionNodeMatches(nextNode, currentPath, state.EntryConditionsCompleted, conditionKey)

		if matched && complete {
			mu.Lock()
			state.EntryConditionsCompleted[conditionKey] = true
			mu.Unlock()

			log.Printf("✅ [STRATEGY] Entry condition %d/%d completed IN ORDER: %s",
				nextConditionIndex+1, len(entryConditions.Conditions), getNodeDescription(nextNode))

			// Check if ALL conditions are now completed
			allComplete := true
			for i := 0; i < len(entryConditions.Conditions); i++ {
				key := fmt.Sprintf("%scondition_%d", conditionPrefix, i)
				if !state.EntryConditionsCompleted[key] {
					allComplete = false
					log.Printf("⏳ [STRATEGY] Waiting for condition %d/%d: %s",
						i+1, len(entryConditions.Conditions), getNodeDescription(&entryConditions.Conditions[i]))
					break
				}
			}

			if allComplete {
				log.Printf("🎯 [STRATEGY] All entry conditions completed IN ORDER!")
				// Reset conditions for next trade
				mu.Lock()
				state.EntryConditionsCompleted = make(map[string]bool)
				mu.Unlock()
				return true
			}
		} else if matched && !complete {
			// Matched a child in a group, but group not complete yet
			log.Printf("⏳ [GROUP] Partial match in group - waiting for more conditions")
		} else {
			// Webhook fired but it's not the next expected condition
			comment := getNodeDescription(nextNode)
			log.Printf("⚠️  [STRATEGY] Received %s but expecting condition %d/%d: %s (conditions must be completed IN ORDER)",
				currentPath, nextConditionIndex+1, len(entryConditions.Conditions), comment)
		}
		return false

	case "all":
		// All conditions/groups must be met (order doesn't matter)
		// Check each condition node
		for i, node := range entryConditions.Conditions {
			conditionKey := fmt.Sprintf("%scondition_%d", conditionPrefix, i)

			// When called from auto re-entry (r == nil), check actual state instead of tracking map
			if currentPath == "" && node.Type == "condition" && node.Webhook != "" {
				if isConditionCurrentlyMet(node.Webhook, state) {
					mu.Lock()
					state.EntryConditionsCompleted[conditionKey] = true
					mu.Unlock()
				}
			} else {
				matched, complete := conditionNodeMatches(&node, currentPath, state.EntryConditionsCompleted, conditionKey)

				if matched && complete {
					mu.Lock()
					state.EntryConditionsCompleted[conditionKey] = true
					mu.Unlock()
					log.Printf("✅ [STRATEGY] Entry condition met: %s", getNodeDescription(&node))
				}
			}
		}

		// Check if all conditions are currently met (check actual state, not just tracking map)
		allComplete := true
		for i := 0; i < len(entryConditions.Conditions); i++ {
			condition := &entryConditions.Conditions[i]

			// For simple conditions with webhooks, check if they're currently met
			if condition.Type == "condition" && condition.Webhook != "" {
				if !isConditionCurrentlyMet(condition.Webhook, state) {
					allComplete = false
					break
				}
			} else {
				// For groups or other types, fall back to tracking map
				key := fmt.Sprintf("%scondition_%d", conditionPrefix, i)
				if !state.EntryConditionsCompleted[key] {
					allComplete = false
					break
				}
			}
		}

		if allComplete {
			log.Printf("🎯 [STRATEGY] All entry conditions met!")
			// Reset conditions for next trade
			mu.Lock()
			state.EntryConditionsCompleted = make(map[string]bool)
			mu.Unlock()
			return true
		}
		return false

	case "any":
		// Any condition/group triggers entry
		for i, node := range entryConditions.Conditions {
			conditionKey := fmt.Sprintf("%scondition_%d", conditionPrefix, i)

			matched, complete := conditionNodeMatches(&node, currentPath, state.EntryConditionsCompleted, conditionKey)

			if matched && complete {
				log.Printf("🎯 [STRATEGY] Entry condition met: %s", getNodeDescription(&node))
				return true
			}
		}
		return false
	}

	return false
}

// Check if any exit condition is met
func shouldExitPosition(symbol string, isLong bool, r *http.Request) (bool, string) {
	state := getPositionState(symbol)

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

	// Check exit based on combination mode
	currentPath := r.URL.Path

	switch exitConditions.Combination {
	case "sequential":
		// Sequential: conditions/groups must be completed IN EXACT ORDER
		// Find the next expected condition (first incomplete condition)
		nextConditionIndex := -1
		for i := 0; i < len(exitConditions.Conditions); i++ {
			key := fmt.Sprintf("condition_%d", i)
			if !state.ExitConditionsCompleted[key] {
				nextConditionIndex = i
				break
			}
		}

		// If all conditions already completed, shouldn't happen but handle gracefully
		if nextConditionIndex == -1 {
			log.Printf("⚠️  [EXIT] All exit conditions already completed")
			return false, ""
		}

		nextNode := &exitConditions.Conditions[nextConditionIndex]
		conditionKey := fmt.Sprintf("condition_%d", nextConditionIndex)

		// Check if this node (condition or group) matches
		matched, complete := conditionNodeMatches(nextNode, currentPath, state.ExitConditionsCompleted, conditionKey)

		if matched && complete {
			mu.Lock()
			state.ExitConditionsCompleted[conditionKey] = true
			mu.Unlock()

			log.Printf("✅ [EXIT] Exit condition %d/%d completed IN ORDER: %s",
				nextConditionIndex+1, len(exitConditions.Conditions), getNodeDescription(nextNode))

			// Check if ALL conditions are now completed
			allCompleted := true
			for i := 0; i < len(exitConditions.Conditions); i++ {
				key := fmt.Sprintf("condition_%d", i)
				if !state.ExitConditionsCompleted[key] {
					allCompleted = false
					log.Printf("⏳ [EXIT] Waiting for condition %d/%d: %s",
						i+1, len(exitConditions.Conditions), getNodeDescription(&exitConditions.Conditions[i]))
					break
				}
			}

			if allCompleted {
				reason := "All exit conditions completed IN ORDER"
				log.Printf("🎯 [EXIT] %s", reason)
				// Reset exit conditions for next position
				mu.Lock()
				state.ExitConditionsCompleted = make(map[string]bool)
				mu.Unlock()
				return true, reason
			}
		} else if matched && !complete {
			// Matched a child in a group, but group not complete yet
			log.Printf("⏳ [GROUP] Partial match in exit group - waiting for more conditions")
		} else {
			// Webhook fired but it's not the next expected condition
			comment := getNodeDescription(nextNode)
			log.Printf("⚠️  [EXIT] Received %s but expecting condition %d/%d: %s (conditions must be completed IN ORDER)",
				currentPath, nextConditionIndex+1, len(exitConditions.Conditions), comment)
		}
		return false, ""

	case "all":
		// All conditions/groups must be met (order doesn't matter)
		// Check each condition node
		for i, node := range exitConditions.Conditions {
			conditionKey := fmt.Sprintf("condition_%d", i)

			matched, complete := conditionNodeMatches(&node, currentPath, state.ExitConditionsCompleted, conditionKey)

			if matched && complete {
				mu.Lock()
				state.ExitConditionsCompleted[conditionKey] = true
				mu.Unlock()
				log.Printf("✅ [EXIT] Exit condition met: %s", getNodeDescription(&node))
			}
		}

		// Check if all are completed
		allCompleted := true
		for i := 0; i < len(exitConditions.Conditions); i++ {
			key := fmt.Sprintf("condition_%d", i)
			if !state.ExitConditionsCompleted[key] {
				allCompleted = false
				break
			}
		}

		if allCompleted {
			reason := "All exit conditions met"
			log.Printf("🎯 [EXIT] %s", reason)
			// Reset exit conditions for next position
			mu.Lock()
			state.ExitConditionsCompleted = make(map[string]bool)
			mu.Unlock()
			return true, reason
		}
		return false, ""

	case "any":
		// Any condition/group triggers exit
		for _, node := range exitConditions.Conditions {
			conditionKey := fmt.Sprintf("condition_%d", 0) // Temp key for any mode

			matched, complete := conditionNodeMatches(&node, currentPath, state.ExitConditionsCompleted, conditionKey)

			if matched && complete {
				reason := node.Comment
				if reason == "" {
					reason = fmt.Sprintf("exit condition: %s", currentPath)
				}
				log.Printf("🎯 [EXIT] %s", reason)
				return true, reason
			}
		}
		return false, ""
	}

	return false, ""
}

// Helper: Clear entry condition for a specific webhook when its state becomes false
func clearEntryConditionForWebhook(symbol string, webhookPath string) {
	state := getPositionState(symbol)

	// Don't clear if position is already open
	if state.PositionOpen {
		return
	}

	mu.Lock()
	defer mu.Unlock()

	// Helper to clear from a specific entry conditions structure
	clearFromConditions := func(entryConditions *EntryConditions, prefix string) {
		if entryConditions == nil {
			return
		}

		for i, condition := range entryConditions.Conditions {
			if condition.Type == "condition" && condition.Webhook == webhookPath {
				conditionKey := fmt.Sprintf("%scondition_%d", prefix, i)
				if state.EntryConditionsCompleted[conditionKey] {
					delete(state.EntryConditionsCompleted, conditionKey)
					log.Printf("🧹 [STATE] Cleared stale entry condition: %s (state now false)", webhookPath)
				}
				break
			}
		}
	}

	// Check unified format (same entry/exit for both long and short)
	if activeStrategy.Entry != nil {
		clearFromConditions(activeStrategy.Entry, "")
	}

	// Check separate format (long and short have their own entry/exit)
	// Need to clear from BOTH since we don't know which direction is being attempted
	if activeStrategy.Long != nil {
		clearFromConditions(&activeStrategy.Long.Entry, "long_")
	}
	if activeStrategy.Short != nil {
		clearFromConditions(&activeStrategy.Short.Entry, "short_")
	}
}

// Helper: Clear exit condition for a specific webhook when its state becomes false
func clearExitConditionForWebhook(symbol string, webhookPath string) {
	state := getPositionState(symbol)

	// Only clear if position is open (exit conditions only matter when position exists)
	if !state.PositionOpen {
		return
	}

	mu.Lock()
	defer mu.Unlock()

	// Helper to clear from a specific exit conditions structure
	clearFromConditions := func(exitConditions *ExitConditions, prefix string) {
		if exitConditions == nil {
			return
		}

		for i, condition := range exitConditions.Conditions {
			if condition.Type == "condition" && condition.Webhook == webhookPath {
				conditionKey := fmt.Sprintf("%scondition_%d", prefix, i)
				if state.ExitConditionsCompleted[conditionKey] {
					delete(state.ExitConditionsCompleted, conditionKey)
					log.Printf("🧹 [STATE] Cleared stale exit condition: %s (state now false)", webhookPath)
				}
				break
			}
		}
	}

	// Check unified format (same entry/exit for both long and short)
	if activeStrategy.Exit != nil {
		clearFromConditions(activeStrategy.Exit, "")
	}

	// Check separate format (long and short have their own entry/exit)
	// Clear from the appropriate direction based on current position
	if state.Position == "long" && activeStrategy.Long != nil {
		clearFromConditions(&activeStrategy.Long.Exit, "long_")
	}
	if state.Position == "short" && activeStrategy.Short != nil {
		clearFromConditions(&activeStrategy.Short.Exit, "short_")
	}
}

// Helper: Check if a webhook path is used in the active strategy
func isWebhookUsedInStrategy(webhookPath string) bool {
	// Check if webhook is used in entry or exit conditions
	checkConditions := func(conditions []ConditionNode) bool {
		for _, node := range conditions {
			if node.Type == "group" {
				// Check children in group
				for _, child := range node.Conditions {
					if child.Webhook == webhookPath {
						return true
					}
				}
			} else if node.Webhook == webhookPath {
				return true
			}
		}
		return false
	}

	// Check unified format (same entry/exit for both long and short)
	if activeStrategy.Entry != nil && checkConditions(activeStrategy.Entry.Conditions) {
		return true
	}
	if activeStrategy.Exit != nil && checkConditions(activeStrategy.Exit.Conditions) {
		return true
	}

	// Check separate format (long and short have their own entry/exit)
	if activeStrategy.Long != nil {
		if checkConditions(activeStrategy.Long.Entry.Conditions) {
			return true
		}
		if checkConditions(activeStrategy.Long.Exit.Conditions) {
			return true
		}
	}
	if activeStrategy.Short != nil {
		if checkConditions(activeStrategy.Short.Entry.Conditions) {
			return true
		}
		if checkConditions(activeStrategy.Short.Exit.Conditions) {
			return true
		}
	}

	return false
}

// ============================================================================
// RSI EVENT HANDLERS
// ============================================================================

// POST /webhook/rsi/crossed-up
// ============================================================================
// MACD EVENT HANDLERS
// ============================================================================

// POST /webhook/macd/cross-up
func handleMACDCrossUp(w http.ResponseWriter, r *http.Request) {
	// Check if this webhook is used in the active strategy
	if !isWebhookUsedInStrategy("/webhook/macd/cross-up") {
		log.Printf("⏭️  [WEBHOOK] MACD Cross Up not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

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
	if !validateSymbol(w, symbol) {
		return
	}
	updateLatestPrice(symbol, event.Close)
	state := getPositionState(symbol)

	// Track if this is an opposite direction condition
	trackOppositeDirection(symbol, "/webhook/macd/cross-up")

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
			state = getPositionState(symbol)
		}
		// Don't try to open a position if we're still in a SHORT position
		respondSuccess(w, "MACD cross up condition set")
		return
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
	// Check if this webhook is used in the active strategy
	if !isWebhookUsedInStrategy("/webhook/macd/cross-down") {
		log.Printf("⏭️  [WEBHOOK] MACD Cross Down not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

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
	if !validateSymbol(w, symbol) {
		return
	}
	updateLatestPrice(symbol, event.Close)
	state := getPositionState(symbol)

	// Track if this is an opposite direction condition
	trackOppositeDirection(symbol, "/webhook/macd/cross-down")

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
			state = getPositionState(symbol)
		}
		// Don't try to open a position if we're still in a LONG position
		respondSuccess(w, "MACD cross down condition set")
		return
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
// STOCHASTIC INDICATOR HANDLERS
// ============================================================================

// POST /webhook/stochastic/oversold
func handleStochasticOversold(w http.ResponseWriter, r *http.Request) {
	// Check if this webhook is used in the active strategy
	if !isWebhookUsedInStrategy("/webhook/stochastic/oversold") {
		log.Printf("⏭️  [WEBHOOK] Stochastic Oversold not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received Stochastic Oversold event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON in Stochastic Oversold: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	eventJSON, _ := json.MarshalIndent(event, "", "  ")
	log.Printf("📥 [REQUEST] %s", string(eventJSON))

	symbol := normalizeSymbol(event.Ticker)
	if !validateSymbol(w, symbol) {
		return
	}
	updateLatestPrice(symbol, event.Close)
	state := getPositionState(symbol)

	log.Printf("📊 Stochastic K&D entered OVERSOLD for %s", symbol)

	mu.Lock()
	state.StochInOversold = true
	state.StochInOverbought = false // Reset opposite condition
	mu.Unlock()

	// Clear entry condition that depended on StochInOverbought being true
	clearEntryConditionForWebhook(symbol, "/webhook/stochastic/overbought")

	// Check if we should exit SHORT position
	if state.PositionOpen && state.Position == "short" {
		shouldExit, reason := shouldExitPosition(symbol, false, r)
		if shouldExit {
			log.Printf("⚠️  [EXIT] %s → closing SHORT position", reason)
			closePosition(symbol)
			state = getPositionState(symbol)
		}
		// Don't try to open a position if we're still in a SHORT position
		respondSuccess(w, "Stochastic oversold condition set")
		return
	}

	// Check if we should open LONG position
	if shouldOpenPosition(symbol, true, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening LONG position")
		openLongPosition(symbol, event.Close)
		respondSuccess(w, "Stochastic oversold + strategy → LONG opened")
		return
	}

	respondSuccess(w, "Stochastic oversold condition set")
}

// POST /webhook/stochastic/overbought
func handleStochasticOverbought(w http.ResponseWriter, r *http.Request) {
	// Check if this webhook is used in the active strategy
	if !isWebhookUsedInStrategy("/webhook/stochastic/overbought") {
		log.Printf("⏭️  [WEBHOOK] Stochastic Overbought not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received Stochastic Overbought event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON in Stochastic Overbought: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	eventJSON, _ := json.MarshalIndent(event, "", "  ")
	log.Printf("📥 [REQUEST] %s", string(eventJSON))

	symbol := normalizeSymbol(event.Ticker)
	if !validateSymbol(w, symbol) {
		return
	}
	updateLatestPrice(symbol, event.Close)
	state := getPositionState(symbol)

	log.Printf("📊 Stochastic K&D entered OVERBOUGHT for %s", symbol)

	mu.Lock()
	state.StochInOverbought = true
	state.StochInOversold = false // Reset opposite condition
	mu.Unlock()

	// Clear entry condition that depended on StochInOversold being true
	clearEntryConditionForWebhook(symbol, "/webhook/stochastic/oversold")

	// Check if we should exit LONG position
	if state.PositionOpen && state.Position == "long" {
		shouldExit, reason := shouldExitPosition(symbol, true, r)
		if shouldExit {
			log.Printf("⚠️  [EXIT] %s → closing LONG position", reason)
			closePosition(symbol)
			state = getPositionState(symbol)
		}
		// Don't try to open a position if we're still in a LONG position
		respondSuccess(w, "Stochastic overbought condition set")
		return
	}

	// Check if we should open SHORT position
	if shouldOpenPosition(symbol, false, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening SHORT position")
		openShortPosition(symbol, event.Close)
		respondSuccess(w, "Stochastic overbought + strategy → SHORT opened")
		return
	}

	respondSuccess(w, "Stochastic overbought condition set")
}

// ============================================================================
// STOCHASTIC RSI INDICATOR HANDLERS
// ============================================================================

// POST /webhook/stochastic-rsi/cross-up-20
func handleStochRSICrossUp20(w http.ResponseWriter, r *http.Request) {
	// Check if this webhook is used in the active strategy
	if !isWebhookUsedInStrategy("/webhook/stochastic-rsi/cross-up-20") {
		log.Printf("⏭️  [WEBHOOK] Stochastic RSI Cross Up 20 not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received Stochastic RSI Cross Up 20 event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	eventJSON, _ := json.MarshalIndent(event, "", "  ")
	log.Printf("📥 [REQUEST] %s", string(eventJSON))

	symbol := normalizeSymbol(event.Ticker)
	if !validateSymbol(w, symbol) {
		return
	}
	updateLatestPrice(symbol, event.Close)
	state := getPositionState(symbol)

	log.Printf("📊 Stochastic RSI crossed UP above 20 for %s", symbol)

	mu.Lock()
	state.StochRSICrossedUp20 = true
	state.StochRSICrossedDown20 = false // Reset opposite direction in same zone
	state.StochRSICrossedUp80 = false   // Clear 80 zone - now moving toward overbought from oversold
	state.StochRSICrossedDown80 = false // Clear 80 zone - now moving toward overbought from oversold
	mu.Unlock()

	// Check if we should exit SHORT position
	if state.PositionOpen && state.Position == "short" {
		shouldExit, reason := shouldExitPosition(symbol, false, r)
		if shouldExit {
			log.Printf("⚠️  [EXIT] %s → closing SHORT position", reason)
			closePosition(symbol)
			respondSuccess(w, fmt.Sprintf("StochRSI cross up 20: %s → closed SHORT", reason))
			return
		}
		// Don't try to open a position if we're still in a SHORT position
		respondSuccess(w, "Stochastic RSI crossed up 20 condition set")
		return
	}

	// Check if we should open LONG position
	if shouldOpenPosition(symbol, true, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening LONG position")
		openLongPosition(symbol, event.Close)
		respondSuccess(w, "StochRSI cross up 20 + strategy → LONG opened")
		return
	}

	respondSuccess(w, "Stochastic RSI crossed up 20 condition set")
}

// POST /webhook/stochastic-rsi/cross-down-20
func handleStochRSICrossDown20(w http.ResponseWriter, r *http.Request) {
	// Check if this webhook is used in the active strategy
	if !isWebhookUsedInStrategy("/webhook/stochastic-rsi/cross-down-20") {
		log.Printf("⏭️  [WEBHOOK] Stochastic RSI Cross Down 20 not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received Stochastic RSI Cross Down 20 event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	eventJSON, _ := json.MarshalIndent(event, "", "  ")
	log.Printf("📥 [REQUEST] %s", string(eventJSON))

	symbol := normalizeSymbol(event.Ticker)
	if !validateSymbol(w, symbol) {
		return
	}
	updateLatestPrice(symbol, event.Close)
	state := getPositionState(symbol)

	log.Printf("📊 Stochastic RSI crossed DOWN below 20 for %s", symbol)

	mu.Lock()
	state.StochRSICrossedDown20 = true
	state.StochRSICrossedUp20 = false // Reset opposite direction in same zone
	mu.Unlock()

	// Check if we should exit SHORT position (Stochastic RSI crossing back down below 20 invalidates the up-20 exit)
	if state.PositionOpen && state.Position == "short" {
		log.Printf("📊 [SHORT EXIT CHECK] Stochastic RSI crossed back down below 20 - up-20 exit condition no longer valid")
	}

	respondSuccess(w, "Stochastic RSI crossed down 20 condition set")
}

// POST /webhook/stochastic-rsi/cross-up-50
func handleStochRSICrossUp50(w http.ResponseWriter, r *http.Request) {
	// Check if this webhook is used in the active strategy
	if !isWebhookUsedInStrategy("/webhook/stochastic-rsi/cross-up-50") {
		log.Printf("⏭️  [WEBHOOK] Stochastic RSI Cross Up 50 not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received Stochastic RSI Cross Up 50 event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	eventJSON, _ := json.MarshalIndent(event, "", "  ")
	log.Printf("📥 [REQUEST] %s", string(eventJSON))

	symbol := normalizeSymbol(event.Ticker)
	if !validateSymbol(w, symbol) {
		return
	}
	updateLatestPrice(symbol, event.Close)
	state := getPositionState(symbol)

	log.Printf("📊 Stochastic RSI crossed UP above 50 for %s", symbol)

	mu.Lock()
	state.StochRSICrossedUp50 = true
	state.StochRSICrossedDown50 = false // Reset opposite direction
	state.StochInOversold = false       // Clear oversold state
	state.StochInOverbought = false     // Clear overbought state
	mu.Unlock()

	// Check if we should exit SHORT position
	if state.PositionOpen && state.Position == "short" {
		shouldExit, reason := shouldExitPosition(symbol, false, r)
		if shouldExit {
			log.Printf("⚠️  [EXIT] %s → closing SHORT position", reason)
			closePosition(symbol)
			respondSuccess(w, fmt.Sprintf("StochRSI cross up 50: %s → closed SHORT", reason))
			return
		}
		respondSuccess(w, "Stochastic RSI crossed up 50 condition set")
		return
	}

	// Check if we should open LONG position
	if shouldOpenPosition(symbol, true, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening LONG position")
		openLongPosition(symbol, event.Close)
		respondSuccess(w, "StochRSI cross up 50 + strategy → LONG opened")
		return
	}

	respondSuccess(w, "Stochastic RSI crossed up 50 condition set")
}

// POST /webhook/stochastic-rsi/cross-down-50
func handleStochRSICrossDown50(w http.ResponseWriter, r *http.Request) {
	// Check if this webhook is used in the active strategy
	if !isWebhookUsedInStrategy("/webhook/stochastic-rsi/cross-down-50") {
		log.Printf("⏭️  [WEBHOOK] Stochastic RSI Cross Down 50 not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received Stochastic RSI Cross Down 50 event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	eventJSON, _ := json.MarshalIndent(event, "", "  ")
	log.Printf("📥 [REQUEST] %s", string(eventJSON))

	symbol := normalizeSymbol(event.Ticker)
	if !validateSymbol(w, symbol) {
		return
	}
	updateLatestPrice(symbol, event.Close)
	state := getPositionState(symbol)

	log.Printf("📊 Stochastic RSI crossed DOWN below 50 for %s", symbol)

	mu.Lock()
	state.StochRSICrossedDown50 = true
	state.StochRSICrossedUp50 = false // Reset opposite direction
	state.StochInOversold = false     // Clear oversold state
	state.StochInOverbought = false   // Clear overbought state
	mu.Unlock()

	// Check if we should exit LONG position
	if state.PositionOpen && state.Position == "long" {
		shouldExit, reason := shouldExitPosition(symbol, true, r)
		if shouldExit {
			log.Printf("⚠️  [EXIT] %s → closing LONG position", reason)
			closePosition(symbol)
			respondSuccess(w, fmt.Sprintf("StochRSI cross down 50: %s → closed LONG", reason))
			return
		}
		respondSuccess(w, "Stochastic RSI crossed down 50 condition set")
		return
	}

	// Check if we should open SHORT position
	if shouldOpenPosition(symbol, false, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening SHORT position")
		openShortPosition(symbol, event.Close)
		respondSuccess(w, "StochRSI cross down 50 + strategy → SHORT opened")
		return
	}

	respondSuccess(w, "Stochastic RSI crossed down 50 condition set")
}

// POST /webhook/stochastic-rsi/cross-up-80
func handleStochRSICrossUp80(w http.ResponseWriter, r *http.Request) {
	// Check if this webhook is used in the active strategy
	if !isWebhookUsedInStrategy("/webhook/stochastic-rsi/cross-up-80") {
		log.Printf("⏭️  [WEBHOOK] Stochastic RSI Cross Up 80 not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received Stochastic RSI Cross Up 80 event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	eventJSON, _ := json.MarshalIndent(event, "", "  ")
	log.Printf("📥 [REQUEST] %s", string(eventJSON))

	symbol := normalizeSymbol(event.Ticker)
	if !validateSymbol(w, symbol) {
		return
	}
	updateLatestPrice(symbol, event.Close)
	state := getPositionState(symbol)

	log.Printf("📊 Stochastic RSI crossed UP above 80 for %s", symbol)

	mu.Lock()
	state.StochRSICrossedUp80 = true
	state.StochRSICrossedDown80 = false // Reset opposite direction in same zone
	mu.Unlock()

	// Check if we should exit LONG position (Stochastic RSI crossing back up above 80 invalidates the down-80 exit)
	if state.PositionOpen && state.Position == "long" {
		log.Printf("📊 [LONG EXIT CHECK] Stochastic RSI crossed back up above 80 - down-80 exit condition no longer valid")
	}

	respondSuccess(w, "Stochastic RSI crossed up 80 condition set")
}

// POST /webhook/stochastic-rsi/cross-down-80
func handleStochRSICrossDown80(w http.ResponseWriter, r *http.Request) {
	// Check if this webhook is used in the active strategy
	if !isWebhookUsedInStrategy("/webhook/stochastic-rsi/cross-down-80") {
		log.Printf("⏭️  [WEBHOOK] Stochastic RSI Cross Down 80 not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received Stochastic RSI Cross Down 80 event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	eventJSON, _ := json.MarshalIndent(event, "", "  ")
	log.Printf("📥 [REQUEST] %s", string(eventJSON))

	symbol := normalizeSymbol(event.Ticker)
	if !validateSymbol(w, symbol) {
		return
	}
	updateLatestPrice(symbol, event.Close)
	state := getPositionState(symbol)

	log.Printf("📊 Stochastic RSI crossed DOWN below 80 for %s", symbol)

	mu.Lock()
	state.StochRSICrossedDown80 = true
	state.StochRSICrossedUp80 = false   // Reset opposite direction in same zone
	state.StochRSICrossedUp20 = false   // Clear 20 zone - now moving toward oversold from overbought
	state.StochRSICrossedDown20 = false // Clear 20 zone - now moving toward oversold from overbought
	mu.Unlock()

	// Check if we should exit LONG position
	if state.PositionOpen && state.Position == "long" {
		shouldExit, reason := shouldExitPosition(symbol, true, r)
		if shouldExit {
			log.Printf("⚠️  [EXIT] %s → closing LONG position", reason)
			closePosition(symbol)
			respondSuccess(w, fmt.Sprintf("StochRSI cross down 80: %s → closed LONG", reason))
			return
		}
		// Don't try to open a position if we're still in a LONG position
		respondSuccess(w, "Stochastic RSI crossed down 80 condition set")
		return
	}

	// Check if we should open SHORT position
	if shouldOpenPosition(symbol, false, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening SHORT position")
		openShortPosition(symbol, event.Close)
		respondSuccess(w, "StochRSI cross down 80 + strategy → SHORT opened")
		return
	}

	respondSuccess(w, "Stochastic RSI crossed down 80 condition set")
}

// POST /webhook/stochastic-rsi/oversold
func handleStochRSIOversold(w http.ResponseWriter, r *http.Request) {
	if !isWebhookUsedInStrategy("/webhook/stochastic-rsi/oversold") {
		log.Printf("⏭️  [WEBHOOK] Stochastic RSI Oversold not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received Stochastic RSI Oversold event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	eventJSON, _ := json.MarshalIndent(event, "", "  ")
	log.Printf("📥 [REQUEST] %s", string(eventJSON))

	symbol := normalizeSymbol(event.Ticker)
	if !validateSymbol(w, symbol) {
		return
	}
	updateLatestPrice(symbol, event.Close)
	state := getPositionState(symbol)

	log.Printf("📊 Stochastic RSI entered OVERSOLD zone for %s", symbol)

	mu.Lock()
	state.StochInOversold = true
	state.StochInOverbought = false // Reset opposite condition
	mu.Unlock()

	// Check if we should exit SHORT position
	if state.PositionOpen && state.Position == "short" {
		shouldExit, reason := shouldExitPosition(symbol, false, r)
		if shouldExit {
			log.Printf("⚠️  [EXIT] %s → closing SHORT position", reason)
			closePosition(symbol)
			respondSuccess(w, fmt.Sprintf("StochRSI oversold: %s → closed SHORT", reason))
			return
		}
		respondSuccess(w, "Stochastic RSI oversold condition set")
		return
	}

	// Check if we should open LONG position
	if shouldOpenPosition(symbol, true, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening LONG position")
		openLongPosition(symbol, event.Close)
		respondSuccess(w, "StochRSI oversold + strategy → LONG opened")
		return
	}

	respondSuccess(w, "Stochastic RSI oversold condition set")
}

// POST /webhook/stochastic-rsi/overbought
func handleStochRSIOverbought(w http.ResponseWriter, r *http.Request) {
	if !isWebhookUsedInStrategy("/webhook/stochastic-rsi/overbought") {
		log.Printf("⏭️  [WEBHOOK] Stochastic RSI Overbought not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received Stochastic RSI Overbought event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	eventJSON, _ := json.MarshalIndent(event, "", "  ")
	log.Printf("📥 [REQUEST] %s", string(eventJSON))

	symbol := normalizeSymbol(event.Ticker)
	if !validateSymbol(w, symbol) {
		return
	}
	updateLatestPrice(symbol, event.Close)
	state := getPositionState(symbol)

	log.Printf("📊 Stochastic RSI entered OVERBOUGHT zone for %s", symbol)

	mu.Lock()
	state.StochInOverbought = true
	state.StochInOversold = false // Reset opposite condition
	mu.Unlock()

	// Check if we should exit LONG position
	if state.PositionOpen && state.Position == "long" {
		shouldExit, reason := shouldExitPosition(symbol, true, r)
		if shouldExit {
			log.Printf("⚠️  [EXIT] %s → closing LONG position", reason)
			closePosition(symbol)
			respondSuccess(w, fmt.Sprintf("StochRSI overbought: %s → closed LONG", reason))
			return
		}
		respondSuccess(w, "Stochastic RSI overbought condition set")
		return
	}

	// Check if we should open SHORT position
	if shouldOpenPosition(symbol, false, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening SHORT position")
		openShortPosition(symbol, event.Close)
		respondSuccess(w, "StochRSI overbought + strategy → SHORT opened")
		return
	}

	respondSuccess(w, "Stochastic RSI overbought condition set")
}

// ============================================================================
// RSI TREND INDICATOR HANDLERS
// ============================================================================

// POST /webhook/rsi/above-50
func handleRSIAbove50(w http.ResponseWriter, r *http.Request) {
	// Check if this webhook is used in the active strategy
	if !isWebhookUsedInStrategy("/webhook/rsi/above-50") {
		log.Printf("⏭️  [WEBHOOK] RSI Above 50 not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received RSI Above 50 event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	eventJSON, _ := json.MarshalIndent(event, "", "  ")
	log.Printf("📥 [REQUEST] %s", string(eventJSON))

	symbol := normalizeSymbol(event.Ticker)
	if !validateSymbol(w, symbol) {
		return
	}
	updateLatestPrice(symbol, event.Close)
	state := getPositionState(symbol)

	// Track if this is an opposite direction condition
	trackOppositeDirection(symbol, "/webhook/rsi/above-50")

	// Update latest price for P/L calculation
	updateLatestPrice(symbol, event.Close)

	// Store exchange information
	mu.Lock()
	state.Exchange = event.Exchange
	mu.Unlock()

	log.Printf("📊 RSI crossed ABOVE 50 (uptrend) for %s [%s]", symbol, event.Exchange)

	mu.Lock()
	state.RSIAbove50 = true
	state.RSIBelow50 = false
	mu.Unlock()

	// Check if we should open LONG position
	if shouldOpenPosition(symbol, true, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening LONG position")
		openLongPosition(symbol, event.Close)
		respondSuccess(w, "RSI above 50 + strategy → LONG opened")
		return
	}

	respondSuccess(w, "RSI above 50 condition set")
}

// POST /webhook/rsi/below-50
func handleRSIBelow50(w http.ResponseWriter, r *http.Request) {
	// Check if this webhook is used in the active strategy
	if !isWebhookUsedInStrategy("/webhook/rsi/below-50") {
		log.Printf("⏭️  [WEBHOOK] RSI Below 50 not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received RSI Below 50 event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	eventJSON, _ := json.MarshalIndent(event, "", "  ")
	log.Printf("📥 [REQUEST] %s", string(eventJSON))

	symbol := normalizeSymbol(event.Ticker)
	if !validateSymbol(w, symbol) {
		return
	}
	updateLatestPrice(symbol, event.Close)
	state := getPositionState(symbol)

	// Track if this is an opposite direction condition
	trackOppositeDirection(symbol, "/webhook/rsi/below-50")

	log.Printf("📊 RSI crossed BELOW 50 (downtrend) for %s", symbol)

	mu.Lock()
	state.RSIBelow50 = true
	state.RSIAbove50 = false
	mu.Unlock()

	// Check if we should open SHORT position
	if shouldOpenPosition(symbol, false, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening SHORT position")
		openShortPosition(symbol, event.Close)
		respondSuccess(w, "RSI below 50 + strategy → SHORT opened")
		return
	}

	respondSuccess(w, "RSI below 50 condition set")
}

// ============================================================================
// RSI CENTERLINE EXIT HANDLER
// ============================================================================

// POST /webhook/rsi/crossed-center
// ============================================================================
// RSI SPECIFIC LEVEL HANDLERS (25, 30, 40, 50, 60, 70, 75)
// ============================================================================

// POST /webhook/rsi/cross-up-oversell-25
func handleRSICrossUpOversell25(w http.ResponseWriter, r *http.Request) {
	// Check if this webhook is used in the active strategy
	if !isWebhookUsedInStrategy("/webhook/rsi/cross-up-oversell-25") {
		log.Printf("⏭️  [WEBHOOK] RSI Cross Up Oversell 25 not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received RSI Cross Up from Oversell 25 event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	eventJSON, _ := json.MarshalIndent(event, "", "  ")
	log.Printf("📥 [REQUEST] %s", string(eventJSON))

	symbol := normalizeSymbol(event.Ticker)
	if !validateSymbol(w, symbol) {
		return
	}
	updateLatestPrice(symbol, event.Close)
	log.Printf("🔄 [CONVERT] Normalized %s → %s", event.Ticker, symbol)
	state := getPositionState(symbol)

	log.Printf("📊 RSI crossed UP from oversold at 25 for %s", symbol)

	// Update RSI state
	mu.Lock()
	state.RSICrossedUp25 = true
	state.RSICrossedDown25 = false // Clear opposite state
	mu.Unlock()

	// Check if we should exit SHORT position
	if state.PositionOpen && state.Position == "short" {
		shouldExit, reason := shouldExitPosition(symbol, false, r)
		if shouldExit {
			log.Printf("⚠️  [EXIT] %s → closing SHORT position", reason)
			closePosition(symbol)
			respondSuccess(w, fmt.Sprintf("RSI cross up 25: %s → closed SHORT", reason))
			return
		}
		// Don't try to open a position if we're still in a SHORT position
		respondSuccess(w, "RSI crossed up from oversell 25")
		return
	}

	// Check if we should open LONG position
	if shouldOpenPosition(symbol, true, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening LONG position")
		openLongPosition(symbol, event.Close)
		respondSuccess(w, "RSI cross up 25 + strategy → LONG opened")
		return
	}

	respondSuccess(w, "RSI crossed up from oversell 25")
}

// POST /webhook/rsi/cross-oversell-30
func handleRSICrossOversell30(w http.ResponseWriter, r *http.Request) {
	// Check if this webhook is used in the active strategy
	if !isWebhookUsedInStrategy("/webhook/rsi/cross-oversell-30") {
		log.Printf("⏭️  [WEBHOOK] RSI Cross Oversell 30 not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received RSI Cross Oversell 30 event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	eventJSON, _ := json.MarshalIndent(event, "", "  ")
	log.Printf("📥 [REQUEST] %s", string(eventJSON))

	symbol := normalizeSymbol(event.Ticker)
	if !validateSymbol(w, symbol) {
		return
	}
	updateLatestPrice(symbol, event.Close)
	state := getPositionState(symbol)

	log.Printf("📊 RSI crossed oversold at 30 for %s", symbol)

	// Check if we should exit SHORT position
	if state.PositionOpen && state.Position == "short" {
		shouldExit, reason := shouldExitPosition(symbol, false, r)
		if shouldExit {
			log.Printf("⚠️  [EXIT] %s → closing SHORT position", reason)
			closePosition(symbol)
			state = getPositionState(symbol)
		}
		// Don't try to open a position if we're still in a SHORT position
		respondSuccess(w, "RSI crossed oversell 30")
		return
	}

	// Check if we should open LONG position
	if shouldOpenPosition(symbol, true, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening LONG position")
		openLongPosition(symbol, event.Close)
		respondSuccess(w, "RSI cross 30 + strategy → LONG opened")
		return
	}

	respondSuccess(w, "RSI crossed oversell 30")
}

// POST /webhook/rsi/cross-40
func handleRSICross40(w http.ResponseWriter, r *http.Request) {
	// Check if this webhook is used in the active strategy
	if !isWebhookUsedInStrategy("/webhook/rsi/cross-40") {
		log.Printf("⏭️  [WEBHOOK] RSI Cross 40 not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received RSI Cross 40 event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	eventJSON, _ := json.MarshalIndent(event, "", "  ")
	log.Printf("📥 [REQUEST] %s", string(eventJSON))

	symbol := normalizeSymbol(event.Ticker)
	if !validateSymbol(w, symbol) {
		return
	}
	updateLatestPrice(symbol, event.Close)
	state := getPositionState(symbol)

	log.Printf("📊 RSI crossed 40 for %s", symbol)

	// Check if we should exit SHORT position
	if state.PositionOpen && state.Position == "short" {
		shouldExit, reason := shouldExitPosition(symbol, false, r)
		if shouldExit {
			log.Printf("⚠️  [EXIT] %s → closing SHORT position", reason)
			closePosition(symbol)
			respondSuccess(w, fmt.Sprintf("RSI cross 40: %s → closed SHORT", reason))
			return
		}
	}

	respondSuccess(w, "RSI crossed 40")
}

// POST /webhook/rsi/cross-center-50
func handleRSICrossCenter50(w http.ResponseWriter, r *http.Request) {
	// Check if this webhook is used in the active strategy
	if !isWebhookUsedInStrategy("/webhook/rsi/cross-center-50") {
		log.Printf("⏭️  [WEBHOOK] RSI Cross Center 50 not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received RSI Cross Center 50 event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	eventJSON, _ := json.MarshalIndent(event, "", "  ")
	log.Printf("📥 [REQUEST] %s", string(eventJSON))

	symbol := normalizeSymbol(event.Ticker)
	if !validateSymbol(w, symbol) {
		return
	}
	updateLatestPrice(symbol, event.Close)
	state := getPositionState(symbol)

	log.Printf("📊 RSI MA crossed center at 50 for %s", symbol)

	// Check if we should exit position (for LONG or SHORT)
	if state.PositionOpen {
		isLong := state.Position == "long"
		shouldExit, reason := shouldExitPosition(symbol, isLong, r)
		if shouldExit {
			log.Printf("⚠️  [EXIT] %s → closing %s position", reason, strings.ToUpper(state.Position))
			closePosition(symbol)
			respondSuccess(w, fmt.Sprintf("RSI center 50: %s → closed %s", reason, strings.ToUpper(state.Position)))
			return
		}
	}

	respondSuccess(w, "RSI crossed center 50")
}

// POST /webhook/rsi/cross-60
func handleRSICross60(w http.ResponseWriter, r *http.Request) {
	// Check if this webhook is used in the active strategy
	if !isWebhookUsedInStrategy("/webhook/rsi/cross-60") {
		log.Printf("⏭️  [WEBHOOK] RSI Cross 60 not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received RSI Cross 60 event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	eventJSON, _ := json.MarshalIndent(event, "", "  ")
	log.Printf("📥 [REQUEST] %s", string(eventJSON))

	symbol := normalizeSymbol(event.Ticker)
	if !validateSymbol(w, symbol) {
		return
	}
	updateLatestPrice(symbol, event.Close)
	state := getPositionState(symbol)

	log.Printf("📊 RSI crossed 60 for %s", symbol)

	// Check if we should exit LONG position
	if state.PositionOpen && state.Position == "long" {
		shouldExit, reason := shouldExitPosition(symbol, true, r)
		if shouldExit {
			log.Printf("⚠️  [EXIT] %s → closing LONG position", reason)
			closePosition(symbol)
			respondSuccess(w, fmt.Sprintf("RSI cross 60: %s → closed LONG", reason))
			return
		}
	}

	respondSuccess(w, "RSI crossed 60")
}

// POST /webhook/rsi/cross-overbuy-70
func handleRSICrossOverbuy70(w http.ResponseWriter, r *http.Request) {
	// Check if this webhook is used in the active strategy
	if !isWebhookUsedInStrategy("/webhook/rsi/cross-overbuy-70") {
		log.Printf("⏭️  [WEBHOOK] RSI Cross Overbuy 70 not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received RSI Cross Overbought 70 event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	eventJSON, _ := json.MarshalIndent(event, "", "  ")
	log.Printf("📥 [REQUEST] %s", string(eventJSON))

	symbol := normalizeSymbol(event.Ticker)
	if !validateSymbol(w, symbol) {
		return
	}
	updateLatestPrice(symbol, event.Close)
	state := getPositionState(symbol)

	log.Printf("📊 RSI crossed overbought at 70 for %s", symbol)

	// Check if we should exit LONG position
	if state.PositionOpen && state.Position == "long" {
		shouldExit, reason := shouldExitPosition(symbol, true, r)
		if shouldExit {
			log.Printf("⚠️  [EXIT] %s → closing LONG position", reason)
			closePosition(symbol)
			state = getPositionState(symbol)
		}
		// Don't try to open a position if we're still in a LONG position
		respondSuccess(w, "RSI crossed overbought 70")
		return
	}

	// Check if we should open SHORT position
	if shouldOpenPosition(symbol, false, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening SHORT position")
		openShortPosition(symbol, event.Close)
		respondSuccess(w, "RSI cross 70 + strategy → SHORT opened")
		return
	}

	respondSuccess(w, "RSI crossed overbought 70")
}

// POST /webhook/rsi/cross-down-overbuy-75
func handleRSICrossDownOverbuy75(w http.ResponseWriter, r *http.Request) {
	// Check if this webhook is used in the active strategy
	if !isWebhookUsedInStrategy("/webhook/rsi/cross-down-overbuy-75") {
		log.Printf("⏭️  [WEBHOOK] RSI Cross Down Overbuy 75 not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received RSI Cross Down from Overbought 75 event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	eventJSON, _ := json.MarshalIndent(event, "", "  ")
	log.Printf("📥 [REQUEST] %s", string(eventJSON))

	symbol := normalizeSymbol(event.Ticker)
	if !validateSymbol(w, symbol) {
		return
	}
	updateLatestPrice(symbol, event.Close)
	state := getPositionState(symbol)

	log.Printf("📊 RSI crossed DOWN from overbought at 75 for %s", symbol)

	// Update RSI state
	mu.Lock()
	state.RSICrossedDown75 = true
	state.RSICrossedUp75 = false // Clear opposite state
	mu.Unlock()

	// Check if we should exit LONG position
	if state.PositionOpen && state.Position == "long" {
		shouldExit, reason := shouldExitPosition(symbol, true, r)
		if shouldExit {
			log.Printf("⚠️  [EXIT] %s → closing LONG position", reason)
			closePosition(symbol)
			state = getPositionState(symbol)
			respondSuccess(w, fmt.Sprintf("RSI cross down 75: %s → closed LONG", reason))
			return
		}
		// Don't try to open a position if we're still in a LONG position
		respondSuccess(w, "RSI crossed down from overbought 75")
		return
	}

	// Check if we should open SHORT position
	if shouldOpenPosition(symbol, false, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening SHORT position")
		openShortPosition(symbol, event.Close)
		respondSuccess(w, "RSI cross down 75 + strategy → SHORT opened")
		return
	}

	respondSuccess(w, "RSI crossed down from overbought 75")
}

// ============================================================================
// EMA (EXPONENTIAL MOVING AVERAGE) TREND HANDLERS
// ============================================================================

// POST /webhook/ema/price-above-ema20
func handlePriceAboveEMA20(w http.ResponseWriter, r *http.Request) {
	if !isWebhookUsedInStrategy("/webhook/ema/price-above-ema20") {
		log.Printf("⏭️  [WEBHOOK] Price Above EMA20 not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received Price Above EMA20 event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	symbol := normalizeSymbol(event.Ticker)
	if !validateSymbol(w, symbol) {
		return
	}
	updateLatestPrice(symbol, event.Close)
	state := getPositionState(symbol)

	log.Printf("📊 Price crossed ABOVE EMA 20 for %s", symbol)

	mu.Lock()
	state.PriceAboveEMA20 = true
	state.PriceBelowEMA20 = false
	mu.Unlock()

	if shouldOpenPosition(symbol, true, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening LONG position")
		openLongPosition(symbol, event.Close)
		respondSuccess(w, "Price above EMA20 + strategy → LONG opened")
		return
	}

	respondSuccess(w, "Price above EMA20 condition set")
}

// POST /webhook/ema/price-below-ema20
func handlePriceBelowEMA20(w http.ResponseWriter, r *http.Request) {
	if !isWebhookUsedInStrategy("/webhook/ema/price-below-ema20") {
		log.Printf("⏭️  [WEBHOOK] Price Below EMA20 not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received Price Below EMA20 event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	symbol := normalizeSymbol(event.Ticker)
	if !validateSymbol(w, symbol) {
		return
	}
	updateLatestPrice(symbol, event.Close)
	state := getPositionState(symbol)

	log.Printf("📊 Price crossed BELOW EMA 20 for %s", symbol)

	mu.Lock()
	state.PriceBelowEMA20 = true
	state.PriceAboveEMA20 = false
	mu.Unlock()

	// Check if we should exit LONG position
	if state.PositionOpen && state.Position == "long" {
		shouldExit, reason := shouldExitPosition(symbol, true, r)
		if shouldExit {
			log.Printf("⚠️  [EXIT] %s → closing LONG position", reason)
			closePosition(symbol)
			state = getPositionState(symbol)
		}
		// Don't try to open a position if we're still in a LONG position
		respondSuccess(w, "Price below EMA20 condition set")
		return
	}

	if shouldOpenPosition(symbol, false, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening SHORT position")
		openShortPosition(symbol, event.Close)
		respondSuccess(w, "Price below EMA20 + strategy → SHORT opened")
		return
	}

	respondSuccess(w, "Price below EMA20 condition set")
}

// POST /webhook/ema/price-above-ema50
func handlePriceAboveEMA50(w http.ResponseWriter, r *http.Request) {
	if !isWebhookUsedInStrategy("/webhook/ema/price-above-ema50") {
		log.Printf("⏭️  [WEBHOOK] Price Above EMA50 not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received Price Above EMA50 event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	symbol := normalizeSymbol(event.Ticker)
	if !validateSymbol(w, symbol) {
		return
	}
	updateLatestPrice(symbol, event.Close)
	state := getPositionState(symbol)

	log.Printf("📊 Price crossed ABOVE EMA 50 for %s", symbol)

	mu.Lock()
	state.PriceAboveEMA50 = true
	state.PriceBelowEMA50 = false
	mu.Unlock()

	if shouldOpenPosition(symbol, true, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening LONG position")
		openLongPosition(symbol, event.Close)
		respondSuccess(w, "Price above EMA50 + strategy → LONG opened")
		return
	}

	respondSuccess(w, "Price above EMA50 condition set")
}

// POST /webhook/ema/price-below-ema50
func handlePriceBelowEMA50(w http.ResponseWriter, r *http.Request) {
	if !isWebhookUsedInStrategy("/webhook/ema/price-below-ema50") {
		log.Printf("⏭️  [WEBHOOK] Price Below EMA50 not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received Price Below EMA50 event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	symbol := normalizeSymbol(event.Ticker)
	if !validateSymbol(w, symbol) {
		return
	}
	updateLatestPrice(symbol, event.Close)
	state := getPositionState(symbol)

	log.Printf("📊 Price crossed BELOW EMA 50 for %s", symbol)

	mu.Lock()
	state.PriceBelowEMA50 = true
	state.PriceAboveEMA50 = false
	mu.Unlock()

	if shouldOpenPosition(symbol, false, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening SHORT position")
		openShortPosition(symbol, event.Close)
		respondSuccess(w, "Price below EMA50 + strategy → SHORT opened")
		return
	}

	respondSuccess(w, "Price below EMA50 condition set")
}

// POST /webhook/ema/price-above-ema200
func handlePriceAboveEMA200(w http.ResponseWriter, r *http.Request) {
	if !isWebhookUsedInStrategy("/webhook/ema/price-above-ema200") {
		log.Printf("⏭️  [WEBHOOK] Price Above EMA200 not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received Price Above EMA200 event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	symbol := normalizeSymbol(event.Ticker)
	if !validateSymbol(w, symbol) {
		return
	}
	updateLatestPrice(symbol, event.Close)
	state := getPositionState(symbol)

	log.Printf("📊 Price crossed ABOVE EMA 200 for %s", symbol)

	mu.Lock()
	state.PriceAboveEMA200 = true
	state.PriceBelowEMA200 = false
	mu.Unlock()

	if shouldOpenPosition(symbol, true, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening LONG position")
		openLongPosition(symbol, event.Close)
		respondSuccess(w, "Price above EMA200 + strategy → LONG opened")
		return
	}

	respondSuccess(w, "Price above EMA200 condition set")
}

// POST /webhook/ema/price-below-ema200
func handlePriceBelowEMA200(w http.ResponseWriter, r *http.Request) {
	if !isWebhookUsedInStrategy("/webhook/ema/price-below-ema200") {
		log.Printf("⏭️  [WEBHOOK] Price Below EMA200 not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received Price Below EMA200 event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	symbol := normalizeSymbol(event.Ticker)
	if !validateSymbol(w, symbol) {
		return
	}
	updateLatestPrice(symbol, event.Close)
	state := getPositionState(symbol)

	log.Printf("📊 Price crossed BELOW EMA 200 for %s", symbol)

	mu.Lock()
	state.PriceBelowEMA200 = true
	state.PriceAboveEMA200 = false
	mu.Unlock()

	if shouldOpenPosition(symbol, false, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening SHORT position")
		openShortPosition(symbol, event.Close)
		respondSuccess(w, "Price below EMA200 + strategy → SHORT opened")
		return
	}

	respondSuccess(w, "Price below EMA200 condition set")
}

// ============================================================================
// MA RIBBON HANDLERS (Generic MA#1-4)
// ============================================================================

// POST /webhook/ma/price-above-ma4
func handlePriceAboveMA4(w http.ResponseWriter, r *http.Request) {
	if !isWebhookUsedInStrategy("/webhook/ma/price-above-ma4") {
		log.Printf("⏭️  [WEBHOOK] Price Above MA#4 not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received Price Above MA#4 event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	symbol := normalizeSymbol(event.Ticker)
	if !validateSymbol(w, symbol) {
		return
	}
	updateLatestPrice(symbol, event.Close)
	state := getPositionState(symbol)

	log.Printf("📊 Price crossed ABOVE MA#4 for %s", symbol)

	mu.Lock()
	state.PriceAboveEMA200 = true
	state.PriceBelowEMA200 = false
	mu.Unlock()

	if shouldOpenPosition(symbol, true, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening LONG position")
		openLongPosition(symbol, event.Close)
		respondSuccess(w, "Price above MA#4 + strategy → LONG opened")
		return
	}

	respondSuccess(w, "Price above MA#4 condition set")
}

// POST /webhook/ma/price-below-ma4
func handlePriceBelowMA4(w http.ResponseWriter, r *http.Request) {
	if !isWebhookUsedInStrategy("/webhook/ma/price-below-ma4") {
		log.Printf("⏭️  [WEBHOOK] Price Below MA#4 not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received Price Below MA#4 event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	symbol := normalizeSymbol(event.Ticker)
	if !validateSymbol(w, symbol) {
		return
	}
	updateLatestPrice(symbol, event.Close)
	state := getPositionState(symbol)

	log.Printf("📊 Price crossed BELOW MA#4 for %s", symbol)

	mu.Lock()
	state.PriceBelowEMA200 = true
	state.PriceAboveEMA200 = false
	mu.Unlock()

	if shouldOpenPosition(symbol, false, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening SHORT position")
		openShortPosition(symbol, event.Close)
		respondSuccess(w, "Price below MA#4 + strategy → SHORT opened")
		return
	}

	respondSuccess(w, "Price below MA#4 condition set")
}

// POST /webhook/ma/ma1-cross-up-ma2
func handleMA1CrossUpMA2(w http.ResponseWriter, r *http.Request) {
	if !isWebhookUsedInStrategy("/webhook/ma/ma1-cross-up-ma2") {
		log.Printf("⏭️  [WEBHOOK] MA#1 Cross Up MA#2 not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received MA#1 Cross Up MA#2 event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	symbol := normalizeSymbol(event.Ticker)
	if !validateSymbol(w, symbol) {
		return
	}
	updateLatestPrice(symbol, event.Close)
	state := getPositionState(symbol)

	// Update latest price for P/L calculation
	updateLatestPrice(symbol, event.Close)

	log.Printf("📊 MA#1 crossed UP through MA#2 for %s", symbol)

	// Update EMA cross state
	mu.Lock()
	state.EMA9CrossedUpEMA21 = true
	state.EMA9CrossedDownEMA21 = false // Clear opposite state
	mu.Unlock()

	if shouldOpenPosition(symbol, true, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening LONG position")
		openLongPosition(symbol, event.Close)
		respondSuccess(w, "MA#1 cross up MA#2 + strategy → LONG opened")
		return
	}

	// Check if we should exit SHORT position
	if state.PositionOpen && state.Position == "short" {
		shouldExit, reason := shouldExitPosition(symbol, false, r)
		if shouldExit {
			log.Printf("⚠️  [EXIT] %s → closing SHORT position", reason)
			closePosition(symbol)
			respondSuccess(w, fmt.Sprintf("MA#1 cross up MA#2: %s → closed SHORT", reason))
			return
		}
	}

	respondSuccess(w, "MA#1 cross up MA#2 condition set")
}

// POST /webhook/ma/ma1-cross-down-ma2
func handleMA1CrossDownMA2(w http.ResponseWriter, r *http.Request) {
	if !isWebhookUsedInStrategy("/webhook/ma/ma1-cross-down-ma2") {
		log.Printf("⏭️  [WEBHOOK] MA#1 Cross Down MA#2 not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received MA#1 Cross Down MA#2 event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	symbol := normalizeSymbol(event.Ticker)
	if !validateSymbol(w, symbol) {
		return
	}
	updateLatestPrice(symbol, event.Close)
	state := getPositionState(symbol)

	// Update latest price for P/L calculation
	updateLatestPrice(symbol, event.Close)

	log.Printf("📊 MA#1 crossed DOWN through MA#2 for %s", symbol)

	// Update EMA cross state
	mu.Lock()
	state.EMA9CrossedDownEMA21 = true
	state.EMA9CrossedUpEMA21 = false // Clear opposite state
	mu.Unlock()

	// Check if we should exit LONG position
	if state.PositionOpen && state.Position == "long" {
		shouldExit, reason := shouldExitPosition(symbol, true, r)
		if shouldExit {
			log.Printf("⚠️  [EXIT] %s → closing LONG position", reason)
			closePosition(symbol)
			respondSuccess(w, fmt.Sprintf("MA#1 cross down MA#2: %s → closed LONG", reason))
			return
		}
	}

	if shouldOpenPosition(symbol, false, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening SHORT position")
		openShortPosition(symbol, event.Close)
		respondSuccess(w, "MA#1 cross down MA#2 + strategy → SHORT opened")
		return
	}

	respondSuccess(w, "MA#1 cross down MA#2 condition set")
}

// POST /webhook/ma/price-above-ma2
func handlePriceAboveMA2(w http.ResponseWriter, r *http.Request) {
	if !isWebhookUsedInStrategy("/webhook/ma/price-above-ma2") {
		log.Printf("⏭️  [WEBHOOK] Price Above MA#2 not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received Price Above MA#2 event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	symbol := normalizeSymbol(event.Ticker)
	if !validateSymbol(w, symbol) {
		return
	}
	updateLatestPrice(symbol, event.Close)
	state := getPositionState(symbol)

	// Track if this is an opposite direction condition
	trackOppositeDirection(symbol, "/webhook/ma/price-above-ma2")

	log.Printf("📊 Price is above MA#2 for %s", symbol)

	mu.Lock()
	state.PriceAboveEMA20 = true
	state.PriceBelowEMA20 = false
	mu.Unlock()

	if shouldOpenPosition(symbol, true, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening LONG position")
		openLongPosition(symbol, event.Close)
		respondSuccess(w, "Price above MA#2 + strategy → LONG opened")
		return
	}

	respondSuccess(w, "Price above MA#2 condition set")
}

// POST /webhook/ma/price-below-ma2
func handlePriceBelowMA2(w http.ResponseWriter, r *http.Request) {
	if !isWebhookUsedInStrategy("/webhook/ma/price-below-ma2") {
		log.Printf("⏭️  [WEBHOOK] Price Below MA#2 not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received Price Below MA#2 event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	symbol := normalizeSymbol(event.Ticker)
	if !validateSymbol(w, symbol) {
		return
	}
	updateLatestPrice(symbol, event.Close)
	state := getPositionState(symbol)

	// Track if this is an opposite direction condition
	trackOppositeDirection(symbol, "/webhook/ma/price-below-ma2")

	log.Printf("📊 Price is below MA#2 for %s", symbol)

	mu.Lock()
	state.PriceBelowEMA20 = true
	state.PriceAboveEMA20 = false
	mu.Unlock()

	if shouldOpenPosition(symbol, false, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening SHORT position")
		openShortPosition(symbol, event.Close)
		respondSuccess(w, "Price below MA#2 + strategy → SHORT opened")
		return
	}

	respondSuccess(w, "Price below MA#2 condition set")
}

// POST /webhook/ma/price-cross-up-ma2
func handlePriceCrossUpMA2(w http.ResponseWriter, r *http.Request) {
	if !isWebhookUsedInStrategy("/webhook/ma/price-cross-up-ma2") {
		log.Printf("⏭️  [WEBHOOK] Price Cross Up MA#2 not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received Price Cross Up MA#2 event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	symbol := normalizeSymbol(event.Ticker)
	if !validateSymbol(w, symbol) {
		return
	}
	updateLatestPrice(symbol, event.Close)
	state := getPositionState(symbol)

	log.Printf("📊 Price crossed UP through MA#2 for %s", symbol)

	mu.Lock()
	state.PriceAboveEMA20 = true
	state.PriceBelowEMA20 = false
	mu.Unlock()

	// Clear opposite entry condition
	clearEntryConditionForWebhook(symbol, "/webhook/ma/price-cross-down-ma2")

	if shouldOpenPosition(symbol, true, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening LONG position")
		openLongPosition(symbol, event.Close)
		respondSuccess(w, "Price cross up MA#2 + strategy → LONG opened")
		return
	}

	// Check if we should exit SHORT position
	if state.PositionOpen && state.Position == "short" {
		shouldExit, reason := shouldExitPosition(symbol, false, r)
		if shouldExit {
			log.Printf("⚠️  [EXIT] %s → closing SHORT position", reason)
			closePosition(symbol)
			respondSuccess(w, "Price cross up MA#2 → closed SHORT")
			return
		}
	}

	respondSuccess(w, "Price cross up MA#2 condition set")
}

// POST /webhook/ma/price-cross-down-ma2
func handlePriceCrossDownMA2(w http.ResponseWriter, r *http.Request) {
	if !isWebhookUsedInStrategy("/webhook/ma/price-cross-down-ma2") {
		log.Printf("⏭️  [WEBHOOK] Price Cross Down MA#2 not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received Price Cross Down MA#2 event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	symbol := normalizeSymbol(event.Ticker)
	if !validateSymbol(w, symbol) {
		return
	}
	updateLatestPrice(symbol, event.Close)
	state := getPositionState(symbol)

	log.Printf("📊 Price crossed DOWN through MA#2 for %s", symbol)

	mu.Lock()
	state.PriceBelowEMA20 = true
	state.PriceAboveEMA20 = false
	mu.Unlock()

	// Clear opposite entry condition
	clearEntryConditionForWebhook(symbol, "/webhook/ma/price-cross-up-ma2")

	// Check if we should exit LONG position
	if state.PositionOpen && state.Position == "long" {
		shouldExit, reason := shouldExitPosition(symbol, true, r)
		if shouldExit {
			log.Printf("⚠️  [EXIT] %s → closing LONG position", reason)
			closePosition(symbol)
			respondSuccess(w, "Price cross down MA#2 → closed LONG")
			return
		}
	}

	if shouldOpenPosition(symbol, false, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening SHORT position")
		openShortPosition(symbol, event.Close)
		respondSuccess(w, "Price cross down MA#2 + strategy → SHORT opened")
		return
	}

	respondSuccess(w, "Price cross down MA#2 condition set")
}

// ============================================================================
// MA POSITION HANDLERS (Detects crosses internally)
// ============================================================================

// POST /webhook/ma/ma1-above-ma2
func handleMA1AboveMA2(w http.ResponseWriter, r *http.Request) {
	if !isWebhookUsedInStrategy("/webhook/ma/ma1-above-ma2") {
		log.Printf("⏭️  [WEBHOOK] MA#1 Above MA#2 not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received MA#1 Above MA#2 event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	symbol := normalizeSymbol(event.Ticker)
	if !validateSymbol(w, symbol) {
		return
	}
	updateLatestPrice(symbol, event.Close)
	state := getPositionState(symbol)

	// Check if this is a cross by comparing with the opposite state
	// MA1 was below MA2, now above = cross detected
	wasCross := state.EMA9BelowEMA21

	log.Printf("📊 MA#1 above MA#2 for %s", symbol)

	// Track if this is an opposite direction condition after position close
	trackOppositeDirection(symbol, "/webhook/ma/ma1-above-ma2")

	mu.Lock()
	state.EMA9AboveEMA21 = true
	state.EMA9BelowEMA21 = false
	state.EMA9EMA21StateInitialized = true
	mu.Unlock()

	// Only allow LONG entry if: no position open AND (first trade OR last was SHORT OR saw opposite cross after LONG close)
	canEnterLong := !state.PositionOpen && (state.LastClosedDirection == "" || state.LastClosedDirection == "short" || state.OppositeDirectionOccurred)

	if wasCross && canEnterLong {
		log.Printf("✅ [STRATEGY] Entry condition met: MA1 crosses above MA2 - triggers entry")
		if shouldOpenPosition(symbol, true, r) {
			log.Printf("✅ [TRADE] Strategy conditions met! Opening LONG position")
			openLongPosition(symbol, event.Close)
			respondSuccess(w, "MA1 crossed above MA2 + strategy → LONG opened")
			return
		}
	} else if wasCross && state.PositionOpen && state.Position == "short" {
		// Check if this should trigger SHORT exit
		shouldExit, _ := shouldExitPosition(symbol, false, r)
		if shouldExit {
			log.Printf("🚨 [EXIT] MA1 crossed above MA2 - exiting SHORT position")
			closePosition(symbol)
			respondSuccess(w, "MA1 crossed above MA2 → SHORT closed")
			return
		}
	}

	respondSuccess(w, "MA#1 above MA#2 condition set")
}

// POST /webhook/ma/ma1-below-ma2
func handleMA1BelowMA2(w http.ResponseWriter, r *http.Request) {
	if !isWebhookUsedInStrategy("/webhook/ma/ma1-below-ma2") {
		log.Printf("⏭️  [WEBHOOK] MA#1 Below MA#2 not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received MA#1 Below MA#2 event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	symbol := normalizeSymbol(event.Ticker)
	if !validateSymbol(w, symbol) {
		return
	}
	updateLatestPrice(symbol, event.Close)
	state := getPositionState(symbol)

	// Check if this is a cross by comparing with the opposite state
	// MA1 was above MA2, now below = cross detected
	wasCross := state.EMA9AboveEMA21

	log.Printf("📊 MA#1 below MA#2 for %s", symbol)

	// Track if this is an opposite direction condition after position close
	trackOppositeDirection(symbol, "/webhook/ma/ma1-below-ma2")

	mu.Lock()
	state.EMA9BelowEMA21 = true
	state.EMA9AboveEMA21 = false
	state.EMA9EMA21StateInitialized = true
	mu.Unlock()

	// Only allow SHORT entry if: no position open AND (first trade OR last was LONG OR saw opposite cross after SHORT close)
	canEnterShort := !state.PositionOpen && (state.LastClosedDirection == "" || state.LastClosedDirection == "long" || state.OppositeDirectionOccurred)

	if wasCross && canEnterShort {
		log.Printf("✅ [STRATEGY] Entry condition met: MA1 crosses below MA2 - triggers entry")
		if shouldOpenPosition(symbol, false, r) {
			log.Printf("✅ [TRADE] Strategy conditions met! Opening SHORT position")
			openShortPosition(symbol, event.Close)
			respondSuccess(w, "MA1 crossed below MA2 + strategy → SHORT opened")
			return
		}
	} else if wasCross && state.PositionOpen && state.Position == "long" {
		// Check if this should trigger LONG exit
		shouldExit, _ := shouldExitPosition(symbol, true, r)
		if shouldExit {
			log.Printf("🚨 [EXIT] MA1 crossed below MA2 - exiting LONG position")
			closePosition(symbol)
			respondSuccess(w, "MA1 crossed below MA2 → LONG closed")
			return
		}
	}

	respondSuccess(w, "MA#1 below MA#2 condition set")
}

// POST /webhook/ma/ma2-above-ma3
func handleMA2AboveMA3(w http.ResponseWriter, r *http.Request) {
	if !isWebhookUsedInStrategy("/webhook/ma/ma2-above-ma3") {
		log.Printf("⏭️  [WEBHOOK] MA#2 Above MA#3 not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received MA#2 Above MA#3 event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	symbol := normalizeSymbol(event.Ticker)
	if !validateSymbol(w, symbol) {
		return
	}
	updateLatestPrice(symbol, event.Close)
	state := getPositionState(symbol)

	// Track if this is an opposite direction condition
	trackOppositeDirection(symbol, "/webhook/ma/ma2-above-ma3")

	log.Printf("📊 MA#2 above MA#3 for %s", symbol)

	mu.Lock()
	state.MA2AboveMA3 = true
	state.MA2BelowMA3 = false
	mu.Unlock()

	// Only allow LONG entry if reversal requirement met
	canEnterLong := !state.PositionOpen && (state.LastClosedDirection == "" || state.LastClosedDirection == "short" || state.OppositeDirectionOccurred)

	// Check if we should open LONG position
	if shouldOpenPosition(symbol, true, r) && canEnterLong {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening LONG position")
		openLongPosition(symbol, event.Close)
		respondSuccess(w, "MA#2 above MA#3 + strategy → LONG opened")
		return
	}

	// Check if we should open SHORT position
	if shouldOpenPosition(symbol, false, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening SHORT position")
		openShortPosition(symbol, event.Close)
		respondSuccess(w, "MA#2 above MA#3 + strategy → SHORT opened")
		return
	}

	respondSuccess(w, "MA#2 above MA#3 condition set")
}

// POST /webhook/ma/ma2-below-ma3
func handleMA2BelowMA3(w http.ResponseWriter, r *http.Request) {
	if !isWebhookUsedInStrategy("/webhook/ma/ma2-below-ma3") {
		log.Printf("⏭️  [WEBHOOK] MA#2 Below MA#3 not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received MA#2 Below MA#3 event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	symbol := normalizeSymbol(event.Ticker)
	if !validateSymbol(w, symbol) {
		return
	}
	updateLatestPrice(symbol, event.Close)
	state := getPositionState(symbol)

	// Track if this is an opposite direction condition
	trackOppositeDirection(symbol, "/webhook/ma/ma2-below-ma3")

	log.Printf("📊 MA#2 below MA#3 for %s", symbol)

	mu.Lock()
	state.MA2BelowMA3 = true
	state.MA2AboveMA3 = false
	mu.Unlock()

	// Only allow SHORT entry if reversal requirement met
	canEnterShort := !state.PositionOpen && (state.LastClosedDirection == "" || state.LastClosedDirection == "long" || state.OppositeDirectionOccurred)

	// Check if we should open LONG position (shouldn't happen with MA2 below MA3, but keep for consistency)
	if shouldOpenPosition(symbol, true, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening LONG position")
		openLongPosition(symbol, event.Close)
		respondSuccess(w, "MA#2 below MA#3 + strategy → LONG opened")
		return
	}

	// Check if we should open SHORT position
	if shouldOpenPosition(symbol, false, r) && canEnterShort {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening SHORT position")
		openShortPosition(symbol, event.Close)
		respondSuccess(w, "MA#2 below MA#3 + strategy → SHORT opened")
		return
	}

	respondSuccess(w, "MA#2 below MA#3 condition set")
}

// ============================================================================
// SMC (SMART MONEY CONCEPT) STRUCTURE HANDLERS
// ============================================================================

// POST /webhook/smc/low-low
func handleSMCLowLow(w http.ResponseWriter, r *http.Request) {
	if !isWebhookUsedInStrategy("/webhook/smc/low-low") {
		log.Printf("⏭️  [WEBHOOK] SMC Lower Low not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received SMC Lower Low (LL) event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	symbol := normalizeSymbol(event.Ticker)
	if !validateSymbol(w, symbol) {
		return
	}
	updateLatestPrice(symbol, event.Close)
	state := getPositionState(symbol)

	log.Printf("📊 SMC Lower Low (LL) detected for %s", symbol)

	mu.Lock()
	state.SMCLowLow = true
	mu.Unlock()

	// Check if we should open LONG position (LL is a potential reversal)
	if shouldOpenPosition(symbol, true, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening LONG position on SMC LL")
		openLongPosition(symbol, event.Close)
		respondSuccess(w, "SMC Lower Low → LONG opened")
		return
	}

	// Check if we should exit SHORT position (LL is bearish continuation confirmation)
	if state.PositionOpen && state.Position == "short" {
		shouldExit, reason := shouldExitPosition(symbol, false, r)
		if shouldExit {
			log.Printf("⚠️  [EXIT] %s → closing SHORT position", reason)
			closePosition(symbol)
			respondSuccess(w, "SMC Lower Low → SHORT closed")
			return
		}
	}

	respondSuccess(w, "SMC Lower Low condition set")
}

// POST /webhook/smc/high-low
func handleSMCHighLow(w http.ResponseWriter, r *http.Request) {
	if !isWebhookUsedInStrategy("/webhook/smc/high-low") {
		log.Printf("⏭️  [WEBHOOK] SMC Higher Low not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received SMC Higher Low (HL) event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	symbol := normalizeSymbol(event.Ticker)
	if !validateSymbol(w, symbol) {
		return
	}
	updateLatestPrice(symbol, event.Close)
	state := getPositionState(symbol)

	log.Printf("📊 SMC Higher Low (HL) detected for %s", symbol)

	mu.Lock()
	state.SMCHighLow = true
	mu.Unlock()

	// Check if we should open LONG position (HL is bullish continuation)
	if shouldOpenPosition(symbol, true, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening LONG position on SMC HL")
		openLongPosition(symbol, event.Close)
		respondSuccess(w, "SMC Higher Low → LONG opened")
		return
	}

	// Check if we should exit SHORT position (HL is bullish signal)
	if state.PositionOpen && state.Position == "short" {
		shouldExit, reason := shouldExitPosition(symbol, false, r)
		if shouldExit {
			log.Printf("⚠️  [EXIT] %s → closing SHORT position", reason)
			closePosition(symbol)
			respondSuccess(w, "SMC Higher Low → SHORT closed")
			return
		}
	}

	respondSuccess(w, "SMC Higher Low condition set")
}

// POST /webhook/smc/low-high
func handleSMCLowHigh(w http.ResponseWriter, r *http.Request) {
	if !isWebhookUsedInStrategy("/webhook/smc/low-high") {
		log.Printf("⏭️  [WEBHOOK] SMC Lower High not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received SMC Lower High (LH) event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	symbol := normalizeSymbol(event.Ticker)
	if !validateSymbol(w, symbol) {
		return
	}
	updateLatestPrice(symbol, event.Close)
	state := getPositionState(symbol)

	log.Printf("📊 SMC Lower High (LH) detected for %s", symbol)

	mu.Lock()
	state.SMCLowHigh = true
	mu.Unlock()

	// Check if we should open SHORT position (LH is bearish continuation)
	if shouldOpenPosition(symbol, false, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening SHORT position on SMC LH")
		openShortPosition(symbol, event.Close)
		respondSuccess(w, "SMC Lower High → SHORT opened")
		return
	}

	// Check if we should exit LONG position (LH is bearish signal)
	if state.PositionOpen && state.Position == "long" {
		shouldExit, reason := shouldExitPosition(symbol, true, r)
		if shouldExit {
			log.Printf("⚠️  [EXIT] %s → closing LONG position", reason)
			closePosition(symbol)
			respondSuccess(w, "SMC Lower High → LONG closed")
			return
		}
	}

	respondSuccess(w, "SMC Lower High condition set")
}

// POST /webhook/smc/high-high
func handleSMCHighHigh(w http.ResponseWriter, r *http.Request) {
	if !isWebhookUsedInStrategy("/webhook/smc/high-high") {
		log.Printf("⏭️  [WEBHOOK] SMC Higher High not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received SMC Higher High (HH) event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	symbol := normalizeSymbol(event.Ticker)
	if !validateSymbol(w, symbol) {
		return
	}
	updateLatestPrice(symbol, event.Close)
	state := getPositionState(symbol)

	log.Printf("📊 SMC Higher High (HH) detected for %s", symbol)

	mu.Lock()
	state.SMCHighHigh = true
	mu.Unlock()

	// Check if we should open SHORT position (HH is a potential reversal)
	if shouldOpenPosition(symbol, false, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening SHORT position on SMC HH")
		openShortPosition(symbol, event.Close)
	}

	// Check if we should close LONG position
	if state.PositionOpen && state.Position == "long" {
		shouldExit, reason := shouldExitPosition(symbol, true, r)
		if shouldExit {
			log.Printf("✅ [TRADE] %s - Closing LONG position on SMC HH", reason)
			closePosition(symbol)
		}
	}

	respondSuccess(w, "SMC Higher High processed")
}

// ============================================================================
// Generic TradingView Webhook Handlers
// ============================================================================

func handleGenericTakeLongPosition(w http.ResponseWriter, r *http.Request) {
	if !isWebhookUsedInStrategy("/webhook/generic/take-long-position") {
		log.Printf("⏭️  [WEBHOOK] Generic take-long-position not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received Generic Take Long Position event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	symbol := normalizeSymbol(event.Ticker)
	if !validateSymbol(w, symbol) {
		return
	}
	updateLatestPrice(symbol, event.Close)
	state := getPositionState(symbol)

	log.Printf("📊 Generic Take Long Position signal for %s", symbol)

	// Immediately open LONG position
	if shouldOpenPosition(symbol, true, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Opening LONG position from generic signal")
		openLongPosition(symbol, event.Close)
	}

	respondSuccess(w, "Generic take long position processed")
}

func handleGenericTakeShortPosition(w http.ResponseWriter, r *http.Request) {
	if !isWebhookUsedInStrategy("/webhook/generic/take-short-position") {
		log.Printf("⏭️  [WEBHOOK] Generic take-short-position not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received Generic Take Short Position event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	symbol := normalizeSymbol(event.Ticker)
	if !validateSymbol(w, symbol) {
		return
	}
	updateLatestPrice(symbol, event.Close)
	state := getPositionState(symbol)

	log.Printf("📊 Generic Take Short Position signal for %s", symbol)

	// Immediately open SHORT position
	if shouldOpenPosition(symbol, false, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Opening SHORT position from generic signal")
		openShortPosition(symbol, event.Close)
	}

	respondSuccess(w, "Generic take short position processed")
}

func handleGenericExit(w http.ResponseWriter, r *http.Request) {
	if !isWebhookUsedInStrategy("/webhook/generic/exit") {
		log.Printf("⏭️  [WEBHOOK] Generic exit not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received Generic Exit event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	symbol := normalizeSymbol(event.Ticker)
	if !validateSymbol(w, symbol) {
		return
	}
	updateLatestPrice(symbol, event.Close)
	state := getPositionState(symbol)

	log.Printf("📊 Generic Exit signal for %s", symbol)

	// Close any open position (long or short)
	if state.PositionOpen {
		if state.Position == "long" {
			shouldExit, reason := shouldExitPosition(symbol, true, r)
			if shouldExit {
				log.Printf("✅ [TRADE] %s - Closing LONG position from generic exit signal", reason)
				closePosition(symbol)
			}
		} else if state.Position == "short" {
			shouldExit, reason := shouldExitPosition(symbol, false, r)
			if shouldExit {
				log.Printf("✅ [TRADE] %s - Closing SHORT position from generic exit signal", reason)
				closePosition(symbol)
			}
		}
	}

	respondSuccess(w, "Generic exit processed")
}

// ============================================================================
// MACD HISTOGRAM ZERO-LINE CROSS HANDLERS
// ============================================================================

// POST /webhook/macd/histogram-cross-up-0
func handleMACDHistCrossUp0(w http.ResponseWriter, r *http.Request) {
	if !isWebhookUsedInStrategy("/webhook/macd/histogram-cross-up-0") {
		log.Printf("⏭️  [WEBHOOK] MACD Histogram Cross Up 0 not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received MACD Histogram Cross Up 0 event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	symbol := normalizeSymbol(event.Ticker)
	if !validateSymbol(w, symbol) {
		return
	}
	updateLatestPrice(symbol, event.Close)
	state := getPositionState(symbol)

	log.Printf("📊 MACD Histogram crossed UP through zero for %s", symbol)

	mu.Lock()
	state.MACDHistAboveZero = true
	state.MACDHistBelowZero = false
	mu.Unlock()

	// Clear entry condition that depended on MACDHistBelowZero being true
	clearEntryConditionForWebhook(symbol, "/webhook/macd/histogram-cross-down-0")

	// Check if we should exit SHORT position
	if state.PositionOpen && state.Position == "short" {
		shouldExit, reason := shouldExitPosition(symbol, false, r)
		if shouldExit {
			log.Printf("⚠️  [EXIT] %s → closing SHORT position", reason)
			closePosition(symbol)
			state = getPositionState(symbol)
		}
	}

	if shouldOpenPosition(symbol, true, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening LONG position")
		openLongPosition(symbol, event.Close)
		respondSuccess(w, "MACD histogram cross up 0 + strategy → LONG opened")
		return
	}

	respondSuccess(w, "MACD histogram cross up 0 condition set")
}

// ============================================================================
// RSI DIRECTIONAL CROSS HANDLERS (Specific levels with direction)
// ============================================================================

// POST /webhook/rsi/cross-down-40
func handleRSICrossDown40(w http.ResponseWriter, r *http.Request) {
	if !isWebhookUsedInStrategy("/webhook/rsi/cross-down-40") {
		log.Printf("⏭️  [WEBHOOK] RSI Cross Down 40 not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received RSI Cross Down 40 event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	symbol := normalizeSymbol(event.Ticker)
	if !validateSymbol(w, symbol) {
		return
	}
	updateLatestPrice(symbol, event.Close)
	state := getPositionState(symbol)

	log.Printf("📊 RSI crossed DOWN through 40 for %s", symbol)

	// Update RSI state
	mu.Lock()
	state.RSICrossedDown40 = true
	state.RSICrossedUp40 = false // Clear opposite state
	mu.Unlock()

	// Check if we should exit LONG position
	if state.PositionOpen && state.Position == "long" {
		shouldExit, reason := shouldExitPosition(symbol, true, r)
		if shouldExit {
			log.Printf("⚠️  [EXIT] %s → closing LONG position", reason)
			closePosition(symbol)
			state = getPositionState(symbol)
		}
	}

	if shouldOpenPosition(symbol, false, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening SHORT position")
		openShortPosition(symbol, event.Close)
		respondSuccess(w, "RSI cross down 40 + strategy → SHORT opened")
		return
	}

	respondSuccess(w, "RSI cross down 40 condition set")
}

// POST /webhook/rsi/cross-up-60
func handleRSICrossUp60(w http.ResponseWriter, r *http.Request) {
	if !isWebhookUsedInStrategy("/webhook/rsi/cross-up-60") {
		log.Printf("⏭️  [WEBHOOK] RSI Cross Up 60 not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received RSI Cross Up 60 event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	symbol := normalizeSymbol(event.Ticker)
	if !validateSymbol(w, symbol) {
		return
	}
	updateLatestPrice(symbol, event.Close)
	state := getPositionState(symbol)

	log.Printf("📊 RSI crossed UP through 60 for %s", symbol)

	// Update RSI state
	mu.Lock()
	state.RSICrossedUp60 = true
	state.RSICrossedDown60 = false // Clear opposite state
	mu.Unlock()

	if shouldOpenPosition(symbol, true, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening LONG position")
		openLongPosition(symbol, event.Close)
		respondSuccess(w, "RSI cross up 60 + strategy → LONG opened")
		return
	}

	respondSuccess(w, "RSI cross up 60 condition set")
}

// POST /webhook/rsi/cross-down-60
func handleRSICrossDown60(w http.ResponseWriter, r *http.Request) {
	if !isWebhookUsedInStrategy("/webhook/rsi/cross-down-60") {
		log.Printf("⏭️  [WEBHOOK] RSI Cross Down 60 not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received RSI Cross Down 60 event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	symbol := normalizeSymbol(event.Ticker)
	if !validateSymbol(w, symbol) {
		return
	}
	updateLatestPrice(symbol, event.Close)
	state := getPositionState(symbol)

	log.Printf("📊 RSI crossed DOWN through 60 for %s", symbol)

	// Check if we should exit LONG position
	if state.PositionOpen && state.Position == "long" {
		shouldExit, reason := shouldExitPosition(symbol, true, r)
		if shouldExit {
			log.Printf("⚠️  [EXIT] %s → closing LONG position", reason)
			closePosition(symbol)
			state = getPositionState(symbol)
			shouldOpenPosition(symbol, false, r)
		}
	}

	if shouldOpenPosition(symbol, false, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening SHORT position")
		openShortPosition(symbol, event.Close)
		respondSuccess(w, "RSI cross down 60 + strategy → SHORT opened")
		return
	}

	respondSuccess(w, "RSI cross down 60 condition set")
}

// POST /webhook/rsi/cross-up-80
func handleRSICrossUp80(w http.ResponseWriter, r *http.Request) {
	if !isWebhookUsedInStrategy("/webhook/rsi/cross-up-80") {
		log.Printf("⏭️  [WEBHOOK] RSI Cross Up 80 not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received RSI Cross Up 80 event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	symbol := normalizeSymbol(event.Ticker)
	if !validateSymbol(w, symbol) {
		return
	}
	updateLatestPrice(symbol, event.Close)
	state := getPositionState(symbol)

	log.Printf("📊 RSI crossed UP through 80 for %s (extreme overbought)", symbol)

	// Check if we should exit LONG position
	if state.PositionOpen && state.Position == "long" {
		shouldExit, reason := shouldExitPosition(symbol, true, r)
		if shouldExit {
			log.Printf("⚠️  [EXIT] %s → closing LONG position", reason)
			closePosition(symbol)
			state = getPositionState(symbol)
			shouldOpenPosition(symbol, false, r)
		}
	}

	if shouldOpenPosition(symbol, false, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening SHORT position")
		openShortPosition(symbol, event.Close)
		respondSuccess(w, "RSI cross up 80 + strategy → SHORT opened")
		return
	}

	respondSuccess(w, "RSI cross up 80 condition set")
}

// POST /webhook/rsi/cross-down-80
func handleRSICrossDown80(w http.ResponseWriter, r *http.Request) {
	if !isWebhookUsedInStrategy("/webhook/rsi/cross-down-80") {
		log.Printf("⏭️  [WEBHOOK] RSI Cross Down 80 not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received RSI Cross Down 80 event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	symbol := normalizeSymbol(event.Ticker)
	if !validateSymbol(w, symbol) {
		return
	}
	updateLatestPrice(symbol, event.Close)
	state := getPositionState(symbol)

	log.Printf("📊 RSI crossed DOWN through 80 for %s", symbol)

	if shouldOpenPosition(symbol, false, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening SHORT position")
		openShortPosition(symbol, event.Close)
		respondSuccess(w, "RSI cross down 80 + strategy → SHORT opened")
		return
	}

	respondSuccess(w, "RSI cross down 80 condition set")
}

// POST /webhook/rsi/cross-up-50
func handleRSICrossUp50(w http.ResponseWriter, r *http.Request) {
	if !isWebhookUsedInStrategy("/webhook/rsi/cross-up-50") {
		log.Printf("⏭️  [WEBHOOK] RSI Cross Up 50 not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received RSI Cross Up 50 event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	symbol := normalizeSymbol(event.Ticker)
	if !validateSymbol(w, symbol) {
		return
	}
	updateLatestPrice(symbol, event.Close)
	state := getPositionState(symbol)

	log.Printf("📊 RSI crossed UP through 50 for %s", symbol)

	// Update RSI state
	mu.Lock()
	state.RSIAbove50 = true
	state.RSIBelow50 = false
	mu.Unlock()

	// Clear entry condition that depended on RSIBelow50 being true
	clearEntryConditionForWebhook(symbol, "/webhook/rsi/cross-down-50")

	if shouldOpenPosition(symbol, true, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening LONG position")
		openLongPosition(symbol, event.Close)
		respondSuccess(w, "RSI cross up 50 + strategy → LONG opened")
		return
	}

	respondSuccess(w, "RSI cross up 50 condition set")
}

// POST /webhook/rsi/cross-down-50
func handleRSICrossDown50(w http.ResponseWriter, r *http.Request) {
	if !isWebhookUsedInStrategy("/webhook/rsi/cross-down-50") {
		log.Printf("⏭️  [WEBHOOK] RSI Cross Down 50 not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received RSI Cross Down 50 event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	symbol := normalizeSymbol(event.Ticker)
	if !validateSymbol(w, symbol) {
		return
	}
	updateLatestPrice(symbol, event.Close)
	state := getPositionState(symbol)

	log.Printf("📊 RSI crossed DOWN through 50 for %s", symbol)

	// Update RSI state
	mu.Lock()
	state.RSIBelow50 = true
	state.RSIAbove50 = false
	mu.Unlock()

	// Clear entry condition that depended on RSIAbove50 being true
	clearEntryConditionForWebhook(symbol, "/webhook/rsi/cross-up-50")

	if shouldOpenPosition(symbol, false, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening SHORT position")
		openShortPosition(symbol, event.Close)
		respondSuccess(w, "RSI cross down 50 + strategy → SHORT opened")
		return
	}

	respondSuccess(w, "RSI cross down 50 condition set")
}

// POST /webhook/rsi/cross-down-70
func handleRSICrossDown70(w http.ResponseWriter, r *http.Request) {
	if !isWebhookUsedInStrategy("/webhook/rsi/cross-down-70") {
		log.Printf("⏭️  [WEBHOOK] RSI Cross Down 70 not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received RSI Cross Down 70 event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	symbol := normalizeSymbol(event.Ticker)
	if !validateSymbol(w, symbol) {
		return
	}
	updateLatestPrice(symbol, event.Close)
	state := getPositionState(symbol)

	log.Printf("📊 RSI crossed DOWN from 70 for %s", symbol)

	// Update RSI state
	mu.Lock()
	state.RSICrossedDown70 = true
	state.RSICrossedUp70 = false // Clear opposite state
	mu.Unlock()

	// Check if we should exit LONG position
	if state.PositionOpen && state.Position == "long" {
		shouldExit, reason := shouldExitPosition(symbol, true, r)
		if shouldExit {
			log.Printf("⚠️  [EXIT] %s → closing LONG position", reason)
			closePosition(symbol)
			respondSuccess(w, fmt.Sprintf("RSI cross down 70: %s → closed LONG", reason))
			return
		}
	}

	respondSuccess(w, "RSI crossed down from 70")
}

// POST /webhook/rsi/cross-down-overbuy
func handleRSICrossDownOverbuy(w http.ResponseWriter, r *http.Request) {
	if !isWebhookUsedInStrategy("/webhook/rsi/cross-down-overbuy") {
		log.Printf("⏭️  [WEBHOOK] RSI Cross Down Overbuy not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received RSI Cross Down Overbuy event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	symbol := normalizeSymbol(event.Ticker)
	if !validateSymbol(w, symbol) {
		return
	}
	updateLatestPrice(symbol, event.Close)
	state := getPositionState(symbol)

	log.Printf("📊 RSI crossed DOWN from overbought for %s", symbol)

	// Update RSI state
	mu.Lock()
	state.RSICrossedDown70 = true
	state.RSICrossedUp70 = false // Clear opposite state
	mu.Unlock()

	// Clear opposite entry condition
	clearEntryConditionForWebhook(symbol, "/webhook/rsi/cross-up-oversell")

	// Check if we should exit LONG position
	if state.PositionOpen && state.Position == "long" {
		shouldExit, reason := shouldExitPosition(symbol, true, r)
		if shouldExit {
			log.Printf("⚠️  [EXIT] %s → closing LONG position", reason)
			closePosition(symbol)
			respondSuccess(w, "RSI cross down overbuy → closed LONG")
			return
		}
	}

	respondSuccess(w, "RSI crossed down from overbought")
}

// POST /webhook/rsi/cross-up-oversell
func handleRSICrossUpOversell(w http.ResponseWriter, r *http.Request) {
	if !isWebhookUsedInStrategy("/webhook/rsi/cross-up-oversell") {
		log.Printf("⏭️  [WEBHOOK] RSI Cross Up Oversell not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received RSI Cross Up Oversell event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	symbol := normalizeSymbol(event.Ticker)
	if !validateSymbol(w, symbol) {
		return
	}
	updateLatestPrice(symbol, event.Close)
	state := getPositionState(symbol)

	log.Printf("📊 RSI crossed UP from oversold for %s", symbol)

	// Update RSI state
	mu.Lock()
	state.RSICrossedUp30 = true
	state.RSICrossedDown30 = false // Clear opposite state
	mu.Unlock()

	// Clear opposite entry condition
	clearEntryConditionForWebhook(symbol, "/webhook/rsi/cross-down-overbuy")

	// Check if we should exit SHORT position
	if state.PositionOpen && state.Position == "short" {
		shouldExit, reason := shouldExitPosition(symbol, false, r)
		if shouldExit {
			log.Printf("⚠️  [EXIT] %s → closing SHORT position", reason)
			closePosition(symbol)
			respondSuccess(w, "RSI cross up oversell → closed SHORT")
			return
		}
	}

	respondSuccess(w, "RSI crossed up from oversold")
}

// POST /webhook/rsi/cross-up-30
func handleRSICrossUp30(w http.ResponseWriter, r *http.Request) {
	if !isWebhookUsedInStrategy("/webhook/rsi/cross-up-30") {
		log.Printf("⏭️  [WEBHOOK] RSI Cross Up 30 not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received RSI Cross Up 30 event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	symbol := normalizeSymbol(event.Ticker)
	if !validateSymbol(w, symbol) {
		return
	}
	updateLatestPrice(symbol, event.Close)
	state := getPositionState(symbol)

	log.Printf("📊 RSI crossed UP from 30 for %s", symbol)

	// Update RSI state
	mu.Lock()
	state.RSICrossedUp30 = true
	state.RSICrossedDown30 = false // Clear opposite state
	mu.Unlock()

	// Check if we should exit SHORT position
	if state.PositionOpen && state.Position == "short" {
		shouldExit, reason := shouldExitPosition(symbol, false, r)
		if shouldExit {
			log.Printf("⚠️  [EXIT] %s → closing SHORT position", reason)
			closePosition(symbol)
			respondSuccess(w, fmt.Sprintf("RSI cross up 30: %s → closed SHORT", reason))
			return
		}
	}

	respondSuccess(w, "RSI crossed up from 30")
}

// POST /webhook/macd/histogram-cross-down-0
func handleMACDHistCrossDown0(w http.ResponseWriter, r *http.Request) {
	if !isWebhookUsedInStrategy("/webhook/macd/histogram-cross-down-0") {
		log.Printf("⏭️  [WEBHOOK] MACD Histogram Cross Down 0 not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received MACD Histogram Cross Down 0 event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	symbol := normalizeSymbol(event.Ticker)
	if !validateSymbol(w, symbol) {
		return
	}
	updateLatestPrice(symbol, event.Close)
	state := getPositionState(symbol)

	log.Printf("📊 MACD Histogram crossed DOWN through zero for %s", symbol)

	mu.Lock()
	state.MACDHistBelowZero = true
	state.MACDHistAboveZero = false
	mu.Unlock()

	// Clear entry condition that depended on MACDHistAboveZero being true
	clearEntryConditionForWebhook(symbol, "/webhook/macd/histogram-cross-up-0")

	// Check if we should exit LONG position
	if state.PositionOpen && state.Position == "long" {
		shouldExit, reason := shouldExitPosition(symbol, true, r)
		if shouldExit {
			log.Printf("⚠️  [EXIT] %s → closing LONG position", reason)
			closePosition(symbol)
			state = getPositionState(symbol)
		}
	}

	if shouldOpenPosition(symbol, false, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening SHORT position")
		openShortPosition(symbol, event.Close)
		respondSuccess(w, "MACD histogram cross down 0 + strategy → SHORT opened")
		return
	}

	respondSuccess(w, "MACD histogram cross down 0 condition set")
}

// POST /webhook/macd/histogram-above-0
func handleMACDHistAbove0(w http.ResponseWriter, r *http.Request) {
	if !isWebhookUsedInStrategy("/webhook/macd/histogram-above-0") {
		log.Printf("⏭️  [WEBHOOK] MACD Histogram Above 0 not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received MACD Histogram Above 0 event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	symbol := normalizeSymbol(event.Ticker)
	if !validateSymbol(w, symbol) {
		return
	}
	updateLatestPrice(symbol, event.Close)
	state := getPositionState(symbol)

	log.Printf("📊 MACD Histogram is above zero for %s", symbol)

	mu.Lock()
	state.MACDHistAboveZero = true
	state.MACDHistBelowZero = false
	mu.Unlock()

	// Check if we should exit SHORT position
	if state.PositionOpen && state.Position == "short" {
		shouldExit, reason := shouldExitPosition(symbol, false, r)
		if shouldExit {
			log.Printf("⚠️  [EXIT] %s → closing SHORT position", reason)
			closePosition(symbol)
			state = getPositionState(symbol)
		}
	}

	if shouldOpenPosition(symbol, true, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening LONG position")
		openLongPosition(symbol, event.Close)
		respondSuccess(w, "MACD histogram above 0 + strategy → LONG opened")
		return
	}

	respondSuccess(w, "MACD histogram above 0 condition set")
}

// POST /webhook/macd/histogram-below-0
func handleMACDHistBelow0(w http.ResponseWriter, r *http.Request) {
	if !isWebhookUsedInStrategy("/webhook/macd/histogram-below-0") {
		log.Printf("⏭️  [WEBHOOK] MACD Histogram Below 0 not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received MACD Histogram Below 0 event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	symbol := normalizeSymbol(event.Ticker)
	if !validateSymbol(w, symbol) {
		return
	}
	updateLatestPrice(symbol, event.Close)
	state := getPositionState(symbol)

	log.Printf("📊 MACD Histogram is below zero for %s", symbol)

	mu.Lock()
	state.MACDHistBelowZero = true
	state.MACDHistAboveZero = false
	mu.Unlock()

	// Check if we should exit LONG position
	if state.PositionOpen && state.Position == "long" {
		shouldExit, reason := shouldExitPosition(symbol, true, r)
		if shouldExit {
			log.Printf("⚠️  [EXIT] %s → closing LONG position", reason)
			closePosition(symbol)
			state = getPositionState(symbol)
		}
	}

	if shouldOpenPosition(symbol, false, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening SHORT position")
		openShortPosition(symbol, event.Close)
		respondSuccess(w, "MACD histogram below 0 + strategy → SHORT opened")
		return
	}

	respondSuccess(w, "MACD histogram below 0 condition set")
}

// POST /webhook/ema/9-cross-up-21
func handleEMA9CrossUp21(w http.ResponseWriter, r *http.Request) {
	// Allow this webhook if either the old EMA path or new MA path is used in strategy
	if !isWebhookUsedInStrategy("/webhook/ema/9-cross-up-21") && !isWebhookUsedInStrategy("/webhook/ma/ma1-cross-up-ma2") {
		log.Printf("⏭️  [WEBHOOK] EMA 9 Cross Up 21 not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received EMA 9 Cross Up 21 event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	symbol := normalizeSymbol(event.Ticker)
	if !validateSymbol(w, symbol) {
		return
	}
	updateLatestPrice(symbol, event.Close)
	state := getPositionState(symbol)

	log.Printf("📊 EMA 9 crossed UP through EMA 21 for %s", symbol)

	// Update EMA cross state
	mu.Lock()
	state.EMA9CrossedUpEMA21 = true
	state.EMA9CrossedDownEMA21 = false // Clear opposite state
	mu.Unlock()

	if shouldOpenPosition(symbol, true, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening LONG position")
		openLongPosition(symbol, event.Close)
		respondSuccess(w, "EMA 9 cross up 21 + strategy → LONG opened")
		return
	}

	// Check if we should exit SHORT position
	if state.PositionOpen && state.Position == "short" {
		shouldExit, reason := shouldExitPosition(symbol, false, r)
		if shouldExit {
			log.Printf("⚠️  [EXIT] %s → closing SHORT position", reason)
			closePosition(symbol)
			respondSuccess(w, fmt.Sprintf("EMA 9 cross up 21: %s → closed SHORT", reason))
			return
		}
	}

	respondSuccess(w, "EMA 9 cross up 21 condition set")
}

// POST /webhook/ema/9-cross-down-21
func handleEMA9CrossDown21(w http.ResponseWriter, r *http.Request) {
	// Allow this webhook if either the old EMA path or new MA path is used in strategy
	if !isWebhookUsedInStrategy("/webhook/ema/9-cross-down-21") && !isWebhookUsedInStrategy("/webhook/ma/ma1-cross-down-ma2") {
		log.Printf("⏭️  [WEBHOOK] EMA 9 Cross Down 21 not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received EMA 9 Cross Down 21 event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	symbol := normalizeSymbol(event.Ticker)
	if !validateSymbol(w, symbol) {
		return
	}
	updateLatestPrice(symbol, event.Close)
	state := getPositionState(symbol)

	log.Printf("📊 EMA 9 crossed DOWN through EMA 21 for %s", symbol)

	// Update EMA cross state
	mu.Lock()
	state.EMA9CrossedDownEMA21 = true
	state.EMA9CrossedUpEMA21 = false // Clear opposite state
	mu.Unlock()

	// Check if we should exit LONG position
	if state.PositionOpen && state.Position == "long" {
		shouldExit, reason := shouldExitPosition(symbol, true, r)
		if shouldExit {
			log.Printf("⚠️  [EXIT] %s → closing LONG position", reason)
			closePosition(symbol)
			respondSuccess(w, fmt.Sprintf("EMA 9 cross down 21: %s → closed LONG", reason))
			return
		}
	}

	if shouldOpenPosition(symbol, false, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening SHORT position")
		openShortPosition(symbol, event.Close)
		respondSuccess(w, "EMA 9 cross down 21 + strategy → SHORT opened")
		return
	}

	respondSuccess(w, "EMA 9 cross down 21 condition set")
}

// POST /webhook/ema/9-above-21
func handleEMA9Above21(w http.ResponseWriter, r *http.Request) {
	if !isWebhookUsedInStrategy("/webhook/ema/9-above-21") {
		log.Printf("⏭️  [WEBHOOK] EMA 9 Above 21 not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received EMA 9 Above 21 event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	symbol := normalizeSymbol(event.Ticker)
	if !validateSymbol(w, symbol) {
		return
	}
	updateLatestPrice(symbol, event.Close)
	state := getPositionState(symbol)

	log.Printf("📊 EMA 9 is above EMA 21 for %s", symbol)

	// Update EMA state
	mu.Lock()
	state.EMA9CrossedUpEMA21 = true
	state.EMA9CrossedDownEMA21 = false
	mu.Unlock()

	if shouldOpenPosition(symbol, true, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening LONG position")
		openLongPosition(symbol, event.Close)
		respondSuccess(w, "EMA 9 above 21 + strategy → LONG opened")
		return
	}

	// Check if we should exit SHORT position
	if state.PositionOpen && state.Position == "short" {
		shouldExit, reason := shouldExitPosition(symbol, false, r)
		if shouldExit {
			log.Printf("⚠️  [EXIT] %s → closing SHORT position", reason)
			closePosition(symbol)
			respondSuccess(w, fmt.Sprintf("EMA 9 above 21: %s → closed SHORT", reason))
			return
		}
	}

	respondSuccess(w, "EMA 9 above 21 condition set")
}

// POST /webhook/ema/9-below-21
func handleEMA9Below21(w http.ResponseWriter, r *http.Request) {
	if !isWebhookUsedInStrategy("/webhook/ema/9-below-21") {
		log.Printf("⏭️  [WEBHOOK] EMA 9 Below 21 not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received EMA 9 Below 21 event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	symbol := normalizeSymbol(event.Ticker)
	if !validateSymbol(w, symbol) {
		return
	}
	updateLatestPrice(symbol, event.Close)
	state := getPositionState(symbol)

	log.Printf("📊 EMA 9 is below EMA 21 for %s", symbol)

	// Update EMA state
	mu.Lock()
	state.EMA9CrossedDownEMA21 = true
	state.EMA9CrossedUpEMA21 = false
	mu.Unlock()

	if state.PositionOpen && state.Position == "long" {
		shouldExit, reason := shouldExitPosition(symbol, true, r)
		if shouldExit {
			log.Printf("⚠️  [EXIT] %s → closing LONG position", reason)
			closePosition(symbol)
			respondSuccess(w, fmt.Sprintf("EMA 9 below 21: %s → closed LONG", reason))
			return
		}
	}

	if shouldOpenPosition(symbol, false, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening SHORT position")
		openShortPosition(symbol, event.Close)
		respondSuccess(w, "EMA 9 below 21 + strategy → SHORT opened")
		return
	}

	respondSuccess(w, "EMA 9 below 21 condition set")
}

// POST /webhook/ema/price-cross-down-50
func handlePriceCrossDownEMA50(w http.ResponseWriter, r *http.Request) {
	if !isWebhookUsedInStrategy("/webhook/ema/price-cross-down-50") {
		log.Printf("⏭️  [WEBHOOK] Price Cross Down EMA50 not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received Price Cross Down EMA50 event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	symbol := normalizeSymbol(event.Ticker)
	if !validateSymbol(w, symbol) {
		return
	}
	updateLatestPrice(symbol, event.Close)
	state := getPositionState(symbol)

	log.Printf("📊 Price crossed DOWN through EMA 50 for %s", symbol)

	mu.Lock()
	state.PriceBelowEMA50 = true
	state.PriceAboveEMA50 = false
	mu.Unlock()

	// Clear entry condition that depended on PriceAboveEMA50 being true
	clearEntryConditionForWebhook(symbol, "/webhook/ema/price-above-ema50")
	clearEntryConditionForWebhook(symbol, "/webhook/ema/price-cross-up-50")

	// Check if we should exit LONG position (hard stop)
	if state.PositionOpen && state.Position == "long" {
		shouldExit, reason := shouldExitPosition(symbol, true, r)
		if shouldExit {
			log.Printf("⚠️  [EXIT] %s → closing LONG position (hard stop)", reason)
			closePosition(symbol)
			respondSuccess(w, fmt.Sprintf("Price cross down EMA50: %s → closed LONG", reason))
			return
		}
	}

	respondSuccess(w, "Price crossed down EMA50")
}

// POST /webhook/ema/price-cross-up-50
func handlePriceCrossUpEMA50(w http.ResponseWriter, r *http.Request) {
	if !isWebhookUsedInStrategy("/webhook/ema/price-cross-up-50") {
		log.Printf("⏭️  [WEBHOOK] Price Cross Up EMA50 not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received Price Cross Up EMA50 event")

	var event TradingViewEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("❌ [ERROR] Invalid JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	symbol := normalizeSymbol(event.Ticker)
	if !validateSymbol(w, symbol) {
		return
	}
	updateLatestPrice(symbol, event.Close)
	state := getPositionState(symbol)

	log.Printf("📊 Price crossed UP through EMA 50 for %s", symbol)

	mu.Lock()
	state.PriceAboveEMA50 = true
	state.PriceBelowEMA50 = false
	mu.Unlock()

	// Clear entry condition that depended on PriceBelowEMA50 being true
	clearEntryConditionForWebhook(symbol, "/webhook/ema/price-below-ema50")
	clearEntryConditionForWebhook(symbol, "/webhook/ema/price-cross-down-50")

	// Check if we should exit SHORT position (hard stop)
	if state.PositionOpen && state.Position == "short" {
		shouldExit, reason := shouldExitPosition(symbol, false, r)
		if shouldExit {
			log.Printf("⚠️  [EXIT] %s → closing SHORT position (hard stop)", reason)
			closePosition(symbol)
			respondSuccess(w, fmt.Sprintf("Price cross up EMA50: %s → closed SHORT", reason))
			return
		}
	}

	respondSuccess(w, "Price crossed up EMA50")
}

// ============================================================================
// OANDA TRADING FUNCTIONS
// ============================================================================

// Get instrument-specific leverage from OANDA
func getInstrumentInfo(symbol string) (leverage float64, pipLocation int, err error) {
	url := fmt.Sprintf("%s/v3/accounts/%s/instruments", oandaBaseURL, oandaAccountID)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+oandaAPIKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, 0, err
	}

	instruments, ok := result["instruments"].([]interface{})
	if !ok {
		return 0, 0, fmt.Errorf("invalid instruments data")
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
			return 0, 0, fmt.Errorf("marginRate not found for %s", symbol)
		}

		marginRate := 0.0
		fmt.Sscanf(marginRateStr, "%f", &marginRate)

		if marginRate <= 0 {
			return 0, 0, fmt.Errorf("invalid margin rate: %f", marginRate)
		}

		// Leverage = 1 / marginRate
		// e.g., marginRate 0.02 = 50:1, 0.05 = 20:1, 0.03 = 33:1
		calculatedLeverage := 1.0 / marginRate

		// Get pip location (displayPrecision tells us decimal places)
		// pipLocation is negative: -4 means pip is at 4th decimal place (0.0001)
		pipLocationFloat, _ := instrument["pipLocation"].(float64)
		calculatedPipLocation := int(pipLocationFloat)

		return calculatedLeverage, calculatedPipLocation, nil
	}

	return 0, 0, fmt.Errorf("instrument %s not found", symbol)
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

// Calculate price move needed to achieve a specific dollar amount P&L
// Accounts for currency conversion factors from OANDA
func calculatePriceMoveForDollars(symbol string, currentPrice float64, targetDollars float64, units int, isLong bool) (float64, error) {
	// Get pricing info from OANDA which includes homeConversionFactors
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
		return 0, fmt.Errorf("no pricing data available")
	}

	priceData, ok := prices[0].(map[string]interface{})
	if !ok {
		return 0, fmt.Errorf("invalid price data format")
	}

	// Get home conversion factors (converts P&L to account currency - USD)
	homeConversion, ok := priceData["homeConversionFactors"].(map[string]interface{})
	if !ok {
		// Fallback: assume it's a XXX_USD pair where conversion is simple
		return targetDollars / float64(units), nil
	}

	// For LONG positions, we use gainQuoteHome (quote currency gain -> home currency)
	// For SHORT positions, we use lossQuoteHome (quote currency gain -> home currency)
	// These account for the current exchange rates
	var conversionFactor float64
	if isLong {
		gainFactorStr, ok := homeConversion["gainQuoteHome"].(string)
		if ok {
			fmt.Sscanf(gainFactorStr, "%f", &conversionFactor)
		}
	} else {
		lossFactorStr, ok := homeConversion["lossQuoteHome"].(string)
		if ok {
			fmt.Sscanf(lossFactorStr, "%f", &conversionFactor)
		}
	}

	if conversionFactor == 0 {
		// Fallback
		return targetDollars / float64(units), nil
	}

	// P&L in USD = price_move × units × conversionFactor
	// Therefore: price_move = targetDollars / (units × conversionFactor)
	priceMove := targetDollars / (float64(units) * conversionFactor)

	log.Printf("💱 [CONVERSION] Factor: %.6f, Price move: %.5f for $%.2f target", conversionFactor, priceMove, targetDollars)

	return priceMove, nil
}

// Calculate take profit price based on pips or percentage
func calculateTakeProfitPrice(symbol string, entryPrice float64, isLong bool, units int) (string, error) {
	if takeProfitPips == "" && takeProfitPct == "" && takeProfitDollars == "" {
		return "", nil // No take profit set
	}

	var tpPrice float64

	if takeProfitDollars != "" {
		// Calculate based on dollar profit target using OANDA's home conversion
		dollars := 0.0
		fmt.Sscanf(takeProfitDollars, "%f", &dollars)

		// Query OANDA for accurate conversion factor
		priceMove, err := calculatePriceMoveForDollars(symbol, entryPrice, dollars, units, isLong)
		if err != nil {
			log.Printf("⚠️  [WARNING] Failed to get accurate conversion, using simple calculation: %v", err)
			// Fallback to simple calculation (works for XXX_USD pairs)
			priceMove = dollars / float64(units)
		}

		if isLong {
			tpPrice = entryPrice + priceMove
		} else {
			tpPrice = entryPrice - priceMove
		}

		log.Printf("🎯 [TP CALC] $%.2f profit target with %d units = %.5f price move", dollars, units, priceMove)
		log.Printf("🎯 [TP CALC] Entry: %.5f → TP: %.5f (%s)", entryPrice, tpPrice, map[bool]string{true: "LONG", false: "SHORT"}[isLong])

	} else if takeProfitPips != "" {
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

func calculateStopLossPrice(symbol string, entryPrice float64, isLong bool, units int) (string, error) {
	if stopLossPips == "" && stopLossPct == "" && stopLossDollars == "" {
		return "", nil // No stop loss set
	}

	var slPrice float64

	if stopLossDollars != "" {
		// Calculate based on dollar loss limit using OANDA's home conversion
		dollars := 0.0
		fmt.Sscanf(stopLossDollars, "%f", &dollars)

		// Query OANDA for accurate conversion factor
		priceMove, err := calculatePriceMoveForDollars(symbol, entryPrice, dollars, units, !isLong)
		if err != nil {
			log.Printf("⚠️  [WARNING] Failed to get accurate conversion, using simple calculation: %v", err)
			// Fallback to simple calculation (works for XXX_USD pairs)
			priceMove = dollars / float64(units)
		}

		if isLong {
			slPrice = entryPrice - priceMove
		} else {
			slPrice = entryPrice + priceMove
		}

		log.Printf("🛑 [SL CALC] $%.2f loss limit with %d units = %.5f price move", dollars, units, priceMove)
		log.Printf("🛑 [SL CALC] Entry: %.5f → SL: %.5f (%s)", entryPrice, slPrice, map[bool]string{true: "LONG", false: "SHORT"}[isLong])

	} else if stopLossPips != "" {
		// Calculate based on pips
		pips := 0.0
		fmt.Sscanf(stopLossPips, "%f", &pips)

		// Determine pip value based on instrument
		pipValue := 0.0001
		if strings.Contains(symbol, "JPY") {
			pipValue = 0.01
		}

		pipDistance := pips * pipValue

		if isLong {
			slPrice = entryPrice - pipDistance
		} else {
			slPrice = entryPrice + pipDistance
		}

		log.Printf("🛑 [SL CALC] %.0f pips = %.5f price distance", pips, pipDistance)
		log.Printf("🛑 [SL CALC] Entry: %.5f → SL: %.5f (%s)", entryPrice, slPrice, map[bool]string{true: "LONG", false: "SHORT"}[isLong])

	} else if stopLossPct != "" {
		// Calculate based on percentage
		pct := 0.0
		fmt.Sscanf(stopLossPct, "%f", &pct)

		priceMove := entryPrice * (pct / 100.0)

		if isLong {
			slPrice = entryPrice - priceMove
		} else {
			slPrice = entryPrice + priceMove
		}

		log.Printf("🛑 [SL CALC] %.2f%% = %.5f price distance", pct, priceMove)
		log.Printf("🛑 [SL CALC] Entry: %.5f → SL: %.5f (%s)", entryPrice, slPrice, map[bool]string{true: "LONG", false: "SHORT"}[isLong])
	}

	// Format price with appropriate precision
	precision := 5
	if strings.Contains(symbol, "JPY") {
		precision = 3
	}

	return fmt.Sprintf("%.*f", precision, slPrice), nil
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
			leverage, _, err := getInstrumentInfo(symbol)
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
			if takeProfitPips != "" || takeProfitPct != "" || takeProfitDollars != "" {
				tpPrice, err := calculateTakeProfitPrice(symbol, price, isLong, units)
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

			// Add stop loss if configured
			if stopLossPips != "" || stopLossPct != "" || stopLossDollars != "" {
				slPrice, err := calculateStopLossPrice(symbol, price, isLong, units)
				if err != nil {
					log.Printf("⚠️  [WARNING] Failed to calculate stop loss: %v", err)
				} else if slPrice != "" {
					orderSpec["stopLossOnFill"] = map[string]interface{}{
						"price":       slPrice,
						"timeInForce": "GTC",
					}
					log.Printf("🛑 [SL] Stop loss set at %s", slPrice)
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

	// Parse units as integer for TP calculation
	unitsInt := 0
	fmt.Sscanf(units, "%d", &unitsInt)
	if unitsInt < 0 {
		unitsInt = -unitsInt // Make positive for calculation
	}

	// Add take profit if configured
	if takeProfitPips != "" || takeProfitPct != "" || takeProfitDollars != "" {
		// Get current price for TP calculation
		price, err := getCurrentPrice(symbol)
		if err != nil {
			log.Printf("⚠️  [WARNING] Failed to get price for TP calculation: %v", err)
		} else {
			tpPrice, err := calculateTakeProfitPrice(symbol, price, isLong, unitsInt)
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

	// Add stop loss if configured
	if stopLossPips != "" || stopLossPct != "" || stopLossDollars != "" {
		// Get current price for SL calculation
		price, err := getCurrentPrice(symbol)
		if err != nil {
			log.Printf("⚠️  [WARNING] Failed to get price for SL calculation: %v", err)
		} else {
			slPrice, err := calculateStopLossPrice(symbol, price, isLong, unitsInt)
			if err != nil {
				log.Printf("⚠️  [WARNING] Failed to calculate stop loss: %v", err)
			} else if slPrice != "" {
				orderSpec["stopLossOnFill"] = map[string]interface{}{
					"price":       slPrice,
					"timeInForce": "GTC",
				}
				log.Printf("🛑 [SL] Stop loss set at %s", slPrice)
			}
		}
	}

	return orderSpec
}

// Query OANDA to determine why a trade closed (TP, SL, or manual)
func queryTradeCloseReason(tradeID string) (string, error) {
	// Get recent transactions for this account
	url := fmt.Sprintf("%s/v3/accounts/%s/transactions", oandaBaseURL, oandaAccountID)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+oandaAPIKey)
	req.Header.Set("Content-Type", "application/json")

	// Add query parameters to get recent transactions
	q := req.URL.Query()
	q.Add("count", "100") // Get last 100 transactions
	req.URL.RawQuery = q.Encode()

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch transactions: %v", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to parse transactions: %v", err)
	}

	transactions, ok := result["transactions"].([]interface{})
	if !ok {
		return "", fmt.Errorf("no transactions found")
	}

	// Look for the ORDER_FILL transaction that closed this trade
	for i := len(transactions) - 1; i >= 0; i-- {
		txn, ok := transactions[i].(map[string]interface{})
		if !ok {
			continue
		}

		txnType, _ := txn["type"].(string)
		if txnType != "ORDER_FILL" {
			continue
		}

		// Check if this ORDER_FILL is for our trade
		tradesClosed, ok := txn["tradesClosed"].([]interface{})
		if !ok || len(tradesClosed) == 0 {
			continue
		}

		for _, tc := range tradesClosed {
			tcMap, ok := tc.(map[string]interface{})
			if !ok {
				continue
			}

			closedTradeID, _ := tcMap["tradeID"].(string)
			if closedTradeID == tradeID {
				// Found the close transaction - determine the reason
				reason, _ := txn["reason"].(string)

				// OANDA reasons: "MARKET_ORDER_TRADE_CLOSE", "TAKE_PROFIT_ORDER", "STOP_LOSS_ORDER"
				if reason == "TAKE_PROFIT_ORDER" || strings.Contains(reason, "TAKE_PROFIT") {
					return "TAKE_PROFIT", nil
				} else if reason == "STOP_LOSS_ORDER" || strings.Contains(reason, "STOP_LOSS") {
					return "STOP_LOSS", nil
				} else {
					return "MANUAL", nil
				}
			}
		}
	}

	return "UNKNOWN", nil
}

// Check if any bot positions have been closed by OANDA (TP/SL hit)
func checkClosedPositions() {
	mu.RLock()
	// Collect currently open positions from bot state
	var botOpenTrades []struct {
		symbol  string
		tradeID string
	}

	for symbol, state := range positions {
		if state.PositionOpen && state.TradeID != "" && !state.IsSimulated {
			botOpenTrades = append(botOpenTrades, struct {
				symbol  string
				tradeID string
			}{symbol, state.TradeID})
		}
	}
	mu.RUnlock()

	if len(botOpenTrades) == 0 {
		return // No open positions to check
	}

	// Fetch current open trades from OANDA
	url := fmt.Sprintf("%s/v3/accounts/%s/openTrades", oandaBaseURL, oandaAccountID)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+oandaAPIKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("⚠️  [CHECK] Failed to fetch open trades: %v", err)
		return
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("⚠️  [CHECK] Failed to parse OANDA response: %v", err)
		return
	}

	// Build a set of OANDA's open trade IDs
	oandaTradeIDs := make(map[string]bool)
	if trades, ok := result["trades"].([]interface{}); ok {
		for _, trade := range trades {
			if tradeMap, ok := trade.(map[string]interface{}); ok {
				if tradeID, ok := tradeMap["id"].(string); ok {
					oandaTradeIDs[tradeID] = true
				}
			}
		}
	}

	// Check if any bot positions are no longer in OANDA
	for _, botTrade := range botOpenTrades {
		if !oandaTradeIDs[botTrade.tradeID] {
			// Trade was closed by OANDA - determine why
			closeReason, err := queryTradeCloseReason(botTrade.tradeID)
			if err != nil {
				log.Printf("⚠️  [CHECK] Could not determine close reason for %s: %v", botTrade.tradeID, err)
				closeReason = "UNKNOWN"
			}

			mu.Lock()
			state := positions[botTrade.symbol]
			positionType := state.Position

			// Update bot state
			state.PositionOpen = false
			state.Position = ""
			state.TradeID = ""
			state.EntryConditionsCompleted = make(map[string]bool)
			state.ExitConditionsCompleted = make(map[string]bool)
			resetIndicatorStates(state)

			// Clear entry conditions for all other symbols since only one position allowed at a time
			for sym, otherState := range positions {
				if sym != botTrade.symbol {
					otherState.EntryConditionsCompleted = make(map[string]bool)
					resetIndicatorStates(otherState)
					log.Printf("🧹 [CLEANUP] Cleared entry conditions and indicator states for %s (position closed on %s)", sym, botTrade.symbol)
				}
			}
			mu.Unlock()

			// Log based on closure reason
			if closeReason == "TAKE_PROFIT" {
				log.Println(strings.Repeat("🎯", 40))
				log.Printf("🎯 TAKE PROFIT HIT - %s %s", strings.ToUpper(positionType), botTrade.symbol)
				log.Println(strings.Repeat("🎯", 40))
				log.Printf("Trade ID: %s", botTrade.tradeID)
				log.Printf("Target: $%s profit", takeProfitDollars)
				log.Println(strings.Repeat("🎯", 40))
				log.Printf("✅ Position closed automatically by OANDA")
			} else if closeReason == "STOP_LOSS" {
				log.Println(strings.Repeat("🛑", 40))
				log.Printf("🛑 STOP LOSS HIT - %s %s", strings.ToUpper(positionType), botTrade.symbol)
				log.Println(strings.Repeat("🛑", 40))
				log.Printf("Trade ID: %s", botTrade.tradeID)
				log.Printf("Loss limit: $%s", stopLossDollars)
				log.Println(strings.Repeat("🛑", 40))
				log.Printf("✅ Position closed automatically by OANDA")
			} else {
				log.Printf("ℹ️  [SYNC] Position closed in OANDA: %s %s (ID: %s, Reason: %s)",
					strings.ToUpper(positionType), botTrade.symbol, botTrade.tradeID, closeReason)
			}

			log.Printf("💾 [STATE] Position updated - Open=false, Type='', TradeID='', Flags cleared")
		}
	}
}

// Reset all indicator states for a symbol (requires fresh webhooks to re-enter)
func resetIndicatorStates(state *PositionState) {
	state.MACDCrossedUp = false
	state.MACDCrossedDown = false
	state.StochInOversold = false
	state.StochInOverbought = false
	state.StochRSICrossedUp20 = false
	state.StochRSICrossedDown20 = false
	state.StochRSICrossedUp50 = false
	state.StochRSICrossedDown50 = false
	state.StochRSICrossedUp80 = false
	state.StochRSICrossedDown80 = false
	state.RSIAbove50 = false
	state.RSIBelow50 = false
	state.RSICrossedUp25 = false
	state.RSICrossedDown25 = false
	state.RSICrossedUp30 = false
	state.RSICrossedDown30 = false
	state.RSICrossedUp40 = false
	state.RSICrossedDown40 = false
	state.RSICrossedUp60 = false
	state.RSICrossedDown60 = false
	state.RSICrossedUp70 = false
	state.RSICrossedDown70 = false
	state.RSICrossedUp75 = false
	state.RSICrossedDown75 = false
	state.PriceAboveEMA9 = false
	state.PriceBelowEMA9 = false
	state.PriceAboveEMA50 = false
	state.PriceBelowEMA50 = false
	state.PriceAboveEMA20 = false
	state.PriceBelowEMA20 = false
	state.PriceAboveEMA200 = false
	state.PriceBelowEMA200 = false
	state.EMA9CrossedUpEMA21 = false
	state.EMA9CrossedDownEMA21 = false
	state.EMA9AboveEMA21 = false
	state.EMA9BelowEMA21 = false
	state.EMA9EMA21StateInitialized = false
	state.MA2AboveMA3 = false
	state.MA2BelowMA3 = false
	state.ATRAboveAverage = false
	state.ATRBelowAverage = false
	state.MACDHistIncreasing = false
	state.MACDHistDecreasing = false
	state.MACDHistAboveZero = false
	state.MACDHistBelowZero = false
	state.MARibbonBullish = false
	state.MARibbonBearish = false
	state.SMCLowLow = false
	state.SMCHighLow = false
	state.SMCLowHigh = false
	state.SMCHighHigh = false

	// Also clear entry/exit condition tracking since indicators are reset
	state.EntryConditionsCompleted = make(map[string]bool)
	state.ExitConditionsCompleted = make(map[string]bool)
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
	mu.Lock()
	state := positions[symbol]

	// Double-check position isn't already open (race condition protection)
	if state.PositionOpen {
		mu.Unlock()
		log.Printf("⚠️  [RACE] Position already open for %s - skipping duplicate open request", symbol)
		return
	}

	exchange := state.Exchange
	mu.Unlock()

	// Determine if this will be a simulated trade
	isSimulated := exchange != "OANDA" && exchange != ""

	// Check if any position is already open on the SAME exchange type
	mu.RLock()
	for sym, s := range positions {
		if s.PositionOpen {
			// Only block if same exchange type (both OANDA or both simulated)
			otherIsSimulated := s.Exchange != "OANDA" && s.Exchange != ""
			if isSimulated == otherIsSimulated {
				mu.RUnlock()
				log.Println(strings.Repeat("🚫", 40))
				log.Printf("🚫 [BLOCKED] Cannot open LONG on %s", symbol)
				log.Printf("   Reason: Position already open on %s (%s)", sym, strings.ToUpper(s.Position))
				log.Printf("   Action: Clearing entry conditions for %s to avoid false 'ready' state", symbol)
				log.Println(strings.Repeat("🚫", 40))

				// Clear entry conditions for this symbol since it can't open
				mu.Lock()
				blockedState := positions[symbol]
				blockedState.EntryConditionsCompleted = make(map[string]bool)
				mu.Unlock()
				return
			}
		}
	}
	mu.RUnlock()

	mu.Lock()
	state = positions[symbol]
	exchange = state.Exchange
	mu.Unlock()

	// Check if this is a non-OANDA exchange (simulated trade)
	if exchange != "OANDA" && exchange != "" {
		// Calculate position size with appropriate leverage for simulation
		var positionSize float64
		var positionUnits int

		// Determine leverage based on symbol type
		// Forex pairs typically use 50:1, crypto uses 10:1
		leverage := 10.0 // Default for crypto
		if strings.Contains(symbol, "_") {
			parts := strings.Split(symbol, "_")
			// Check if it's a forex pair (both are 3-letter currency codes)
			if len(parts) == 2 && len(parts[0]) == 3 && len(parts[1]) == 3 {
				leverage = 50.0 // Forex leverage
			}
		}

		priceFloat, err := strconv.ParseFloat(price, 64)
		if err != nil {
			log.Printf("❌ [ERROR] Failed to parse price for simulated trade: %v", err)
			priceFloat = 1.0
		}

		if tradeMargin != "" {
			marginAmount, err := strconv.ParseFloat(tradeMargin, 64)
			if err == nil {
				// Formula: margin_per_unit = price / leverage
				//          units = margin_amount / margin_per_unit
				marginPerUnit := priceFloat / leverage
				positionUnits = int(marginAmount / marginPerUnit)
				positionSize = float64(positionUnits) * priceFloat
				log.Printf("💱 [SIM MARGIN CALC] Price: %.5f, Leverage: %.0f:1", priceFloat, leverage)
				log.Printf("💱 [SIM MARGIN CALC] Margin per unit: $%.6f", marginPerUnit)
				log.Printf("💱 [SIM MARGIN CALC] Margin: $%.2f ÷ $%.6f = %d units", marginAmount, marginPerUnit, positionUnits)
				log.Printf("💱 [SIM POSITION SIZE] %d units × $%.5f = $%.2f position", positionUnits, priceFloat, positionSize)
			}
		} else if tradeUSDAmount != "" {
			usdAmount, err := strconv.ParseFloat(tradeUSDAmount, 64)
			if err == nil {
				positionUnits = int(usdAmount / priceFloat)
				positionSize = usdAmount
				log.Printf("💱 [SIM USD CALC] $%.2f ÷ $%.5f = %d units", usdAmount, priceFloat, positionUnits)
				log.Printf("💱 [SIM POSITION SIZE] $%.2f position", positionSize)
			}
		} else if tradeUnits != "" {
			units, err := strconv.Atoi(tradeUnits)
			if err == nil {
				positionUnits = units
				positionSize = float64(units) * priceFloat
				log.Printf("💱 [SIM UNITS CALC] %d units × $%.5f = $%.2f position", units, priceFloat, positionSize)
			}
		}

		mu.Lock()
		state.PositionOpen = true
		state.Position = "long"
		state.IsSimulated = true
		state.SimulatedEntry = formatTimeWithZone(getLocalTime())
		state.SimulatedPrice = price
		state.TradeID = fmt.Sprintf("SIM-%s-%d", symbol, time.Now().Unix())
		// Clear entry/exit tracking and reset all indicator states
		state.EntryConditionsCompleted = make(map[string]bool)
		state.ExitConditionsCompleted = make(map[string]bool)
		state.OppositeDirectionOccurred = false // Reset reversal tracking
		resetIndicatorStates(state)
		mu.Unlock()

		log.Println(strings.Repeat("🟢", 40))
		log.Printf("📊 SIMULATED LONG TRADE - %s", exchange)
		log.Println(strings.Repeat("🟢", 40))
		log.Printf("Symbol: %s", symbol)
		log.Printf("Entry Time: %s", state.SimulatedEntry)
		log.Printf("Entry Price: %s", price)
		if positionUnits > 0 {
			log.Printf("Position Size: %d units ($%.2f)", positionUnits, positionSize)
			log.Printf("Leverage: %.0f:1", leverage)
		}
		log.Printf("Trade ID: %s", state.TradeID)
		log.Println(strings.Repeat("🟢", 40))
		log.Println("⚠️  MANUAL ACTION REQUIRED: Mark this entry in TradingView")
		log.Println(strings.Repeat("🟢", 40))
		return
	}

	// Real OANDA trade
	orderSpec := getTradeSpec(symbol, true) // true = LONG

	orderData := map[string]interface{}{
		"order": orderSpec,
	}

	log.Printf("🟢 OPENING LONG: %s @ %s", symbol, price)

	tradeID, err := sendOandaOrder(orderData)
	if err != nil {
		log.Printf("❌ Failed to open LONG position: %v", err)
		return
	}

	mu.Lock()
	state = positions[symbol]
	state.PositionOpen = true
	state.Position = "long"
	state.TradeID = tradeID
	state.IsSimulated = false
	// Clear entry/exit tracking and reset all indicator states
	state.EntryConditionsCompleted = make(map[string]bool)
	state.ExitConditionsCompleted = make(map[string]bool)
	state.OppositeDirectionOccurred = false // Reset reversal tracking
	resetIndicatorStates(state)
	mu.Unlock()

	log.Printf("✅ LONG position opened: %s (ID: %s)", symbol, tradeID)
	log.Printf("💾 [STATE] Position updated - Open=%v, Type=%s, TradeID=%s, All flags cleared",
		state.PositionOpen, state.Position, state.TradeID)
}

func openShortPosition(symbol string, price string) {
	mu.Lock()
	state := positions[symbol]

	// Double-check position isn't already open (race condition protection)
	if state.PositionOpen {
		mu.Unlock()
		log.Printf("⚠️  [RACE] Position already open for %s - skipping duplicate open request", symbol)
		return
	}

	exchange := state.Exchange
	mu.Unlock()

	// Determine if this will be a simulated trade
	isSimulated := exchange != "OANDA" && exchange != ""

	// Check if any position is already open on the SAME exchange type
	mu.RLock()
	for sym, s := range positions {
		if s.PositionOpen {
			// Only block if same exchange type (both OANDA or both simulated)
			otherIsSimulated := s.Exchange != "OANDA" && s.Exchange != ""
			if isSimulated == otherIsSimulated {
				mu.RUnlock()
				log.Println(strings.Repeat("🚫", 40))
				log.Printf("🚫 [BLOCKED] Cannot open SHORT on %s", symbol)
				log.Printf("   Reason: Position already open on %s (%s)", sym, strings.ToUpper(s.Position))
				log.Printf("   Action: Clearing entry conditions for %s to avoid false 'ready' state", symbol)
				log.Println(strings.Repeat("🚫", 40))

				// Clear entry conditions for this symbol since it can't open
				mu.Lock()
				blockedState := positions[symbol]
				blockedState.EntryConditionsCompleted = make(map[string]bool)
				mu.Unlock()
				return
			}
		}
	}
	mu.RUnlock()

	mu.Lock()
	state = positions[symbol]
	exchange = state.Exchange
	mu.Unlock() // Check if this is a non-OANDA exchange (simulated trade)
	if exchange != "OANDA" && exchange != "" {
		// Calculate position size with appropriate leverage for simulation
		var positionSize float64
		var positionUnits int

		// Determine leverage based on symbol type
		// Forex pairs typically use 50:1, crypto uses 10:1
		leverage := 10.0 // Default for crypto
		if strings.Contains(symbol, "_") {
			parts := strings.Split(symbol, "_")
			// Check if it's a forex pair (both are 3-letter currency codes)
			if len(parts) == 2 && len(parts[0]) == 3 && len(parts[1]) == 3 {
				leverage = 50.0 // Forex leverage
			}
		}

		priceFloat, err := strconv.ParseFloat(price, 64)
		if err != nil {
			log.Printf("❌ [ERROR] Failed to parse price for simulated trade: %v", err)
			priceFloat = 1.0
		}

		if tradeMargin != "" {
			marginAmount, err := strconv.ParseFloat(tradeMargin, 64)
			if err == nil {
				// Formula: margin_per_unit = price / leverage
				//          units = margin_amount / margin_per_unit
				marginPerUnit := priceFloat / leverage
				positionUnits = int(marginAmount / marginPerUnit)
				positionSize = float64(positionUnits) * priceFloat
				log.Printf("💱 [SIM MARGIN CALC] Price: %.5f, Leverage: %.0f:1", priceFloat, leverage)
				log.Printf("💱 [SIM MARGIN CALC] Margin per unit: $%.6f", marginPerUnit)
				log.Printf("💱 [SIM MARGIN CALC] Margin: $%.2f ÷ $%.6f = %d units", marginAmount, marginPerUnit, positionUnits)
				log.Printf("💱 [SIM POSITION SIZE] %d units × $%.5f = $%.2f position", positionUnits, priceFloat, positionSize)
			}
		} else if tradeUSDAmount != "" {
			usdAmount, err := strconv.ParseFloat(tradeUSDAmount, 64)
			if err == nil {
				positionUnits = int(usdAmount / priceFloat)
				positionSize = usdAmount
				log.Printf("💱 [SIM USD CALC] $%.2f ÷ $%.5f = %d units", usdAmount, priceFloat, positionUnits)
				log.Printf("💱 [SIM POSITION SIZE] $%.2f position", positionSize)
			}
		} else if tradeUnits != "" {
			units, err := strconv.Atoi(tradeUnits)
			if err == nil {
				positionUnits = units
				positionSize = float64(units) * priceFloat
				log.Printf("💱 [SIM UNITS CALC] %d units × $%.5f = $%.2f position", units, priceFloat, positionSize)
			}
		}

		mu.Lock()
		state.PositionOpen = true
		state.Position = "short"
		state.IsSimulated = true
		state.SimulatedEntry = formatTimeWithZone(getLocalTime())
		state.SimulatedPrice = price
		state.TradeID = fmt.Sprintf("SIM-%s-%d", symbol, time.Now().Unix())
		// Clear entry/exit tracking and reset all indicator states
		state.EntryConditionsCompleted = make(map[string]bool)
		state.ExitConditionsCompleted = make(map[string]bool)
		state.OppositeDirectionOccurred = false // Reset reversal tracking
		resetIndicatorStates(state)
		mu.Unlock()

		log.Println(strings.Repeat("🔴", 40))
		log.Printf("📊 SIMULATED SHORT TRADE - %s", exchange)
		log.Println(strings.Repeat("🔴", 40))
		log.Printf("Symbol: %s", symbol)
		log.Printf("Entry Time: %s", state.SimulatedEntry)
		log.Printf("Entry Price: %s", price)
		if positionUnits > 0 {
			log.Printf("Position Size: %d units ($%.2f)", positionUnits, positionSize)
			log.Printf("Leverage: %.0f:1", leverage)
		}
		log.Printf("Trade ID: %s", state.TradeID)
		log.Println(strings.Repeat("🔴", 40))
		log.Println("⚠️  MANUAL ACTION REQUIRED: Mark this entry in TradingView")
		log.Println(strings.Repeat("🔴", 40))
		return
	}

	// Real OANDA trade
	orderSpec := getTradeSpec(symbol, false) // false = SHORT

	orderData := map[string]interface{}{
		"order": orderSpec,
	}

	log.Printf("🔴 OPENING SHORT: %s @ %s", symbol, price)

	tradeID, err := sendOandaOrder(orderData)
	if err != nil {
		log.Printf("❌ Failed to open SHORT position: %v", err)
		return
	}

	mu.Lock()
	state = positions[symbol]
	state.PositionOpen = true
	state.Position = "short"
	state.TradeID = tradeID
	state.IsSimulated = false
	// Clear entry/exit tracking and reset all indicator states
	state.EntryConditionsCompleted = make(map[string]bool)
	state.ExitConditionsCompleted = make(map[string]bool)
	state.OppositeDirectionOccurred = false // Reset reversal tracking
	resetIndicatorStates(state)
	mu.Unlock()

	log.Printf("✅ SHORT position opened: %s (ID: %s)", symbol, tradeID)
	log.Printf("💾 [STATE] Position updated - Open=%v, Type=%s, TradeID=%s, All flags cleared",
		state.PositionOpen, state.Position, state.TradeID)
}

func closePosition(symbol string) {
	state := getPositionState(symbol)

	mu.RLock()
	tradeID := state.TradeID
	position := state.Position
	isSimulated := state.IsSimulated
	exchange := state.Exchange
	entryTime := state.SimulatedEntry
	entryPrice := state.SimulatedPrice
	mu.RUnlock()

	if tradeID == "" {
		log.Printf("⚠️  No trade ID found for %s", symbol)
		return
	}

	// Handle simulated trade close
	if isSimulated {
		exitTime := formatTimeWithZone(getLocalTime())

		// Calculate P/L
		var plDollars float64
		var plPercent float64
		if entryPrice != "" && state.LatestPrice != "" {
			entryPriceFloat, _ := strconv.ParseFloat(entryPrice, 64)
			exitPriceFloat, _ := strconv.ParseFloat(state.LatestPrice, 64)

			if position == "long" {
				plDollars = exitPriceFloat - entryPriceFloat
			} else {
				plDollars = entryPriceFloat - exitPriceFloat
			}
			plPercent = (plDollars / entryPriceFloat) * 100
		}

		plColor := "🟢"
		plSign := "+"
		if plDollars < 0 {
			plColor = "🔴"
			plSign = ""
		}

		log.Println(strings.Repeat("🔵", 40))
		log.Printf("📊 SIMULATED %s TRADE CLOSED - %s", strings.ToUpper(position), exchange)
		log.Println(strings.Repeat("🔵", 40))
		log.Printf("Symbol: %s", symbol)
		log.Printf("Entry Time: %s", entryTime)
		log.Printf("Entry Price: %s", entryPrice)
		log.Printf("Exit Time: %s", exitTime)
		log.Printf("Exit Price: %s", state.LatestPrice)
		if plDollars != 0 {
			log.Printf("Final P/L: %s %s$%.5f / %s%.2f%%", plColor, plSign, plDollars, plSign, plPercent)
		}
		log.Printf("Trade ID: %s", tradeID)
		log.Println(strings.Repeat("🔵", 40))
		log.Println("⚠️  MANUAL ACTION REQUIRED: Mark this exit in TradingView")
		log.Println(strings.Repeat("🔵", 40))

		mu.Lock()
		state.LastClosedDirection = position // Track which direction was closed
		state.PositionOpen = false
		state.Position = ""
		state.TradeID = ""
		state.SimulatedExit = exitTime
		state.SimulatedExitPrice = state.LatestPrice
		// Store final P/L for summary display
		if plDollars != 0 {
			state.SimulatedPL = fmt.Sprintf("%s %s$%.5f / %s%.2f%%", plColor, plSign, plDollars, plSign, plPercent)
		}
		// Clear exit tracking but KEEP entry condition states (don't clear indicator flags)
		state.ExitConditionsCompleted = make(map[string]bool)

		// Clear entry conditions for all other symbols since only one position allowed at a time
		for sym, otherState := range positions {
			if sym != symbol {
				otherState.EntryConditionsCompleted = make(map[string]bool)
				log.Printf("🧹 [CLEANUP] Cleared entry conditions for %s (position closed on %s)", sym, symbol)
			}
		}
		mu.Unlock()

		// Check if we should immediately re-enter with reversal protection
		log.Printf("✅ [EXIT] Position closed - checking if conditions met for re-entry...")

		// Check LONG entry (only if last close was SHORT or opposite direction occurred)
		canEnterLong := state.LastClosedDirection == "" || state.LastClosedDirection == "short" || state.OppositeDirectionOccurred
		if canEnterLong && shouldOpenPosition(symbol, true, nil) {
			log.Printf("✅ [TRADE] LONG entry conditions still met after close + reversal requirement satisfied → Opening LONG")
			openLongPosition(symbol, state.LatestPrice)
			return
		}

		// Check SHORT entry (only if last close was LONG or opposite direction occurred)
		canEnterShort := state.LastClosedDirection == "" || state.LastClosedDirection == "long" || state.OppositeDirectionOccurred
		if canEnterShort && shouldOpenPosition(symbol, false, nil) {
			log.Printf("✅ [TRADE] SHORT entry conditions still met after close + reversal requirement satisfied → Opening SHORT")
			openShortPosition(symbol, state.LatestPrice)
			return
		}

		log.Printf("⏳ [ENTRY] No immediate re-entry: reversal requirement not met or conditions not satisfied")

		return
	}

	// Real OANDA trade close
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
		state.LastClosedDirection = position // Track which direction was closed
		state.PositionOpen = false
		state.Position = "none"
		state.TradeID = ""
		// Clear exit tracking but KEEP entry condition states (don't clear indicator flags)
		state.ExitConditionsCompleted = make(map[string]bool)

		// Clear entry conditions for all other symbols since only one position allowed at a time
		for sym, otherState := range positions {
			if sym != symbol {
				otherState.EntryConditionsCompleted = make(map[string]bool)
				log.Printf("🧹 [CLEANUP] Cleared entry conditions for %s (position closed on %s)", sym, symbol)
			}
		}
		mu.Unlock()

		log.Printf("✅ Position closed: %s", symbol)
		log.Printf("💾 [STATE] Position updated - Open=%v, Type=%s", state.PositionOpen, state.Position)

		// Check if we should immediately re-enter with reversal protection
		log.Printf("🔍 [ENTRY] Checking if conditions met for re-entry with reversal protection...")

		// Check LONG entry (only if last close was SHORT or opposite direction occurred)
		canEnterLong := state.LastClosedDirection == "" || state.LastClosedDirection == "short" || state.OppositeDirectionOccurred
		if canEnterLong && shouldOpenPosition(symbol, true, nil) {
			log.Printf("✅ [TRADE] LONG entry conditions met + reversal requirement satisfied → Opening LONG")
			openLongPosition(symbol, "")
		} else if !canEnterLong && shouldOpenPosition(symbol, true, nil) {
			log.Printf("🚫 [BLOCKED] LONG conditions met but reversal requirement not satisfied (last close: %s, opposite occurred: %v)", state.LastClosedDirection, state.OppositeDirectionOccurred)
		}

		// Check SHORT entry (only if last close was LONG or opposite direction occurred)
		canEnterShort := state.LastClosedDirection == "" || state.LastClosedDirection == "long" || state.OppositeDirectionOccurred
		if canEnterShort && shouldOpenPosition(symbol, false, nil) {
			log.Printf("✅ [TRADE] SHORT entry conditions met + reversal requirement satisfied → Opening SHORT")
			openShortPosition(symbol, "")
		} else if !canEnterShort && shouldOpenPosition(symbol, false, nil) {
			log.Printf("🚫 [BLOCKED] SHORT conditions met but reversal requirement not satisfied (last close: %s, opposite occurred: %v)", state.LastClosedDirection, state.OppositeDirectionOccurred)
		}

		if !shouldOpenPosition(symbol, true, nil) && !shouldOpenPosition(symbol, false, nil) {
			log.Printf("⏳ [ENTRY] No entry conditions met yet")
		}
	} else {
		// Read response body for error details
		var responseBody map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&responseBody)
		log.Printf("❌ Failed to close position (status %d)", resp.StatusCode)
		log.Printf("📄 [RESPONSE] %+v", responseBody)
	}
}

// Get leverage (margin rate) for an instrument from OANDA
func getOandaLeverage(symbol string) float64 {
	url := fmt.Sprintf("%s/v3/accounts/%s/instruments/%s", oandaBaseURL, oandaAccountID, symbol)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Printf("❌ [LEVERAGE] Failed to create request: %v", err)
		return 50.0 // Default forex leverage
	}

	req.Header.Set("Authorization", "Bearer "+oandaAPIKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("❌ [LEVERAGE] API request failed: %v", err)
		return 50.0
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Printf("⚠️  [LEVERAGE] API returned status %d for %s", resp.StatusCode, symbol)
		return 50.0
	}

	var result struct {
		Instruments []struct {
			Name       string `json:"name"`
			MarginRate string `json:"marginRate"`
		} `json:"instruments"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("❌ [LEVERAGE] Failed to decode response: %v", err)
		return 50.0
	}

	if len(result.Instruments) > 0 {
		marginRate, err := strconv.ParseFloat(result.Instruments[0].MarginRate, 64)
		if err != nil {
			log.Printf("❌ [LEVERAGE] Failed to parse margin rate: %v", err)
			return 50.0
		}
		// Leverage = 1 / marginRate
		// e.g., marginRate of 0.02 = 50:1 leverage (1 / 0.02 = 50)
		leverage := 1.0 / marginRate
		log.Printf("📊 [LEVERAGE] %s: Margin Rate = %s, Leverage = %.0f:1", symbol, result.Instruments[0].MarginRate, leverage)
		return leverage
	}

	log.Printf("⚠️  [LEVERAGE] No instrument data for %s, using default 50:1", symbol)
	return 50.0
}

func sendOandaOrder(orderData map[string]interface{}) (string, error) {
	url := fmt.Sprintf("%s/v3/accounts/%s/orders", oandaBaseURL, oandaAccountID)

	jsonData, _ := json.MarshalIndent(orderData, "", "  ")

	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	req.Header.Set("Authorization", "Bearer "+oandaAPIKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("❌ API ERROR: %v", err)
		return "", err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if orderFill, ok := result["orderFillTransaction"].(map[string]interface{}); ok {
			if tradeOpened, ok := orderFill["tradeOpened"].(map[string]interface{}); ok {
				if tradeID, ok := tradeOpened["tradeID"].(string); ok {
					log.Printf("✅ ORDER FILLED - Trade ID: %s", tradeID)
					return tradeID, nil
				}
			}
		}

		log.Printf("⚠️  Could not extract trade ID, using 'unknown'")
		return "unknown", nil
	}

	log.Printf("❌ ORDER REJECTED (Status %d): %+v", resp.StatusCode, result)
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
	// Check if TradingView sent the template literally without replacing it
	if ticker == "{{ticker}}" || ticker == "{{ ticker }}" || ticker == "{{Ticker}}" {
		log.Printf("⚠️  [WARNING] TradingView sent literal template variable: %s (template not replaced)", ticker)
		return ""
	}

	// Simple conversion - adjust as needed
	if len(ticker) == 6 {
		return ticker[:3] + "_" + ticker[3:]
	}
	return ticker
}

// validateSymbol checks if symbol is valid and returns true if webhook should continue
func validateSymbol(w http.ResponseWriter, symbol string) bool {
	if symbol == "" {
		log.Printf("⚠️  [IGNORED] Webhook ignored - invalid or template ticker value")
		respondSuccess(w, "Invalid ticker - webhook ignored")
		return false
	}
	return true
}

// getLocalTime returns current time adjusted for timezone offset
func getLocalTime() time.Time {
	return time.Now().UTC().Add(time.Duration(timezoneOffset) * time.Hour)
}

// formatTimeWithZone formats time with timezone label
func formatTimeWithZone(t time.Time) string {
	if timezoneOffset == 0 {
		return t.Format("2006-01-02 03:04:05 PM") + " UTC"
	} else if timezoneOffset > 0 {
		return t.Format("2006-01-02 03:04:05 PM") + fmt.Sprintf(" UTC+%d", timezoneOffset)
	} else {
		return t.Format("2006-01-02 03:04:05 PM") + fmt.Sprintf(" UTC%d", timezoneOffset)
	}
}

func respondSuccess(w http.ResponseWriter, message string) {
	// Show status report after each webhook event
	reportStrategyStatus()

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

// reportStrategyStatus prints a periodic status update showing which entry conditions are met
func reportStrategyStatus() {
	log.Println(strings.Repeat("=", 80))
	log.Printf("📊 STRATEGY STATUS REPORT - %s", formatTimeWithZone(getLocalTime()))
	log.Println(strings.Repeat("=", 80))
	log.Printf("Strategy: %s", activeStrategy.Name)
	log.Println("")

	mu.RLock()
	positionsCount := len(positions)
	mu.RUnlock()

	// If no positions have been created yet, show a default symbol report
	if positionsCount == 0 {
		log.Println("⚠️  No webhook events received yet. Waiting for first signal...")
		log.Println("")
		log.Println("Expected conditions for entry:")

		// Create a dummy state to show what conditions are expected
		dummyState := &PositionState{
			Symbol:                   "Waiting for symbol...",
			PositionOpen:             false,
			Position:                 "none",
			EntryConditionsCompleted: make(map[string]bool),
			ExitConditionsCompleted:  make(map[string]bool),
		}

		// Report LONG entry conditions
		if activeStrategy.Long != nil {
			reportEntryConditions("N/A", "LONG", &activeStrategy.Long.Entry, "long_", dummyState)
		} else if activeStrategy.Entry != nil {
			reportEntryConditions("N/A", "LONG", activeStrategy.Entry, "", dummyState)
		}

		log.Println("")

		// Report SHORT entry conditions
		if activeStrategy.Short != nil {
			reportEntryConditions("N/A", "SHORT", &activeStrategy.Short.Entry, "short_", dummyState)
		} else if activeStrategy.Entry != nil {
			reportEntryConditions("N/A", "SHORT", activeStrategy.Entry, "", dummyState)
		}

		log.Println("")
		log.Println(strings.Repeat("=", 80))
		return
	}

	mu.RLock()
	defer mu.RUnlock()

	// Get symbols and sort them for consistent display order
	symbols := make([]string, 0, len(positions))
	for symbol := range positions {
		symbols = append(symbols, symbol)
	}
	sort.Strings(symbols)

	// Report for each symbol in alphabetical order
	for _, symbol := range symbols {
		state := positions[symbol]
		exchangeTag := ""
		if state.Exchange != "" {
			exchangeTag = fmt.Sprintf(" [%s]", state.Exchange)
		}
		log.Printf("Symbol: %s%s", symbol, exchangeTag)

		if state.PositionOpen {
			// Visual indicator for open position
			if state.IsSimulated {
				log.Println("  ┌─────────────────────────────────────────────────────────────────────┐")
				log.Printf("  │ 🔄 SIMULATED %s POSITION - %s%-29s│",
					strings.ToUpper(state.Position), state.Exchange, "")
				log.Printf("  │ Entry: %s @ %s%-33s│",
					state.SimulatedEntry, state.SimulatedPrice, "")

				// Calculate and display P/L if we have latest price
				if state.LatestPrice != "" && state.SimulatedPrice != "" {
					entryPrice, err1 := strconv.ParseFloat(state.SimulatedPrice, 64)
					currentPrice, err2 := strconv.ParseFloat(state.LatestPrice, 64)
					if err1 == nil && err2 == nil {
						var plDollars, plPercent float64
						if state.Position == "long" {
							plDollars = currentPrice - entryPrice
							plPercent = (plDollars / entryPrice) * 100
						} else {
							plDollars = entryPrice - currentPrice
							plPercent = (plDollars / entryPrice) * 100
						}

						plColor := "🟢"
						plSign := "+"
						if plDollars < 0 {
							plColor = "🔴"
							plSign = ""
						}

						log.Printf("  │ Current: %s (%s P/L: %s$%.5f / %s%.2f%%)%-15s│",
							state.LatestPrice, plColor, plSign, plDollars, plSign, plPercent, "")
					}
				}

				log.Printf("  │ Trade ID: %s%-48s│", state.TradeID, "")
				log.Println("  └─────────────────────────────────────────────────────────────────────┘")
			} else {
				log.Println("  ┌─────────────────────────────────────────────────────────────────────┐")
				log.Printf("  │ 📈 POSITION OPEN: %s (Trade ID: %s)%-24s│",
					strings.ToUpper(state.Position), state.TradeID, "")
				log.Println("  └─────────────────────────────────────────────────────────────────────┘")
			}
			log.Println("") // Show ONLY exit conditions for the current position
			if state.Position == "long" {
				if activeStrategy.Long != nil {
					reportExitConditions(symbol, "LONG", &activeStrategy.Long.Exit, "long_", state)
				} else if activeStrategy.Exit != nil {
					reportExitConditions(symbol, "LONG", activeStrategy.Exit, "", state)
				}
			} else if state.Position == "short" {
				if activeStrategy.Short != nil {
					reportExitConditions(symbol, "SHORT", &activeStrategy.Short.Exit, "short_", state)
				} else if activeStrategy.Exit != nil {
					reportExitConditions(symbol, "SHORT", activeStrategy.Exit, "", state)
				}
			}
		} else {
			exchangeInfo := ""
			if state.Exchange != "" {
				exchangeInfo = fmt.Sprintf(" [%s]", state.Exchange)
			}
			log.Printf("  Position: NONE%s", exchangeInfo)
			log.Println("")

			// Show entry conditions for both directions when no position is open
			// Report LONG entry conditions
			if activeStrategy.Long != nil {
				reportEntryConditions(symbol, "LONG", &activeStrategy.Long.Entry, "long_", state)
			} else if activeStrategy.Entry != nil {
				reportEntryConditions(symbol, "LONG", activeStrategy.Entry, "", state)
			}

			log.Println("")

			// Report SHORT entry conditions
			if activeStrategy.Short != nil {
				reportEntryConditions(symbol, "SHORT", &activeStrategy.Short.Entry, "short_", state)
			} else if activeStrategy.Entry != nil {
				reportEntryConditions(symbol, "SHORT", activeStrategy.Entry, "", state)
			}
		}

		log.Println("")
	}

	// Show recently closed positions
	closedPositions := make([]string, 0)
	for _, symbol := range symbols {
		state := positions[symbol]
		if !state.PositionOpen && state.SimulatedExit != "" {
			closedPositions = append(closedPositions, symbol)
		}
	}

	if len(closedPositions) > 0 {
		log.Println(strings.Repeat("-", 80))
		log.Printf("📜 CLOSED POSITIONS (Last Session)")
		log.Println(strings.Repeat("-", 80))

		for _, symbol := range closedPositions {
			state := positions[symbol]
			log.Printf("Symbol: %s [%s]", symbol, state.Exchange)
			log.Printf("  Entry:  %s @ %s", state.SimulatedEntry, state.SimulatedPrice)
			log.Printf("  Exit:   %s @ %s", state.SimulatedExit, state.SimulatedExitPrice)
			if state.SimulatedPL != "" {
				log.Printf("  P/L:    %s", state.SimulatedPL)
			}
			log.Println("")
		}
	}

	log.Println(strings.Repeat("=", 80))
}

// reportEntryConditions reports the status of entry conditions for a position direction
func reportEntryConditions(symbol string, direction string, entryConditions *EntryConditions, prefix string, state *PositionState) {
	log.Printf("  %s Entry Conditions (%s mode):", direction, entryConditions.Combination)

	metCount := 0
	totalCount := len(entryConditions.Conditions)

	for i, condition := range entryConditions.Conditions {
		// Check if condition is currently met based on actual state, not just completion tracking
		var isMet bool
		if condition.Type == "condition" && condition.Webhook != "" {
			isMet = isConditionCurrentlyMet(condition.Webhook, state)
		} else {
			// For groups or unknown types, fall back to completion tracking
			key := fmt.Sprintf("%scondition_%d", prefix, i)
			isMet = state.EntryConditionsCompleted[key]
		}

		if isMet {
			metCount++
		}

		status := "❌"
		if isMet {
			status = "✅"
		}

		description := getNodeDescription(&condition)
		log.Printf("    %s [%d/%d] %s", status, i+1, totalCount, description)
	}

	log.Printf("  Summary: %d/%d conditions met", metCount, totalCount)

	if metCount == totalCount && totalCount > 0 {
		log.Printf("  🎯 READY: All %s entry conditions are satisfied!", direction)
	} else if metCount > 0 {
		log.Printf("  ⏳ WAITING: Need %d more condition(s) for %s entry", totalCount-metCount, direction)
	} else {
		log.Printf("  💤 IDLE: No %s entry conditions met yet", direction)
	}
}

// reportExitConditions reports the status of exit conditions for a position direction
func reportExitConditions(symbol string, direction string, exitConditions *ExitConditions, prefix string, state *PositionState) {
	log.Printf("  %s Exit Conditions (%s mode):", direction, exitConditions.Combination)

	metCount := 0
	totalCount := len(exitConditions.Conditions)

	for i, condition := range exitConditions.Conditions {
		// Check if condition is currently met based on actual state, not just completion tracking
		var isMet bool
		if condition.Type == "condition" && condition.Webhook != "" {
			isMet = isConditionCurrentlyMet(condition.Webhook, state)
		} else {
			// For groups or unknown types, fall back to completion tracking
			key := fmt.Sprintf("%scondition_%d", prefix, i)
			isMet = state.ExitConditionsCompleted[key]
		}

		if isMet {
			metCount++
		}

		status := "❌"
		if isMet {
			status = "✅"
		}

		description := getNodeDescription(&condition)
		log.Printf("    %s [%d/%d] %s", status, i+1, totalCount, description)
	}

	log.Printf("  Summary: %d/%d conditions met", metCount, totalCount)

	// For exit conditions, interpretation depends on combination mode
	if exitConditions.Combination == "any" {
		if metCount > 0 {
			log.Printf("  🚨 TRIGGERED: Exit signal active (%s mode - any condition triggers exit)", exitConditions.Combination)
		} else {
			log.Printf("  ✅ SAFE: No exit conditions met yet")
		}
	} else if exitConditions.Combination == "all" {
		if metCount == totalCount && totalCount > 0 {
			log.Printf("  🚨 TRIGGERED: All exit conditions met - position should close!")
		} else if metCount > 0 {
			log.Printf("  ⚠️  PARTIAL: %d/%d exit conditions met (all required for %s mode)", metCount, totalCount, exitConditions.Combination)
		} else {
			log.Printf("  ✅ SAFE: No exit conditions met yet")
		}
	} else if exitConditions.Combination == "sequential" {
		if metCount == totalCount && totalCount > 0 {
			log.Printf("  🚨 TRIGGERED: All sequential exit conditions completed - position should close!")
		} else if metCount > 0 {
			log.Printf("  ⚠️  PARTIAL: %d/%d sequential exit conditions met", metCount, totalCount)
		} else {
			log.Printf("  ✅ SAFE: No exit conditions met yet")
		}
	}
}

// extractWebhooks extracts webhook paths from a list of conditions
func extractWebhooks(conditions []ConditionNode) []string {
	var webhooks []string
	for _, condition := range conditions {
		if condition.Type == "condition" && condition.Webhook != "" {
			webhooks = append(webhooks, condition.Webhook)
		} else if condition.Type == "group" && len(condition.Conditions) > 0 {
			// Recursively extract from nested groups
			webhooks = append(webhooks, extractWebhooks(condition.Conditions)...)
		}
	}
	return webhooks
}

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

	// Stop loss configuration
	stopLossPips = os.Getenv("STOP_LOSS_PIPS")
	stopLossPct = os.Getenv("STOP_LOSS_PCT")
	stopLossDollars = os.Getenv("STOP_LOSS_DOLLARS")

	// Strategy configuration
	strategyName = os.Getenv("STRATEGY_FILE")
	if strategyName == "" {
		strategyName = "default" // Use default strategy if not specified
	}

	// Timezone configuration
	timezoneOffsetStr := os.Getenv("TIMEZONE_OFFSET")
	if timezoneOffsetStr != "" {
		if offset, err := strconv.Atoi(timezoneOffsetStr); err == nil {
			timezoneOffset = offset
			log.Printf("⏰ Timezone offset: UTC%+d", timezoneOffset)
		} else {
			log.Printf("⚠️  Invalid TIMEZONE_OFFSET value '%s', using UTC (0)", timezoneOffsetStr)
			timezoneOffset = 0
		}
	} else {
		timezoneOffset = 0 // Default to UTC
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
	} else if takeProfitDollars != "" {
		log.Printf("🎯 Take Profit: $%s", takeProfitDollars)
	} else {
		log.Printf("🎯 Take Profit: None (manual exit only)")
	}

	// Show stop loss settings
	if stopLossPips != "" {
		log.Printf("🛑 Stop Loss: %s pips", stopLossPips)
	} else if stopLossPct != "" {
		log.Printf("🛑 Stop Loss: %s%%", stopLossPct)
	} else if stopLossDollars != "" {
		log.Printf("🛑 Stop Loss: $%s", stopLossDollars)
	} else {
		log.Printf("🛑 Stop Loss: None")
	}

	// Sync existing positions from OANDA on startup
	if err := syncPositionsFromOanda(); err != nil {
		log.Printf("⚠️  [WARNING] Could not sync positions from OANDA: %v", err)
		log.Printf("⚠️  [WARNING] Continuing with empty state - be careful of duplicate positions!")
	}

	// Health & Status
	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/status", statusHandler)

	// RSI Specific Level Webhooks
	http.HandleFunc("/webhook/rsi/cross-up-oversell-25", handleRSICrossUpOversell25)
	http.HandleFunc("/webhook/rsi/cross-oversell-30", handleRSICrossOversell30)
	http.HandleFunc("/webhook/rsi/cross-40", handleRSICross40)
	http.HandleFunc("/webhook/rsi/cross-center-50", handleRSICrossCenter50)
	http.HandleFunc("/webhook/rsi/cross-60", handleRSICross60)
	http.HandleFunc("/webhook/rsi/cross-overbuy-70", handleRSICrossOverbuy70)
	http.HandleFunc("/webhook/rsi/cross-down-overbuy-75", handleRSICrossDownOverbuy75)

	// MACD Webhooks
	http.HandleFunc("/webhook/macd/cross-up", handleMACDCrossUp)
	http.HandleFunc("/webhook/macd/cross-down", handleMACDCrossDown)

	// Stochastic Webhooks
	http.HandleFunc("/webhook/stochastic/oversold", handleStochasticOversold)
	http.HandleFunc("/webhook/stochastic/overbought", handleStochasticOverbought)

	// Stochastic RSI Webhooks
	http.HandleFunc("/webhook/stochastic-rsi/cross-up-20", handleStochRSICrossUp20)
	http.HandleFunc("/webhook/stochastic-rsi/cross-down-20", handleStochRSICrossDown20)
	http.HandleFunc("/webhook/stochastic-rsi/cross-up-50", handleStochRSICrossUp50)
	http.HandleFunc("/webhook/stochastic-rsi/cross-down-50", handleStochRSICrossDown50)
	http.HandleFunc("/webhook/stochastic-rsi/cross-up-80", handleStochRSICrossUp80)
	http.HandleFunc("/webhook/stochastic-rsi/cross-down-80", handleStochRSICrossDown80)
	http.HandleFunc("/webhook/stochastic-rsi/oversold", handleStochRSIOversold)
	http.HandleFunc("/webhook/stochastic-rsi/overbought", handleStochRSIOverbought)

	// RSI Trend Webhooks
	http.HandleFunc("/webhook/rsi/above-50", handleRSIAbove50)
	http.HandleFunc("/webhook/rsi/below-50", handleRSIBelow50)

	// EMA Trend Webhooks
	http.HandleFunc("/webhook/ema/price-above-ema20", handlePriceAboveEMA20)
	http.HandleFunc("/webhook/ema/price-below-ema20", handlePriceBelowEMA20)
	http.HandleFunc("/webhook/ema/price-above-ema50", handlePriceAboveEMA50)
	http.HandleFunc("/webhook/ema/price-below-ema50", handlePriceBelowEMA50)
	http.HandleFunc("/webhook/ema/price-above-ema200", handlePriceAboveEMA200)
	http.HandleFunc("/webhook/ema/price-below-ema200", handlePriceBelowEMA200)

	// MA Ribbon Webhooks (Generic MA#1-4)
	http.HandleFunc("/webhook/ma/price-above-ma4", handlePriceAboveMA4)
	http.HandleFunc("/webhook/ma/price-below-ma4", handlePriceBelowMA4)
	http.HandleFunc("/webhook/ma/price-above-ma2", handlePriceAboveMA2)
	http.HandleFunc("/webhook/ma/price-below-ma2", handlePriceBelowMA2)
	http.HandleFunc("/webhook/ma/price-cross-up-ma2", handlePriceCrossUpMA2)
	http.HandleFunc("/webhook/ma/price-cross-down-ma2", handlePriceCrossDownMA2)
	http.HandleFunc("/webhook/ma/ma1-cross-up-ma2", handleMA1CrossUpMA2)
	http.HandleFunc("/webhook/ma/ma1-cross-down-ma2", handleMA1CrossDownMA2)
	http.HandleFunc("/webhook/ma/ma1-above-ma2", handleMA1AboveMA2) // Position tracking (cross detection)
	http.HandleFunc("/webhook/ma/ma1-below-ma2", handleMA1BelowMA2) // Position tracking (cross detection)
	http.HandleFunc("/webhook/ma/ma2-above-ma3", handleMA2AboveMA3) // Position tracking
	http.HandleFunc("/webhook/ma/ma2-below-ma3", handleMA2BelowMA3) // Position tracking

	// SMC (Smart Money Concept) Structure Webhooks
	http.HandleFunc("/webhook/smc/low-low", handleSMCLowLow)     // Lower Low (LL)
	http.HandleFunc("/webhook/smc/high-low", handleSMCHighLow)   // Higher Low (HL)
	http.HandleFunc("/webhook/smc/low-high", handleSMCLowHigh)   // Lower High (LH)
	http.HandleFunc("/webhook/smc/high-high", handleSMCHighHigh) // Higher High (HH)

	// Generic TradingView Webhooks
	http.HandleFunc("/webhook/generic/take-long-position", handleGenericTakeLongPosition)
	http.HandleFunc("/webhook/generic/take-short-position", handleGenericTakeShortPosition)
	http.HandleFunc("/webhook/generic/exit", handleGenericExit)

	// RSI Directional Cross Webhooks (RSI crossing specific levels with direction)
	http.HandleFunc("/webhook/rsi/cross-down-40", handleRSICrossDown40)
	http.HandleFunc("/webhook/rsi/cross-up-60", handleRSICrossUp60)
	http.HandleFunc("/webhook/rsi/cross-up-50", handleRSICrossUp50)
	http.HandleFunc("/webhook/rsi/cross-down-50", handleRSICrossDown50)
	http.HandleFunc("/webhook/rsi/cross-down-70", handleRSICrossDown70)
	http.HandleFunc("/webhook/rsi/cross-up-30", handleRSICrossUp30)
	http.HandleFunc("/webhook/rsi/cross-down-overbuy", handleRSICrossDownOverbuy)
	http.HandleFunc("/webhook/rsi/cross-up-oversell", handleRSICrossUpOversell)

	// MACD Histogram Webhooks
	http.HandleFunc("/webhook/macd/histogram-cross-up-0", handleMACDHistCrossUp0)
	http.HandleFunc("/webhook/macd/histogram-cross-down-0", handleMACDHistCrossDown0)
	http.HandleFunc("/webhook/macd/histogram-above-0", handleMACDHistAbove0)
	http.HandleFunc("/webhook/macd/histogram-below-0", handleMACDHistBelow0)

	// EMA Cross Webhooks
	http.HandleFunc("/webhook/ema/9-cross-up-21", handleEMA9CrossUp21)
	http.HandleFunc("/webhook/ema/9-cross-down-21", handleEMA9CrossDown21)
	http.HandleFunc("/webhook/ema/9-above-21", handleEMA9Above21)
	http.HandleFunc("/webhook/ema/9-below-21", handleEMA9Below21)
	http.HandleFunc("/webhook/ema/price-cross-down-50", handlePriceCrossDownEMA50)
	http.HandleFunc("/webhook/ema/price-cross-up-50", handlePriceCrossUpEMA50)

	// Alias old EMA endpoints to new MA ribbon endpoints for backward compatibility
	http.HandleFunc("/webhook/ema/9-cross-up-21-alias", handleMA1CrossUpMA2)
	http.HandleFunc("/webhook/ema/9-cross-down-21-alias", handleMA1CrossDownMA2)

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
		log.Printf("\n📱 TradingView Webhook URLs for %s:", activeStrategy.Name)

		// Display LONG entry conditions
		if activeStrategy.Long != nil {
			log.Printf("\n   LONG ENTRY (%s):", activeStrategy.Long.Entry.Combination)
			webhooks := extractWebhooks(activeStrategy.Long.Entry.Conditions)
			for _, webhook := range webhooks {
				log.Printf("     %s%s", ngrokURL, webhook)
			}

			// Display LONG exit conditions
			log.Printf("\n   LONG EXIT (%s):", activeStrategy.Long.Exit.Combination)
			webhooks = extractWebhooks(activeStrategy.Long.Exit.Conditions)
			for _, webhook := range webhooks {
				log.Printf("     %s%s", ngrokURL, webhook)
			}
		} else if activeStrategy.Entry != nil {
			log.Printf("\n   ENTRY (%s):", activeStrategy.Entry.Combination)
			webhooks := extractWebhooks(activeStrategy.Entry.Conditions)
			for _, webhook := range webhooks {
				log.Printf("     %s%s", ngrokURL, webhook)
			}

			// Display exit conditions
			if activeStrategy.Exit != nil {
				log.Printf("\n   EXIT (%s):", activeStrategy.Exit.Combination)
				webhooks = extractWebhooks(activeStrategy.Exit.Conditions)
				for _, webhook := range webhooks {
					log.Printf("     %s%s", ngrokURL, webhook)
				}
			}
		}

		// Display SHORT entry/exit conditions if separate strategy
		if activeStrategy.Short != nil {
			log.Printf("\n   SHORT ENTRY (%s):", activeStrategy.Short.Entry.Combination)
			webhooks := extractWebhooks(activeStrategy.Short.Entry.Conditions)
			for _, webhook := range webhooks {
				log.Printf("     %s%s", ngrokURL, webhook)
			}

			// Display SHORT exit conditions
			log.Printf("\n   SHORT EXIT (%s):", activeStrategy.Short.Exit.Combination)
			webhooks = extractWebhooks(activeStrategy.Short.Exit.Conditions)
			for _, webhook := range webhooks {
				log.Printf("     %s%s", ngrokURL, webhook)
			}
		}

		log.Println("\n📊 Monitoring:")
		log.Printf("   GET %s/health", ngrokURL)
		log.Printf("   GET %s/status", ngrokURL)
		log.Println("\n🖥️  ngrok Web Interface: http://localhost:4040")
	} else {
		log.Println("⚠️  Could not fetch ngrok URL (ngrok may not be running)")
		log.Println("   Run 'docker-compose up' to start ngrok automatically")
		log.Printf("\n📱 Local Webhook Endpoints for %s:", activeStrategy.Name)

		// Display LONG entry conditions
		if activeStrategy.Long != nil {
			log.Printf("   LONG ENTRY (%s):", activeStrategy.Long.Entry.Combination)
			webhooks := extractWebhooks(activeStrategy.Long.Entry.Conditions)
			for _, webhook := range webhooks {
				log.Printf("                %s", webhook)
			}

			// Display LONG exit conditions
			log.Printf("   LONG EXIT (%s):", activeStrategy.Long.Exit.Combination)
			webhooks = extractWebhooks(activeStrategy.Long.Exit.Conditions)
			for _, webhook := range webhooks {
				log.Printf("                %s", webhook)
			}
		} else if activeStrategy.Entry != nil {
			log.Printf("   ENTRY (%s):", activeStrategy.Entry.Combination)
			webhooks := extractWebhooks(activeStrategy.Entry.Conditions)
			for _, webhook := range webhooks {
				log.Printf("                %s", webhook)
			}

			// Display exit conditions
			if activeStrategy.Exit != nil {
				log.Printf("   EXIT (%s):", activeStrategy.Exit.Combination)
				webhooks = extractWebhooks(activeStrategy.Exit.Conditions)
				for _, webhook := range webhooks {
					log.Printf("                %s", webhook)
				}
			}
		}

		// Display SHORT entry/exit conditions if separate strategy
		if activeStrategy.Short != nil {
			log.Printf("   SHORT ENTRY (%s):", activeStrategy.Short.Entry.Combination)
			webhooks := extractWebhooks(activeStrategy.Short.Entry.Conditions)
			for _, webhook := range webhooks {
				log.Printf("                %s", webhook)
			}

			// Display SHORT exit conditions
			log.Printf("   SHORT EXIT (%s):", activeStrategy.Short.Exit.Combination)
			webhooks = extractWebhooks(activeStrategy.Short.Exit.Conditions)
			for _, webhook := range webhooks {
				log.Printf("                %s", webhook)
			}
		}

		log.Println("\n📊 Monitoring:")
		log.Println("   GET /health")
		log.Println("   GET /status")
	}
	log.Println("")

	// Start periodic TP/SL detection checker
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			checkClosedPositions()
		}
	}()

	log.Fatal(http.ListenAndServe(":"+port, nil))
}
