# Sequential Order Execution

## Overview

When using `"combination": "all"` with steps (for both entry and exit), the trading bot **enforces sequential order execution**. This means webhooks must fire in the exact order specified in your strategy JSON.

## How It Works

### Entry Steps (Sequential)
```json
{
  "entry": {
    "combination": "all",
    "steps": [
      {"webhook": "/webhook/rsi/crossed-down", "comment": "Step 1"},
      {"webhook": "/webhook/macd/cross-up", "comment": "Step 2"},
      {"webhook": "/webhook/rsi/cross-center", "comment": "Step 3"}
    ]
  }
}
```

**Required Order:**
1. ✅ `/webhook/rsi/crossed-down` must fire first
2. ✅ `/webhook/macd/cross-up` must fire second
3. ✅ `/webhook/rsi/cross-center` must fire third
4. 🎯 Position opens only after all 3 steps complete IN ORDER

### Exit Steps (Sequential)
```json
{
  "exit": {
    "combination": "all",
    "steps": [
      {"webhook": "/webhook/macd/cross-down", "comment": "Step 1"},
      {"webhook": "/webhook/rsi/crossed-center", "comment": "Step 2"}
    ]
  }
}
```

**Required Order:**
1. ✅ `/webhook/macd/cross-down` must fire first
2. ✅ `/webhook/rsi/crossed-center` must fire second
3. 🎯 Position closes only after both steps complete IN ORDER

## Out-of-Order Rejection

If webhooks fire out of order, they are **rejected**:

### Example: Wrong Order

```
Scenario: Strategy expects [Step 1: MACD, Step 2: RSI]

Time 10:00 → RSI webhook fires
  ⚠️ [EXIT-STEP] Received /webhook/rsi/crossed-center but expecting step 1/2: 
     /webhook/macd/cross-down (steps must be completed IN ORDER)
  ❌ Rejected - position stays open

Time 10:05 → MACD webhook fires
  ✅ [EXIT-STEP] Step 1/2 completed IN ORDER: MACD bearish crossover
  ⏳ [EXIT-STEP] Waiting for step 2/2: RSI crosses back down through 50

Time 10:10 → RSI webhook fires (again)
  ✅ [EXIT-STEP] Step 2/2 completed IN ORDER: RSI crosses back down through 50
  🎯 [EXIT-STEP] All exit steps completed IN ORDER!
  ✅ Position closed
```

## Log Messages

### Success Messages
- `✅ [ENTRY-STEP] Step 1/3 completed IN ORDER: RSI crosses 30`
- `⏳ [ENTRY-STEP] Waiting for step 2/3: MACD bullish crossover`
- `🎯 [ENTRY-STEP] All entry steps completed IN ORDER!`

### Warning Messages
- `⚠️ [ENTRY-STEP] Received /webhook/macd/cross-up but expecting step 1/3: /webhook/rsi/crossed-down (steps must be completed IN ORDER)`

### Strategy Loading
When loading a strategy with `"combination": "all"`, you'll see:
- `🟢 LONG ENTRY (ALL - 3 steps IN ORDER):`
- `🔴 LONG EXIT (ALL - 2 steps IN ORDER):`

## Why Sequential Order?

### Benefits
1. **Precise control**: Ensures signals fire in logical sequence
2. **Prevents false signals**: Rejects out-of-order noise
3. **Market confirmation**: Later steps confirm earlier signals
4. **Momentum validation**: Ensures proper trend development

### Example: Momentum Strategy

The momentum strategy requires sequential confirmation:

**Long Entry:**
1. RSI crosses 30 (oversold) → Market showing potential
2. MACD crosses up → Momentum shift confirmed
3. RSI crosses 50 → Upward momentum validated
4. ✅ Open LONG position

**Long Exit:**
1. MACD crosses down → Momentum reversal detected
2. RSI crosses down through 50 → Downward momentum confirmed
3. ✅ Close LONG position

This sequence ensures you're not entering on a false MACD signal before RSI confirms oversold conditions, and you're not exiting prematurely before both indicators confirm reversal.

## Alternative: "any" Combination

If you don't want sequential order enforcement, use `"combination": "any"`:

```json
{
  "exit": {
    "combination": "any",
    "steps": [
      {"webhook": "/webhook/macd/cross-down", "comment": "Exit on MACD"},
      {"webhook": "/webhook/rsi/crossed-up", "comment": "Exit on RSI"}
    ]
  }
}
```

This exits immediately when **ANY** webhook fires (no order required).

## State Management

- Entry steps are tracked per symbol
- Exit steps are tracked per symbol
- Steps reset when:
  - Position is opened (resets entry steps, initializes exit steps)
  - Position is closed (resets exit steps, initializes entry steps)
- If a position is already open, entry step tracking is ignored
- If no position is open, exit step tracking is ignored

## Best Practices

1. **Design logical sequences**: Later steps should confirm earlier ones
2. **Avoid circular dependencies**: Don't create impossible sequences
3. **Test your strategy**: Use test scripts to validate step order
4. **Monitor logs**: Watch for out-of-order rejections
5. **Use meaningful comments**: Clearly describe what each step validates

## Technical Implementation

- Entry steps use key format: `"step_0"`, `"step_1"`, `"step_2"`, etc.
- Exit steps use key format: `"step_0"`, `"step_1"`, `"step_2"`, etc.
- Only the NEXT incomplete step is accepted
- Previous steps must be completed before later steps are accepted
- State is stored in `PositionState.EntryStepsCompleted` and `PositionState.ExitStepsCompleted` maps
