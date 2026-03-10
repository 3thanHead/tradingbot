#!/bin/bash

# Test pip-based stop loss accuracy
# This script:
# 1. Gets instrument info (pipLocation) from OANDA
# 2. Places a small test trade with STOP_LOSS_PIPS
# 3. Queries the trade to verify the actual stop loss distance
# 4. Compares expected vs actual

set -e

# Load environment variables
source .env

SYMBOL="EUR_USD"
TEST_PIPS=15
BASE_URL="http://localhost:8080"

# Determine OANDA API base URL
if [ "$OANDA_LIVE" = "true" ]; then
    OANDA_URL="https://api-fxtrade.oanda.com"
else
    OANDA_URL="https://api-fxpractice.oanda.com"
fi

echo "🧪 Testing Pip-Based Stop Loss Accuracy"
echo "========================================"
echo ""
echo "📊 Test Parameters:"
echo "   Symbol: $SYMBOL"
echo "   Stop Loss Pips: $TEST_PIPS"
echo "   OANDA URL: $OANDA_URL"
echo ""

# Step 1: Get instrument info including pipLocation
echo "📋 Step 1: Getting instrument info from OANDA..."
INSTRUMENT_INFO=$(curl -s -X GET "$OANDA_URL/v3/accounts/${OANDA_ACCOUNT_ID}/instruments?instruments=$SYMBOL" \
  -H "Authorization: Bearer ${OANDA_API_KEY}" \
  -H "Content-Type: application/json")

PIP_LOCATION=$(echo "$INSTRUMENT_INFO" | python3 -c "import sys, json; data = json.load(sys.stdin); print(data['instruments'][0]['pipLocation'])")
DISPLAY_PRECISION=$(echo "$INSTRUMENT_INFO" | python3 -c "import sys, json; data = json.load(sys.stdin); print(data['instruments'][0]['displayPrecision'])")

echo "   pipLocation: $PIP_LOCATION"
echo "   displayPrecision: $DISPLAY_PRECISION"

# Calculate expected pip value (e.g., pipLocation=-4 means pip=0.0001)
EXPECTED_PIP_VALUE=$(python3 -c "print(10 ** $PIP_LOCATION)")
EXPECTED_DISTANCE=$(python3 -c "print(f'{$TEST_PIPS * 10 ** $PIP_LOCATION:.5f}')")

echo "   Expected pip value: $EXPECTED_PIP_VALUE"
echo "   Expected distance for $TEST_PIPS pips: $EXPECTED_DISTANCE"
echo ""

# Step 2: Get current price
echo "📋 Step 2: Getting current price..."
PRICE_INFO=$(curl -s -X GET "$OANDA_URL/v3/accounts/${OANDA_ACCOUNT_ID}/pricing?instruments=$SYMBOL" \
  -H "Authorization: Bearer ${OANDA_API_KEY}" \
  -H "Content-Type: application/json")

BID_PRICE=$(echo "$PRICE_INFO" | python3 -c "import sys, json; data = json.load(sys.stdin); print(data['prices'][0]['bids'][0]['price'])")
ASK_PRICE=$(echo "$PRICE_INFO" | python3 -c "import sys, json; data = json.load(sys.stdin); print(data['prices'][0]['asks'][0]['price'])")

echo "   Bid: $BID_PRICE"
echo "   Ask: $ASK_PRICE"
echo ""

# Step 3: Close any existing positions first
echo "📋 Step 3: Closing any existing $SYMBOL positions..."
curl -s -X PUT "$OANDA_URL/v3/accounts/${OANDA_ACCOUNT_ID}/positions/$SYMBOL/close" \
  -H "Authorization: Bearer ${OANDA_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{"longUnits": "ALL", "shortUnits": "ALL"}' > /dev/null 2>&1 || true
echo "   Done"
echo ""

# Step 4: Place a test trade with pip-based stop loss using direct OANDA API
# Using small position size (1000 units = micro lot)
echo "📋 Step 4: Placing test LONG trade with $TEST_PIPS pip stop loss..."

ORDER_RESPONSE=$(curl -s -X POST "$OANDA_URL/v3/accounts/${OANDA_ACCOUNT_ID}/orders" \
  -H "Authorization: Bearer ${OANDA_API_KEY}" \
  -H "Content-Type: application/json" \
  -d "{
    \"order\": {
      \"type\": \"MARKET\",
      \"instrument\": \"$SYMBOL\",
      \"units\": \"1000\",
      \"timeInForce\": \"FOK\",
      \"positionFill\": \"DEFAULT\",
      \"stopLossOnFill\": {
        \"distance\": \"$EXPECTED_DISTANCE\",
        \"timeInForce\": \"GTC\"
      }
    }
  }")

