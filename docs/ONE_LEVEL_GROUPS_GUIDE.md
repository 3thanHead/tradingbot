# One-Level Grouped Conditions - User Guide

## Overview

You can now use **one-level groups** to create complex "AND", "OR", and "THEN" logic in your strategies.

## How It Works

### Top Level Combination Modes:
- **`"sequential"`** = Conditions must fire in order (THEN logic)
- **`"all"`** = All conditions must fire (AND logic)
- **`"any"`** = Any condition fires (OR logic)

### Condition Types:
- **`"condition"`** = Simple webhook
- **`"group"`** = A group of webhooks with its own combination mode

## Simple Example (No Groups)

```json
{
  "entry": {
    "combination": "all",
    "conditions": [
      {
        "webhook": "/webhook/rsi/cross-up-oversell-25",
        "comment": "RSI 25"
      },
      {
        "webhook": "/webhook/macd/cross-up",
        "comment": "MACD up"
      }
    ]
  }
}
```

**Result**: Entry when RSI 25 **AND** MACD up (any order)

## One-Level Group Example

```json
{
  "exit": {
    "combination": "sequential",
    "conditions": [
      {
        "type": "condition",
        "webhook": "/webhook/rsi/cross-50",
        "comment": "Step 1: RSI crosses 50"
      },
      {
        "type": "condition",
        "webhook": "/webhook/rsi/cross-60",
        "comment": "Step 2: RSI crosses 60"
      },
      {
        "type": "group",
        "combination": "any",
        "comment": "Step 3: Exit on ANY of these",
        "conditions": [
          {
            "webhook": "/webhook/rsi/cross-70",
            "comment": "RSI 70"
          },
          {
            "webhook": "/webhook/macd/cross-down",
            "comment": "MACD reversal"
          }
        ]
      }
    ]
  }
}
```

**Result**: 
1. First, wait for RSI to cross 50
2. **THEN** wait for RSI to cross 60  
3. **THEN** exit when **EITHER** RSI hits 70 **OR** MACD reverses

## Your Original Request Simplified

### LONG Exit Logic:

**What you wanted:**
```
RSI crosses 50
THEN RSI crosses 60
THEN (
  RSI crosses 60 again
  || RSI crosses 70
  || RSI crosses down from 75
  || MACD crosses down
)
```

**How to write it:**
```json
{
  "exit": {
    "combination": "sequential",
    "conditions": [
      {
        "type": "condition",
        "webhook": "/webhook/rsi/cross-center-50"
      },
      {
        "type": "condition",
        "webhook": "/webhook/rsi/cross-60"
      },
      {
        "type": "group",
        "combination": "any",
        "comment": "Exit on any of these",
        "conditions": [
          {"webhook": "/webhook/rsi/cross-60"},
          {"webhook": "/webhook/rsi/cross-overbuy-70"},
          {"webhook": "/webhook/rsi/cross-down-overbuy-75"},
          {"webhook": "/webhook/macd/cross-down"}
        ]
      }
    ]
  }
}
```

## More Examples

### Example 1: Multiple Entry Confirmations (AND)

```json
{
  "entry": {
    "combination": "all",
    "conditions": [
      {"webhook": "/webhook/rsi/cross-30"},
      {"webhook": "/webhook/macd/cross-up"},
      {"webhook": "/webhook/ma/ribbon-bullish"}
    ]
  }
}
```

**Result**: Needs RSI **AND** MACD **AND** MA ribbon (any order)

### Example 2: Quick Exit Options (OR)

```json
{
  "exit": {
    "combination": "any",
    "conditions": [
      {"webhook": "/webhook/rsi/cross-70"},
      {"webhook": "/webhook/macd/cross-down"}
    ]
  }
}
```

**Result**: Exit on **EITHER** RSI 70 **OR** MACD reversal (first one wins)

### Example 3: Sequential with Final OR Group

```json
{
  "exit": {
    "combination": "sequential",
    "conditions": [
      {
        "type": "condition",
        "webhook": "/webhook/rsi/cross-50",
        "comment": "First: RSI crosses center"
      },
      {
        "type": "group",
        "combination": "any",
        "comment": "Then: Exit on any extreme",
        "conditions": [
          {"webhook": "/webhook/rsi/cross-70"},
          {"webhook": "/webhook/rsi/cross-30"}
        ]
      }
    ]
  }
}
```

**Result**: Wait for RSI 50, **THEN** exit on **EITHER** RSI 70 **OR** RSI 30

## Rules & Limitations

### ✅ Supported:
- Top-level: "sequential", "all", or "any"
- Groups: "sequential", "all", or "any"
- Groups can contain multiple simple conditions
- Backward compatible (old strategies still work)

### ❌ Not Supported:
- **NO nested groups inside groups** (one level only)
- This won't work: `group → group → condition`
- Groups can ONLY contain simple conditions

## Backward Compatibility

Old strategies without `type` field still work:

```json
{
  "entry": {
    "combination": "all",
    "conditions": [
      {"webhook": "/webhook/rsi/cross-30"},
      {"webhook": "/webhook/macd/cross-up"}
    ]
  }
}
```

This automatically treats all conditions as `type="condition"`.

## Testing Your Strategy

```bash
# Run with your strategy
STRATEGY=advanced_momentum docker-compose up

# Watch the logs
docker-compose logs -f trader_bot
```

Look for:
- `✅ [STRATEGY] Entry condition met`
- `⏳ [GROUP] Partial match in group - waiting for more`
- `🎯 [STRATEGY] All entry conditions completed IN ORDER!`

## Complete Example (advanced_momentum.json)

See `/strategies/advanced_momentum.json` for a full working example that implements your original request:

- **Entry**: RSI 25 **AND** MACD up
- **Exit**: RSI 50 **THEN** RSI 60 **THEN** (RSI 60 again **OR** RSI 70 **OR** RSI 75 down **OR** MACD down)

Run it:
```bash
STRATEGY=advanced_momentum docker-compose up
```

## Key Takeaways

1. **Use `type="condition"` for simple webhooks**
2. **Use `type="group"` for OR/AND within a sequence**
3. **Groups can only be ONE level deep**
4. **Groups combine webhooks, not other groups**

This gives you the flexibility you need while keeping the code simple and maintainable!
