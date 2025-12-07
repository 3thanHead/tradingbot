#!/bin/bash

# Test Internal Cross Detection
# Sends "above" and "below" position data - bot detects crosses

BASE_URL="http://localhost:8080"
SYMBOL="BTC_USD"

echo "=========================================="
echo "Internal Cross Detection Test"
echo "=========================================="
echo ""

# Test 0: Initialization - First Event Should NOT Trigger Cross
echo "📊 Test 0: Initialization (First webhook after startup)"
echo "Expected: State initialized, but NO CROSS detected"
echo ""

echo "Call 1: MA#1 above MA#2 (first event - initializing state)"
curl -s -X POST "$BASE_URL/webhook/ma/ma1-above-ma2" \
  -H "Content-Type: application/json" \
  -d '{
    "ticker": "BTC_USD",
    "close": "45000.00",
    "time": "2024-01-15T09:59:00Z",
    "exchange": "BINANCE"
  }' | jq -r '.message'

echo ""
echo "→ State initialized. Bot now knows MA#1 is above MA#2"
echo "→ No cross detected (no previous state to compare)"
echo ""
sleep 2

# Test 1: Bullish Cross Detection
echo "=========================================="
echo "📊 Test 1: Sending MA#1 ABOVE MA#2 sequence"
echo "Expected: First call = no cross, Second call = CROSS UP detected"
echo ""

echo "Call 1: MA#1 above MA#2 (initial state)"
curl -s -X POST "$BASE_URL/webhook/ma/ma1-above-ma2" \
  -H "Content-Type: application/json" \
  -d '{
    "ticker": "BTC_USD",
    "close": "45000.00",
    "time": "2024-01-15T10:00:00Z",
    "exchange": "BINANCE"
  }' | jq -r '.message'

echo ""
sleep 1

echo "Call 2: MA#1 below MA#2 (state change)"
curl -s -X POST "$BASE_URL/webhook/ma/ma1-below-ma2" \
  -H "Content-Type: application/json" \
  -d '{
    "ticker": "BTC_USD",
    "close": "44950.00",
    "time": "2024-01-15T10:01:00Z",
    "exchange": "BINANCE"
  }' | jq -r '.message'

echo ""
sleep 1

echo "Call 3: MA#1 above MA#2 again (CROSS UP should trigger)"
curl -s -X POST "$BASE_URL/webhook/ma/ma1-above-ma2" \
  -H "Content-Type: application/json" \
  -d '{
    "ticker": "BTC_USD",
    "close": "45100.00",
    "time": "2024-01-15T10:02:00Z",
    "exchange": "BINANCE"
  }' | jq -r '.message'

echo ""
echo "=========================================="
sleep 2

# Test 2: Bearish Cross Detection
echo "📊 Test 2: Sending MA#1 BELOW MA#2 sequence"
echo "Expected: Cross DOWN detected"
echo ""

echo "Call 1: MA#1 above MA#2"
curl -s -X POST "$BASE_URL/webhook/ma/ma1-above-ma2" \
  -H "Content-Type: application/json" \
  -d '{
    "ticker": "ETH_USD",
    "close": "2400.00",
    "time": "2024-01-15T10:05:00Z",
    "exchange": "BINANCE"
  }' | jq -r '.message'

echo ""
sleep 1

echo "Call 2: MA#1 below MA#2 (CROSS DOWN should trigger)"
curl -s -X POST "$BASE_URL/webhook/ma/ma1-below-ma2" \
  -H "Content-Type: application/json" \
  -d '{
    "ticker": "ETH_USD",
    "close": "2380.00",
    "time": "2024-01-15T10:06:00Z",
    "exchange": "BINANCE"
  }' | jq -r '.message'

echo ""
echo "=========================================="
sleep 2

# Test 3: No Cross - Same State Repeated
echo "📊 Test 3: Sending same state repeatedly (no cross)"
echo "Expected: No cross detected"
echo ""

echo "Call 1: MA#1 above MA#2"
curl -s -X POST "$BASE_URL/webhook/ma/ma1-above-ma2" \
  -H "Content-Type: application/json" \
  -d '{
    "ticker": "SOL_USD",
    "close": "100.00",
    "time": "2024-01-15T10:10:00Z",
    "exchange": "BINANCE"
  }' | jq -r '.message'

echo ""
sleep 1

echo "Call 2: MA#1 above MA#2 (same state, no cross)"
curl -s -X POST "$BASE_URL/webhook/ma/ma1-above-ma2" \
  -H "Content-Type: application/json" \
  -d '{
    "ticker": "SOL_USD",
    "close": "100.50",
    "time": "2024-01-15T10:11:00Z",
    "exchange": "BINANCE"
  }' | jq -r '.message'

echo ""
sleep 1

echo "Call 3: MA#1 above MA#2 (still no cross)"
curl -s -X POST "$BASE_URL/webhook/ma/ma1-above-ma2" \
  -H "Content-Type: application/json" \
  -d '{
    "ticker": "SOL_USD",
    "close": "101.00",
    "time": "2024-01-15T10:12:00Z",
    "exchange": "BINANCE"
  }' | jq -r '.message'

echo ""
echo "=========================================="
echo "✅ Cross Detection Test Complete"
echo ""
echo "Summary:"
echo "  • First webhook initializes state (no cross detected)"
echo "  • TradingView sends position data (above/below) every candle"
echo "  • Bot tracks state changes internally"
echo "  • Cross detected only when state transitions"
echo "  • Prevents false crosses on bot startup"
echo "  • More reliable than depending on TradingView cross detection"
echo "=========================================="
