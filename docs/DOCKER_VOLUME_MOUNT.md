# Docker Volume Mount Fix

## ✅ Issue Fixed

Added volume mount for `strategies/` directory to make strategy JSON files accessible inside the Docker container.

## 🔧 Changes Made

### 1. Updated `docker-compose.yml`

**Added:**
- `STRATEGY` environment variable (defaults to "default")
- Volume mount: `./strategies:/app/strategies:ro`

```yaml
trading-bot:
  environment:
    - STRATEGY=${STRATEGY:-default}  # ← NEW: Strategy selection
  volumes:
    - ./strategies:/app/strategies:ro  # ← NEW: Mount strategies folder
```

### 2. Updated `Dockerfile`

**Fixed working directory** in final stage:
- Changed from `/root/` to `/app`
- Ensures volume mount path matches working directory

```dockerfile
# Final stage
FROM alpine:latest
WORKDIR /app  # ← Changed from /root/
```

## 📁 How It Works

### Directory Structure in Container

```
Container /app/
├── trading-bot           # Binary
└── strategies/           # ← Mounted from host
    ├── README.md
    ├── default.json
    ├── ma_ribbon.json
    └── scalping.json
```

### Host to Container Mapping

```
Host:                           Container:
./strategies/default.json  →    /app/strategies/default.json
./strategies/ma_ribbon.json →   /app/strategies/ma_ribbon.json
./strategies/scalping.json  →   /app/strategies/scalping.json
```

## 🎯 Benefits

### 1. Edit Strategies Without Rebuild
```bash
# Edit strategy on host
nano strategies/my_strategy.json

# Just restart container - no rebuild needed!
docker-compose restart
```

### 2. Easy Strategy Switching
```bash
# In .env file
STRATEGY=ma_ribbon

# Restart
docker-compose restart
```

### 3. Read-Only Mount (`:ro`)
- Protects strategy files from accidental modification inside container
- Best practice for configuration files

## 🧪 Testing

### Verify Volume Mount
```bash
# Start container
docker-compose up -d

# Check mounted files
docker exec tradingview-webhook-bot ls -la /app/strategies/

# Should see:
# default.json
# ma_ribbon.json
# scalping.json
# README.md
```

### Verify Strategy Loading
```bash
# Check logs
docker-compose logs trading-bot | grep STRATEGY

# Should see:
# ✅ [STRATEGY] Loaded: default
# 📊 [STRATEGY] Entry: 2 steps (all_sequential combination)
```

## 📝 Usage Examples

### Use Default Strategy
```bash
# .env file
STRATEGY=default  # or omit - defaults to "default"

docker-compose up -d
```

### Use MA Ribbon Strategy
```bash
# .env file
STRATEGY=ma_ribbon

docker-compose restart
```

### Use Custom Strategy
```bash
# 1. Create strategy on host
cat > strategies/my_strategy.json << 'EOF'
{
  "name": "my_strategy",
  "description": "My custom strategy",
  "entry": {
    "combination": "all_sequential",
    "steps": [
      {"webhook": "/webhook/rsi/crossed-down", "comment": "RSI oversold"},
      {"webhook": "/webhook/macd/cross-up", "comment": "MACD confirms"}
    ]
  },
  "exit": {
    "combination": "any",
    "conditions": [
      {"webhook": "/webhook/macd/cross-down", "is_long": true, "comment": "Exit"}
    ]
  }
}
EOF

# 2. Use it
echo "STRATEGY=my_strategy" >> .env

# 3. Restart
docker-compose restart
```

## ✅ Verification Checklist

- [x] `STRATEGY` environment variable added to docker-compose.yml
- [x] Volume mount `./strategies:/app/strategies:ro` added
- [x] Dockerfile WORKDIR changed to `/app`
- [x] Read-only mount (`:ro`) for security
- [x] Default value set (`${STRATEGY:-default}`)

## 🚀 Result

**Before:** Strategies were in the Docker image, requiring rebuild to change them.

**After:** Strategies are mounted as a volume - edit on host, restart container, done! 🎉

---

**No more rebuilding just to change strategies!** Just edit the JSON file and restart. ✨
