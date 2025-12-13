# Nested Condition Groups - Implementation Status

## Current State: PARTIALLY IMPLEMENTED

### ✅ What Works Now:

1. **Type System Updated**:
   - `ConditionNode` type supports both "condition" and "group"
   - `EntryConditions` and `ExitConditions` now use `[]ConditionNode` instead of `[]Condition`
   - Recursive structure allows unlimited nesting depth

2. **Validation Works**:
   - `validateConditionNode()` recursively validates nested groups
   - Checks combination modes at each level
   - Ensures webhooks exist for leaf conditions
   - **Both simple and nested formats validate successfully**

3. **Backward Compatibility**:
   - Simple strategies (without `type` field) still work
   - Defaults to `type="condition"` when not specified
   - All existing strategies continue to function

### ❌ What Doesn't Work Yet:

1. **Condition Evaluation Logic**:
   - `shouldOpenPosition()` and `shouldExitPosition()` still expect flat arrays
   - They don't recursively evaluate nested groups
   - State tracking (`EntryConditionsCompleted`) uses flat keys

2. **Logging**:
   - `loadStrategy()` logging doesn't understand nested groups
   - Would show incorrect condition counts

## Example: What You Can Write (But Won't Execute Yet)

```json
{
  "name": "advanced_momentum",
  "long": {
    "entry": {
      "type": "group",
      "combination": "all",
      "conditions": [
        {
          "type": "condition",
          "webhook": "/webhook/rsi/cross-up-oversell-25",
          "comment": "RSI crosses up from 25"
        },
        {
          "type": "condition",
          "webhook": "/webhook/macd/cross-up",
          "comment": "MACD crosses up"
        }
      ]
    },
    "exit": {
      "type": "group",
      "combination": "sequential",
      "conditions": [
        {
          "type": "condition",
          "webhook": "/webhook/rsi/cross-center-50",
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
          "comment": "Step 3: Any of these exits",
          "conditions": [
            {
              "type": "condition",
              "webhook": "/webhook/rsi/cross-60",
              "comment": "RSI 60 again"
            },
            {
              "type": "condition",
              "webhook": "/webhook/macd/cross-down",
              "comment": "MACD reversal"
            }
          ]
        }
      ]
    }
  }
}
```

**Status**: ✅ Validates successfully, ❌ Won't execute correctly

## What's Needed to Complete:

### 1. State Tracking (Medium Complexity)

Current:
```go
EntryConditionsCompleted map[string]bool  // "condition_0", "condition_1", etc.
```

Needed:
```go
EntryConditionsCompleted map[string]interface{}  // Supports nested structure
// Example: {
//   "condition_0": true,
//   "condition_1": true,
//   "group_2": {
//     "condition_0": false,
//     "condition_1": true
//   }
// }
```

### 2. Recursive Evaluation Function (High Complexity)

```go
func evaluateConditionGroup(
	group *EntryConditions,
	currentWebhook string,
	state map[string]interface{},
	path string,
) (matched bool, complete bool, shouldTrigger bool) {
	// Recursively evaluate each child node
	// Handle "any", "all", "sequential" at each level
	// Track completion state in nested map
	// Return whether this group is complete
}
```

### 3. Updated shouldOpenPosition (High Complexity)

```go
func shouldOpenPosition(symbol string, isLong bool, r *http.Request) bool {
	// Get entry conditions
	// Call recursive evaluator
	// Update nested state
	// Check if top-level group is complete
}
```

### 4. Logging Updates (Low Complexity)

- Show nested group structure
- Indicate which level/group conditions are firing
- Display progress through nested sequential groups

## Estimated Effort:

- **State Tracking**: 2-3 hours
- **Recursive Evaluation**: 4-6 hours
- **Testing**: 2-3 hours
- **Documentation**: 1 hour
- **Total**: ~10-15 hours of development

## Alternative: Simplified Approach

Instead of full nesting, support **one level** of grouping:

```json
{
  "exit": {
    "combination": "sequential",
    "conditions": [
      {"webhook": "/webhook/rsi/cross-50"},
      {"webhook": "/webhook/rsi/cross-60"},
      {
        "type": "group",
        "combination": "any",
        "conditions": [
          {"webhook": "/webhook/rsi/cross-70"},
          {"webhook": "/webhook/macd/cross-down"}
        ]
      }
    ]
  }
}
```

**Effort**: ~5-7 hours (half the work)

## Recommendation:

For now, use the **simple `rsi_precision.json` strategy** which works perfectly today. It achieves your trading goals without complexity.

If you need the nested logic:
1. I can implement **one-level nesting** (~5-7 hours)
2. Or full **unlimited nesting** (~10-15 hours)

Both require significant development time and extensive testing.

## Current Working Strategies:

- ✅ `rsi_precision.json` - Simple, production-ready
- ✅ `momentum.json` - Sequential conditions
- ✅ `default.json` - All conditions required
- ✅ `ma_ribbon.json` - MA trend following
- ⚠️ `advanced_momentum.json` - Validates but won't execute correctly

