#!/usr/bin/env bash
set -euo pipefail

# Integration test for ReleaseBeacon.
# Requires: docker, go, curl, jq
#
# Tests:
#   1. Health endpoint
#   2. Project CRUD via API
#   3. Source creation via API
#   4. GitHub webhook → release ingestion → River job
#   5. Duplicate webhook idempotency
#   6. Invalid signature rejection
#   7. Channel & subscription CRUD

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# --- Config ---
CONTAINER_NAME="releaseguard-test-pg"
DB_PORT=5433
DB_USER="postgres"
DB_PASS="testpass"
DB_NAME="releaseguard_test"
DATABASE_URL="postgres://${DB_USER}:${DB_PASS}@localhost:${DB_PORT}/${DB_NAME}?sslmode=disable"
WEBHOOK_SECRET="integration-test-secret"
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

# Make an API call and return the response body. Sets HTTP_CODE global.
api() {
    local method="$1" path="$2"
    shift 2
    local response
    response=$(curl -s -w "\n%{http_code}" -X "$method" "${BASE_URL}${path}" \
        -H "Content-Type: application/json" "$@")
    HTTP_CODE=$(echo "$response" | tail -1)
    echo "$response" | sed '$d'
}

# Extract .data.id from JSON response
extract_id() {
    echo "$1" | jq -r '.data.id'
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
go build -o /tmp/releaseguard-test ./cmd/server/
pass "Build succeeded"

section "Starting server"
DATABASE_URL="$DATABASE_URL" \
GITHUB_WEBHOOK_SECRET="$WEBHOOK_SECRET" \
LISTEN_ADDR=":${SERVER_PORT}" \
NO_AUTH=true \
    /tmp/releaseguard-test &
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
BODY=$(api POST "/api/v1/projects" -d '{"name":"integration-test-project","description":"Created by integration test"}')
if [ "$HTTP_CODE" = "201" ]; then
    PROJECT_ID=$(extract_id "$BODY")
    pass "Create project (id: $PROJECT_ID)"
else
    fail "Create project returned $HTTP_CODE (expected 201). Body: $BODY"
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
    # Create source (github provider matching the webhook repo)
    BODY=$(api POST "/api/v1/projects/$PROJECT_ID/sources" \
        -d '{"provider":"github","repository":"testorg/testrepo","poll_interval_seconds":3600}')
    if [ "$HTTP_CODE" = "201" ]; then
        SOURCE_ID=$(extract_id "$BODY")
        pass "Create source (id: $SOURCE_ID)"
    else
        fail "Create source returned $HTTP_CODE (expected 201). Body: $BODY"
        SOURCE_ID=""
    fi
else
    fail "Skipping source tests — no project"
    SOURCE_ID=""
fi

# --- 6. GitHub webhook → release ingestion ---
section "GitHub webhook flow"

PAYLOAD='{"action":"published","release":{"tag_name":"v2.0.0","body":"## Release Notes\n* New feature","prerelease":false,"published_at":"2024-06-01T12:00:00Z"},"repository":{"full_name":"testorg/testrepo"}}'
SIGNATURE=$(printf '%s' "$PAYLOAD" | openssl dgst -sha256 -hmac "$WEBHOOK_SECRET" | awk '{print $NF}')

HTTP_CODE=$(curl -s -o /tmp/webhook-response.txt -w "%{http_code}" \
    -X POST "${BASE_URL}/webhook/github" \
    -H "Content-Type: application/json" \
    -H "X-GitHub-Event: release" \
    -H "X-Hub-Signature-256: sha256=${SIGNATURE}" \
    -d "$PAYLOAD")

if [ "$HTTP_CODE" = "200" ]; then
    pass "Webhook accepted (HTTP 200)"
else
    fail "Webhook returned HTTP $HTTP_CODE (expected 200). Body: $(cat /tmp/webhook-response.txt)"
fi

sleep 1

# Verify release in database (using source_id + version, the actual schema)
if [ -n "$SOURCE_ID" ]; then
    ROW_COUNT=$(docker exec "$CONTAINER_NAME" psql -U "$DB_USER" -d "$DB_NAME" -tAc \
        "SELECT COUNT(*) FROM releases WHERE source_id='$SOURCE_ID' AND version='v2.0.0';")
    if [ "$ROW_COUNT" = "1" ]; then
        pass "Release persisted in database (source_id=$SOURCE_ID, version=v2.0.0)"
    else
        fail "Expected 1 release row, got: '$ROW_COUNT'"
    fi
fi

# Verify release via API
if [ -n "$SOURCE_ID" ]; then
    BODY=$(api GET "/api/v1/sources/$SOURCE_ID/releases")
    if [ "$HTTP_CODE" = "200" ]; then
        RELEASE_COUNT=$(echo "$BODY" | jq '.data | length')
        if [ "$RELEASE_COUNT" = "1" ]; then
            pass "Release visible via API (1 release for source)"
        else
            fail "Expected 1 release via API, got: $RELEASE_COUNT"
        fi
    else
        fail "List releases returned $HTTP_CODE (expected 200)"
    fi
fi

# Verify River job was enqueued
JOB_COUNT=$(docker exec "$CONTAINER_NAME" psql -U "$DB_USER" -d "$DB_NAME" -tAc \
    "SELECT COUNT(*) FROM river_job WHERE kind='notify_release';")
if [ "$JOB_COUNT" -ge 1 ]; then
    pass "River notify_release job enqueued"
else
    fail "Expected at least 1 notify_release job, got: '$JOB_COUNT'"
fi

# --- 7. Duplicate webhook (idempotent skip) ---
section "Duplicate webhook idempotency"

HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST "${BASE_URL}/webhook/github" \
    -H "Content-Type: application/json" \
    -H "X-GitHub-Event: release" \
    -H "X-Hub-Signature-256: sha256=${SIGNATURE}" \
    -d "$PAYLOAD")

