#!/bin/bash

# Test pip-based stop loss through the trading bot
# This tests the actual code path in the bot using Docker

set -e

# Load environment variables
set -a  # Export all variables
source .env
set +a

SYMBOL="EUR_USD"
TEST_PIPS=20
BASE_URL="http://localhost:8080"

# Determine OANDA API base URL
if [ "$OANDA_LIVE" = "true" ]; then
    OANDA_URL="https://api-fxtrade.oanda.com"
else
    OANDA_URL="https://api-fxpractice.oanda.com"
fi

echo "🧪 Testing Pip-Based Stop Loss Through Bot (Docker)"
echo "===================================================="
echo ""
echo "📊 Test Parameters:"
echo "   Symbol: $SYMBOL"
echo "   Stop Loss Pips: $TEST_PIPS"
echo ""

# Step 1: Get instrument info including pipLocation
echo "📋 Step 1: Getting instrument info from OANDA..."
INSTRUMENT_INFO=$(curl -s -X GET "$OANDA_URL/v3/accounts/${OANDA_ACCOUNT_ID}/instruments?instruments=$SYMBOL" \
  -H "Authorization: Bearer ${OANDA_API_KEY}" \
  -H "Content-Type: application/json")

PIP_LOCATION=$(echo "$INSTRUMENT_INFO" | python3 -c "import sys, json; data = json.load(sys.stdin); print(data['instruments'][0]['pipLocation'])")
EXPECTED_PIP_VALUE=$(python3 -c "print(10 ** $PIP_LOCATION)")
EXPECTED_DISTANCE=$(python3 -c "print(f'{$TEST_PIPS * 10 ** $PIP_LOCATION:.5f}')")

echo "   pipLocation: $PIP_LOCATION"
echo "   Expected pip value: $EXPECTED_PIP_VALUE"
echo "   Expected distance for $TEST_PIPS pips: $EXPECTED_DISTANCE"
echo ""

# Step 2: Close any existing positions
echo "📋 Step 2: Closing any existing $SYMBOL positions..."
curl -s -X PUT "$OANDA_URL/v3/accounts/${OANDA_ACCOUNT_ID}/positions/$SYMBOL/close" \
  -H "Authorization: Bearer ${OANDA_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{"longUnits": "ALL", "shortUnits": "ALL"}' > /dev/null 2>&1 || true
echo "   Done"
echo ""

# Step 3: Stop any existing bot container
echo "📋 Step 3: Stopping any existing bot container..."
docker-compose down 2>/dev/null || true
sleep 2
echo "   Done"
echo ""

# Step 4: Build and start bot with STOP_LOSS_PIPS
echo "📋 Step 4: Starting bot with STOP_LOSS_PIPS=$TEST_PIPS..."
# Override environment variables for test
export STOP_LOSS_PIPS="$TEST_PIPS"
export STOP_LOSS_DOLLARS=""
export TAKE_PROFIT_DOLLARS=""
export MARGIN_AMOUNT=""
export TRADE_UNITS="1000"

docker-compose build --quiet trading-bot
docker-compose up -d trading-bot

# Wait for bot to be ready
echo "   Waiting for bot to start..."
sleep 8

# Check if bot is responding
for i in {1..5}; do
    if curl -s "$BASE_URL/status" > /dev/null 2>&1; then
        break
    fi
    echo "   Retry $i..."
    sleep 2
done

if ! curl -s "$BASE_URL/status" > /dev/null 2>&1; then
    echo "❌ Bot failed to start! Checking logs..."
    docker-compose logs trading-bot | tail -20
    docker-compose down
    exit 1
fi

echo "   ✅ Bot running"
echo ""

# Step 5: Trigger a trade through ATR webhook
# ATR strategy requires state change (cross), so we send SHORT first, then LONG
echo "📋 Step 5a: Sending SHORT signal to initialize state..."

curl -s -X POST "$BASE_URL/webhook/atr/short" \
  -H "Content-Type: application/json" \
  -d "{
    \"ticker\": \"$SYMBOL\",
    \"exchange\": \"OANDA\",
    \"close\": \"1.1600\"
  }" > /dev/null

sleep 1

echo "📋 Step 5b: Triggering LONG trade via /webhook/atr/long (cross)..."

RESPONSE=$(curl -s -X POST "$BASE_URL/webhook/atr/long" \
  -H "Content-Type: application/json" \
  -d "{
    \"ticker\": \"$SYMBOL\",
    \"exchange\": \"OANDA\",
    \"close\": \"1.1600\"
  }")

