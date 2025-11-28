#!/bin/bash

# Get the directory where this script is located
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Test data payload file
PAYLOAD_FILE="$SCRIPT_DIR/test-webhook-payload.json"

if [ ! -f "$PAYLOAD_FILE" ]; then
    echo "❌ Error: $PAYLOAD_FILE not found"
    exit 1
fi

echo "📋 Using test payload from: $PAYLOAD_FILE"
echo ""
cat $PAYLOAD_FILE
echo ""
echo ""

# Base URL (change to your ngrok URL for remote testing)
BASE_URL="${1:-http://localhost:8080}"

echo "🎯 Testing against: $BASE_URL"
echo "================================================"
echo ""

# Test Health
echo "1️⃣  Testing /health"
curl -s "$BASE_URL/health"
echo ""
echo ""

# Test Status
echo "2️⃣  Testing /status"
curl -s "$BASE_URL/status"
echo ""
echo ""

# RSI Webhooks (using actual endpoints from strategy)
echo "3️⃣  Testing RSI Cross Up 60 (LONG entry)"
curl -s -X POST "$BASE_URL/webhook/rsi/cross-up-60" \
  -H "Content-Type: application/json" \
  -d @$PAYLOAD_FILE
echo ""
echo ""

echo "4️⃣  Testing RSI Cross Down Overbuy 75 (LONG exit)"
curl -s -X POST "$BASE_URL/webhook/rsi/cross-down-overbuy-75" \
  -H "Content-Type: application/json" \
  -d @$PAYLOAD_FILE
echo ""
echo ""

echo "5️⃣  Testing RSI Cross Down 40 (SHORT entry)"
curl -s -X POST "$BASE_URL/webhook/rsi/cross-down-40" \
  -H "Content-Type: application/json" \
  -d @$PAYLOAD_FILE
echo ""
echo ""

echo "6️⃣  Testing RSI Cross Up Oversell 25 (SHORT exit)"
curl -s -X POST "$BASE_URL/webhook/rsi/cross-up-oversell-25" \
  -H "Content-Type: application/json" \
  -d @$PAYLOAD_FILE
echo ""
echo ""

# MACD Webhooks
echo "7️⃣  Testing MACD Cross Up"
curl -s -X POST "$BASE_URL/webhook/macd/cross-up" \
  -H "Content-Type: application/json" \
  -d @$PAYLOAD_FILE
echo ""
echo ""

echo "8️⃣  Testing MACD Cross Down"
curl -s -X POST "$BASE_URL/webhook/macd/cross-down" \
  -H "Content-Type: application/json" \
  -d @$PAYLOAD_FILE
echo ""
echo ""

# EMA Webhooks (strategy requirements)
echo "9️⃣  Testing Price Above EMA 200 (LONG entry requirement)"
curl -s -X POST "$BASE_URL/webhook/ema/price-above-ema200" \
  -H "Content-Type: application/json" \
  -d @$PAYLOAD_FILE
echo ""
echo ""

echo "🔟 Testing Price Below EMA 200 (SHORT entry requirement)"
curl -s -X POST "$BASE_URL/webhook/ema/price-below-ema200" \
  -H "Content-Type: application/json" \
  -d @$PAYLOAD_FILE
echo ""
echo ""

echo "================================================"
echo "✅ All tests complete!"
echo ""
echo "💡 Tips:"
echo "  - Test locally: ./test-with-payload.sh"
echo "  - Test with ngrok: ./test-with-payload.sh https://your-url.ngrok-free.app"
echo "  - Edit test-webhook-payload.json to change test data"
echo ""
echo "📋 Strategy Endpoints Tested:"
echo "  LONG Entry: price-above-ema200 + macd/cross-up + rsi/cross-up-60"
echo "  LONG Exit: macd/cross-down OR rsi/cross-down-overbuy-75"
echo "  SHORT Entry: price-below-ema200 + macd/cross-down + rsi/cross-down-40"
echo "  SHORT Exit: macd/cross-up OR rsi/cross-up-oversell-25"
echo ""
