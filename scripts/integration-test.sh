#!/usr/bin/env bash
set -euo pipefail

# Integration test for Changelogue.
# Requires: docker, go, curl, jq
#
# Tests:
#   1. Health endpoint
#   2. Project CRUD via API
#   3. Source creation via API
#   4. Channel & subscription CRUD

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# --- Config ---
CONTAINER_NAME="changelogue-test-pg"
DB_PORT=5433
DB_USER="postgres"
DB_PASS="testpass"
DB_NAME="changelogue_test"
DATABASE_URL="postgres://${DB_USER}:${DB_PASS}@localhost:${DB_PORT}/${DB_NAME}?sslmode=disable"
SERVER_PORT=8089
SERVER_PID=""
BASE_URL="http://localhost:${SERVER_PORT}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BOLD='\033[1m'
NC='\033[0m'

TESTS_PASSED=0
TESTS_FAILED=0
TESTS_TOTAL=0

# --- Helpers ---
cleanup() {
    echo ""
    echo "--- Cleanup ---"
    if [ -n "$SERVER_PID" ] && kill -0 "$SERVER_PID" 2>/dev/null; then
        kill "$SERVER_PID" 2>/dev/null || true
        wait "$SERVER_PID" 2>/dev/null || true
        echo "Server stopped"
    fi
    docker rm -f "$CONTAINER_NAME" 2>/dev/null || true
    echo "Postgres container removed"
}
trap cleanup EXIT

fail() {
    TESTS_FAILED=$((TESTS_FAILED + 1))
    TESTS_TOTAL=$((TESTS_TOTAL + 1))
    echo -e "  ${RED}FAIL: $1${NC}" >&2
}

pass() {
    TESTS_PASSED=$((TESTS_PASSED + 1))
    TESTS_TOTAL=$((TESTS_TOTAL + 1))
    echo -e "  ${GREEN}PASS: $1${NC}"
}

section() {
    echo ""
    echo -e "${BOLD}--- $1 ---${NC}"
}

# Make an API call. Sets globals: HTTP_CODE, API_BODY.
# Usage: api METHOD /path [-d 'json']
api() {
    local method="$1" path="$2"
    shift 2
    HTTP_CODE=$(curl -s -o /tmp/api-response.txt -w "%{http_code}" -X "$method" "${BASE_URL}${path}" \
        -H "Content-Type: application/json" "$@")
    API_BODY=$(cat /tmp/api-response.txt)
}

# Extract .data.id from API_BODY
extract_id() {
    echo "$API_BODY" | jq -r '.data.id'
}

# --- 1. Start Postgres ---
section "Starting Postgres on port $DB_PORT"
docker rm -f "$CONTAINER_NAME" 2>/dev/null || true
docker run -d --name "$CONTAINER_NAME" \
    -e POSTGRES_DB="$DB_NAME" \
    -e POSTGRES_USER="$DB_USER" \
    -e POSTGRES_PASSWORD="$DB_PASS" \
    -p "${DB_PORT}:5432" \
    postgres:16 >/dev/null

echo "Waiting for Postgres..."
for i in $(seq 1 30); do
    if docker exec "$CONTAINER_NAME" pg_isready -U "$DB_USER" >/dev/null 2>&1; then
        echo "Postgres ready"
        break
    fi
    if [ "$i" -eq 30 ]; then
        echo "Postgres did not become ready in 30s"
        exit 1
    fi
    sleep 1
done

# --- 2. Build and start server ---
section "Building server"
cd "$PROJECT_ROOT"
go build -o /tmp/changelogue-test ./cmd/server/
pass "Build succeeded"

section "Starting server"
DATABASE_URL="$DATABASE_URL" \
LISTEN_ADDR=":${SERVER_PORT}" \
NO_AUTH=true \
    /tmp/changelogue-test >/tmp/changelogue-test.log 2>&1 &
SERVER_PID=$!
sleep 2

if ! kill -0 "$SERVER_PID" 2>/dev/null; then
    echo "Server failed to start"
    exit 1
fi
pass "Server started (PID $SERVER_PID)"

# ============================================================
# TEST SUITE
# ============================================================

# --- 3. Health check ---
section "Health check"
api GET "/api/v1/health"
if [ "$HTTP_CODE" = "200" ]; then
    pass "Health endpoint returned 200"
else
    fail "Health endpoint returned $HTTP_CODE (expected 200)"
fi

# --- 4. Project CRUD ---
section "Project CRUD"

