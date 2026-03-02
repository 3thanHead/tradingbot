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
	Name         string `json:"name"`
	Description  string `json:"description"`
	OneTradeMode bool   `json:"oneTradeMode"` // If true, strategy disables after one trade cycle

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
	Symbol               string
	PositionOpen         bool
	PositionOpening      bool   // True when position open is in progress (prevents duplicate opens)
	Position             string // "long" or "short"
	TradeID              string
	Exchange             string  // Exchange name (OANDA, NYSE, NASDAQ, etc.)
	IsSimulated          bool    // True if this is a simulated trade (non-OANDA)
	SimulatedEntry       string  // Entry time for simulated trade
	SimulatedExit        string  // Exit time for simulated trade
	SimulatedPrice       string  // Entry price for simulated trade
	SimulatedExitPrice   string  // Exit price for simulated trade
	SimulatedPL          string  // Final P/L for simulated trade (stored on close)
	LatestPrice          string  // Latest price from webhook (for P/L calculation)
	MACDCrossedUp        bool    // Tracks if MACD crossed up (true cross detected)
	MACDCrossedDown      bool    // Tracks if MACD crossed down (true cross detected)
	MACDAboveSignal      bool    // MACD line is currently above signal line
	MACDBelowSignal      bool    // MACD line is currently below signal line
	MACDStateInitialized bool    // True once we've seen at least one MACD state
	SwingHigh            float64 // Latest swing high price level
	SwingLow             float64 // Latest swing low price level

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

	// MA cross event tracking (set only on actual crosses, not position changes)
	MA1CrossedAboveMA2 bool // MA1 actually crossed above MA2 (requires state change)
	MA1CrossedBelowMA2 bool // MA1 actually crossed below MA2 (requires state change)

	// MA Ribbon position tracking (for MA1/MA2/MA3 alignment)
	MA2AboveMA3 bool // MA2 is currently above MA3
	MA2BelowMA3 bool // MA2 is currently below MA3

	// MA1 vs MA4 position tracking
	MA1AboveMA4 bool // MA1 is currently above MA4
	MA1BelowMA4 bool // MA1 is currently below MA4

	// ATR volatility tracking
	ATRAboveAverage         bool // ATR is above its 20-period average (high volatility)
	ATRBelowAverage         bool // ATR is below its 20-period average (low volatility)
	ATRAboveThreshold       bool // ATR is above a specific threshold value
	ATRBelowThreshold       bool // ATR is below a specific threshold value
	ATRDirectionLong        bool // Current ATR direction is long (for tracking persistent state)
	ATRDirectionInitialized bool // Whether ATR direction has been initialized
	ATRFlipLong             bool // ATR trailing stop flipped to long (actual change detected)
	ATRFlipShort            bool // ATR trailing stop flipped to short (actual change detected)
	ATRLong                 bool // ATR long signal
	ATRShort                bool // ATR short signal
	ATRIdle                 bool // ATR idle signal (trend conflict - no trading)
	ATRLongCrossed          bool // ATR actually crossed to long (not just initial state)
	ATRShortCrossed         bool // ATR actually crossed to short (not just initial state)
	PositionOpenedWhileIdle bool // Position was opened externally while ATR was idle (wait for cross to close)

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
	EMA9EMA21StateInitialized bool // True once we've seen at least one position state (prevents false crosses on startup)

	// OANDA Real-time P/L tracking (fetched from OANDA API)
	OandaUnrealizedPL string // Current unrealized P/L from OANDA (e.g., "-2.45")
	OandaRealizedPL   string // Realized P/L when position was closed

	// Track which entry conditions have been completed
	EntryConditionsCompleted map[string]bool // Maps condition index to completion status

	// Track which exit conditions have been completed
	ExitConditionsCompleted map[string]bool // Maps condition index to completion status

	// Whipsaw protection
	LastClosedDirection string // Tracks the last direction that was closed (for reversal logic)
}

// Global state management
var (
	positions       = make(map[string]*PositionState)
	mu              sync.RWMutex
	strategyEnabled = true // Strategy enabled flag - set to false after one trade cycle

	oandaAPIKey       = os.Getenv("OANDA_API_KEY")
	oandaAccountID    = os.Getenv("OANDA_ACCOUNT_ID")
	oandaBaseURL      string // Set in main() based on OANDA_LIVE env var
	tradeUnits        string // Trading units (fixed amount)
	tradeUSDAmount    string // USD notional amount (calculates units from price)
	tradeMargin       string // Margin amount (OANDA calculates position size based on leverage)
	takeProfitPips    string // Take profit in pips (e.g., "50")
	takeProfitPct     string // Take profit in percentage (e.g., "2.5" for 2.5%)
	takeProfitDollars string // Take profit in dollar amount (e.g., "100" for $100 gain)
	stopLossPips      string // Stop loss in pips (e.g., "30")
	stopLossPct       string // Stop loss in percentage (e.g., "1.5" for 1.5%)
	stopLossDollars   string // Stop loss in dollar amount (e.g., "50" for $50 loss)

	// Strategy system
	activeStrategy          Strategy  // Currently loaded strategy
	strategyName            string    // Name of strategy file to load
	firstWebhookStatusShown bool      // Track if first webhook status has been shown
	lastStatusReportTime    time.Time // Track last time status was reported
	timezoneOffset          int       // Timezone offset in hours (e.g., -5 for UTC-5, 0 for UTC)

	// Trading hours configuration (supports two sessions)
	tradingHoursEnabled   bool           // Whether trading hours restriction is enabled
	tradingStartHour      int            // Session 1: Start hour (0-23) for allowed trading
	tradingStartMinute    int            // Session 1: Start minute (0-59) for allowed trading
	tradingEndHour        int            // Session 1: End hour (0-23) for allowed trading
	tradingEndMinute      int            // Session 1: End minute (0-59) for allowed trading
	tradingStartHour2     int            // Session 2: Start hour (0-23) for allowed trading
	tradingStartMinute2   int            // Session 2: Start minute (0-59) for allowed trading
	tradingEndHour2       int            // Session 2: End hour (0-23) for allowed trading
	tradingEndMinute2     int            // Session 2: End minute (0-59) for allowed trading
	session2Enabled       bool           // Whether session 2 is configured
	tradingDays           []int          // Days of week when trading is allowed (0=Sunday, 1=Monday, ...6=Saturday)
	tradingTimezone       *time.Location // Timezone for trading hours (default: uses timezoneOffset)
	wasWithinTradingHours bool           // Track last known trading hours state to detect when hours open

	// Daily profit tracking
	dailyProfitTarget        float64   // Target profit for the day (disables trading when reached)
	dailyProfitTargetEnabled bool      // Whether daily profit limit is enabled
	dailyRealizedProfit      float64   // Total realized profit for today
	dailyProfitResetTime     time.Time // When the daily profit counter was last reset
	dailyProfitTargetReached bool      // Flag indicating daily target has been reached
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

		// For real OANDA positions, update P/L in background
		if state.PositionOpen && !state.IsSimulated && state.TradeID != "" {
			go updateOandaPositionPL(symbol)
		}
	}
}

// ============================================================================
// TRADING HOURS RESTRICTION
// ============================================================================

// isWithinTradingHours checks if the current time is within allowed trading hours
// Returns true if trading is allowed, false if outside trading hours
func isWithinTradingHours() bool {
	// If trading hours restriction is disabled, always allow trading
	if !tradingHoursEnabled {
		return true
	}

	// Get current time in the configured timezone
	var now time.Time
	if tradingTimezone != nil {
		now = time.Now().In(tradingTimezone)
	} else {
		// Fallback to UTC with offset
		now = time.Now().UTC().Add(time.Duration(timezoneOffset) * time.Hour)
	}

	currentHour := now.Hour()
	currentMinute := now.Minute()
	currentDay := int(now.Weekday()) // 0=Sunday, 1=Monday, ...6=Saturday

	// Check if current day is in allowed trading days
	dayAllowed := false
	if len(tradingDays) == 0 {
		// If no days specified, allow all days
		dayAllowed = true
	} else {
		for _, day := range tradingDays {
			if day == currentDay {
				dayAllowed = true
				break
			}
		}
	}

	if !dayAllowed {
		return false
	}

	// Convert current time to minutes since midnight for easier comparison
	currentMinutesSinceMidnight := currentHour*60 + currentMinute

	// Check Session 1
	if isTimeInSession(currentMinutesSinceMidnight, tradingStartHour, tradingStartMinute, tradingEndHour, tradingEndMinute) {
		return true
	}

	// Check Session 2 if enabled
	if session2Enabled {
		if isTimeInSession(currentMinutesSinceMidnight, tradingStartHour2, tradingStartMinute2, tradingEndHour2, tradingEndMinute2) {
			return true
		}
	}

	return false
}

// isTimeInSession checks if the current time (in minutes since midnight) is within a session
func isTimeInSession(currentMinutes, startHour, startMinute, endHour, endMinute int) bool {
	startMinutesSinceMidnight := startHour*60 + startMinute
	endMinutesSinceMidnight := endHour*60 + endMinute

	// Handle case where end time is less than start time (overnight trading)
	if endMinutesSinceMidnight > startMinutesSinceMidnight {
		// Normal hours: e.g., 9:30-17:00
		return currentMinutes >= startMinutesSinceMidnight && currentMinutes < endMinutesSinceMidnight
	} else {
		// Overnight hours: e.g., 22:30-06:00
		return currentMinutes >= startMinutesSinceMidnight || currentMinutes < endMinutesSinceMidnight
	}
}

// formatTimeAMPM formats hour and minute to 12-hour AM/PM format
func formatTimeAMPM(hour, minute int) string {
	period := "AM"
	displayHour := hour

	if hour == 0 {
		displayHour = 12
	} else if hour == 12 {
		period = "PM"
	} else if hour > 12 {
		displayHour = hour - 12
		period = "PM"
	}

	if minute == 0 {
		return fmt.Sprintf("%d%s", displayHour, period)
	}
	return fmt.Sprintf("%d:%02d%s", displayHour, minute, period)
}

// getTradingHoursStatus returns a human-readable status of trading hours configuration
func getTradingHoursStatus() string {
	if !tradingHoursEnabled {
		return "Trading hours: No restrictions (24/7)"
	}

	dayNames := []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}
	var allowedDays []string
	if len(tradingDays) == 0 {
		allowedDays = dayNames
	} else {
		for _, d := range tradingDays {
			if d >= 0 && d <= 6 {
				allowedDays = append(allowedDays, dayNames[d])
			}
		}
	}

	tzName := "UTC"
	if tradingTimezone != nil {
		tzName = tradingTimezone.String()
	} else if timezoneOffset != 0 {
		tzName = fmt.Sprintf("UTC%+d", timezoneOffset)
	}

	// Format Session 1 with AM/PM for readability
	startTime1 := formatTimeAMPM(tradingStartHour, tradingStartMinute)
	endTime1 := formatTimeAMPM(tradingEndHour, tradingEndMinute)
	session1 := fmt.Sprintf("%s-%s", startTime1, endTime1)

	if session2Enabled {
		// Format Session 2
		startTime2 := formatTimeAMPM(tradingStartHour2, tradingStartMinute2)
		endTime2 := formatTimeAMPM(tradingEndHour2, tradingEndMinute2)
		session2 := fmt.Sprintf("%s-%s", startTime2, endTime2)

		return fmt.Sprintf("Trading hours: Session 1: %s | Session 2: %s (%s) on %s",
			session1, session2, tzName, strings.Join(allowedDays, ","))
	}

	return fmt.Sprintf("Trading hours: %s %s on %s",
		session1, tzName, strings.Join(allowedDays, ","))
}

