#!/usr/bin/env bash
set -euo pipefail

if [[ "${KLEIN_REAL_SMOKE:-0}" != "1" ]]; then
  echo "skip: set KLEIN_REAL_SMOKE=1 to run real tmux/codex smoke"
  exit 0
fi

if ! command -v tmux >/dev/null 2>&1; then
  echo "skip: tmux is not available" >&2
  exit 0
fi

if ! command -v codex >/dev/null 2>&1; then
  echo "skip: codex is not available" >&2
  exit 0
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
HARNESS_BIN="${HARNESS_BIN:-${REPO_ROOT}/.tmp-smoke-harness}"
WORK_ROOT="$(mktemp -d)"
TARGET_REPO="${WORK_ROOT}/repo"
mkdir -p "${TARGET_REPO}"

cleanup() {
  if [[ -d "${WORK_ROOT}" ]]; then
    echo "smoke workspace: ${WORK_ROOT}"
  fi
}
trap cleanup EXIT
trap 'echo "tmux/codex smoke failed"; while IFS= read -r path; do echo "=== ${path}"; sed -n "1,120p" "${path}"; done < <(find "${TARGET_REPO}/.harness" -maxdepth 5 -type f 2>/dev/null | sort)' ERR

(cd "${REPO_ROOT}" && go build -o "${HARNESS_BIN}" ./cmd/harness)

cat > "${TARGET_REPO}/README.md" <<'EOF'
# tmux codex smoke
EOF

git -C "${TARGET_REPO}" init -q
git -C "${TARGET_REPO}" config user.name "Klein Smoke"
git -C "${TARGET_REPO}" config user.email "smoke@example.invalid"
git -C "${TARGET_REPO}" add README.md
git -C "${TARGET_REPO}" commit -q -m "init smoke repo"

"${HARNESS_BIN}" init "${TARGET_REPO}"
"${HARNESS_BIN}" submit "${TARGET_REPO}" --goal "Create a tiny verified README touch for native tmux/codex smoke"
"${HARNESS_BIN}" daemon run-once "${TARGET_REPO}" --skip-git-repo-check

test -f "${TARGET_REPO}/.harness/state/tmux-summary.json"
test -d "${TARGET_REPO}/.harness/logs/tmux/T-001"
test -f "${TARGET_REPO}/.harness/checkpoints/T-001/outcome.json"
test -f "${TARGET_REPO}/.harness/checkpoints/T-001/tmux-run.sh"
grep -q 'codex exec ' "${TARGET_REPO}/.harness/checkpoints/T-001/tmux-run.sh"
grep -q '"status": "succeeded"' "${TARGET_REPO}/.harness/checkpoints/T-001/outcome.json"
grep -q 'tmux attach-session -t kh_' "${TARGET_REPO}/.harness/state/tmux-summary.json"

echo "tmux/codex smoke passed"
