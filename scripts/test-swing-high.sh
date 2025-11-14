#!/bin/bash

# Test New Swing High webhook
curl -X POST http://localhost:8080/webhook/swing/new-high \
  -H "Content-Type: application/json" \
  -d '{
    "ticker": "EUR_USD",
    "exchange": "OANDA",
    "interval": "5",
    "close": "1.09500",
    "high": "1.09750",
    "timenow": "2024-01-15T10:30:00Z"
  }'

echo ""
