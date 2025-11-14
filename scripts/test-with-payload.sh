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

# RSI Webhooks
echo "3️⃣  Testing RSI Crossed Up"
curl -s -X POST "$BASE_URL/webhook/rsi/crossed-up" \
  -H "Content-Type: application/json" \
  -d @$PAYLOAD_FILE
echo ""
echo ""

echo "4️⃣  Testing RSI Crossed Down"
curl -s -X POST "$BASE_URL/webhook/rsi/crossed-down" \
  -H "Content-Type: application/json" \
  -d @$PAYLOAD_FILE
echo ""
echo ""

echo "5️⃣  Testing RSI Moving Down (attempts SHORT if flag set)"
curl -s -X POST "$BASE_URL/webhook/rsi/moving-down" \
  -H "Content-Type: application/json" \
  -d @$PAYLOAD_FILE
echo ""
echo ""

echo "6️⃣  Testing RSI Moving Up (attempts LONG if flag set)"
curl -s -X POST "$BASE_URL/webhook/rsi/moving-up" \
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

echo "9️⃣  Testing MACD Moving Up (attempts to close SHORT if flag set)"
curl -s -X POST "$BASE_URL/webhook/macd/moving-up" \
  -H "Content-Type: application/json" \
  -d @$PAYLOAD_FILE
echo ""
echo ""

echo "🔟 Testing MACD Moving Down (attempts to close LONG if flag set)"
curl -s -X POST "$BASE_URL/webhook/macd/moving-down" \
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