// getActiveSessionName returns which session is currently active (for logging)
func getActiveSessionName() string {
	if !tradingHoursEnabled {
		return "24/7"
	}

	var now time.Time
	if tradingTimezone != nil {
		now = time.Now().In(tradingTimezone)
	} else {
		now = time.Now().UTC().Add(time.Duration(timezoneOffset) * time.Hour)
	}

	currentMinutes := now.Hour()*60 + now.Minute()

	if isTimeInSession(currentMinutes, tradingStartHour, tradingStartMinute, tradingEndHour, tradingEndMinute) {
		return fmt.Sprintf("Session 1 (%s-%s)",
			formatTimeAMPM(tradingStartHour, tradingStartMinute),
			formatTimeAMPM(tradingEndHour, tradingEndMinute))
	}

	if session2Enabled && isTimeInSession(currentMinutes, tradingStartHour2, tradingStartMinute2, tradingEndHour2, tradingEndMinute2) {
		return fmt.Sprintf("Session 2 (%s-%s)",
			formatTimeAMPM(tradingStartHour2, tradingStartMinute2),
			formatTimeAMPM(tradingEndHour2, tradingEndMinute2))
	}

	return "CLOSED"
}

// parseTimeString parses a time string in format "H" or "H:MM" and returns hour and minute
// Supports formats like "9", "9:30", "14", "16:45", etc.
func parseTimeString(timeStr string) (hour int, minute int, err error) {
	timeStr = strings.TrimSpace(timeStr)

	// Check if it contains a colon (H:MM format)
	if strings.Contains(timeStr, ":") {
		parts := strings.Split(timeStr, ":")
		if len(parts) != 2 {
			return 0, 0, fmt.Errorf("invalid time format (expected H:MM)")
		}

		hour, err = strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil {
			return 0, 0, fmt.Errorf("invalid hour: %v", err)
		}

		minute, err = strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil {
			return 0, 0, fmt.Errorf("invalid minute: %v", err)
		}
	} else {
		// Just hour (H format)
		hour, err = strconv.Atoi(timeStr)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid hour: %v", err)
		}
		minute = 0
	}

	// Validate ranges
	if hour < 0 || hour > 24 {
		return 0, 0, fmt.Errorf("hour must be 0-24, got %d", hour)
	}
	if minute < 0 || minute > 59 {
		return 0, 0, fmt.Errorf("minute must be 0-59, got %d", minute)
	}

	return hour, minute, nil
}

// parseTradingDays parses a comma-separated string of day numbers (0-6) or day names
func parseTradingDays(daysStr string) ([]int, error) {
	if daysStr == "" {
		return nil, nil // Empty means all days allowed
	}

	dayNameToNum := map[string]int{
		"sun": 0, "sunday": 0,
		"mon": 1, "monday": 1,
		"tue": 2, "tuesday": 2,
		"wed": 3, "wednesday": 3,
		"thu": 4, "thursday": 4,
		"fri": 5, "friday": 5,
		"sat": 6, "saturday": 6,
	}

	parts := strings.Split(daysStr, ",")
	var days []int
	seen := make(map[int]bool)

	for _, part := range parts {
		part = strings.TrimSpace(strings.ToLower(part))
		if part == "" {
			continue
		}

		// Try parsing as number first
		if num, err := strconv.Atoi(part); err == nil {
			if num < 0 || num > 6 {
				return nil, fmt.Errorf("day number %d out of range (0-6)", num)
			}
			if !seen[num] {
				days = append(days, num)
				seen[num] = true
			}
			continue
		}

		// Try parsing as day name
		if num, ok := dayNameToNum[part]; ok {
			if !seen[num] {
				days = append(days, num)
				seen[num] = true
			}
			continue
		}

		return nil, fmt.Errorf("invalid day value: %s", part)
	}

	// Sort days for consistent display
	sort.Ints(days)
	return days, nil
}

// ============================================================================
// OANDA/FOREX DETECTION
// ============================================================================

// isForexPair checks if a symbol is a forex pair that should be traded on OANDA
// This detects pairs like EUR_USD, GBP_JPY, EURUSD, GBPJPY etc. and ensures they use real OANDA trading
// even if TradingView sends an exchange like "FX", "FOREX", "FXCM", etc.
func isForexPair(symbol string) bool {
	var base, quote string

	if strings.Contains(symbol, "_") {
		// Format: EUR_USD
		parts := strings.Split(symbol, "_")
		if len(parts) != 2 || len(parts[0]) != 3 || len(parts[1]) != 3 {
			return false
		}
		base, quote = parts[0], parts[1]
	} else if len(symbol) == 6 {
		// Format: EURUSD (6 characters = two 3-letter currency codes)
		base, quote = symbol[:3], symbol[3:]
	} else {
		return false
	}

	// Exclude crypto symbols (BTC, ETH, etc.)
	cryptos := []string{"BTC", "ETH", "XRP", "LTC", "BCH", "ADA", "DOT", "SOL", "XLM", "UNI", "TRX", "EOS", "NEO", "VET", "FIL", "XMR", "XTZ", "BNB", "CRO", "FTM", "ATOM", "ALGO", "HBAR", "EGLD", "ONE", "QNT", "CHZ", "ENJ", "MANA", "SAND", "AXS", "GALA", "DOGE", "SHIB", "MATIC", "AVAX", "LINK"}
	for _, crypto := range cryptos {
		if base == crypto || quote == crypto {
			return false
		}
	}
	return true
}

