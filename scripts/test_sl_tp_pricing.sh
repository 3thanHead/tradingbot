#!/bin/bash

# Test SL/TP calculation using real-time OANDA pricing
# Starts the bot, sends a webhook to open a LONG position,
# then queries OANDA to verify SL/TP are set correctly based on pips or dollars.
#
# Usage:
#   ./test_sl_tp_pricing.sh              # Test with current .env settings
#   ./test_sl_tp_pricing.sh pips 30 50   # Test with 30 pip SL / 50 pip TP
#   ./test_sl_tp_pricing.sh dollars 285 285  # Test with $285 SL / $285 TP

set -e

source .env

SYMBOL="EUR_USD"
BASE_URL="http://localhost:8080"
BOT_PORT=8080

# Parse optional overrides
MODE="${1:-}"
SL_VALUE="${2:-}"
TP_VALUE="${3:-}"

# Determine OANDA API base URL
if [ "$OANDA_LIVE" = "true" ]; then
    OANDA_URL="https://api-fxtrade.oanda.com"
else
    OANDA_URL="https://api-fxpractice.oanda.com"
fi

echo "🧪 Test: SL/TP Pricing Accuracy via Bot Webhook"
echo "================================================"
echo ""

# Build env overrides for the bot
declare -a ENV_OVERRIDES
ENV_OVERRIDES+=("OANDA_API_KEY=$OANDA_API_KEY")
ENV_OVERRIDES+=("OANDA_ACCOUNT_ID=$OANDA_ACCOUNT_ID")
ENV_OVERRIDES+=("OANDA_LIVE=$OANDA_LIVE")
ENV_OVERRIDES+=("MARGIN_AMOUNT=${MARGIN_AMOUNT:-5000}")   # Use .env margin amount
ENV_OVERRIDES+=("STRATEGY_FILE=atr_flip")
ENV_OVERRIDES+=("TIMEZONE_OFFSET=${TIMEZONE_OFFSET:--4}")

# Clear all SL/TP env vars first, then set the requested mode
ENV_OVERRIDES+=("STOP_LOSS_PIPS=")
ENV_OVERRIDES+=("STOP_LOSS_DOLLARS=")
ENV_OVERRIDES+=("STOP_LOSS_PCT=")
ENV_OVERRIDES+=("TAKE_PROFIT_PIPS=")
ENV_OVERRIDES+=("TAKE_PROFIT_DOLLARS=")
ENV_OVERRIDES+=("TAKE_PROFIT_PCT=")

if [ "$MODE" = "dollars" ] && [ -n "$SL_VALUE" ] && [ -n "$TP_VALUE" ]; then
    ENV_OVERRIDES+=("STOP_LOSS_DOLLARS=$SL_VALUE")
    ENV_OVERRIDES+=("TAKE_PROFIT_DOLLARS=$TP_VALUE")
    echo "📊 Mode: DOLLARS — SL: \$$SL_VALUE / TP: \$$TP_VALUE"
elif [ "$MODE" = "pips" ] && [ -n "$SL_VALUE" ] && [ -n "$TP_VALUE" ]; then
    ENV_OVERRIDES+=("STOP_LOSS_PIPS=$SL_VALUE")
    ENV_OVERRIDES+=("TAKE_PROFIT_PIPS=$TP_VALUE")
    echo "📊 Mode: PIPS — SL: ${SL_VALUE} pips / TP: ${TP_VALUE} pips"
else
    # Use current .env settings
    [ -n "$STOP_LOSS_PIPS" ] && ENV_OVERRIDES+=("STOP_LOSS_PIPS=$STOP_LOSS_PIPS")
    [ -n "$STOP_LOSS_DOLLARS" ] && ENV_OVERRIDES+=("STOP_LOSS_DOLLARS=$STOP_LOSS_DOLLARS")
    [ -n "$STOP_LOSS_PCT" ] && ENV_OVERRIDES+=("STOP_LOSS_PCT=$STOP_LOSS_PCT")
    [ -n "$TAKE_PROFIT_PIPS" ] && ENV_OVERRIDES+=("TAKE_PROFIT_PIPS=$TAKE_PROFIT_PIPS")
    [ -n "$TAKE_PROFIT_DOLLARS" ] && ENV_OVERRIDES+=("TAKE_PROFIT_DOLLARS=$TAKE_PROFIT_DOLLARS")
    [ -n "$TAKE_PROFIT_PCT" ] && ENV_OVERRIDES+=("TAKE_PROFIT_PCT=$TAKE_PROFIT_PCT")
    echo "📊 Mode: from .env — SL_PIPS=${STOP_LOSS_PIPS:-unset} SL_DOLLARS=${STOP_LOSS_DOLLARS:-unset} / TP_PIPS=${TAKE_PROFIT_PIPS:-unset} TP_DOLLARS=${TAKE_PROFIT_DOLLARS:-unset}"
fi

echo "   Symbol: $SYMBOL"
echo "   Margin: \$${MARGIN_AMOUNT:-5000}"
echo "   OANDA: $OANDA_URL"
echo ""

