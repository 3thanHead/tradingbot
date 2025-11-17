# Exit Steps Support

## Overview

The trading bot now supports using `steps` for exit conditions, not just `conditions`. This provides more flexibility in defining multi-step exit criteria that mirror the entry logic.

## Key Differences

### Conditions (Original)
- **Independent triggers**: Any condition can fire at any time
- **Use case**: Simple exits where any single event triggers closure
- **Combination modes**:
  - `"any"`: Exit on first matching webhook
  - `"all"`: Wait for all webhooks (rare for conditions)

### Steps (New)
- **Sequential tracking**: Tracks progress through exit criteria **IN ORDER**
- **Use case**: Multi-confirmation exits that mirror entry logic
- **Combination modes**:
  - `"any"`: Exit when any single step fires (like conditions)
  - `"all"`: Exit only when all steps are completed **IN SEQUENTIAL ORDER** (step 1 → step 2 → step 3)

## Example: Momentum Strategy

```json
{
  "long": {
    "entry": {
      "combination": "all",
      "steps": [
        {"webhook": "/webhook/rsi/crossed-down", "comment": "..."},
        {"webhook": "/webhook/macd/cross-up", "comment": "..."},
        {"webhook": "/webhook/rsi/cross-center", "comment": "..."}
      ]
    },
    "exit": {
      "combination": "all",
      "steps": [
        {"webhook": "/webhook/macd/cross-down", "comment": "MACD reversal"},
        {"webhook": "/webhook/rsi/crossed-center", "comment": "RSI confirms"}
      ]
    }
  }
}
```

## Implementation Details

### Code Changes

1. **ExitConditions struct** - Added `Steps` field alongside `Conditions`
2. **PositionState** - Added `ExitStepsCompleted` map to track progress
3. **Validation** - Updated to accept either `conditions` OR `steps`, not both
4. **shouldExitPosition()** - Enhanced to handle both conditions and steps
5. **Position opening** - Resets exit steps when opening new positions
6. **Logging** - Shows whether using "steps" or "conditions" and "IN ORDER" for sequential execution

### How It Works

**Steps with "all" combination (SEQUENTIAL):**
1. Only the NEXT expected step is accepted
2. Steps must be completed IN ORDER (step 1 → step 2 → step 3)
3. If a webhook fires out of order, it's rejected with a warning
4. Exit only triggers when ALL steps are completed in sequence
5. Steps reset when position opens or closes

**Steps with "any" combination:**
1. Any matching webhook triggers immediate exit
2. Behaves similarly to conditions but uses step structure

## When to Use Which

### Use Conditions When:
- Simple exit logic (single trigger or independent events)
- Example: "Exit on RSI > 70 OR MACD bearish cross"

### Use Steps When:
- Multi-confirmation exits that mirror entry logic
- **Sequential validation needed (steps must fire IN ORDER)**
- Example: "Exit when MACD reverses (step 1) AND THEN RSI confirms weakness (step 2)"

## Sequential Order Enforcement

When using `"combination": "all"` with steps, the bot **enforces sequential execution**:

```
✅ CORRECT ORDER:
1. /webhook/macd/cross-down fires → Step 1 completed
2. /webhook/rsi/crossed-center fires → Step 2 completed → EXIT TRIGGERED

❌ WRONG ORDER (rejected):
1. /webhook/rsi/crossed-center fires → ⚠️ Rejected (expecting step 1 first)
2. /webhook/macd/cross-down fires → Step 1 completed
3. /webhook/rsi/crossed-center fires → Step 2 completed → EXIT TRIGGERED
```

### Log Messages

You'll see these messages indicating sequential order:
- `✅ [ENTRY-STEP] Step 1/3 completed IN ORDER: ...`
- `⏳ [ENTRY-STEP] Waiting for step 2/3: ...`
- `⚠️ [ENTRY-STEP] Received /webhook/... but expecting step 2/3: ... (steps must be completed IN ORDER)`
- `🎯 [ENTRY-STEP] All entry steps completed IN ORDER!`

## Validation Rules

✅ **Valid:**
- Exit with only `conditions`
- Exit with only `steps`

❌ **Invalid:**
- Exit with both `conditions` AND `steps`
- Exit with neither `conditions` nor `steps`
- Invalid combination mode (must be "any" or "all")

## Migration Guide

To convert existing strategies from conditions to steps:

```json
// Before (conditions)
"exit": {
  "combination": "any",
  "conditions": [
    {"webhook": "/webhook/macd/cross-down", "comment": "..."}
  ]
}

// After (steps)
"exit": {
  "combination": "all",
  "steps": [
    {"webhook": "/webhook/macd/cross-down", "comment": "..."},
    {"webhook": "/webhook/rsi/crossed-center", "comment": "..."}
  ]
}
```

Note: Change `combination` to `"all"` for multi-confirmation exits.
