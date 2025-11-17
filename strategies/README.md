# Trading Strategies

JSON strategy files that define when to enter and exit trades. Edit these files to change trading logic without coding.

## Quick Start

1. **Choose a strategy**:
   ```bash
   STRATEGY=momentum docker-compose up       # ⭐ RECOMMENDED - Best simple strategy
   STRATEGY=default docker-compose up        # MACD + RSI extremes
   STRATEGY=ma_ribbon docker-compose up      # Trend following
   STRATEGY=scalping docker-compose up       # Fast trading
   ```

2. **Create custom strategy**:
   ```bash
   cp strategies/default.json strategies/my_strategy.json
   # Edit my_strategy.json
   STRATEGY=my_strategy docker-compose restart
   ```

## Strategy Structure

**Option 1: Unified (same entry/exit for LONG and SHORT)**

```json
{
  "name": "my_strategy",
  "description": "What this strategy does",
  "entry": {
    "combination": "sequential",
    "conditions": [
      {"webhook": "/webhook/rsi/crossed-down", "comment": "Wait for oversold"},
      {"webhook": "/webhook/macd/cross-up", "comment": "Confirm with MACD"}
    ]
  },
  "exit": {
    "combination": "any",
    "conditions": [
      {"webhook": "/webhook/rsi/crossed-up", "comment": "Exit if overbought"}
    ]
  }
}
```

**Option 2: Separate LONG and SHORT configurations**

```json
{
  "name": "my_strategy",
  "description": "Different logic for LONG vs SHORT",
  "long": {
    "entry": {
      "combination": "all",
      "conditions": [
        {"webhook": "/webhook/macd/cross-up", "comment": "MACD bullish"},
        {"webhook": "/webhook/rsi/crossed-down", "comment": "RSI oversold"}
      ]
    },
    "exit": {
      "combination": "any",
      "conditions": [
        {"webhook": "/webhook/rsi/crossed-up", "comment": "RSI >70"}
      ]
    }
  },
  "short": {
    "entry": {
      "combination": "all",
      "conditions": [
        {"webhook": "/webhook/macd/cross-down", "comment": "MACD bearish"},
        {"webhook": "/webhook/rsi/crossed-up", "comment": "RSI overbought"}
      ]
    },
    "exit": {
      "combination": "any",
      "conditions": [
        {"webhook": "/webhook/rsi/crossed-down", "comment": "RSI <30"}
      ]
    }
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

**All modes (Entry & Exit):**
- `sequential` - Conditions fire in exact order (A → then B → then C)
- `all` - All must fire (order doesn't matter)
- `any` - Any single condition triggers action

## Built-in Strategies

### momentum.json ⭐ RECOMMENDED
**Best simple strategy** - Momentum trading with RSI confirmation
- **LONG Entry**: MACD cross up + RSI crosses 50 (upward momentum)
- **LONG Exit**: MACD reversal OR RSI >70
- **SHORT Entry**: MACD cross down + RSI crosses 50 (downward momentum)
- **SHORT Exit**: MACD reversal OR RSI <30

**Why it works:**
- Enters when momentum shifts (MACD) AND RSI confirms direction (crosses 50)
- Exits early on momentum reversal (MACD opposite cross)
- Also exits at extremes for profit-taking (RSI >70 or <30)
- Simple, clean logic that catches trends early

### default.json
MACD momentum + RSI confirmation (unified format)
- **Entry**: All (RSI oversold + MACD cross up)
- **Exit**: RSI opposite extreme

### default-separate.json
Separate LONG/SHORT logic (separate format)
- **LONG Entry**: MACD cross up + RSI oversold
- **LONG Exit**: RSI overbought (>70)
- **SHORT Entry**: MACD cross down + RSI overbought
- **SHORT Exit**: RSI oversold (<30)

### ma_ribbon.json
Trend following - enter on strong trends
- **Entry**: All (MA ribbon bullish + MACD cross up)
- **Exit**: MA ribbon reversal

### scalping.json
Fast trading - quick entries and exits
- **Entry**: All (MACD cross + RSI neutral)
- **Exit**: Quick reversals
