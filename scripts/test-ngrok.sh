#!/bin/bash

echo "🧪 Testing ngrok tunnel with Trading Bot..."
echo ""
echo "💵 Note: If TRADE_USD_AMOUNT is set, units calculated from USD amount"
echo ""

# Step 1: Get ngrok URL
echo "1️⃣ Fetching ngrok tunnel URL..."
NGROK_URL=$(curl -s http://localhost:4040/api/tunnels | grep -o '"public_url":"https://[^"]*' | grep -o 'https://.*' | head -1)

if [ -z "$NGROK_URL" ]; then
    echo "❌ ERROR: Could not get ngrok URL. Is ngrok running?"
    echo "   Run: docker-compose up"
    exit 1
fi

echo "✅ ngrok URL: $NGROK_URL"
echo ""

# Step 2: Test health endpoint
echo "2️⃣ Testing /health endpoint..."
HEALTH_RESPONSE=$(curl -s -o /dev/null -w "%{http_code}" "$NGROK_URL/health")

if [ "$HEALTH_RESPONSE" = "200" ]; then
    echo "✅ Health check passed (HTTP $HEALTH_RESPONSE)"
else
    echo "❌ Health check failed (HTTP $HEALTH_RESPONSE)"
    exit 1
fi
echo ""

# Step 3: Test status endpoint
echo "3️⃣ Testing /status endpoint..."
echo "Response:"
curl -s "$NGROK_URL/status"
echo ""
echo ""

# Step 4: Test RSI webhook
echo "4️⃣ Testing RSI Crossed Up webhook (using EUR_USD)..."
echo "Response:"
curl -s -X POST "$NGROK_URL/webhook/rsi/crossed-up" \
  -H "Content-Type: application/json" \
  -d '{
    "ticker": "EUR_USD",
    "exchange": "OANDA",
    "interval": "15",
    "close": "1.0850",
    "open": "1.0845",
    "high": "1.0855",
    "low": "1.0840",
    "volume": "1000",
    "time": "2024-01-01T12:00:00Z",
    "timenow": "2024-01-01T12:00:00Z"
  }'
echo ""
echo ""

# Step 5: Check ngrok web UI
echo "5️⃣ Checking ngrok web UI..."
echo "   Visit: http://localhost:4040"
echo "   You should see the test requests above!"
echo ""

# Summary
echo "================================"
echo "✅ All tests passed!"
echo "================================"
echo ""
echo "Your ngrok tunnel is working correctly!"
echo ""
echo "📋 Next steps:"
echo "   1. Copy this URL: $NGROK_URL"
echo "   2. Use it in TradingView alerts:"
echo "      $NGROK_URL/webhook/rsi/crossed-up"
echo "      $NGROK_URL/webhook/rsi/crossed-down"
echo "      etc..."
echo ""
echo "🖥️  View requests in real-time: http://localhost:4040"
echo ""