// shouldUseOANDA determines if a symbol should be traded on OANDA (real trading)
// Returns true for explicit OANDA exchange OR forex pairs (regardless of exchange field)
func shouldUseOANDA(symbol string, exchange string) bool {
	return exchange == "OANDA" || isForexPair(symbol)
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
	if strategy.OneTradeMode {
		log.Printf("🔒 One Trade Mode: ENABLED (strategy will disable after one trade cycle)")
	} else {
		log.Printf("♻️  Continuous Mode: Strategy will continue monitoring after trades")
	}
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

// requiresCrossEvent checks if a webhook condition requires a cross/flip event to trigger
func requiresCrossEvent(webhookPath string) bool {
	switch webhookPath {
	case "/webhook/atr/long", "/webhook/atr/short",
		"/webhook/atr/flip-long", "/webhook/atr/flip-short":
		return true
	default:
		return false
	}
}

// hasCrossedRecently checks if a cross-required condition has actually crossed
func hasCrossedRecently(webhookPath string, state *PositionState) bool {
	switch webhookPath {
	case "/webhook/atr/long":
		// Only true if we actually crossed from short to long
		return state.ATRLongCrossed
	case "/webhook/atr/short":
		// Only true if we actually crossed from long to short
		return state.ATRShortCrossed
	case "/webhook/atr/flip-long":
		return state.ATRFlipLong
	case "/webhook/atr/flip-short":
		return state.ATRFlipShort
	default:
		return true // Non-cross conditions are always "crossed"
	}
}

// isPendingCross checks if we're waiting for the ATR to cross TO this condition
// (i.e., the opposite direction is currently active and initialized)
func isPendingCross(webhookPath string, state *PositionState) bool {
	if !state.ATRDirectionInitialized {
		return false
	}

	switch webhookPath {
	case "/webhook/atr/long", "/webhook/atr/flip-long":
		// Pending cross to long = we're currently in short direction
		return !state.ATRDirectionLong
	case "/webhook/atr/short", "/webhook/atr/flip-short":
		// Pending cross to short = we're currently in long direction
		return state.ATRDirectionLong
	default:
		return false
	}
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
		return state.EMA9AboveEMA21 // Check persistent state
	case "/webhook/ma/ma1-below-ma2":
		return state.EMA9BelowEMA21 // Check persistent state
	case "/webhook/ma/ma2-above-ma3":
		return state.MA2AboveMA3
	case "/webhook/ma/ma2-below-ma3":
		return state.MA2BelowMA3
	case "/webhook/ma/ma1-above-ma4":
		return state.MA1AboveMA4
	case "/webhook/ma/ma1-below-ma4":
		return state.MA1BelowMA4

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
	case "/webhook/atr/above-threshold":
		return state.ATRAboveThreshold
	case "/webhook/atr/below-threshold":
		return state.ATRBelowThreshold
	case "/webhook/atr/flip-long":
		return state.ATRFlipLong
	case "/webhook/atr/flip-short":
		return state.ATRFlipShort
	case "/webhook/atr/long":
		return state.ATRLong
	case "/webhook/atr/short":
		return state.ATRShort
	case "/webhook/atr/idle":
		return state.ATRIdle

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
	// Check if strategy is enabled first
	mu.RLock()
	enabled := strategyEnabled
	targetReached := dailyProfitTargetReached
	mu.RUnlock()
	if !enabled {
		return false
	}

	// Check if daily profit target has been reached
	if dailyProfitTargetEnabled && targetReached {
		log.Printf("🎯 [DAILY] Daily profit target reached - new positions blocked")
		return false
	}

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

				// Check if within allowed trading hours AFTER conditions are met
				if !isWithinTradingHours() {
					log.Printf("⏰ [BLOCKED] Position ready to open but outside trading hours")
					log.Printf("   %s", getTradingHoursStatus())
					log.Printf("   Conditions remain tracked and will execute when trading hours resume")
					// Don't reset conditions - keep them for when trading hours resume
					return false
				}

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
				// For conditions that require a cross, also check if the cross actually happened
				if requiresCrossEvent(condition.Webhook) && !hasCrossedRecently(condition.Webhook, state) {
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

			// Check if within allowed trading hours AFTER conditions are met
			if !isWithinTradingHours() {
				log.Printf("⏰ [BLOCKED] Position ready to open but outside trading hours")
				log.Printf("   %s", getTradingHoursStatus())
				log.Printf("   Conditions remain tracked and will execute when trading hours resume")
				// Don't reset conditions - keep them for when trading hours resume
				return false
			}

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

				// Check if within allowed trading hours AFTER condition is met
				if !isWithinTradingHours() {
					log.Printf("⏰ [BLOCKED] Position ready to open but outside trading hours")
					log.Printf("   %s", getTradingHoursStatus())
					return false
				}

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
	// Check if strategy is enabled
	mu.RLock()
	enabled := strategyEnabled
	mu.RUnlock()
	if !enabled {
		log.Printf("🛑 [STRATEGY] Strategy DISABLED - ignoring MACD Cross Up webhook")
		respondSuccess(w, "Strategy disabled")
		return
	}

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

	// Check if this is a real cross by comparing with the opposite state
	// MACD was below signal, now above = cross detected
	// Only count as cross if state was initialized AND opposite state was true
	wasCross := state.MACDStateInitialized && state.MACDBelowSignal

	log.Printf("📊 MACD Cross Up for %s (wasCross=%v, initialized=%v)", symbol, wasCross, state.MACDStateInitialized)

	mu.Lock()
	state.MACDAboveSignal = true
	state.MACDBelowSignal = false
	state.MACDStateInitialized = true
	// Set cross flag ONLY if actual cross detected (state change from below to above)
	if wasCross {
		state.MACDCrossedUp = true
		state.MACDCrossedDown = false
	}
	mu.Unlock()

	// Check if we should exit SHORT position (reversal)
	if wasCross && state.PositionOpen && state.Position == "short" {
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

	// Check if we should open LONG position (only on real cross)
	if wasCross && shouldOpenPosition(symbol, true, r) && !state.PositionOpen {
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

	// Check if this is a real cross by comparing with the opposite state
	// MACD was above signal, now below = cross detected
	// Only count as cross if state was initialized AND opposite state was true
	wasCross := state.MACDStateInitialized && state.MACDAboveSignal

	log.Printf("📊 MACD Cross Down for %s (wasCross=%v, initialized=%v)", symbol, wasCross, state.MACDStateInitialized)

	mu.Lock()
	state.MACDBelowSignal = true
	state.MACDAboveSignal = false
	state.MACDStateInitialized = true
	// Set cross flag ONLY if actual cross detected (state change from above to below)
	if wasCross {
		state.MACDCrossedDown = true
		state.MACDCrossedUp = false
	}
	mu.Unlock()

	// Check if we should exit LONG position (reversal)
	if wasCross && state.PositionOpen && state.Position == "long" {
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

	// Check if we should open SHORT position (only on real cross)
	if wasCross && shouldOpenPosition(symbol, false, r) && !state.PositionOpen {
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

	// Check if we should exit SHORT position (RSI above 50 is bullish)
	if state.PositionOpen && state.Position == "short" {
		shouldExit, reason := shouldExitPosition(symbol, false, r)
		if shouldExit {
			log.Printf("⚠️ [EXIT] %s → closing SHORT position", reason)
			closePosition(symbol)
			state = getPositionState(symbol)
		}
		respondSuccess(w, "RSI above 50 condition set")
		return
	}

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

	log.Printf("📊 RSI crossed BELOW 50 (downtrend) for %s", symbol)

	mu.Lock()
	state.RSIBelow50 = true
	state.RSIAbove50 = false
	mu.Unlock()

	// Check if we should exit LONG position (RSI below 50 is bearish)
	if state.PositionOpen && state.Position == "long" {
		shouldExit, reason := shouldExitPosition(symbol, true, r)
		if shouldExit {
			log.Printf("⚠️ [EXIT] %s → closing LONG position", reason)
			closePosition(symbol)
			state = getPositionState(symbol)
		}
		respondSuccess(w, "RSI below 50 condition set")
		return
	}

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
// ATR THRESHOLD HANDLERS
// ============================================================================

// POST /webhook/atr/above-threshold
func handleATRAboveThreshold(w http.ResponseWriter, r *http.Request) {
	// Check if this webhook is used in the active strategy
	if !isWebhookUsedInStrategy("/webhook/atr/above-threshold") {
		log.Printf("⏭️  [WEBHOOK] ATR Above Threshold not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received ATR Above Threshold event")

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
	log.Printf("📊 ATR is above threshold for %s (high volatility)", symbol)

	mu.Lock()
	state.ATRAboveThreshold = true
	state.ATRBelowThreshold = false
	mu.Unlock()

	// Check if we should open position (can be used for either direction)
	if shouldOpenPosition(symbol, true, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening LONG position")
		openLongPosition(symbol, event.Close)
		respondSuccess(w, "ATR above threshold + strategy → LONG opened")
		return
	}

	if shouldOpenPosition(symbol, false, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening SHORT position")
		openShortPosition(symbol, event.Close)
		respondSuccess(w, "ATR above threshold + strategy → SHORT opened")
		return
	}

	respondSuccess(w, "ATR above threshold condition set")
}

// POST /webhook/atr/below-threshold
func handleATRBelowThreshold(w http.ResponseWriter, r *http.Request) {
	// Check if this webhook is used in the active strategy
	// Also accept if above-threshold is used (we need to reset its state)
	if !isWebhookUsedInStrategy("/webhook/atr/below-threshold") && !isWebhookUsedInStrategy("/webhook/atr/above-threshold") {
		log.Printf("⏭️  [WEBHOOK] ATR Below Threshold not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received ATR Below Threshold event")

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
	log.Printf("📊 ATR is below threshold for %s (low volatility - blocking entries)", symbol)

	mu.Lock()
	state.ATRBelowThreshold = true
	state.ATRAboveThreshold = false
	mu.Unlock()

	// ATR below threshold should prevent new entries - no position checks
	log.Printf("⛔ ATR below threshold - no new positions allowed")
	respondSuccess(w, "ATR below threshold - no entries")
}

// POST /webhook/atr/flip-long
func handleATRFlipLong(w http.ResponseWriter, r *http.Request) {
	// Check if this webhook is used in the active strategy
	if !isWebhookUsedInStrategy("/webhook/atr/flip-long") {
		log.Printf("⏭️  [WEBHOOK] ATR Flip Long not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received ATR Flip Long event")

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

	// Check if this is a real flip by comparing with the opposite state
	// ATR was short direction, now long = flip detected
	// Only count as flip if state was initialized AND opposite direction was true
	wasFlip := state.ATRDirectionInitialized && !state.ATRDirectionLong

	log.Printf("📊 ATR Flip Long for %s (wasFlip=%v, initialized=%v, wasLong=%v)",
		symbol, wasFlip, state.ATRDirectionInitialized, state.ATRDirectionLong)

	mu.Lock()
	// Update direction tracking
	state.ATRDirectionLong = true
	state.ATRDirectionInitialized = true
	// Set flip flag ONLY if actual flip detected (state change from short to long)
	if wasFlip {
		state.ATRFlipLong = true
		state.ATRFlipShort = false
	}
	mu.Unlock()

	// Check if we should exit SHORT position (reversal) - only on real flip
	if wasFlip && state.PositionOpen && state.Position == "short" {
		shouldExit, reason := shouldExitPosition(symbol, false, r)
		if shouldExit {
			log.Printf("⚠️ [EXIT] %s → closing SHORT position", reason)
			closePosition(symbol)
			state = getPositionState(symbol)
		}
	}

	// Check if we should open LONG position (only on real flip)
	if wasFlip && shouldOpenPosition(symbol, true, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening LONG position")
		openLongPosition(symbol, event.Close)
		respondSuccess(w, "ATR flip long + strategy → LONG opened")
		return
	}

	respondSuccess(w, "ATR flip long condition set")
}

// POST /webhook/atr/flip-short
func handleATRFlipShort(w http.ResponseWriter, r *http.Request) {
	// Check if this webhook is used in the active strategy
	if !isWebhookUsedInStrategy("/webhook/atr/flip-short") {
		log.Printf("⏭️  [WEBHOOK] ATR Flip Short not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received ATR Flip Short event")

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

	// Check if this is a real flip by comparing with the opposite state
	// ATR was long direction, now short = flip detected
	// Only count as flip if state was initialized AND opposite direction was true
	wasFlip := state.ATRDirectionInitialized && state.ATRDirectionLong

	log.Printf("📊 ATR Flip Short for %s (wasFlip=%v, initialized=%v, wasLong=%v)",
		symbol, wasFlip, state.ATRDirectionInitialized, state.ATRDirectionLong)

	mu.Lock()
	// Update direction tracking
	state.ATRDirectionLong = false
	state.ATRDirectionInitialized = true
	// Set flip flag ONLY if actual flip detected (state change from long to short)
	if wasFlip {
		state.ATRFlipShort = true
		state.ATRFlipLong = false
	}
	mu.Unlock()

	// Check if we should exit LONG position (reversal) - only on real flip
	if wasFlip && state.PositionOpen && state.Position == "long" {
		shouldExit, reason := shouldExitPosition(symbol, true, r)
		if shouldExit {
			log.Printf("⚠️ [EXIT] %s → closing LONG position", reason)
			closePosition(symbol)
			state = getPositionState(symbol)
		}
	}

	// Check if we should open SHORT position (only on real flip)
	if wasFlip && shouldOpenPosition(symbol, false, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening SHORT position")
		openShortPosition(symbol, event.Close)
		respondSuccess(w, "ATR flip short + strategy → SHORT opened")
		return
	}

	respondSuccess(w, "ATR flip short condition set")
}

// POST /webhook/atr/long
func handleATRLong(w http.ResponseWriter, r *http.Request) {
	// Check if this webhook is used in the active strategy
	if !isWebhookUsedInStrategy("/webhook/atr/long") {
		log.Printf("⏭️  [WEBHOOK] ATR Long not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received ATR Long event")

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

	// Check previous state for cross detection
	wasShort := state.ATRShort
	wasLong := state.ATRLong
	wasIdle := state.ATRIdle
	wasInitialized := state.ATRLong || state.ATRShort || state.ATRIdle

	// Cross = any state change (short→long OR idle→long)
	isCross := wasShort || wasIdle

	log.Printf("📊 ATR long signal for %s (wasShort=%v, wasLong=%v, wasIdle=%v, initialized=%v, isCross=%v)",
		symbol, wasShort, wasLong, wasIdle, wasInitialized, isCross)

	mu.Lock()
	state.ATRLong = true
	state.ATRShort = false
	state.ATRIdle = false
	if isCross {
		state.ATRLongCrossed = true
		state.ATRShortCrossed = false
		// Clear the idle-opened flag on direction change - position will be evaluated for exit
		state.PositionOpenedWhileIdle = false
	}
	mu.Unlock()

	// First signal after startup/reset - initialize only, no trade
	if !wasInitialized {
		log.Printf("⏳ [INIT] First ATR signal - initializing as LONG, waiting for next cross")
		respondSuccess(w, "ATR long initialized - waiting for cross")
		return
	}

	// Repeated signal - no state change, no action
	if wasLong {
		log.Printf("ℹ️  [LONG→LONG] Already in long state - no change")
		respondSuccess(w, "ATR long condition maintained")
		return
	}

	// Cross detected (from short or idle)
	if wasShort {
		log.Printf("✅ [SHORT→LONG] Cross detected")
	} else if wasIdle {
		log.Printf("✅ [IDLE→LONG] Cross detected")
	}

	// Check if we should exit SHORT position
	if state.PositionOpen && state.Position == "short" {
		shouldExit, reason := shouldExitPosition(symbol, false, r)
		if shouldExit {
			log.Printf("⚠️ [EXIT] %s → closing SHORT position", reason)
			closePosition(symbol)
			respondSuccess(w, "ATR long cross → SHORT closed")
			return
		}
	}

	// Check if we should open LONG position
	if !state.PositionOpen && shouldOpenPosition(symbol, true, r) {
		log.Printf("✅ [TRADE] ATR long cross + strategy conditions met! Opening LONG position")
		openLongPosition(symbol, event.Close)
		respondSuccess(w, "ATR long cross → LONG opened")
		return
	}

	respondSuccess(w, "ATR long cross condition set")
}

// POST /webhook/atr/short
func handleATRShort(w http.ResponseWriter, r *http.Request) {
	// Check if this webhook is used in the active strategy
	if !isWebhookUsedInStrategy("/webhook/atr/short") {
		log.Printf("⏭️  [WEBHOOK] ATR Short not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received ATR Short event")

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

	// Check previous state for cross detection
	wasLong := state.ATRLong
	wasShort := state.ATRShort
	wasIdle := state.ATRIdle
	wasInitialized := state.ATRLong || state.ATRShort || state.ATRIdle

	// Cross = any state change (long→short OR idle→short)
	isCross := wasLong || wasIdle

	log.Printf("📊 ATR short signal for %s (wasLong=%v, wasShort=%v, wasIdle=%v, initialized=%v, isCross=%v)",
		symbol, wasLong, wasShort, wasIdle, wasInitialized, isCross)

	mu.Lock()
	state.ATRShort = true
	state.ATRLong = false
	state.ATRIdle = false
	if isCross {
		state.ATRShortCrossed = true
		state.ATRLongCrossed = false
		// Clear the idle-opened flag on direction change - position will be evaluated for exit
		state.PositionOpenedWhileIdle = false
	}
	mu.Unlock()

	// First signal after startup/reset - initialize only, no trade
	if !wasInitialized {
		log.Printf("⏳ [INIT] First ATR signal - initializing as SHORT, waiting for next cross")
		respondSuccess(w, "ATR short initialized - waiting for cross")
		return
	}

	// Repeated signal - no state change, no action
	if wasShort {
		log.Printf("ℹ️  [SHORT→SHORT] Already in short state - no change")
		respondSuccess(w, "ATR short condition maintained")
		return
	}

	// Cross detected (from long or idle)
	if wasLong {
		log.Printf("✅ [LONG→SHORT] Cross detected")
	} else if wasIdle {
		log.Printf("✅ [IDLE→SHORT] Cross detected")
	}

	// Check if we should exit LONG position
	if state.PositionOpen && state.Position == "long" {
		shouldExit, reason := shouldExitPosition(symbol, true, r)
		if shouldExit {
			log.Printf("⚠️ [EXIT] %s → closing LONG position", reason)
			closePosition(symbol)
			respondSuccess(w, "ATR short cross → LONG closed")
			return
		}
	}

	// Check if we should open SHORT position
	if !state.PositionOpen && shouldOpenPosition(symbol, false, r) {
		log.Printf("✅ [TRADE] ATR short cross + strategy conditions met! Opening SHORT position")
		openShortPosition(symbol, event.Close)
		respondSuccess(w, "ATR short cross → SHORT opened")
		return
	}

	respondSuccess(w, "ATR short cross condition set")
}

// POST /webhook/atr/idle
// ATR Idle signal - ATR direction conflicts with long-term trend (1H 200 EMA)
// This closes any open position and prevents new trades until Long or Short fires again
func handleATRIdle(w http.ResponseWriter, r *http.Request) {
	// Check if this webhook is used in the active strategy
	if !isWebhookUsedInStrategy("/webhook/atr/idle") {
		log.Printf("⏭️  [WEBHOOK] ATR Idle not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received ATR Idle event (trend conflict)")

	// Sync positions from OANDA to detect externally opened positions before deciding
	if err := periodicSyncPositionsFromOanda(); err != nil {
		log.Printf("⚠️  [SYNC] Failed to sync positions: %v", err)
	}

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

	// Check previous state for cross detection
	wasLong := state.ATRLong
	wasShort := state.ATRShort
	wasIdle := state.ATRIdle
	wasInitialized := state.ATRLong || state.ATRShort || state.ATRIdle

	// Cross = any state change (long→idle OR short→idle)
	isCross := wasLong || wasShort

	log.Printf("⚠️  ATR Idle for %s (wasLong=%v, wasShort=%v, wasIdle=%v, initialized=%v, isCross=%v)",
		symbol, wasLong, wasShort, wasIdle, wasInitialized, isCross)

	// Set idle state
	mu.Lock()
	state.ATRIdle = true
	state.ATRLong = false
	state.ATRShort = false
	state.ATRFlipLong = false
	state.ATRFlipShort = false
	state.ATRLongCrossed = false
	state.ATRShortCrossed = false
	mu.Unlock()

	// First signal after startup/reset - initialize only, no action
	if !wasInitialized {
		log.Printf("⏳ [INIT] First ATR signal - initializing as IDLE, waiting for next cross")
		respondSuccess(w, "ATR idle initialized - waiting for cross")
		return
	}

	// Repeated signal - no state change, no action
	if wasIdle {
		log.Printf("ℹ️  [IDLE→IDLE] Already in idle state - no change")
		respondSuccess(w, "ATR idle condition maintained")
		return
	}

	// Cross detected (from long or short) - close any open position
	if wasLong {
		log.Printf("✅ [LONG→IDLE] Cross detected - trend conflict")
	} else if wasShort {
		log.Printf("✅ [SHORT→IDLE] Cross detected - trend conflict")
	}

	if state.PositionOpen {
		// Check if position was opened externally during idle - let it ride until next ATR direction change
		if state.PositionOpenedWhileIdle {
			log.Printf("ℹ️  [IDLE] Position was opened externally during idle - letting it ride until next ATR cross")
			respondSuccess(w, "ATR idle - external position waiting for next cross")
			return
		}
		posType := state.Position
		log.Println(strings.Repeat("⚪", 40))
		log.Printf("⚪ ATR IDLE - Trend conflict detected!")
		log.Printf("⚪ Closing %s position for %s", strings.ToUpper(posType), symbol)
		log.Printf("⚪ Will stay FLAT until ATR Long or ATR Short fires")
		log.Println(strings.Repeat("⚪", 40))
		closePosition(symbol)
		respondSuccess(w, fmt.Sprintf("ATR idle cross → %s closed, staying flat", strings.ToUpper(posType)))
		return
	}

	log.Printf("ℹ️  [IDLE] No open position - staying flat until trend aligns")

	respondSuccess(w, "ATR idle cross - no trading until Long or Short fires")
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
	log.Printf("📊 Price is above MA#2 for %s", symbol)

	mu.Lock()
	state.PriceAboveEMA20 = true
	state.PriceBelowEMA20 = false
	mu.Unlock()

	// Check if we should exit SHORT position (price above MA2 is bullish)
	if state.PositionOpen && state.Position == "short" {
		shouldExit, reason := shouldExitPosition(symbol, false, r)
		if shouldExit {
			log.Printf("⚠️ [EXIT] %s → closing SHORT position", reason)
			closePosition(symbol)
			state = getPositionState(symbol)
		}
		respondSuccess(w, "Price above MA#2 condition set")
		return
	}

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
	log.Printf("📊 Price is below MA#2 for %s", symbol)

	mu.Lock()
	state.PriceBelowEMA20 = true
	state.PriceAboveEMA20 = false
	mu.Unlock()

	// Check if we should exit LONG position (price below MA2 is bearish)
	if state.PositionOpen && state.Position == "long" {
		shouldExit, reason := shouldExitPosition(symbol, true, r)
		if shouldExit {
			log.Printf("⚠️ [EXIT] %s → closing LONG position", reason)
			closePosition(symbol)
			state = getPositionState(symbol)
		}
		respondSuccess(w, "Price below MA#2 condition set")
		return
	}

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

	// Check if this is a real cross by comparing with previous state
	// MA1 was below MA2, now above = cross detected
	// Only count as cross if state was initialized AND opposite state was true
	wasCross := state.EMA9EMA21StateInitialized && state.EMA9BelowEMA21

	log.Printf("📊 MA#1 above MA#2 for %s (wasCross=%v, initialized=%v)", symbol, wasCross, state.EMA9EMA21StateInitialized)

	// Set state and clear opposite condition
	mu.Lock()
	state.EMA9AboveEMA21 = true
	state.EMA9BelowEMA21 = false
	state.EMA9EMA21StateInitialized = true
	// Set cross flag ONLY if actual cross detected
	if wasCross {
		state.MA1CrossedAboveMA2 = true
		state.MA1CrossedBelowMA2 = false
	}
	mu.Unlock()

	// Clear opposite entry condition if it was set
	clearEntryConditionForWebhook(symbol, "/webhook/ma/ma1-below-ma2")

	// Check if we should exit SHORT position (only on real cross)
	if wasCross && state.PositionOpen && state.Position == "short" {
		shouldExit, _ := shouldExitPosition(symbol, false, r)
		if shouldExit {
			log.Printf("🚨 [EXIT] MA1 crossed above MA2 - exiting SHORT position")
			closePosition(symbol)
			respondSuccess(w, "MA1 above MA2 → SHORT closed")
			return
		}
	}

	// Check if we should open LONG position (only on real cross)
	if wasCross && shouldOpenPosition(symbol, true, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] MA1 crossed above MA2 + strategy conditions met! Opening LONG position")
		openLongPosition(symbol, event.Close)
		respondSuccess(w, "MA1 above MA2 + strategy → LONG opened")
		return
	}

	if !state.EMA9EMA21StateInitialized || !wasCross {
		log.Printf("⏳ [INIT] MA1 above MA2 - state initialized, waiting for next cross to trigger")
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

	// Check if this is a real cross by comparing with previous state
	// MA1 was above MA2, now below = cross detected
	// Only count as cross if state was initialized AND opposite state was true
	wasCross := state.EMA9EMA21StateInitialized && state.EMA9AboveEMA21

	log.Printf("📊 MA#1 below MA#2 for %s (wasCross=%v, initialized=%v)", symbol, wasCross, state.EMA9EMA21StateInitialized)

	// Set state and clear opposite condition
	mu.Lock()
	state.EMA9BelowEMA21 = true
	state.EMA9AboveEMA21 = false
	state.EMA9EMA21StateInitialized = true
	// Set cross flag ONLY if actual cross detected
	if wasCross {
		state.MA1CrossedBelowMA2 = true
		state.MA1CrossedAboveMA2 = false
	}
	mu.Unlock()

	// Clear opposite entry condition if it was set
	clearEntryConditionForWebhook(symbol, "/webhook/ma/ma1-above-ma2")

	// Check if we should exit LONG position (only on real cross)
	if wasCross && state.PositionOpen && state.Position == "long" {
		shouldExit, _ := shouldExitPosition(symbol, true, r)
		if shouldExit {
			log.Printf("🚨 [EXIT] MA1 crossed below MA2 - exiting LONG position")
			closePosition(symbol)
			respondSuccess(w, "MA1 below MA2 → LONG closed")
			return
		}
	}

	// Check if we should open SHORT position (only on real cross)
	if wasCross && shouldOpenPosition(symbol, false, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] MA1 crossed below MA2 + strategy conditions met! Opening SHORT position")
		openShortPosition(symbol, event.Close)
		respondSuccess(w, "MA1 below MA2 + strategy → SHORT opened")
		return
	}

	if !state.EMA9EMA21StateInitialized || !wasCross {
		log.Printf("⏳ [INIT] MA1 below MA2 - state initialized, waiting for next cross to trigger")
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
	log.Printf("📊 MA#2 above MA#3 for %s", symbol)

	mu.Lock()
	state.MA2AboveMA3 = true
	state.MA2BelowMA3 = false
	mu.Unlock()

	// Check if we should open LONG position
	if shouldOpenPosition(symbol, true, r) && !state.PositionOpen {
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
	log.Printf("📊 MA#2 below MA#3 for %s", symbol)

	mu.Lock()
	state.MA2BelowMA3 = true
	state.MA2AboveMA3 = false
	mu.Unlock()

	// Check if we should open LONG position (shouldn't happen with MA2 below MA3, but keep for consistency)
	if shouldOpenPosition(symbol, true, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening LONG position")
		openLongPosition(symbol, event.Close)
		respondSuccess(w, "MA#2 below MA#3 + strategy → LONG opened")
		return
	}

	// Check if we should open SHORT position
	if shouldOpenPosition(symbol, false, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening SHORT position")
		openShortPosition(symbol, event.Close)
		respondSuccess(w, "MA#2 below MA#3 + strategy → SHORT opened")
		return
	}

	respondSuccess(w, "MA#2 below MA#3 condition set")
}

// POST /webhook/ma/ma1-above-ma4
func handleMA1AboveMA4(w http.ResponseWriter, r *http.Request) {
	if !isWebhookUsedInStrategy("/webhook/ma/ma1-above-ma4") {
		log.Printf("⏭️  [WEBHOOK] MA#1 Above MA#4 not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received MA#1 Above MA#4 event")

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
	log.Printf("📊 MA#1 above MA#4 for %s", symbol)

	mu.Lock()
	state.MA1AboveMA4 = true
	state.MA1BelowMA4 = false
	mu.Unlock()

	// Check if we should open LONG position
	if shouldOpenPosition(symbol, true, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening LONG position")
		openLongPosition(symbol, event.Close)
		respondSuccess(w, "MA#1 above MA#4 + strategy → LONG opened")
		return
	}

	// Check if we should open SHORT position
	if shouldOpenPosition(symbol, false, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening SHORT position")
		openShortPosition(symbol, event.Close)
		respondSuccess(w, "MA#1 above MA#4 + strategy → SHORT opened")
		return
	}

	respondSuccess(w, "MA#1 above MA#4 condition set")
}

// POST /webhook/ma/ma1-below-ma4
func handleMA1BelowMA4(w http.ResponseWriter, r *http.Request) {
	if !isWebhookUsedInStrategy("/webhook/ma/ma1-below-ma4") {
		log.Printf("⏭️  [WEBHOOK] MA#1 Below MA#4 not used in current strategy - ignoring")
		respondSuccess(w, "Webhook not used in strategy")
		return
	}

	log.Printf("🔔 [WEBHOOK] Received MA#1 Below MA#4 event")

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
	log.Printf("📊 MA#1 below MA#4 for %s", symbol)

	mu.Lock()
	state.MA1BelowMA4 = true
	state.MA1AboveMA4 = false
	mu.Unlock()

	// Check if we should open LONG position
	if shouldOpenPosition(symbol, true, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening LONG position")
		openLongPosition(symbol, event.Close)
		respondSuccess(w, "MA#1 below MA#4 + strategy → LONG opened")
		return
	}

	// Check if we should open SHORT position
	if shouldOpenPosition(symbol, false, r) && !state.PositionOpen {
		log.Printf("✅ [TRADE] Strategy conditions met! Opening SHORT position")
		openShortPosition(symbol, event.Close)
		respondSuccess(w, "MA#1 below MA#4 + strategy → SHORT opened")
		return
	}

	respondSuccess(w, "MA#1 below MA#4 condition set")
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

// Get unrealized P/L for an open trade from OANDA
// Returns the unrealized P/L in account currency (typically USD)
func getOandaTradeUnrealizedPL(tradeID string) (float64, error) {
	if tradeID == "" {
		return 0, fmt.Errorf("no trade ID provided")
	}

	url := fmt.Sprintf("%s/v3/accounts/%s/trades/%s", oandaBaseURL, oandaAccountID, tradeID)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+oandaAPIKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch trade: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return 0, fmt.Errorf("OANDA API returned status %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("failed to parse response: %v", err)
	}

	trade, ok := result["trade"].(map[string]interface{})
	if !ok {
		return 0, fmt.Errorf("invalid trade data")
	}

	// OANDA returns unrealizedPL as a string
	unrealizedPLStr, ok := trade["unrealizedPL"].(string)
	if !ok {
		return 0, fmt.Errorf("no unrealized P/L in response")
	}

	unrealizedPL := 0.0
	fmt.Sscanf(unrealizedPLStr, "%f", &unrealizedPL)
	return unrealizedPL, nil
}

// Get realized P/L from a closed trade (used when position is closed)
func getOandaTradeRealizedPL(tradeID string) (float64, error) {
	if tradeID == "" {
		return 0, fmt.Errorf("no trade ID provided")
	}

	// Query the transactions endpoint to find the close transaction
	url := fmt.Sprintf("%s/v3/accounts/%s/transactions", oandaBaseURL, oandaAccountID)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+oandaAPIKey)
	req.Header.Set("Content-Type", "application/json")

	// Get recent transactions
	q := req.URL.Query()
	q.Add("count", "100")
	req.URL.RawQuery = q.Encode()

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch transactions: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return 0, fmt.Errorf("OANDA API returned status %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("failed to parse response: %v", err)
	}

	transactions, ok := result["transactions"].([]interface{})
	if !ok {
		return 0, fmt.Errorf("no transactions found")
	}

	// Look for ORDER_FILL transaction that closed this trade
	for _, txn := range transactions {
		txnMap, ok := txn.(map[string]interface{})
		if !ok {
			continue
		}

		// Check if this transaction involves our trade
		if tradesClosed, ok := txnMap["tradesClosed"].([]interface{}); ok {
			for _, closedTrade := range tradesClosed {
				closedTradeMap, ok := closedTrade.(map[string]interface{})
				if !ok {
					continue
				}
				if closedTradeMap["tradeID"] == tradeID {
					// Found the close transaction for our trade
					if realizedPLStr, ok := closedTradeMap["realizedPL"].(string); ok {
						realizedPL := 0.0
						fmt.Sscanf(realizedPLStr, "%f", &realizedPL)
						return realizedPL, nil
					}
				}
			}
		}
	}

	return 0, fmt.Errorf("could not find close transaction for trade %s", tradeID)
}

// Update position state with real-time P/L from OANDA
// Called on webhook events to refresh P/L data
func updateOandaPositionPL(symbol string) {
	mu.RLock()
	state, exists := positions[symbol]
	if !exists || !state.PositionOpen || state.IsSimulated || state.TradeID == "" {
		mu.RUnlock()
		return
	}
	tradeID := state.TradeID
	mu.RUnlock()

	unrealizedPL, err := getOandaTradeUnrealizedPL(tradeID)
	if err != nil {
		log.Printf("⚠️  [P/L] Failed to fetch P/L for %s (ID: %s): %v", symbol, tradeID, err)
		return
	}

	mu.Lock()
	state.OandaUnrealizedPL = fmt.Sprintf("%.2f", unrealizedPL)
	mu.Unlock()

	// Log P/L update
	plColor := "🟢"
	plSign := "+"
	if unrealizedPL < 0 {
		plColor = "🔴"
		plSign = ""
	}
	log.Printf("💰 [OANDA P/L] %s %s: %s%s$%.2f", plColor, symbol, plSign, "", unrealizedPL)
}

// Get all open positions with their P/L from OANDA
// Returns a map of symbol -> P/L details
func getOandaOpenPositionsPL() map[string]map[string]interface{} {
	result := make(map[string]map[string]interface{})

	url := fmt.Sprintf("%s/v3/accounts/%s/openTrades", oandaBaseURL, oandaAccountID)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+oandaAPIKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("⚠️  [P/L] Failed to fetch open trades: %v", err)
		return result
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Printf("⚠️  [P/L] OANDA API returned status %d", resp.StatusCode)
		return result
	}

	var apiResult map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&apiResult); err != nil {
		log.Printf("⚠️  [P/L] Failed to parse response: %v", err)
		return result
	}

	trades, ok := apiResult["trades"].([]interface{})
	if !ok {
		return result
	}

	for _, trade := range trades {
		tradeMap, ok := trade.(map[string]interface{})
		if !ok {
			continue
		}

		instrument, _ := tradeMap["instrument"].(string)
		if instrument == "" {
			continue
		}

		result[instrument] = map[string]interface{}{
			"tradeID":       tradeMap["id"],
			"unrealizedPL":  tradeMap["unrealizedPL"],
			"currentUnits":  tradeMap["currentUnits"],
			"price":         tradeMap["price"],
			"openTime":      tradeMap["openTime"],
			"initialMargin": tradeMap["initialMarginRequired"],
		}
	}

	return result
}

// Check if daily profit target has been reached and disable trading if so
func checkDailyProfitTarget() {
	if !dailyProfitTargetEnabled || dailyProfitTargetReached {
		return
	}

	// Check if we need to reset the daily counter (new day)
	now := getLocalTime()
	if now.YearDay() != dailyProfitResetTime.YearDay() || now.Year() != dailyProfitResetTime.Year() {
		mu.Lock()
		dailyRealizedProfit = 0
		dailyProfitTargetReached = false
		dailyProfitResetTime = now
		mu.Unlock()
		log.Printf("🔄 [DAILY] New trading day - daily profit counter reset to $0.00")
	}

	if dailyRealizedProfit >= dailyProfitTarget {
		mu.Lock()
		dailyProfitTargetReached = true
		strategyEnabled = false
		mu.Unlock()
		log.Println(strings.Repeat("💰", 40))
		log.Printf("🎯 [DAILY PROFIT] Target reached! Today's profit: $%.2f (Target: $%.2f)", dailyRealizedProfit, dailyProfitTarget)
		log.Printf("🛑 [DAILY PROFIT] Trading DISABLED for the rest of the day")
		log.Printf("   Re-enable manually via /enable-strategy or wait for new trading day")
		log.Println(strings.Repeat("💰", 40))
	}
}

// Add realized profit to daily total (called when position is closed)
func addDailyRealizedProfit(profit float64) {
	mu.Lock()
	dailyRealizedProfit += profit
	mu.Unlock()

	log.Printf("💵 [DAILY] Added $%.2f to daily profit. Today's total: $%.2f", profit, dailyRealizedProfit)

	// Check if target reached
	checkDailyProfitTarget()
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
func calculatePriceMoveForDollars(symbol string, currentPrice float64, targetDollars float64, units int, isGain bool) (float64, error) {
	// Get pricing info from OANDA which includes homeConversionFactors
	// isGain: true for take profit (use gainQuoteHome), false for stop loss (use lossQuoteHome)
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

	// For gains (take profit), we use gainQuoteHome (converts positive P&L to home currency)
	// For losses (stop loss), we use lossQuoteHome (converts negative P&L to home currency)
	// These account for the current exchange rates and bid/ask spread
	var conversionFactor float64
	factorKey := "lossQuoteHome"
	if isGain {
		factorKey = "gainQuoteHome"
	}

	// OANDA API v20 returns nested objects: {"gainQuoteHome": {"factor": "1.0"}}
	if factorObj, ok := homeConversion[factorKey].(map[string]interface{}); ok {
		if factorStr, ok := factorObj["factor"].(string); ok {
			fmt.Sscanf(factorStr, "%f", &conversionFactor)
			log.Printf("💱 [CONVERSION] Using %s factor: %.6f", factorKey, conversionFactor)
		}
	} else if factorStr, ok := homeConversion[factorKey].(string); ok {
		// Fallback: try direct string (older API format)
		fmt.Sscanf(factorStr, "%f", &conversionFactor)
		log.Printf("💱 [CONVERSION] Using %s factor (direct): %.6f", factorKey, conversionFactor)
	}

	// For XXX_USD pairs (quote currency = home currency), factor should be ~1.0
	// If we can't get the factor, use 1.0 for USD-quoted pairs, otherwise calculate
	if conversionFactor == 0 {
		if strings.HasSuffix(symbol, "_USD") {
			conversionFactor = 1.0
			log.Printf("💱 [CONVERSION] Using 1.0 for USD-quoted pair %s", symbol)
		} else {
			// For non-USD quoted pairs without a conversion factor, fall back to simple calc
			log.Printf("⚠️  [WARNING] No conversion factor for %s, using simple calculation", symbol)
			return targetDollars / float64(units), nil
		}
	}

	// P&L in USD = price_move × units × conversionFactor
	// Therefore: price_move = targetDollars / (units × conversionFactor)
	priceMove := targetDollars / (float64(units) * conversionFactor)

	log.Printf("💱 [CONVERSION] Factor: %.6f, Price move: %.5f for $%.2f target with %d units", conversionFactor, priceMove, targetDollars, units)

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
		// Take profit is always a gain, so use isGain=true to get gainQuoteHome factor
		priceMove, err := calculatePriceMoveForDollars(symbol, entryPrice, dollars, units, true)
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
		// Stop loss is always a loss, so use isGain=false to get lossQuoteHome factor
		priceMove, err := calculatePriceMoveForDollars(symbol, entryPrice, dollars, units, false)
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

// queryTradeCloseDetails returns both the close reason and the realized P/L for a closed trade
func queryTradeCloseDetails(tradeID string) (reason string, realizedPL float64, err error) {
	// Get recent transactions for this account
	url := fmt.Sprintf("%s/v3/accounts/%s/transactions", oandaBaseURL, oandaAccountID)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+oandaAPIKey)
	req.Header.Set("Content-Type", "application/json")

	q := req.URL.Query()
	q.Add("count", "100")
	req.URL.RawQuery = q.Encode()

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "UNKNOWN", 0, fmt.Errorf("failed to fetch transactions: %v", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "UNKNOWN", 0, fmt.Errorf("failed to parse transactions: %v", err)
	}

	transactions, ok := result["transactions"].([]interface{})
	if !ok {
		return "UNKNOWN", 0, fmt.Errorf("no transactions found")
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
				// Found the close transaction
				txnReason, _ := txn["reason"].(string)

				// Get realized P/L from the trade close details
				if plStr, ok := tcMap["realizedPL"].(string); ok {
					fmt.Sscanf(plStr, "%f", &realizedPL)
				}

				// Determine close reason
				if txnReason == "TAKE_PROFIT_ORDER" || strings.Contains(txnReason, "TAKE_PROFIT") {
					return "TAKE_PROFIT", realizedPL, nil
				} else if txnReason == "STOP_LOSS_ORDER" || strings.Contains(txnReason, "STOP_LOSS") {
					return "STOP_LOSS", realizedPL, nil
				} else {
					return "MANUAL", realizedPL, nil
				}
			}
		}
	}

	return "UNKNOWN", 0, nil
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
			// Trade was closed by OANDA - get close reason and realized P/L
			closeReason, realizedPL, err := queryTradeCloseDetails(botTrade.tradeID)
			if err != nil {
				log.Printf("⚠️  [CHECK] Could not determine close details for %s: %v", botTrade.tradeID, err)
				closeReason = "UNKNOWN"
			}

			// Track daily profit if enabled
			if dailyProfitTargetEnabled && realizedPL != 0 {
				addDailyRealizedProfit(realizedPL)
			}

			mu.Lock()
			state := positions[botTrade.symbol]
			positionType := state.Position

			// Update bot state
			state.PositionOpen = false
			state.Position = ""
			state.TradeID = ""
			state.OandaRealizedPL = fmt.Sprintf("%.2f", realizedPL)
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
			plColor := "🟢"
			plSign := "+"
			if realizedPL < 0 {
				plColor = "🔴"
				plSign = ""
			}

			if closeReason == "TAKE_PROFIT" {
				log.Println(strings.Repeat("🎯", 40))
				log.Printf("🎯 TAKE PROFIT HIT - %s %s", strings.ToUpper(positionType), botTrade.symbol)
				log.Println(strings.Repeat("🎯", 40))
				log.Printf("Trade ID: %s", botTrade.tradeID)
				log.Printf("Target: $%s profit", takeProfitDollars)
				log.Printf("💰 Realized P/L: %s %s$%.2f", plColor, plSign, realizedPL)
				log.Println(strings.Repeat("🎯", 40))
				log.Printf("✅ Position closed automatically by OANDA")
			} else if closeReason == "STOP_LOSS" {
				log.Println(strings.Repeat("🛑", 40))
				log.Printf("🛑 STOP LOSS HIT - %s %s", strings.ToUpper(positionType), botTrade.symbol)
				log.Println(strings.Repeat("🛑", 40))
				log.Printf("Trade ID: %s", botTrade.tradeID)
				log.Printf("Loss limit: $%s", stopLossDollars)
				log.Printf("💰 Realized P/L: %s %s$%.2f", plColor, plSign, realizedPL)
				log.Println(strings.Repeat("🛑", 40))
				log.Printf("✅ Position closed automatically by OANDA")
			} else {
				log.Printf("ℹ️  [SYNC] Position closed in OANDA: %s %s (ID: %s, Reason: %s, P/L: %s%s$%.2f)",
					strings.ToUpper(positionType), botTrade.symbol, botTrade.tradeID, closeReason, plSign, "", realizedPL)
			}

			log.Printf("💾 [STATE] Position updated - Open=false, Type='', TradeID='', Flags cleared")
		}
	}
}

// Reset all indicator states for a symbol (requires fresh webhooks to re-enter)
func resetIndicatorStates(state *PositionState) {
	// NOTE: We no longer reset indicator states on position close.
	// Indicator states (like PriceAboveEMA200, RSIAbove50, etc.) reflect actual market conditions
	// that don't change just because a position was closed. Higher timeframe indicators (4H)
	// take a long time to get new events, so clearing them causes unnecessary delays.
	//
	// We only reset entry/exit condition tracking to require fresh confirmation.

	// Clear entry/exit condition tracking to require fresh webhook confirmation
	state.EntryConditionsCompleted = make(map[string]bool)
	state.ExitConditionsCompleted = make(map[string]bool)

	// Reset one-time cross/event flags that should only trigger once per setup
	// These are events that happened in the past and shouldn't carry over
	state.MACDCrossedUp = false
	state.MACDCrossedDown = false
	state.EMA9CrossedUpEMA21 = false
	state.EMA9CrossedDownEMA21 = false
	state.MA1CrossedAboveMA2 = false
	state.MA1CrossedBelowMA2 = false
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
	state.StochRSICrossedUp20 = false
	state.StochRSICrossedDown20 = false
	state.StochRSICrossedUp50 = false
	state.StochRSICrossedDown50 = false
	state.StochRSICrossedUp80 = false
	state.StochRSICrossedDown80 = false

	// Reset ATR cross flags (require fresh cross for next trade)
	state.ATRLongCrossed = false
	state.ATRShortCrossed = false

	// PRESERVED persistent state indicators - these reflect current market conditions:
	// - PriceAboveEMA200, PriceBelowEMA200 (MA4 - 4H timeframe, takes 4hrs to repopulate)
	// - PriceAboveEMA50, PriceBelowEMA50
	// - PriceAboveEMA20, PriceBelowEMA20
	// - PriceAboveEMA9, PriceBelowEMA9
	// - RSIAbove50, RSIBelow50
	// - MACDAboveSignal, MACDBelowSignal
	// - EMA9AboveEMA21, EMA9BelowEMA21 (MA1/MA2)
	// - MA2AboveMA3, MA2BelowMA3
	// - ATRAboveThreshold, ATRBelowThreshold, ATRAboveAverage, ATRBelowAverage
	// - StochInOversold, StochInOverbought
	// - MACDHistIncreasing, MACDHistDecreasing, MACDHistAboveZero, MACDHistBelowZero
	// - MARibbonBullish, MARibbonBearish
	// - SMCLowLow, SMCHighLow, SMCLowHigh, SMCHighHigh
}

// Fetch existing open positions from OANDA on startup
func syncPositionsFromOanda() error {
	return syncPositionsFromOandaWithLogging(true)
}

// Periodic sync - less verbose logging
func periodicSyncPositionsFromOanda() error {
	return syncPositionsFromOandaWithLogging(false)
}

func syncPositionsFromOandaWithLogging(verbose bool) error {
	if verbose {
		log.Printf("🔄 [SYNC] Fetching open positions from OANDA...")
	}

	url := fmt.Sprintf("%s/v3/accounts/%s/openTrades", oandaBaseURL, oandaAccountID)
	if verbose {
		log.Printf("📤 [OANDA] GET %s", url)
	}

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+oandaAPIKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("❌ [SYNC] Failed to fetch open trades: %v", err)
		return err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("❌ [SYNC] Failed to parse OANDA response: %v", err)
		return err
	}

	if verbose {
		log.Printf("📥 [OANDA] Response status: %d", resp.StatusCode)
	}

	if resp.StatusCode != 200 {
		log.Printf("❌ [SYNC] Failed to fetch positions (status %d)", resp.StatusCode)
		return fmt.Errorf("failed to fetch positions: status %d", resp.StatusCode)
	}

	trades, ok := result["trades"].([]interface{})
	if !ok {
		trades = []interface{}{} // Empty trades list
	}

	if verbose {
		log.Printf("📊 [SYNC] Found %d open trade(s)", len(trades))
	}

	// Build a map of current OANDA positions
	oandaPositions := make(map[string]struct {
		tradeID      string
		positionType string
		units        string
	})

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

		var positionType string
		if currentUnits[0] == '-' {
			positionType = "short"
		} else {
			positionType = "long"
		}

		oandaPositions[instrument] = struct {
			tradeID      string
			positionType string
			units        string
		}{tradeID, positionType, currentUnits}
	}

	mu.Lock()
	defer mu.Unlock()

	// Check for positions that were closed in OANDA (stop loss, take profit, manual close)
	for symbol, state := range positions {
		if state.PositionOpen {
			if _, exists := oandaPositions[symbol]; !exists {
				// Position was closed in OANDA but we think it's open
				log.Printf("🔔 [SYNC] Detected position CLOSED in OANDA: %s %s (ID: %s)",
					strings.ToUpper(state.Position), symbol, state.TradeID)
				log.Printf("   💡 Position may have been closed by: Stop Loss, Take Profit, or Manual Close")

				// Update local state
				state.PositionOpen = false
				state.Position = "none"
				state.TradeID = ""
				state.PositionOpening = false

				// Clear entry conditions for potential re-entry
				state.EntryConditionsCompleted = make(map[string]bool)
				state.ExitConditionsCompleted = make(map[string]bool)

				// CRITICAL: Reset ATR direction state to prevent false re-entry
				// When position is closed externally (manual close, SL, TP), we need to
				// reset the ATR direction tracking so the next ATR signal is treated as
				// initialization rather than a cross. This prevents the bug where price
				// whipsaws and briefly crosses the ATR stop then flips back, causing
				// an unwanted position re-open.
				state.ATRDirectionInitialized = false
				state.ATRDirectionLong = false
				state.ATRLong = false
				state.ATRShort = false
				state.ATRLongCrossed = false
				state.ATRShortCrossed = false
				state.ATRFlipLong = false
				state.ATRFlipShort = false
				log.Printf("   🔄 Reset ATR direction state to prevent false re-entry on whipsaw")

				// If one-trade mode, the strategy should already be disabled
				// But if position was closed externally, we might want to keep strategy enabled
				// for manual trades - leave strategy state as is
			}
		}
	}

	// Check for positions opened directly in OANDA
	for instrument, pos := range oandaPositions {
		state, exists := positions[instrument]
		if !exists {
			// Create new state for this instrument
			newState := &PositionState{
				Symbol:                   instrument,
				PositionOpen:             true,
				Position:                 pos.positionType,
				TradeID:                  pos.tradeID,
				EntryConditionsCompleted: make(map[string]bool),
				ExitConditionsCompleted:  make(map[string]bool),
			}
			positions[instrument] = newState
			log.Printf("🔔 [SYNC] Detected NEW position opened in OANDA: %s %s (ID: %s, Units: %s)",
				strings.ToUpper(pos.positionType), instrument, pos.tradeID, pos.units)
		} else if !state.PositionOpen {
			// We thought position was closed, but it's open in OANDA
			state.PositionOpen = true
			state.Position = pos.positionType
			state.TradeID = pos.tradeID
			state.PositionOpening = false
			// If ATR is currently idle, mark this position as opened during idle
			// so it won't be closed by subsequent idle signals - wait for ATR cross
			if state.ATRIdle {
				state.PositionOpenedWhileIdle = true
				log.Printf("🔔 [SYNC] Detected NEW position opened in OANDA during ATR IDLE: %s %s (ID: %s, Units: %s)",
					strings.ToUpper(pos.positionType), instrument, pos.tradeID, pos.units)
				log.Printf("   💡 Position will be held until next ATR direction change (long/short cross)")
			} else {
				log.Printf("🔔 [SYNC] Detected NEW position opened in OANDA: %s %s (ID: %s, Units: %s)",
					strings.ToUpper(pos.positionType), instrument, pos.tradeID, pos.units)
			}
		} else if state.TradeID != pos.tradeID {
			// Different trade ID - position was closed and reopened
			log.Printf("🔔 [SYNC] Detected position REPLACED in OANDA: %s %s (Old ID: %s → New ID: %s)",
				strings.ToUpper(pos.positionType), instrument, state.TradeID, pos.tradeID)
			state.TradeID = pos.tradeID
			state.Position = pos.positionType
			// If ATR is idle, mark as opened during idle
			if state.ATRIdle {
				state.PositionOpenedWhileIdle = true
				log.Printf("   💡 Position replaced during ATR IDLE - will be held until next ATR direction change")
			}
		}
	}

	if verbose {
		log.Printf("✅ [SYNC] Position sync complete")
	}
	return nil
}

func openLongPosition(symbol string, price string) {
	mu.Lock()
	state := positions[symbol]

	// Double-check position isn't already open OR opening (race condition protection)
	if state.PositionOpen {
		mu.Unlock()
		log.Printf("⚠️  [RACE] Position already open for %s - skipping duplicate open request", symbol)
		return
	}
	if state.PositionOpening {
		mu.Unlock()
		log.Printf("⚠️  [RACE] Position already opening for %s - skipping duplicate open request", symbol)
		return
	}

	// Set PositionOpening flag to block other concurrent open attempts
	state.PositionOpening = true
	exchange := state.Exchange
	mu.Unlock()

	// Ensure we clear PositionOpening flag on exit (success or failure)
	defer func() {
		mu.Lock()
		positions[symbol].PositionOpening = false
		mu.Unlock()
	}()

	// Determine if this will be a simulated trade (forex pairs always use OANDA)
	isSimulated := !shouldUseOANDA(symbol, exchange)

	// Check if any position is already open on the SAME exchange type
	mu.RLock()
	for sym, s := range positions {
		if s.PositionOpen {
			// Only block if same exchange type (both OANDA or both simulated)
			otherIsSimulated := !shouldUseOANDA(sym, s.Exchange)
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

	// Check if this is a non-OANDA symbol (simulated trade)
	// Forex pairs always use OANDA regardless of exchange field from TradingView
	if !shouldUseOANDA(symbol, exchange) {
		// Calculate position size with appropriate leverage for simulation
		var positionSize float64
		var positionUnits int

		// Determine leverage based on symbol type
		// Forex pairs typically use 50:1, crypto uses 10:1
		leverage := 10.0 // Default for crypto
		if isForexPair(symbol) {
			leverage = 50.0 // Forex leverage
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
	resetIndicatorStates(state)
	mu.Unlock()

	log.Printf("✅ LONG position opened: %s (ID: %s)", symbol, tradeID)
	log.Printf("💾 [STATE] Position updated - Open=%v, Type=%s, TradeID=%s, All flags cleared",
		state.PositionOpen, state.Position, state.TradeID)
}

func openShortPosition(symbol string, price string) {
	mu.Lock()
	state := positions[symbol]

	// Double-check position isn't already open OR opening (race condition protection)
	if state.PositionOpen {
		mu.Unlock()
		log.Printf("⚠️  [RACE] Position already open for %s - skipping duplicate open request", symbol)
		return
	}
	if state.PositionOpening {
		mu.Unlock()
		log.Printf("⚠️  [RACE] Position already opening for %s - skipping duplicate open request", symbol)
		return
	}

	// Set PositionOpening flag to block other concurrent open attempts
	state.PositionOpening = true
	exchange := state.Exchange
	mu.Unlock()

	// Ensure we clear PositionOpening flag on exit (success or failure)
	defer func() {
		mu.Lock()
		positions[symbol].PositionOpening = false
		mu.Unlock()
	}()

	// Determine if this will be a simulated trade (forex pairs always use OANDA)
	isSimulated := !shouldUseOANDA(symbol, exchange)

	// Check if any position is already open on the SAME exchange type
	mu.RLock()
	for sym, s := range positions {
		if s.PositionOpen {
			// Only block if same exchange type (both OANDA or both simulated)
			otherIsSimulated := !shouldUseOANDA(sym, s.Exchange)
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
	mu.Unlock()

	// Check if this is a non-OANDA symbol (simulated trade)
	// Forex pairs always use OANDA regardless of exchange field from TradingView
	if !shouldUseOANDA(symbol, exchange) {
		// Calculate position size with appropriate leverage for simulation
		var positionSize float64
		var positionUnits int

		// Determine leverage based on symbol type
		// Forex pairs typically use 50:1, crypto uses 10:1
		leverage := 10.0 // Default for crypto
		if isForexPair(symbol) {
			leverage = 50.0 // Forex leverage
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

		// Disable strategy after completing one trade cycle (if one trade mode enabled)
		if activeStrategy.OneTradeMode {
			strategyEnabled = false
			log.Printf("🛑 [STRATEGY] Strategy DISABLED after completing trade (oneTradeMode=true) - re-enable via /enable-strategy")
		}
		mu.Unlock()

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

	// Parse response to extract realized P/L
	var responseBody map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&responseBody)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		// Extract realized P/L from OANDA response
		var realizedPL float64
		if orderFillTxn, ok := responseBody["orderFillTransaction"].(map[string]interface{}); ok {
			if plStr, ok := orderFillTxn["pl"].(string); ok {
				fmt.Sscanf(plStr, "%f", &realizedPL)
				log.Printf("💰 [OANDA] Realized P/L: $%.2f", realizedPL)

				// Track daily profit
				if dailyProfitTargetEnabled {
					addDailyRealizedProfit(realizedPL)
				}
			}
		}

		mu.Lock()
		state.PositionOpen = false
		state.Position = "none"
		state.OandaRealizedPL = fmt.Sprintf("%.2f", realizedPL)
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

		// Disable strategy after completing one trade cycle (if one trade mode enabled)
		if activeStrategy.OneTradeMode {
			strategyEnabled = false
		}
		mu.Unlock()

		log.Printf("✅ Position closed: %s", symbol)
		log.Printf("💾 [STATE] Position updated - Open=%v, Type=%s", state.PositionOpen, state.Position)
		if activeStrategy.OneTradeMode {
			log.Printf("🛑 [STRATEGY] Strategy DISABLED after completing trade (oneTradeMode=true) - re-enable via /enable-strategy")
		}
	} else {
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
	// Check if trading hours just opened and execute any pending positions
	checkTradingHoursTransition()

	// Show status report after each webhook event
	reportStrategyStatus()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"message": message,
	})
}

// checkTradingHoursTransition checks if trading hours just opened and logs the transition
func checkTradingHoursTransition() {
	if !tradingHoursEnabled {
		return
	}

	currentlyWithinHours := isWithinTradingHours()

	// Detect transition from closed to open
	if currentlyWithinHours && !wasWithinTradingHours {
		log.Printf("\n" + strings.Repeat("=", 80))
		log.Printf("🔔 TRADING HOURS OPENED - %s", getActiveSessionName())
		log.Printf("   New positions now allowed. Waiting for fresh entry signals...")
		log.Printf(strings.Repeat("=", 80) + "\n")

		// Reset ALL ATR state so first signal initializes, second signal triggers
		// This prevents whipsaw trading when hours open with stale state
		mu.Lock()
		for _, state := range positions {
			// Reset direction tracking (for flip detection)
			state.ATRDirectionInitialized = false
			state.ATRFlipLong = false
			state.ATRFlipShort = false
			// Reset ATR state (for idle-aware long/short)
			// This ensures first signal after hours open is treated as initialization
			state.ATRLong = false
			state.ATRShort = false
			state.ATRIdle = false
			state.ATRLongCrossed = false
			state.ATRShortCrossed = false
			log.Printf("   🔄 Reset ATR state for %s (will wait for fresh cross)", state.Symbol)
		}
		mu.Unlock()
	}

	// Detect transition from open to closed
	if !currentlyWithinHours && wasWithinTradingHours {
		log.Printf("\n" + strings.Repeat("=", 80))
		log.Printf("🔒 TRADING HOURS CLOSED - New positions blocked")
		log.Printf("   Exit signals still processed. Entry conditions continue to be tracked.")
		log.Printf(strings.Repeat("=", 80) + "\n")
	}

	// Update the tracking variable
	wasWithinTradingHours = currentlyWithinHours
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
	enabled := strategyEnabled
	dailyProfit := dailyRealizedProfit
	dailyTarget := dailyProfitTarget
	dailyTargetEnabled := dailyProfitTargetEnabled
	dailyTargetReached := dailyProfitTargetReached
	mu.RUnlock()

	// Fetch real-time P/L from OANDA for open positions
	oandaPL := getOandaOpenPositionsPL()

	// Build response with enhanced P/L data
	response := map[string]interface{}{
		"positions":       positions,
		"strategyEnabled": enabled,
		"oneTradeMode":    activeStrategy.OneTradeMode,
		"strategyName":    activeStrategy.Name,
		"oandaPositions":  oandaPL,
	}

	// Add daily profit tracking if enabled
	if dailyTargetEnabled {
		response["dailyProfit"] = map[string]interface{}{
			"realized":      fmt.Sprintf("%.2f", dailyProfit),
			"target":        fmt.Sprintf("%.2f", dailyTarget),
			"targetReached": dailyTargetReached,
			"remaining":     fmt.Sprintf("%.2f", dailyTarget-dailyProfit),
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Enable strategy after manual intervention
func enableStrategyHandler(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	strategyEnabled = true
	mu.Unlock()

	log.Printf("✅ [STRATEGY] Strategy ENABLED - webhooks will be processed")
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":         true,
		"strategyEnabled": true,
		"message":         "Strategy enabled",
	})
}

// Disable strategy manually
func disableStrategyHandler(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	strategyEnabled = false
	mu.Unlock()

	log.Printf("🛑 [STRATEGY] Strategy DISABLED - webhooks will be ignored")
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":         true,
		"strategyEnabled": false,
		"message":         "Strategy disabled",
	})
}

// GET /daily-profit - Get current daily profit stats
func dailyProfitHandler(w http.ResponseWriter, r *http.Request) {
	mu.RLock()
	dailyProfit := dailyRealizedProfit
	dailyTarget := dailyProfitTarget
	dailyTargetEnabled := dailyProfitTargetEnabled
	dailyTargetReached := dailyProfitTargetReached
	resetTime := dailyProfitResetTime
	mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")

	if !dailyTargetEnabled {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"enabled": false,
			"message": "Daily profit target not configured. Set DAILY_PROFIT_TARGET env var.",
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"enabled":         true,
		"realized":        fmt.Sprintf("%.2f", dailyProfit),
		"target":          fmt.Sprintf("%.2f", dailyTarget),
		"remaining":       fmt.Sprintf("%.2f", dailyTarget-dailyProfit),
		"targetReached":   dailyTargetReached,
		"resetTime":       resetTime.Format(time.RFC3339),
		"percentOfTarget": fmt.Sprintf("%.1f%%", (dailyProfit/dailyTarget)*100),
	})
}

// POST /reset-daily-profit - Reset daily profit counter
func resetDailyProfitHandler(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	dailyRealizedProfit = 0
	dailyProfitTargetReached = false
	dailyProfitResetTime = getLocalTime()

	// Also re-enable strategy if it was disabled due to daily target
	if dailyProfitTargetEnabled {
		strategyEnabled = true
	}
	mu.Unlock()

	log.Printf("🔄 [DAILY] Daily profit counter manually reset to $0.00")
	log.Printf("✅ [STRATEGY] Strategy re-enabled")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":         true,
		"message":         "Daily profit reset to $0.00 and strategy re-enabled",
		"strategyEnabled": true,
		"dailyProfit":     "0.00",
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
	if tradingHoursEnabled {
		if isWithinTradingHours() {
			log.Printf("Trading Hours: ✅ OPEN - %s", getActiveSessionName())
		} else {
			log.Printf("Trading Hours: 🔒 CLOSED (new positions blocked)")
		}
	}
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

	totalCount := len(entryConditions.Conditions)
	metCount := 0
	readyCount := 0

	for i, condition := range entryConditions.Conditions {
		var isMet bool
		if condition.Type == "condition" && condition.Webhook != "" {
			isMet = isConditionCurrentlyMet(condition.Webhook, state)
		} else {
			key := fmt.Sprintf("%scondition_%d", prefix, i)
			isMet = state.EntryConditionsCompleted[key]
		}

		if isMet {
			metCount++
		}

		status := "❌"
		statusSuffix := ""
		waitingForCross := false

		if condition.Type == "condition" && condition.Webhook != "" && requiresCrossEvent(condition.Webhook) {
			// For cross-required conditions
			hasCrossed := hasCrossedRecently(condition.Webhook, state)
			if isMet && hasCrossed {
				// Crossed and met - fully ready
				status = "✅"
			} else if isMet && !hasCrossed {
				// In correct direction but hasn't crossed yet
				status = "⏳"
				statusSuffix = " (waiting for cross)"
				waitingForCross = true
			}
			// else stays ❌ (opposite direction)
		} else if isMet {
			status = "✅"
		}

		if isMet && !waitingForCross {
			readyCount++
		}

		description := getNodeDescription(&condition)
		log.Printf("    %s [%d/%d] %s%s", status, i+1, totalCount, description, statusSuffix)
	}

	log.Printf("  Summary: %d/%d conditions met", metCount, totalCount)

	if readyCount == totalCount && totalCount > 0 {
		log.Printf("  🎯 READY: All %s entry conditions are satisfied!", direction)
	} else if metCount > 0 {
		waitingCount := metCount - readyCount
		missingCount := totalCount - metCount
		if waitingCount > 0 && missingCount > 0 {
			log.Printf("  ⏳ WAITING: Need %d more condition(s) + %d waiting for cross for %s entry", missingCount, waitingCount, direction)
		} else if waitingCount > 0 {
			log.Printf("  ⏳ WAITING: %d condition(s) waiting for cross for %s entry", waitingCount, direction)
		} else {
			log.Printf("  ⏳ WAITING: Need %d more condition(s) for %s entry", missingCount, direction)
		}
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
		statusSuffix := ""

		if condition.Type == "condition" && condition.Webhook != "" && requiresCrossEvent(condition.Webhook) {
			// For cross-required exit conditions, only show ✅ if actually crossed
			if isMet && hasCrossedRecently(condition.Webhook, state) {
				status = "✅"
			} else if isPendingCross(condition.Webhook, state) {
				// We're in opposite direction, waiting for cross to exit
				status = "⏳"
				statusSuffix = " (waiting for cross)"
			}
			// else stays ❌
		} else if isMet {
			status = "✅"
		}

		description := getNodeDescription(&condition)
		log.Printf("    %s [%d/%d] %s%s", status, i+1, totalCount, description, statusSuffix)
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

	// Set OANDA API URL based on OANDA_LIVE environment variable
	if os.Getenv("OANDA_LIVE") == "true" {
		oandaBaseURL = "https://api-fxtrade.oanda.com"
		log.Println("� LIVE TRADING MODE - Using production OANDA API")
	} else {
		oandaBaseURL = "https://api-fxpractice.oanda.com"
		log.Println("🧪 PRACTICE MODE - Using demo OANDA API")
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

	// Daily profit target configuration
	dailyProfitTargetStr := os.Getenv("DAILY_PROFIT_TARGET")
	if dailyProfitTargetStr != "" {
		if target, err := strconv.ParseFloat(dailyProfitTargetStr, 64); err == nil && target > 0 {
			dailyProfitTarget = target
			dailyProfitTargetEnabled = true
			dailyProfitResetTime = getLocalTime()
			log.Printf("🎯 Daily Profit Target: $%.2f (trading will stop when reached)", dailyProfitTarget)
		} else {
			log.Printf("⚠️  Invalid DAILY_PROFIT_TARGET value '%s', feature disabled", dailyProfitTargetStr)
		}
	}

	// Strategy configuration
	strategyName = os.Getenv("STRATEGY_FILE")
	if strategyName == "" {
		strategyName = "ma_trend_rsi_atr" // Use default strategy if not specified
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

	// Trading hours configuration
	tradingStartHourStr := os.Getenv("TRADING_START_HOUR")
	tradingEndHourStr := os.Getenv("TRADING_END_HOUR")
	tradingDaysStr := os.Getenv("TRADING_DAYS")
	tradingTimezoneStr := os.Getenv("TRADING_TIMEZONE")

	// Parse trading hours if specified
	if tradingStartHourStr != "" || tradingEndHourStr != "" {
		tradingHoursEnabled = true

		// Parse start time (supports "9" or "9:30" format)
		if tradingStartHourStr != "" {
			hour, minute, err := parseTimeString(tradingStartHourStr)
			if err == nil {
				tradingStartHour = hour
				tradingStartMinute = minute
			} else {
				log.Printf("⚠️  Invalid TRADING_START_HOUR '%s': %v, using 00:00", tradingStartHourStr, err)
				tradingStartHour = 0
				tradingStartMinute = 0
			}
		}

		// Parse end time (supports "17" or "16:30" format, default: 24:00)
		if tradingEndHourStr != "" {
			hour, minute, err := parseTimeString(tradingEndHourStr)
			if err == nil {
				tradingEndHour = hour
				tradingEndMinute = minute
			} else {
				log.Printf("⚠️  Invalid TRADING_END_HOUR '%s': %v, using 24:00", tradingEndHourStr, err)
				tradingEndHour = 24
				tradingEndMinute = 0
			}
		} else {
			tradingEndHour = 24
			tradingEndMinute = 0
		}

		// Parse trading days
		if days, err := parseTradingDays(tradingDaysStr); err != nil {
			log.Printf("⚠️  Invalid TRADING_DAYS '%s': %v, allowing all days", tradingDaysStr, err)
			tradingDays = nil
		} else {
			tradingDays = days
		}

		// Parse trading timezone
		if tradingTimezoneStr != "" {
			if loc, err := time.LoadLocation(tradingTimezoneStr); err == nil {
				tradingTimezone = loc
			} else {
				log.Printf("⚠️  Invalid TRADING_TIMEZONE '%s': %v, using UTC offset", tradingTimezoneStr, err)
			}
		}

		// Parse Session 2 (optional)
		tradingStartHour2Str := os.Getenv("TRADING_START_HOUR_2")
		tradingEndHour2Str := os.Getenv("TRADING_END_HOUR_2")

		if tradingStartHour2Str != "" && tradingEndHour2Str != "" {
			session2Enabled = true

			// Parse session 2 start time
			hour, minute, err := parseTimeString(tradingStartHour2Str)
			if err == nil {
				tradingStartHour2 = hour
				tradingStartMinute2 = minute
			} else {
				log.Printf("⚠️  Invalid TRADING_START_HOUR_2 '%s': %v, disabling session 2", tradingStartHour2Str, err)
				session2Enabled = false
			}

			// Parse session 2 end time
			if session2Enabled {
				hour, minute, err := parseTimeString(tradingEndHour2Str)
				if err == nil {
					tradingEndHour2 = hour
					tradingEndMinute2 = minute
				} else {
					log.Printf("⚠️  Invalid TRADING_END_HOUR_2 '%s': %v, disabling session 2", tradingEndHour2Str, err)
					session2Enabled = false
				}
			}
		}

		log.Printf("🕐 %s", getTradingHoursStatus())
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

	// Show trading hours settings
	if tradingHoursEnabled {
		log.Println("🕐 Trading Hours Configuration:")
		log.Printf("   Session 1: %s - %s",
			formatTimeAMPM(tradingStartHour, tradingStartMinute),
			formatTimeAMPM(tradingEndHour, tradingEndMinute))
		if session2Enabled {
			log.Printf("   Session 2: %s - %s",
				formatTimeAMPM(tradingStartHour2, tradingStartMinute2),
				formatTimeAMPM(tradingEndHour2, tradingEndMinute2))
		}

		// Show timezone
		tzName := "UTC"
		if tradingTimezone != nil {
			tzName = tradingTimezone.String()
		} else if timezoneOffset != 0 {
			tzName = fmt.Sprintf("UTC%+d", timezoneOffset)
		}

		// Show days
		dayNames := []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}
		var allowedDays []string
		if len(tradingDays) == 0 {
			allowedDays = dayNames
		} else {
			for _, d := range tradingDays {
				if d >= 0 && d <= 6 {
					allowedDays = append(allowedDays, dayNames[d])
				}
			}
		}
		log.Printf("   Timezone: %s | Days: %s", tzName, strings.Join(allowedDays, ", "))

		if isWithinTradingHours() {
			log.Printf("   ✅ Status: OPEN - %s", getActiveSessionName())
		} else {
			log.Printf("   🔒 Status: CLOSED (new positions blocked)")
		}
	} else {
		log.Printf("🕐 Trading Hours: No restrictions (24/7)")
	}

	// Sync existing positions from OANDA on startup
	if err := syncPositionsFromOanda(); err != nil {
		log.Printf("⚠️  [WARNING] Could not sync positions from OANDA: %v", err)
		log.Printf("⚠️  [WARNING] Continuing with empty state - be careful of duplicate positions!")
	}

	// Initialize trading hours state and reset ATR direction state for fresh session
	// This ensures the first signal after startup/restart initializes state, not trades
	if tradingHoursEnabled {
		wasWithinTradingHours = isWithinTradingHours()
		if wasWithinTradingHours {
			log.Printf("🔄 [STARTUP] Trading hours OPEN - resetting ATR state for fresh signals")
			mu.Lock()
			for _, state := range positions {
				// Reset direction tracking (for flip detection)
				state.ATRDirectionInitialized = false
				state.ATRFlipLong = false
				state.ATRFlipShort = false
				// Reset ATR state (for idle-aware long/short)
				state.ATRLong = false
				state.ATRShort = false
				state.ATRIdle = false
				state.ATRLongCrossed = false
				state.ATRShortCrossed = false
			}
			mu.Unlock()
		}
	}

	// Start periodic OANDA sync (every 5 minutes)
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			if err := periodicSyncPositionsFromOanda(); err != nil {
				log.Printf("⚠️  [SYNC] Periodic sync failed: %v", err)
			}
		}
	}()
	log.Printf("🔄 [SYNC] Periodic OANDA sync enabled (every 5 minutes)")

	// Health & Status
	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/status", statusHandler)
	http.HandleFunc("/enable-strategy", enableStrategyHandler)
	http.HandleFunc("/disable-strategy", disableStrategyHandler)
	http.HandleFunc("/daily-profit", dailyProfitHandler)
	http.HandleFunc("/reset-daily-profit", resetDailyProfitHandler)

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

	// ATR Threshold Webhooks
	http.HandleFunc("/webhook/atr/above-threshold", handleATRAboveThreshold)
	http.HandleFunc("/webhook/atr/below-threshold", handleATRBelowThreshold)
	http.HandleFunc("/webhook/atr/flip-long", handleATRFlipLong)
	http.HandleFunc("/webhook/atr/flip-short", handleATRFlipShort)
	http.HandleFunc("/webhook/atr/long", handleATRLong)
	http.HandleFunc("/webhook/atr/short", handleATRShort)
	http.HandleFunc("/webhook/atr/idle", handleATRIdle)

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
	http.HandleFunc("/webhook/ma/ma1-above-ma4", handleMA1AboveMA4) // Position tracking
	http.HandleFunc("/webhook/ma/ma1-below-ma4", handleMA1BelowMA4) // Position tracking

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
