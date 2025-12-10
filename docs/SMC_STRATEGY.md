# Smart Money Concept (SMC) Structure Strategy

This strategy uses Smart Money Concept market structure breaks to identify high-probability entry and exit points.

## Strategy Overview

The SMC strategy identifies market structure patterns:
- **LL (Lower Low)** - Bearish continuation, but potential bullish reversal point
- **HL (Higher Low)** - Bullish continuation 
- **LH (Lower High)** - Bearish continuation
- **HH (Higher High)** - Bullish continuation, but potential bearish reversal point

## Entry Conditions

### LONG Entry (ANY of these):
1. **SMC Lower Low (LL)** - Enter at potential bullish reversal
2. **SMC Higher Low (HL)** - Enter on bullish continuation

### SHORT Entry (ANY of these):
1. **SMC Higher High (HH)** - Enter at potential bearish reversal  
2. **SMC Lower High (LH)** - Enter on bearish continuation

## Exit Conditions

### LONG Exit (ANY of these):
1. **SMC Lower High (LH)** - Exit when bearish structure forms
2. **SMC Higher High (HH)** - Take profit at structure high

### SHORT Exit (ANY of these):
1. **SMC Higher Low (HL)** - Exit when bullish structure forms
2. **SMC Lower Low (LL)** - Take profit at structure low

## TradingView Setup

### 1. Use SMC Indicator
Add a Smart Money Concept indicator to your TradingView chart (like "CRR - Smart Money Concept ES (Pro Expo)" or similar).

### 2. Create Alerts

Set up alerts for each structure point:

#### For LONG Entry:
**Alert 1: Lower Low (LL)**
- Webhook URL: `https://your-bot.ngrok-free.app/webhook/smc/low-low`
- Message:
```json
{
  "ticker": "EUR_USD",
  "exchange": "OANDA",
  "close": "{{close}}"
}
```

**Alert 2: Higher Low (HL)**
- Webhook URL: `https://your-bot.ngrok-free.app/webhook/smc/high-low`
- Message:
```json
{
  "ticker": "EUR_USD",
  "exchange": "OANDA",
  "close": "{{close}}"
}
```

#### For LONG Exit:
**Alert 3: Lower High (LH)**
- Webhook URL: `https://your-bot.ngrok-free.app/webhook/smc/low-high`
- Message:
```json
{
  "ticker": "EUR_USD",
  "exchange": "OANDA",
  "close": "{{close}}"
}
```

**Alert 4: Higher High (HH)**
- Webhook URL: `https://your-bot.ngrok-free.app/webhook/smc/high-high`
- Message:
```json
{
  "ticker": "EUR_USD",
  "exchange": "OANDA",
  "close": "{{close}}"
}
```

## Running the Strategy

```bash
# Start the bot with SMC strategy
export STRATEGY_FILE=smc_structure
./trader_bot

# Or set in .env
STRATEGY=smc_structure
```

## Position Sizing

Configure your position size:

```bash
# In .env file
MARGIN_AMOUNT=100          # $100 margin per trade (recommended)
# OR
TRADE_USD_AMOUNT=1000      # $1000 notional value
# OR
TRADE_UNITS=1000           # Fixed units
```

## Available Webhooks

- `/webhook/smc/low-low` - SMC Lower Low (LL)
- `/webhook/smc/high-low` - SMC Higher Low (HL)  
- `/webhook/smc/low-high` - SMC Lower High (LH)
- `/webhook/smc/high-high` - SMC Higher High (HH)

## Testing

Test the webhooks manually:

```bash
# Test LONG entry on LL
curl -X POST http://localhost:8080/webhook/smc/low-low \
  -H "Content-Type: application/json" \
  -d '{"ticker":"EUR_USD","exchange":"OANDA","close":"1.0850"}'

# Test LONG entry on HL
curl -X POST http://localhost:8080/webhook/smc/high-low \
  -H "Content-Type: application/json" \
  -d '{"ticker":"EUR_USD","exchange":"OANDA","close":"1.0860"}'

# Test LONG exit on LH
curl -X POST http://localhost:8080/webhook/smc/low-high \
  -H "Content-Type: application/json" \
  -d '{"ticker":"EUR_USD","exchange":"OANDA","close":"1.0870"}'

# Test LONG exit on HH
curl -X POST http://localhost:8080/webhook/smc/high-high \
  -H "Content-Type: application/json" \
  -d '{"ticker":"EUR_USD","exchange":"OANDA","close":"1.0880"}'
```

## Strategy Logic

**LONG Trades:**
- Enter when market creates LL (potential reversal) or HL (continuation)
- Exit when market creates LH (bearish) or HH (take profit)

**SHORT Trades:**
- Enter when market creates HH (potential reversal) or LH (continuation)
- Exit when market creates HL (bullish) or LL (take profit)

## Risk Management

Since SMC identifies structure breaks, consider:
- Using stop losses below recent structure lows for LONG
- Using stop losses above recent structure highs for SHORT
- Position sizing based on structure distance
- Multiple timeframe confirmation

## Notes

- The strategy works best with proper SMC indicator that accurately detects structure points
- Use "Once Per Bar Close" for alerts to avoid false signals
- Consider adding filters (RSI, volume, etc.) for higher probability setups
- Works on any timeframe but higher timeframes (15m+) tend to be more reliable
