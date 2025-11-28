# New RSI Precision Webhook Endpoints

## Summary

Added 7 new RSI-specific level webhooks to support precision trading strategies with exact RSI levels (25, 30, 40, 50, 60, 70, 75).

## New Endpoints

| Endpoint | Event Name | Description |
|----------|-----------|-------------|
| `/webhook/rsi/cross-up-oversell-25` | RSI_CROSS_UP_FROM_OVERSELL | RSI crosses UP from oversold at 25 |
| `/webhook/rsi/cross-oversell-30` | RSI_CROSS_OVERSELL | RSI crosses oversold at 30 |
| `/webhook/rsi/cross-40` | RSI_CROSS_40 | RSI crosses 40 level |
| `/webhook/rsi/cross-center-50` | RSI_MA_CROSS_CENTER | RSI MA crosses center at 50 |
| `/webhook/rsi/cross-60` | RSI_CROSS_60 | RSI crosses 60 level |
| `/webhook/rsi/cross-overbuy-70` | RSI_CROSS_OVERBUY | RSI crosses overbought at 70 |
| `/webhook/rsi/cross-down-overbuy-75` | RSI_CROSS_DOWN_FROM_OVERBUY | RSI crosses DOWN from overbought at 75 |

## TradingView Alert Configuration

For each level, create an alert with:

### Example: RSI Cross Up from Oversell 25
```
Indicator: RSI (14)
Condition: Crossing Up
Value: 25.00
Alert Name: AUDUSD - RSI - CROSS UP FROM OVERSELL (25.00)
Webhook URL: https://your-ngrok-url.ngrok.io/webhook/rsi/cross-up-oversell-25
```

### Webhook Message (JSON):
```json
{
  "event": "RSI_CROSS_UP_FROM_OVERSELL",
  "ticker": "{{ticker}}",
  "exchange": "{{exchange}}",
  "interval": "{{interval}}",
  "close": "{{close}}",
  "open": "{{open}}",
  "high": "{{high}}",
  "low": "{{low}}",
  "volume": "{{volume}}",
  "time": "{{time}}",
  "timenow": "{{timenow}}"
}
```

## New Strategy: RSI Precision

**File**: `/strategies/rsi_precision.json`

**Logic**:
- **LONG Entry**: RSI crosses up from 25 AND MACD crosses up
- **LONG Exit**: RSI crosses down from 75 OR MACD crosses down
- **SHORT Entry**: RSI crosses down from 75 AND MACD crosses down  
- **SHORT Exit**: RSI crosses up from 25 OR MACD crosses up

**Run**:
```bash
STRATEGY=rsi_precision docker-compose up
```

## Complete Endpoint List

### RSI Endpoints (10 total)
- `/webhook/rsi/crossed-up` - RSI > 70 (original)
- `/webhook/rsi/crossed-down` - RSI < 30 (original)
- `/webhook/rsi/crossed-center` - RSI crosses 50 (original)
- `/webhook/rsi/cross-up-oversell-25` - **NEW**
- `/webhook/rsi/cross-oversell-30` - **NEW**
- `/webhook/rsi/cross-40` - **NEW**
- `/webhook/rsi/cross-center-50` - **NEW**
- `/webhook/rsi/cross-60` - **NEW**
- `/webhook/rsi/cross-overbuy-70` - **NEW**
- `/webhook/rsi/cross-down-overbuy-75` - **NEW**

### MACD Endpoints (2 total)
- `/webhook/macd/cross-up` - MACD crosses above signal
- `/webhook/macd/cross-down` - MACD crosses below signal

### MA Ribbon Endpoints (2 total)
- `/webhook/ma/ribbon-bullish` - MA ribbon aligned bullish
- `/webhook/ma/ribbon-bearish` - MA ribbon aligned bearish

## Files Modified

1. **main.go**
   - Added 7 new handler functions (lines 1057-1324)
   - Registered 7 new routes in main()
   - Updated startup logs to show all endpoints

2. **strategies/rsi_precision.json** (NEW)
   - Simple, production-ready strategy using precise RSI levels

3. **strategies/advanced_momentum.json** (NEW)
   - Complex nested condition example (requires future implementation)

4. **docs/RSI_PRECISION_STRATEGY.md** (NEW)
   - Complete documentation for setting up TradingView alerts
   - Strategy explanation and backtesting notes

## Testing

```bash
# Test the new strategy
STRATEGY=rsi_precision docker-compose up

# Test a specific webhook
curl -X POST http://localhost:8080/webhook/rsi/cross-up-oversell-25 \
  -H "Content-Type: application/json" \
  -d '{
    "event": "RSI_CROSS_UP_FROM_OVERSELL",
    "ticker": "EURUSD",
    "exchange": "OANDA",
    "interval": "5",
    "close": "1.0850"
  }'
```

## Future Enhancement: Nested Condition Groups

The `advanced_momentum.json` file shows the desired nested structure:
```json
{
  "type": "group",
  "combination": "sequential",
  "conditions": [
    {"type": "condition", "webhook": "..."},
    {
      "type": "group",
      "combination": "any",
      "conditions": [
        {"type": "condition", "webhook": "..."},
        {
          "type": "group",
          "combination": "sequential",
          "conditions": [...]
        }
      ]
    }
  ]
}
```

This would require:
1. Recursive condition group evaluation
2. Nested state tracking (tree structure)
3. Complex validation logic
4. Extensive testing

For now, use the simpler `rsi_precision.json` strategy which achieves similar goals with flat conditions.
