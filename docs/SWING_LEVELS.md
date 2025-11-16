# Swing High/Low Support & Resistance Trading

The bot uses swing high and swing low levels to automatically close positions at key support/resistance levels, providing logical profit-taking exits.

### Logs

When a swing level closes a position:

```
2025/11/14 16:29:34 📈 [SWING HIGH] New resistance level for EUR_USD at 1.09750
2025/11/14 16:29:34 ✅ [TRADE] Swing high reached! Closing LONG position at resistance 1.09750
2025/11/14 16:29:34 🔵 Closing long position for EUR_USD (ID: 848)
2025/11/14 16:29:34 ✅ Position closed: EUR_USD
```

```
2025/11/14 16:29:58 📉 [SWING LOW] New support level for GBP_USD at 1.26250
2025/11/14 16:29:58 ✅ [TRADE] Swing low reached! Closing SHORT position at support 1.26250
2025/11/14 16:29:58 🔵 Closing short position for GBP_USD (ID: 852)
2025/11/14 16:29:58 ✅ Position closed: GBP_USD
```

When no position is open:

```
2025/11/14 16:21:06 📈 [SWING HIGH] New resistance level for EUR_USD at 1.09750
2025/11/14 16:21:06 ✅ [STATE] Swing High updated to 1.09750
```### Swing High (Resistance Level)
- **Closes LONG positions** when a new swing high is detected
- Logic: Price reaching a new high indicates potential resistance - take profit on longs
- Replaces MACD moving down for closing longs

### Swing Low (Support Level)
- **Closes SHORT positions** when a new swing low is detected  
- Logic: Price reaching a new low indicates potential support - take profit on shorts
- Replaces MACD moving up for closing shorts

## Webhook Endpoints

### New Swing High (Close LONG)
```
POST /webhook/swing/new-high
```

Automatically closes any open LONG position when price reaches a new swing high (resistance).

**Payload:**
```json
{
  "ticker": "EUR_USD",
  "exchange": "OANDA",
  "interval": "5",
  "close": "1.09500",
  "high": "1.09750",
  "timenow": "2024-01-15T10:30:00Z"
}
```

**Response:**
```json
{
  "status": "success",
  "message": "Swing high at 1.09750 → LONG closed"
}
```

Or if no position is open:
```json
{
  "status": "success",
  "message": "New swing high recorded at 1.09750"
}
```

### New Swing Low (Close SHORT)
```
POST /webhook/swing/new-low
```

Automatically closes any open SHORT position when price reaches a new swing low (support).

**Payload:**
```json
{
  "ticker": "EUR_USD",
  "exchange": "OANDA",
  "interval": "5",
  "close": "1.08500",
  "low": "1.08250",
  "timenow": "2024-01-15T10:30:00Z"
}
```

**Response:**
```json
{
  "status": "success",
  "message": "Swing low at 1.08250 → SHORT closed"
}
```

Or if no position is open:
```json
{
  "status": "success",
  "message": "New swing low recorded at 1.08250"
}
```

## TradingView Alert Setup

For the **Swing 5 5 high low Left Side** indicator:

### Swing High Alert
1. Create Alert on your chart
2. Condition: `Swing 5 5 high low Left Side` → `New Swing High`
3. Webhook URL: `https://your-ngrok-domain.ngrok-free.dev/webhook/swing/new-high`
4. Message:
```json
{
  "ticker": "{{ticker}}",
  "exchange": "{{exchange}}",
  "interval": "{{interval}}",
  "close": "{{close}}",
  "high": "{{high}}",
  "timenow": "{{timenow}}"
}
```

### Swing Low Alert
1. Create Alert on your chart
2. Condition: `Swing 5 5 high low Left Side` → `New Swing Low`
3. Webhook URL: `https://your-ngrok-domain.ngrok-free.dev/webhook/swing/new-low`
4. Message:
```json
{
  "ticker": "{{ticker}}",
  "exchange": "{{exchange}}",
  "interval": "{{interval}}",
  "close": "{{close}}",
  "low": "{{low}}",
  "timenow": "{{timenow}}"
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
- **Swing High** → Closes LONG at resistance (profit taking)
- **Swing Low** → Closes SHORT at support (profit taking)
- **MACD Moving Up** → Alternative close for SHORT (still available)
- **MACD Moving Down** → Alternative close for LONG (still available)

## State Tracking

The bot maintains swing high/low levels for each symbol in the position state:

```json
{
  "Symbol": "EUR_USD",
  "PositionOpen": false,
  "Position": "none",
  "TradeID": "",
  "RSICrossedUp": false,
  "RSICrossedDown": false,
  "MACDCrossedUp": false,
  "MACDCrossedDown": false,
  "SwingHigh": 1.0975,
  "SwingLow": 1.0825
}
```

Check current state:
```bash
curl http://localhost:8080/status
```

## Use Cases

1. **Profit Taking at Key Levels**: Automatically exit positions when price reaches resistance (LONG) or support (SHORT)
2. **Logical Exit Strategy**: Use swing points identified by the indicator rather than arbitrary MACD signals
3. **Risk Management**: Swing levels represent areas where price historically reverses
4. **Multiple Exit Options**: Can still use MACD moving events as alternative exits if needed

## Example Trade Flow

**LONG Trade:**
```
1. Price drops, RSI < 30 → Set oversold flag
2. MACD crosses up → Open LONG at 1.09000
3. Price rallies to new swing high 1.09750 → Close LONG (profit: 75 pips)
```

**SHORT Trade:**
```
1. Price rallies, RSI > 70 → Set overbought flag
2. MACD crosses down → Open SHORT at 1.27000
3. Price drops to new swing low 1.26250 → Close SHORT (profit: 75 pips)
```

## Testing

Test the endpoints locally:

```bash
# Test swing high
./scripts/test-swing-high.sh

# Test swing low
./scripts/test-swing-low.sh
```

## Logs

When a swing level is updated:

```
2025/11/14 16:21:06 🔔 [WEBHOOK] Received New Swing High event
2025/11/14 16:21:06 📈 [SWING HIGH] New resistance level for EUR_USD at 1.09750
2025/11/14 16:21:06 🔍 [STATE] Previous Swing High: 0.00000
2025/11/14 16:21:06 ✅ [STATE] Swing High updated to 1.09750
```

```
2025/11/14 16:21:12 🔔 [WEBHOOK] Received New Swing Low event
2025/11/14 16:21:12 📉 [SWING LOW] New support level for EUR_USD at 1.08250
2025/11/14 16:21:12 🔍 [STATE] Previous Swing Low: 0.00000
2025/11/14 16:21:12 ✅ [STATE] Swing Low updated to 1.08250
```

## Benefits Over MACD Moving Events

1. **More Logical Exits**: Swing points represent actual market structure changes, not just indicator movement
2. **Better Profit Taking**: Exit at historically significant levels where price is likely to reverse
3. **Reduced Noise**: Swing highs/lows are less frequent than MACD movements, avoiding premature exits
4. **Market Structure Based**: Uses actual price action rather than derivative indicators
5. **Flexible Strategy**: MACD moving events still available as backup/alternative exits
