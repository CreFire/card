#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

EXCEL_DIR="${REPO_ROOT}/excel"
OUT_DIR="${REPO_ROOT}/server/src/proto/docpb"
JSON_OUT_DIR="${REPO_ROOT}/server/configdoc/json"
LUBAN_DLL="${REPO_ROOT}/tool/bin/net8.0/Luban.dll"
STAGE_ROOT="${REPO_ROOT}/tool/.server_excel_stage"
STAGE_PROTO_DIR="${STAGE_ROOT}/proto"
STAGE_JSON_DIR="${STAGE_ROOT}/json"
STAGE_CONF_FILE="${STAGE_ROOT}/luban.conf"

if [ ! -d "${EXCEL_DIR}" ]; then
  echo "excel source directory not found: ${EXCEL_DIR}" >&2
  exit 1
fi

if [ ! -f "${LUBAN_DLL}" ]; then
  echo "Luban dll not found: ${LUBAN_DLL}" >&2
  exit 1
fi

mkdir -p "${OUT_DIR}"
mkdir -p "${JSON_OUT_DIR}"
rm -rf "${STAGE_ROOT}"
mkdir -p "${STAGE_PROTO_DIR}" "${STAGE_JSON_DIR}"

if [ ! -f "${EXCEL_DIR}/enum.xlsx" ]; then
  echo "required file not found: ${EXCEL_DIR}/enum.xlsx" >&2
  exit 1
fi

cp "${EXCEL_DIR}/enum.xlsx" "${STAGE_ROOT}/enum.xlsx"

while IFS= read -r -d '' file; do
  name="$(basename "${file}")"
  case "${name}" in
    enum.xlsx|__tables__.xlsx|__beans__.xlsx|__enums__.xlsx)
      ;;
    *)
      if [[ "${name}" == \#* ]]; then
        cp "${file}" "${STAGE_ROOT}/${name}"
      else
        cp "${file}" "${STAGE_ROOT}/#${name}"
      fi
      ;;
  esac
done < <(find "${EXCEL_DIR}" -maxdepth 1 -type f -name '*.xlsx' -print0)

cat > "${STAGE_CONF_FILE}" <<'EOF'
{
  "groups": [
    { "names": ["s"], "default": true }
  ],
  "schemaFiles": [
    { "fileName": "enum.xlsx", "type": "enum" }
  ],
  "dataDir": ".",
  "targets": [
    { "name": "server", "manager": "Tables", "groups": ["s"], "topModule": "cfg" }
  ],
  "xargs": []
}
EOF

find "${OUT_DIR}" -maxdepth 1 -type f \( -name '*.proto' -o -name '*.go' -o -name '*.cs' -o -name '*.json' \) -delete
find "${JSON_OUT_DIR}" -maxdepth 1 -type f -name '*.json' -delete

echo "server luban generate"
echo "  input : ${EXCEL_DIR}"
echo "  output: ${OUT_DIR}"
echo "  json  : ${JSON_OUT_DIR}"
echo "  stage : ${STAGE_ROOT}"

DOTNET_ROLL_FORWARD=Major \
dotnet "${LUBAN_DLL}" \
  -t server \
  -c protobuf3 \
  -d protobuf3-json \
  --conf "${STAGE_CONF_FILE}" \
  -x outputCodeDir="${STAGE_PROTO_DIR}" \
  -x outputDataDir="${STAGE_JSON_DIR}"

if [ ! -f "${STAGE_PROTO_DIR}/schema.proto" ]; then
  echo "Luban did not generate schema.proto" >&2
  exit 1
fi

DOTNET_ROLL_FORWARD=Major \
dotnet "${LUBAN_DLL}" \
  -t server \
  -c go-json \
  -d json \
  --conf "${STAGE_CONF_FILE}" \
  -x lubanGoModule=backend \
  -x outputCodeDir="${OUT_DIR}" \
  -x outputDataDir="${JSON_OUT_DIR}"

cp "${STAGE_PROTO_DIR}/schema.proto" "${OUT_DIR}/schema.proto"

echo "server excel proto generated to ${OUT_DIR}"
