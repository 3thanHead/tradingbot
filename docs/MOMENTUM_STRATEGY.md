# Momentum Strategy - Recommended Simple Strategy

The **momentum** strategy is the best simple strategy using the available webhooks. It combines MACD momentum detection with RSI confirmation for reliable entries and smart exits.

## Why This Strategy Works

### 1. Clear Entry Logic
- **MACD crossover** detects momentum shift (leading indicator)
- **RSI crossing 50** confirms the direction is gaining strength
- Both signals required = higher probability trades

### 2. Smart Exit Logic
- **MACD reversal** exits when momentum changes (protect profits)
- **RSI extremes** exits in profit-taking zones (>70 for LONG, <30 for SHORT)
- Two exit options = flexible risk management

### 3. Separate LONG/SHORT
- Different entry/exit conditions for each direction
- Optimized for both bullish and bearish moves
- Symmetric logic = balanced trading

## Strategy Breakdown

### LONG Positions

**Entry:**
1. MACD crosses above signal line (bullish momentum starting)
2. RSI crosses above 50 (confirms upward strength)
→ Open LONG

**Exit (first to trigger):**
- MACD crosses below signal line (momentum reversed - GET OUT)
- RSI crosses above 70 (overbought - TAKE PROFIT)

### SHORT Positions

**Entry:**
1. MACD crosses below signal line (bearish momentum starting)
2. RSI crosses below 50 (confirms downward strength)
→ Open SHORT

**Exit (first to trigger):**
- MACD crosses above signal line (momentum reversed - GET OUT)
- RSI crosses below 30 (oversold - TAKE PROFIT)

## TradingView Setup

### LONG Alerts

1. **Entry Signal 1**: MACD cross up
   - Condition: MACD line crosses above signal line
   - Webhook: `https://your-bot.ngrok-free.app/webhook/macd/cross-up`

2. **Entry Signal 2**: RSI crosses 50 upward
   - Condition: RSI crosses above 50
   - Webhook: `https://your-bot.ngrok-free.app/webhook/rsi/crossed-center`

3. **Exit Signal 1**: MACD cross down
   - Condition: MACD line crosses below signal line
   - Webhook: `https://your-bot.ngrok-free.app/webhook/macd/cross-down`

4. **Exit Signal 2**: RSI overbought
   - Condition: RSI crosses above 70
   - Webhook: `https://your-bot.ngrok-free.app/webhook/rsi/crossed-up`

### SHORT Alerts

1. **Entry Signal 1**: MACD cross down
   - Condition: MACD line crosses below signal line
   - Webhook: `https://your-bot.ngrok-free.app/webhook/macd/cross-down`

2. **Entry Signal 2**: RSI crosses 50 downward
   - Condition: RSI crosses below 50
   - Webhook: `https://your-bot.ngrok-free.app/webhook/rsi/crossed-center`

3. **Exit Signal 1**: MACD cross up
   - Condition: MACD line crosses above signal line
   - Webhook: `https://your-bot.ngrok-free.app/webhook/macd/cross-up`

4. **Exit Signal 2**: RSI oversold
   - Condition: RSI crosses below 30
   - Webhook: `https://your-bot.ngrok-free.app/webhook/rsi/crossed-down`

## Advantages

✅ **Simple to understand** - Only 2 indicators (MACD + RSI)
✅ **Clear signals** - No ambiguity in entry/exit
✅ **Catches trends early** - RSI 50 cross happens before extremes
✅ **Protects profits** - MACD reversal exits before full reversal
✅ **Takes profits** - RSI extremes capture good moves
✅ **Balanced** - Works for both LONG and SHORT equally
✅ **Low maintenance** - Only 4 alerts per direction (8 total)

## Comparison to Other Strategies

| Strategy | Entry | Exit | Complexity | When to Use |
|----------|-------|------|------------|-------------|
| **momentum** | MACD + RSI 50 | MACD reverse OR RSI extreme | ⭐ Simple | **Best all-around** |
| default | MACD + RSI extreme | RSI opposite extreme | Simple | Mean reversion |
| ma_ribbon | MA + MACD | MA reverse | Medium | Strong trends |
| scalping | MACD + RSI 50 | Quick reversals | Simple | Fast markets |

## Recommended Settings

### Position Sizing
```bash
POSITION_SIZE_MODE=margin
POSITION_SIZE_VALUE=100      # $100 margin per trade
```

### Take Profit (optional but recommended)
```bash
TAKE_PROFIT_PIPS=50         # 50 pip target
# OR
TAKE_PROFIT_DOLLARS=150     # $150 profit target
```

### Risk Management
- **Max positions**: 1 (avoid overtrading)
- **Timeframe**: 15min or 1H (good for momentum)
- **Pairs**: Major pairs (EUR/USD, GBP/USD) for tight spreads

## Usage

```bash
# Use momentum strategy
STRATEGY=momentum docker-compose up

# Check logs to verify it loaded
docker logs tradingview-webhook-bot

# Should see:
# ✅ [STRATEGY] Loaded: momentum
# 📊 [LONG] Entry: 2 steps (all), Exit: 2 conditions (any)
# 📊 [SHORT] Entry: 2 steps (all), Exit: 2 conditions (any)
```

## Expected Behavior

**Scenario 1: LONG Trade**
1. MACD crosses up → Bot waits for RSI confirmation
2. RSI crosses 50 → **LONG OPENED**
3. Price moves up, RSI hits 70 → **POSITION CLOSED** (profit taken)

**Scenario 2: Quick Exit**
1. MACD crosses up → Bot waits
2. RSI crosses 50 → **LONG OPENED**
3. MACD crosses down (false signal) → **POSITION CLOSED** (small loss avoided)

**Scenario 3: No Trade**
1. MACD crosses up → Bot waits
2. RSI stays below 50 → No trade opened (momentum not confirmed)

## Tips

- **Use on trending pairs** - Works best when clear momentum exists
- **Avoid during news** - Wait for 30min after major economic releases
- **Check timeframe** - 15min/1H work best; avoid 1min (too noisy)
- **Monitor RSI 50** - Most important confirmation level
- **Trust the exits** - Don't override MACD reversal signals

---

**Bottom line**: This is the simplest profitable strategy using MACD momentum with RSI confirmation. Clean logic, clear signals, smart exits. Perfect starting point for automated trading.
