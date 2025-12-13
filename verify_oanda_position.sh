#!/bin/bash

# Load environment variables
source .env

echo "🔍 Verifying OANDA Position Details"
echo "===================================="
echo ""

# Get the trade details from OANDA
curl -s -X GET "https://api-fxpractice.oanda.com/v3/accounts/${OANDA_ACCOUNT_ID}/trades/1272" \
  -H "Authorization: Bearer ${OANDA_API_KEY}" \
  -H "Content-Type: application/json" | python3 -m json.tool

echo ""
echo "📊 Key details to verify:"
echo "   - currentUnits: Should be 214500"
echo "   - initialMarginRequired: Should be ~\$5000"
echo "   - instrument: EUR_USD"
echo ""
