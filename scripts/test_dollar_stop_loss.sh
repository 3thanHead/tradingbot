#!/bin/bash

# Test dollar-based stop loss accuracy
# This tests that the stop loss dollar amount matches the price distance

set -e

source .env

SYMBOL="EUR_USD"
TEST_MARGIN=5000
TEST_SL_DOLLARS=150
BASE_URL="http://localhost:8080"

# Determine OANDA API base URL
if [ "$OANDA_LIVE" = "true" ]; then
    OANDA_URL="https://api-fxtrade.oanda.com"
else
    OANDA_URL="https://api-fxpractice.oanda.com"
fi

echo "🧪 Testing Dollar-Based Stop Loss Accuracy"
echo "==========================================="
echo ""
echo "📊 Test Parameters:"
echo "   Symbol: $SYMBOL"
echo "   Margin: \$$TEST_MARGIN"
echo "   Stop Loss: \$$TEST_SL_DOLLARS"
echo ""

# Step 1: Close any existing positions
echo "📋 Step 1: Closing any existing $SYMBOL positions..."
curl -s -X PUT "$OANDA_URL/v3/accounts/${OANDA_ACCOUNT_ID}/positions/$SYMBOL/close" \
  -H "Authorization: Bearer ${OANDA_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{"longUnits": "ALL", "shortUnits": "ALL"}' > /dev/null 2>&1 || true
echo "   Done"
echo ""

# Step 2: Get instrument info
echo "📋 Step 2: Getting instrument info..."
INSTRUMENT_INFO=$(curl -s -X GET "$OANDA_URL/v3/accounts/${OANDA_ACCOUNT_ID}/instruments?instruments=$SYMBOL" \
  -H "Authorization: Bearer ${OANDA_API_KEY}")

PIP_LOCATION=$(echo "$INSTRUMENT_INFO" | python3 -c "import sys, json; data = json.load(sys.stdin); print(data['instruments'][0]['pipLocation'])")
PIP_VALUE=$(python3 -c "print(10 ** $PIP_LOCATION)")

echo "   Pip Location: $PIP_LOCATION"
echo "   Pip Value: $PIP_VALUE"
echo ""

# Step 3: Get current price and home conversion factors
echo "📋 Step 3: Getting current price and conversion factors..."
PRICE_INFO=$(curl -s -X GET "$OANDA_URL/v3/accounts/${OANDA_ACCOUNT_ID}/pricing?instruments=$SYMBOL&includeHomeConversions=true" \
  -H "Authorization: Bearer ${OANDA_API_KEY}")

ASK_PRICE=$(echo "$PRICE_INFO" | python3 -c "import sys, json; data = json.load(sys.stdin); print(data['prices'][0]['asks'][0]['price'])")

# For EUR_USD, quote currency is USD (home currency), so conversion is 1.0
# For other pairs, we need to look at homeConversions array
LOSS_FACTOR=$(echo "$PRICE_INFO" | python3 -c "
import sys, json
data = json.load(sys.stdin)
# For XXX_USD pairs, factor is 1.0 (USD is home currency)
# Look for USD in homeConversions
for conv in data.get('homeConversions', []):
    if conv['currency'] == 'USD':
        print(conv['accountLoss'])
        break
else:
    print('1.0')
")

echo "   Ask Price: $ASK_PRICE"
echo "   Loss Conversion Factor: $LOSS_FACTOR"
echo ""

# Step 4: Calculate expected values
echo "📋 Step 4: Calculating expected values..."

# Calculate units from margin (50:1 leverage for EUR_USD)
UNITS=$(python3 -c "import math; price=$ASK_PRICE; margin=$TEST_MARGIN; leverage=50; units=int((margin*leverage)/price); print(units)")

# Calculate expected price move for dollar stop loss
# Formula: priceMove = targetDollars / (units × conversionFactor)
EXPECTED_PRICE_MOVE=$(python3 -c "print(f'{$TEST_SL_DOLLARS / ($UNITS * $LOSS_FACTOR):.5f}')")
EXPECTED_SL_PRICE=$(python3 -c "print(f'{$ASK_PRICE - $TEST_SL_DOLLARS / ($UNITS * $LOSS_FACTOR):.5f}')")
EXPECTED_PIPS=$(python3 -c "print(f'{$TEST_SL_DOLLARS / ($UNITS * $LOSS_FACTOR) / $PIP_VALUE:.1f}')")

# Calculate pip value per pip for this position
PIP_VALUE_DOLLARS=$(python3 -c "print(f'{$UNITS * $PIP_VALUE * $LOSS_FACTOR:.2f}')")

echo "   Units: $UNITS"
echo "   Pip Value: \$$PIP_VALUE_DOLLARS per pip"
echo "   Expected Price Move: $EXPECTED_PRICE_MOVE"
echo "   Expected Stop Loss Price: $EXPECTED_SL_PRICE"
echo "   Expected Pips: $EXPECTED_PIPS"
echo ""

# Step 5: Place order with dollar-based stop loss
echo "📋 Step 5: Placing order with \$$TEST_SL_DOLLARS stop loss..."

# We need to calculate the distance ourselves since we're hitting OANDA directly
# distance = targetDollars / (units × conversionFactor)
DISTANCE=$(python3 -c "print(f'{$TEST_SL_DOLLARS / ($UNITS * $LOSS_FACTOR):.5f}')")

RESPONSE=$(curl -s -X POST "$OANDA_URL/v3/accounts/${OANDA_ACCOUNT_ID}/orders" \
  -H "Authorization: Bearer ${OANDA_API_KEY}" \
  -H "Content-Type: application/json" \
  -d "{
    \"order\": {
      \"type\": \"MARKET\",
      \"instrument\": \"$SYMBOL\",
      \"units\": \"$UNITS\",
      \"timeInForce\": \"FOK\",
      \"positionFill\": \"DEFAULT\",
      \"stopLossOnFill\": {
        \"distance\": \"$DISTANCE\",
        \"timeInForce\": \"GTC\"
      }
    }
  }")

