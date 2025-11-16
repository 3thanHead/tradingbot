# Strategy Quick Start - 5 Minutes

## 🎯 Overview

Creating a custom trading strategy is **super simple** - just match your TradingView webhook URLs in a JSON file!

**No need to remember abstract condition names.** Just copy/paste your webhook URLs.

## 📋 Quick Example

### 1. Create Your Strategy File

```bash
nano strategies/my_strategy.json
```

### 2. Define Entry Conditions

Use the **exact webhook URLs** from your TradingView alerts:

```json
{
  "name": "my_strategy",
  "description": "My custom trading strategy",
  "entry": {
    "combination": "all_sequential",
    "steps": [
      {
        "webhook": "/webhook/rsi/crossed-down",
        "comment": "Wait for RSI oversold"
      },
      {
        "webhook": "/webhook/macd/cross-up",
        "comment": "Then MACD confirms bullish"
      }
    ]
  }
}
```

**That's it for entry!** When TradingView sends those webhooks in order, the bot opens a position.

### 3. Define Exit Conditions

```json
  "exit": {
    "combination": "any",
    "conditions": [
      {
        "webhook": "/webhook/macd/cross-down",
        "is_long": true,
        "comment": "MACD reversal"
      },
      {
        "webhook": "/webhook/rsi/crossed-up",
        "is_long": true,
        "comment": "RSI overbought"
      }
    ]
  }
```

**Any** of these webhooks will close the position!

### 4. Complete Strategy File

<details>
<summary>Click to expand full my_strategy.json</summary>

```json
{
  "name": "my_strategy",
  "description": "Custom RSI + MACD strategy with quick exits",
  "entry": {
    "combination": "all_sequential",
    "steps": [
      {
        "webhook": "/webhook/rsi/crossed-down",
        "comment": "RSI crosses below 30"
      },
      {
        "webhook": "/webhook/macd/cross-up",
        "comment": "MACD confirms bullish"
      }
    ]
  },
  "exit": {
    "combination": "any",
    "conditions": [
      {
        "webhook": "/webhook/macd/cross-down",
        "is_long": true,
        "comment": "MACD bearish reversal"
      },
      {
        "webhook": "/webhook/rsi/crossed-up",
        "is_long": true,
        "comment": "RSI overbought (> 70)"
      },
      {
        "webhook": "/webhook/rsi/crossed-center",
        "is_long": true,
        "comment": "RSI crosses 50 midpoint"
      }
    ]
  }
}
```
</details>

### 5. Use Your Strategy

```bash
# Edit .env
STRATEGY=my_strategy

# Restart bot
docker-compose restart
```

Done! 🎉

## 📚 Field Reference

### Entry Fields

| Field | Values | Description |
|-------|--------|-------------|
| `combination` | `all_sequential`, `all`, `any` | How steps combine |
| `steps` | Array | List of webhooks to wait for |
| `webhook` | Webhook path | Must match TradingView alert URL |
| `comment` | Text | Optional - helps you remember what it does |