# Create project
api POST "/api/v1/projects" -d '{"name":"integration-test-project","description":"Created by integration test"}'
if [ "$HTTP_CODE" = "201" ]; then
    PROJECT_ID=$(extract_id)
    pass "Create project (id: $PROJECT_ID)"
else
    fail "Create project returned $HTTP_CODE (expected 201). Body: $API_BODY"
    PROJECT_ID=""
fi

# Get project
if [ -n "$PROJECT_ID" ]; then
    api GET "/api/v1/projects/$PROJECT_ID"
    if [ "$HTTP_CODE" = "200" ]; then
        pass "Get project"
    else
        fail "Get project returned $HTTP_CODE (expected 200)"
    fi
fi

# List projects
api GET "/api/v1/projects"
if [ "$HTTP_CODE" = "200" ]; then
    pass "List projects"
else
    fail "List projects returned $HTTP_CODE (expected 200)"
fi

# --- 5. Source CRUD ---
section "Source CRUD"

if [ -n "$PROJECT_ID" ]; then
    # Create source (github provider, polling-based)
    api POST "/api/v1/projects/$PROJECT_ID/sources" \
        -d '{"provider":"github","repository":"testorg/testrepo","poll_interval_seconds":3600,"enabled":true}'
    if [ "$HTTP_CODE" = "201" ]; then
        SOURCE_ID=$(extract_id)
        pass "Create source (id: $SOURCE_ID)"
    else
        fail "Create source returned $HTTP_CODE (expected 201). Body: $API_BODY"
        SOURCE_ID=""
    fi
else
    fail "Skipping source tests — no project"
    SOURCE_ID=""
fi

# --- 6. Channel & subscription CRUD ---
section "Channel & subscription CRUD"

# Create channel
api POST "/api/v1/channels" \
    -d '{"name":"test-webhook","type":"webhook","config":{"url":"https://example.com/hook"}}'
if [ "$HTTP_CODE" = "201" ]; then
    CHANNEL_ID=$(extract_id)
    pass "Create channel (id: $CHANNEL_ID)"
else
    fail "Create channel returned $HTTP_CODE (expected 201). Body: $API_BODY"
    CHANNEL_ID=""
fi

# Create source subscription
if [ -n "$CHANNEL_ID" ] && [ -n "$SOURCE_ID" ]; then
    api POST "/api/v1/subscriptions" \
        -d "{\"type\":\"source\",\"channel_id\":\"$CHANNEL_ID\",\"source_id\":\"$SOURCE_ID\"}"
    if [ "$HTTP_CODE" = "201" ]; then
        SUB_ID=$(extract_id)
        pass "Create source subscription (id: $SUB_ID)"
    else
        fail "Create subscription returned $HTTP_CODE (expected 201). Body: $API_BODY"
        SUB_ID=""
    fi
fi

# List subscriptions
api GET "/api/v1/subscriptions"
if [ "$HTTP_CODE" = "200" ]; then
    pass "List subscriptions"
else
    fail "List subscriptions returned $HTTP_CODE (expected 200)"
fi

# Delete subscription
if [ -n "${SUB_ID:-}" ]; then
    api DELETE "/api/v1/subscriptions/$SUB_ID"
    if [ "$HTTP_CODE" = "204" ]; then
        pass "Delete subscription"
    else
        fail "Delete subscription returned $HTTP_CODE (expected 204)"
    fi
fi

# Delete channel
if [ -n "${CHANNEL_ID:-}" ]; then
    api DELETE "/api/v1/channels/$CHANNEL_ID"
    if [ "$HTTP_CODE" = "204" ]; then
        pass "Delete channel"
    else
        fail "Delete channel returned $HTTP_CODE (expected 204)"
    fi
fi

# --- 7. Cleanup test data ---
section "Cleanup test data"

if [ -n "$PROJECT_ID" ]; then
    api DELETE "/api/v1/projects/$PROJECT_ID"
    if [ "$HTTP_CODE" = "204" ]; then
        pass "Delete project (cascades sources, releases)"
    else
        fail "Delete project returned $HTTP_CODE (expected 204)"
    fi
fi

# ============================================================
# SUMMARY
# ============================================================
echo ""
echo -e "${BOLD}============================================================${NC}"
echo -e "${BOLD}  Test Results: $TESTS_PASSED passed, $TESTS_FAILED failed (${TESTS_TOTAL} total)${NC}"
echo -e "${BOLD}============================================================${NC}"

if [ "$TESTS_FAILED" -gt 0 ]; then
    echo -e "${RED}  SOME TESTS FAILED${NC}"
    exit 1
else
    echo -e "${GREEN}  ALL TESTS PASSED${NC}"
    exit 0
fi
