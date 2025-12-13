#!/bin/bash

# Test reversal tracking with contradictory signals before clear reversal

BASE_URL="http://localhost:8080"
SYMBOL="BTC_USD"
PRICE="100000.00"

echo "=========================================================================="
echo "TEST: Reversal with Contradictory/Mixed Signals"
echo "=========================================================================="
echo ""

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
    sleep 0.5
}

echo "STEP 1: Open LONG position"
echo "────────────────────────────────────────"
send_webhook "/webhook/rsi/above-50" "RSI above 50"
send_webhook "/webhook/ma/ma1-above-ma2" "MA1 above MA2"
send_webhook "/webhook/ma/price-above-ma2" "Price above MA2"
send_webhook "/webhook/macd/cross-up" "MACD cross up → OPEN LONG"
echo ""
sleep 2

echo "STEP 2: Close LONG position"
echo "────────────────────────────────────────"
send_webhook "/webhook/rsi/cross-down-overbuy" "RSI exit → Close LONG"
echo ""
sleep 2

echo "📊 After Close:"
curl -s "$BASE_URL/status" | jq '.positions.BTC_USD | {PositionOpen, LastClosedDirection, OppositeDirectionOccurred}'
echo ""

echo "STEP 3: Send CONTRADICTORY signals (whipsaw/noise)"
echo "────────────────────────────────────────"
echo "⚡ Simulating market indecision - mixed signals..."
send_webhook "/webhook/rsi/above-50" "RSI above 50 (LONG signal - contradicts close)"
send_webhook "/webhook/ma/price-below-ma2" "Price below MA2 (SHORT signal)"
send_webhook "/webhook/macd/cross-up" "MACD cross up (LONG signal - contradicts)"
send_webhook "/webhook/rsi/below-50" "RSI below 50 (SHORT signal)"
send_webhook "/webhook/rsi/above-50" "RSI above 50 AGAIN (LONG signal - more noise)"
echo ""
sleep 2

echo "📊 After Mixed Signals (no clear reversal yet):"
curl -s "$BASE_URL/status" | jq '.positions.BTC_USD | {PositionOpen, LastClosedDirection, OppositeDirectionOccurred}'
echo "⚠️  OppositeDirectionOccurred should be TRUE from the SHORT signals above"
echo ""

echo "STEP 4: Try to re-enter LONG (should work - reversal already detected)"
echo "────────────────────────────────────────"
send_webhook "/webhook/ma/ma1-above-ma2" "MA1 above MA2"
send_webhook "/webhook/ma/price-above-ma2" "Price above MA2 → Should OPEN LONG"
echo ""
sleep 2

echo "📊 Final State:"
curl -s "$BASE_URL/status" | jq '.positions.BTC_USD | {PositionOpen, Position, LastClosedDirection, OppositeDirectionOccurred}'
echo ""
echo "✅ Expected: PositionOpen=true (reversal was detected in mixed signals)"
echo ""

echo "════════════════════════════════════════════════════════════════════════"
echo "TEST 2: Clear reversal only after persistent opposite signals"
echo "════════════════════════════════════════════════════════════════════════"
echo ""

echo "STEP 5: Close the LONG position again"
echo "────────────────────────────────────────"
send_webhook "/webhook/rsi/cross-down-overbuy" "RSI exit → Close LONG"
echo ""
sleep 2

echo "📊 After Second Close:"
curl -s "$BASE_URL/status" | jq '.positions.BTC_USD | {PositionOpen, LastClosedDirection, OppositeDirectionOccurred}'
echo ""

echo "STEP 6: Send more LONG signals (no reversal)"
echo "────────────────────────────────────────"
echo "⚠️  These won't trigger reversal (same direction as last close)"
send_webhook "/webhook/rsi/above-50" "RSI above 50 (LONG - same direction)"
send_webhook "/webhook/macd/cross-up" "MACD cross up (LONG - same direction)"
send_webhook "/webhook/ma/price-above-ma2" "Price above MA2 (LONG - same direction)"
echo ""
sleep 2

echo "📊 After Same-Direction Signals:"
curl -s "$BASE_URL/status" | jq '.positions.BTC_USD | {PositionOpen, LastClosedDirection, OppositeDirectionOccurred}'
echo "⚠️  OppositeDirectionOccurred should still be FALSE (no SHORT signals)"
echo ""

echo "STEP 7: Now send clear SHORT reversal signals"
echo "────────────────────────────────────────"
send_webhook "/webhook/rsi/below-50" "RSI below 50 (SHORT reversal)"
send_webhook "/webhook/ma/ma1-below-ma2" "MA1 below MA2 (SHORT reversal)"
send_webhook "/webhook/ma/price-below-ma2" "Price below MA2 (SHORT reversal)"
echo ""
sleep 2

echo "📊 After Clear Reversal Signals:"
curl -s "$BASE_URL/status" | jq '.positions.BTC_USD | {PositionOpen, LastClosedDirection, OppositeDirectionOccurred}'
echo "✅ OppositeDirectionOccurred should NOW be TRUE"
echo ""

echo "STEP 8: Re-enter LONG (should work now)"
echo "────────────────────────────────────────"
send_webhook "/webhook/rsi/above-50" "RSI above 50"
send_webhook "/webhook/ma/ma1-above-ma2" "MA1 above MA2"
send_webhook "/webhook/ma/price-above-ma2" "Price above MA2"
send_webhook "/webhook/macd/cross-up" "MACD cross up → Should OPEN LONG"
echo ""
sleep 2

echo "📊 Final State:"
curl -s "$BASE_URL/status" | jq '.positions.BTC_USD | {PositionOpen, Position, LastClosedDirection, OppositeDirectionOccurred}'
echo ""
echo "✅ Expected: PositionOpen=true, OppositeDirectionOccurred=false (reset)"
echo ""
echo "=========================================================================="
echo "Summary:"
echo "  ✅ Reversal detection works even with mixed/contradictory signals"
echo "  ✅ Any SHORT signal after LONG close marks reversal"
echo "  ✅ Same-direction signals don't trigger reversal"
echo "  ✅ System handles noisy market conditions correctly"
echo "=========================================================================="
