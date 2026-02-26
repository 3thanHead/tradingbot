# Trading Hours Configuration

Restrict trading to specific hours and days of the week to avoid low-liquidity periods, news events, or to match your preferred trading sessions.

## Quick Setup

```bash
# Trade only during London/NY overlap (1pm-5pm UTC)
TRADING_START_HOUR=13
TRADING_END_HOUR=17

# Trade Monday through Friday only
TRADING_DAYS=1,2,3,4,5

# Use a specific timezone
TRADING_TIMEZONE=America/New_York
```

## Environment Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `TRADING_START_HOUR` | Hour (0-23) when trading starts | `9` |
| `TRADING_END_HOUR` | Hour (0-24) when trading ends | `17` |
| `TRADING_DAYS` | Days of week to trade | `1,2,3,4,5` or `mon,tue,wed,thu,fri` |
| `TRADING_TIMEZONE` | Timezone name (IANA format) | `America/New_York`, `Europe/London` |
| `TIMEZONE_OFFSET` | UTC offset (fallback if timezone not set) | `-5` for EST |

## Day Values

Days can be specified as numbers or names:

| Number | Name | Aliases |
|--------|------|---------|
| 0 | Sunday | `sun`, `sunday` |
| 1 | Monday | `mon`, `monday` |
| 2 | Tuesday | `tue`, `tuesday` |
| 3 | Wednesday | `wed`, `wednesday` |
| 4 | Thursday | `thu`, `thursday` |
| 5 | Friday | `fri`, `friday` |
| 6 | Saturday | `sat`, `saturday` |

## Examples

### Weekday Trading Only (9am-5pm New York Time)

```bash
TRADING_START_HOUR=9
TRADING_END_HOUR=17
TRADING_DAYS=mon,tue,wed,thu,fri
TRADING_TIMEZONE=America/New_York
```

### Asian Session Trading (7pm-3am UTC)

```bash
# Overnight hours are supported
TRADING_START_HOUR=19
TRADING_END_HOUR=3
TRADING_DAYS=0,1,2,3,4,5,6
```

### London Session Only (8am-4pm London)

```bash
TRADING_START_HOUR=8
TRADING_END_HOUR=16
TRADING_DAYS=1,2,3,4,5
TRADING_TIMEZONE=Europe/London
```

### 24/5 Forex Hours (Sunday 5pm to Friday 5pm EST)

```bash
# For continuous forex trading, you can use a permissive setup
# and rely on your strategy's own filters
TRADING_START_HOUR=0
TRADING_END_HOUR=24
TRADING_DAYS=1,2,3,4,5
TRADING_TIMEZONE=America/New_York
```

### No Restrictions (default)

If no `TRADING_START_HOUR` or `TRADING_END_HOUR` is set, trading is allowed 24/7.

## How It Works

1. **Webhooks are still received** - The bot continues to receive and process webhooks during off-hours
2. **Condition tracking continues** - Entry/exit conditions are still tracked and updated
3. **Position opening is blocked** - New positions cannot be opened outside trading hours
4. **Exits are NOT blocked** - Existing positions can still be closed (important for risk management)

## Logs

When trading is blocked due to hours/days, you'll see logs like:

```
⏰ [TRADING HOURS] Trading blocked - hour 6 is outside trading hours (9:00-17:00)
⏰ [TRADING HOURS] Trading blocked - Saturday is not a trading day (allowed: [1 2 3 4 5])
```

At startup, the configuration is displayed:

```
🕐 Trading hours: 09:00-17:00 America/New_York on Mon,Tue,Wed,Thu,Fri
```

## Timezone Support

### Using IANA Timezone Names (Recommended)

Use `TRADING_TIMEZONE` with standard IANA timezone names:
- `America/New_York` - Eastern Time (auto-adjusts for DST)
- `America/Chicago` - Central Time
- `America/Los_Angeles` - Pacific Time
- `Europe/London` - UK Time
- `Europe/Paris` - Central European Time
- `Asia/Tokyo` - Japan Standard Time
- `Asia/Singapore` - Singapore Time
- `Australia/Sydney` - Australian Eastern Time

### Using UTC Offset (Fallback)

If `TRADING_TIMEZONE` is not set, use `TIMEZONE_OFFSET`:
- `-5` for UTC-5 (EST)
- `-4` for UTC-4 (EDT)
- `0` for UTC
- `1` for UTC+1 (CET)

⚠️ **Note:** UTC offset doesn't automatically adjust for daylight saving time.

## Docker Compose Example

```yaml
services:
  trader:
    build: .
    environment:
      - OANDA_API_KEY=${OANDA_API_KEY}
      - OANDA_ACCOUNT_ID=${OANDA_ACCOUNT_ID}
      - TRADING_START_HOUR=9
      - TRADING_END_HOUR=17
      - TRADING_DAYS=mon,tue,wed,thu,fri
      - TRADING_TIMEZONE=America/New_York
```

## Tips

1. **Test first** - Run with `OANDA_ENVIRONMENT=practice` to verify hours work correctly
2. **Consider overlaps** - The London/New York overlap (1pm-5pm UTC) typically has highest forex liquidity
3. **Avoid news** - Consider excluding hours around major economic releases
4. **Weekend gaps** - Forex markets close Friday 5pm EST and reopen Sunday 5pm EST - weekend gaps can cause slippage
