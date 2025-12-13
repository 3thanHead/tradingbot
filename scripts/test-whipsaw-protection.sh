#!/bin/bash

# Comprehensive test for whipsaw protection with immediate re-entry capability
# This tests:
# 1. Normal position open
# 2. Position close with immediate re-entry (first close - allowed)
# 3. Second close without opposite signal (should NOT allow immediate re-entry - BLOCKED)
# 4. Opposite direction signal to mark reversal
# 5. Re-entry after reversal confirmation (allowed)

BASE_URL="http://localhost:8080"
SYMBOL="BTC_USD"
PRICE="100000.00"

echo "==================================================================================="
echo "WHIPSAW PROTECTION TEST - Comprehensive Flow"
echo "==================================================================================="
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

# Get current whipsaw protection state
get_whipsaw_state() {
    echo "📊 Whipsaw Protection State:"
    curl -s "$BASE_URL/status" | jq '.positions.BTC_USD | {
        PositionOpen,
        Position,
        LastClosedDirection,
        OppositeDirectionOccurred
    }' 2>/dev/null || echo "Could not fetch status"
    echo ""
}

# Set all LONG entry conditions
send_long_conditions() {
    echo "🔵 Sending all LONG entry conditions..."
    send_webhook "/webhook/rsi/above-50" "RSI above 50"
    sleep 0.5
    send_webhook "/webhook/ma/ma1-above-ma2" "MA1 above MA2"
    sleep 0.5
    send_webhook "/webhook/ma/price-above-ma2" "Price above MA2"
    sleep 0.5
    send_webhook "/webhook/macd/cross-up" "MACD cross up"
    sleep 1
}

echo "═══════════════════════════════════════════════════════════════════════════════"
echo "SCENARIO 1: Open initial LONG position"
echo "═══════════════════════════════════════════════════════════════════════════════"
send_long_conditions
get_whipsaw_state

echo "═══════════════════════════════════════════════════════════════════════════════"
echo "SCENARIO 2: Close LONG → Should immediately re-enter (first close)"
echo "Expected: LastClosedDirection="" allows first re-entry"
echo "═══════════════════════════════════════════════════════════════════════════════"
send_webhook "/webhook/rsi/cross-down-overbuy" "RSI exit signal → Close LONG"
sleep 2
get_whipsaw_state
echo "✅ Expected Result: Position re-opened immediately"
echo "✅ LastClosedDirection should now = 'long'"
echo "✅ OppositeDirectionOccurred should = false"
echo ""

echo "═══════════════════════════════════════════════════════════════════════════════"
echo "SCENARIO 3: Close LONG again → Should NOT re-enter (whipsaw protection)"
echo "Expected: LastClosedDirection='long' + OppositeDirectionOccurred=false = BLOCKED"
echo "═══════════════════════════════════════════════════════════════════════════════"
send_webhook "/webhook/rsi/cross-down-overbuy" "RSI exit signal → Close LONG (2nd time)"
sleep 2
get_whipsaw_state
echo "✅ Expected Result: Position should be CLOSED, no re-entry"
echo "✅ LastClosedDirection should = 'long'"
echo "✅ OppositeDirectionOccurred should = false"
echo "✅ Position should = 'none' or PositionOpen = false"
echo ""

echo "═══════════════════════════════════════════════════════════════════════════════"
echo "SCENARIO 4: Trigger SHORT signal to mark reversal"
echo "Expected: OppositeDirectionOccurred = true"
echo "═══════════════════════════════════════════════════════════════════════════════"
send_webhook "/webhook/rsi/below-50" "RSI below 50 (SHORT signal) → Mark reversal"
sleep 1
get_whipsaw_state
echo "✅ Expected Result: OppositeDirectionOccurred = true"
echo ""

echo "═══════════════════════════════════════════════════════════════════════════════"
echo "SCENARIO 5: Now send LONG conditions → Should open (reversal confirmed)"
echo "Expected: LastClosedDirection='long' + OppositeDirectionOccurred=true = ALLOWED"
echo "═══════════════════════════════════════════════════════════════════════════════"
send_long_conditions
get_whipsaw_state
echo "✅ Expected Result: LONG position opened"
echo "✅ OppositeDirectionOccurred reset to false"
echo ""

echo "═══════════════════════════════════════════════════════════════════════════════"
echo "TEST COMPLETE"
echo "═══════════════════════════════════════════════════════════════════════════════"
echo ""
echo "Summary of Whipsaw Protection Logic:"
echo "  • First close (LastClosedDirection='') → Allow immediate re-entry ✅"
echo "  • Second close (same direction, no reversal) → BLOCK re-entry ✅"
echo "  • After opposite signal seen → Reset protection, allow re-entry ✅"
echo ""
echo "This prevents whipsaws while allowing legitimate quick reversals!"
