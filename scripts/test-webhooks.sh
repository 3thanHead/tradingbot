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

# Test RSI overbought exit (LONG exit signal)
echo "2️⃣ Testing RSI Cross Down Overbuy 75 (LONG exit)..."
curl -s -X POST $BASE_URL/webhook/rsi/cross-down-overbuy-75 \
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

# Test SHORT entry signal
echo "3️⃣ Testing RSI Cross Down 40 (SHORT entry signal)..."
curl -s -X POST $BASE_URL/webhook/rsi/cross-down-40 \
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

# Test SHORT exit signal
echo "6️⃣ Testing RSI Cross Up Oversell 25 (SHORT exit signal)..."
curl -s -X POST $BASE_URL/webhook/rsi/cross-up-oversell-25 \
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
