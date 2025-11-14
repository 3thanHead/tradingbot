# Setting Up ngrok Static Domain

## Why Use a Static Domain?

By default, ngrok generates a **random URL** every time you restart (e.g., `https://abc123.ngrok-free.app`). This means:
- ❌ You need to update TradingView webhooks after every restart
- ❌ Lose webhook history/logs when URL changes
- ❌ Manual work to get the new URL

With a **static domain**, you get:
- ✅ Same URL every time you restart
- ✅ Set TradingView webhooks **once** and forget
- ✅ Consistent logging and debugging
- ✅ **FREE** on ngrok free plan!

---

## How to Set Up (5 minutes)

### Step 1: Get Your Free Static Domain

1. Go to [ngrok Dashboard](https://dashboard.ngrok.com)
2. Log in or create a free account
3. Navigate to **"Domains"** in the left menu
4. Click **"Create Domain"** or **"New Domain"**
5. ngrok will assign you a free static domain like:
   ```
   your-trading-bot-12345.ngrok-free.app
   ```
6. Copy this domain name

### Step 2: Add to Your `.env` File

Edit your `.env` file and add:

```bash
NGROK_STATIC_DOMAIN=your-trading-bot-12345.ngrok-free.app
```

**Example `.env`:**
```bash
OANDA_API_KEY=abc123xyz
OANDA_ACCOUNT_ID=101-004-12345
NGROK_AUTHTOKEN=2abc123def456_xyz789
NGROK_STATIC_DOMAIN=my-tradingbot-abc123.ngrok-free.app
MARGIN_AMOUNT=100
```

### Step 3: Restart Docker

```bash
docker-compose down
docker-compose up -d --build
```

### Step 4: Verify

Check the logs to see your static domain:

```bash
docker logs tradingview-webhook-bot
```

You should see:
```
✅ Public ngrok URL: https://your-trading-bot-12345.ngrok-free.app
```

---

## Configure TradingView Webhooks (One Time)

Now you can set your TradingView alerts with **permanent URLs**:

```
https://your-trading-bot-12345.ngrok-free.app/webhook/rsi/crossed-up
https://your-trading-bot-12345.ngrok-free.app/webhook/rsi/crossed-down
https://your-trading-bot-12345.ngrok-free.app/webhook/rsi/moving-up
https://your-trading-bot-12345.ngrok-free.app/webhook/rsi/moving-down
https://your-trading-bot-12345.ngrok-free.app/webhook/macd/cross-up
https://your-trading-bot-12345.ngrok-free.app/webhook/macd/cross-down
https://your-trading-bot-12345.ngrok-free.app/webhook/macd/moving-up
https://your-trading-bot-12345.ngrok-free.app/webhook/macd/moving-down
```

These URLs will **never change** - set them once and you're done! 🎉

---

## Optional: Without Static Domain

If you don't want to set up a static domain, the bot will still work but with a random URL:

1. Just **comment out** the `NGROK_STATIC_DOMAIN` line in `.env`
2. After starting, check logs: `docker logs tradingview-webhook-bot`
3. Copy the generated ngrok URL
4. Update your TradingView webhooks each time you restart

---

## Troubleshooting

### Error: "Domain not found"
- Make sure you've created the domain in ngrok dashboard first
- Check that the domain name is **exactly** as shown in dashboard (no https://, just the domain)

### Error: "Authentication failed"
- Verify your `NGROK_AUTHTOKEN` is correct
- Get a new token from: https://dashboard.ngrok.com/get-started/your-authtoken

### Static domain not working
- Make sure the `.env` file has the variable set (no spaces around `=`)
- Restart docker completely: `docker-compose down && docker-compose up -d --build`

### ngrok Free Plan Limits
- 1 free static domain per account
- 3 endpoints per domain
- Unlimited bandwidth
- If you need more, consider ngrok paid plans ($8/month for Personal)

---

## Testing Your Static Domain

Once set up, test it:

```bash
# Test from your machine
curl https://your-trading-bot-12345.ngrok-free.app/health

# Test a webhook
./scripts/quick-test-usd.sh --margin 100 --url https://your-trading-bot-12345.ngrok-free.app
```

You should get a `200 OK` response! ✅
