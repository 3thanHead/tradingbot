# TradingView Webhook Trading Bot

A **simple, single-service** trading bot that receives TradingView webhook alerts and executes trades on OANDA based on RSI and MACD indicators.

## 🎯 How It Works

```
TradingView Alert → Webhook → Strategy System → OANDA Trade
```

### 🎨 Flexible Strategy System

**NEW:** The bot now uses a **JSON-based strategy system** that lets you define custom entry and exit conditions without changing code!

Choose from built-in strategies or create your own:
- **`default`** - Original bot behavior (RSI extremes + MACD confirmation)
- **`ma_ribbon`** - MA ribbon trend-following strategy
- **`scalping`** - Fast scalping with tight exits

```bash
# Set in .env file
STRATEGY=default  # or ma_ribbon, scalping, or your-custom-strategy
```

📚 **Learn More:**
- [Strategy Quick Start](docs/STRATEGY_QUICK_START.md) - Create your first strategy in 5 minutes
- [Webhook Strategy System](docs/WEBHOOK_STRATEGY_SIMPLIFICATION.md) - How it works
- [strategies/](strategies/) - View and modify example strategies
- [Complete Documentation](docs/) - All guides and references

### Default Trading Logic

The `default` strategy preserves the original behavior from pseudo.txt:

**Entry (Sequential - both conditions required):**
1. **RSI < 30** (Oversold) → Set condition
2. **MACD Cross Up** → Open LONG (if RSI was < 30)

**OR**

1. **RSI > 70** (Overbought) → Set condition  
2. **MACD Cross Down** → Open SHORT (if RSI was > 70)

**Exit (Any condition triggers close):**
- **RSI Center Double-Cross** - RSI crosses 50 twice in same direction
- **MACD Reversal** - MACD crosses opposite direction
- **RSI Extreme After Warning** - RSI hits opposite extreme after center cross

## 🚀 Quick Start

### 1. Setup Environment

Get a free ngrok auth token from https://dashboard.ngrok.com/signup

**Optional but RECOMMENDED:** Get a [free static domain](https://dashboard.ngrok.com/domains) so your webhook URL never changes! See [docs/NGROK_STATIC_DOMAIN.md](docs/NGROK_STATIC_DOMAIN.md) for setup.

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

Set up **7 webhook alerts** in TradingView (8 if using MA ribbon strategy):

#### RSI Alerts (3 alerts)

**Alert 1: RSI Crossed Above 70**
- Condition: `RSI crosses above 70`
- Webhook URL: `https://your-domain.ngrok-free.app/webhook/rsi/crossed-up`
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

**Alert 2: RSI Crossed Below 30**
- Condition: `RSI crosses below 30`
- Webhook URL: `https://your-domain.ngrok-free.app/webhook/rsi/crossed-down`
- Message: Same JSON as above

**Alert 3: RSI Crossed 50 (Center)**
- Condition: `RSI crosses 50` (both directions)
- Webhook URL: `https://your-domain.ngrok-free.app/webhook/rsi/crossed-center`
- Message: Same JSON as above

#### MACD Alerts (2 alerts)

**Alert 4: MACD Line Crossed Above Signal**
- Condition: `MACD line crosses above signal line`
- Webhook URL: `https://your-domain.ngrok-free.app/webhook/macd/cross-up`
- Message: Same JSON format

**Alert 5: MACD Line Crossed Below Signal**
- Condition: `MACD line crosses below signal line`
- Webhook URL: `https://your-domain.ngrok-free.app/webhook/macd/cross-down`
- Message: Same JSON format

#### MA Ribbon Alerts (2 alerts - OPTIONAL, for `ma_ribbon` strategy)

**Alert 6: All MAs Bullish Aligned**
- Condition: `MA(5) > MA(10) > MA(20) > MA(50) > MA(100)`
- Webhook URL: `https://your-domain.ngrok-free.app/webhook/ma/ribbon-bullish`
- Message: Same JSON format

**Alert 7: All MAs Bearish Aligned**
- Condition: `MA(5) < MA(10) < MA(20) < MA(50) < MA(100)`
- Webhook URL: `https://your-domain.ngrok-free.app/webhook/ma/ribbon-bearish`
- Message: Same JSON format

## 📡 API Endpoints

### Webhook Endpoints (POST)

| Endpoint | Event | Strategy Condition |
|----------|-------|-------------------|
| `/webhook/rsi/crossed-up` | RSI crosses above 70 | `rsi_crossed_up` |
| `/webhook/rsi/crossed-down` | RSI crosses below 30 | `rsi_crossed_down` |
| `/webhook/rsi/crossed-center` | RSI crosses 50 (both ways) | `rsi_crossed_center` |
| `/webhook/macd/cross-up` | MACD line crosses above signal | `macd_cross_up` |
| `/webhook/macd/cross-down` | MACD line crosses below signal | `macd_cross_down` |
| `/webhook/ma/ribbon-bullish` | All MAs bullish aligned | `ma_ribbon_bullish` |
| `/webhook/ma/ribbon-bearish` | All MAs bearish aligned | `ma_ribbon_bearish` |

**Note:** The bot's behavior depends on your active strategy (see [Strategy System](#-flexible-strategy-system))

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

Set automatic take profit using pips, dollars, or percentage:

**Pips (Highest Priority):**
```bash
TAKE_PROFIT_PIPS=50  # 50 pip take profit
```

**Dollar Amount:**
```bash
TAKE_PROFIT_DOLLARS=100  # $100 profit target
```

**Percentage (Lowest Priority):**
```bash
TAKE_PROFIT_PCT=2.5  # 2.5% gain take profit
```

See [docs/TAKE_PROFIT.md](docs/TAKE_PROFIT.md) for detailed examples and configuration.

### Strategy Selection

Choose your trading strategy:

```bash
STRATEGY=default  # default, ma_ribbon, scalping, or custom
```

See [Strategy System Guide](STRATEGY_SYSTEM.md) for creating custom strategies.

### Switch to Live Trading

Edit `main.go`:
```go
oandaBaseURL = "https://api-fxtrade.oanda.com"  // Change from api-fxpractice
```

⚠️ **Test thoroughly on practice account first!**

## 📁 Project Structure

```
trader_bot/
├── main.go                      # Core trading bot with strategy system
├── Dockerfile                   # Docker build
├── docker-compose.yml           # Easy deployment
├── .env.example                 # Environment template
├── strategies/                  # Trading strategy definitions
│   ├── README.md               # Strategy directory guide
│   ├── default.json            # Original bot behavior
│   ├── ma_ribbon.json          # MA ribbon trend-following
│   └── scalping.json           # Fast scalping strategy
├── docs/                        # Complete documentation
│   ├── README.md               # Documentation index
│   ├── STRATEGY_QUICK_START.md # 5-minute strategy guide
│   ├── WEBHOOK_STRATEGY_SIMPLIFICATION.md # Strategy system explanation
│   ├── TAKE_PROFIT.md          # Take profit configuration
│   ├── NGROK_STATIC_DOMAIN.md  # Static domain setup
│   ├── TRADINGVIEW_ALERTS.md   # TradingView setup guide
│   ├── CHANGELOG.md            # Version history
│   ├── MIGRATION_TO_V2.md      # Upgrade guide
│   └── ... (more guides)
├── data/
│   └── trading_view_event.json # Webhook payload format
└── pseudo.txt                   # Original trading logic
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
