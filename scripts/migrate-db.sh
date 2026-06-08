#!/usr/bin/env bash
#
# Migrate the Changelogue PostgreSQL database from Azure (Flexible Server)
# into the in-cluster Zalando postgres-operator instance on sentio-sea.
#
# Strategy: one-time cutover with a short write freeze.
#   1. Quiesce the Azure app (stop new writes / drain River jobs).
#   2. pg_dump the Azure database to a local file.
#   3. Create the target database in the cluster (idempotent).
#   4. Restore into the cluster DB through a kubectl port-forward.
# The Changelogue app re-creates its schema on boot (CREATE TABLE IF NOT EXISTS
# + River migrations), so a full dump/restore into an empty DB is safe.
#
# Requirements: az CLI (logged in), kubectl (sea context), pg_dump/psql >= 16.
#
# Usage:
#   ./scripts/migrate-db.sh dump      # dump Azure -> ./changelogue-dump.sql
#   ./scripts/migrate-db.sh restore   # create DB + restore dump into cluster
#   ./scripts/migrate-db.sh all       # dump then restore
#
set -euo pipefail

# --- Source (Azure) ---
AZURE_DB_SERVER="${AZURE_DB_SERVER:-changelogue-db}"
AZURE_DB_HOST="${AZURE_DB_HOST:-${AZURE_DB_SERVER}.postgres.database.azure.com}"
AZURE_DB_NAME="${AZURE_DB_NAME:-changelogue}"
AZURE_DB_USER="${AZURE_DB_USER:-pgadmin}"
AZURE_DB_PASSWORD="${AZURE_DB_PASSWORD:-}"   # required for dump
AZURE_RG="${AZURE_RG:-changelogue-res}"
AZURE_APP_NAME="${AZURE_APP_NAME:-changelogue-app}"

# --- Target (sentio-sea, Zalando operator) ---
KUBE_CONTEXT="${KUBE_CONTEXT:-sentio-sea}"
PG_NAMESPACE="${PG_NAMESPACE:-postgres}"
PG_SERVICE="${PG_SERVICE:-prod-postgres}"
TARGET_DB_NAME="${TARGET_DB_NAME:-changelogue-db}"
TARGET_DB_USER="${TARGET_DB_USER:-postgres}"
TARGET_DB_PASSWORD="${TARGET_DB_PASSWORD:-}"  # postgres superuser password; required for restore
LOCAL_PORT="${LOCAL_PORT:-55432}"

DUMP_FILE="${DUMP_FILE:-./changelogue-dump.sql}"

GREEN='\033[0;32m'; YELLOW='\033[1;33m'; RED='\033[0;31m'; NC='\033[0m'
log()   { echo -e "${GREEN}[+]${NC} $*"; }
warn()  { echo -e "${YELLOW}[!]${NC} $*"; }
error() { echo -e "${RED}[x]${NC} $*" >&2; exit 1; }

quiesce_azure() {
  if command -v az >/dev/null 2>&1; then
    warn "Stopping Azure app '$AZURE_APP_NAME' to freeze writes and drain jobs..."
    az webapp stop --resource-group "$AZURE_RG" --name "$AZURE_APP_NAME" --output none || \
      warn "Could not stop app (continuing — ensure it is not writing)."
  else
    warn "az CLI not found; ensure the Azure app is stopped before dumping."
  fi
}

cmd_dump() {
  [[ -n "$AZURE_DB_PASSWORD" ]] || error "AZURE_DB_PASSWORD is required for dump."
  quiesce_azure
  log "Dumping $AZURE_DB_NAME from $AZURE_DB_HOST -> $DUMP_FILE"
  # --no-owner/--no-privileges: roles differ between Azure and the cluster.
  # --no-acl: skip GRANTs. Full dump includes schema + data (River + app tables).
  PGPASSWORD="$AZURE_DB_PASSWORD" pg_dump \
    --host="$AZURE_DB_HOST" \
    --port=5432 \
    --username="$AZURE_DB_USER" \
    --dbname="$AZURE_DB_NAME" \
    --no-owner --no-privileges --no-acl \
    --format=plain \
    --file="$DUMP_FILE" \
    "sslmode=require"
  log "Dump complete: $(du -h "$DUMP_FILE" | cut -f1)"
}

start_port_forward() {
  log "Port-forwarding svc/$PG_SERVICE ($PG_NAMESPACE) -> localhost:$LOCAL_PORT"
  kubectl --context "$KUBE_CONTEXT" -n "$PG_NAMESPACE" \
    port-forward "svc/$PG_SERVICE" "$LOCAL_PORT:5432" >/tmp/clg-pf.log 2>&1 &
  PF_PID=$!
  trap 'kill "$PF_PID" 2>/dev/null || true' EXIT
  sleep 4
  kill -0 "$PF_PID" 2>/dev/null || error "port-forward failed; see /tmp/clg-pf.log"
}

cmd_restore() {
  [[ -f "$DUMP_FILE" ]] || error "Dump file not found: $DUMP_FILE (run 'dump' first)."
  [[ -n "$TARGET_DB_PASSWORD" ]] || error "TARGET_DB_PASSWORD (postgres superuser) is required."
  start_port_forward
  local conn="host=localhost port=$LOCAL_PORT user=$TARGET_DB_USER sslmode=disable"

  log "Creating database '$TARGET_DB_NAME' if absent..."
  PGPASSWORD="$TARGET_DB_PASSWORD" psql "$conn dbname=postgres" -tc \
    "SELECT 1 FROM pg_database WHERE datname='$TARGET_DB_NAME'" | grep -q 1 || \
  PGPASSWORD="$TARGET_DB_PASSWORD" psql "$conn dbname=postgres" -c \
    "CREATE DATABASE \"$TARGET_DB_NAME\""

  log "Restoring dump into '$TARGET_DB_NAME'..."
  PGPASSWORD="$TARGET_DB_PASSWORD" psql \
    "$conn dbname=$TARGET_DB_NAME" \
    --set ON_ERROR_STOP=on \
    --single-transaction \
    --file="$DUMP_FILE"
  log "Restore complete."

  log "Row counts (sanity check):"
  PGPASSWORD="$TARGET_DB_PASSWORD" psql "$conn dbname=$TARGET_DB_NAME" -c \
    "SELECT 'projects' t, count(*) FROM projects
     UNION ALL SELECT 'sources', count(*) FROM sources
     UNION ALL SELECT 'releases', count(*) FROM releases
     UNION ALL SELECT 'semantic_releases', count(*) FROM semantic_releases;" || true
}

case "${1:-}" in
  dump)    cmd_dump ;;
  restore) cmd_restore ;;
  all)     cmd_dump; cmd_restore ;;
  *)
    echo "Usage: $0 {dump|restore|all}"
    echo "  dump    - stop Azure app and pg_dump to \$DUMP_FILE ($DUMP_FILE)"
    echo "  restore - create \$TARGET_DB_NAME and psql-restore via port-forward"
    echo "  all     - dump then restore"
    exit 1 ;;
esac
