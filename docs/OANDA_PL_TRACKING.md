# OANDA P/L Tracking & Daily Profit Target

This document describes the real-time P/L tracking from OANDA and the daily profit target feature.

## Features

### 1. Real-Time P/L from OANDA

When you have an open position with OANDA (not simulated), the bot will:

- **Fetch unrealized P/L** on every webhook event
- **Display P/L** in logs with clear formatting (🟢 for profit, 🔴 for loss)
- **Include P/L in /status** endpoint response

The P/L is fetched directly from OANDA's API, giving you the accurate account currency P/L.

### 2. Daily Profit Target

Automatically stop trading after reaching a daily profit goal. Perfect for:
- Disciplined trading (take profits and stop)
- Risk management (prevent overtrading on good days)
- Capital preservation

## Configuration

### Environment Variables

```bash
# Daily profit target (optional)
DAILY_PROFIT_TARGET=100    # Stop trading after $100 profit for the day
```

### How It Works

1. **Tracking**: Every closed position's realized P/L is added to the daily total
2. **Check**: Before opening new positions, the bot checks if daily target is reached
3. **Auto-Disable**: When target is reached, strategy is disabled automatically
4. **Daily Reset**: Counter resets automatically at midnight (based on your timezone)

## API Endpoints

### GET /status

Returns enhanced status including OANDA P/L data:

```json
{
  "positions": {
    "EUR_USD": {
      "Symbol": "EUR_USD",
      "PositionOpen": true,
      "Position": "long",
      "TradeID": "12345",
      "OandaUnrealizedPL": "15.50",
      ...
    }
  },
  "oandaPositions": {
    "EUR_USD": {
      "tradeID": "12345",
      "unrealizedPL": "15.50",
      "currentUnits": "100000",
      "price": "1.0850",
      "openTime": "2026-01-20T10:30:00Z",
      "initialMargin": "2000.00"
    }
  },
  "dailyProfit": {
    "realized": "45.00",
    "target": "100.00",
    "targetReached": false,
    "remaining": "55.00"
  },
  "strategyEnabled": true,
  "strategyName": "ma_trend_rsi_atr"
}
```

### GET /daily-profit

Get detailed daily profit statistics:

```json
{
  "enabled": true,
  "realized": "45.00",
  "target": "100.00",
  "remaining": "55.00",
  "targetReached": false,
  "resetTime": "2026-01-20T00:00:00-05:00",
  "percentOfTarget": "45.0%"
}
```

### POST /reset-daily-profit

Manually reset the daily profit counter and re-enable trading:

```bash
curl -X POST http://localhost:8080/reset-daily-profit
```

Response:
```json
{
  "success": true,
  "message": "Daily profit reset to $0.00 and strategy re-enabled",
  "strategyEnabled": true,
  "dailyProfit": "0.00"
}
```

## Example Usage

### Set Daily Target in .env

```bash
# OANDA Configuration
OANDA_API_KEY=your_api_key
OANDA_ACCOUNT_ID=your_account_id

# Position sizing
MARGIN_AMOUNT=100

# Take profit and stop loss
TAKE_PROFIT_DOLLARS=25
STOP_LOSS_DOLLARS=15

# Daily profit target - stop after $100 profit
DAILY_PROFIT_TARGET=100

# Strategy
STRATEGY_FILE=ma_trend_rsi_atr
```

### Monitor Progress

```bash
# Check current daily profit
curl http://localhost:8080/daily-profit

# Check full status with P/L
curl http://localhost:8080/status | jq '.dailyProfit, .oandaPositions'
```

### Manual Reset

If you want to continue trading after reaching the target:

```bash
curl -X POST http://localhost:8080/reset-daily-profit
```

## Log Output Examples

### Position P/L Update
```
💰 [OANDA P/L] 🟢 EUR_USD: +$15.50
```

### Trade Closed with P/L
```
🎯🎯🎯🎯🎯🎯🎯🎯🎯🎯🎯🎯🎯🎯🎯🎯🎯🎯🎯🎯
🎯 TAKE PROFIT HIT - LONG EUR_USD
🎯🎯🎯🎯🎯🎯🎯🎯🎯🎯🎯🎯🎯🎯🎯🎯🎯🎯🎯🎯
Trade ID: 12345
Target: $25 profit
💰 Realized P/L: 🟢 +$25.50
🎯🎯🎯🎯🎯🎯🎯🎯🎯🎯🎯🎯🎯🎯🎯🎯🎯🎯🎯🎯
✅ Position closed automatically by OANDA
💵 [DAILY] Added $25.50 to daily profit. Today's total: $75.50
```

### Daily Target Reached
```
💰💰💰💰💰💰💰💰💰💰💰💰💰💰💰💰💰💰💰💰
🎯 [DAILY PROFIT] Target reached! Today's profit: $102.30 (Target: $100.00)
🛑 [DAILY PROFIT] Trading DISABLED for the rest of the day
   Re-enable manually via /enable-strategy or wait for new trading day
💰💰💰💰💰💰💰💰💰💰💰💰💰💰💰💰💰💰💰💰
```

## Technical Details

### P/L Data Source

- **Unrealized P/L**: Fetched from `GET /v3/accounts/{accountID}/trades/{tradeID}`
- **Realized P/L**: Extracted from `GET /v3/accounts/{accountID}/trades/{tradeID}/close` response
- **All Open Positions**: Fetched from `GET /v3/accounts/{accountID}/openTrades`

### Daily Reset Logic

- Counter resets when the date changes (based on configured timezone)
- Uses `TRADING_TIMEZONE` or `TIMEZONE_OFFSET` for determining the day boundary
- Reset happens automatically on first webhook after midnight

### P/L State Fields

New fields added to `PositionState`:
- `OandaUnrealizedPL` - Current unrealized P/L from OANDA (string, e.g., "15.50")
- `OandaRealizedPL` - Realized P/L when position was closed (string, e.g., "-5.00")

### Global Daily Profit Variables

- `dailyProfitTarget` - Target amount (float64)
- `dailyProfitTargetEnabled` - Whether feature is enabled (bool)
- `dailyRealizedProfit` - Running total for today (float64)
- `dailyProfitResetTime` - When counter was last reset (time.Time)
- `dailyProfitTargetReached` - Flag when target is hit (bool)

## Best Practices

1. **Start Conservative**: Begin with a realistic daily target based on your strategy's expected performance
2. **Account for Losses**: The daily profit is NET (profits minus losses)
3. **Use with Trading Hours**: Combine with `TRADING_HOURS` for disciplined session-based trading
4. **Monitor Early**: Use `/daily-profit` endpoint to track progress throughout the day
5. **Don't Chase**: If target is reached early, resist the urge to reset and continue

## Troubleshooting

### P/L Not Updating

1. Ensure the position is not simulated (`IsSimulated: false`)
2. Check that `TradeID` is set
3. Verify OANDA API credentials are correct

### Daily Profit Not Tracking

1. Confirm `DAILY_PROFIT_TARGET` is set in environment
2. Check logs for "Daily Profit Target: $X.XX" on startup
3. Verify positions are being closed through OANDA (not simulated)

### Target Not Reached Despite Profits

1. Check if losses offset the gains (net must exceed target)
2. Verify `TRADING_TIMEZONE` is set correctly for day boundaries
