# Test Scripts

Collection of scripts to test your TradingView Webhook Trading Bot.

## 📋 Available Scripts

### `test-all-scenarios.sh` 🎯 COMPREHENSIVE
**Purpose:** Test ALL trading scenarios from pseudo.txt with edge cases  
**Usage:**
```bash
./scripts/test-all-scenarios.sh
```
**What it does:**
- **Scenario 1:** RSI > 70 → RSI Moving Down → Open SHORT
- **Scenario 2:** RSI < 30 → RSI Moving Up → Open LONG
- **Scenario 3:** MACD Cross Up → MACD Moving Up → Close SHORT
- **Scenario 4:** MACD Cross Down → MACD Moving Down → Close LONG
- **Edge Cases:**
  - No flag set = No action
  - Position already open = Skip open
  - No position open = Skip close
  - Wrong close signal = Skip close

**Perfect for:** Verifying complete trading logic before going live!

**Example Output:**
```
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
TEST SCENARIO 1: SHORT Position (RSI Overbought)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Step 1.1: Send RSI > 70 event (set RSICrossedUp flag)
Step 1.2: Send RSI Moving Down event (should open SHORT)
Step 1.3: Verify SHORT position opened
Expected: position_open=true, position='short'
```

---

### `test-webhooks.sh`
**Purpose:** Test all webhook endpoints locally  
**Usage:**
```bash
./scripts/test-webhooks.sh
```
**What it does:**
- Tests health and status endpoints
- Simulates RSI > 70 → RSI Moving Down (SHORT position)
- Simulates MACD Cross Up → MACD Moving Up (close SHORT)
- Shows current position state

---

### `test-usd-amount.sh` ⭐
**Purpose:** Test USD notional amount position sizing  
**Usage:**
```bash
./scripts/test-usd-amount.sh
```
**What it does:**
- Triggers RSI > 70 condition
- Opens SHORT position using USD amount calculation
- Shows logs with price fetching and unit conversion
- Demonstrates real-time USD → units conversion

**Example Output:**
```
💱 [CALCULATE] Converting $1000.00 USD to units for EUR_USD
📊 [PRICE] Current price for EUR_USD: 1.08500
✅ [CALCULATE] $1000.00 USD = 921 units at price 1.08500
```

---

### `test-margin-amount.sh` 💰 NEW!
**Purpose:** Test margin-based position sizing  
**Usage:**
```bash
./scripts/test-margin-amount.sh
```
**What it does:**
- Triggers RSI > 70 condition
- Opens SHORT position using margin amount calculation
- Shows margin → position size → units conversion
- Demonstrates leverage-aware trading (50:1)

**Example Output:**
```
💰 [MARGIN] Using margin amount: $100.00
💱 [MARGIN CALC] Margin: $100.00 × Leverage: 50x = Position: $5000.00
📊 [PRICE] Current price: 1.08500
✅ [UNITS] $100.00 margin = 4608 units
```

---

### `quick-test-usd.sh` 🚀
**Purpose:** Quickly test different amounts without editing .env  
**Usage:**
```bash
# Test with margin (default)
./scripts/quick-test-usd.sh 100           # $100 margin (~$5000 position with 50:1 leverage)
./scripts/quick-test-usd.sh --margin 50   # $50 margin (~$2500 position)

# Test with USD notional
./scripts/quick-test-usd.sh --usd 1000    # $1000 notional value
./scripts/quick-test-usd.sh --usd 2500    # $2500 notional value
```
**What it does:**
- Automatically restarts bot with specified amount
- Runs the appropriate test (margin or USD)
- Shows unit calculation in real-time
- Tells you how to restore .env settings

**Perfect for:** Testing different position sizes quickly!

**Margin vs USD:**
- `--margin 100` = $100 margin × 50 leverage = ~$5000 position
- `--usd 5000` = exactly $5000 position (no leverage multiplier)

---

### `test-with-payload.sh`
**Purpose:** Test with custom JSON payload file  
**Usage:**
```bash
# Test locally
./scripts/test-with-payload.sh

# Test with ngrok tunnel
./scripts/test-with-payload.sh https://your-ngrok-url.ngrok-free.app
```
**What it does:**
- Uses `test-webhook-payload.json` for all tests
- Tests all 8 webhook endpoints + health/status
- Can test locally or remotely via ngrok

---

### `test-ngrok.sh`
**Purpose:** Test ngrok tunnel connectivity  
**Usage:**
```bash
# Start bot first: docker-compose up
./scripts/test-ngrok.sh
```
**What it does:**
- Fetches your ngrok public URL
- Tests health endpoint through tunnel
- Tests a sample webhook through tunnel
- Verifies end-to-end connectivity

---

### `get-tunnel-url.sh`
**Purpose:** Display current ngrok tunnel URL  
**Usage:**
```bash
./scripts/get-tunnel-url.sh
```
**What it does:**
- Shows your current ngrok public URL
- Lists all webhook endpoint URLs
- Reminds you about the ngrok web UI (http://localhost:4040)

---

## 🚀 Quick Start Testing Workflow

**1. Start the bot:**
```bash
docker-compose up --build
```

**2. Get your tunnel URL:**
```bash
./scripts/get-tunnel-url.sh
```

**3. Test USD amount feature:**
```bash
./scripts/test-usd-amount.sh
```

**4. View logs:**
```bash
docker logs -f tradingview-webhook-bot
```

---

## 📝 Configuration Files

### `test-webhook-payload.json`
Sample TradingView webhook payload used by `test-with-payload.sh`.

**Format:**
```json
{
  "ticker": "EUR_USD",
  "exchange": "OANDA",
  "interval": "15",
  "close": "1.0850",
  "open": "1.0845",
  "high": "1.0855",
  "low": "1.0840",
  "volume": "1000",
  "time": "2024-01-01T12:00:00Z",
  "timenow": "2024-01-01T12:00:00Z"
}
```

Edit this file to test with different tickers or values.

---

## 💡 Tips

- **Always check logs** after running tests to see detailed execution flow
- **Use ngrok web UI** (http://localhost:4040) to inspect webhook requests
- **Test USD amount** before going live to verify calculations
- **Run tests in sequence** to avoid race conditions with state management
