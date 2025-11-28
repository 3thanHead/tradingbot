# Stochastic + RSI + MACD High Win-Rate Strategy

## Overview

This strategy is based on a proven high win-rate approach that combines three powerful indicators:
- **Stochastic Oscillator** - Entry signal generator (identifies overbought/oversold)
- **RSI (50 line)** - Trend confirmation (not used as overbought/oversold)
- **MACD** - Momentum confirmation

**Source:** YouTube Strategy - [Watch Video](https://www.youtube.com/watch?v=hh3BKTFE1dc)

**Backtest Results (100 trades on EUR/USD 30min):**
- Win Rate: **56%**
- Profit: **28% gain** ($280 on $1000 capital)
- Risk per trade: 2% with 200x leverage
- Max consecutive wins: 5
- Max consecutive losses: 3

## Strategy File

Use strategy: `stochastic_rsi_macd.json`

## How It Works

### LONG Entry Rules (Sequential Order)

1. ✅ **Stochastic enters oversold** - Both K and D lines drop below 20
2. ✅ **RSI confirms uptrend** - RSI is above 50 (NOT used as overbought/oversold indicator)
3. ✅ **MACD confirms momentum** - MACD line crosses above signal line
4. ✅ **Stochastic verification** - Ensure stochastic still hasn't hit overbought (>80)

**All 4 conditions must be met in this exact order.**

### SHORT Entry Rules (Sequential Order)

1. ✅ **Stochastic enters overbought** - Both K and D lines rise above 80
2. ✅ **RSI confirms downtrend** - RSI is below 50
3. ✅ **MACD confirms momentum** - MACD line crosses below signal line
4. ✅ **Stochastic verification** - Ensure stochastic still hasn't hit oversold (<20)

**All 4 conditions must be met in this exact order.**

### Exit Strategy

- **Stop Loss:** Place at nearest swing high (SHORT) or swing low (LONG)
- **Take Profit:** Set at 1.5x the stop loss distance
- **Optional Limit:** Cap stop loss at 0.15% max risk to avoid large losses

### Key Strategy Rules

⚠️ **IMPORTANT:**
- **One position at a time** - Don't open new positions until previous one closes
- **Sequential execution** - All entry conditions must occur in order
- **Stochastic verification** - After MACD confirms, ensure stochastic hasn't reversed to opposite extreme

## TradingView Alert Setup

### Required Indicators on TradingView

1. **Stochastic Oscillator** (default settings: K=14, D=3, Smooth=3)
   - Oversold level: 20
   - Overbought level: 80

2. **RSI** (14 period)
   - Upper/Lower bands: Both set to 50

3. **MACD** (12, 26, 9)

### Webhook Alerts to Create

You need to create **6 TradingView alerts** for each symbol you trade:

#### 1. Stochastic Oversold Entry
```
Alert Condition: Stochastic %K crosses below 20 AND %D is below 20
Webhook URL: https://your-ngrok-url.ngrok-free.dev/webhook/stochastic/oversold
Message:
{
  "ticker": "{{ticker}}",
  "close": "{{close}}",
  "time": "{{time}}"
}
```

#### 2. Stochastic Overbought Entry
```
Alert Condition: Stochastic %K crosses above 80 AND %D is above 80
Webhook URL: https://your-ngrok-url.ngrok-free.dev/webhook/stochastic/overbought
Message:
{
  "ticker": "{{ticker}}",
  "close": "{{close}}",
  "time": "{{time}}"
}
```

#### 3. Stochastic Exit Oversold
```
Alert Condition: Stochastic %K crosses above 20 (exiting oversold zone)
Webhook URL: https://your-ngrok-url.ngrok-free.dev/webhook/stochastic/exit-oversold
Message:
{
  "ticker": "{{ticker}}",
  "close": "{{close}}",
  "time": "{{time}}"
}
```

#### 4. Stochastic Exit Overbought
```
Alert Condition: Stochastic %K crosses below 80 (exiting overbought zone)
Webhook URL: https://your-ngrok-url.ngrok-free.dev/webhook/stochastic/exit-overbought
Message:
{
  "ticker": "{{ticker}}",
  "close": "{{close}}",
  "time": "{{time}}"
}
```

#### 5. RSI Above 50 (Uptrend Confirmation)
```
Alert Condition: RSI crosses above 50
Webhook URL: https://your-ngrok-url.ngrok-free.dev/webhook/rsi/above-50
Message:
{
  "ticker": "{{ticker}}",
  "close": "{{close}}",
  "time": "{{time}}"
}
```

#### 6. RSI Below 50 (Downtrend Confirmation)
```
Alert Condition: RSI crosses below 50
Webhook URL: https://your-ngrok-url.ngrok-free.dev/webhook/rsi/below-50
Message:
{
  "ticker": "{{ticker}}",
  "close": "{{close}}",
  "time": "{{time}}"
}
```

#### 7. MACD Cross Up (Existing)
```
Alert Condition: MACD line crosses above signal line
Webhook URL: https://your-ngrok-url.ngrok-free.dev/webhook/macd/cross-up
Message:
{
  "ticker": "{{ticker}}",
  "close": "{{close}}",
  "time": "{{time}}"
}
```

#### 8. MACD Cross Down (Existing)
```
Alert Condition: MACD line crosses below signal line
Webhook URL: https://your-ngrok-url.ngrok-free.dev/webhook/macd/cross-down
Message:
{
  "ticker": "{{ticker}}",
  "close": "{{close}}",
  "time": "{{time}}"
}
```

## Running the Strategy

### Start the Bot

```bash
# Set environment variables
export STRATEGY=stochastic_rsi_macd
export OANDA_API_KEY=your_api_key
export OANDA_ACCOUNT_ID=your_account_id
export TRADE_USD_AMOUNT=100  # Or use MARGIN_AMOUNT or TRADE_UNITS

# Run with docker-compose
docker-compose up
```

### Recommended Settings

Based on the backtest:
- **Capital:** $1000+
- **Risk per trade:** 2% of capital
- **Leverage:** 50:1 to 200:1 (depending on your broker)
- **Timeframe:** 30-minute chart (backtested)
- **Symbol:** EUR/USD (backtested), but works on other pairs

### Environment Variables

```bash
STRATEGY=stochastic_rsi_macd
TRADE_USD_AMOUNT=100          # $100 per trade
# OR
MARGIN_AMOUNT=50              # $50 margin (with leverage determines position size)
```

## Strategy Optimizations

### 1. Stop Loss Limiting

To avoid excessive risk when swing highs/lows are far away:

```bash
# Limit stop loss to max 0.15% of account
# This prevents massive losses from wide stop placements
```

**Implementation:** When calculating stop loss, cap it at 0.15% regardless of swing distance.

### 2. One Position at a Time

The bot is configured to **NOT open new positions while one is already open**. This rule:
- Simplifies trade management
- Prevents overexposure
- Matches the backtested approach

### 3. Sequential Condition Validation

All entry conditions must occur **in order**:
- Missing a step resets the sequence
- Prevents false signals from random indicator alignments
- Ensures all three indicators agree in proper sequence

## Example Trade Flow

### LONG Position Example

```
1. ⏬ Stochastic K&D drop below 20 (oversold)
   └─ Bot: "Waiting for RSI confirmation..."

2. ⬆️ RSI crosses above 50 (uptrend)
   └─ Bot: "Waiting for MACD confirmation..."

3. ⬆️ MACD crosses above signal line (momentum up)
   └─ Bot: "Checking stochastic status..."

4. ✅ Stochastic still below 80 (not overbought yet)
   └─ Bot: "All conditions met! Opening LONG position"

📈 Position opened at current price
🛑 Stop loss: Nearest swing low
🎯 Take profit: 1.5x stop loss distance
```

### SHORT Position Example

```
1. ⏫ Stochastic K&D rise above 80 (overbought)
   └─ Bot: "Waiting for RSI confirmation..."

2. ⬇️ RSI crosses below 50 (downtrend)
   └─ Bot: "Waiting for MACD confirmation..."

3. ⬇️ MACD crosses below signal line (momentum down)
   └─ Bot: "Checking stochastic status..."

4. ✅ Stochastic still above 20 (not oversold yet)
   └─ Bot: "All conditions met! Opening SHORT position"

📉 Position opened at current price
🛑 Stop loss: Nearest swing high
🎯 Take profit: 1.5x stop loss distance
```

## Common Mistakes to Avoid

❌ **Using stochastic alone** - Don't trade just on overbought/oversold
❌ **Using RSI as overbought/oversold** - Use it for trend confirmation (50 line)
❌ **Ignoring sequence** - All conditions must occur in order
❌ **Opening multiple positions** - Stick to one trade at a time
❌ **Wide stop losses** - Consider limiting stops to 0.15% max
❌ **Ignoring stochastic verification** - Check it hasn't reversed after MACD

## Performance Notes

From the 100-trade backtest:
- **Consistent profitability** over 134 days
- **Manageable drawdown** with max 3 consecutive losses
- **Good risk/reward** at 1.5:1 take profit ratio
- **Works best in trending markets** (RSI 50 line filter helps)

## Monitoring Your Trades

Check bot logs for sequential condition tracking:

```bash
docker-compose logs -f trader_bot
```

Look for messages like:
- `📊 Stochastic K&D entered OVERSOLD`
- `📊 RSI crossed ABOVE 50 (uptrend)`
- `📊 MACD Cross Up`
- `✅ [TRADE] Strategy conditions met! Opening LONG position`

## Testing on BTCUSD

Since BTCUSD is available on weekends, you can test this strategy:

1. Create all 8 TradingView alerts for BTCUSD symbol
2. Set to 30-minute timeframe (or experiment with others)
3. Monitor performance over weekends
4. Compare against EUR/USD results

## Notes

- Strategy requires all 8 webhooks properly configured
- Sequential mode means conditions must happen in order
- Bot automatically tracks state per symbol (can run multiple pairs)
- Take profit/stop loss should be set via OANDA position parameters
- Consider backtesting on your specific symbol/timeframe first
