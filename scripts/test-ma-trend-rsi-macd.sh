#!/bin/bash

# Test script for MA Trend + RSI 50 + MACD Strategy
# Tests entry/exit conditions and ensures opposite events reverse the state

BASE_URL="http://localhost:8080"
STRATEGY="ma_trend_rsi_macd"

echo "🧪 Testing MA Trend + RSI 50 + MACD Strategy"
echo "=============================================="
echo ""
echo "Strategy: $STRATEGY"
echo "This test verifies:"
echo "  1. LONG entry requires: MA1>MA2 (cross), MA1>MA4, RSI>50, ATR>threshold, MACD cross up"
echo "  2. LONG exit on: MACD cross down"
echo "  3. SHORT entry requires: MA1<MA2 (cross), MA1<MA4, RSI<50, ATR>threshold, MACD cross down"
echo "  4. SHORT exit on: MACD cross up"
echo "  5. Opposite events should reverse the state"
echo ""

# Helper function to check status
check_status() {
    echo "📊 Current Status:"
    curl -s "$BASE_URL/status?strategy=$STRATEGY" | jq '.'
    echo ""
}

# Helper function to wait
wait_seconds() {
    echo "⏳ Waiting $1 seconds..."
    sleep $1
    echo ""
}

# Helper function to send webhook
send_webhook() {
    local endpoint=$1
    local description=$2
    echo "➡️  $description"
    echo "    Endpoint: $endpoint"
    curl -s -X POST "$BASE_URL$endpoint" \
      -H "Content-Type: application/json" \
      -d '{
        "ticker": "EUR_USD",
        "exchange": "OANDA",
        "strategy": "'"$STRATEGY"'",
        "interval": "15",
        "close": "1.0850",
        "open": "1.0840",
        "high": "1.0855",
        "low": "1.0835",
        "volume": "1000",
        "time": "2025-12-18T14:00:00Z",
        "timenow": "2025-12-18T14:00:00Z"
      }' | jq '.'
    echo ""
}

# Reset simulation mode
echo "🔄 Resetting simulation..."
curl -s -X POST "$BASE_URL/simulation/reset?strategy=$STRATEGY" | jq '.'
echo ""
wait_seconds 1

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "TEST 1: LONG ENTRY - All Conditions Must Be Met"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

echo "Step 1.1: Set ATR above threshold (required for both long and short)"
send_webhook "/webhook/atr/above-threshold" "ATR above threshold"
wait_seconds 1

echo "Step 1.2: Set RSI above 50"
send_webhook "/webhook/rsi/above-50" "RSI above 50"
wait_seconds 1

echo "Step 1.3: Set MA1 above MA4 (trend confirmation)"
send_webhook "/webhook/ma/ma1-above-ma4" "MA1 above MA4"
wait_seconds 1

echo "Step 1.4: MACD crosses up (trigger)"
send_webhook "/webhook/macd/cross-up" "MACD cross up"
wait_seconds 1

echo "Step 1.5: Verify NO position yet (missing MA1>MA2 cross trigger)"
check_status
echo "✅ Expected: No position yet (missing MA1>MA2 trigger)"
echo ""

echo "Step 1.6: MA1 crosses above MA2 (final trigger - should open LONG)"
send_webhook "/webhook/ma/ma1-above-ma2" "MA1 crosses above MA2"
wait_seconds 2

echo "Step 1.7: Verify LONG position opened"
check_status
echo "✅ Expected: position_open=true, position='long', all conditions met"
echo ""

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "TEST 2: LONG EXIT - MACD Cross Down"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

echo "Step 2.1: MACD crosses down (should exit LONG)"
send_webhook "/webhook/macd/cross-down" "MACD cross down"
wait_seconds 2

echo "Step 2.2: Verify LONG position closed"
check_status
echo "✅ Expected: position_open=false, position closed"
echo ""

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "TEST 3: OPPOSITE EVENTS REVERSE STATE (LONG → SHORT)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

echo "Step 3.1: Reset and set up for LONG entry again"
curl -s -X POST "$BASE_URL/simulation/reset?strategy=$STRATEGY" | jq '.'
wait_seconds 1

