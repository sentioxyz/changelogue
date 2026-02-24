#!/usr/bin/env bash
set -euo pipefail

# Integration test for the ingestion layer.
# Requires: docker, go, curl
#
# Spins up Postgres, starts the server, sends a GitHub webhook,
# verifies the release was persisted, then cleans up.

CONTAINER_NAME="releaseguard-test-pg"
DB_PORT=5433
DB_USER="postgres"
DB_PASS="testpass"
DB_NAME="releaseguard"
DATABASE_URL="postgres://${DB_USER}:${DB_PASS}@localhost:${DB_PORT}/${DB_NAME}?sslmode=disable"
WEBHOOK_SECRET="integration-test-secret"
SERVER_PORT=8089
SERVER_PID=""

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

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
    echo -e "${RED}FAIL: $1${NC}" >&2
    exit 1
}

pass() {
    echo -e "${GREEN}PASS: $1${NC}"
}

# --- 1. Start Postgres ---
echo "--- Starting Postgres on port $DB_PORT ---"
docker rm -f "$CONTAINER_NAME" 2>/dev/null || true
docker run -d --name "$CONTAINER_NAME" \
    -e POSTGRES_DB="$DB_NAME" \
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
        fail "Postgres did not become ready in 30s"
    fi
    sleep 1
done

# --- 2. Build and start server ---
echo ""
echo "--- Building server ---"
go build -o /tmp/releaseguard-test ./cmd/server/
pass "Build succeeded"

echo ""
echo "--- Starting server ---"
DATABASE_URL="$DATABASE_URL" \
GITHUB_WEBHOOK_SECRET="$WEBHOOK_SECRET" \
LISTEN_ADDR=":${SERVER_PORT}" \
    /tmp/releaseguard-test &
SERVER_PID=$!
sleep 2

if ! kill -0 "$SERVER_PID" 2>/dev/null; then
    fail "Server failed to start"
fi
pass "Server started (PID $SERVER_PID)"

# --- 3. Send a valid GitHub release webhook ---
echo ""
echo "--- Sending GitHub release webhook ---"

PAYLOAD='{"action":"published","release":{"tag_name":"v2.0.0","body":"## Release Notes\n* New feature","prerelease":false,"published_at":"2024-06-01T12:00:00Z"},"repository":{"full_name":"testorg/testrepo"}}'

# Compute HMAC-SHA256 signature
SIGNATURE=$(printf '%s' "$PAYLOAD" | openssl dgst -sha256 -hmac "$WEBHOOK_SECRET" | awk '{print $NF}')

HTTP_CODE=$(curl -s -o /tmp/webhook-response.txt -w "%{http_code}" \
    -X POST "http://localhost:${SERVER_PORT}/webhook/github" \
    -H "Content-Type: application/json" \
    -H "X-GitHub-Event: release" \
    -H "X-Hub-Signature-256: sha256=${SIGNATURE}" \
    -d "$PAYLOAD")

if [ "$HTTP_CODE" != "200" ]; then
    fail "Webhook returned HTTP $HTTP_CODE (expected 200). Body: $(cat /tmp/webhook-response.txt)"
fi
pass "Webhook accepted (HTTP 200)"

# Give server a moment to process
sleep 1

# --- 4. Verify release was persisted ---
echo ""
echo "--- Verifying release in database ---"

ROW_COUNT=$(docker exec "$CONTAINER_NAME" psql -U "$DB_USER" -d "$DB_NAME" -tAc \
    "SELECT COUNT(*) FROM releases WHERE repository='testorg/testrepo' AND version='v2.0.0';")

if [ "$ROW_COUNT" != "1" ]; then
    fail "Expected 1 release row, got: '$ROW_COUNT'"
fi
pass "Release persisted in database"

# Verify payload contents
SOURCE=$(docker exec "$CONTAINER_NAME" psql -U "$DB_USER" -d "$DB_NAME" -tAc \
    "SELECT source FROM releases WHERE repository='testorg/testrepo' AND version='v2.0.0';")

if [ "$SOURCE" != "github" ]; then
    fail "Expected source='github', got: '$SOURCE'"
fi
pass "Release source is correct"

# --- 5. Verify River job was enqueued ---
echo ""
echo "--- Verifying River job was enqueued ---"

JOB_COUNT=$(docker exec "$CONTAINER_NAME" psql -U "$DB_USER" -d "$DB_NAME" -tAc \
    "SELECT COUNT(*) FROM river_job WHERE kind='pipeline_process';")

if [ "$JOB_COUNT" -lt 1 ]; then
    fail "Expected at least 1 River job, got: '$JOB_COUNT'"
fi
pass "River pipeline_process job enqueued"

# --- 6. Send duplicate webhook (idempotent skip) ---
echo ""
echo "--- Sending duplicate webhook (should be skipped) ---"

HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST "http://localhost:${SERVER_PORT}/webhook/github" \
    -H "Content-Type: application/json" \
    -H "X-GitHub-Event: release" \
    -H "X-Hub-Signature-256: sha256=${SIGNATURE}" \
    -d "$PAYLOAD")

if [ "$HTTP_CODE" != "200" ]; then
    fail "Duplicate webhook returned HTTP $HTTP_CODE (expected 200)"
fi

sleep 1

ROW_COUNT=$(docker exec "$CONTAINER_NAME" psql -U "$DB_USER" -d "$DB_NAME" -tAc \
    "SELECT COUNT(*) FROM releases WHERE repository='testorg/testrepo' AND version='v2.0.0';")

if [ "$ROW_COUNT" != "1" ]; then
    fail "Duplicate was not skipped — got $ROW_COUNT rows instead of 1"
fi
pass "Duplicate correctly skipped (still 1 row)"

# --- 7. Send webhook with invalid signature ---
echo ""
echo "--- Sending webhook with invalid signature ---"

HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" \
    -X POST "http://localhost:${SERVER_PORT}/webhook/github" \
    -H "Content-Type: application/json" \
    -H "X-GitHub-Event: release" \
    -H "X-Hub-Signature-256: sha256=invalidsignature" \
    -d "$PAYLOAD")

if [ "$HTTP_CODE" != "403" ]; then
    fail "Invalid signature returned HTTP $HTTP_CODE (expected 403)"
fi
pass "Invalid signature rejected (HTTP 403)"

# --- Done ---
echo ""
echo -e "${GREEN}=== All integration tests passed ===${NC}"
