# TradingView Alert Configuration Guide

## 🎯 All 8 Webhook Events to Create

You need to create **8 separate alerts** in TradingView. Each alert will trigger a specific webhook endpoint.

---

## 📊 RSI Alerts (4 Alerts)

### Alert 1: RSI Greater Than 70 (Overbought)
**Condition:**
- Indicator: RSI (14-period)
- Trigger: Crossing Up
- Value: 70

**Webhook URL:**
```
http://your-server:8080/webhook/rsi/greater-than-70
```

**Message (JSON):**
```json
{
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

**Alert Name:** `RSI > 70 - {{ticker}}`

---

### Alert 2: RSI Less Than 30 (Oversold)
**Condition:**
- Indicator: RSI (14-period)
- Trigger: Crossing Down
- Value: 30

**Webhook URL:**
```
http://your-server:8080/webhook/rsi/less-than-30
```

**Message (JSON):**
```json
{
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

**Alert Name:** `RSI < 30 - {{ticker}}`

---

### Alert 3: RSI Moving Down
**Condition:**
- Indicator: RSI (14-period)
- Trigger: Crossing Down (use a value like 69 to detect downward movement after being overbought)
- Value: 69

**Webhook URL:**
```
http://your-server:8080/webhook/rsi/moving-down
```

**Message (JSON):**
```json
{
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

**Alert Name:** `RSI Moving Down - {{ticker}}`

**Alternative Setup:**
You could also trigger this on any bar close when RSI is decreasing (current RSI < previous RSI and previous RSI was > 70).

---

### Alert 4: RSI Moving Up
**Condition:**
- Indicator: RSI (14-period)
- Trigger: Crossing Up (use a value like 31 to detect upward movement after being oversold)
- Value: 31

**Webhook URL:**
```
http://your-server:8080/webhook/rsi/moving-up
```

**Message (JSON):**
```json
{
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

**Alert Name:** `RSI Moving Up - {{ticker}}`

**Alternative Setup:**
You could also trigger this on any bar close when RSI is increasing (current RSI > previous RSI and previous RSI was < 30).

---

## 📈 MACD Alerts (4 Alerts)

### Alert 5: MACD Cross Above Zero (Bullish)
**Condition:**
- Indicator: MACD (12, 26, 9)
- Trigger: MACD Line Crossing Up
- Value: 0 (zero line)

**Webhook URL:**
```
http://your-server:8080/webhook/macd/cross-up
```

**Message (JSON):**
```json
{
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

**Alert Name:** `MACD Cross Up - {{ticker}}`

---

### Alert 6: MACD Cross Below Zero (Bearish)
**Condition:**
- Indicator: MACD (12, 26, 9)
- Trigger: MACD Line Crossing Down
- Value: 0 (zero line)

**Webhook URL:**
```
http://your-server:8080/webhook/macd/cross-down
```

**Message (JSON):**
```json
{
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

**Alert Name:** `MACD Cross Down - {{ticker}}`

---

### Alert 7: MACD Moving Up
**Condition:**
- Indicator: MACD (12, 26, 9)
- Trigger: MACD Line Moving Up (use histogram or MACD increasing while above zero)

**Webhook URL:**
```
http://your-server:8080/webhook/macd/moving-up
```

**Message (JSON):**
```json
{
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

**Alert Name:** `MACD Moving Up - {{ticker}}`

**Setup Note:**
In TradingView, you might need to:
- Create alert on MACD Histogram > 0 (increasing)
- OR use Pine Script for custom "MACD increasing" condition

---

### Alert 8: MACD Moving Down
**Condition:**
- Indicator: MACD (12, 26, 9)
- Trigger: MACD Line Moving Down (use histogram or MACD decreasing while below zero)

**Webhook URL:**
```
http://your-server:8080/webhook/macd/moving-down
```

**Message (JSON):**
```json
{
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

**Alert Name:** `MACD Moving Down - {{ticker}}`

**Setup Note:**
In TradingView, you might need to:
- Create alert on MACD Histogram < 0 (decreasing)
- OR use Pine Script for custom "MACD decreasing" condition

---

## 🔧 Quick Setup Steps for Each Alert

1. **Open TradingView chart** (e.g., EUR/USD on 15-minute timeframe)
2. **Add indicator** (RSI or MACD)
3. **Click Alert button** (bell icon in top toolbar)
4. **Set condition** as specified above
5. **Set options:**
   - Trigger: "Once Per Bar Close" (recommended)
   - Expiration: "Open-ended"
6. **Enable "Webhook URL"** checkbox
7. **Paste webhook URL** from list above
8. **Paste JSON message** in Message field
9. **Name the alert** using the suggested name
10. **Click "Create"**

---

## 📋 Summary Checklist

- [ ] Alert 1: RSI > 70 → `/webhook/rsi/greater-than-70`
- [ ] Alert 2: RSI < 30 → `/webhook/rsi/less-than-30`
- [ ] Alert 3: RSI Moving Down → `/webhook/rsi/moving-down`
- [ ] Alert 4: RSI Moving Up → `/webhook/rsi/moving-up`
- [ ] Alert 5: MACD Cross Up → `/webhook/macd/cross-up`
- [ ] Alert 6: MACD Cross Down → `/webhook/macd/cross-down`
- [ ] Alert 7: MACD Moving Up → `/webhook/macd/moving-up`
- [ ] Alert 8: MACD Moving Down → `/webhook/macd/moving-down`

---

## 🌐 Getting Your Webhook URL

### Development (ngrok):
```bash
ngrok http 8080
# Use the HTTPS URL: https://abc123.ngrok.io/webhook/...
```

### Production:
```
http://your-server-ip:8080/webhook/...
# or
https://your-domain.com/webhook/...
```

---

## 🎨 Pro Tips

1. **Color code alerts** in TradingView for easy visual identification
2. **Test each alert** individually before enabling all
3. **Use descriptive names** with ticker symbol for multi-pair trading
4. **Set to "Once Per Bar Close"** to avoid false signals mid-candle
5. **Create separate chart layouts** for each trading pair
6. **Duplicate alerts** for multiple symbols (EUR/USD, GBP/USD, etc.)

---

## 🧪 Testing Your Alerts

After creating each alert, test with curl:

```bash
# Test the bot receives it
curl http://localhost:8080/status

# Manually trigger (for testing without TradingView)
curl -X POST http://localhost:8080/webhook/rsi/greater-than-70 \
  -H "Content-Type: application/json" \
  -d '{
    "ticker": "EURUSD",
    "exchange": "OANDA",
    "interval": "15",
    "close": "1.0850",
    "open": "1.0840",
    "high": "1.0855",
    "low": "1.0835",
    "volume": "1000",
    "time": "2025-11-12T14:00:00Z",
    "timenow": "2025-11-12T14:00:00Z"
  }'
```

---

## ❓ Common Issues

**Alert not triggering?**
- Check TradingView alert is active (green checkmark)
- Verify webhook URL is accessible from internet
- Check TradingView alert log for errors

**Wrong ticker format?**
- OANDA uses underscores: `EUR_USD` not `EURUSD`
- The bot converts automatically, but verify in logs

**Too many alerts firing?**
- Change from "Once Per Bar" to "Once Per Bar Close"
- Increase timeframe (use 15-min or 1-hour instead of 1-min)

---

**Ready to trade!** Once all 8 alerts are created, your bot will automatically execute trades based on RSI and MACD signals. 🚀
