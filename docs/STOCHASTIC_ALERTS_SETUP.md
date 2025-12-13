# Stochastic + RSI + MACD Strategy - TradingView Alert Cheat Sheet

## Quick Setup Guide

### Step 1: Add Indicators to Chart

1. **Stochastic** (settings: 14, 3, 3)
   - Levels: 20 (oversold), 80 (overbought)

2. **RSI** (settings: 14)
   - Upper band: 50
   - Lower band: 50

3. **MACD** (settings: 12, 26, 9)

### Step 2: Get Your Webhook URL

Run your bot and copy the ngrok URL from logs:
```bash
docker-compose logs trader_bot | grep "Public ngrok URL"
```

Example: `https://abc123.ngrok-free.dev`

### Step 3: Create 8 Alerts

Use this webhook payload for ALL alerts:
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

## Alert Configuration Table

| # | Alert Name | Condition | Webhook URL Path |
|---|------------|-----------|------------------|
| 1 | Stoch Oversold | Stochastic %K < 20 AND %D < 20 | `/webhook/stochastic/oversold` |
| 2 | Stoch Overbought | Stochastic %K > 80 AND %D > 80 | `/webhook/stochastic/overbought` |
| 3 | Stoch Exit Oversold | Stochastic %K crossing 20 upward | `/webhook/stochastic/exit-oversold` |
| 4 | Stoch Exit Overbought | Stochastic %K crossing 80 downward | `/webhook/stochastic/exit-overbought` |
| 5 | RSI Above 50 | RSI crossing above 50 | `/webhook/rsi/above-50` |
| 6 | RSI Below 50 | RSI crossing below 50 | `/webhook/rsi/below-50` |
| 7 | MACD Cross Up | MACD line crossing above Signal | `/webhook/macd/cross-up` |
| 8 | MACD Cross Down | MACD line crossing below Signal | `/webhook/macd/cross-down` |

## Detailed Alert Setup

### Alert 1: Stochastic Oversold
```
Name: [SYMBOL] Stoch Oversold
Condition: Stochastic %K
           Crossing Down 20
Webhook URL: https://YOUR-NGROK-URL/webhook/stochastic/oversold
Settings:
  ✅ Webhook URL
  ⚙️ Once Per Bar Close
  📧 (Optional) Notifications
```

### Alert 2: Stochastic Overbought
```
Name: [SYMBOL] Stoch Overbought
Condition: Stochastic %K
           Crossing Up 80
Webhook URL: https://YOUR-NGROK-URL/webhook/stochastic/overbought
Settings:
  ✅ Webhook URL
  ⚙️ Once Per Bar Close
```

### Alert 3: Stochastic Exit Oversold
```
Name: [SYMBOL] Stoch Exit Oversold
Condition: Stochastic %K
           Crossing Up 20
Webhook URL: https://YOUR-NGROK-URL/webhook/stochastic/exit-oversold
Settings:
  ✅ Webhook URL
  ⚙️ Once Per Bar Close
```

### Alert 4: Stochastic Exit Overbought
```
Name: [SYMBOL] Stoch Exit Overbought
Condition: Stochastic %K
           Crossing Down 80
Webhook URL: https://YOUR-NGROK-URL/webhook/stochastic/exit-overbought
Settings:
  ✅ Webhook URL
  ⚙️ Once Per Bar Close
```

### Alert 5: RSI Above 50
```
Name: [SYMBOL] RSI Above 50
Condition: RSI
           Crossing Up 50
Webhook URL: https://YOUR-NGROK-URL/webhook/rsi/above-50
Settings:
  ✅ Webhook URL
  ⚙️ Once Per Bar Close
```

### Alert 6: RSI Below 50
```
Name: [SYMBOL] RSI Below 50
Condition: RSI
           Crossing Down 50
Webhook URL: https://YOUR-NGROK-URL/webhook/rsi/below-50
Settings:
  ✅ Webhook URL
  ⚙️ Once Per Bar Close
```

