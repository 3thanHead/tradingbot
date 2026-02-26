#!/bin/bash

# Test script for MA Trend + ATR Exit Strategy (simplified)
# Tests LONG and SHORT positions with OANDA integration
# Entry: MA1 above/below MA2
# Exit: ATR long/short signals

BASE_URL="${BASE_URL:-http://localhost:8080}"
SYMBOL="EUR_USD"
PRICE="1.1050"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m' # No Color

echo -e "${CYAN}========================================${NC}"
echo -e "${CYAN}  MA Trend + ATR Exit Strategy Test${NC}"
echo -e "${CYAN}  (With OANDA Position Verification)${NC}"
echo -e "${CYAN}========================================${NC}"
echo ""
echo "Entry: MA1 above MA2 (long) / MA1 below MA2 (short)"
echo "Exit:  ATR short (exits long) / ATR long (exits short)"
echo ""

# Helper function to send webhook
send_webhook() {
    local endpoint=$1
    local description=$2
    echo -e "${YELLOW}► ${description}${NC}"
    echo -e "  Endpoint: ${endpoint}"
    
    response=$(curl -s -X POST "${BASE_URL}${endpoint}" \
        -H "Content-Type: application/json" \
        -d '{"ticker": "'"${SYMBOL}"'", "close": "'"${PRICE}"'"}')
    
    echo -e "  Response: ${response}"
    echo ""
    sleep 1
}

# Helper function to check OANDA positions
check_oanda_positions() {
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${BLUE}  Checking OANDA Positions${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    
    # Check via bot's status endpoint
    echo -e "${CYAN}Bot Status:${NC}"
    curl -s "${BASE_URL}/status" | grep -E "(Position|positionOpen|position:|TradeID|OANDA)" | head -20
    echo ""
}

# Helper function to reset state
reset_state() {
    echo -e "${RED}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${RED}  Resetting All State${NC}"
    echo -e "${RED}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    curl -s -X POST "${BASE_URL}/reset" | head -10
    echo ""
    sleep 1
}

# ============================================================================
# TEST 1: LONG POSITION - Entry and Exit
# ============================================================================
echo -e "${GREEN}============================================${NC}"
echo -e "${GREEN}  TEST 1: LONG Position Cycle${NC}"
echo -e "${GREEN}============================================${NC}"
echo ""

reset_state

echo -e "${BOLD}Step 1: Trigger LONG entry (MA1 above MA2)${NC}"
echo -e "${CYAN}This should open a LONG position in OANDA${NC}"
echo ""
send_webhook "/webhook/ma/ma1-above-ma2" "MA1 above MA2 → Enter LONG"

echo -e "${CYAN}Waiting 3 seconds for OANDA order to process...${NC}"
sleep 3

check_oanda_positions

echo ""
echo -e "${BOLD}Step 2: Trigger LONG exit (ATR Short)${NC}"
echo -e "${CYAN}This should close the LONG position in OANDA${NC}"
echo ""
send_webhook "/webhook/atr/short" "ATR Short → Exit LONG"

echo -e "${CYAN}Waiting 3 seconds for OANDA order to process...${NC}"
sleep 3

check_oanda_positions

# ============================================================================
# TEST 2: SHORT POSITION - Entry and Exit
# ============================================================================
echo -e "${GREEN}============================================${NC}"
echo -e "${GREEN}  TEST 2: SHORT Position Cycle${NC}"
echo -e "${GREEN}============================================${NC}"
echo ""

reset_state

echo -e "${BOLD}Step 1: Trigger SHORT entry (MA1 below MA2)${NC}"
echo -e "${CYAN}This should open a SHORT position in OANDA${NC}"
echo ""
send_webhook "/webhook/ma/ma1-below-ma2" "MA1 below MA2 → Enter SHORT"

echo -e "${CYAN}Waiting 3 seconds for OANDA order to process...${NC}"
sleep 3

check_oanda_positions

echo ""
echo -e "${BOLD}Step 2: Trigger SHORT exit (ATR Long)${NC}"
echo -e "${CYAN}This should close the SHORT position in OANDA${NC}"
echo ""
send_webhook "/webhook/atr/long" "ATR Long → Exit SHORT"

echo -e "${CYAN}Waiting 3 seconds for OANDA order to process...${NC}"
sleep 3

check_oanda_positions

# ============================================================================
# FINAL STATUS
# ============================================================================
echo -e "${GREEN}============================================${NC}"
echo -e "${GREEN}  FINAL STATUS${NC}"
echo -e "${GREEN}============================================${NC}"
echo ""

echo -e "${CYAN}Full Bot Status:${NC}"
curl -s "${BASE_URL}/status" | head -80
echo ""

echo -e "${CYAN}========================================${NC}"
echo -e "${CYAN}  Test Complete!${NC}"
echo -e "${CYAN}========================================${NC}"
echo ""
echo "Check docker logs for detailed OANDA API calls:"
echo "  docker logs tradingview-webhook-bot --tail 100"
echo ""
