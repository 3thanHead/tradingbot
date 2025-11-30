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
	Symbol          string
	PositionOpen    bool
	Position        string // "long" or "short"
	TradeID         string
	Exchange        string  // Exchange name (OANDA, NYSE, NASDAQ, etc.)
	IsSimulated     bool    // True if this is a simulated trade (non-OANDA)
	SimulatedEntry  string  // Entry time for simulated trade
	SimulatedExit   string  // Exit time for simulated trade
	SimulatedPrice  string  // Entry price for simulated trade
	MACDCrossedUp   bool    // Tracks if MACD crossed up
	MACDCrossedDown bool    // Tracks if MACD crossed down
	SwingHigh       float64 // Latest swing high price level
	SwingLow        float64 // Latest swing low price level

	// Stochastic indicator tracking
	StochInOversold   bool // Both K and D lines are in oversold (<20)
	StochInOverbought bool // Both K and D lines are in overbought (>80)

	// Stochastic RSI indicator tracking
	StochRSICrossedUp20   bool // Stochastic RSI crossed up above 20
	StochRSICrossedDown20 bool // Stochastic RSI crossed down below 20
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
	case "/webhook/stochastic-rsi/cross-up-80":
		return state.StochRSICrossedUp80
	case "/webhook/stochastic-rsi/cross-down-80":
		return state.StochRSICrossedDown80

	// ATR conditions
	case "/webhook/atr/above-average":
		return state.ATRAboveAverage
	case "/webhook/atr/below-average":
		return state.ATRBelowAverage

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

	currentPath := r.URL.Path

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

			matched, complete := conditionNodeMatches(&node, currentPath, state.EntryConditionsCompleted, conditionKey)

			if matched && complete {
				mu.Lock()
				state.EntryConditionsCompleted[conditionKey] = true
				mu.Unlock()
				log.Printf("✅ [STRATEGY] Entry condition met: %s", getNodeDescription(&node))
			}
		}

		// Check if all are completed
		allComplete := true
		for i := 0; i < len(entryConditions.Conditions); i++ {
			key := fmt.Sprintf("%scondition_%d", conditionPrefix, i)
			if !state.EntryConditionsCompleted[key] {
				allComplete = false
				break
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
	state := getPositionState(symbol)

	log.Printf("📊 MACD Cross Up for %s", symbol)

	mu.Lock()
	state.MACDCrossedUp = true
	state.MACDCrossedDown = false // Reset opposite condition
	mu.Unlock()

	// Clear entry condition that depended on MACDCrossedDown being true
	clearEntryConditionForWebhook(symbol, "/webhook/macd/cross-down")

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
	state := getPositionState(symbol)

	log.Printf("📊 MACD Cross Down for %s", symbol)

	mu.Lock()
	state.MACDCrossedDown = true
	state.MACDCrossedUp = false // Reset opposite condition
	mu.Unlock()

	// Clear entry condition that depended on MACDCrossedUp being true
	clearEntryConditionForWebhook(symbol, "/webhook/macd/cross-up")

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
	state := getPositionState(symbol)

	log.Printf("📊 Stochastic RSI crossed UP above 20 for %s", symbol)

	mu.Lock()
	state.StochRSICrossedUp20 = true
	state.StochRSICrossedDown20 = false // Reset opposite condition
	state.StochRSICrossedUp80 = false   // Clear overbought zone
	state.StochRSICrossedDown80 = false // Clear overbought zone
	mu.Unlock()

	// Clear entry condition that depended on StochRSICrossedDown20 being true
	clearEntryConditionForWebhook(symbol, "/webhook/stochastic-rsi/cross-down-20")
	clearEntryConditionForWebhook(symbol, "/webhook/stochastic-rsi/cross-up-80")
	clearEntryConditionForWebhook(symbol, "/webhook/stochastic-rsi/cross-down-80")

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
	state := getPositionState(symbol)

	log.Printf("📊 Stochastic RSI crossed DOWN below 20 for %s", symbol)

	mu.Lock()
	state.StochRSICrossedDown20 = true
	state.StochRSICrossedUp20 = false   // Reset opposite condition
	state.StochRSICrossedUp80 = false   // Clear overbought zone
	state.StochRSICrossedDown80 = false // Clear overbought zone
	mu.Unlock()

	// Clear entry condition that depended on StochRSICrossedUp20 being true
	clearEntryConditionForWebhook(symbol, "/webhook/stochastic-rsi/cross-up-20")
	clearEntryConditionForWebhook(symbol, "/webhook/stochastic-rsi/cross-up-80")
	clearEntryConditionForWebhook(symbol, "/webhook/stochastic-rsi/cross-down-80")

	respondSuccess(w, "Stochastic RSI crossed down 20 condition set")
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
	state := getPositionState(symbol)

	log.Printf("📊 Stochastic RSI crossed UP above 80 for %s", symbol)

	mu.Lock()
	state.StochRSICrossedUp80 = true
	state.StochRSICrossedDown80 = false // Reset opposite condition
	state.StochRSICrossedUp20 = false   // Clear oversold zone
	state.StochRSICrossedDown20 = false // Clear oversold zone
	mu.Unlock()

	// Clear entry condition that depended on StochRSICrossedDown80 being true
	clearEntryConditionForWebhook(symbol, "/webhook/stochastic-rsi/cross-down-80")
	clearEntryConditionForWebhook(symbol, "/webhook/stochastic-rsi/cross-up-20")
	clearEntryConditionForWebhook(symbol, "/webhook/stochastic-rsi/cross-down-20")

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
	state := getPositionState(symbol)

	log.Printf("📊 Stochastic RSI crossed DOWN below 80 for %s", symbol)

	mu.Lock()
	state.StochRSICrossedDown80 = true
	state.StochRSICrossedUp80 = false   // Reset opposite condition
	state.StochRSICrossedUp20 = false   // Clear oversold zone
	state.StochRSICrossedDown20 = false // Clear oversold zone
	mu.Unlock()

	// Clear entry condition that depended on StochRSICrossedUp80 being true
	clearEntryConditionForWebhook(symbol, "/webhook/stochastic-rsi/cross-up-80")
	clearEntryConditionForWebhook(symbol, "/webhook/stochastic-rsi/cross-up-20")
	clearEntryConditionForWebhook(symbol, "/webhook/stochastic-rsi/cross-down-20")

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

	respondSuccess(w, "Stochastic RSI crossed down 80 condition set")
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
	state := getPositionState(symbol)

	// Store exchange information
	mu.Lock()
	state.Exchange = event.Exchange
	mu.Unlock()

	log.Printf("📊 RSI crossed ABOVE 50 (uptrend) for %s [%s]", symbol, event.Exchange)

	mu.Lock()
	state.RSIAbove50 = true
	state.RSIBelow50 = false
	mu.Unlock()

	// Clear entry condition that depended on RSIBelow50 being true
	clearEntryConditionForWebhook(symbol, "/webhook/rsi/below-50")

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
	state := getPositionState(symbol)

	log.Printf("📊 RSI crossed BELOW 50 (downtrend) for %s", symbol)

	mu.Lock()
	state.RSIBelow50 = true
	state.RSIAbove50 = false
	mu.Unlock()

	// Clear entry condition that depended on RSIAbove50 being true
	clearEntryConditionForWebhook(symbol, "/webhook/rsi/above-50")

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
	state := getPositionState(symbol)

	log.Printf("📊 Price crossed ABOVE EMA 20 for %s", symbol)

	mu.Lock()
	state.PriceAboveEMA20 = true
	state.PriceBelowEMA20 = false
	mu.Unlock()

	// Clear entry condition that depended on PriceBelowEMA20 being true
	clearEntryConditionForWebhook(symbol, "/webhook/ema/price-below-ema20")

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
	state := getPositionState(symbol)

	log.Printf("📊 Price crossed ABOVE EMA 50 for %s", symbol)

	mu.Lock()
	state.PriceAboveEMA50 = true
	state.PriceBelowEMA50 = false
	mu.Unlock()

	// Clear entry condition that depended on PriceBelowEMA50 being true
	clearEntryConditionForWebhook(symbol, "/webhook/ema/price-below-ema50")

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
	state := getPositionState(symbol)

	log.Printf("📊 Price crossed BELOW EMA 50 for %s", symbol)

	mu.Lock()
	state.PriceBelowEMA50 = true
	state.PriceAboveEMA50 = false
	mu.Unlock()

	// Clear entry condition that depended on PriceAboveEMA50 being true
	clearEntryConditionForWebhook(symbol, "/webhook/ema/price-above-ema50")

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
	state := getPositionState(symbol)

	log.Printf("📊 Price crossed ABOVE EMA 200 for %s", symbol)

	mu.Lock()
	state.PriceAboveEMA200 = true
	state.PriceBelowEMA200 = false
	mu.Unlock()

	// Clear entry condition that depended on PriceBelowEMA200 being true
	clearEntryConditionForWebhook(symbol, "/webhook/ema/price-below-ema200")

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
	state := getPositionState(symbol)

	log.Printf("📊 Price crossed BELOW EMA 200 for %s", symbol)

	mu.Lock()
	state.PriceBelowEMA200 = true
	state.PriceAboveEMA200 = false
	mu.Unlock()

	// Clear entry condition that depended on PriceAboveEMA200 being true
	clearEntryConditionForWebhook(symbol, "/webhook/ema/price-above-ema200")

	if shouldOpenPosition(symbol, false, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening SHORT position")
		openShortPosition(symbol, event.Close)
		respondSuccess(w, "Price below EMA200 + strategy → SHORT opened")
		return
	}

	respondSuccess(w, "Price below EMA200 condition set")
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
	if !isWebhookUsedInStrategy("/webhook/ema/9-cross-up-21") {
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
	state := getPositionState(symbol)

	log.Printf("📊 EMA 9 crossed UP through EMA 21 for %s", symbol)

	// Update EMA cross state
	mu.Lock()
	state.EMA9CrossedUpEMA21 = true
	state.EMA9CrossedDownEMA21 = false // Clear opposite state
	mu.Unlock()

	// Clear entry condition that depended on EMA9CrossedDownEMA21 being true
	clearEntryConditionForWebhook(symbol, "/webhook/ema/9-cross-down-21")

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
	if !isWebhookUsedInStrategy("/webhook/ema/9-cross-down-21") {
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
	state := getPositionState(symbol)

	log.Printf("📊 EMA 9 crossed DOWN through EMA 21 for %s", symbol)

	// Update EMA cross state
	mu.Lock()
	state.EMA9CrossedDownEMA21 = true
	state.EMA9CrossedUpEMA21 = false // Clear opposite state
	mu.Unlock()

	// Clear entry condition that depended on EMA9CrossedUpEMA21 being true
	clearEntryConditionForWebhook(symbol, "/webhook/ema/9-cross-up-21")

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
	// Check if any position is already open across all symbols
	mu.RLock()
	for sym, s := range positions {
		if s.PositionOpen {
			mu.RUnlock()
			log.Printf("⚠️  [BLOCKED] Cannot open LONG on %s - position already open on %s (%s)", symbol, sym, strings.ToUpper(s.Position))
			return
		}
	}
	mu.RUnlock()

	mu.Lock()
	state := positions[symbol]
	exchange := state.Exchange
	mu.Unlock()

	// Check if this is a non-OANDA exchange (simulated trade)
	if exchange != "OANDA" && exchange != "" {
		mu.Lock()
		state.PositionOpen = true
		state.Position = "long"
		state.IsSimulated = true
		state.SimulatedEntry = formatTimeWithZone(getLocalTime())
		state.SimulatedPrice = price
		state.TradeID = fmt.Sprintf("SIM-%s-%d", symbol, time.Now().Unix())
		// Clear only entry/exit tracking - keep indicator states
		state.EntryConditionsCompleted = make(map[string]bool)
		state.ExitConditionsCompleted = make(map[string]bool)
		mu.Unlock()

		log.Println(strings.Repeat("🟢", 40))
		log.Printf("📊 SIMULATED LONG TRADE - %s", exchange)
		log.Println(strings.Repeat("🟢", 40))
		log.Printf("Symbol: %s", symbol)
		log.Printf("Entry Time: %s", state.SimulatedEntry)
		log.Printf("Entry Price: %s", price)
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
	// Clear only entry/exit tracking - keep indicator states
	state.EntryConditionsCompleted = make(map[string]bool)
	state.ExitConditionsCompleted = make(map[string]bool)
	mu.Unlock()

	log.Printf("✅ LONG position opened: %s (ID: %s)", symbol, tradeID)
	log.Printf("💾 [STATE] Position updated - Open=%v, Type=%s, TradeID=%s, All flags cleared",
		state.PositionOpen, state.Position, state.TradeID)
}

func openShortPosition(symbol string, price string) {
	// Check if any position is already open across all symbols
	mu.RLock()
	for sym, s := range positions {
		if s.PositionOpen {
			mu.RUnlock()
			log.Printf("⚠️  [BLOCKED] Cannot open SHORT on %s - position already open on %s (%s)", symbol, sym, strings.ToUpper(s.Position))
			return
		}
	}
	mu.RUnlock()

	mu.Lock()
	state := positions[symbol]
	exchange := state.Exchange
	mu.Unlock()

	// Check if this is a non-OANDA exchange (simulated trade)
	if exchange != "OANDA" && exchange != "" {
		mu.Lock()
		state.PositionOpen = true
		state.Position = "short"
		state.IsSimulated = true
		state.SimulatedEntry = formatTimeWithZone(getLocalTime())
		state.SimulatedPrice = price
		state.TradeID = fmt.Sprintf("SIM-%s-%d", symbol, time.Now().Unix())
		// Clear only entry/exit tracking - keep indicator states
		state.EntryConditionsCompleted = make(map[string]bool)
		state.ExitConditionsCompleted = make(map[string]bool)
		mu.Unlock()

		log.Println(strings.Repeat("🔴", 40))
		log.Printf("📊 SIMULATED SHORT TRADE - %s", exchange)
		log.Println(strings.Repeat("🔴", 40))
		log.Printf("Symbol: %s", symbol)
		log.Printf("Entry Time: %s", state.SimulatedEntry)
		log.Printf("Entry Price: %s", price)
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
	// Clear only entry/exit tracking - keep indicator states
	state.EntryConditionsCompleted = make(map[string]bool)
	state.ExitConditionsCompleted = make(map[string]bool)
	mu.Unlock()

	log.Printf("✅ SHORT position opened: %s (ID: %s)", symbol, tradeID)
	log.Printf("💾 [STATE] Position updated - Open=%v, Type=%s, TradeID=%s, All flags cleared",
		state.PositionOpen, state.Position, state.TradeID)
}

func closePosition(symbol string) {
	mu.RLock()
	state := positions[symbol]
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

		log.Println(strings.Repeat("🔵", 40))
		log.Printf("📊 SIMULATED %s TRADE CLOSED - %s", strings.ToUpper(position), exchange)
		log.Println(strings.Repeat("🔵", 40))
		log.Printf("Symbol: %s", symbol)
		log.Printf("Entry Time: %s", entryTime)
		log.Printf("Entry Price: %s", entryPrice)
		log.Printf("Exit Time: %s", exitTime)
		log.Printf("Trade ID: %s", tradeID)
		log.Println(strings.Repeat("🔵", 40))
		log.Println("⚠️  MANUAL ACTION REQUIRED: Mark this exit in TradingView")
		log.Println(strings.Repeat("🔵", 40))

		mu.Lock()
		state.PositionOpen = false
		state.Position = ""
		state.TradeID = ""
		state.SimulatedExit = exitTime
		// Clear only entry/exit tracking - keep indicator states
		state.EntryConditionsCompleted = make(map[string]bool)
		state.ExitConditionsCompleted = make(map[string]bool)
		mu.Unlock()

		// Check if entry conditions are already met for a new position
		log.Printf("🔍 [ENTRY] Checking if conditions met for new position...")
		if shouldOpenPosition(symbol, true, nil) {
			log.Printf("✅ [TRADE] LONG entry conditions already met! Opening position")
			openLongPosition(symbol, entryPrice)
		} else if shouldOpenPosition(symbol, false, nil) {
			log.Printf("✅ [TRADE] SHORT entry conditions already met! Opening position")
			openShortPosition(symbol, entryPrice)
		} else {
			log.Printf("⏳ [ENTRY] No entry conditions met yet")
		}

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
		state.PositionOpen = false
		state.Position = "none"
		state.TradeID = ""
		// Clear only entry/exit tracking - keep indicator states
		state.EntryConditionsCompleted = make(map[string]bool)
		state.ExitConditionsCompleted = make(map[string]bool)
		mu.Unlock()

		log.Printf("✅ Position closed: %s", symbol)
		log.Printf("💾 [STATE] Position updated - Open=%v, Type=%s, Condition tracking reset", state.PositionOpen, state.Position)

		// Check if entry conditions are already met for a new position
		log.Printf("🔍 [ENTRY] Checking if conditions met for new position...")
		if shouldOpenPosition(symbol, true, nil) {
			log.Printf("✅ [TRADE] LONG entry conditions already met! Opening position")
			openLongPosition(symbol, entryPrice)
		} else if shouldOpenPosition(symbol, false, nil) {
			log.Printf("✅ [TRADE] SHORT entry conditions already met! Opening position")
			openShortPosition(symbol, entryPrice)
		} else {
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
	// Simple conversion - adjust as needed
	if len(ticker) == 6 {
		return ticker[:3] + "_" + ticker[3:]
	}
	return ticker
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
	mu.Lock()
	defer mu.Unlock()

	log.Println(strings.Repeat("=", 80))
	log.Printf("📊 STRATEGY STATUS REPORT - %s", formatTimeWithZone(getLocalTime()))
	log.Println(strings.Repeat("=", 80))
	log.Printf("Strategy: %s", activeStrategy.Name)
	log.Println("")

	// If no positions have been created yet, show a default symbol report
	if len(positions) == 0 {
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

	// Report for each symbol
	for symbol, state := range positions {
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
				log.Printf("  │ Trade ID: %s%-48s│", state.TradeID, "")
				log.Println("  └─────────────────────────────────────────────────────────────────────┘")
			} else {
				log.Println("  ┌─────────────────────────────────────────────────────────────────────┐")
				log.Printf("  │ 📈 POSITION OPEN: %s (Trade ID: %s)%-24s│",
					strings.ToUpper(state.Position), state.TradeID, "")
				log.Println("  └─────────────────────────────────────────────────────────────────────┘")
			}
			log.Println("")

			// Show ONLY exit conditions for the current position
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

	// Strategy configuration
	strategyName = os.Getenv("STRATEGY")
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
	http.HandleFunc("/webhook/stochastic-rsi/cross-up-80", handleStochRSICrossUp80)
	http.HandleFunc("/webhook/stochastic-rsi/cross-down-80", handleStochRSICrossDown80)

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

	// RSI Directional Cross Webhooks (RSI crossing specific levels with direction)
	http.HandleFunc("/webhook/rsi/cross-down-40", handleRSICrossDown40)
	http.HandleFunc("/webhook/rsi/cross-up-60", handleRSICrossUp60)
	http.HandleFunc("/webhook/rsi/cross-up-50", handleRSICrossUp50)
	http.HandleFunc("/webhook/rsi/cross-down-50", handleRSICrossDown50)
	http.HandleFunc("/webhook/rsi/cross-down-70", handleRSICrossDown70)
	http.HandleFunc("/webhook/rsi/cross-up-30", handleRSICrossUp30)

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

	// Start periodic status reporter
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			reportStrategyStatus()
		}
	}()

	log.Fatal(http.ListenAndServe(":"+port, nil))
}
