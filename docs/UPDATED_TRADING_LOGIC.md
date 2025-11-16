# Updated Trading Logic

## ✅ Changes Made

### Old Logic (RSI-triggered)
- RSI > 70 → **RSI Moving Down** → Open SHORT
- RSI < 30 → **RSI Moving Up** → Open LONG

**Problem:** Trades opened too early based on RSI momentum alone.

### New Logic (MACD-confirmed)
- RSI > 70 → **MACD Cross Down** → Open SHORT
- RSI < 30 → **MACD Cross Up** → Open LONG

**Benefit:** Waits for MACD confirmation in the same direction, reducing false signals.

---

## 📋 Updated Webhook Flow

### Opening SHORT Position
```
1. RSI Crossed Up (RSI > 70)      → Sets RSICrossedUp flag
2. MACD Cross Down                → Checks RSICrossedUp flag + Opens SHORT
```

**TradingView Setup:**
- Alert 1: RSI > 70 → Webhook: `/webhook/rsi/crossed-up`
- Alert 2: MACD Cross Down → Webhook: `/webhook/macd/cross-down`

### Opening LONG Position
```
1. RSI Crossed Down (RSI < 30)    → Sets RSICrossedDown flag
2. MACD Cross Up                  → Checks RSICrossedDown flag + Opens LONG
```

**TradingView Setup:**
- Alert 1: RSI < 30 → Webhook: `/webhook/rsi/crossed-down`
- Alert 2: MACD Cross Up → Webhook: `/webhook/macd/cross-up`

### Closing Positions (Unchanged)
```
SHORT Open + MACD Cross Up + MACD Moving Up     → Close SHORT
LONG Open + MACD Cross Down + MACD Moving Down  → Close LONG
```

---

## 🔧 Webhook Endpoints

| Endpoint | Action | Description |
|----------|--------|-------------|
| `/webhook/rsi/crossed-up` | Set flag | RSI > 70 detected |
| `/webhook/rsi/crossed-down` | Set flag | RSI < 30 detected |
| `/webhook/rsi/moving-down` | Info only | No action (previously opened SHORT) |
| `/webhook/rsi/moving-up` | Info only | No action (previously opened LONG) |
| `/webhook/macd/cross-up` | **Open LONG** | If RSICrossedDown=true |
| `/webhook/macd/cross-down` | **Open SHORT** | If RSICrossedUp=true |
| `/webhook/macd/moving-up` | Close SHORT | If SHORT position open |
| `/webhook/macd/moving-down` | Close LONG | If LONG position open |

---

## 📊 Example Trade Flow

### SHORT Trade Example

1. **Price at 1.0850** - RSI climbs above 70
   ```
   TradingView → POST /webhook/rsi/crossed-up
   Bot: ✅ RSICrossedUp flag set
   ```

2. **Price at 1.0870** - RSI moving down (ignored now)
   ```
   TradingView → POST /webhook/rsi/moving-down
   Bot: ℹ️ Info only (no action taken)
   ```

3. **Price at 1.0860** - MACD crosses down (confirmation!)
   ```
   TradingView → POST /webhook/macd/cross-down
   Bot: ✅ RSICrossedUp=true + MACDCrossedDown → Opening SHORT!
   OANDA: SHORT position opened at 1.0860
   ```

4. **Price at 1.0820** - MACD crosses up
   ```
   TradingView → POST /webhook/macd/cross-up
   Bot: ✅ MACDCrossedUp flag set
   ```

5. **Price at 1.0830** - MACD moving up
   ```
   TradingView → POST /webhook/macd/moving-up
   Bot: ✅ SHORT position + MACDCrossedUp + Moving Up → Closing SHORT!
   OANDA: SHORT position closed at 1.0830
   Profit: 30 pips
   ```

---

## 🧪 Testing

Use the test scripts with updated logic:

```bash
# Test SHORT opening (RSI > 70 + MACD Cross Down)
./scripts/test-webhooks.sh
```

Or manual testing:
```bash
# Step 1: Set RSI > 70 flag
curl -X POST https://your-domain.ngrok-free.dev/webhook/rsi/crossed-up \
  -H "Content-Type: application/json" \
  -d '{"ticker":"EURUSD","close":"1.0850"}'

# Step 2: Trigger SHORT with MACD Cross Down
curl -X POST https://your-domain.ngrok-free.dev/webhook/macd/cross-down \
  -H "Content-Type: application/json" \
  -d '{"ticker":"EURUSD","close":"1.0860"}'
```

---

## ⚠️ Important Notes

1. **RSI Moving Up/Down webhooks are now informational only** - they don't trigger trades
2. **MACD Cross events now trigger position opening** - when RSI condition is already set
3. **Position closing logic remains unchanged** - MACD Cross + MACD Moving in same direction
4. **Flags are automatically reset** after opening positions to prevent duplicate trades

---

## 📱 TradingView Alert Configuration

### For Opening Positions

**RSI Overbought Alert (>70):**
- Condition: RSI(14) > 70
- Webhook URL: `https://your-domain/webhook/rsi/crossed-up`

**MACD Bearish Cross:**
- Condition: MACD Line crosses below Signal Line
- Webhook URL: `https://your-domain/webhook/macd/cross-down`

**RSI Oversold Alert (<30):**
- Condition: RSI(14) < 30
- Webhook URL: `https://your-domain/webhook/rsi/crossed-down`

**MACD Bullish Cross:**
- Condition: MACD Line crosses above Signal Line
- Webhook URL: `https://your-domain/webhook/macd/cross-up`

### For Closing Positions

(Same as before - use MACD Moving Up/Down webhooks)

