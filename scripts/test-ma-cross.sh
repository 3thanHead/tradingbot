#!/bin/bash

# Test MA Cross Endpoints
BASE_URL="http://localhost:8080"

echo "Testing MA#1 Cross Up MA#2 endpoint..."
curl -v -X POST "$BASE_URL/webhook/ma/ma1-cross-up-ma2" \
  -H "Content-Type: application/json" \
  -d '{
    "ticker": "BTC_USD",
    "close": "45000.00",
    "time": "2024-01-15T10:00:00Z",
    "exchange": "BINANCE"
  }'

echo ""
echo ""
echo "Testing MA#1 Cross Down MA#2 endpoint..."
curl -v -X POST "$BASE_URL/webhook/ma/ma1-cross-down-ma2" \
  -H "Content-Type: application/json" \
  -d '{
    "ticker": "BTC_USD",
    "close": "44950.00",
    "time": "2024-01-15T10:01:00Z",
    "exchange": "BINANCE"
  }'
