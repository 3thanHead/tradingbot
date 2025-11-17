# Test Scripts

Scripts to test webhook endpoints and trading logic.

## Main Scripts

**`test-all-scenarios.sh`** - Test complete trading scenarios (LONG/SHORT entry/exit)
```bash
./scripts/test-all-scenarios.sh
```

**`test-webhooks.sh`** - Test all webhook endpoints locally
```bash
./scripts/test-webhooks.sh
```

**`test-ngrok.sh`** - Test ngrok tunnel connectivity
```bash
./scripts/test-ngrok.sh
```

**`get-tunnel-url.sh`** - Display current ngrok URL
```bash
./scripts/get-tunnel-url.sh
```

## Position Sizing Tests

**`test-margin-amount.sh`** - Test margin-based sizing
```bash
./scripts/test-margin-amount.sh
```

**`test-usd-amount.sh`** - Test USD notional sizing  
```bash
./scripts/test-usd-amount.sh
```

**`quick-test-usd.sh`** - Test different amounts quickly
```bash
./scripts/quick-test-usd.sh 100          # $100 margin
./scripts/quick-test-usd.sh --usd 1000   # $1000 notional
```

## Quick Start

```bash
# 1. Start bot
docker-compose up --build

# 2. Test complete trading logic
./scripts/test-all-scenarios.sh

# 3. Check logs
docker logs -f tradingview-webhook-bot
```
