# TradingView Webhook Trading Bot

A **simple, single-service** trading bot that receives TradingView webhook alerts and executes trades on OANDA based on RSI and MACD indicators.

## 🎯 How It Works

```
TradingView Alert → Webhook → State Machine → OANDA Trade
```

### Trading Logic (from pseudo.txt)

1. **RSI > 70** (Overbought) + **RSI Moving Down** → Open SHORT
2. **RSI < 30** (Oversold) + **RSI Moving Up** → Open LONG  
3. **MACD Cross Up** + **MACD Moving Up** + Short Position Open → Close SHORT
4. **MACD Cross Down** + **MACD Moving Down** + Long Position Open → Close LONG

## 🚀 Quick Start

### 1. Setup Environment

Get a free ngrok auth token from https://dashboard.ngrok.com/signup

**Optional but RECOMMENDED:** Get a [free static domain](https://dashboard.ngrok.com/domains) so your webhook URL never changes! See [NGROK_STATIC_DOMAIN.md](NGROK_STATIC_DOMAIN.md) for setup.

```bash
cp .env.example .env
nano .env  # Add your OANDA credentials AND ngrok auth token
```

**With Static Domain (Recommended):**
```bash
NGROK_STATIC_DOMAIN=your-bot-12345.ngrok-free.app
```

This keeps the same URL forever - set TradingView webhooks once and forget! 🎉

### 2. Run with Docker

**Default: With ngrok Tunnel (for TradingView webhooks)**
```bash
docker-compose up --build
```

Wait a few seconds, then get your public URL:
```bash
./get-tunnel-url.sh
```

This displays your HTTPS URL (e.g., `https://abc-123-def.ngrok-free.app`) to use in TradingView alerts.

**Bonus:** Visit http://localhost:4040 to see ngrok's web UI with real-time webhook requests, replay features, and debugging tools!

**Private Mode: Local Only (no tunnel)**
```bash
docker-compose up trading-bot --build
```

Only starts the bot on `localhost:8080` without public exposure.

**Get your tunnel URL anytime:**
```bash
./scripts/get-tunnel-url.sh
```

### 3. Configure TradingView Alerts

Set up **8 separate alerts** in TradingView:

#### RSI Alerts (4 alerts)

**Alert 1: RSI > 70**
- Webhook URL: `http://your-server:8080/webhook/rsi/greater-than-70`
- Message:
```json
{
  "ticker": "{{ticker}}",
  "exchange": "{{exchange}}",
  "interval": "{{interval}}",
  "close": "{{close}}",
  "open": "{{open}}",
  "high": "{{high}}",
  "low": "{{low}}",
  "volume": "{{volume}}",
  "time": "{{time}}",
  "timenow": "{{timenow}}"
}
```

**Alert 2: RSI < 30**
- Webhook URL: `http://your-server:8080/webhook/rsi/less-than-30`
- Message: Same JSON as above

**Alert 3: RSI Moving Down**
- Webhook URL: `http://your-server:8080/webhook/rsi/moving-down`
- Message: Same JSON as above

**Alert 4: RSI Moving Up**
- Webhook URL: `http://your-server:8080/webhook/rsi/moving-up`
- Message: Same JSON as above

#### MACD Alerts (4 alerts)

**Alert 5: MACD Cross Above Zero**
- Webhook URL: `http://your-server:8080/webhook/macd/cross-up`
- Message: Same JSON format

**Alert 6: MACD Cross Below Zero**
- Webhook URL: `http://your-server:8080/webhook/macd/cross-down`
- Message: Same JSON format

**Alert 7: MACD Moving Up**
- Webhook URL: `http://your-server:8080/webhook/macd/moving-up`
- Message: Same JSON format

**Alert 8: MACD Moving Down**
- Webhook URL: `http://your-server:8080/webhook/macd/moving-down`
- Message: Same JSON format

## 📡 API Endpoints

### Webhook Endpoints (POST)

| Endpoint | Event | Purpose |
|----------|-------|---------|
| `/webhook/rsi/greater-than-70` | RSI > 70 | Set overbought condition |
| `/webhook/rsi/less-than-30` | RSI < 30 | Set oversold condition |
| `/webhook/rsi/moving-down` | RSI ↓ | Trigger SHORT if RSI was > 70 |
| `/webhook/rsi/moving-up` | RSI ↑ | Trigger LONG if RSI was < 30 |
| `/webhook/macd/cross-up` | MACD crosses up | Set bullish condition |
| `/webhook/macd/cross-down` | MACD crosses down | Set bearish condition |
| `/webhook/macd/moving-up` | MACD ↑ | Close SHORT if MACD crossed up |
| `/webhook/macd/moving-down` | MACD ↓ | Close LONG if MACD crossed down |

### Monitoring Endpoints (GET)

| Endpoint | Purpose |
|----------|---------|
| `/health` | Health check |
| `/status` | View all positions and states |

## 🧪 Testing

### Test Webhook Locally

```bash
# Test RSI > 70
curl -X POST http://localhost:8080/webhook/rsi/greater-than-70 \
  -H "Content-Type: application/json" \
  -d '{
    "ticker": "EURUSD",
    "exchange": "OANDA",
    "close": "1.0850"
  }'

# Check status
curl http://localhost:8080/status
```

### Expose to Internet (for TradingView)

**Development (ngrok):**
```bash
ngrok http 8080
# Use the HTTPS URL in TradingView
```

**Production:**
Deploy to cloud and use public IP/domain

## 📊 Example Flow

### Opening a SHORT Position

1. TradingView sends: `POST /webhook/rsi/greater-than-70`
   - Bot sets flag: `RSICrossedUp = true`
   
2. TradingView sends: `POST /webhook/rsi/moving-down`
   - Bot checks: RSI was > 70 AND no position open
   - Bot executes: Open SHORT position via OANDA
   - Bot updates: `position_open = true`, `position = "short"`

### Closing a SHORT Position

3. TradingView sends: `POST /webhook/macd/cross-up`
   - Bot sets flag: `MACDCrossedUp = true`
   
4. TradingView sends: `POST /webhook/macd/moving-up`
   - Bot checks: MACD crossed up AND SHORT position is open
   - Bot executes: Close position via OANDA
   - Bot updates: `position_open = false`

## 🔧 Configuration

### Position Sizing

Choose one of three methods in your `.env` file:

**Option 1: Margin-Based (Recommended)**
```bash
MARGIN_AMOUNT=100  # $100 margin, leverage applied automatically
```

**Option 2: USD Amount**
```bash
TRADE_USD_AMOUNT=1000  # $1000 position size
```

**Option 3: Fixed Units**
```bash
TRADE_UNITS=1000  # 1000 units
```

See [MARGIN_AMOUNT.md](MARGIN_AMOUNT.md) for detailed explanation.

### Take Profit

Set automatic take profit in pips or percentage:

**Pips:**
```bash
TAKE_PROFIT_PIPS=50  # 50 pip take profit
```

**Percentage:**
```bash
TAKE_PROFIT_PCT=2.5  # 2.5% gain take profit
```

See [TAKE_PROFIT.md](TAKE_PROFIT.md) for detailed examples and configuration.

### Switch to Live Trading

Edit `main.go`:
```go
oandaBaseURL = "https://api-fxtrade.oanda.com"  // Change from api-fxpractice
```

⚠️ **Test thoroughly on practice account first!**

## 📁 Project Structure

```
trader_bot/
├── main.go              # Single service with all logic
├── Dockerfile           # Docker build
├── docker-compose.yml   # Easy deployment
├── .env.example         # Environment template
├── data/
│   └── trading_view_event.json  # Webhook payload format
└── pseudo.txt           # Original trading logic
```

## 🔒 Security

- Never commit `.env` file
- Use HTTPS webhooks in production
- Validate webhook payloads
- Start with small position sizes
- Monitor the bot closely

## 📝 Logs

```bash
# View logs
docker-compose logs -f

# You'll see:
# 📊 RSI > 70 for EUR_USD
# 🟢 Opening LONG position for EUR_USD at 1.0850
# ✅ LONG position opened: EUR_USD (ID: 1234)
```

## ⚠️ Disclaimer

Trading involves risk. This bot is for educational purposes. Test on OANDA practice account before live trading.

---

**Simple. Clean. Effective.** 🎯
