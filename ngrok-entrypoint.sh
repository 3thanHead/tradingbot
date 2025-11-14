#!/bin/sh

# ngrok entrypoint - supports optional static domain

if [ -n "$NGROK_STATIC_DOMAIN" ]; then
    echo "🔗 Starting ngrok with static domain: $NGROK_STATIC_DOMAIN"
    exec ngrok http trading-bot:8080 --domain="$NGROK_STATIC_DOMAIN" --config=/etc/ngrok.yml
else
    echo "🔗 Starting ngrok with random domain"
    exec ngrok http trading-bot:8080 --config=/etc/ngrok.yml
fi