if [ "$HTTP_CODE" = "200" ]; then
    pass "Duplicate webhook accepted (HTTP 200)"
else
    fail "Duplicate webhook returned HTTP $HTTP_CODE (expected 200)"
fi

sleep 1

if [ -n "$SOURCE_ID" ]; then
    ROW_COUNT=$(docker exec "$CONTAINER_NAME" psql -U "$DB_USER" -d "$DB_NAME" -tAc \
        "SELECT COUNT(*) FROM releases WHERE source_id='$SOURCE_ID' AND version='v2.0.0';")
    if [ "$ROW_COUNT" = "1" ]; then
        pass "Duplicate correctly skipped (still 1 row)"
    else
        fail "Duplicate was not skipped — got $ROW_COUNT rows instead of 1"
    fi
fi

# --- 8. Invalid signature rejection ---
section "Invalid signature rejection"

HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST "${BASE_URL}/webhook/github" \
    -H "Content-Type: application/json" \
    -H "X-GitHub-Event: release" \
    -H "X-Hub-Signature-256: sha256=invalidsignature" \
    -d "$PAYLOAD")

if [ "$HTTP_CODE" = "403" ]; then
    pass "Invalid signature rejected (HTTP 403)"
else
    fail "Invalid signature returned HTTP $HTTP_CODE (expected 403)"
fi

# --- 9. Channel & subscription CRUD ---
section "Channel & subscription CRUD"

# Create channel
BODY=$(api POST "/api/v1/channels" \
    -d '{"name":"test-webhook","type":"webhook","config":{"url":"https://example.com/hook"}}')
if [ "$HTTP_CODE" = "201" ]; then
    CHANNEL_ID=$(extract_id "$BODY")
    pass "Create channel (id: $CHANNEL_ID)"
else
    fail "Create channel returned $HTTP_CODE (expected 201). Body: $BODY"
    CHANNEL_ID=""
fi

# Create source subscription
if [ -n "$CHANNEL_ID" ] && [ -n "$SOURCE_ID" ]; then
    BODY=$(api POST "/api/v1/subscriptions" \
        -d "{\"type\":\"source\",\"channel_id\":\"$CHANNEL_ID\",\"source_id\":\"$SOURCE_ID\"}")
    if [ "$HTTP_CODE" = "201" ]; then
        SUB_ID=$(extract_id "$BODY")
        pass "Create source subscription (id: $SUB_ID)"
    else
        fail "Create subscription returned $HTTP_CODE (expected 201). Body: $BODY"
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

# --- 10. Cleanup test data ---
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
