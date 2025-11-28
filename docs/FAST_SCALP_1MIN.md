# Fast Scalp 1-Minute Strategy

## Overview
High-frequency scalping strategy designed for 1-minute charts using Fast MACD, RSI, and EMA trend filters.

## Strategy File
`strategies/fast_scalp_1min.json`

## Indicators Required (TradingView)
- **EMA(9)** - Fast exit trigger
- **EMA(200)** - Long-term trend filter
- **Fast MACD(5,13,5)** - Histogram zero-line crosses
- **RSI(7)** - Fast momentum oscillator

## Entry Conditions

### LONG Entry (ALL required)
1. Price crosses UP through EMA 200
2. MACD histogram crosses UP through zero
3. RSI crosses UP through 60

### SHORT Entry (ALL required)
1. Price crosses DOWN through EMA 200
2. MACD histogram crosses DOWN through zero
3. RSI crosses DOWN through 40

## Exit Conditions

### LONG Exit (ANY triggers exit)
1. MACD histogram crosses DOWN through zero
2. Price crosses DOWN through EMA 9
3. RSI crosses UP through 80 (overbought exit)

### SHORT Exit (ANY triggers exit)
1. MACD histogram crosses UP through zero
2. Price crosses UP through EMA 9
3. RSI crosses DOWN through 20 (oversold exit)

## Active Webhook Endpoints (11 total)

### EMA Crosses (4)
- `POST /webhook/ema/price-cross-up-9`
- `POST /webhook/ema/price-cross-down-9`
- `POST /webhook/ema/price-cross-up-200`
- `POST /webhook/ema/price-cross-down-200`

### MACD Histogram Zero Crosses (2)
- `POST /webhook/macd/histogram-cross-up-0`
- `POST /webhook/macd/histogram-cross-down-0`

### RSI Directional Crosses (5)
- `POST /webhook/rsi/cross-down-20` (SHORT exit)
- `POST /webhook/rsi/cross-down-40` (SHORT entry)
- `POST /webhook/rsi/cross-up-60` (LONG entry)
- `POST /webhook/rsi/cross-down-60` (not used in current strategy)
- `POST /webhook/rsi/cross-up-80` (LONG exit)

## TradingView Alert Setup

### 1. Chart Setup (1-minute timeframe)
```
Add Indicators:
- EMA(9)
- EMA(200)
- MACD(5,13,5) - Set to "Fast" mode
- RSI(7)
```

### 2. Create 11 Alerts

**LONG Entry Alerts:**
```
Alert 1: Price > EMA(200)
Condition: Crossing Up
URL: {{ngrok_url}}/webhook/ema/price-cross-up-200
Body: {"ticker":"{{ticker}}","close":{{close}},"strategy":"fast_scalp_1min"}

Alert 2: MACD Histogram > 0
Condition: Crossing Up
URL: {{ngrok_url}}/webhook/macd/histogram-cross-up-0
Body: {"ticker":"{{ticker}}","close":{{close}},"strategy":"fast_scalp_1min"}

Alert 3: RSI(7) > 60
Condition: Crossing Up
URL: {{ngrok_url}}/webhook/rsi/cross-up-60
Body: {"ticker":"{{ticker}}","close":{{close}},"strategy":"fast_scalp_1min"}
```

**LONG Exit Alerts:**
```
Alert 4: MACD Histogram < 0
Condition: Crossing Down
URL: {{ngrok_url}}/webhook/macd/histogram-cross-down-0
Body: {"ticker":"{{ticker}}","close":{{close}},"strategy":"fast_scalp_1min"}

Alert 5: Price < EMA(9)
Condition: Crossing Down
URL: {{ngrok_url}}/webhook/ema/price-cross-down-9
Body: {"ticker":"{{ticker}}","close":{{close}},"strategy":"fast_scalp_1min"}

Alert 6: RSI(7) > 80
Condition: Crossing Up
URL: {{ngrok_url}}/webhook/rsi/cross-up-80
Body: {"ticker":"{{ticker}}","close":{{close}},"strategy":"fast_scalp_1min"}
```

