#!/usr/bin/env bash
#
# Deploy Changelogue to Azure App Service (Container).
# Idempotent — safe to re-run at any point.
#
# Usage:
#   # First-time setup (creates all resources):
#   ./scripts/azure-deploy.sh setup
#
#   # Redeploy after code changes:
#   ./scripts/azure-deploy.sh deploy
#
#   # Tear down everything:
#   ./scripts/azure-deploy.sh destroy
#
# Required environment variables (or set in .env):
#   AZURE_DB_PASSWORD   - PostgreSQL admin password
#
# Optional environment variables:
#   AZURE_LOCATION      - Azure region (default: westus)
#   AZURE_RG            - Resource group name (default: changelogue-res)
#   AZURE_ACR           - ACR name (default: changelogueacr)
#   AZURE_DB_SERVER     - Postgres server name (default: changelogue-db)
#   AZURE_APP_PLAN      - App Service plan name (default: changelogue-plan)
#   AZURE_APP_NAME      - Web app name (default: changelogue-app)
#   AZURE_APP_SKU       - App Service SKU (default: F1)
#   AZURE_APP_LOCATION  - App Service region, if different from AZURE_LOCATION
#   GOOGLE_API_KEY      - For AI features (optional)
#   NO_AUTH             - Skip API auth (default: true)

set -euo pipefail

# Load .env if present
if [[ -f .env ]]; then
  # shellcheck disable=SC1091
  source .env
fi

# --- Configuration ---
LOCATION="${AZURE_LOCATION:-westus}"
RG="${AZURE_RG:-changelogue-res}"
ACR="${AZURE_ACR:-changelogueacr}"
DB_SERVER="${AZURE_DB_SERVER:-changelogue-db}"
DB_NAME="changelogue"
DB_USER="pgadmin"
DB_PASSWORD="${AZURE_DB_PASSWORD:-}"
APP_PLAN="${AZURE_APP_PLAN:-changelogue-plan}"
APP_NAME="${AZURE_APP_NAME:-changelogue-app}"
APP_SKU="${AZURE_APP_SKU:-F1}"
APP_LOCATION="${AZURE_APP_LOCATION:-westus3}"
IMAGE="${ACR}.azurecr.io/changelogue:latest"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log()   { echo -e "${GREEN}[+]${NC} $*"; }
warn()  { echo -e "${YELLOW}[!]${NC} $*"; }
skip()  { echo -e "${YELLOW}[=]${NC} $* (already exists, skipping)"; }
error() { echo -e "${RED}[x]${NC} $*" >&2; exit 1; }

require_password() {
  if [[ -z "$DB_PASSWORD" ]]; then
    error "AZURE_DB_PASSWORD is required. Set it in .env or export it."
  fi
}

# Check if an Azure resource exists. Returns 0 if it does.
resource_exists() {
  local type="$1"
  shift
  case "$type" in
    rg)
      az group show --name "$1" --output none 2>/dev/null
      ;;
    acr)
      az acr show --name "$1" --output none 2>/dev/null
      ;;
    postgres)
      az postgres flexible-server show --resource-group "$1" --name "$2" --output none 2>/dev/null
      ;;
    postgres-db)
      az postgres flexible-server db show --resource-group "$1" --server-name "$2" --database-name "$3" --output none 2>/dev/null
      ;;
    plan)
      az appservice plan show --resource-group "$1" --name "$2" --output none 2>/dev/null
      ;;
    webapp)
      az webapp show --resource-group "$1" --name "$2" --output none 2>/dev/null
      ;;
    acr-image)
      az acr repository show --name "$1" --image "$2" --output none 2>/dev/null
      ;;
  esac
}

# --- Commands ---