### Alert 7: MACD Cross Up
```
Name: [SYMBOL] MACD Cross Up
Condition: MACD
           Crossing Up Signal
Webhook URL: https://YOUR-NGROK-URL/webhook/macd/cross-up
Settings:
  ✅ Webhook URL
  ⚙️ Once Per Bar Close
```

### Alert 8: MACD Cross Down
```
Name: [SYMBOL] MACD Cross Down
Condition: MACD
           Crossing Down Signal
Webhook URL: https://YOUR-NGROK-URL/webhook/macd/cross-down
Settings:
  ✅ Webhook URL
  ⚙️ Once Per Bar Close
```

## Alert Settings

For EACH alert, configure:

**Alert actions:**
- ✅ Webhook URL: `https://your-ngrok-url/webhook/[endpoint]`

**Message:** (paste this for all)
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

**Options:**
- ⚙️ Trigger: Once Per Bar Close (prevents false signals mid-bar)
- 📧 Optional: Email/SMS/App notifications for monitoring

## Testing Your Alerts

1. Start your bot:
   ```bash
   STRATEGY=stochastic_rsi_macd docker-compose up
   ```

2. Watch logs for webhook events:
   ```bash
   docker-compose logs -f trader_bot
   ```

3. Check alert creation in TradingView:
   - Go to Alerts panel
   - Should see 8 alerts for your symbol
   - All should show "Active" status

4. Test with historical data:
   - Use TradingView's replay feature
   - Fast-forward to trigger alerts
   - Verify bot receives webhooks in logs

## Multi-Symbol Setup

To trade multiple symbols (e.g., EUR/USD + BTC/USD):

1. Create all 8 alerts for **each symbol** separately
2. Name them clearly: `EURUSD Stoch Oversold`, `BTCUSD Stoch Oversold`, etc.
3. Use same webhook URLs - bot handles multiple symbols automatically

Example for 2 symbols = 16 total alerts:
- EUR/USD: 8 alerts
- BTC/USD: 8 alerts

## Troubleshooting

**Alert not triggering:**
- Check indicator values match trigger levels
- Verify "Once Per Bar Close" is enabled
- Make sure timeframe matches your strategy

**Webhook not received:**
- Check ngrok URL is correct and active
- Test webhook with curl:
  ```bash
  curl -X POST https://your-url/webhook/stochastic/oversold \
    -H "Content-Type: application/json" \
    -d '{"ticker":"EURUSD","close":"1.0850"}'
  ```
- Check docker-compose logs for errors

**Bot not opening position:**
- Sequential mode requires ALL 4 conditions in order
- Check logs to see which conditions are met
- Verify strategy file is loaded correctly

## Strategy Verification

After creating alerts, verify in bot logs:
```
🚀 [STRATEGY] Active: Stochastic + RSI + MACD Strategy
⏭️  [WEBHOOK] RSI Cross Up Oversell 25 not used in current strategy - ignoring
📊 Stochastic K&D entered OVERSOLD for EUR_USD
```

Look for:
- ✅ Strategy loaded correctly
- ✅ Unused webhooks ignored
- ✅ Sequential conditions tracked

## Performance Monitoring

Watch for these log patterns indicating successful entries:

```
📊 Stochastic K&D entered OVERSOLD for EUR_USD
   (Waiting for RSI confirmation...)
📊 RSI crossed ABOVE 50 (uptrend) for EUR_USD
   (Waiting for MACD confirmation...)
📊 MACD Cross Up for EUR_USD
   (Checking stochastic status...)
✅ [TRADE] Strategy conditions met! Opening LONG position
```

## Quick Reference

**Stochastic Levels:**
- Oversold: < 20
- Overbought: > 80

**RSI Trend:**
- Uptrend: > 50
- Downtrend: < 50

**MACD:**
- Bullish: MACD > Signal
- Bearish: MACD < Signal

**Entry Sequence:**
1. Stochastic extreme
2. RSI trend
3. MACD momentum
4. Stochastic verification
