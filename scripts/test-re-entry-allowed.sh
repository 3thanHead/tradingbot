#!/bin/bash

# Simple test showing re-entry ALLOWED after opposite direction signal

BASE_URL="http://localhost:8080"
SYMBOL="BTC_USD"
PRICE="100000.00"

echo "=========================================================================="
echo "TEST: Re-entry ALLOWED after opposite direction signal"
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

echo "📊 Current State:"
curl -s "$BASE_URL/status" | jq '.positions.BTC_USD | {PositionOpen, LastClosedDirection, OppositeDirectionOccurred}'
echo ""

echo "STEP 3: Send opposite (SHORT) signal to mark reversal"
echo "────────────────────────────────────────"
send_webhook "/webhook/rsi/below-50" "RSI below 50 (SHORT signal)"
send_webhook "/webhook/ma/ma1-below-ma2" "MA1 below MA2 (SHORT signal)"
echo ""
sleep 2

echo "📊 After Opposite Signal:"
curl -s "$BASE_URL/status" | jq '.positions.BTC_USD | {PositionOpen, LastClosedDirection, OppositeDirectionOccurred}'
echo ""
echo "✅ OppositeDirectionOccurred should now be TRUE"
echo ""

echo "STEP 4: Re-send LONG conditions → Should ALLOW re-entry"
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
echo "✅ Expected: PositionOpen=true, OppositeDirectionOccurred=false (reset on open)"
echo ""
echo "=========================================================================="
