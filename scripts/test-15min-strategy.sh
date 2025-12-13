#!/bin/bash

# Test script for 15min MACD + EMA + Stochastic RSI strategy
# Tests LONG entry, LONG exit, SHORT entry, SHORT exit scenarios

BASE_URL="http://localhost:8080"
SYMBOL="AUD_USD"
EXCHANGE="BINANCE"  # Use non-OANDA exchange for simulated trades in testing

echo "=========================================="
echo "15min Strategy Test Suite"
echo "=========================================="
echo ""

# Helper function to send webhook
send_webhook() {
    local path=$1
    local name=$2
    echo "📤 Sending: $name"
    curl -s -X POST "${BASE_URL}${path}" \
        -H "Content-Type: application/json" \
        -d "{\"ticker\":\"${SYMBOL}\",\"exchange\":\"${EXCHANGE}\",\"interval\":\"15\",\"close\":\"1.0850\"}" | jq -r '.message'
    sleep 1
}

# Test 1: LONG Entry Scenario
echo "=========================================="
echo "TEST 1: LONG Entry (all 3 conditions)"
echo "=========================================="
send_webhook "/webhook/macd/cross-up" "MACD Cross Up"
send_webhook "/webhook/ema/9-cross-up-21" "EMA 9 Cross Up 21"
send_webhook "/webhook/stochastic-rsi/cross-up-20" "Stochastic RSI Cross Up 20"
echo ""
echo "✅ Expected: LONG position opened"
echo "Checking status..."
sleep 2
curl -s "${BASE_URL}/status" | jq '.positions.AUD_USD | {position: .Position, open: .PositionOpen}'
echo ""

# Test 2: LONG Exit via EMA
echo "=========================================="
echo "TEST 2: LONG Exit via EMA (first signal)"
echo "=========================================="
send_webhook "/webhook/ema/9-cross-down-21" "EMA 9 Cross Down 21"
echo ""
echo "✅ Expected: LONG position closed"
echo "Checking status..."
sleep 2
curl -s "${BASE_URL}/status" | jq '.positions.AUD_USD | {position: .Position, open: .PositionOpen}'
echo ""

# Test 3: SHORT Entry Scenario
echo "=========================================="
echo "TEST 3: SHORT Entry (all 3 conditions)"
echo "=========================================="
send_webhook "/webhook/macd/cross-down" "MACD Cross Down"
send_webhook "/webhook/ema/9-cross-down-21" "EMA 9 Cross Down 21"
send_webhook "/webhook/stochastic-rsi/cross-down-80" "Stochastic RSI Cross Down 80"
echo ""
echo "✅ Expected: SHORT position opened"
echo "Checking status..."
sleep 2
curl -s "${BASE_URL}/status" | jq '.positions.AUD_USD | {position: .Position, open: .PositionOpen}'
echo ""

# Test 4: SHORT Exit via MACD
echo "=========================================="
echo "TEST 4: SHORT Exit via MACD reversal"
echo "=========================================="
send_webhook "/webhook/macd/cross-up" "MACD Cross Up"
echo ""
echo "✅ Expected: SHORT position closed"
echo "Checking status..."
sleep 2
curl -s "${BASE_URL}/status" | jq '.positions.AUD_USD | {position: .Position, open: .PositionOpen}'
echo ""

# Test 5: Stochastic RSI Window Test
echo "=========================================="
echo "TEST 5: Stochastic RSI Window Persistence"
echo "=========================================="
echo "Testing that cross-up-20 persists until cross-down-80"
send_webhook "/webhook/stochastic-rsi/cross-up-20" "Stochastic RSI Cross Up 20"
send_webhook "/webhook/macd/cross-up" "MACD Cross Up"
echo "Waiting 3 seconds..."
sleep 3
send_webhook "/webhook/ema/9-cross-up-21" "EMA 9 Cross Up 21 (delayed)"
echo ""
echo "✅ Expected: LONG position opened (Stoch RSI condition persisted)"
echo "Checking status..."
sleep 2
curl -s "${BASE_URL}/status" | jq '.positions.AUD_USD | {position: .Position, open: .PositionOpen}'
echo ""

# Test 6: Close and check re-entry
echo "=========================================="
echo "TEST 6: Exit and verify no position"
echo "=========================================="
send_webhook "/webhook/macd/cross-down" "MACD Cross Down"
echo ""
echo "✅ Expected: Position closed, no immediate re-entry"
sleep 2
curl -s "${BASE_URL}/status" | jq '.positions.AUD_USD | {position: .Position, open: .PositionOpen}'
echo ""

# Test 7: Partial conditions (should NOT open)
echo "=========================================="
echo "TEST 7: Partial Conditions (no position)"
echo "=========================================="
send_webhook "/webhook/macd/cross-up" "MACD Cross Up"
send_webhook "/webhook/ema/9-cross-up-21" "EMA 9 Cross Up 21"
echo "Missing: Stochastic RSI Cross Up 20"
echo ""
echo "✅ Expected: NO position opened (missing 1/3 conditions)"
sleep 2
curl -s "${BASE_URL}/status" | jq '.positions.AUD_USD | {position: .Position, open: .PositionOpen}'
echo ""

echo "=========================================="
echo "Test Suite Complete"
echo "=========================================="
