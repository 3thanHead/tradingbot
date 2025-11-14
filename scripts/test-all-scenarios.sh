#!/bin/bash

# Comprehensive test for all scenarios from pseudo.txt
# Tests all 4 trading logic patterns

BASE_URL="http://localhost:8080"

echo "🧪 Complete Trading Logic Test (Based on pseudo.txt)"
echo "===================================================="
echo ""
echo "This test covers all 4 scenarios:"
echo "  1. RSI > 70 → RSI Moving Down → Open SHORT"
echo "  2. RSI < 30 → RSI Moving Up → Open LONG"
echo "  3. MACD Cross Up → MACD Moving Up → Close SHORT"
echo "  4. MACD Cross Down → MACD Moving Down → Close LONG"
echo ""

# Helper function to check status
check_status() {
    echo "📊 Current Status:"
    curl -s $BASE_URL/status | jq '.'
    echo ""
}

# Helper function to wait
wait_seconds() {
    echo "⏳ Waiting $1 seconds..."
    sleep $1
    echo ""
}

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "TEST SCENARIO 1: SHORT Position (RSI Overbought)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

echo "Step 1.1: Send RSI Crossed Up event (set RSICrossedUp flag)"
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
  }' | jq '.'
echo ""

wait_seconds 2

echo "Step 1.2: Send RSI Moving Down event (should open SHORT)"
curl -s -X POST $BASE_URL/webhook/rsi/moving-down \
  -H "Content-Type: application/json" \
  -d '{
    "ticker": "EUR_USD",
    "exchange": "OANDA",
    "close": "1.0845"
  }' | jq '.'
echo ""

wait_seconds 3

echo "Step 1.3: Verify SHORT position opened"
check_status

echo "Expected: position_open=true, position='short'"
echo ""

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "TEST SCENARIO 3: Close SHORT Position (MACD Signal)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

echo "Step 3.1: Send MACD Cross Up event (set MACDCrossedUp flag)"
curl -s -X POST $BASE_URL/webhook/macd/cross-up \
  -H "Content-Type: application/json" \
  -d '{
    "ticker": "EUR_USD",
    "exchange": "OANDA",
    "close": "1.0840"
  }' | jq '.'
echo ""

wait_seconds 2

echo "Step 3.2: Send MACD Moving Up event (should close SHORT)"
curl -s -X POST $BASE_URL/webhook/macd/moving-up \
  -H "Content-Type: application/json" \
  -d '{
    "ticker": "EUR_USD",
    "exchange": "OANDA",
    "close": "1.0835"
  }' | jq '.'
echo ""

wait_seconds 3

echo "Step 3.3: Verify SHORT position closed"
check_status

echo "Expected: position_open=false, position='none'"
echo ""

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "TEST SCENARIO 2: LONG Position (RSI Oversold)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

echo "Step 2.1: Send RSI Crossed Down event (set RSICrossedDown flag)"
curl -s -X POST $BASE_URL/webhook/rsi/crossed-down \
  -H "Content-Type: application/json" \
  -d '{
    "ticker": "EUR_USD",
    "exchange": "OANDA",
    "interval": "15",
    "close": "1.0780",
    "open": "1.0790",
    "high": "1.0795",
    "low": "1.0775",
    "volume": "1000",
    "time": "2025-11-12T15:00:00Z",
    "timenow": "2025-11-12T15:00:00Z"
  }' | jq '.'
echo ""

wait_seconds 2

echo "Step 2.2: Send RSI Moving Up event (should open LONG)"
curl -s -X POST $BASE_URL/webhook/rsi/moving-up \
  -H "Content-Type: application/json" \
  -d '{
    "ticker": "EUR_USD",
    "exchange": "OANDA",
    "close": "1.0785"
  }' | jq '.'
echo ""

wait_seconds 3

echo "Step 2.3: Verify LONG position opened"
check_status

echo "Expected: position_open=true, position='long'"
echo ""

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "TEST SCENARIO 4: Close LONG Position (MACD Signal)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

echo "Step 4.1: Send MACD Cross Down event (set MACDCrossedDown flag)"
curl -s -X POST $BASE_URL/webhook/macd/cross-down \
  -H "Content-Type: application/json" \
  -d '{
    "ticker": "EUR_USD",
    "exchange": "OANDA",
    "close": "1.0790"
  }' | jq '.'
echo ""

wait_seconds 2

echo "Step 4.2: Send MACD Moving Down event (should close LONG)"
curl -s -X POST $BASE_URL/webhook/macd/moving-down \
  -H "Content-Type: application/json" \
  -d '{
    "ticker": "EUR_USD",
    "exchange": "OANDA",
    "close": "1.0795"
  }' | jq '.'
echo ""

wait_seconds 3

echo "Step 4.3: Verify LONG position closed"
check_status

echo "Expected: position_open=false, position='none'"
echo ""

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "EDGE CASE TESTS"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

echo "Edge Case 1: RSI Moving Down WITHOUT RSI Crossed Up flag (should NOT open)"
curl -s -X POST $BASE_URL/webhook/rsi/moving-down \
  -H "Content-Type: application/json" \
  -d '{
    "ticker": "EUR_USD",
    "exchange": "OANDA",
    "close": "1.0800"
  }' | jq '.'
echo ""
wait_seconds 2
check_status
echo "Expected: No position opened (flag was not set)"
echo ""

