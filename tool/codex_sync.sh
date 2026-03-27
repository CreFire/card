#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
pwsh "${SCRIPT_DIR}/codex_sync.ps1" "$@"
