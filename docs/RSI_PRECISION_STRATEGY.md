# RSI Precision Strategy

## Overview
This strategy uses precise RSI levels (25, 30, 40, 50, 60, 70, 75) combined with MACD confirmation for high-probability entries and exits.

## Strategy Logic

### LONG Entry
- **RSI crosses UP from oversold at 25** (extreme reversal signal)
- **AND MACD histogram crosses up** (momentum confirmation)

### LONG Exit
- **RSI crosses down from overbought at 75** (extreme reversal)
- **OR MACD histogram crosses down** (momentum reversal)

### SHORT Entry
- **RSI crosses DOWN from overbought at 75** (extreme reversal signal)
- **AND MACD histogram crosses down** (momentum confirmation)

### SHORT Exit
- **RSI crosses up from oversold at 25** (extreme reversal)
- **OR MACD histogram crosses up** (momentum reversal)

## TradingView Alert Setup

You need to create the following alerts in TradingView:

### RSI Alerts (use RSI indicator with period 14)
1. **RSI Cross Up from Oversell 25**
   - Condition: `RSI(14) crossing up 25.00`
   - Webhook: `/webhook/rsi/cross-up-oversell-25`
   - Event: `RSI_CROSS_UP_FROM_OVERSELL`

2. **RSI Cross Oversell 30**
   - Condition: `RSI(14) crossing 30.00`
   - Webhook: `/webhook/rsi/cross-oversell-30`
   - Event: `RSI_CROSS_OVERSELL`

3. **RSI Cross 40**
   - Condition: `RSI(14) crossing 40.00`
   - Webhook: `/webhook/rsi/cross-40`
   - Event: `RSI_CROSS_40`

4. **RSI MA Cross Center 50**
   - Condition: `RSI(14) crossing 50.00`
   - Webhook: `/webhook/rsi/cross-center-50`
   - Event: `RSI_MA_CROSS_CENTER`

5. **RSI Cross 60**
   - Condition: `RSI(14) crossing 60.00`
   - Webhook: `/webhook/rsi/cross-60`
   - Event: `RSI_CROSS_60`

6. **RSI Cross Overbought 70**
   - Condition: `RSI(14) crossing 70.00`
   - Webhook: `/webhook/rsi/cross-overbuy-70`
   - Event: `RSI_CROSS_OVERBUY`

7. **RSI Cross Down from Overbought 75**
   - Condition: `RSI(14) crossing down 75.00`
   - Webhook: `/webhook/rsi/cross-down-overbuy-75`
   - Event: `RSI_CROSS_DOWN_FROM_OVERBUY`

### MACD Alerts (use MACD indicator)
8. **MACD Cross Up**
   - Condition: `MACD line crossing above Signal line`
   - Webhook: `/webhook/macd/cross-up`
   - Event: `MACD_CROSS_UP`

9. **MACD Cross Down**
   - Condition: `MACD line crossing below Signal line`
   - Webhook: `/webhook/macd/cross-down`
   - Event: `MACD_CROSS_DOWN`

## Webhook Payload Format

All alerts should send JSON in this format:

```json
{
  "event": "RSI_CROSS_UP_FROM_OVERSELL",
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

## Running the Strategy

```bash
STRATEGY=rsi_precision docker-compose up
```

## Why This Strategy Works

1. **Extreme Entry Points**: Enters at RSI 25/75 which are more extreme than typical 30/70, reducing false signals
2. **MACD Confirmation**: Requires momentum confirmation before entry
3. **Simple Exit**: Exits on reversal signals (opposite extreme or MACD reversal)
4. **Clear Rules**: No ambiguity - either both conditions met or no trade

## Backtesting Notes

- Best on: 5m, 15m, 1h timeframes
- Works well on: Forex majors (EUR/USD, GBP/USD), crypto pairs
- Entry frequency: Low (waits for extremes)
- Win rate: Moderate to high (due to extreme entry levels)
- Risk/reward: Good (enters near reversals)

## Customization

To modify entry/exit levels, edit `/strategies/rsi_precision.json`:

- Change `combination` from `"all"` to `"any"` for faster signals
- Add additional conditions for more confirmation
- Adjust webhook paths to use different RSI levels

## Alternative: Advanced Momentum Strategy

If you want even more complex logic with nested conditions (sequential steps with multiple exit options), see `/strategies/advanced_momentum.json`. Note: This requires implementing nested condition groups in the code (not yet supported).
