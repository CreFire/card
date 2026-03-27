#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

PROTO_SRC_DIR="${REPO_ROOT}/proto"
CLIENT_ROOT="${REPO_ROOT}/client/wuziqi"
CLIENT_PROTO_DIR="${CLIENT_ROOT}/proto/pb"

mkdir -p "${CLIENT_PROTO_DIR}"

if [ ! -d "${PROTO_SRC_DIR}" ]; then
  echo "proto source directory not found: ${PROTO_SRC_DIR}" >&2
  exit 1
fi

find "${CLIENT_PROTO_DIR}" -type f -name '*.proto' -delete

if find "${PROTO_SRC_DIR}" -maxdepth 1 -type f -name '*.proto' | grep -q .; then
  cp "${PROTO_SRC_DIR}"/*.proto "${CLIENT_PROTO_DIR}/"
fi

echo "client proto synced to ${CLIENT_PROTO_DIR}"
