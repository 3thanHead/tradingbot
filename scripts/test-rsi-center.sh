#!/bin/bash

# Test RSI Crossed Center webhook
curl -X POST http://localhost:8080/webhook/rsi/crossed-center \
  -H "Content-Type: application/json" \
  -d '{
    "ticker": "EUR_USD",
    "exchange": "OANDA",
    "interval": "15",
    "close": "1.09200",
    "timenow": "2024-01-15T10:35:00Z"
  }'

echo ""
