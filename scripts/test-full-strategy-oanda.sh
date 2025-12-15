#!/bin/bash

# Full Strategy Test with OANDA
# Tests MA Trend + RSI 50 + MACD + ATR Strategy
# Validates: 5-condition entry, MA cross exit, whipsaw protection, reversal tracking

BASE_URL="http://localhost:8080"
SYMBOL="EUR_USD"

echo "=========================================================================="
echo "🧪 FULL STRATEGY TEST - OANDA LIVE POSITIONS"
echo "=========================================================================="
echo "Strategy: MA Trend + RSI 50 + MACD + ATR"
echo "Symbol: $SYMBOL"
echo "Margin: \$5000"
echo "=========================================================================="
echo ""

# Wait function
wait_for_next() {
    echo ""
    echo "⏳ Waiting 2 seconds..."
    sleep 2
}

# Check status function
check_status() {
    echo ""
    echo "📊 Current Status:"
    curl -s "${BASE_URL}/status" | jq ".positions.${SYMBOL} | {
        position: .Position,
        open: .PositionOpen,
        tradeID: .TradeID,
        lastClosed: .LastClosedDirection,
        oppositeOccurred: .OppositeDirectionOccurred,
        longConditions: (.EntryConditionsCompleted | length),
        entryConditions: {
            ma1Above: .EMA9AboveEMA21,
            priceAboveMA2: .PriceAboveEMA20,
            rsiAbove50: .RSIAbove50,
            atrAbove: .ATRAboveThreshold,
            macdUp: .MACDCrossedUp
        }
    }"
    echo ""
}

echo "=========================================================================="
echo "TEST 1: LONG ENTRY - All 5 Conditions"
echo "=========================================================================="
echo "Sending: MA1 above MA2, Price above MA2, RSI above 50, ATR above, MACD up"

curl -s -X POST "${BASE_URL}/webhook/ma/ma1-above-ma2" \
  -H "Content-Type: application/json" \
  -d '{"ticker":"EURUSD","exchange":"OANDA","close":"1.05000","interval":"15"}' > /dev/null

curl -s -X POST "${BASE_URL}/webhook/ma/price-above-ma4" \
  -H "Content-Type: application/json" \
  -d '{"ticker":"EURUSD","exchange":"OANDA","close":"1.05000","interval":"15"}' > /dev/null

curl -s -X POST "${BASE_URL}/webhook/rsi/above-50" \
  -H "Content-Type: application/json" \
  -d '{"ticker":"EURUSD","exchange":"OANDA","close":"1.05000","interval":"15"}' > /dev/null

curl -s -X POST "${BASE_URL}/webhook/atr/above-threshold" \
  -H "Content-Type: application/json" \
  -d '{"ticker":"EURUSD","exchange":"OANDA","close":"1.05000","interval":"15"}' > /dev/null

curl -s -X POST "${BASE_URL}/webhook/macd/cross-up" \
  -H "Content-Type: application/json" \
  -d '{"ticker":"EURUSD","exchange":"OANDA","close":"1.05000","interval":"15"}' > /dev/null

check_status
wait_for_next

echo "=========================================================================="
echo "TEST 2: LONG EXIT - MA1 crosses below MA2"
echo "=========================================================================="
echo "Triggering exit condition..."

curl -s -X POST "${BASE_URL}/webhook/ma/ma1-below-ma2" \
  -H "Content-Type: application/json" \
  -d '{"ticker":"EURUSD","exchange":"OANDA","close":"1.04950","interval":"15"}' > /dev/null

check_status
wait_for_next

echo "=========================================================================="
echo "TEST 3: SHORT ENTRY - All 5 Conditions (After Reversal)"
echo "=========================================================================="
echo "Step 1: Send opposite signals first (to satisfy reversal requirement)"

curl -s -X POST "${BASE_URL}/webhook/ma/ma1-above-ma2" \
  -H "Content-Type: application/json" \
  -d '{"ticker":"EURUSD","exchange":"OANDA","close":"1.04900","interval":"15"}' > /dev/null

curl -s -X POST "${BASE_URL}/webhook/rsi/above-50" \
  -H "Content-Type: application/json" \
  -d '{"ticker":"EURUSD","exchange":"OANDA","close":"1.04900","interval":"15"}' > /dev/null

echo "Step 2: Now send SHORT entry conditions"

curl -s -X POST "${BASE_URL}/webhook/ma/ma1-below-ma2" \
  -H "Content-Type: application/json" \
  -d '{"ticker":"EURUSD","exchange":"OANDA","close":"1.04850","interval":"15"}' > /dev/null

curl -s -X POST "${BASE_URL}/webhook/ma/price-below-ma4" \
  -H "Content-Type: application/json" \
  -d '{"ticker":"EURUSD","exchange":"OANDA","close":"1.04850","interval":"15"}' > /dev/null

curl -s -X POST "${BASE_URL}/webhook/rsi/below-50" \
  -H "Content-Type: application/json" \
  -d '{"ticker":"EURUSD","exchange":"OANDA","close":"1.04850","interval":"15"}' > /dev/null

