#!/bin/bash

# Test script for trading hours functionality
# This simulates webhooks at different times to verify trading hours restrictions

WEBHOOK_URL="http://localhost:8080"
SYMBOL="EUR_USD"

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "🧪 Testing Trading Hours Feature"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "Configuration from .env:"
echo "  TRADING_START_HOUR=3  (3 AM)"
echo "  TRADING_END_HOUR=16   (4 PM)"
echo "  TRADING_DAYS=1,2,3,4,5 (Mon-Fri)"
echo "  TIMEZONE_OFFSET=-5    (EST/CDT)"
echo ""

# Get current time info
CURRENT_HOUR=$(TZ=America/New_York date +%H)
CURRENT_DAY=$(TZ=America/New_York date +%u)  # 1=Mon, 7=Sun
CURRENT_TIME=$(TZ=America/New_York date +"%Y-%m-%d %H:%M:%S %Z")
DAY_NAME=$(TZ=America/New_York date +%A)

echo "Current time (EST/EDT): $CURRENT_TIME"
echo "Current hour: $CURRENT_HOUR"
echo "Current day: $DAY_NAME (day $CURRENT_DAY of week)"
echo ""

# Determine if we're in trading hours
IN_TRADING_HOURS=false
if [ $CURRENT_DAY -ge 1 ] && [ $CURRENT_DAY -le 5 ]; then
    if [ $CURRENT_HOUR -ge 3 ] && [ $CURRENT_HOUR -lt 16 ]; then
        IN_TRADING_HOURS=true
    fi
fi

if [ "$IN_TRADING_HOURS" = true ]; then
    echo "✅ Currently WITHIN trading hours (Mon-Fri 3AM-4PM)"
    echo "   Expected: Positions should open when conditions are met"
else
    echo "🚫 Currently OUTSIDE trading hours"
    echo "   Expected: Conditions tracked but positions blocked"
fi
echo ""

# Prepare webhook payload
WEBHOOK_PAYLOAD=$(cat <<EOF
{
  "ticker": "$SYMBOL",
  "exchange": "OANDA",
  "interval": "15",
  "close": "1.05000",
  "open": "1.04900",
  "high": "1.05100",
  "low": "1.04800",
  "volume": "1000",
  "time": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "timenow": "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
}
EOF
)

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "📊 Test 1: Sending LONG entry signals"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# LONG entry conditions for ma_trend_rsi_atr strategy:
# 1. Price above MA4 (HTF trend)
# 2. RSI above 50 (momentum)
# 3. MA1 above MA2 (entry signal)
# 4. ATR above threshold (volatility)

echo "Sending condition 1/4: Price above MA4..."
curl -s -X POST "$WEBHOOK_URL/webhook/ma/price-above-ma4" \
  -H "Content-Type: application/json" \
  -d "$WEBHOOK_PAYLOAD" > /dev/null
sleep 1

echo "Sending condition 2/4: RSI above 50..."
curl -s -X POST "$WEBHOOK_URL/webhook/rsi/above-50" \
  -H "Content-Type: application/json" \
  -d "$WEBHOOK_PAYLOAD" > /dev/null
sleep 1

echo "Sending condition 3/4: MA1 above MA2..."
curl -s -X POST "$WEBHOOK_URL/webhook/ma/ma1-above-ma2" \
  -H "Content-Type: application/json" \
  -d "$WEBHOOK_PAYLOAD" > /dev/null
sleep 1

echo "Sending condition 4/4: ATR above threshold..."
curl -s -X POST "$WEBHOOK_URL/webhook/atr/above-threshold" \
  -H "Content-Type: application/json" \
  -d "$WEBHOOK_PAYLOAD" > /dev/null
sleep 1

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "📋 Check Docker Logs for Results"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "Looking for key log messages..."
sleep 1

# Check logs for key messages
echo ""
echo "Last 30 lines of logs:"
docker logs tradingview-webhook-bot --tail 30 2>&1 | grep -E "(Entry condition|All entry conditions|BLOCKED|Trading hours|Opening LONG)"

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "✅ Test Complete"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
if [ "$IN_TRADING_HOURS" = true ]; then
    echo "Expected result: Position should have opened (within trading hours)"
else
    echo "Expected result: Position blocked but conditions tracked (outside trading hours)"
    echo "                 Look for '⏰ [BLOCKED] Position ready to open but outside trading hours'"
fi
echo ""
echo "Run 'docker logs tradingview-webhook-bot -f' to see live logs"
