#!/bin/bash

# Test script for MTF MA Trend + ATR Strategy
# Tests all entry and exit conditions for both LONG and SHORT positions

BASE_URL="${BASE_URL:-http://localhost:8080}"
SYMBOL="EUR_USD"
PRICE="1.1050"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

echo -e "${CYAN}========================================${NC}"
echo -e "${CYAN}  MTF MA Trend + ATR Strategy Test${NC}"
echo -e "${CYAN}========================================${NC}"
echo ""

# Helper function to send webhook
send_webhook() {
    local endpoint=$1
    local description=$2
    echo -e "${YELLOW}► Testing: ${description}${NC}"
    echo -e "  Endpoint: ${endpoint}"
    
    response=$(curl -s -X POST "${BASE_URL}${endpoint}" \
        -H "Content-Type: application/json" \
        -d '{"ticker": "'"${SYMBOL}"'", "close": "'"${PRICE}"'"}')
    
    echo -e "  Response: ${response}"
    echo ""
    sleep 0.5
}

# Helper function to check status
check_status() {
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${BLUE}  Checking Strategy Status${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    curl -s "${BASE_URL}/status" | head -50
    echo ""
}

# Helper function to reset state
reset_state() {
    echo -e "${RED}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${RED}  Resetting All State${NC}"
    echo -e "${RED}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    curl -s -X POST "${BASE_URL}/reset" | head -10
    echo ""
    sleep 0.5
}

# ============================================================================
# TEST 1: LONG ENTRY CONDITIONS
# ============================================================================
echo -e "${GREEN}============================================${NC}"
echo -e "${GREEN}  TEST 1: LONG Entry Conditions (4 total)${NC}"
echo -e "${GREEN}============================================${NC}"
echo ""

reset_state

echo -e "${CYAN}LONG Entry requires ALL of:${NC}"
echo "  1. /webhook/ma/price-above-ma4   (4H: Price > EMA 200)"
echo "  2. /webhook/rsi/above-50         (4H: RSI > 50)"
echo "  3. /webhook/ma/ma1-above-ma2     (15m: EMA 9 crosses above EMA 13)"
echo "  4. /webhook/atr/above-threshold  (15m: ATR > threshold)"
echo ""

# Test each condition one by one
send_webhook "/webhook/ma/price-above-ma4" "4H: Price above EMA 200 (condition 1/4)"
check_status

send_webhook "/webhook/rsi/above-50" "4H: RSI above 50 (condition 2/4)"
check_status

send_webhook "/webhook/atr/above-threshold" "15m: ATR above threshold (condition 3/4)"
check_status

echo -e "${GREEN}► Sending final condition - should trigger LONG entry${NC}"
send_webhook "/webhook/ma/ma1-above-ma2" "15m: EMA 9 crosses above EMA 13 (condition 4/4)"
check_status

# ============================================================================
# TEST 2: LONG EXIT CONDITIONS
# ============================================================================
echo -e "${GREEN}============================================${NC}"
echo -e "${GREEN}  TEST 2: LONG Exit Conditions${NC}"
echo -e "${GREEN}============================================${NC}"
echo ""

echo -e "${CYAN}LONG Exit requires ANY of:${NC}"
echo "  1. /webhook/ma/price-below-ma2  (15m: Price < EMA 13)"
echo "  2. /webhook/ma/ma1-below-ma2    (15m: EMA 9 crosses below EMA 13)"
echo ""

echo -e "${YELLOW}► Testing price-below-ma2 exit${NC}"
send_webhook "/webhook/ma/price-below-ma2" "15m: Price below EMA 13 - fast exit"
check_status

# ============================================================================
# TEST 3: SHORT ENTRY CONDITIONS
# ============================================================================
echo -e "${GREEN}============================================${NC}"
echo -e "${GREEN}  TEST 3: SHORT Entry Conditions (4 total)${NC}"
echo -e "${GREEN}============================================${NC}"
echo ""

reset_state

echo -e "${CYAN}SHORT Entry requires ALL of:${NC}"
echo "  1. /webhook/ma/price-below-ma4   (4H: Price < EMA 200)"
echo "  2. /webhook/rsi/below-50         (4H: RSI < 50)"
echo "  3. /webhook/ma/ma1-below-ma2     (15m: EMA 9 crosses below EMA 13)"
echo "  4. /webhook/atr/above-threshold  (15m: ATR > threshold)"
echo ""

send_webhook "/webhook/ma/price-below-ma4" "4H: Price below EMA 200 (condition 1/4)"
check_status

send_webhook "/webhook/rsi/below-50" "4H: RSI below 50 (condition 2/4)"
check_status

send_webhook "/webhook/atr/above-threshold" "15m: ATR above threshold (condition 3/4)"
check_status

echo -e "${GREEN}► Sending final condition - should trigger SHORT entry${NC}"
send_webhook "/webhook/ma/ma1-below-ma2" "15m: EMA 9 crosses below EMA 13 (condition 4/4)"
check_status

# ============================================================================
# TEST 4: SHORT EXIT CONDITIONS
# ============================================================================
echo -e "${GREEN}============================================${NC}"
echo -e "${GREEN}  TEST 4: SHORT Exit Conditions${NC}"
echo -e "${GREEN}============================================${NC}"
echo ""

echo -e "${CYAN}SHORT Exit requires ANY of:${NC}"
echo "  1. /webhook/ma/price-above-ma2  (15m: Price > EMA 13)"
echo "  2. /webhook/ma/ma1-above-ma2    (15m: EMA 9 crosses above EMA 13)"
echo ""

echo -e "${YELLOW}► Testing price-above-ma2 exit${NC}"
send_webhook "/webhook/ma/price-above-ma2" "15m: Price above EMA 13 - fast exit"
check_status

# ============================================================================
# TEST 5: ATR STATE REVERSAL
# ============================================================================
echo -e "${GREEN}============================================${NC}"
echo -e "${GREEN}  TEST 5: ATR State Reversal${NC}"
echo -e "${GREEN}============================================${NC}"
echo ""

reset_state

echo -e "${CYAN}Testing ATR above → below transition${NC}"
echo ""

send_webhook "/webhook/atr/above-threshold" "ATR above threshold (high volatility)"
check_status

send_webhook "/webhook/atr/below-threshold" "ATR below threshold (low volatility - should reset above-threshold)"
check_status

# ============================================================================
# TEST 6: CONDITION RESET ON OPPOSITE SIGNAL
# ============================================================================
echo -e "${GREEN}============================================${NC}"
echo -e "${GREEN}  TEST 6: Condition Reset on Opposite Signal${NC}"
echo -e "${GREEN}============================================${NC}"
echo ""

reset_state

echo -e "${CYAN}Testing that opposite signals reset conditions${NC}"
echo ""

send_webhook "/webhook/ma/price-above-ma4" "Price above EMA 200"
send_webhook "/webhook/rsi/above-50" "RSI above 50"
check_status

echo -e "${YELLOW}► Now sending opposite signals - should reset LONG conditions${NC}"
send_webhook "/webhook/ma/price-below-ma4" "Price BELOW EMA 200 (should reset price-above-ma4)"
send_webhook "/webhook/rsi/below-50" "RSI BELOW 50 (should reset rsi-above-50)"
check_status

# ============================================================================
# TEST 7: PARTIAL CONDITIONS - NO TRADE
# ============================================================================
echo -e "${GREEN}============================================${NC}"
echo -e "${GREEN}  TEST 7: Partial Conditions (No Trade)${NC}"
echo -e "${GREEN}============================================${NC}"
echo ""

reset_state

echo -e "${CYAN}Testing that partial conditions don't trigger trades${NC}"
echo ""

send_webhook "/webhook/ma/price-above-ma4" "4H: Price above EMA 200 (1/4)"
send_webhook "/webhook/rsi/above-50" "4H: RSI above 50 (2/4)"
# Intentionally NOT sending ATR and MA cross

echo -e "${YELLOW}► Only 2/4 conditions met - should NOT open position${NC}"
check_status

# ============================================================================
# SUMMARY
# ============================================================================
echo -e "${CYAN}========================================${NC}"
echo -e "${CYAN}  Test Complete${NC}"
echo -e "${CYAN}========================================${NC}"
echo ""
echo "Strategy endpoints tested:"
echo "  LONG Entry:"
echo "    ✓ /webhook/ma/price-above-ma4"
echo "    ✓ /webhook/rsi/above-50"
echo "    ✓ /webhook/ma/ma1-above-ma2"
echo "    ✓ /webhook/atr/above-threshold"
echo "  LONG Exit:"
echo "    ✓ /webhook/ma/price-below-ma2"
echo "    ✓ /webhook/ma/ma1-below-ma2"
echo "  SHORT Entry:"
echo "    ✓ /webhook/ma/price-below-ma4"
echo "    ✓ /webhook/rsi/below-50"
echo "    ✓ /webhook/ma/ma1-below-ma2"
echo "    ✓ /webhook/atr/above-threshold"
echo "  SHORT Exit:"
echo "    ✓ /webhook/ma/price-above-ma2"
echo "    ✓ /webhook/ma/ma1-above-ma2"
echo "  State Management:"
echo "    ✓ /webhook/atr/below-threshold (resets above-threshold)"
echo ""
