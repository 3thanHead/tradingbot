package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
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
}

// Global state management
var (
	positions = make(map[string]*PositionState)
	mu        sync.RWMutex

	oandaAPIKey    = os.Getenv("OANDA_API_KEY")
	oandaAccountID = os.Getenv("OANDA_ACCOUNT_ID")
	oandaBaseURL   = "https://api-fxpractice.oanda.com" // Change to api-fxtrade.oanda.com for live
	tradeUnits     string                               // Trading units (fixed amount)
	tradeUSDAmount string                               // USD notional amount (calculates units from price)
	tradeMargin    string                               // Margin amount (OANDA calculates position size based on leverage)
	takeProfitPips string                               // Take profit in pips (e.g., "50")
	takeProfitPct  string                               // Take profit in percentage (e.g., "2.5" for 2.5%)
)

// Get or create position state for a symbol
func getPositionState(symbol string) *PositionState {
	mu.Lock()
	defer mu.Unlock()

	if _, exists := positions[symbol]; !exists {
		positions[symbol] = &PositionState{
			Symbol:       symbol,
			PositionOpen: false,
			Position:     "none",
		}
	}
	return positions[symbol]
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
	log.Printf("🔍 [STATE] Before - RSICrossedUp: %v, RSICrossedDown: %v, PositionOpen: %v",
		state.RSICrossedUp, state.RSICrossedDown, state.PositionOpen)

	// Check if we should close a LONG position (RSI extreme after center cross warning)
	if state.PositionOpen && state.Position == "long" && state.RSICrossedCenter {
		log.Printf("⚠️ [EXIT] RSI > 70 after center cross warning → closing LONG position")
		mu.Lock()
		state.RSICrossedCenter = false // Reset the warning flag
		mu.Unlock()
		closePosition(symbol)
		respondSuccess(w, "RSI > 70 after warning → closed LONG position")
		return
	}

	// Set the condition flag
	mu.Lock()
	state.RSICrossedUp = true
	state.RSICrossedDown = false // Reset opposite condition
	mu.Unlock()

	log.Printf("✅ [STATE] After - RSICrossedUp: %v, RSICrossedDown: %v", state.RSICrossedUp, state.RSICrossedDown)

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
	log.Printf("🔍 [STATE] Before - RSICrossedUp: %v, RSICrossedDown: %v, PositionOpen: %v",
		state.RSICrossedUp, state.RSICrossedDown, state.PositionOpen)

	// Check if we should close a SHORT position (RSI extreme after center cross warning)
	if state.PositionOpen && state.Position == "short" && state.RSICrossedCenter {
		log.Printf("⚠️ [EXIT] RSI < 30 after center cross warning → closing SHORT position")
		mu.Lock()
		state.RSICrossedCenter = false // Reset the warning flag
		mu.Unlock()
		closePosition(symbol)
		respondSuccess(w, "RSI < 30 after warning → closed SHORT position")
		return
	}

	// Set the condition flag
	mu.Lock()
	state.RSICrossedDown = true
	state.RSICrossedUp = false // Reset opposite condition
	mu.Unlock()

	log.Printf("✅ [STATE] After - RSICrossedUp: %v, RSICrossedDown: %v", state.RSICrossedUp, state.RSICrossedDown)

	respondSuccess(w, "RSI < 30 condition set")
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

	// Check if we should close a SHORT position (MACD reversal)
	if state.PositionOpen && state.Position == "short" {
		mu.Unlock()
		log.Printf("⚠️ [EXIT] MACD crossed up while SHORT → reversal signal, closing position")
		closePosition(symbol)
		respondSuccess(w, "MACD cross up reversal → closed SHORT position")
		return
	}

	// NEW LOGIC: If RSI < 30 condition was set, open LONG position
	if state.RSICrossedDown && !state.PositionOpen {
		mu.Unlock() // Unlock before opening position
		log.Printf("🔍 [CONDITION CHECK] RSICrossedDown=true + MACDCrossedUp=true, PositionOpen=false")
		log.Printf("✅ [TRADE] Conditions met! Opening LONG position")
		openLongPosition(symbol, event.Close)

		// Reset the RSI flag after opening position
		mu.Lock()
		state.RSICrossedDown = false
		mu.Unlock()

		respondSuccess(w, "MACD cross up + RSI oversold → LONG opened")
		return
	}
	mu.Unlock()

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

	// Check if we should close a LONG position (MACD reversal)
	if state.PositionOpen && state.Position == "long" {
		mu.Unlock()
		log.Printf("⚠️ [EXIT] MACD crossed down while LONG → reversal signal, closing position")
		closePosition(symbol)
		respondSuccess(w, "MACD cross down reversal → closed LONG position")
		return
	}

	// NEW LOGIC: If RSI > 70 condition was set, open SHORT position
	if state.RSICrossedUp && !state.PositionOpen {
		mu.Unlock() // Unlock before opening position
		log.Printf("🔍 [CONDITION CHECK] RSICrossedUp=true + MACDCrossedDown=true, PositionOpen=false")
		log.Printf("✅ [TRADE] Conditions met! Opening SHORT position")
		openShortPosition(symbol, event.Close)

		// Reset the RSI flag after opening position
		mu.Lock()
		state.RSICrossedUp = false
		mu.Unlock()

		respondSuccess(w, "MACD cross down + RSI overbought → SHORT opened")
		return
	}
	mu.Unlock()

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
	log.Printf("🔍 [STATE] PositionOpen: %v, Position: %s, RSICrossedCenter: %v",
		state.PositionOpen, state.Position, state.RSICrossedCenter)

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

		// Second center cross → Close position
		position := state.Position
		state.RSICrossedCenter = false // Reset flag
		mu.Unlock()
		log.Printf("✅ [TRADE] Second RSI center cross! Closing %s position", strings.ToUpper(position))
		closePosition(symbol)
		respondSuccess(w, fmt.Sprintf("RSI crossed center (2nd time) → %s closed", strings.ToUpper(position)))
		return
	}
	mu.Unlock()

	log.Printf("ℹ️  [INFO] No position open for %s", symbol)
	respondSuccess(w, "RSI crossed center (no position to close)")
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

	// Default to 100 units if nothing specified
	if tradeUnits == "" && tradeUSDAmount == "" && tradeMargin == "" {
		tradeUnits = "100"
	}

	log.Println("🚀 TradingView Webhook Trading Bot Starting...")
	log.Printf("📡 OANDA Account: %s", oandaAccountID)
	log.Printf("🌐 OANDA API: %s", oandaBaseURL)

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

	// MACD Webhooks
	http.HandleFunc("/webhook/macd/cross-up", handleMACDCrossUp)
	http.HandleFunc("/webhook/macd/cross-down", handleMACDCrossDown)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("✅ Server listening on port %s", port)
	log.Println("\n📋 Webhook Endpoints:")
	log.Println("   POST /webhook/rsi/crossed-up       (RSI > 70)")
	log.Println("   POST /webhook/rsi/crossed-down     (RSI < 30)")
	log.Println("   POST /webhook/rsi/crossed-center   (Close any position)")
	log.Println("   POST /webhook/macd/cross-up        (Open LONG if RSI < 30)")
	log.Println("   POST /webhook/macd/cross-down      (Open SHORT if RSI > 70)")
	log.Println("\n📊 Monitoring:")
	log.Println("   GET /health")
	log.Println("   GET /status")

	// Fetch and display ngrok tunnel URL
	log.Println("\n🌐 Fetching ngrok tunnel URL...")
	time.Sleep(2 * time.Second) // Give ngrok time to start
	ngrokURL := getNgrokURL()
	if ngrokURL != "" {
		log.Printf("✅ Public ngrok URL: %s", ngrokURL)
		log.Println("\n📱 TradingView Webhook URLs:")
		log.Printf("   • RSI Crossed Up:     %s/webhook/rsi/crossed-up", ngrokURL)
		log.Printf("   • RSI Crossed Down:   %s/webhook/rsi/crossed-down", ngrokURL)
		log.Printf("   • RSI Crossed Center: %s/webhook/rsi/crossed-center", ngrokURL)
		log.Printf("   • MACD Cross Up:      %s/webhook/macd/cross-up", ngrokURL)
		log.Printf("   • MACD Cross Down:    %s/webhook/macd/cross-down", ngrokURL)
		log.Println("\n🖥️  ngrok Web Interface: http://localhost:4040")
	} else {
		log.Println("⚠️  Could not fetch ngrok URL (ngrok may not be running)")
		log.Println("   Run 'docker-compose up' to start ngrok automatically")
	}
	log.Println("")

	log.Fatal(http.ListenAndServe(":"+port, nil))
}
