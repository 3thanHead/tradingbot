#!/bin/basecho "📊 Step 1: Check current status..."
echo "Response:"
curl -s $BASE_URL/status
echo ""
echo ""

echo "📈 Step 2: Trigger RSI Crossed Up condition..."
curl -s -X POST $BASE_URL/webhook/rsi/crossed-up \"💵 Testing USD Amount Position Opening"
echo "========================================"
echo ""
echo "� This test will use whatever TRADE_USD_AMOUNT is configured in the running bot"
echo ""

BASE_URL="http://localhost:8080"

echo "🔍 Step 1: Check current status..."
echo "Response:"
curl -s $BASE_URL/status
echo ""
echo ""

echo "📊 Step 2: Trigger RSI > 70 condition..."
curl -s -X POST $BASE_URL/webhook/rsi/greater-than-70 \
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

echo "⏳ Waiting 2 seconds..."
sleep 2
echo ""

echo "🔴 Step 3: Trigger SHORT position (RSI Moving Down)..."
echo ""
echo "🎯 This will:"
echo "   1. Get current EUR_USD price from OANDA"
echo "   2. Calculate units: USD Amount ÷ Current Price"
echo "   3. Open SHORT position with calculated units"
echo ""

curl -s -X POST $BASE_URL/webhook/rsi/moving-down \
  -H "Content-Type: application/json" \
  -d '{
    "ticker": "EUR_USD",
    "exchange": "OANDA",
    "close": "1.0845"
  }'
echo ""
echo ""

echo "⏳ Waiting 3 seconds for position to open..."
sleep 3
echo ""

echo "📋 Step 4: Check position status..."
echo "Response:"
curl -s $BASE_URL/status
echo ""
echo ""

echo "================================================"
echo "✅ Test complete!"
echo ""
echo "📊 View detailed logs (including USD conversion):"
echo "   docker logs tradingview-webhook-bot | tail -50"
echo ""
echo "💡 Look for these log entries:"
echo "   💱 [CALCULATE] Converting \$XXX.XX USD to units for EUR_USD"
echo "   📊 [PRICE] Current price for EUR_USD: X.XXXXX"
echo "   ✅ [CALCULATE] \$XXX.XX USD = XXX units at price X.XXXXX"
echo ""
echo "🧹 To close the position:"
echo "   curl -X POST http://localhost:8080/webhook/macd/cross-up -H 'Content-Type: application/json' -d '{\"ticker\":\"EUR_USD\",\"exchange\":\"OANDA\"}'"
echo "   curl -X POST http://localhost:8080/webhook/macd/moving-up -H 'Content-Type: application/json' -d '{\"ticker\":\"EUR_USD\",\"exchange\":\"OANDA\"}'"
echo ""
