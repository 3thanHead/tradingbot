# Separate LONG/SHORT Strategy Support

Added ability to define different entry/exit logic for LONG vs SHORT positions.

## Two Strategy Formats

### Option 1: Unified (Original)
Same entry/exit logic for both directions:

```json
{
  "name": "my_strategy",
  "entry": { ... },
  "exit": { ... }
}
```

### Option 2: Separate (New!)
Different logic for LONG and SHORT:

```json
{
  "name": "my_strategy",
  "long": {
    "entry": { ... },
    "exit": { ... }
  },
  "short": {
    "entry": { ... },
    "exit": { ... }
  }
}
```

## Use Cases

**Unified format** - When LONG and SHORT use same indicators:
- Simple mean reversion
- Symmetric strategies
- Same exit rules for both directions

**Separate format** - When LONG and SHORT need different logic:
- Asymmetric risk management
- Different entry zones (RSI 30-50 for LONG, 50-70 for SHORT)
- Direction-specific exit conditions
- Trend-following (only LONG in uptrends, only SHORT in downtrends)

## Example: Separate LONG/SHORT

See `strategies/default-separate.json`:

**LONG:**
- Entry: MACD cross up + RSI oversold
- Exit: RSI overbought (>70)

**SHORT:**
- Entry: MACD cross down + RSI overbought  
- Exit: RSI oversold (<30)

## Changes Made

### Code Changes
- Updated `Strategy` struct to support both formats
- Added `PositionStrategy` type for LONG/SHORT configurations
- Updated `validateStrategy()` to handle both formats
- Updated `shouldOpenPosition()` to select correct entry conditions
- Updated `shouldExitPosition()` to select correct exit conditions

### Files Modified
- `main.go` - Added separate LONG/SHORT support
- `strategies/README.md` - Documented both formats
- `strategies/default-separate.json` - Example strategy

## Usage

```bash
# Use separate LONG/SHORT strategy
STRATEGY=default-separate docker-compose up

# Or use unified strategy
STRATEGY=default docker-compose up
```

The bot automatically detects which format your strategy uses.
