# TradingView Trading Bot

Receives TradingView webhook alerts and executes trades on OANDA using JSON-based strategies.

## Quick Start

```bash
# 1. Setup environment
cp .env.example .env
nano .env  # Add OANDA credentials and ngrok auth token

# 2. Run
docker-compose up --build

# 3. Get webhook URL
./get-tunnel-url.sh  # Use this URL in TradingView alerts
```

## Strategy System

Edit JSON files to change trading logic without coding:

```bash
STRATEGY=momentum docker-compose up       # ⭐ RECOMMENDED - Best simple strategy
STRATEGY=default docker-compose up        # MACD + RSI extremes
STRATEGY=ma_ribbon docker-compose up      # Trend following
STRATEGY=scalping docker-compose up       # Fast trading
```

**Momentum Strategy** (recommended):
- Entry: MACD crossover + RSI crosses 50 (momentum confirmation)
- Exit: MACD reversal OR RSI extremes (smart profit-taking)
- See **[docs/MOMENTUM_STRATEGY.md](docs/MOMENTUM_STRATEGY.md)** for details

See **[strategies/README.md](strategies/README.md)** for creating custom strategies.

## Environment Variables

```bash
# OANDA (required)
OANDA_ACCOUNT_ID=xxx
OANDA_API_TOKEN=xxx
OANDA_ENVIRONMENT=practice  # or live

# Trading (required)
SYMBOL=EUR_USD
POSITION_SIZE_MODE=margin      # margin, usd, or units  
POSITION_SIZE_VALUE=100        # Amount based on mode
MAX_POSITIONS=1

# Strategy (optional)
STRATEGY=default               # default, ma_ribbon, scalping, or custom

# Take Profit (optional - choose one)
TAKE_PROFIT_PIPS=50           # Set TP in pips
TAKE_PROFIT_DOLLARS=100       # Set TP in dollars
TAKE_PROFIT_PCT=2.5           # Set TP in percentage

# Trading Hours (optional - restrict when trades can be opened)
TRADING_START_HOUR=9          # Start time: "9" or "9:30" (supports minutes)
TRADING_END_HOUR=17           # End time: "17" or "16:45" (supports minutes)
TRADING_DAYS=1,2,3,4,5        # Days to trade (0=Sun..6=Sat) or mon,tue,wed,thu,fri
TRADING_TIMEZONE=America/New_York  # Timezone for trading hours (or use TIMEZONE_OFFSET)
TIMEZONE_OFFSET=-5            # UTC offset if TRADING_TIMEZONE not set

# Ngrok (required for webhooks)
NGROK_AUTHTOKEN=your_token
NGROK_STATIC_DOMAIN=your-bot.ngrok-free.app  # Optional but recommended

# Server
PORT=8080
```

## Available Webhooks

Set these in TradingView alerts (use URL from `./get-tunnel-url.sh`):

```
https://your-bot.ngrok-free.app/webhook/rsi/crossed-up
https://your-bot.ngrok-free.app/webhook/rsi/crossed-down
https://your-bot.ngrok-free.app/webhook/rsi/crossed-center
https://your-bot.ngrok-free.app/webhook/macd/cross-up
https://your-bot.ngrok-free.app/webhook/macd/cross-down
https://your-bot.ngrok-free.app/webhook/ma/ribbon-bullish
https://your-bot.ngrok-free.app/webhook/ma/ribbon-bearish
```

## Documentation

- **[strategies/README.md](strategies/README.md)** - Create and edit trading strategies
- **[docs/TRADINGVIEW_ALERTS.md](docs/TRADINGVIEW_ALERTS.md)** - TradingView webhook setup
- **[docs/TRADING_HOURS.md](docs/TRADING_HOURS.md)** - Trading hours and days restrictions
- **[docs/TAKE_PROFIT.md](docs/TAKE_PROFIT.md)** - Take profit configuration
- **[docs/NGROK_STATIC_DOMAIN.md](docs/NGROK_STATIC_DOMAIN.md)** - Static domain setup
- **[docs/CHANGELOG.md](docs/CHANGELOG.md)** - Version history

## Architecture

```
┌─────────────────┐
│  TradingView    │
│    Alerts       │
└────────┬────────┘
         │ HTTPS Webhook
         ▼
┌─────────────────┐
│  Ngrok Tunnel   │
└────────┬────────┘
         │
         ▼
┌─────────────────┐      ┌──────────────────┐
│   Go Trading    │◄────►│ Strategy JSON    │
│      Bot        │      │ (strategies/*.json)│
└────────┬────────┘      └──────────────────┘
         │
         │ OANDA API
         ▼
┌─────────────────┐
│  OANDA Broker   │
│   (Live/Demo)   │
└─────────────────┘
```

## License

MIT
