#!/bin/bash

# Quick helper to restart bot with a specific amount for testing
# Supports both TRADE_USD_AMOUNT and MARGIN_AMOUNT

MODE=""
AMOUNT=""

# Parse arguments
if [ "$1" == "--margin" ]; then
    MODE="margin"
    AMOUNT=$2
elif [ "$1" == "--usd" ]; then
    MODE="usd"
    AMOUNT=$2
elif [ -n "$1" ] && [ -z "$2" ]; then
    # Default to margin if just a number is provided
    MODE="margin"
    AMOUNT=$1
else
    echo "Usage:"
    echo "  ./scripts/quick-test-usd.sh <amount>          # Test with margin (default)"
    echo "  ./scripts/quick-test-usd.sh --margin 100      # Test with \$100 margin"
    echo "  ./scripts/quick-test-usd.sh --usd 1000        # Test with \$1000 notional"
    echo ""
    echo "Examples:"
    echo "  ./scripts/quick-test-usd.sh 100        # \$100 margin (50:1 leverage = ~\$5000 position)"
    echo "  ./scripts/quick-test-usd.sh --margin 50       # \$50 margin"
    echo "  ./scripts/quick-test-usd.sh --usd 1000        # \$1000 notional value"
    echo ""
    exit 1
fi

if [ "$MODE" == "margin" ]; then
    ENV_VAR="MARGIN_AMOUNT"
    DISPLAY_NAME="margin"
    CALC_INFO="(with 50:1 leverage = ~\$$(($AMOUNT * 50)) position)"
else
    ENV_VAR="TRADE_USD_AMOUNT"
    DISPLAY_NAME="USD notional"
    CALC_INFO=""
fi

echo "🔄 Temporarily updating .env with $ENV_VAR=\$$AMOUNT..."
echo ""

# Backup current .env
cp .env .env.backup

# Update or add the environment variable in .env
# First, remove both MARGIN_AMOUNT and TRADE_USD_AMOUNT to avoid conflicts
sed -i '/^MARGIN_AMOUNT=/d' .env
sed -i '/^TRADE_USD_AMOUNT=/d' .env

# Add the selected one
echo "" >> .env
echo "# Temporary test value" >> .env
echo "$ENV_VAR=$AMOUNT" >> .env

echo "✅ .env updated (backup saved as .env.backup)"
echo ""
echo "🔄 Restarting trading bot..."
docker-compose up -d --force-recreate trading-bot

echo ""
echo "⏳ Waiting 5 seconds for bot to start..."
sleep 5
echo ""

echo "📋 Checking bot status..."
docker ps --filter "name=tradingview-webhook-bot" --format "table {{.Names}}\t{{.Status}}"
echo ""

# Check if container is running
if ! docker ps --filter "name=tradingview-webhook-bot" --filter "status=running" | grep -q tradingview-webhook-bot; then
    echo "❌ ERROR: Bot failed to start!"
    echo ""
    echo "📋 Last 20 lines of logs:"
    docker logs --tail 20 tradingview-webhook-bot
    echo ""
    echo "🔄 Restoring .env from backup..."
    mv .env.backup .env
    exit 1
fi

echo "✅ Bot started successfully! Now running test..."
echo ""
echo "💰 Testing with \$$AMOUNT $DISPLAY_NAME $CALC_INFO"
echo ""

# Run the appropriate test
if [ "$MODE" == "margin" ]; then
    ./scripts/test-margin-amount.sh
else
    ./scripts/test-usd-amount.sh
fi

echo ""
echo "================================================"
echo "🔄 Restoring original .env..."
mv .env.backup .env

echo "✅ .env restored!"
echo ""
echo "💡 To keep using this setting, add to your .env:"
echo "   $ENV_VAR=$AMOUNT"
echo ""
echo "💡 To apply restored .env settings:"
echo "   docker-compose up -d --force-recreate trading-bot"
echo ""
