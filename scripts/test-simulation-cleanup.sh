#!/bin/bash

# Test script to verify condition cleanup works correctly with simulated trades
# Tests that conditions are properly cleared when positions open/close

BASE_URL="http://localhost:8080"
SYMBOL1="EUR_USD"
SYMBOL2="AUD_USD"
EXCHANGE="BINANCE"  # Non-OANDA exchange triggers simulation

echo "=========================================="
echo "Simulation Condition Cleanup Test"
echo "=========================================="
echo "Testing:"
echo "  1. Simulated position open clears conditions"
echo "  2. Position blocks other symbols and clears their conditions"
echo "  3. Position close clears all symbol conditions"
echo "=========================================="
echo ""

# Helper function to send webhook
send_webhook() {
    local path=$1
    local symbol=$2
    local name=$3
    echo "📤 [$symbol] $name"
    curl -s -X POST "${BASE_URL}${path}" \
        -H "Content-Type: application/json" \
        -d "{\"ticker\":\"${symbol}\",\"exchange\":\"${EXCHANGE}\",\"interval\":\"15\",\"close\":\"1.0850\"}" | jq -r '.message'
    sleep 1
}

# Helper to check detailed status
check_detailed_status() {
    local symbol=$1
    echo ""
    echo "📊 Status for $symbol:"
    curl -s "${BASE_URL}/status" | jq ".positions.${symbol} | {
        position: .Position,
        open: .PositionOpen,
        tradeID: .TradeID,
        isSimulated: .IsSimulated,
        RSIAbove50: .RSIAbove50,
        RSIBelow50: .RSIBelow50,
        EMA9CrossedUpEMA21: .EMA9CrossedUpEMA21,
        EMA9CrossedDownEMA21: .EMA9CrossedDownEMA21,
        entryConditionsCount: (.EntryConditionsCompleted | length)
    }"
    echo ""
}

echo "=========================================="
echo "TEST 1: Open LONG on ${SYMBOL1}"
echo "=========================================="
echo "Setting up entry conditions for LONG..."
send_webhook "/webhook/rsi/above-50" "$SYMBOL1" "RSI Above 50"
check_detailed_status "$SYMBOL1"
echo "✅ Expected: RSIAbove50=true, 1 condition tracked, no position"
echo ""

send_webhook "/webhook/ma/ma1-cross-up-ma2" "$SYMBOL1" "MA#1 Cross Up MA#2"
check_detailed_status "$SYMBOL1"
echo "✅ Expected: LONG position opened (simulated), all indicators reset, entry conditions cleared"
echo ""

echo "=========================================="
echo "TEST 2: Try to open on ${SYMBOL2} (should block)"
echo "=========================================="
echo "Setting up entry conditions for ${SYMBOL2}..."
send_webhook "/webhook/rsi/above-50" "$SYMBOL2" "RSI Above 50"
send_webhook "/webhook/ma/ma1-cross-up-ma2" "$SYMBOL2" "MA#1 Cross Up MA#2"
check_detailed_status "$SYMBOL2"
echo "✅ Expected: Position NOT opened, entry conditions CLEARED (blocked message in logs)"
echo ""

echo "Verifying ${SYMBOL1} still has open position:"
check_detailed_status "$SYMBOL1"
echo "✅ Expected: ${SYMBOL1} still shows LONG position open"
echo ""

echo "=========================================="
echo "TEST 3: Close position on ${SYMBOL1}"
echo "=========================================="
echo "Sending exit signal (Stochastic RSI cross down 50)..."
send_webhook "/webhook/stochastic-rsi/cross-down-50" "$SYMBOL1" "Stochastic RSI Cross Down 50"
check_detailed_status "$SYMBOL1"
echo "✅ Expected: Position closed, all indicators reset, entry conditions cleared"
echo ""

echo "Verifying ${SYMBOL2} conditions also cleared:"
check_detailed_status "$SYMBOL2"
echo "✅ Expected: ${SYMBOL2} entry conditions also cleared (entryConditionsCount=0)"
echo ""

echo "=========================================="
echo "TEST 4: Verify fresh signals needed"
echo "=========================================="
echo "Both symbols should now require fresh signals..."
curl -s "${BASE_URL}/status" | jq ".positions | to_entries[] | {
    symbol: .key,
    position: .value.Position,
    open: .value.PositionOpen,
    longConditions: (.value.EntryConditionsCompleted | length),
    RSIAbove50: .value.RSIAbove50,
    EMA9CrossedUpEMA21: .value.EMA9CrossedUpEMA21
}"
echo ""
echo "✅ Expected: Both symbols show 0 conditions, all indicators false"
echo ""

echo "=========================================="
echo "TEST 5: Fresh entry after cleanup"
echo "=========================================="
echo "Sending fresh signals to ${SYMBOL2}..."
send_webhook "/webhook/rsi/below-50" "$SYMBOL2" "RSI Below 50"
check_detailed_status "$SYMBOL2"
echo "✅ Expected: RSIBelow50=true, 1 condition tracked"
echo ""

send_webhook "/webhook/ma/ma1-cross-down-ma2" "$SYMBOL2" "MA#1 Cross Down MA#2"
check_detailed_status "$SYMBOL2"
echo "✅ Expected: SHORT position opened on ${SYMBOL2}, indicators reset"
echo ""

echo "Cleanup: Closing ${SYMBOL2} position..."
send_webhook "/webhook/stochastic-rsi/cross-up-50" "$SYMBOL2" "Stochastic RSI Cross Up 50"
sleep 1

echo ""
echo "=========================================="
echo "Final Status - All positions closed:"
echo "=========================================="
curl -s "${BASE_URL}/status" | jq ".positions | to_entries[] | select(.value.Exchange == \"BINANCE\") | {
    symbol: .key,
    position: .value.Position,
    open: .value.PositionOpen,
    entryConditions: (.value.EntryConditionsCompleted | length),
    indicators: {
        RSIAbove50: .value.RSIAbove50,
        RSIBelow50: .value.RSIBelow50,
        EMA9Up: .value.EMA9CrossedUpEMA21,
        EMA9Down: .value.EMA9CrossedDownEMA21
    }
}"

echo ""
echo "=========================================="
echo "Test Complete!"
echo "=========================================="
echo "Review logs: tail -100 /tmp/bot_test.log | grep -E 'BLOCKED|CLEANUP|SIMULATED'"
echo "=========================================="