# Check if order was filled
ORDER_FILL_ID=$(echo "$ORDER_RESPONSE" | python3 -c "import sys, json; data = json.load(sys.stdin); print(data.get('orderFillTransaction', {}).get('tradeOpened', {}).get('tradeID', 'NOT_FOUND'))" 2>/dev/null || echo "ERROR")

if [ "$ORDER_FILL_ID" = "NOT_FOUND" ] || [ "$ORDER_FILL_ID" = "ERROR" ]; then
    echo "❌ Failed to place order!"
    echo "$ORDER_RESPONSE" | python3 -m json.tool
    exit 1
fi

FILL_PRICE=$(echo "$ORDER_RESPONSE" | python3 -c "import sys, json; data = json.load(sys.stdin); print(data['orderFillTransaction']['price'])")

echo "   ✅ Trade opened: ID $ORDER_FILL_ID"
echo "   Fill price: $FILL_PRICE"
echo ""

# Step 5: Wait for order to settle
sleep 1

# Step 6: Query the trade to see actual stop loss
echo "📋 Step 5: Querying trade details..."
TRADE_INFO=$(curl -s -X GET "$OANDA_URL/v3/accounts/${OANDA_ACCOUNT_ID}/trades/$ORDER_FILL_ID" \
  -H "Authorization: Bearer ${OANDA_API_KEY}" \
  -H "Content-Type: application/json")

echo ""
echo "📊 Trade Details:"
echo "$TRADE_INFO" | python3 -m json.tool

# Extract stop loss price
SL_PRICE=$(echo "$TRADE_INFO" | python3 -c "import sys, json; data = json.load(sys.stdin); print(data['trade'].get('stopLossOrder', {}).get('price', 'NOT_SET'))")

echo ""
echo "========================================"
echo "📋 VERIFICATION RESULTS"
echo "========================================"
echo ""
echo "Expected Stop Loss:"
echo "   Pips: $TEST_PIPS"
echo "   Pip Value: $EXPECTED_PIP_VALUE"
echo "   Distance: $EXPECTED_DISTANCE"
echo ""
echo "Actual Results:"
echo "   Fill Price: $FILL_PRICE"
echo "   Stop Loss Price: $SL_PRICE"

if [ "$SL_PRICE" != "NOT_SET" ]; then
    # Calculate actual distance
    ACTUAL_DISTANCE=$(python3 -c "print(f'{abs(float(\"$FILL_PRICE\") - float(\"$SL_PRICE\")):.5f}')")
    ACTUAL_PIPS=$(python3 -c "print(f'{abs(float(\"$FILL_PRICE\") - float(\"$SL_PRICE\")) / (10 ** $PIP_LOCATION):.1f}')")
    
    echo "   Actual Distance: $ACTUAL_DISTANCE"
    echo "   Actual Pips: $ACTUAL_PIPS"
    echo ""
    
    # Compare
    DIFF=$(python3 -c "print(f'{abs(float(\"$EXPECTED_DISTANCE\") - float(\"$ACTUAL_DISTANCE\")):.6f}')")
    
    if python3 -c "exit(0 if abs(float('$EXPECTED_DISTANCE') - float('$ACTUAL_DISTANCE')) < 0.00005 else 1)"; then
        echo "✅ SUCCESS: Stop loss distance matches expected! (diff: $DIFF)"
    else
        echo "❌ MISMATCH: Expected $EXPECTED_DISTANCE, got $ACTUAL_DISTANCE (diff: $DIFF)"
    fi
else
    echo "❌ ERROR: Stop loss was not set on the trade!"
fi

echo ""

# Step 7: Close the test trade
echo "📋 Step 6: Closing test trade..."
curl -s -X PUT "$OANDA_URL/v3/accounts/${OANDA_ACCOUNT_ID}/trades/$ORDER_FILL_ID/close" \
  -H "Authorization: Bearer ${OANDA_API_KEY}" \
  -H "Content-Type: application/json" > /dev/null

echo "   ✅ Test trade closed"
echo ""
echo "🧪 Test complete!"
