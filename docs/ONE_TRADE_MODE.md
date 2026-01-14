# One Trade Mode

## Overview

The trading bot supports **One Trade Mode** as a per-strategy configuration. When enabled in a strategy file, the bot will automatically disable itself after completing one trade cycle (open + close), requiring manual intervention to re-enable.

## Configuration

Add `"oneTradeMode": true` to your strategy JSON file:

```json
{
  "name": "MA Trend + RSI 50 + MACD Strategy",
  "description": "...",
  "oneTradeMode": true,
  "long": {
    ...
  }
}
```

Set to `false` (or omit) for continuous trading mode where the strategy remains enabled after trades.

## How It Works

1. **Strategy starts enabled** - `strategyEnabled = true` on startup
2. **Webhooks are processed normally** - Bot monitors conditions and opens positions
3. **Position closes** - When exit conditions are met and position is closed
4. **Strategy automatically disables** - `strategyEnabled = false` 
5. **Webhooks are ignored** - All subsequent webhook events return early with "Strategy disabled"
6. **Manual re-enable required** - User must call `/enable-strategy` endpoint

## Key Features

- ✅ **No re-entry logic** - Removed all automatic re-entry checks after position close
- ✅ **Works for both simulated and OANDA trades** - Disables after any position close
- ✅ **Status endpoint shows state** - `/status` includes `"strategyEnabled": true/false`
- ✅ **Clear logging** - Shows when strategy is disabled and when webhooks are blocked

## API Endpoints

### Check Strategy Status
```bash
GET http://localhost:8080/status
```

Response:
```json
{
  "positions": {},
  "strategyEnabled": true,
  "oneTradeMode": true,
  "strategyName": "MA Trend + RSI 50 + MACD Strategy"
}
```

### Enable Strategy
```bash
POST http://localhost:8080/enable-strategy
```

Response:
```json
{
  "success": true,
  "strategyEnabled": true,
  "message": "Strategy enabled"
}
```

### Disable Strategy
```bash
POST http://localhost:8080/disable-strategy
```

Response:
```json
{
  "success": true,
  "strategyEnabled": false,
  "message": "Strategy disabled"
}
```

## Typical Workflow

1. Start bot (strategy enabled by default)
2. Wait for entry conditions to be met
3. Position opens automatically
4. Wait for exit conditions
5. Position closes automatically
6. **Strategy disables automatically** 🛑
7. Analyze trade results
8. When ready for next trade, call `/enable-strategy`
9. Repeat from step 2

## Log Messages

### When Position Closes (oneTradeMode=true)
```
✅ Position closed: EUR_USD
💾 [STATE] Position updated - Open=false, Type=none
🛑 [STRATEGY] Strategy DISABLED after completing trade (oneTradeMode=true) - re-enable via /enable-strategy
```

### When Position Closes (oneTradeMode=false)
```
✅ Position closed: EUR_USD
💾 [STATE] Position updated - Open=false, Type=none
```
(Strategy remains enabled, continues monitoring)

### When Strategy Loads (oneTradeMode=true)
```
⭐ STRATEGY LOADED: MA TREND + RSI 50 + MACD STRATEGY
📖 Long entry requires ALL: MA1 crosses above MA2...
🔒 One Trade Mode: ENABLED (strategy will disable after one trade cycle)
```

### When Strategy Loads (oneTradeMode=false)
```
⭐ STRATEGY LOADED: RSI 30/70 WITH MACD CONFIRMATION
📖 Long entry: RSI 30 + MACD up...
♻️  Continuous Mode: Strategy will continue monitoring after trades
```

### When Webhook Arrives (Strategy Disabled)
```
🛑 [STRATEGY] Strategy DISABLED - ignoring MACD Cross Up webhook
```

### When Manually Enabled
```
✅ [STRATEGY] Strategy ENABLED - webhooks will be processed
```

### When Manually Disabled
```
🛑 [STRATEGY] Strategy DISABLED - webhooks will be ignored
```

## Implementation Details

### Strategy Configuration
```json
{
  "name": "MA Trend + RSI 50 + MACD Strategy",
  "oneTradeMode": true,  // Enable one trade mode for this strategy
  "long": { ... },
  "short": { ... }
}
```

### Strategy Struct
```go
type Strategy struct {
    Name         string `json:"name"`
    Description  string `json:"description"`
    OneTradeMode bool   `json:"oneTradeMode"` // If true, strategy disables after one trade cycle
    // ... other fields
}
```

### Global Variable
```go
var (
    strategyEnabled = true // Strategy enabled flag - set to false after one trade cycle
)
```

### Webhook Check (Example)
```go
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
    // ... rest of webhook logic
}
```

### Auto-Disable on Position Close
```go
// In closePosition() function - Simulated Trade
mu.Lock()
state.LastClosedDirection = position
state.PositionOpen = false
state.Position = ""
state.TradeID = ""
// ... other cleanup

// Disable strategy after completing one trade cycle (if one trade mode enabled)
if activeStrategy.OneTradeMode {
    strategyEnabled = false
    log.Printf("🛑 [STRATEGY] Strategy DISABLED after completing trade (oneTradeMode=true)")
}
mu.Unlock()
```

## Benefits

- ✅ **Risk Control** - Prevents runaway trading scenarios
- ✅ **Manual Review** - Forces review of each trade before next entry
- ✅ **Testing Safety** - Ideal for testing strategies without continuous execution
- ✅ **Capital Preservation** - Gives trader time to adjust strategy if needed

## Migration Notes

Previous behavior:
- Bot would continuously monitor and re-enter positions
- Reversal protection logic allowed re-entry if opposite conditions occurred

New behavior:
- Bot executes ONE trade cycle then stops
- Manual `/enable-strategy` call required to resume trading
- All reversal and re-entry logic removed from closePosition

## Testing

Test the feature:
```bash
# 1. Check initial state
curl http://localhost:8080/status | jq .strategyEnabled

# 2. Manually disable
curl -X POST http://localhost:8080/disable-strategy | jq .

# 3. Verify webhooks are blocked
curl -X POST http://localhost:8080/webhook/macd/cross-up \
  -H "Content-Type: application/json" \
  -d '{"ticker":"EUR_USD","close":"1.05"}' | jq .

# 4. Re-enable
curl -X POST http://localhost:8080/enable-strategy | jq .

# 5. Verify enabled
curl http://localhost:8080/status | jq .strategyEnabled
```

## Future Enhancements

Potential additions:
- 🔒 Add authentication to `/enable-strategy` and `/disable-strategy` endpoints
- 📧 Email notification when strategy auto-disables
- 📊 Trade count limit (e.g., "disable after N trades")
- ⏰ Time-based re-enable (e.g., "re-enable tomorrow at market open")
- 📱 Webhook notification to external service when disabled

## Related Documentation

- [Exit Steps](EXIT_STEPS.md) - How exit conditions work
- [MA Trend Strategy](MOMENTUM_STRATEGY.md) - Current strategy configuration
- [Sequential Order](SEQUENTIAL_ORDER.md) - Entry/exit condition ordering
