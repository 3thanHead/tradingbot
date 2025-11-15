# Changelog

## [Unreleased] - 2025-11-14

### Added
- **Take Profit Support**: Automatic take profit orders with two calculation methods
  - `TAKE_PROFIT_PIPS`: Set TP in pips (e.g., 50 pips)
  - `TAKE_PROFIT_PCT`: Set TP in percentage (e.g., 2.5%)
  - Works with all position sizing methods (margin, USD, units)
  - Automatic pip value detection for JPY pairs vs other pairs
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
