# Quick Setup: ngrok Static Domain

## ✅ Benefits
- Same webhook URL every restart
- Set TradingView alerts **once**
- **FREE** on ngrok free plan

## 🚀 Setup (3 steps)

### 1. Get Free Domain
1. Go to https://dashboard.ngrok.com/domains
2. Click "Create Domain"
3. Copy your domain (e.g., `my-bot-abc123.ngrok-free.app`)

### 2. Add to `.env`
```bash
NGROK_STATIC_DOMAIN=my-bot-abc123.ngrok-free.app
```

### 3. Restart
```bash
docker-compose down
docker-compose up -d --build
```

## ✨ Done!

Your webhook URLs are now permanent:
```
https://my-bot-abc123.ngrok-free.app/webhook/rsi/crossed-up
https://my-bot-abc123.ngrok-free.app/webhook/rsi/crossed-down
... etc
```

Set them in TradingView **once** and never change them again! 🎉

---

**See [NGROK_STATIC_DOMAIN.md](NGROK_STATIC_DOMAIN.md) for detailed guide**