TRADE_ID=$(echo "$RESPONSE" | python3 -c "import sys,json; print(json.load(sys.stdin)['orderFillTransaction']['tradeOpened']['tradeID'])" 2>/dev/null || echo "ERROR")

if [ "$TRADE_ID" = "ERROR" ]; then
    echo "❌ Failed to place order!"
    echo "$RESPONSE" | python3 -m json.tool
    exit 1
fi

FILL_PRICE=$(echo "$RESPONSE" | python3 -c "import sys,json; print(json.load(sys.stdin)['orderFillTransaction']['price'])")
MARGIN_USED=$(echo "$RESPONSE" | python3 -c "import sys,json; print(json.load(sys.stdin)['orderFillTransaction']['tradeOpened']['initialMarginRequired'])")

echo "   ✅ Trade opened!"
echo "   Trade ID: $TRADE_ID"
echo "   Fill Price: $FILL_PRICE"
echo "   Margin Used: \$$MARGIN_USED"
echo ""

# Step 6: Query trade details
sleep 1
echo "📋 Step 6: Querying trade details..."
TRADE_INFO=$(curl -s -X GET "$OANDA_URL/v3/accounts/${OANDA_ACCOUNT_ID}/trades/$TRADE_ID" \
  -H "Authorization: Bearer ${OANDA_API_KEY}")

SL_PRICE=$(echo "$TRADE_INFO" | python3 -c "import sys,json; print(json.load(sys.stdin)['trade']['stopLossOrder']['price'])")

echo ""
echo "========================================"
echo "📋 VERIFICATION RESULTS"
echo "========================================"
echo ""
echo "Trade Details:"
echo "   Fill Price: $FILL_PRICE"
echo "   Stop Loss: $SL_PRICE"
echo ""

# Calculate actual values
ACTUAL_DISTANCE=$(python3 -c "print(f'{abs(float(\"$FILL_PRICE\") - float(\"$SL_PRICE\")):.5f}')")
ACTUAL_PIPS=$(python3 -c "print(f'{abs(float(\"$FILL_PRICE\") - float(\"$SL_PRICE\")) / $PIP_VALUE:.1f}')")
ACTUAL_DOLLAR_LOSS=$(python3 -c "print(f'{abs(float(\"$FILL_PRICE\") - float(\"$SL_PRICE\")) * $UNITS * $LOSS_FACTOR:.2f}')")

echo "Actual Stop Loss:"
echo "   Distance: $ACTUAL_DISTANCE"
echo "   Pips: $ACTUAL_PIPS"
echo "   Dollar Loss: \$$ACTUAL_DOLLAR_LOSS"
echo ""
echo "Expected:"
echo "   Target Dollar Loss: \$$TEST_SL_DOLLARS"
echo "   Expected Pips: $EXPECTED_PIPS"
echo ""

# Verify
DIFF=$(python3 -c "print(f'{abs($TEST_SL_DOLLARS - float(\"$ACTUAL_DOLLAR_LOSS\")):.2f}')")

if python3 -c "exit(0 if abs($TEST_SL_DOLLARS - float('$ACTUAL_DOLLAR_LOSS')) < 1.0 else 1)"; then
    echo "✅ SUCCESS: Stop loss dollar amount matches! (diff: \$$DIFF)"
else
    echo "❌ MISMATCH: Expected \$$TEST_SL_DOLLARS, got \$$ACTUAL_DOLLAR_LOSS (diff: \$$DIFF)"
fi

echo ""
echo "📌 Trade left open for inspection (Trade ID: $TRADE_ID)"
echo ""
echo "🧪 Test complete!"