send_webhook "/webhook/atr/above-threshold" "ATR above threshold"
send_webhook "/webhook/rsi/above-50" "RSI above 50"
send_webhook "/webhook/ma/ma1-above-ma4" "MA1 above MA4"
send_webhook "/webhook/macd/cross-up" "MACD cross up"
send_webhook "/webhook/ma/ma1-above-ma2" "MA1 crosses above MA2"
wait_seconds 2

echo "Step 3.2: Verify LONG position"
check_status
echo ""

echo "Step 3.3: Send OPPOSITE signals for SHORT"
echo "This should reverse the state from LONG setup to SHORT setup"
echo ""

send_webhook "/webhook/rsi/below-50" "RSI below 50 (opposite of above-50)"
wait_seconds 1

send_webhook "/webhook/ma/ma1-below-ma4" "MA1 below MA4 (opposite of above-ma4)"
wait_seconds 1

send_webhook "/webhook/ma/ma1-below-ma2" "MA1 below MA2 (opposite of above-ma2)"
wait_seconds 1

echo "Step 3.4: Check if LONG was exited due to MACD cross down requirement"
echo "Note: LONG should still be open because exit requires MACD cross down"
check_status
echo ""

echo "Step 3.5: Now send MACD cross down (LONG exit trigger)"
send_webhook "/webhook/macd/cross-down" "MACD cross down (LONG exit + SHORT entry trigger)"
wait_seconds 2

echo "Step 3.6: Verify LONG closed and SHORT potentially setup"
check_status
echo "✅ Expected: LONG closed. SHORT might open if all conditions met simultaneously"
echo ""

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "TEST 4: SHORT ENTRY - All Conditions Must Be Met"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

echo "Step 4.1: Reset for clean SHORT test"
curl -s -X POST "$BASE_URL/simulation/reset?strategy=$STRATEGY" | jq '.'
wait_seconds 1

echo "Step 4.2: Set ATR above threshold"
send_webhook "/webhook/atr/above-threshold" "ATR above threshold"
wait_seconds 1

echo "Step 4.3: Set RSI below 50"
send_webhook "/webhook/rsi/below-50" "RSI below 50"
wait_seconds 1

echo "Step 4.4: Set MA1 below MA4 (trend confirmation)"
send_webhook "/webhook/ma/ma1-below-ma4" "MA1 below MA4"
wait_seconds 1

echo "Step 4.5: MACD crosses down (trigger)"
send_webhook "/webhook/macd/cross-down" "MACD cross down"
wait_seconds 1

echo "Step 4.6: Verify NO position yet (missing MA1<MA2 cross trigger)"
check_status
echo "✅ Expected: No position yet (missing MA1<MA2 trigger)"
echo ""

echo "Step 4.7: MA1 crosses below MA2 (final trigger - should open SHORT)"
send_webhook "/webhook/ma/ma1-below-ma2" "MA1 crosses below MA2"
wait_seconds 2

echo "Step 4.8: Verify SHORT position opened"
check_status
echo "✅ Expected: position_open=true, position='short', all conditions met"
echo ""

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "TEST 5: SHORT EXIT - MACD Cross Up"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

echo "Step 5.1: MACD crosses up (should exit SHORT)"
send_webhook "/webhook/macd/cross-up" "MACD cross up"
wait_seconds 2

echo "Step 5.2: Verify SHORT position closed"
check_status
echo "✅ Expected: position_open=false, position closed"
echo ""

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "TEST 6: OPPOSITE EVENTS REVERSE STATE (SHORT → LONG)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

echo "Step 6.1: Reset and set up for SHORT entry again"
curl -s -X POST "$BASE_URL/simulation/reset?strategy=$STRATEGY" | jq '.'
wait_seconds 1

send_webhook "/webhook/atr/above-threshold" "ATR above threshold"
send_webhook "/webhook/rsi/below-50" "RSI below 50"
send_webhook "/webhook/ma/ma1-below-ma4" "MA1 below MA4"
send_webhook "/webhook/macd/cross-down" "MACD cross down"
send_webhook "/webhook/ma/ma1-below-ma2" "MA1 crosses below MA2"
wait_seconds 2