cmd_setup() {
  require_password

  log "Checking Azure CLI login..."
  az account show > /dev/null 2>&1 || az login

  # Resource group
  if resource_exists rg "$RG"; then
    skip "Resource group: $RG"
  else
    log "Creating resource group: $RG"
    az group create --name "$RG" --location "$LOCATION" --output none
  fi

  # Container Registry
  if resource_exists acr "$ACR"; then
    skip "Azure Container Registry: $ACR"
  else
    log "Creating Azure Container Registry: $ACR"
    az acr create \
      --resource-group "$RG" \
      --name "$ACR" \
      --sku Basic \
      --admin-enabled true \
      --output none
  fi

  # PostgreSQL Flexible Server
  if resource_exists postgres "$RG" "$DB_SERVER"; then
    skip "PostgreSQL Flexible Server: $DB_SERVER"
  else
    log "Creating PostgreSQL Flexible Server: $DB_SERVER"
    az postgres flexible-server create \
      --resource-group "$RG" \
      --name "$DB_SERVER" \
      --admin-user "$DB_USER" \
      --admin-password "$DB_PASSWORD" \
      --sku-name Standard_B1ms \
      --tier Burstable \
      --version 16 \
      --public-access 0.0.0.0 \
      --output none \
      --yes
  fi

  # Database
  if resource_exists postgres-db "$RG" "$DB_SERVER" "$DB_NAME"; then
    skip "Database: $DB_NAME"
  else
    log "Creating database: $DB_NAME"
    az postgres flexible-server db create \
      --resource-group "$RG" \
      --server-name "$DB_SERVER" \
      --database-name "$DB_NAME" \
      --output none
  fi

  # Build container image (skip if image already exists in ACR)
  if resource_exists acr-image "$ACR" "changelogue:latest"; then
    skip "Container image: changelogue:latest"
  else
    log "Building container image in ACR..."
    az acr build --registry "$ACR" --image changelogue:latest . --no-logs
  fi

  # App Service plan
  if resource_exists plan "$RG" "$APP_PLAN"; then
    skip "App Service plan: $APP_PLAN"
  else
    log "Creating App Service plan: $APP_PLAN ($APP_SKU) in $APP_LOCATION"
    az appservice plan create \
      --name "$APP_PLAN" \
      --resource-group "$RG" \
      --sku "$APP_SKU" \
      --is-linux \
      --location "$APP_LOCATION" \
      --output none
  fi

  # Web App
  if resource_exists webapp "$RG" "$APP_NAME"; then
    skip "Web App: $APP_NAME"
    log "Updating container image..."
  else
    log "Creating Web App: $APP_NAME"
    az webapp create \
      --resource-group "$RG" \
      --plan "$APP_PLAN" \
      --name "$APP_NAME" \
      --container-image-name "$IMAGE" \
      --container-registry-url "https://${ACR}.azurecr.io" \
      --output none
  fi

  # Always update ACR credentials and container config
  log "Configuring ACR credentials for App Service..."
  ACR_PASSWORD=$(az acr credential show --name "$ACR" --query "passwords[0].value" -o tsv)
  az webapp config container set \
    --resource-group "$RG" \
    --name "$APP_NAME" \
    --container-image-name "$IMAGE" \
    --container-registry-url "https://${ACR}.azurecr.io" \
    --container-registry-user "$ACR" \
    --container-registry-password "$ACR_PASSWORD" \
    --output none

  DB_HOST="${DB_SERVER}.postgres.database.azure.com"
  DATABASE_URL="postgres://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:5432/${DB_NAME}?sslmode=require"

  # Always update app settings
  log "Setting app configuration..."
  SETTINGS="DATABASE_URL=$DATABASE_URL LISTEN_ADDR=:8080 WEBSITES_PORT=8080 NO_AUTH=${NO_AUTH:-true}"
  if [[ -n "${GOOGLE_API_KEY:-}" ]]; then
    SETTINGS="$SETTINGS GOOGLE_API_KEY=$GOOGLE_API_KEY"
  fi
  if [[ -n "${LLM_PROVIDER:-}" ]]; then
    SETTINGS="$SETTINGS LLM_PROVIDER=$LLM_PROVIDER"
  fi
  if [[ -n "${LLM_MODEL:-}" ]]; then
    SETTINGS="$SETTINGS LLM_MODEL=$LLM_MODEL"
  fi
  if [[ -n "${OPENAI_API_KEY:-}" ]]; then
    SETTINGS="$SETTINGS OPENAI_API_KEY=$OPENAI_API_KEY"
  fi
  if [[ -n "${OPENAI_BASE_URL:-}" ]]; then
    SETTINGS="$SETTINGS OPENAI_BASE_URL=$OPENAI_BASE_URL"
  fi
  az webapp config appsettings set \
    --resource-group "$RG" \
    --name "$APP_NAME" \
    --settings $SETTINGS \
    --output none

  log "Restarting App Service..."
  az webapp restart --resource-group "$RG" --name "$APP_NAME"

  log "Setup complete!"
  echo ""
  echo "  App URL: https://${APP_NAME}.azurewebsites.net"
  echo "  API:     https://${APP_NAME}.azurewebsites.net/api/v1/health"
  echo ""
  echo "  To redeploy after code changes: $0 deploy"
}

cmd_deploy() {
  log "Checking Azure CLI login..."
  az account show > /dev/null 2>&1 || az login

  log "Building container image in ACR..."
  az acr build --registry "$ACR" --image changelogue:latest . --no-logs

  log "Restarting App Service..."
  az webapp restart --resource-group "$RG" --name "$APP_NAME"

  log "Deployed!"
  echo ""
  echo "  App URL: https://${APP_NAME}.azurewebsites.net"
}

cmd_destroy() {
  warn "This will delete ALL resources in resource group: $RG"
  read -rp "Type the resource group name to confirm: " confirm
  if [[ "$confirm" != "$RG" ]]; then
    error "Aborted."
  fi

  log "Deleting resource group: $RG (this may take a few minutes)..."
  az group delete --name "$RG" --yes --no-wait

  log "Deletion initiated. Resources will be removed in the background."
}

cmd_logs() {
  log "Streaming logs from $APP_NAME..."
  az webapp log tail --resource-group "$RG" --name "$APP_NAME"
}

# --- Main ---
case "${1:-}" in
  setup)   cmd_setup ;;
  deploy)  cmd_deploy ;;
  destroy) cmd_destroy ;;
  logs)    cmd_logs ;;
  *)
    echo "Usage: $0 {setup|deploy|destroy|logs}"
    echo ""
    echo "  setup   - Create all Azure resources and deploy (idempotent)"
    echo "  deploy  - Rebuild image and restart (after code changes)"
    echo "  destroy - Delete all Azure resources"
    echo "  logs    - Stream app logs"
    exit 1
    ;;
esac
