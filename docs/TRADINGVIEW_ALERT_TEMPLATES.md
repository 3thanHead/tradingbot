# TradingView Alert Templates for RSI Precision Strategy

Copy and paste these into TradingView when creating alerts.

## Alert 1: RSI Cross Up from Oversell 25

**Alert Name**: `AUDUSD - RSI - CROSS UP FROM OVERSELL (25.00)`

**Condition**: 
- Indicator: RSI (14, close)
- Crosses: Up
- Value: 25.00

**Webhook URL**: `https://YOUR-NGROK-URL/webhook/rsi/cross-up-oversell-25`

**Message (JSON)**:
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

---

## Alert 2: RSI Cross Oversell 30

**Alert Name**: `AUDUSD - RSI - CROSS OVERSELL (30.00)`

**Condition**: 
- Indicator: RSI (14, close)
- Crosses: (any direction)
- Value: 30.00

**Webhook URL**: `https://YOUR-NGROK-URL/webhook/rsi/cross-oversell-30`

**Message (JSON)**:
```json
{
  "event": "RSI_CROSS_OVERSELL",
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

---

## Alert 3: RSI Cross 40

**Alert Name**: `AUDUSD - RSI - CROSS 40`

**Condition**: 
- Indicator: RSI (14, close)
- Crosses: (any direction)
- Value: 40.00

**Webhook URL**: `https://YOUR-NGROK-URL/webhook/rsi/cross-40`

**Message (JSON)**:
```json
{
  "event": "RSI_CROSS_40",
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

---

## Alert 4: RSI MA Cross Center 50

**Alert Name**: `AUDUSD - RSI MA - CROSS CENTER (50.00)`

**Condition**: 
- Indicator: RSI (14, close)
- Crosses: (any direction)
- Value: 50.00

**Webhook URL**: `https://YOUR-NGROK-URL/webhook/rsi/cross-center-50`

**Message (JSON)**:
```json
{
  "event": "RSI_MA_CROSS_CENTER",
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

---

## Alert 5: RSI Cross 60

**Alert Name**: `AUDUSD - RSI - CROSS 60`

**Condition**: 
- Indicator: RSI (14, close)
- Crosses: (any direction)
- Value: 60.00

**Webhook URL**: `https://YOUR-NGROK-URL/webhook/rsi/cross-60`

**Message (JSON)**:
```json
{
  "event": "RSI_CROSS_60",
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

---

## Alert 6: RSI Cross Overbought 70

**Alert Name**: `AUDUSD - RSI - CROSS OVERBUY (70.00)`

**Condition**: 
- Indicator: RSI (14, close)
- Crosses: (any direction)
- Value: 70.00

**Webhook URL**: `https://YOUR-NGROK-URL/webhook/rsi/cross-overbuy-70`

**Message (JSON)**:
```json
{
  "event": "RSI_CROSS_OVERBUY",
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

---

## Alert 7: RSI Cross Down from Overbought 75

**Alert Name**: `AUDUSD - RSI - CROSS DOWN FROM OVERBUY (75.00)`

**Condition**: 
- Indicator: RSI (14, close)
- Crosses: Down
- Value: 75.00

**Webhook URL**: `https://YOUR-NGROK-URL/webhook/rsi/cross-down-overbuy-75`

**Message (JSON)**:
```json
{
  "event": "RSI_CROSS_DOWN_FROM_OVERBUY",
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

---

## Alert 8: MACD Cross Up (Already Exists)

**Alert Name**: `AUDUSD - MACD - CROSS UP`

**Condition**: 
- Indicator: MACD (12, 26, 9)
- MACD line crosses above Signal line

**Webhook URL**: `https://YOUR-NGROK-URL/webhook/macd/cross-up`

**Message (JSON)**:
```json
{
  "event": "MACD_CROSS_UP",
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

---

## Alert 9: MACD Cross Down (Already Exists)

**Alert Name**: `AUDUSD - MACD - CROSS DOWN`

**Condition**: 
- Indicator: MACD (12, 26, 9)
- MACD line crosses below Signal line

**Webhook URL**: `https://YOUR-NGROK-URL/webhook/macd/cross-down`

**Message (JSON)**:
```json
{
  "event": "MACD_CROSS_DOWN",
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

---

## Quick Setup Instructions

1. **Get your ngrok URL**:
   ```bash
   docker-compose up
   # Look for: ✅ Public ngrok URL: https://xxxx.ngrok.io
   ```

2. **In TradingView**:
   - Open your chart (e.g., AUDUSD 5min)
   - Click the Alert button (clock icon)
   - Set up each alert using templates above
   - Replace `YOUR-NGROK-URL` with your actual ngrok URL
   - Set "Once Per Bar Close" for less noise

3. **Test**:
   ```bash
   # Watch logs
   docker-compose logs -f trader_bot
   
   # You should see webhooks firing as RSI crosses levels
   ```

4. **Adjust Strategy**:
   - Edit `/strategies/rsi_precision.json` to customize logic
   - Restart: `docker-compose restart trader_bot`

## Pro Tips

- **Timeframe**: Start with 5min or 15min charts
- **Frequency**: These specific levels fire less often than generic RSI > 70 / < 30
- **Confirmation**: The strategy requires BOTH RSI level AND MACD confirmation
- **Backtesting**: Paper trade for a week before going live
- **Alerts**: Set "Once Per Bar Close" to avoid repainting