**SHORT Entry Alerts:**
```
Alert 7: Price < EMA(200)
Condition: Crossing Down
URL: {{ngrok_url}}/webhook/ema/price-cross-down-200
Body: {"ticker":"{{ticker}}","close":{{close}},"strategy":"fast_scalp_1min"}

Alert 8: MACD Histogram < 0
Condition: Crossing Down
URL: {{ngrok_url}}/webhook/macd/histogram-cross-down-0
Body: {"ticker":"{{ticker}}","close":{{close}},"strategy":"fast_scalp_1min"}

Alert 9: RSI(7) < 40
Condition: Crossing Down
URL: {{ngrok_url}}/webhook/rsi/cross-down-40
Body: {"ticker":"{{ticker}}","close":{{close}},"strategy":"fast_scalp_1min"}
```

**SHORT Exit Alerts:**
```
Alert 10: MACD Histogram > 0
Condition: Crossing Up
URL: {{ngrok_url}}/webhook/macd/histogram-cross-up-0
Body: {"ticker":"{{ticker}}","close":{{close}},"strategy":"fast_scalp_1min"}

Alert 11: Price > EMA(9)
Condition: Crossing Up
URL: {{ngrok_url}}/webhook/ema/price-cross-up-9
Body: {"ticker":"{{ticker}}","close":{{close}},"strategy":"fast_scalp_1min"}

Alert 12: RSI(7) < 20
Condition: Crossing Down
URL: {{ngrok_url}}/webhook/rsi/cross-down-20
Body: {"ticker":"{{ticker}}","close":{{close}},"strategy":"fast_scalp_1min"}
```

## Environment Variables
```bash
export STRATEGY_NAME=fast_scalp_1min
export OANDA_API_KEY=your_key
export OANDA_ACCOUNT_ID=your_account
export OANDA_BASE_URL=https://api-fxpractice.oanda.com
export TRADE_AMOUNT_USD=100
export RISK_PERCENT=1.0
export STOP_LOSS_PIPS=10
```

## Running the Strategy

### Docker Compose
```bash
# Update docker-compose.yml with STRATEGY_NAME=fast_scalp_1min
docker-compose up --build
```

### Get Webhook URL
```bash
./scripts/get-tunnel-url.sh
```

## Risk Management

### Position Sizing
- Uses `TRADE_AMOUNT_USD` environment variable
- Automatic leverage calculation based on OANDA limits
- Default: $100 per trade

### Stop Loss
- Set via `STOP_LOSS_PIPS` (default: 10 pips)
- Converts to price distance based on instrument

### Exit Strategy
- **Fast Exit**: EMA 9 cross (quick trend reversal)
- **Zero Line**: MACD histogram crosses zero (momentum shift)
- **Extreme Exit**: RSI 80 (LONG) / RSI 20 (SHORT) - profit taking

## Strategy Characteristics

### Timeframe
- **1-minute chart** for high-frequency trades
- Expect multiple trades per hour during volatile periods

### Best Market Conditions
- **Trending markets**: Strong EMA 200 trend filter
- **High volatility**: Fast MACD catches quick momentum shifts
- **Clear breakouts**: RSI confirms strength

### Avoid
- **Range-bound markets**: Will get whipsawed
- **Low volatility periods**: Too many false signals
- **Major news events**: Wait for dust to settle

## Performance Tips

1. **Use on liquid pairs**: EUR/USD, GBP/USD, USD/JPY
2. **Trade during active sessions**: London/NY overlap
3. **Monitor win rate**: Should be 50%+ with 1.5:1 RR minimum
4. **Adjust RSI(7)**: Can change to RSI(9) for less sensitivity
5. **Swing trades enabled**: Bot will reverse positions automatically

## Backtesting Recommended
- Paper trade for 1-2 weeks first
- Track win rate, average RR, max drawdown
- Adjust `STOP_LOSS_PIPS` based on pair volatility
- Consider adding ATR-based dynamic stops

## Code Reference
- Strategy logic: `shouldOpenPosition()` in `main.go`
- Exit logic: `shouldExitPosition()` in `main.go`
- State management: `PositionState` struct
- Webhook handlers: Lines 2000-2600 in `main.go`
