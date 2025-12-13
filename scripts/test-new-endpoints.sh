#!/bin/bash

# Test script for new MA and RSI endpoints
BASE_URL="https://endoplasmic-semijuridically-fatimah.ngrok-free.dev"
SYMBOL="EUR_USD"

echo "========================================"
echo "Testing New Endpoint Handlers"
echo "========================================"
echo ""

# Test payload
PAYLOAD='{
  "ticker": "OANDA:EURUSD",
  "close": "1.0850",
  "time": "2024-01-15T10:30:00Z",
  "interval": "5"
}'

echo "1. Testing MA#1 Cross Up MA#2..."
curl -s -X POST "$BASE_URL/webhook/ma/ma1-cross-up-ma2" \
  -H "Content-Type: application/json" \
  -d "$PAYLOAD" | jq -r '.message'
echo ""

echo "2. Testing Price Above MA#2..."
curl -s -X POST "$BASE_URL/webhook/ma/price-above-ma2" \
  -H "Content-Type: application/json" \
  -d "$PAYLOAD" | jq -r '.message'
echo ""

echo "3. Testing RSI Cross Up 50..."
curl -s -X POST "$BASE_URL/webhook/rsi/cross-up-50" \
  -H "Content-Type: application/json" \
  -d "$PAYLOAD" | jq -r '.message'
echo ""

echo "4. Testing MACD Cross Up..."
curl -s -X POST "$BASE_URL/webhook/macd/cross-up" \
  -H "Content-Type: application/json" \
  -d "$PAYLOAD" | jq -r '.message'
echo ""

echo "========================================"
echo "All 4 LONG entry conditions should now be met!"
echo "Checking if LONG position opened..."
echo "========================================"
sleep 1

# Check logs to see if position opened
docker-compose -f /home/ethan/repos/trader_bot/docker-compose.yml logs trading-bot --tail=20 | grep -E "LONG|Strategy conditions met|entry conditions met"

echo ""
echo "========================================"
echo "Testing Exit Conditions"
echo "========================================"
echo ""

echo "5. Testing RSI Cross Down Overbuy..."
curl -s -X POST "$BASE_URL/webhook/rsi/cross-down-overbuy" \
  -H "Content-Type: application/json" \
  -d "$PAYLOAD" | jq -r '.message'
echo ""

echo "6. Testing Price Cross Down MA#2..."
curl -s -X POST "$BASE_URL/webhook/ma/price-cross-down-ma2" \
  -H "Content-Type: application/json" \
  -d "$PAYLOAD" | jq -r '.message'
echo ""

echo "========================================"
echo "Both exit conditions should now be met!"
echo "Checking if position closed..."
echo "========================================"
sleep 1

# Check logs to see if position closed
docker-compose -f /home/ethan/repos/trader_bot/docker-compose.yml logs trading-bot --tail=20 | grep -E "EXIT|closed|exit conditions"

echo ""
echo "========================================"
echo "Testing SHORT Entry"
echo "========================================"
echo ""

echo "7. Testing MA#1 Cross Down MA#2..."
curl -s -X POST "$BASE_URL/webhook/ma/ma1-cross-down-ma2" \
  -H "Content-Type: application/json" \
  -d "$PAYLOAD" | jq -r '.message'
echo ""

echo "8. Testing Price Below MA#2..."
curl -s -X POST "$BASE_URL/webhook/ma/price-below-ma2" \
  -H "Content-Type: application/json" \
  -d "$PAYLOAD" | jq -r '.message'
echo ""

echo "9. Testing RSI Cross Down 50..."
curl -s -X POST "$BASE_URL/webhook/rsi/cross-down-50" \
  -H "Content-Type: application/json" \
  -d "$PAYLOAD" | jq -r '.message'
echo ""

echo "10. Testing MACD Cross Down..."
curl -s -X POST "$BASE_URL/webhook/macd/cross-down" \
  -H "Content-Type: application/json" \
  -d "$PAYLOAD" | jq -r '.message'
echo ""

echo "========================================"
echo "All 4 SHORT entry conditions should now be met!"
echo "Checking if SHORT position opened..."
echo "========================================"
sleep 1

# Check logs to see if position opened
docker-compose -f /home/ethan/repos/trader_bot/docker-compose.yml logs trading-bot --tail=20 | grep -E "SHORT|Strategy conditions met|entry conditions met"

echo ""
echo "========================================"
echo "Testing SHORT Exit Conditions"
echo "========================================"
echo ""

echo "11. Testing RSI Cross Up Oversell..."
curl -s -X POST "$BASE_URL/webhook/rsi/cross-up-oversell" \
  -H "Content-Type: application/json" \
  -d "$PAYLOAD" | jq -r '.message'
echo ""

echo "12. Testing Price Cross Up MA#2..."
curl -s -X POST "$BASE_URL/webhook/ma/price-cross-up-ma2" \
  -H "Content-Type: application/json" \
  -d "$PAYLOAD" | jq -r '.message'
echo ""

echo "========================================"
echo "Both SHORT exit conditions should now be met!"
echo "Checking if position closed..."
echo "========================================"
sleep 1

# Check logs to see if position closed
docker-compose -f /home/ethan/repos/trader_bot/docker-compose.yml logs trading-bot --tail=20 | grep -E "EXIT|closed|exit conditions"

echo ""
echo "========================================"
echo "Test Complete!"
echo "========================================"