echo "Edge Case 2: Set RSI Crossed Up, then try to open when position already exists"
echo "  2a: First open a SHORT position"
curl -s -X POST $BASE_URL/webhook/rsi/crossed-up \
  -H "Content-Type: application/json" \
  -d '{"ticker": "EUR_USD", "exchange": "OANDA", "close": "1.0850"}' | jq '.'
echo ""
wait_seconds 1

curl -s -X POST $BASE_URL/webhook/rsi/moving-down \
  -H "Content-Type: application/json" \
  -d '{"ticker": "EUR_USD", "exchange": "OANDA", "close": "1.0845"}' | jq '.'
echo ""
wait_seconds 2
check_status

echo "  2b: Try to open another position (should be SKIPPED)"
curl -s -X POST $BASE_URL/webhook/rsi/crossed-down \
  -H "Content-Type: application/json" \
  -d '{"ticker": "EUR_USD", "exchange": "OANDA", "close": "1.0820"}' | jq '.'
echo ""
wait_seconds 1

curl -s -X POST $BASE_URL/webhook/rsi/moving-up \
  -H "Content-Type: application/json" \
  -d '{"ticker": "EUR_USD", "exchange": "OANDA", "close": "1.0825"}' | jq '.'
echo ""
wait_seconds 2
check_status
echo "Expected: Still SHORT position (can't open LONG when SHORT is already open)"
echo ""

echo "  2c: Clean up - close the SHORT position"
curl -s -X POST $BASE_URL/webhook/macd/cross-up \
  -H "Content-Type: application/json" \
  -d '{"ticker": "EUR_USD", "exchange": "OANDA", "close": "1.0840"}' | jq '.'
echo ""
wait_seconds 1

curl -s -X POST $BASE_URL/webhook/macd/moving-up \
  -H "Content-Type: application/json" \
  -d '{"ticker": "EUR_USD", "exchange": "OANDA", "close": "1.0835"}' | jq '.'
echo ""
wait_seconds 2
check_status
echo ""

echo "Edge Case 3: Try to close position when none is open (should be SKIPPED)"
curl -s -X POST $BASE_URL/webhook/macd/cross-down \
  -H "Content-Type: application/json" \
  -d '{"ticker": "EUR_USD", "exchange": "OANDA", "close": "1.0840"}' | jq '.'
echo ""
wait_seconds 1

curl -s -X POST $BASE_URL/webhook/macd/moving-down \
  -H "Content-Type: application/json" \
  -d '{"ticker": "EUR_USD", "exchange": "OANDA", "close": "1.0835"}' | jq '.'
echo ""
wait_seconds 2
check_status
echo "Expected: No position (nothing to close)"
echo ""

echo "Edge Case 4: Try to close SHORT with LONG close signal (should be SKIPPED)"
echo "  4a: Open a SHORT position"
curl -s -X POST $BASE_URL/webhook/rsi/crossed-up \
  -H "Content-Type: application/json" \
  -d '{"ticker": "EUR_USD", "exchange": "OANDA", "close": "1.0850"}' | jq '.'
echo ""
wait_seconds 1

curl -s -X POST $BASE_URL/webhook/rsi/moving-down \
  -H "Content-Type: application/json" \
  -d '{"ticker": "EUR_USD", "exchange": "OANDA", "close": "1.0845"}' | jq '.'
echo ""
wait_seconds 2
check_status

echo "  4b: Try to close with MACD Moving Down (LONG close signal)"
curl -s -X POST $BASE_URL/webhook/macd/cross-down \
  -H "Content-Type: application/json" \
  -d '{"ticker": "EUR_USD", "exchange": "OANDA", "close": "1.0840"}' | jq '.'
echo ""
wait_seconds 1

curl -s -X POST $BASE_URL/webhook/macd/moving-down \
  -H "Content-Type: application/json" \
  -d '{"ticker": "EUR_USD", "exchange": "OANDA", "close": "1.0835"}' | jq '.'
echo ""
wait_seconds 2
check_status
echo "Expected: SHORT position still open (wrong close signal)"
echo ""

echo "  4c: Close with correct signal (MACD Moving Up for SHORT)"
curl -s -X POST $BASE_URL/webhook/macd/cross-up \
  -H "Content-Type: application/json" \
  -d '{"ticker": "EUR_USD", "exchange": "OANDA", "close": "1.0840"}' | jq '.'
echo ""
wait_seconds 1

curl -s -X POST $BASE_URL/webhook/macd/moving-up \
  -H "Content-Type: application/json" \
  -d '{"ticker": "EUR_USD", "exchange": "OANDA", "close": "1.0835"}' | jq '.'
echo ""
wait_seconds 2
check_status
echo "Expected: Position closed"
echo ""

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "✅ ALL TESTS COMPLETE!"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "📝 Review detailed logs:"
echo "   docker logs tradingview-webhook-bot | tail -100"
echo ""
echo "📊 Test Summary:"
echo "  ✓ Scenario 1: RSI > 70 → Moving Down → Open SHORT"
echo "  ✓ Scenario 2: RSI < 30 → Moving Up → Open LONG"
echo "  ✓ Scenario 3: MACD Cross Up → Moving Up → Close SHORT"
echo "  ✓ Scenario 4: MACD Cross Down → Moving Down → Close LONG"
echo "  ✓ Edge Case: No flag set = No action"
echo "  ✓ Edge Case: Position already open = Skip open"
echo "  ✓ Edge Case: No position = Skip close"
echo "  ✓ Edge Case: Wrong close signal = Skip close"
echo ""
