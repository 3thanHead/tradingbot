#!/bin/bash

# Integration test for MA Ribbon + RSI 50 Strategy
# Tests MA#1 cross MA#2 endpoints with full scenarios

BASE_URL="http://localhost:8080"
SYMBOL="EUR_USD"
EXCHANGE="BINANCE"  # Use non-OANDA exchange for simulated trades in testing

echo "=========================================="
echo "MA Ribbon Strategy Integration Tests"
echo "=========================================="
echo "Testing endpoints:"
echo "  - /webhook/rsi/above-50"
echo "  - /webhook/rsi/below-50"
echo "  - /webhook/ma/ma1-cross-up-ma2"
echo "  - /webhook/ma/ma1-cross-down-ma2"
echo "  - /webhook/stochastic-rsi/cross-down-50"
echo "  - /webhook/stochastic-rsi/cross-up-50"
echo "=========================================="
echo ""

# Helper function to send webhook
send_webhook() {
    local path=$1
    local name=$2
    echo "📤 Sending: $name"
    echo "   URL: ${BASE_URL}${path}"
    response=$(curl -s -X POST "${BASE_URL}${path}" \
        -H "Content-Type: application/json" \
        -d "{\"ticker\":\"${SYMBOL}\",\"exchange\":\"${EXCHANGE}\",\"interval\":\"15\",\"close\":\"1.0850\"}")
    echo "   Response: $(echo "$response" | jq -r '.message')"
    sleep 1
}

# Helper to check status
check_status() {
    echo ""
    echo "📊 Current Status:"
    curl -s "${BASE_URL}/status" | jq ".positions.${SYMBOL} | {position: .Position, open: .PositionOpen, RSIAbove50: .RSIAbove50, RSIBelow50: .RSIBelow50, EMA9CrossedUpEMA21: .EMA9CrossedUpEMA21, EMA9CrossedDownEMA21: .EMA9CrossedDownEMA21, StochRSICrossedUp50: .StochRSICrossedUp50, StochRSICrossedDown50: .StochRSICrossedDown50}"
    echo ""
}

# Test 1: LONG Entry Scenario
echo "=========================================="
echo "TEST 1: LONG Entry"
echo "Strategy requires:"
echo "  1. RSI above 50"
echo "  2. MA#1 cross up MA#2"
echo "=========================================="
echo ""

echo "Step 1: Send RSI above 50"
send_webhook "/webhook/rsi/above-50" "RSI Above 50"
check_status
echo "✅ Expected: RSIAbove50=true, position not opened yet (1/2 conditions)"
echo ""

echo "Step 2: Send MA#1 cross up MA#2"
send_webhook "/webhook/ma/ma1-cross-up-ma2" "MA#1 Cross Up MA#2"
check_status
echo "✅ Expected: LONG position opened (2/2 conditions met)"
echo ""
sleep 2

# Test 2: LONG Exit via Stochastic RSI
echo "=========================================="
echo "TEST 2: LONG Exit via Stochastic RSI"
echo "Exit triggers:"
echo "  - Stochastic RSI cross down 50 (OR)"
echo "  - MA#1 cross down MA#2"
echo "=========================================="
echo ""

echo "Sending Stochastic RSI cross down 50"
send_webhook "/webhook/stochastic-rsi/cross-down-50" "Stochastic RSI Cross Down 50"
check_status
echo "✅ Expected: LONG position closed, all indicators reset"
echo ""
sleep 2

# Test 3: SHORT Entry Scenario
echo "=========================================="
echo "TEST 3: SHORT Entry"
echo "Strategy requires:"
echo "  1. RSI below 50"
echo "  2. MA#1 cross down MA#2"
echo "=========================================="
echo ""

echo "Step 1: Send RSI below 50"
send_webhook "/webhook/rsi/below-50" "RSI Below 50"
check_status
echo "✅ Expected: RSIBelow50=true, position not opened yet (1/2 conditions)"
echo ""

echo "Step 2: Send MA#1 cross down MA#2"
send_webhook "/webhook/ma/ma1-cross-down-ma2" "MA#1 Cross Down MA#2"
check_status
echo "✅ Expected: SHORT position opened (2/2 conditions met)"
echo ""
sleep 2

# Test 4: SHORT Exit via MA cross
echo "=========================================="
echo "TEST 4: SHORT Exit via MA#1 cross back up"
echo "Exit triggers:"
echo "  - Stochastic RSI cross up 50 (OR)"
echo "  - MA#1 cross up MA#2"
echo "=========================================="
echo ""

echo "Sending MA#1 cross up MA#2"
send_webhook "/webhook/ma/ma1-cross-up-ma2" "MA#1 Cross Up MA#2"
check_status
echo "✅ Expected: SHORT position closed, all indicators reset"
echo ""
sleep 2

# Test 5: Test backward compatibility with old EMA endpoints
echo "=========================================="
echo "TEST 5: Backward Compatibility Test"
echo "Testing old /webhook/ema/9-cross-* endpoints"
echo "=========================================="
echo ""

echo "Step 1: Send RSI above 50"
send_webhook "/webhook/rsi/above-50" "RSI Above 50"
check_status
echo ""

echo "Step 2: Send OLD endpoint /webhook/ema/9-cross-up-21"
send_webhook "/webhook/ema/9-cross-up-21" "EMA 9 Cross Up 21 (OLD ENDPOINT)"
check_status
echo "✅ Expected: LONG position opened (backward compatibility works)"
echo ""
sleep 2

echo "Step 3: Exit via OLD endpoint /webhook/ema/9-cross-down-21"
send_webhook "/webhook/ema/9-cross-down-21" "EMA 9 Cross Down 21 (OLD ENDPOINT)"
check_status
echo "✅ Expected: LONG position closed (old endpoint triggers exit)"
echo ""

# Test 6: Multiple symbol blocking
echo "=========================================="
echo "TEST 6: Multi-Symbol Blocking"
echo "Verify blocked symbol clears conditions"
echo "=========================================="
echo ""

SYMBOL2="AUD_USD"
echo "Opening position on ${SYMBOL}"
send_webhook "/webhook/rsi/above-50" "RSI Above 50 (${SYMBOL})"
send_webhook "/webhook/ma/ma1-cross-up-ma2" "MA#1 Cross Up MA#2 (${SYMBOL})"
check_status
echo ""
sleep 2

echo "Attempting to open on ${SYMBOL2} (should be blocked)"
SYMBOL="$SYMBOL2" send_webhook "/webhook/rsi/above-50" "RSI Above 50 (${SYMBOL2})"
SYMBOL="$SYMBOL2" send_webhook "/webhook/ma/ma1-cross-up-ma2" "MA#1 Cross Up MA#2 (${SYMBOL2})"
echo ""
echo "📊 Checking ${SYMBOL2} status:"
curl -s "${BASE_URL}/status" | jq ".positions.${SYMBOL2} | {position: .Position, open: .PositionOpen, entryConditions: .EntryConditionsCompleted}"
echo "✅ Expected: Position NOT opened, entry conditions CLEARED"
echo ""

# Cleanup
echo "=========================================="
echo "Cleanup: Closing open position"
echo "=========================================="
SYMBOL="EUR_USD"
send_webhook "/webhook/stochastic-rsi/cross-down-50" "Stochastic RSI Cross Down 50"
check_status

echo ""
echo "=========================================="
echo "Integration Tests Complete!"
echo "=========================================="
