# 🚀 SIMPLE START GUIDE

## What You Have Now

A **single, simple service** (370 lines of code) that:
- ✅ Receives 8 TradingView webhook events
- ✅ Manages trading state per symbol
- ✅ Opens/closes positions on OANDA
- ✅ Follows your exact pseudo.txt logic

**No microservices. No complexity. Just works.**

---

## 📦 Files Created

```
trader_bot/
├── main.go                    # All logic in ONE file (370 lines)
├── Dockerfile                 # Docker build
├── docker-compose.yml         # Easy deployment
├── .env.example              # Config template
├── README.md                 # Full documentation
├── test-webhooks.sh          # Test script
└── data/
    └── trading_view_event.json  # Webhook format
```

---

## 🎯 Quick Start (3 Steps)

### Step 0: Get ngrok Auth Token
Sign up for free at https://dashboard.ngrok.com/signup and get your auth token.

### Step 1: Configure
```bash
cp .env.example .env
nano .env  # Add OANDA_API_KEY, OANDA_ACCOUNT_ID, and NGROK_AUTHTOKEN
```

### Step 2: Run
```bash
# Default: Starts with ngrok tunnel (public HTTPS URL)
docker-compose up --build

# Wait a few seconds, then get your public URL:
./get-tunnel-url.sh
```

**Bonus:** Visit http://localhost:4040 for ngrok's web UI - see all incoming webhooks in real-time!

**Private mode (local only):**
```bash
docker-compose up trading-bot --build
```

### Step 3: Configure TradingView
Use the URL from `./get-tunnel-url.sh` and create 8 alerts:
- `https://your-ngrok-url.ngrok-free.app/webhook/rsi-greater-than-70`
- `https://your-ngrok-url.ngrok-free.app/webhook/rsi-less-than-30`
- `https://your-ngrok-url.ngrok-free.app/webhook/rsi-moving-down`
- `https://your-ngrok-url.ngrok-free.app/webhook/rsi-moving-up`
- `https://your-ngrok-url.ngrok-free.app/webhook/macd-cross-up`
- `https://your-ngrok-url.ngrok-free.app/webhook/macd-cross-down`
- `https://your-ngrok-url.ngrok-free.app/webhook/macd-moving-up`
- `https://your-ngrok-url.ngrok-free.app/webhook/macd-moving-down`

Use this JSON for ALL alerts:
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

---

## 🧪 Test Without OANDA

```bash
# Start the service
docker-compose up

# In another terminal, run tests
./test-webhooks.sh
```

This will simulate:
1. RSI > 70 event
2. RSI moving down → Triggers SHORT
3. MACD cross up
4. MACD moving up → Closes SHORT

---

## 📊 How It Works (Your Pseudo.txt Logic)

### Opening Positions

**SHORT:**
1. TradingView: RSI > 70 → Sets flag `RSICrossedUp = true`
2. TradingView: RSI moving down → Checks flag + no position → Opens SHORT

**LONG:**
1. TradingView: RSI < 30 → Sets flag `RSICrossedDown = true`
2. TradingView: RSI moving up → Checks flag + no position → Opens LONG

### Closing Positions

**Close SHORT:**
1. TradingView: MACD cross up → Sets flag `MACDCrossedUp = true`
2. TradingView: MACD moving up → Checks flag + SHORT open → Closes position

**Close LONG:**
1. TradingView: MACD cross down → Sets flag `MACDCrossedDown = true`
2. TradingView: MACD moving down → Checks flag + LONG open → Closes position

---

## 🔍 Monitoring

```bash
# View logs
docker-compose logs -f

# Check positions
curl http://localhost:8080/status

# Health check
curl http://localhost:8080/health
```

---

## 🌐 Expose to Internet (for TradingView)

**Option 1: ngrok (Development)**
```bash
ngrok http 8080
# Use the HTTPS URL in TradingView
```

**Option 2: Cloud (Production)**
Deploy to:
- AWS EC2 / ECS
- Google Cloud Run
- DigitalOcean Droplet
- Any VPS with Docker

---

## 🎛️ Customization

### Change Position Size
Edit `main.go` line 228 and 247:
```go
"units": "100",  // Change this number
```

### Switch to Live Trading
Edit `main.go` line 36:
```go
oandaBaseURL = "https://api-fxtrade.oanda.com"
```

⚠️ **Test on practice first!**

---

## 📝 What Each Endpoint Does

| Endpoint | What Happens |
|----------|-------------|
| `/webhook/rsi/greater-than-70` | Sets "RSI is overbought" flag |
| `/webhook/rsi/moving-down` | If RSI was >70, opens SHORT |
| `/webhook/rsi/less-than-30` | Sets "RSI is oversold" flag |
| `/webhook/rsi/moving-up` | If RSI was <30, opens LONG |
| `/webhook/macd/cross-up` | Sets "MACD bullish" flag |
| `/webhook/macd/moving-up` | If MACD crossed up + SHORT open, closes |
| `/webhook/macd/cross-down` | Sets "MACD bearish" flag |
| `/webhook/macd/moving-down` | If MACD crossed down + LONG open, closes |

---

## ✅ Benefits of This Approach

✅ **Simple** - One file, 370 lines  
✅ **Clear** - Matches your pseudo.txt exactly  
✅ **Stateful** - Tracks conditions per symbol  
✅ **Flexible** - Configure alerts in TradingView UI  
✅ **Safe** - Practice mode by default  
✅ **Debuggable** - Easy to add log statements  

---

## 🆘 Troubleshooting

**Bot not opening trades?**
- Check logs: `docker-compose logs -f`
- Verify OANDA credentials in `.env`
- Ensure webhooks are reaching the bot

**Testing locally?**
- Run `./test-webhooks.sh`
- Check `/status` endpoint

**TradingView webhooks failing?**
- Use ngrok or deploy to cloud
- Verify webhook URL is accessible
- Check TradingView alert log

---

## 🎉 You're Done!

**No microservices maze. No over-engineering. Just a simple bot that works.**

Start it. Configure TradingView. Let it trade.

Questions? Check `README.md` or the code comments in `main.go`.
