# Trading Strategies

This folder contains JSON strategy definitions for the trading bot.

## 🎯 How It Works

Each strategy is a simple JSON file that tells the bot:
1. **When to enter** a trade (which TradingView webhooks to wait for)
2. **When to exit** a trade (which webhooks trigger closing the position)

**No code changes needed!** Just edit JSON and restart.

## 📁 Available Strategies

### `default.json` - Original Bot Behavior
Preserves the exact behavior from v1.x - backward compatible.

**Entry Logic:** Sequential (wait for both in order)
1. RSI crosses below 30 (oversold)
2. MACD crosses up (confirms bullish momentum)

**Exit Logic:** Any of these triggers close:
- RSI crosses center (50) - double-cross exit
- MACD reversal (crosses down)
- RSI crosses above 70 (overbought after warning)

### `ma_ribbon.json` - Trend Following
Enter when strong trend is confirmed by multiple indicators.

**Entry Logic:** All must be true (order doesn't matter)
- All MAs aligned bullish (5>10>20>50>100)
- MACD crosses up (momentum confirmation)

**Exit Logic:** Any triggers close
- MA ribbon reverses (bearish alignment)

### `scalping.json` - Fast Trading
Quick entries and exits for scalping strategies.

**Entry Logic:** All must be true
- MACD crosses up
- RSI neutral (around 50 - not extreme)

**Exit Logic:** Quick exits on:
- MACD reversal
- RSI extreme (overbought/oversold)

## 🎨 Creating Custom Strategies

### Simple 3-Step Process:

**1. Copy an example:**
```bash
cp strategies/default.json strategies/my_strategy.json
```

**2. Edit webhook URLs:**
```json
{
  "name": "my_strategy",
  "description": "My custom strategy",
  "entry": {
    "combination": "all_sequential",
    "steps": [
      {
        "webhook": "/webhook/rsi/crossed-down",
        "comment": "Wait for RSI oversold"
      },
      {
        "webhook": "/webhook/macd/cross-up",
        "comment": "Then confirm with MACD"
      }
    ]
  },
  "exit": {
    "combination": "any",
    "conditions": [
      {
        "webhook": "/webhook/macd/cross-down",
        "is_long": true,
        "comment": "MACD reversal"
      }
    ]
  }
}
```

**3. Use it:**
```bash
STRATEGY=my_strategy docker-compose restart
```

## 📖 Field Reference

### Entry Section

```json
"entry": {
  "combination": "all_sequential",  // How steps combine
  "steps": [                        // Webhook conditions
    {
      "webhook": "/webhook/path",   // TradingView webhook URL
      "comment": "Explanation"      // Optional comment
    }
  ]
}
```

**Combination modes:**
- `all_sequential` - Steps must happen in exact order
- `all` - All must happen (order doesn't matter)
- `any` - Any single step triggers entry

### Exit Section

```json
"exit": {
  "combination": "any",             // Usually "any" for exits
  "conditions": [                   // Exit triggers
    {
      "webhook": "/webhook/path",   // TradingView webhook URL
      "is_long": true,              // true = LONG exits, false = SHORT exits
      "comment": "Explanation"      // Optional comment
    }
  ]
}
```

**Combination modes:**
- `any` - First condition closes position (recommended)
- `all` - All must trigger (rarely used)

## 🔗 Available Webhooks

Copy these exact paths into your strategy JSON:

### RSI
- `/webhook/rsi/crossed-up` - RSI > 70 (overbought)
- `/webhook/rsi/crossed-down` - RSI < 30 (oversold)  
- `/webhook/rsi/crossed-center` - RSI crosses 50

### MACD
- `/webhook/macd/cross-up` - MACD line crosses above signal
- `/webhook/macd/cross-down` - MACD line crosses below signal

### MA Ribbon
- `/webhook/ma/ribbon-bullish` - MAs aligned bullish (5>10>20>50>100)
- `/webhook/ma/ribbon-bearish` - MAs aligned bearish (5<10<20<50<100)

## ✅ Tips

### Keep It Simple
Start with 1-2 entry steps and 2-3 exit conditions. Complex strategies aren't always better!

### Test First
Use the `default` strategy to verify your TradingView alerts work, then create custom strategies.

### Use Comments
The `comment` field helps you remember what each webhook does months later.

### Match Your Alerts
Make sure your TradingView alerts send to the **exact** webhook paths in your strategy.

### One Strategy = Both Directions
You don't need separate rules for LONG and SHORT! The bot automatically mirrors the logic.

Example: `/webhook/rsi/crossed-down` + `/webhook/macd/cross-up`:
- Opens LONG when RSI oversold + MACD bullish
- Opens SHORT when RSI overbought + MACD bearish (automatically!)

## 📚 Learn More

- **Quick Start**: See `STRATEGY_QUICK_START.md` for detailed examples
- **Main README**: See `README.md` for full bot documentation

## 🧪 Validation

The bot validates strategies on startup:
- Checks required fields exist
- Verifies combination modes are valid  
- Ensures webhooks are specified

Invalid strategies will show errors in the logs.

---

**Happy trading!** 🚀
