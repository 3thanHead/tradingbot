#!/bin/bash

# Test script for reversal tracking (whipsaw protection)
# Tests that position can only re-enter same direction after seeing opposite direction signals

BASE_URL="http://localhost:8080"
SYMBOL="BTC_USD"
PRICE="100000.00"

echo "=========================================="
echo "Testing Reversal Tracking (Whipsaw Protection)"
echo "=========================================="
echo ""

# Helper function to send webhook
send_webhook() {
    local endpoint=$1
    local description=$2
    
    echo "➡️  $description"
    curl -s -X POST "$BASE_URL$endpoint" \
        -H "Content-Type: application/json" \
        -d "{
            \"ticker\": \"$SYMBOL\",
            \"close\": \"$PRICE\",
            \"exchange\": \"COINBASE\"
        }" | jq -r '.message' || echo "Request sent"
    echo ""
}

# Get current status
get_status() {
    echo "📊 Current Status:"
    curl -s "$BASE_URL/status" | jq '.positions.BTC_USD | {
        position_open: .position_open,
        position: .position,
        last_closed_direction: .last_closed_direction,
        opposite_direction_occurred: .opposite_direction_occurred
    }' 2>/dev/null || echo "Could not fetch status"
    echo ""
}

echo "🧪 TEST 1: Open LONG position"
echo "════════════════════════════════════════"
send_webhook "/webhook/rsi/above-50" "1/4: RSI above 50"
sleep 1
send_webhook "/webhook/ma/ma1-above-ma2" "2/4: MA1 above MA2"
sleep 1
send_webhook "/webhook/ma/price-above-ma2" "3/4: Price above MA2"
sleep 1
send_webhook "/webhook/macd/cross-up" "4/4: MACD cross up → Should OPEN LONG"
sleep 2
get_status

echo ""
echo "🧪 TEST 2: Close LONG position (via exit condition)"
echo "════════════════════════════════════════"
send_webhook "/webhook/rsi/cross-down-overbuy" "RSI cross down overbuy (exit condition) → Should CLOSE LONG"
sleep 2
get_status

echo ""
echo "🧪 TEST 3: Try to re-enter LONG immediately (should be BLOCKED)"
echo "════════════════════════════════════════"
echo "⚠️  Expected: Entry blocked because no opposite (SHORT) direction seen yet"
send_webhook "/webhook/ma/ma1-above-ma2" "1/4: MA1 above MA2"
sleep 1
send_webhook "/webhook/rsi/above-50" "2/4: RSI above 50"
sleep 1
send_webhook "/webhook/ma/price-above-ma2" "3/4: Price above MA2"
sleep 1
send_webhook "/webhook/macd/cross-up" "4/4: MACD cross up → Should be BLOCKED"
sleep 2
get_status

echo ""
echo "🧪 TEST 4: Trigger SHORT entry condition (marks reversal)"
echo "════════════════════════════════════════"
echo "✅ This should mark opposite_direction_occurred = true"
send_webhook "/webhook/rsi/below-50" "RSI below 50 (SHORT condition) → Marks reversal"
sleep 2
get_status

echo ""
echo "🧪 TEST 5: Now try to enter LONG again (should be ALLOWED)"
echo "════════════════════════════════════════"
echo "✅ Expected: Entry allowed because we saw opposite direction"
send_webhook "/webhook/ma/ma1-above-ma2" "1/4: MA1 above MA2"
sleep 1
send_webhook "/webhook/rsi/above-50" "2/4: RSI above 50"
sleep 1
send_webhook "/webhook/ma/price-above-ma2" "3/4: Price above MA2"
sleep 1
send_webhook "/webhook/macd/cross-up" "4/4: MACD cross up → Should OPEN LONG"
sleep 2
get_status

echo ""
echo "=========================================="
echo "✅ Test Complete!"
echo "=========================================="
echo ""
echo "Summary of Expected Results:"
echo "  TEST 1: LONG position opens ✅"
echo "  TEST 2: LONG position closes ✅"
echo "  TEST 3: Re-entry BLOCKED (no reversal seen) ✅"
echo "  TEST 4: opposite_direction_occurred = true ✅"
echo "  TEST 5: LONG position opens (after reversal) ✅"
