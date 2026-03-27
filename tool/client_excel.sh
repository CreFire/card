#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

SERVER_DOCCONF_DIR="${REPO_ROOT}/server/configdoc/json"
CLIENT_ROOT="${REPO_ROOT}/client/wuziqi"
CLIENT_CONFIG_DIR="${CLIENT_ROOT}/generated/config/json"

mkdir -p "${CLIENT_CONFIG_DIR}"

if [ ! -d "${SERVER_DOCCONF_DIR}" ]; then
  echo "server docconf directory not found: ${SERVER_DOCCONF_DIR}" >&2
  exit 1
fi

find "${CLIENT_CONFIG_DIR}" -type f -name '*.json' -delete

if find "${SERVER_DOCCONF_DIR}" -maxdepth 1 -type f -name '*.json' | grep -q .; then
  cp "${SERVER_DOCCONF_DIR}"/*.json "${CLIENT_CONFIG_DIR}/"
fi

echo "client config synced to ${CLIENT_CONFIG_DIR}"
