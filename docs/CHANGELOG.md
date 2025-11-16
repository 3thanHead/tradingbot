# Changelog

## [2.0.0] - Strategy System Release - 2025-11-14

### 🎨 Major Features Added

#### JSON-Based Strategy Configuration System
- **Flexible Strategy Definition**: Define custom entry and exit strategies using JSON configuration files
- **No Code Changes Required**: Switch between strategies by changing the `STRATEGY` environment variable
- **Built-in Validation**: Comprehensive strategy validation on startup with detailed error messages

#### New Strategy Components
- **Entry Steps**: Define sequential or simultaneous entry conditions
- **Entry Combination Modes**:
  - `all` - All conditions must be met (order doesn't matter)
  - `all_sequential` - All conditions must be met in exact order
  - `any` - Any single condition triggers entry
- **Exit Conditions**: Multiple exit conditions with "any" logic (first to trigger closes position)

#### Supported Conditions
**Entry & Exit:**
- `rsi_crossed_up` - RSI crosses above 70 (overbought)
- `rsi_crossed_down` - RSI crosses below 30 (oversold)
- `rsi_crossed_center` - RSI crosses 50 midpoint
- `macd_cross_up` - MACD line crosses above signal
- `macd_cross_down` - MACD line crosses below signal
- `ma_ribbon_bullish` - All MAs aligned bullish (5>10>20>50>100)
- `ma_ribbon_bearish` - All MAs aligned bearish (5<10<20<50<100)

#### Built-in Strategies
1. **`default`** - Preserves original bot behavior
   - Sequential entry: RSI extreme → MACD confirmation
   - Multiple exit conditions: RSI center double-cross, MACD reversal, RSI extreme after warning
   
2. **`ma_ribbon`** - Trend-following strategy
   - Entry: All MAs aligned + MACD confirmation
   - Exit: MA ribbon reversal
   
3. **`scalping`** - Fast trading strategy
   - Entry: MACD cross + RSI neutral
   - Exit: Quick exits on MACD reversal or RSI extremes

### 🔧 Technical Changes

#### New Webhook Endpoints
- `POST /webhook/ma/ribbon-bullish` - MA ribbon bullish alignment
- `POST /webhook/ma/ribbon-bearish` - MA ribbon bearish alignment

**Total Endpoints: 7** (was 5)

#### Code Architecture
- Added strategy type system (`Strategy`, `EntryStep`, `EntryConditions`, `ExitCondition`, `ExitConditions`)
- Implemented `loadStrategy()` for JSON file loading
- Implemented `validateStrategy()` for configuration validation
- Added `checkEntryStepCondition()` for mapping webhooks to conditions
- Added `shouldOpenPosition()` with entry logic evaluation
- Added `shouldExitPosition()` with exit logic evaluation
- Refactored all webhook handlers to use strategy system
- Updated `PositionState` to track entry step completion and MA ribbon states

#### Configuration
- New environment variable: `STRATEGY` (defaults to `default`)
- Strategy files location: `strategies/` directory
- Backward compatible: `default` strategy matches original behavior exactly

### 📚 Documentation Added
- `STRATEGY_SYSTEM.md` - Complete strategy system guide (400+ lines)
- `STRATEGY_QUICK_START.md` - 5-minute quick start guide
- `strategies/README.md` - Strategy directory documentation
- Updated `README.md` with strategy system overview
- Updated `.env.example` with strategy configuration

### ✅ Backward Compatibility
- Default strategy (`default.json`) preserves exact original behavior
- All existing TradingView alerts remain compatible
- No breaking changes to existing functionality
- Take profit and position sizing unchanged

### 🎯 Benefits
- **Experimentation**: Test different strategies without code changes
- **Rapid Iteration**: Modify strategies in seconds, no recompile needed
- **Clean Separation**: Trading logic separated from execution logic
- **Extensible**: Easy to add new condition types in the future
- **Maintainable**: Clear, declarative strategy definitions

---

## [1.1.0] - Take Profit & Exit Enhancements - 2025-11-14

### Added
- **Take Profit Support**: Automatic take profit orders with three calculation methods
  - `TAKE_PROFIT_PIPS`: Set TP in pips (e.g., 50 pips)
  - `TAKE_PROFIT_DOLLARS`: Set TP in dollar amount (e.g., $100 profit)
  - `TAKE_PROFIT_PCT`: Set TP in percentage (e.g., 2.5%)
  - Priority: Pips > Dollars > Percentage
  - Works with all position sizing methods (margin, USD, units)
  - Automatic pip value detection for JPY pairs vs other pairs
  - Dollar-based TP calculates price distance from position size
  - Detailed logging of TP calculations
  - Documentation in `TAKE_PROFIT.md`

- **MACD Reversal Exits**: Positions now close on MACD reversals
  - SHORT position closes when MACD crosses up (bullish reversal)
  - LONG position closes when MACD crosses down (bearish reversal)
  - Provides additional exit protection beyond RSI signals

- **State Machine Reset**: All flags now reset after position closes
  - Prevents stale signals from affecting next trade
  - Clean slate for each new trading cycle

### Changed
- **RSI Center Exit Logic**: Now requires confirmation before closing
  - First RSI center cross: Sets warning flag, keeps position open
  - Second RSI center cross: Closes position
  - Alternative exit: RSI extreme after warning (30 for SHORT, 70 for LONG)
  - Prevents premature exits on first center cross

- **Route Naming**: Renamed `/webhook/rsi/crossed-zero` to `/webhook/rsi/crossed-center`
  - Better reflects RSI centerline (50) concept
  - More intuitive naming convention

### Fixed
- None

## Trading Strategy Summary

### Entry Conditions
- RSI extreme (>70 or <30) + MACD cross in same direction

### Exit Conditions (any of these)
1. Take profit hit (if configured)
2. RSI crosses center twice (double confirmation)
3. RSI crosses center once, then hits opposite extreme
4. MACD crosses against position direction (reversal)

### Position Management
- Instrument-specific leverage from OANDA API
- Margin-based, USD-based, or unit-based sizing
- Automatic state reset after close
