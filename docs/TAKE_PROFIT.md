# Take Profit Configuration

The trading bot now supports automatic take profit orders with three different calculation methods: **pips**, **percentage**, or **dollar amount**.

## Configuration

Set one of these environment variables in your `.env` file:

### Option 1: Take Profit in Pips

```bash
TAKE_PROFIT_PIPS=50
```

- For most currency pairs (e.g., EUR_USD, GBP_USD): 1 pip = 0.0001
- For JPY pairs (e.g., USD_JPY, EUR_JPY): 1 pip = 0.01

**Example:**
- LONG EUR_USD at 1.0850 with `TAKE_PROFIT_PIPS=50`
- Take profit will be set at: 1.0850 + (50 × 0.0001) = **1.0900**

### Option 2: Take Profit in Dollar Amount

```bash
TAKE_PROFIT_DOLLARS=100
```

- Set exact dollar profit target
- Automatically calculates required price move based on position size

**Example:**
- LONG 1000 units EUR_USD at 1.0850 with `TAKE_PROFIT_DOLLARS=100`
- Price distance needed: $100 ÷ 1000 units = 0.1
- Take profit will be set at: 1.0850 + 0.1 = **1.0950**

### Option 3: Take Profit in Percentage

```bash
TAKE_PROFIT_PCT=2.5
```

- Value represents percentage gain from entry price
- Works for all instruments

**Example:**
- LONG EUR_USD at 1.0850 with `TAKE_PROFIT_PCT=2.5`
- Take profit will be set at: 1.0850 + (1.0850 × 0.025) = **1.1121**

## Priority

If multiple are set, priority order is:
1. **TAKE_PROFIT_PIPS** (highest priority)
2. **TAKE_PROFIT_DOLLARS**
3. **TAKE_PROFIT_PCT** (lowest priority)

## Disable Take Profit

To disable automatic take profit:
- Don't set any of the variables, or
- Set them to empty string: `TAKE_PROFIT_PIPS=`

## How It Works

1. When a position is opened (LONG or SHORT), the bot:
   - Gets the current market price
   - Calculates the take profit price based on your configuration
   - Sends the order with `takeProfitOnFill` attached

2. OANDA automatically creates a take profit order when the position fills

3. The take profit order remains active until:
   - Price hits the TP level (position closes automatically)
   - You close the position manually via webhook
   - MACD reversal closes the position
   - RSI exit signal closes the position

## Examples

### Conservative Strategy (20 pips)
```bash
MARGIN_AMOUNT=100
TAKE_PROFIT_PIPS=20
```

### Fixed Dollar Target ($50 profit)
```bash
MARGIN_AMOUNT=100
TAKE_PROFIT_DOLLARS=50
```

### Aggressive Strategy (5% gain)
```bash
MARGIN_AMOUNT=500
TAKE_PROFIT_PCT=5
```

### Scalping (10 pips)
```bash
MARGIN_AMOUNT=50
TAKE_PROFIT_PIPS=10
```

## Logging

When a position opens, you'll see logs like:

**Pips-based:**
```
🎯 [TP CALC] 50 pips = 0.00500 price distance
🎯 [TP CALC] Entry: 1.08500 → TP: 1.09000 (LONG)
🎯 [TP] Take profit set at 1.09000
```

**Dollar-based:**
```
🎯 [TP CALC] $100.00 ÷ 1000 units = 0.10000 price distance
🎯 [TP CALC] Entry: 1.08500 → TP: 1.18500 (LONG)
🎯 [TP] Take profit set at 1.18500
```

**Percentage-based:**
```
🎯 [TP CALC] 2.50% = 0.02713 price distance
🎯 [TP CALC] Entry: 1.08500 → TP: 1.11213 (LONG)
🎯 [TP] Take profit set at 1.11213
```

## Trade Lifecycle with Take Profit

1. **Entry Signal**: RSI extreme + MACD cross → Position opens with TP
2. **Exit Scenarios** (whichever comes first):
   - ✅ Price hits take profit → Auto close (profit locked)
   - ✅ RSI crosses center twice → Manual close
   - ✅ RSI hits opposite extreme (30 for SHORT, 70 for LONG) → Manual close
   - ✅ MACD reversal → Manual close

## Notes

- Take profit is calculated at order creation time
- Works with all position sizing methods (MARGIN_AMOUNT, TRADE_USD_AMOUNT, TRADE_UNITS)
- Take profit orders use GTC (Good Till Cancelled) time-in-force
- If price calculation fails, order proceeds without take profit (logged as warning)