**Combination Modes:**
- `all_sequential` - Steps must happen in exact order (Step 1 → Step 2 → Step 3)
- `all` - All steps must happen (order doesn't matter)
- `any` - Any single step triggers entry

### Exit Fields

| Field | Values | Description |
|-------|--------|-------------|
| `combination` | `any`, `all` | How conditions combine |
| `conditions` | Array | List of exit triggers |
| `webhook` | Webhook path | Must match TradingView alert URL |
| `is_long` | `true`/`false` | Apply to LONG or SHORT positions |
| `comment` | Text | Optional - explains the exit |

**Combination Modes:**
- `any` - First condition that triggers closes position
- `all` - All conditions must trigger (rare - usually use `any`)

## 🎨 Available Webhooks

These are the webhook URLs you can use in your strategies:

### RSI Webhooks
- `/webhook/rsi/crossed-up` - RSI crosses above 70 (overbought)
- `/webhook/rsi/crossed-down` - RSI crosses below 30 (oversold)
- `/webhook/rsi/crossed-center` - RSI crosses 50 (midpoint)

### MACD Webhooks  
- `/webhook/macd/cross-up` - MACD line crosses above signal
- `/webhook/macd/cross-down` - MACD line crosses below signal

### MA Ribbon Webhooks
- `/webhook/ma/ribbon-bullish` - All MAs aligned bullish (5>10>20>50>100)
- `/webhook/ma/ribbon-bearish` - All MAs aligned bearish (5<10<20<50<100)

## 💡 Common Patterns

### Pattern 1: Sequential Confirmation (Safest)

Wait for extreme, then wait for momentum confirmation:

```json
"entry": {
  "combination": "all_sequential",
  "steps": [
    { "webhook": "/webhook/rsi/crossed-down", "comment": "Oversold" },
    { "webhook": "/webhook/macd/cross-up", "comment": "Momentum confirms" }
  ]
}
```

### Pattern 2: Simultaneous (Trend Following)

All conditions must be true at the same time:

```json
"entry": {
  "combination": "all",
  "steps": [
    { "webhook": "/webhook/ma/ribbon-bullish", "comment": "Trend is bullish" },
    { "webhook": "/webhook/macd/cross-up", "comment": "Momentum is bullish" }
  ]
}
```

### Pattern 3: Quick Entry (Scalping)

Any condition triggers entry:

```json
"entry": {
  "combination": "any",
  "steps": [
    { "webhook": "/webhook/macd/cross-up", "comment": "MACD bullish" },
    { "webhook": "/webhook/rsi/crossed-center", "comment": "RSI neutral" }
  ]
}
```

### Pattern 4: Multiple Exits (Safe)

Exit on reversal OR extreme:

```json
"exit": {
  "combination": "any",
  "conditions": [
    { "webhook": "/webhook/macd/cross-down", "is_long": true },
    { "webhook": "/webhook/rsi/crossed-up", "is_long": true },
    { "webhook": "/webhook/rsi/crossed-center", "is_long": true }
  ]
}
```

## ⚡ Tips

### For LONG Positions
Entry webhooks trigger when you want to **buy** (go long).
Exit `is_long: true` webhooks close LONG positions.

### For SHORT Positions  
Entry webhooks trigger when you want to **sell** (go short).
Exit `is_long: false` webhooks close SHORT positions.

### One Strategy = Both Directions
The **same** entry/exit rules apply to both LONG and SHORT!

Example:
- `rsi/crossed-down` + `macd/cross-up` = Open LONG
- `rsi/crossed-up` + `macd/cross-down` = Open SHORT (automatically!)

The bot mirrors the logic for both directions.

## 🧪 Testing Your Strategy

### 1. Validate JSON

```bash
# Test if JSON is valid
cat strategies/my_strategy.json | python3 -m json.tool
```

### 2. Load Strategy

```bash
# Set strategy and restart
STRATEGY=my_strategy docker-compose restart
```

### 3. Check Logs

Look for:
```
✅ [STRATEGY] Loaded: my_strategy  
📊 [STRATEGY] Entry: 2 steps (all_sequential combination)
📊 [STRATEGY] Exit: 3 conditions (any combination)
```

### 4. Send Test Webhook

```bash
curl -X POST http://localhost:8080/webhook/rsi/crossed-down \
  -H "Content-Type: application/json" \
  -d '{"ticker": "EURUSD", "close": "1.0850"}'
```

Check logs for:
```
✅ [STRATEGY] Entry step 1/2 completed: RSI crosses below 30
```

## 📖 Full Examples

See the `strategies/` folder:
- `default.json` - Original bot behavior
- `ma_ribbon.json` - Trend-following with MA ribbon
- `scalping.json` - Fast scalping strategy

---

**That's it!** You now know everything you need to create custom strategies. 🚀

Just match your TradingView webhook URLs - no complex condition names to remember!
