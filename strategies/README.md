# Trading Strategies

JSON strategy files that define when to enter and exit trades. Edit these files to change trading logic without coding.

## Quick Start

1. **Choose a strategy**:
   ```bash
   STRATEGY=default docker-compose up    # Original bot behavior
   STRATEGY=ma_ribbon docker-compose up  # Trend following
   STRATEGY=scalping docker-compose up   # Fast trading
   ```

2. **Create custom strategy**:
   ```bash
   cp strategies/default.json strategies/my_strategy.json
   # Edit my_strategy.json
   STRATEGY=my_strategy docker-compose restart
   ```

## Strategy Structure

```json
{
  "name": "my_strategy",
  "description": "What this strategy does",
  "entry": {
    "combination": "all_sequential",  // or "all" or "any"
    "steps": [
      {"webhook": "/webhook/rsi/crossed-down", "comment": "Wait for oversold"},
      {"webhook": "/webhook/macd/cross-up", "comment": "Confirm with MACD"}
    ]
  },
  "exit": {
    "combination": "any",  // Usually "any" for quick exits
    "conditions": [
      {"webhook": "/webhook/rsi/crossed-up", "comment": "Exit if overbought"},
      {"webhook": "/webhook/macd/cross-down", "comment": "Exit if reversal"}
    ]
  }
}
```

## Available Webhooks

**RSI:**
- `/webhook/rsi/crossed-up` - RSI > 70 (overbought)
- `/webhook/rsi/crossed-down` - RSI < 30 (oversold)
- `/webhook/rsi/crossed-center` - RSI crosses 50

**MACD:**
- `/webhook/macd/cross-up` - MACD crosses above signal
- `/webhook/macd/cross-down` - MACD crosses below signal

**MA Ribbon:**
- `/webhook/ma/ribbon-bullish` - MAs aligned bullish (5>10>20>50>100)
- `/webhook/ma/ribbon-bearish` - MAs aligned bearish

## Combination Modes

**Entry:**
- `all_sequential` - Steps happen in exact order (wait for A, then B, then C)
- `all` - All must happen (order doesn't matter)
- `any` - Any single step triggers entry

**Exit:**
- `any` - First condition closes position (recommended)
- `all` - All must trigger (rarely used)

## Built-in Strategies

### default.json
Original bot behavior - RSI extremes + MACD confirmation
- **Entry**: Sequential (RSI < 30 → MACD cross up)
- **Exit**: Any of 5 conditions

### ma_ribbon.json
Trend following - enter on strong trends
- **Entry**: All (MA ribbon bullish + MACD cross up)
- **Exit**: MA ribbon reversal

### scalping.json
Fast trading - quick entries and exits
- **Entry**: All (MACD cross + RSI neutral)
- **Exit**: Quick reversals
