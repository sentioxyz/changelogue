#!/usr/bin/env bash
set -euo pipefail

RESOURCE_GROUP="${AZURE_RESOURCE_GROUP:-changelogue-res}"
APP_NAME="${AZURE_APP_NAME:-changelogue-app}"

if [ $# -lt 1 ]; then
  echo "Usage: $0 <github-username> [github-username ...]"
  echo "Adds GitHub usernames to ALLOWED_GITHUB_USERS on Azure App Service."
  exit 1
fi

# Get current value
current=$(az webapp config appsettings list \
  --resource-group "$RESOURCE_GROUP" \
  --name "$APP_NAME" \
  --query "[?name=='ALLOWED_GITHUB_USERS'].value | [0]" \
  --output tsv 2>/dev/null || echo "")

# Append new users (comma-separated, deduped)
for user in "$@"; do
  if [ -z "$current" ]; then
    current="$user"
  elif echo ",$current," | grep -qi ",$user,"; then
    echo "User '$user' already in allowlist, skipping."
  else
    current="$current,$user"
  fi
done

echo "Setting ALLOWED_GITHUB_USERS=$current"

az webapp config appsettings set \
  --resource-group "$RESOURCE_GROUP" \
  --name "$APP_NAME" \
  --settings "ALLOWED_GITHUB_USERS=$current" \
  --output table

echo "Done. App will restart automatically to pick up the change."
