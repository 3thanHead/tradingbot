# Combination Modes

## Overview

All entry and exit logic uses **conditions** with three combination modes that control how webhooks must fire to trigger an action.

## The Three Modes

### 1. `"any"` - First Match Wins
Any single condition triggers the action immediately.

```json
{
  "combination": "any",
  "conditions": [
    {"webhook": "/webhook/rsi/crossed-up", "comment": "RSI >70"},
    {"webhook": "/webhook/macd/cross-down", "comment": "MACD bearish"}
  ]
}
```

**Behavior:**
- Waits for ANY condition to fire
- First webhook that matches triggers the action
- Other conditions are ignored

**Use for:**
- Multiple independent exit triggers
- "Exit on RSI >70 OR MACD reversal"

---

### 2. `"all"` - All Required (Any Order)
All conditions must fire, but order doesn't matter.

```json
{
  "combination": "all",
  "conditions": [
    {"webhook": "/webhook/rsi/crossed-down", "comment": "RSI oversold"},
    {"webhook": "/webhook/macd/cross-up", "comment": "MACD bullish"}
  ]
}
```

**Behavior:**
- Tracks which conditions have fired
- Triggers action when ALL are completed
- Order doesn't matter (can fire as: A→B or B→A)

**Use for:**
- Multiple confirmations needed
- "Entry when RSI oversold AND MACD bullish" (either order)

---

### 3. `"sequential"` - All Required IN ORDER
All conditions must fire in the exact order specified.

```json
{
  "combination": "sequential",
  "conditions": [
    {"webhook": "/webhook/rsi/crossed-down", "comment": "Step 1: RSI oversold"},
    {"webhook": "/webhook/macd/cross-up", "comment": "Step 2: MACD bullish"},
    {"webhook": "/webhook/rsi/cross-center", "comment": "Step 3: RSI confirms"}
  ]
}
```

**Behavior:**
- Only accepts the NEXT expected condition
- Rejects out-of-order webhooks
- Must complete: condition 1 → then 2 → then 3
- Triggers action only after all complete IN ORDER

**Use for:**
- Sequential confirmation strategies
- "Entry when RSI oversold, THEN MACD crosses, THEN RSI crosses 50"
- Prevents premature entries from noisy signals

---

## Comparison Table

| Mode | Order Matters? | All Required? | Example Use Case |
|------|----------------|---------------|------------------|
| `any` | No | No (just one) | Exit on RSI OR MACD |
| `all` | No | Yes | Entry needs RSI AND MACD (any order) |
| `sequential` | **Yes** | Yes | Entry needs RSI → MACD → RSI center (in order) |

---

## Examples

### Example 1: Simple Exit (any)
```json
{
  "exit": {
    "combination": "any",
    "conditions": [
      {"webhook": "/webhook/rsi/crossed-up", "comment": "Exit on overbought"},
      {"webhook": "/webhook/macd/cross-down", "comment": "Exit on reversal"}
    ]
  }
}
```
**Result:** Position closes when EITHER RSI >70 OR MACD reverses.

---

### Example 2: Confirmed Entry (all)
```json
{
  "entry": {
    "combination": "all",
    "conditions": [
      {"webhook": "/webhook/macd/cross-up", "comment": "MACD bullish"},
      {"webhook": "/webhook/rsi/crossed-down", "comment": "RSI oversold"}
    ]
  }
}
```
**Result:** Position opens when BOTH conditions met (MACD can fire first or RSI can fire first).

---

### Example 3: Momentum Strategy (sequential)
```json
{
  "entry": {
    "combination": "sequential",
    "conditions": [
      {"webhook": "/webhook/rsi/crossed-down", "comment": "1. RSI enters oversold"},
      {"webhook": "/webhook/macd/cross-up", "comment": "2. Momentum shifts up"},
      {"webhook": "/webhook/rsi/cross-center", "comment": "3. RSI confirms upward"}
    ]
  }
}
```
**Result:** Position opens only after RSI oversold, THEN MACD crosses up, THEN RSI crosses 50 (in that exact order).

---

## Log Messages

### `any` mode:
```
🎯 [STRATEGY] Entry condition met: MACD bullish crossover
```

### `all` mode:
```
✅ [STRATEGY] Entry condition met: RSI oversold
✅ [STRATEGY] Entry condition met: MACD bullish crossover
🎯 [STRATEGY] All entry conditions met IN ORDER!
```

### `sequential` mode:
```
✅ [STRATEGY] Entry condition 1/3 completed IN ORDER: RSI oversold
⏳ [STRATEGY] Waiting for condition 2/3: MACD bullish crossover
✅ [STRATEGY] Entry condition 2/3 completed IN ORDER: MACD bullish crossover
⏳ [STRATEGY] Waiting for condition 3/3: RSI confirms upward
✅ [STRATEGY] Entry condition 3/3 completed IN ORDER: RSI confirms upward
🎯 [STRATEGY] All entry conditions completed IN ORDER!
```

**Out-of-order rejection:**
```
⚠️ [STRATEGY] Received /webhook/macd/cross-up but expecting condition 1/3: 
   /webhook/rsi/crossed-down (conditions must be completed IN ORDER)
```

---

## When to Use Which?

### Use `"any"` when:
- You want multiple independent triggers
- Exit strategies with multiple safety nets
- "Get me out if ANY of these happens"

### Use `"all"` when:
- You need multiple confirmations
- Order doesn't matter for your logic
- "Only enter when BOTH these things are true"

### Use `"sequential"` when:
- Market progression matters
- Later signals should confirm earlier ones
- Prevents false signals from out-of-order noise
- "Only enter after seeing this progression: A → B → C"

---

## Best Practices

1. **Keep it simple**: Start with `"any"` or `"all"`, only use `"sequential"` when order truly matters
2. **Test your logic**: Use test scripts to verify condition order
3. **Watch the logs**: Monitor for rejected out-of-order conditions
4. **Document intent**: Use clear comments explaining WHY each condition is needed
5. **Avoid over-complexity**: More than 3-4 conditions becomes hard to manage

---

## Technical Notes

- Conditions are tracked per symbol in `PositionState`
- Entry tracking: `EntryConditionsCompleted` map
- Exit tracking: `ExitConditionsCompleted` map
- Tracking resets when:
  - Position opens (resets entry, initializes exit)
  - Position closes (resets exit, initializes entry)
- Condition keys use format: `"condition_0"`, `"condition_1"`, etc.
