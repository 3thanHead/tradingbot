#!/bin/bash

# Test New Swing Low webhook
curl -X POST http://localhost:8080/webhook/swing/new-low \
  -H "Content-Type: application/json" \
  -d '{
    "ticker": "EUR_USD",
    "exchange": "OANDA",
    "interval": "5",
    "close": "1.08500",
    "low": "1.08250",
    "timenow": "2024-01-15T10:30:00Z"
  }'

echo ""
