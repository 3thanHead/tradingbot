#!/bin/bash

# Test all webhook endpoints

BASE_URL="http://localhost:8080"

echo "🧪 Testing TradingView Webhook Trading Bot"
echo "=========================================="
echo ""
echo "💡 Note: Using EUR_USD ticker (OANDA format)"
echo "💵 If TRADE_USD_AMOUNT is set, units will be calculated automatically"
echo ""

# Test health
echo "1️⃣ Testing health endpoint..."
curl -s $BASE_URL/health
echo ""
echo ""

# Simulate RSI > 70 scenario
echo "2️⃣ Simulating RSI Crossed Up (Overbought)..."
curl -s -X POST $BASE_URL/webhook/rsi/crossed-up \
  -H "Content-Type: application/json" \
  -d '{
    "ticker": "EUR_USD",
    "exchange": "OANDA",
    "interval": "15",
    "close": "1.0850",
    "open": "1.0840",
    "high": "1.0855",
    "low": "1.0835",
    "volume": "1000",
    "time": "2025-11-12T14:00:00Z",
    "timenow": "2025-11-12T14:00:00Z"
  }'
echo ""
echo ""

# Simulate RSI moving down (should trigger SHORT)
echo "3️⃣ Simulating RSI Moving Down (Should trigger SHORT)..."
curl -s -X POST $BASE_URL/webhook/rsi/moving-down \
  -H "Content-Type: application/json" \
  -d '{
    "ticker": "EUR_USD",
    "exchange": "OANDA",
    "close": "1.0845"
  }'
echo ""
echo ""

# Check status
echo "4️⃣ Checking position status..."
curl -s $BASE_URL/status
echo ""
echo ""

# Simulate MACD cross up
echo "5️⃣ Simulating MACD Cross Up..."
curl -s -X POST $BASE_URL/webhook/macd/cross-up \
  -H "Content-Type: application/json" \
  -d '{
    "ticker": "EUR_USD",
    "exchange": "OANDA",
    "close": "1.0840"
  }'
echo ""
echo ""

# Simulate MACD moving up (should close SHORT)
echo "6️⃣ Simulating MACD Moving Up (Should close SHORT)..."
curl -s -X POST $BASE_URL/webhook/macd/moving-up \
  -H "Content-Type: application/json" \
  -d '{
    "ticker": "EUR_USD",
    "exchange": "OANDA",
    "close": "1.0835"
  }'
echo ""
echo ""

# Final status
echo "7️⃣ Final status..."
curl -s $BASE_URL/status
echo ""
echo ""

echo "✅ Test complete!"
echo ""
echo "📝 Check the logs for USD amount calculations:"
echo "   docker logs tradingview-webhook-bot"
