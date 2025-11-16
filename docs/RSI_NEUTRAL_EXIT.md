# RSI Crossed Zero Exit Strategy

The bot uses RSI crossing back to 0.00 (neutral) as the signal to close any open position, regardless of direction.

## Trading Logic

### RSI Crossed Zero (Neutral Exit)
- **Closes ANY open position** (LONG or SHORT) when RSI crosses 0.00
- Logic: RSI returning to neutral indicates momentum exhaustion - time to exit
- Clean, universal exit signal that works for both directions

## Webhook Endpoint

### RSI Crossed Zero (Close Position)
```
POST /webhook/rsi/crossed-zero
```

Automatically closes any open position when RSI crosses back to 0.00 (neutral).

**Payload:**
```json
{
  "ticker": "EUR_USD",
  "exchange": "OANDA",
  "interval": "15",
  "close": "1.09200",
  "timenow": "2024-01-15T10:35:00Z"
}
```

**Response (position open):**
```json
{
  "status": "success",
  "message": "RSI crossed 0 → LONG closed"
}
```

**Response (no position):**
```json
{
  "status": "success",
  "message": "RSI crossed 0 (no position to close)"
}
```

## Complete Trading Flow

### Opening Positions
1. **RSI Crossed Down** (< 30) → Sets oversold flag
2. **MACD Cross Up** → Opens LONG if RSI oversold

OR

1. **RSI Crossed Up** (> 70) → Sets overbought flag
2. **MACD Cross Down** → Opens SHORT if RSI overbought

### Closing Positions
- **RSI Crossed Zero** (0.00) → Closes ANY position (universal exit)

## TradingView Alert Setup

For the **RSI** indicator:

### RSI Crossed Zero Alert
1. Create Alert on your chart
2. Condition: `RSI` → `Crossing` → Value: `0`
3. Webhook URL: `https://your-ngrok-domain.ngrok-free.dev/webhook/rsi/crossed-zero`
4. Message:
```json
{
  "ticker": "{{ticker}}",
  "exchange": "{{exchange}}",
  "interval": "{{interval}}",
  "close": "{{close}}",
  "timenow": "{{timenow}}"
}
```

## Example Trade Flows

**LONG Trade:**
```
1. RSI drops below 30 → Set oversold flag
2. MACD crosses up → Open LONG at 1.09000
3. Price rallies, RSI returns to 0 → Close LONG at 1.09200 (profit: 20 pips)
```

**SHORT Trade:**
```
1. RSI rises above 70 → Set overbought flag
2. MACD crosses down → Open SHORT at 1.27000
3. Price drops, RSI returns to 0 → Close SHORT at 1.26500 (profit: 50 pips)
```

## Benefits

1. **Universal Exit**: One signal works for both LONG and SHORT
2. **Momentum-Based**: Exit when momentum returns to neutral
3. **Simple Logic**: RSI = 0 means "get out"
4. **Reduces Complexity**: No need for separate long/short exit signals
5. **Aligns with RSI**: Entry and exit both based on same indicator

## Testing

Test the endpoint locally:

```bash
# Test RSI crossed zero
./scripts/test-rsi-zero.sh

# Check status
curl http://localhost:8080/status
```

## Logs

When RSI crosses zero and closes a position:

```
2025/11/14 17:23:03 ⚖️  [RSI NEUTRAL] RSI crossed 0.00 for EUR_USD
2025/11/14 17:23:03 ✅ [TRADE] RSI neutral! Closing LONG position
2025/11/14 17:23:03 🔵 Closing long position for EUR_USD (ID: 856)
2025/11/14 17:23:03 ✅ Position closed: EUR_USD
```

When no position is open:

```
2025/11/14 17:25:00 ⚖️  [RSI NEUTRAL] RSI crossed 0.00 for EUR_USD
2025/11/14 17:25:00 ℹ️  [INFO] No position open for EUR_USD
```

## Why RSI Zero Instead of Swing Levels?

- **More Reliable**: Swing detection can be subjective and vary
- **Cleaner Signal**: RSI 0 is a clear mathematical threshold
- **Less Noise**: Fewer false signals than swing points
- **Indicator Consistency**: Entry and exit both use RSI
- **Easier to Backtest**: Clear, repeatable condition
