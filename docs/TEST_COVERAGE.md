# Test Coverage: pseudo.txt Requirements

This document shows how the trading bot implementation and tests cover all scenarios from `pseudo.txt`.

## 📋 Requirements from pseudo.txt

### Scenario 1: Open SHORT Position (RSI Overbought)
```
ON (RSI_GREATER_THAN_70_TRADING_VIEW_EVENT):
    IF (!position_open):
        ON (RSI_MOVING_DOWN_TRADING_VIEW_EVENT):
            open_short_position(currency_pair)
```

**Implementation:** ✅ `main.go` lines 138-181
- `handleRSICrossedUp()`: Sets `RSICrossedUp = true` flag
- `handleRSIMovingDown()`: Checks `RSICrossedUp && !PositionOpen`, then calls `openShortPosition()`

**Test Coverage:** ✅ `test-all-scenarios.sh` Scenario 1
- Sends RSI > 70 event
- Sends RSI Moving Down event
- Verifies SHORT position opened

---

### Scenario 2: Open LONG Position (RSI Oversold)
```
ON (RSI_LESS_THAN_30_TRADING_VIEW_EVENT):
    IF (!position_open):
        ON (RSI_MOVING_UP_TRADING_VIEW_EVENT):
            open_long_position(currency_pair)
```

**Implementation:** ✅ `main.go` lines 105-228
- `handleRSICrossedDown()`: Sets `RSICrossedDown = true` flag
- `handleRSIMovingUp()`: Checks `RSICrossedDown && !PositionOpen`, then calls `openLongPosition()`

**Test Coverage:** ✅ `test-all-scenarios.sh` Scenario 2
- Sends RSI < 30 event
- Sends RSI Moving Up event
- Verifies LONG position opened

---

### Scenario 3: Close SHORT Position (MACD Bullish)
```
ON (MACD_CROSS_UP_TRADING_VIEW_EVENT):
    IF (position_open):
        IF (position == "short"):
            ON (MACD_MOVING_UP_TRADING_VIEW_EVENT):
                close_position(currency_pair)
```

**Implementation:** ✅ `main.go` lines 236-329
- `handleMACDCrossUp()`: Sets `MACDCrossedUp = true` flag
- `handleMACDMovingUp()`: Checks `MACDCrossedUp && PositionOpen && Position=="short"`, then calls `closePosition()`

**Test Coverage:** ✅ `test-all-scenarios.sh` Scenario 3
- Opens SHORT position first (using Scenario 1)
- Sends MACD Cross Up event
- Sends MACD Moving Up event
- Verifies SHORT position closed

---

### Scenario 4: Close LONG Position (MACD Bearish)
```
ON (MACD_CROSS_DOWN_TRADING_VIEW_EVENT):
    IF (position_open):
        IF (position == "long"):
            ON (MACD_MOVING_DOWN_TRADING_VIEW_EVENT):
                close_position(currency_pair)
```

**Implementation:** ✅ `main.go` lines 258-380
- `handleMACDCrossDown()`: Sets `MACDCrossedDown = true` flag
- `handleMACDMovingDown()`: Checks `MACDCrossedDown && PositionOpen && Position=="long"`, then calls `closePosition()`

**Test Coverage:** ✅ `test-all-scenarios.sh` Scenario 4
- Opens LONG position first (using Scenario 2)
- Sends MACD Cross Down event
- Sends MACD Moving Down event
- Verifies LONG position closed

---

## 🛡️ Edge Cases Tested

### Edge Case 1: Flag Not Set
**Requirement:** Actions should only trigger when condition flag is set

**Test:** Try RSI Moving Down without first setting RSI > 70
- **Expected:** No position opened
- **Coverage:** `test-all-scenarios.sh` Edge Case 1

---

### Edge Case 2: Position Already Open
**Requirement:** Cannot open new position when one is already open

**Test:** Try to open LONG when SHORT is already open
- **Expected:** Action skipped, SHORT remains open
- **Coverage:** `test-all-scenarios.sh` Edge Case 2

---

### Edge Case 3: No Position to Close
**Requirement:** Cannot close position when none exists

**Test:** Try to close position when position_open = false
- **Expected:** Action skipped
- **Coverage:** `test-all-scenarios.sh` Edge Case 3

---

### Edge Case 4: Wrong Close Signal
**Requirement:** SHORT closes with MACD Up, LONG closes with MACD Down

**Test:** Try to close SHORT with MACD Moving Down (wrong signal)
- **Expected:** Action skipped, SHORT remains open
- **Coverage:** `test-all-scenarios.sh` Edge Case 4

---

## 📊 State Management

The bot uses boolean flags to track conditions:

| Flag | Meaning | Set By | Reset By |
|------|---------|--------|----------|
| `RSICrossedUp` | RSI > 70 occurred | `/webhook/rsi/greater-than-70` | After opening SHORT or when opposite flag set |
| `RSICrossedDown` | RSI < 30 occurred | `/webhook/rsi/less-than-30` | After opening LONG or when opposite flag set |
| `MACDCrossedUp` | MACD crossed above signal | `/webhook/macd/cross-up` | After closing SHORT or when opposite flag set |
| `MACDCrossedDown` | MACD crossed below signal | `/webhook/macd/cross-down` | After closing LONG or when opposite flag set |

---

## ✅ Verification Checklist

Run the comprehensive test to verify all scenarios:

```bash
./scripts/test-all-scenarios.sh
```

### Expected Results:

- ✅ Scenario 1: SHORT position opens when RSI > 70 then moves down
- ✅ Scenario 2: LONG position opens when RSI < 30 then moves up
- ✅ Scenario 3: SHORT position closes when MACD crosses up then moves up
- ✅ Scenario 4: LONG position closes when MACD crosses down then moves down
- ✅ Edge Case 1: No action without flag
- ✅ Edge Case 2: Cannot open when position exists
- ✅ Edge Case 3: Cannot close when no position
- ✅ Edge Case 4: Cannot close with wrong signal

---

## 🔍 Log Verification

After running tests, check logs for these patterns:

```bash
docker logs tradingview-webhook-bot | grep "\[TRADE\]"
```

**Expected log entries:**

```
✅ [TRADE] Conditions met! Opening SHORT position
✅ [TRADE] Conditions met! Closing SHORT position
✅ [TRADE] Conditions met! Opening LONG position
✅ [TRADE] Conditions met! Closing LONG position
⏭️  [SKIP] Conditions not met - No trade executed
```

---

## 📝 Summary

| Requirement | Implementation | Test Coverage | Status |
|-------------|----------------|---------------|--------|
| Open SHORT (RSI) | ✅ handleRSICrossedUp + handleRSIMovingDown | ✅ Scenario 1 | ✅ Complete |
| Open LONG (RSI) | ✅ handleRSICrossedDown + handleRSIMovingUp | ✅ Scenario 2 | ✅ Complete |
| Close SHORT (MACD) | ✅ handleMACDCrossUp + handleMACDMovingUp | ✅ Scenario 3 | ✅ Complete |
| Close LONG (MACD) | ✅ handleMACDCrossDown + handleMACDMovingDown | ✅ Scenario 4 | ✅ Complete |
| Flag validation | ✅ All handlers check flags | ✅ Edge Case 1 | ✅ Complete |
| Position open check | ✅ All open handlers check !PositionOpen | ✅ Edge Case 2 | ✅ Complete |
| Position close check | ✅ All close handlers check PositionOpen | ✅ Edge Case 3 | ✅ Complete |
| Direction matching | ✅ Close handlers check position type | ✅ Edge Case 4 | ✅ Complete |

**All requirements from pseudo.txt are fully implemented and tested!** ✅
