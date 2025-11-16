# Migration Guide: v1.x → v2.0 (Strategy System)

## 🎯 Overview

Version 2.0 introduces a **JSON-based strategy configuration system** that lets you define custom trading strategies without changing code. The default behavior is **100% backward compatible** - your bot will work exactly as before.

## ✅ Zero-Breaking Changes

**Good news:** Nothing breaks! The v2.0 default strategy exactly replicates v1.x behavior.

- ✅ All existing TradingView alerts work unchanged
- ✅ Position sizing methods unchanged (margin/USD/units)
- ✅ Take profit settings unchanged (pips/dollars/percentage)
- ✅ Entry/exit logic identical when using `default` strategy

## 🔄 What Changed

### New Webhook Endpoints (Optional)

Two new endpoints were added for MA ribbon strategies:
- `POST /webhook/ma/ribbon-bullish`
- `POST /webhook/ma/ribbon-bearish`

**You don't need to use these unless you want the `ma_ribbon` strategy.**

### TradingView Alert Updates (Recommended)

The webhook URLs have been slightly renamed for clarity. Update your TradingView alerts:

| Old URL (v1.x) | New URL (v2.0) | Notes |
|----------------|----------------|-------|
| `/webhook/rsi/greater-than-70` | `/webhook/rsi/crossed-up` | Same behavior |
| `/webhook/rsi/less-than-30` | `/webhook/rsi/crossed-down` | Same behavior |
| `/webhook/rsi/crossed-zero` | `/webhook/rsi/crossed-center` | Better naming |
| `/webhook/macd/cross-up` | `/webhook/macd/cross-up` | ✅ Unchanged |
| `/webhook/macd/cross-down` | `/webhook/macd/cross-down` | ✅ Unchanged |

**Note:** The old URLs still exist in the code, but it's recommended to update to the new naming for consistency.

## 📝 Migration Steps

### Step 1: Update Environment File (Optional)

Add the strategy selection to your `.env`:

```bash
# Add this line (optional - defaults to "default")
STRATEGY=default
```

If you don't add this, the bot automatically uses the `default` strategy.

### Step 2: Update TradingView Alerts (Recommended)

Update your 5 existing alerts to use the new URLs:

**Alert 1: RSI Crossed Above 70**
- Old: `https://your-domain.ngrok-free.app/webhook/rsi/greater-than-70`
- New: `https://your-domain.ngrok-free.app/webhook/rsi/crossed-up`

**Alert 2: RSI Crossed Below 30**
- Old: `https://your-domain.ngrok-free.app/webhook/rsi/less-than-30`
- New: `https://your-domain.ngrok-free.app/webhook/rsi/crossed-down`

**Alert 3: RSI Crossed 50 (Center)**
- Old: `https://your-domain.ngrok-free.app/webhook/rsi/crossed-zero`
- New: `https://your-domain.ngrok-free.app/webhook/rsi/crossed-center`

**Alerts 4-5: MACD (Unchanged)**
- `https://your-domain.ngrok-free.app/webhook/macd/cross-up`
- `https://your-domain.ngrok-free.app/webhook/macd/cross-down`

### Step 3: Restart the Bot

```bash
docker-compose down
docker-compose up --build
```

You should see this in the logs:
```
✅ [STRATEGY] Loaded strategy: default
📊 [STRATEGY] Entry logic: all_sequential (2 steps)
📊 [STRATEGY] Exit logic: any (4 conditions)
```

## 🎨 Optional: Try New Strategies

Once you've verified the default strategy works, you can experiment:

### Try the MA Ribbon Strategy

1. Update `.env`:
```bash
STRATEGY=ma_ribbon
```

2. Add 2 new TradingView alerts:
   - MA ribbon bullish alignment → `/webhook/ma/ribbon-bullish`
   - MA ribbon bearish alignment → `/webhook/ma/ribbon-bearish`

3. Restart bot:
```bash
docker-compose restart
```

### Try the Scalping Strategy

```bash
STRATEGY=scalping
```

No new alerts needed - uses existing RSI and MACD webhooks.

## 🔍 Verification

### Check Current Strategy

Visit: `http://localhost:8080/status`

You should see:
```json
{
  "EUR_USD": {
    "position_open": false,
    "entry_steps_completed": {},
    ...
  }
}
```

### Check Logs

Look for strategy loading on startup:
```
✅ [STRATEGY] Loaded strategy: default
📊 [STRATEGY] Description: Original bot behavior - RSI extremes + MACD confirmation
📊 [STRATEGY] Entry logic: all_sequential (2 steps)
```

### Test a Webhook

```bash
curl -X POST http://localhost:8080/webhook/rsi/crossed-down \
  -H "Content-Type: application/json" \
  -d '{"ticker": "EURUSD", "close": "1.0850"}'
```

Should see:
```
🔔 [WEBHOOK] Received RSI Crossed Down event
📊 RSI crossed below 30 for EUR_USD
✅ RSI cross down condition set
```

## 📚 Learn More

- **Complete Guide**: See `STRATEGY_SYSTEM.md` for full documentation
- **Quick Start**: See `STRATEGY_QUICK_START.md` to create your first custom strategy
- **Examples**: Check `strategies/` folder for default, ma_ribbon, and scalping strategies

## ⚠️ Rollback (If Needed)

If anything goes wrong, you can rollback:

```bash
git checkout v1.1.0  # Or your previous version tag
docker-compose up --build
```

But you shouldn't need to - the default strategy is identical to v1.x behavior!

## 🆘 Troubleshooting

### "Strategy file not found"
**Error:** `❌ [ERROR] Strategy file not found: strategies/custom.json`

**Fix:** Either:
1. Set `STRATEGY=default` in `.env`
2. Create the missing strategy file
3. Remove the `STRATEGY` variable (defaults to `default`)

### "Invalid strategy JSON"
**Error:** `❌ [ERROR] Invalid strategy JSON`

**Fix:** Validate your JSON at https://jsonlint.com or use the default strategy

### Logs show wrong strategy
**Check:** Make sure `.env` is loaded properly:
```bash
docker-compose config | grep STRATEGY
```

---

## ✅ Summary

**Migration is optional.** Your bot works exactly as before with zero changes.

**To benefit from v2.0:**
1. Optionally update TradingView alert URLs (recommended for clarity)
2. Keep using the default strategy, or try `ma_ribbon` or `scalping`
3. Create custom strategies as needed

**Questions?** See `STRATEGY_SYSTEM.md` or open an issue!