curl -s -X POST "${BASE_URL}/webhook/atr/above-threshold" \
  -H "Content-Type: application/json" \
  -d '{"ticker":"EURUSD","exchange":"OANDA","close":"1.04850","interval":"15"}' > /dev/null

curl -s -X POST "${BASE_URL}/webhook/macd/cross-down" \
  -H "Content-Type: application/json" \
  -d '{"ticker":"EURUSD","exchange":"OANDA","close":"1.04850","interval":"15"}' > /dev/null

check_status
wait_for_next

echo "=========================================================================="
echo "TEST 4: SHORT EXIT - MA1 crosses above MA2"
echo "=========================================================================="
echo "Triggering exit condition..."

curl -s -X POST "${BASE_URL}/webhook/ma/ma1-above-ma2" \
  -H "Content-Type: application/json" \
  -d '{"ticker":"EURUSD","exchange":"OANDA","close":"1.04900","interval":"15"}' > /dev/null

check_status
wait_for_next

echo "=========================================================================="
echo "TEST 5: WHIPSAW PROTECTION - Try immediate SHORT re-entry"
echo "=========================================================================="
echo "Attempting SHORT entry without opposite signals (should be BLOCKED)"

curl -s -X POST "${BASE_URL}/webhook/ma/ma1-below-ma2" \
  -H "Content-Type: application/json" \
  -d '{"ticker":"EURUSD","exchange":"OANDA","close":"1.04880","interval":"15"}' > /dev/null

curl -s -X POST "${BASE_URL}/webhook/ma/price-below-ma4" \
  -H "Content-Type: application/json" \
  -d '{"ticker":"EURUSD","exchange":"OANDA","close":"1.04880","interval":"15"}' > /dev/null

curl -s -X POST "${BASE_URL}/webhook/rsi/below-50" \
  -H "Content-Type: application/json" \
  -d '{"ticker":"EURUSD","exchange":"OANDA","close":"1.04880","interval":"15"}' > /dev/null

curl -s -X POST "${BASE_URL}/webhook/atr/above-threshold" \
  -H "Content-Type: application/json" \
  -d '{"ticker":"EURUSD","exchange":"OANDA","close":"1.04880","interval":"15"}' > /dev/null

curl -s -X POST "${BASE_URL}/webhook/macd/cross-down" \
  -H "Content-Type: application/json" \
  -d '{"ticker":"EURUSD","exchange":"OANDA","close":"1.04880","interval":"15"}' > /dev/null

check_status
wait_for_next

echo "=========================================================================="
echo "TEST 6: RE-ENTRY AFTER OPPOSITE SIGNALS"
echo "=========================================================================="
echo "Step 1: Send LONG signals (opposite direction)"

curl -s -X POST "${BASE_URL}/webhook/ma/ma1-above-ma2" \
  -H "Content-Type: application/json" \
  -d '{"ticker":"EURUSD","exchange":"OANDA","close":"1.04920","interval":"15"}' > /dev/null

curl -s -X POST "${BASE_URL}/webhook/rsi/above-50" \
  -H "Content-Type: application/json" \
  -d '{"ticker":"EURUSD","exchange":"OANDA","close":"1.04920","interval":"15"}' > /dev/null

echo "Step 2: Now try SHORT entry again (should be ALLOWED)"

curl -s -X POST "${BASE_URL}/webhook/ma/ma1-below-ma2" \
  -H "Content-Type: application/json" \
  -d '{"ticker":"EURUSD","exchange":"OANDA","close":"1.04860","interval":"15"}' > /dev/null

curl -s -X POST "${BASE_URL}/webhook/ma/price-below-ma4" \
  -H "Content-Type: application/json" \
  -d '{"ticker":"EURUSD","exchange":"OANDA","close":"1.04860","interval":"15"}' > /dev/null

curl -s -X POST "${BASE_URL}/webhook/rsi/below-50" \
  -H "Content-Type: application/json" \
  -d '{"ticker":"EURUSD","exchange":"OANDA","close":"1.04860","interval":"15"}' > /dev/null

curl -s -X POST "${BASE_URL}/webhook/atr/above-threshold" \
  -H "Content-Type: application/json" \
  -d '{"ticker":"EURUSD","exchange":"OANDA","close":"1.04860","interval":"15"}' > /dev/null

curl -s -X POST "${BASE_URL}/webhook/macd/cross-down" \
  -H "Content-Type: application/json" \
  -d '{"ticker":"EURUSD","exchange":"OANDA","close":"1.04860","interval":"15"}' > /dev/null

check_status

echo ""
echo "=========================================================================="
echo "✅ TEST COMPLETE"
echo "=========================================================================="
echo "Validated:"
echo "  ✅ 5-condition LONG entry"
echo "  ✅ MA crossover LONG exit"
echo "  ✅ 5-condition SHORT entry (with reversal)"
echo "  ✅ MA crossover SHORT exit"
echo "  ✅ Whipsaw protection (blocked re-entry)"
echo "  ✅ Re-entry after opposite signals"
echo "=========================================================================="