echo "Step 6.2: Verify SHORT position"
check_status
echo ""

echo "Step 6.3: Send OPPOSITE signals for LONG"
echo "This should reverse the state from SHORT setup to LONG setup"
echo ""

send_webhook "/webhook/rsi/above-50" "RSI above 50 (opposite of below-50)"
wait_seconds 1

send_webhook "/webhook/ma/ma1-above-ma4" "MA1 above MA4 (opposite of below-ma4)"
wait_seconds 1

send_webhook "/webhook/ma/ma1-above-ma2" "MA1 above MA2 (opposite of below-ma2)"
wait_seconds 1

echo "Step 6.4: Check if SHORT was exited due to MACD cross up requirement"
echo "Note: SHORT should still be open because exit requires MACD cross up"
check_status
echo ""

echo "Step 6.5: Now send MACD cross up (SHORT exit trigger)"
send_webhook "/webhook/macd/cross-up" "MACD cross up (SHORT exit + LONG entry trigger)"
wait_seconds 2

echo "Step 6.6: Verify SHORT closed and LONG potentially setup"
check_status
echo "✅ Expected: SHORT closed. LONG might open if all conditions met simultaneously"
echo ""

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "TEST 7: PARTIAL CONDITIONS - No Trade Should Open"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

echo "Step 7.1: Reset"
curl -s -X POST "$BASE_URL/simulation/reset?strategy=$STRATEGY" | jq '.'
wait_seconds 1

echo "Step 7.2: Send only 3 out of 5 required LONG conditions"
send_webhook "/webhook/atr/above-threshold" "ATR above threshold (1/5)"
send_webhook "/webhook/rsi/above-50" "RSI above 50 (2/5)"
send_webhook "/webhook/ma/ma1-above-ma4" "MA1 above MA4 (3/5)"
wait_seconds 1

echo "Step 7.3: Verify NO position opened (only 3/5 conditions met)"
check_status
echo "✅ Expected: position_open=false (missing MACD cross up and MA1>MA2)"
echo ""

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "TEST 8: RAPID STATE REVERSAL"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

echo "Step 8.1: Reset"
curl -s -X POST "$BASE_URL/simulation/reset?strategy=$STRATEGY" | jq '.'
wait_seconds 1

echo "Step 8.2: Rapidly alternate between LONG and SHORT signals"
send_webhook "/webhook/rsi/above-50" "RSI above 50 (LONG)"
send_webhook "/webhook/rsi/below-50" "RSI below 50 (SHORT - should override)"
wait_seconds 1

send_webhook "/webhook/ma/ma1-above-ma2" "MA1 above MA2 (LONG)"
send_webhook "/webhook/ma/ma1-below-ma2" "MA1 below MA2 (SHORT - should override)"
wait_seconds 1

send_webhook "/webhook/macd/cross-up" "MACD cross up (LONG)"
send_webhook "/webhook/macd/cross-down" "MACD cross down (SHORT - should override)"
wait_seconds 1

echo "Step 8.3: Check final state (should reflect last signals - SHORT oriented)"
check_status
echo "✅ Expected: State should reflect the most recent (SHORT) signals"
echo ""

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "🏁 TEST SUMMARY"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "Tests completed:"
echo "  ✓ Test 1: LONG entry requires all 5 conditions"
echo "  ✓ Test 2: LONG exit on MACD cross down"
echo "  ✓ Test 3: Opposite signals while in LONG position"
echo "  ✓ Test 4: SHORT entry requires all 5 conditions"
echo "  ✓ Test 5: SHORT exit on MACD cross up"
echo "  ✓ Test 6: Opposite signals while in SHORT position"
echo "  ✓ Test 7: Partial conditions don't trigger trades"
echo "  ✓ Test 8: Rapid state reversal handling"
echo ""
echo "Review the output above to verify:"
echo "  1. Positions only open when ALL conditions are met"
echo "  2. Exits trigger correctly on opposite MACD signals"
echo "  3. Opposite events properly update state flags"
echo "  4. Incomplete conditions don't trigger trades"
echo ""

# Final status check
echo "📊 Final Status:"
check_status
