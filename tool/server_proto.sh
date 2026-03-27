#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

PROTO_SRC_DIR="${REPO_ROOT}/proto"
OUT_DIR="${REPO_ROOT}/server/src/proto/pb"
PROTOC_EXE="${REPO_ROOT}/tool/bin/protoc.exe"
PROTOC_GEN_GO="${REPO_ROOT}/server/tools/.bin/protoc-gen-go.exe"
PROTO_SRC_DIR_WIN="${PROTO_SRC_DIR}"
OUT_DIR_WIN="${OUT_DIR}"

if [ ! -d "${PROTO_SRC_DIR}" ]; then
  echo "proto source directory not found: ${PROTO_SRC_DIR}" >&2
  exit 1
fi

if [ ! -f "${PROTOC_EXE}" ]; then
  echo "protoc not found: ${PROTOC_EXE}" >&2
  exit 1
fi

if [ ! -f "${PROTOC_GEN_GO}" ]; then
  echo "protoc-gen-go not found: ${PROTOC_GEN_GO}" >&2
  exit 1
fi

if command -v cygpath >/dev/null 2>&1; then
  PROTO_SRC_DIR_WIN="$(cygpath -w "${PROTO_SRC_DIR}")"
  OUT_DIR_WIN="$(cygpath -w "${OUT_DIR}")"
fi

mkdir -p "${OUT_DIR}"
find "${OUT_DIR}" -maxdepth 1 -type f -name '*.pb.go' -delete

mapfile -t proto_files < <(find "${PROTO_SRC_DIR}" -maxdepth 1 -type f -name '*.proto' | sort)

if [ "${#proto_files[@]}" -eq 0 ]; then
  echo "no proto files found under ${PROTO_SRC_DIR}, nothing to generate"
  exit 0
fi

export PATH="${REPO_ROOT}/server/tools/.bin:${PATH}"

for file in "${proto_files[@]}"; do
  name="$(basename "${file}")"
  echo "generate ${name}"
  "${PROTOC_EXE}" \
    "--proto_path=${PROTO_SRC_DIR_WIN}" \
    "--go_out=paths=source_relative:${OUT_DIR_WIN}" \
    "${name}"
done

echo "server proto generated to ${OUT_DIR}"
