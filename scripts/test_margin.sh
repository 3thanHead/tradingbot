#!/bin/bash

echo "🧪 Testing Margin Amount in OANDA"
echo "=================================="
echo ""
echo "📊 Current margin setting: \$5000"
echo "⚡ Expected leverage: 50:1"
echo "�� Expected position size: ~\$250,000"
echo ""

BASE_URL="http://localhost:8080"

echo "🔍 Step 1: Check current status..."
curl -s $BASE_URL/status
echo ""
echo ""

echo "📊 Step 2: Trigger LONG entry condition 1 - RSI above 50..."
curl -s -X POST $BASE_URL/webhook/rsi/above-50 \
  -H "Content-Type: application/json" \
  -d '{
    "ticker": "EUR_USD",
    "exchange": "OANDA",
    "close": "1.0850"
  }'
echo ""
echo ""

echo "⏳ Waiting 2 seconds..."
sleep 2

echo "📊 Step 3: Trigger LONG entry condition 2 - MA1 cross up MA2..."
echo "🎯 This should open a LONG position with margin-based sizing"
echo ""
curl -s -X POST $BASE_URL/webhook/ma/ma1-cross-up-ma2 \
  -H "Content-Type: application/json" \
  -d '{
    "ticker": "EUR_USD",
    "exchange": "OANDA",
    "close": "1.0855"
  }'
echo ""
echo ""

echo "⏳ Waiting 3 seconds for position to open..."
sleep 3

echo "📋 Step 4: Check final position status..."
curl -s $BASE_URL/status
echo ""
echo ""

echo "================================================"
echo "✅ Test complete!"
echo ""
echo "📊 Check the bot logs for margin calculation details:"
echo "   Look for lines like:"
echo "   💰 [MARGIN] Using margin amount: \$5000.00"
echo "   💱 [MARGIN CALC] Price: X.XXXXX, Leverage: 50:1"
echo "   💱 [MARGIN CALC] Margin per unit: \$X.XXXXXX"
echo "   💱 [POSITION SIZE] XXXXX units × \$X.XXXXX = \$XXXXXX.XX position"
echo ""