echo "   Response: $RESPONSE"
echo ""

# Wait for trade to be placed
echo "⏳ Waiting 3 seconds for trade execution..."
sleep 3

# Step 6: Query OANDA for open trades
echo "📋 Step 6: Checking open trades in OANDA..."
TRADES_INFO=$(curl -s -X GET "$OANDA_URL/v3/accounts/${OANDA_ACCOUNT_ID}/openTrades" \
  -H "Authorization: Bearer ${OANDA_API_KEY}" \
  -H "Content-Type: application/json")

echo ""
echo "📊 Open Trades:"
echo "$TRADES_INFO" | python3 -m json.tool

# Find EUR_USD trade
TRADE_ID=$(echo "$TRADES_INFO" | python3 -c "
import sys, json
data = json.load(sys.stdin)
for trade in data.get('trades', []):
    if trade.get('instrument') == '$SYMBOL':
        print(trade['id'])
        break
else:
    print('NOT_FOUND')
" 2>/dev/null || echo "ERROR")

echo ""
echo "========================================"
echo "📋 VERIFICATION RESULTS"
echo "========================================"
echo ""

if [ "$TRADE_ID" != "NOT_FOUND" ] && [ "$TRADE_ID" != "ERROR" ] && [ -n "$TRADE_ID" ]; then
    # Get specific trade details
    TRADE_INFO=$(curl -s -X GET "$OANDA_URL/v3/accounts/${OANDA_ACCOUNT_ID}/trades/$TRADE_ID" \
      -H "Authorization: Bearer ${OANDA_API_KEY}" \
      -H "Content-Type: application/json")
    
    FILL_PRICE=$(echo "$TRADE_INFO" | python3 -c "import sys, json; data = json.load(sys.stdin); print(data['trade']['price'])")
    SL_PRICE=$(echo "$TRADE_INFO" | python3 -c "import sys, json; data = json.load(sys.stdin); print(data['trade'].get('stopLossOrder', {}).get('price', 'NOT_SET'))")
    
    echo "Trade ID: $TRADE_ID"
    echo "Fill Price: $FILL_PRICE"
    echo "Stop Loss Price: $SL_PRICE"
    echo ""
    echo "Expected Stop Loss:"
    echo "   Pips: $TEST_PIPS"
    echo "   Distance: $EXPECTED_DISTANCE"
    
    if [ "$SL_PRICE" != "NOT_SET" ]; then
        ACTUAL_DISTANCE=$(python3 -c "print(f'{abs(float(\"$FILL_PRICE\") - float(\"$SL_PRICE\")):.5f}')")
        ACTUAL_PIPS=$(python3 -c "print(f'{abs(float(\"$FILL_PRICE\") - float(\"$SL_PRICE\")) / (10 ** $PIP_LOCATION):.1f}')")
        
        echo ""
        echo "Actual Stop Loss:"
        echo "   Distance: $ACTUAL_DISTANCE"
        echo "   Pips: $ACTUAL_PIPS"
        
        DIFF=$(python3 -c "print(f'{abs(float(\"$EXPECTED_DISTANCE\") - float(\"$ACTUAL_DISTANCE\")):.6f}')")
        
        echo ""
        if python3 -c "exit(0 if abs(float('$EXPECTED_DISTANCE') - float('$ACTUAL_DISTANCE')) < 0.00010 else 1)"; then
            echo "✅ SUCCESS: Stop loss distance matches expected! (diff: $DIFF)"
        else
            echo "❌ MISMATCH: Expected $EXPECTED_DISTANCE, got $ACTUAL_DISTANCE (diff: $DIFF)"
        fi
    else
        echo "❌ ERROR: Stop loss was not set on the trade!"
    fi
    
    # Close the test trade
    echo ""
    echo "📋 Closing test trade..."
    curl -s -X PUT "$OANDA_URL/v3/accounts/${OANDA_ACCOUNT_ID}/trades/$TRADE_ID/close" \
      -H "Authorization: Bearer ${OANDA_API_KEY}" \
      -H "Content-Type: application/json" > /dev/null
    echo "   ✅ Trade closed"
else
    echo "❌ No $SYMBOL trade found - trade may not have been placed"
    echo "   Checking bot logs..."
    docker-compose logs trading-bot | tail -30
fi

# Stop the bot
echo ""
echo "📋 Stopping bot..."
docker-compose down
echo "   ✅ Bot stopped"

echo ""
echo "🧪 Test complete!"
