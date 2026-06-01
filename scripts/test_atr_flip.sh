#!/bin/bash

WEBHOOK_URL="http://localhost:8080"

echo "======================================"
echo "Testing ATR Flip Strategy"
echo "======================================"
echo ""

echo "📊 Checking initial status..."
curl -s "$WEBHOOK_URL/status" | jq '.'
echo ""

sleep 2

echo "======================================"
echo "TEST 1: LONG Position Lifecycle"
echo "======================================"
echo ""

echo "1️⃣ Sending price-above-ma4 (trend filter)..."
curl -X POST "$WEBHOOK_URL/webhook/ma/price-above-ma4" \
  -H "Content-Type: application/json" \
  -d '{"ticker":"EUR_USD","close":"1.09500"}'
echo ""
sleep 3

echo "2️⃣ Sending atr/long (should OPEN LONG position)..."
curl -X POST "$WEBHOOK_URL/webhook/atr/long" \
  -H "Content-Type: application/json" \
  -d '{"ticker":"EUR_USD","close":"1.09500"}'
echo ""
sleep 3

echo "📊 Checking status (should show LONG position)..."
curl -s "$WEBHOOK_URL/status" | jq '.'
echo ""
sleep 2

echo "3️⃣ Sending atr/short (should CLOSE LONG position)..."
curl -X POST "$WEBHOOK_URL/webhook/atr/short" \
  -H "Content-Type: application/json" \
  -d '{"ticker":"EUR_USD","close":"1.09400"}'
echo ""
sleep 3

echo "📊 Checking status (should show NO position)..."
curl -s "$WEBHOOK_URL/status" | jq '.'
echo ""

sleep 2

echo "======================================"
echo "TEST 2: SHORT Position Lifecycle"
echo "======================================"
echo ""

echo "1️⃣ Sending price-below-ma4 (trend filter)..."
curl -X POST "$WEBHOOK_URL/webhook/ma/price-below-ma4" \
  -H "Content-Type: application/json" \
  -d '{"ticker":"EUR_USD","close":"1.09000"}'
echo ""
sleep 3

echo "2️⃣ Sending atr/short (should OPEN SHORT position)..."
curl -X POST "$WEBHOOK_URL/webhook/atr/short" \
  -H "Content-Type: application/json" \
  -d '{"ticker":"EUR_USD","close":"1.09000"}'
echo ""
sleep 3

echo "📊 Checking status (should show SHORT position)..."
curl -s "$WEBHOOK_URL/status" | jq '.'
echo ""
sleep 2

echo "3️⃣ Sending atr/long (should CLOSE SHORT position)..."
curl -X POST "$WEBHOOK_URL/webhook/atr/long" \
  -H "Content-Type: application/json" \
  -d '{"ticker":"EUR_USD","close":"1.09100"}'
echo ""
sleep 3

echo "📊 Final status (should show NO position)..."
curl -s "$WEBHOOK_URL/status" | jq '.'
echo ""

echo "======================================"
echo "✅ Test Complete"
echo "======================================"
echo "Check docker logs for detailed output:"
echo "  docker logs tradingview-webhook-bot --tail 50"
echo "=========================================="