# Step 0: Close any existing positions
echo "📋 Step 0: Closing any existing $SYMBOL positions..."
curl -s -X PUT "$OANDA_URL/v3/accounts/${OANDA_ACCOUNT_ID}/positions/$SYMBOL/close" \
  -H "Authorization: Bearer ${OANDA_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{"longUnits": "ALL", "shortUnits": "ALL"}' > /dev/null 2>&1 || true

# Also close all open trades individually
OPEN_TRADES=$(curl -s -X GET "$OANDA_URL/v3/accounts/${OANDA_ACCOUNT_ID}/openTrades" \
  -H "Authorization: Bearer ${OANDA_API_KEY}" | python3 -c "
import sys, json
data = json.load(sys.stdin)
for t in data.get('trades', []):
    if t['instrument'] == '$SYMBOL':
        print(t['id'])
" 2>/dev/null || true)

for tid in $OPEN_TRADES; do
    curl -s -X PUT "$OANDA_URL/v3/accounts/${OANDA_ACCOUNT_ID}/trades/$tid/close" \
      -H "Authorization: Bearer ${OANDA_API_KEY}" \
      -H "Content-Type: application/json" > /dev/null 2>&1 || true
done
sleep 2
echo "   Done"
echo ""

# Step 1: Get live pricing for reference
echo "📋 Step 1: Getting OANDA pricing..."
PRICE_INFO=$(curl -s -X GET "$OANDA_URL/v3/accounts/${OANDA_ACCOUNT_ID}/pricing?instruments=$SYMBOL" \
  -H "Authorization: Bearer ${OANDA_API_KEY}" \
  -H "Content-Type: application/json")

BID_PRICE=$(echo "$PRICE_INFO" | python3 -c "import sys, json; data = json.load(sys.stdin); print(data['prices'][0]['bids'][0]['price'])")
ASK_PRICE=$(echo "$PRICE_INFO" | python3 -c "import sys, json; data = json.load(sys.stdin); print(data['prices'][0]['asks'][0]['price'])")
SPREAD=$(python3 -c "print(f'{float(\"$ASK_PRICE\") - float(\"$BID_PRICE\"):.5f}')")
SPREAD_PIPS=$(python3 -c "print(f'{(float(\"$ASK_PRICE\") - float(\"$BID_PRICE\")) / 0.0001:.1f}')")

echo "   Bid: $BID_PRICE"
echo "   Ask: $ASK_PRICE"
echo "   Spread: $SPREAD ($SPREAD_PIPS pips)"
echo ""

# Step 2: Build and start the bot
echo "📋 Step 2: Building and starting bot..."
go build -o trader_bot_test_bin . 2>&1

# Kill any existing bot instances on the port
lsof -ti:$BOT_PORT 2>/dev/null | xargs kill -9 2>/dev/null || true
sleep 1

# Start the bot with env overrides
env "${ENV_OVERRIDES[@]}" ./trader_bot_test_bin > /tmp/bot_test_output.log 2>&1 &
BOT_PID=$!

cleanup() {
    echo ""
    echo "📋 Cleanup: Stopping bot (PID $BOT_PID)..."
    kill $BOT_PID 2>/dev/null || true
    wait $BOT_PID 2>/dev/null || true
    # Close the test trade
    echo "📋 Cleanup: Closing test positions..."
    curl -s -X PUT "$OANDA_URL/v3/accounts/${OANDA_ACCOUNT_ID}/positions/$SYMBOL/close" \
      -H "Authorization: Bearer ${OANDA_API_KEY}" \
      -H "Content-Type: application/json" \
      -d '{"longUnits": "ALL", "shortUnits": "ALL"}' > /dev/null 2>&1 || true
    rm -f trader_bot_test_bin
    echo "   ✅ Cleaned up"
}
trap cleanup EXIT

# Wait for bot to start
echo "   Waiting for bot to be ready..."
for i in $(seq 1 15); do
    if curl -s "$BASE_URL/health" > /dev/null 2>&1; then
        echo "   ✅ Bot is running (PID $BOT_PID)"
        break
    fi
    if [ $i -eq 15 ]; then
        echo "   ❌ Bot failed to start. Log output:"
        cat /tmp/bot_test_output.log
        exit 1
    fi
    sleep 1
done
echo ""

# Step 3: Initialize ATR state (must send opposing signal first, then cross)
echo "📋 Step 3a: Initializing ATR state (sending short signal first)..."
curl -s -X POST "$BASE_URL/webhook/atr/short" \
  -H "Content-Type: application/json" \
  -d "{\"ticker\": \"EURUSD\", \"exchange\": \"OANDA\", \"close\": \"$ASK_PRICE\"}" > /dev/null
sleep 1

echo "📋 Step 3b: Sending ATR Long webhook (cross from short → long)..."
WEBHOOK_RESPONSE=$(curl -s -X POST "$BASE_URL/webhook/atr/long" \
  -H "Content-Type: application/json" \
  -d "{\"ticker\": \"EURUSD\", \"exchange\": \"OANDA\", \"close\": \"$ASK_PRICE\"}")

echo "   Response: $WEBHOOK_RESPONSE"
echo ""

# Wait for trade to process
sleep 3

# Step 4: Check bot logs for pricing details
echo "📋 Step 4: Bot pricing logs:"
echo "   ────────────────────────────────────────"
grep -E "OANDA PRICING|SL CALC|TP CALC|SL\]|TP\]|MARGIN CALC|POSITION SIZE" /tmp/bot_test_output.log | tail -20 | sed 's/^/   /'
echo "   ────────────────────────────────────────"
echo ""

# Step 5: Query OANDA for the actual open trade
echo "📋 Step 5: Querying OANDA for open trades..."
TRADES_INFO=$(curl -s -X GET "$OANDA_URL/v3/accounts/${OANDA_ACCOUNT_ID}/openTrades" \
  -H "Authorization: Bearer ${OANDA_API_KEY}" \
  -H "Content-Type: application/json")

TRADE_ID=$(echo "$TRADES_INFO" | python3 -c "
import sys, json
data = json.load(sys.stdin)
trades = data.get('trades', [])
for t in trades:
    if t['instrument'] == '$SYMBOL':
        print(t['id'])
        break
else:
    print('NOT_FOUND')
")

if [ "$TRADE_ID" = "NOT_FOUND" ]; then
    echo "   ❌ No $SYMBOL trade found on OANDA!"
    echo ""
    echo "   Bot log output:"
    tail -40 /tmp/bot_test_output.log
    exit 1
fi

# Get full trade details
TRADE_DETAIL=$(curl -s -X GET "$OANDA_URL/v3/accounts/${OANDA_ACCOUNT_ID}/trades/$TRADE_ID" \
  -H "Authorization: Bearer ${OANDA_API_KEY}" \
  -H "Content-Type: application/json")

FILL_PRICE=$(echo "$TRADE_DETAIL" | python3 -c "import sys, json; print(json.load(sys.stdin)['trade']['price'])")
TRADE_UNITS=$(echo "$TRADE_DETAIL" | python3 -c "import sys, json; print(json.load(sys.stdin)['trade']['currentUnits'])")
SL_PRICE=$(echo "$TRADE_DETAIL" | python3 -c "import sys, json; print(json.load(sys.stdin)['trade'].get('stopLossOrder', {}).get('price', 'NOT_SET'))")
TP_PRICE=$(echo "$TRADE_DETAIL" | python3 -c "import sys, json; print(json.load(sys.stdin)['trade'].get('takeProfitOrder', {}).get('price', 'NOT_SET'))")

echo "   Trade ID: $TRADE_ID"
echo "   Fill Price: $FILL_PRICE"
echo "   Units: $TRADE_UNITS"
echo ""

# Step 6: Verification
echo "================================================"
echo "📋 VERIFICATION RESULTS"
echo "================================================"
echo ""
echo "   Fill Price (OANDA):  $FILL_PRICE"

if [ "$SL_PRICE" != "NOT_SET" ]; then
    SL_DISTANCE=$(python3 -c "print(f'{abs(float(\"$FILL_PRICE\") - float(\"$SL_PRICE\")):.5f}')")
    SL_PIPS=$(python3 -c "print(f'{abs(float(\"$FILL_PRICE\") - float(\"$SL_PRICE\")) / 0.0001:.1f}')")
    echo "   Stop Loss Price:     $SL_PRICE"
    echo "   SL Distance:         $SL_DISTANCE ($SL_PIPS pips from fill)"
else
    echo "   Stop Loss:           ❌ NOT SET"
fi

if [ "$TP_PRICE" != "NOT_SET" ]; then
    TP_DISTANCE=$(python3 -c "print(f'{abs(float(\"$FILL_PRICE\") - float(\"$TP_PRICE\")):.5f}')")
    TP_PIPS=$(python3 -c "print(f'{abs(float(\"$FILL_PRICE\") - float(\"$TP_PRICE\")) / 0.0001:.1f}')")
    echo "   Take Profit Price:   $TP_PRICE"
    echo "   TP Distance:         $TP_DISTANCE ($TP_PIPS pips from fill)"
else
    echo "   Take Profit:         ❌ NOT SET"
fi

echo ""
echo "   Spread at order:     $SPREAD ($SPREAD_PIPS pips)"
echo ""

# Summary
if [ "$SL_PRICE" != "NOT_SET" ] && [ "$TP_PRICE" != "NOT_SET" ]; then
    echo "   ✅ Both SL and TP were set on the OANDA trade"
elif [ "$SL_PRICE" != "NOT_SET" ] || [ "$TP_PRICE" != "NOT_SET" ]; then
    echo "   ⚠️  Only one of SL/TP was set"
else
    echo "   ❌ Neither SL nor TP was set!"
fi

echo ""
echo "🧪 Test complete!"
