# Changelog

## November 16, 2025

### Removed is_long Field
- Exit conditions no longer need `is_long` field
- Position direction (LONG/SHORT) comes from actual OANDA position at runtime
- Simplified strategy JSON files (removed duplicate exit conditions)

### File Corruption Fix
- Restored main.go from clean commit after corruption discovered
- Manually re-added complete strategy system (1460 lines)

### Webhook Path Refactoring
- Changed functions to accept `*http.Request` instead of hardcoded webhook paths
- Handlers now use `r.URL.Path` for cleaner code

## November 14, 2025

### Strategy System (v2.0)
- **JSON-based strategies**: Edit JSON files to change trading logic without coding
- **3 built-in strategies**: default, ma_ribbon, scalping
- **Combination modes**: all_sequential, all, any
- **7 webhook endpoints**: RSI (3), MACD (2), MA Ribbon (2)
- **Environment variable**: `STRATEGY=default` to select strategy

### Take Profit & Exits (v1.1)
- **Take profit modes**: Pips, Dollars, or Percentage
- **MACD reversal exits**: Close on opposite MACD cross
- **RSI double-cross**: Requires two center crosses before exit
- **State reset**: All flags reset after position closes
