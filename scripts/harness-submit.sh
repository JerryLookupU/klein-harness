#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

if [[ -n "${HARNESS_BIN:-}" ]]; then
  exec "${HARNESS_BIN}" submit "$@"
fi

if command -v harness >/dev/null 2>&1; then
  exec harness submit "$@"
fi

exec go run "${REPO_ROOT}/cmd/harness" submit "$@"
