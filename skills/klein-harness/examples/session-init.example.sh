#!/usr/bin/env bash
# generator: klein-harness
# generatedAt: 2026-03-19T14:30:00+08:00
# project: openclaw-brain-plugin
#
# Session init script — read-only except drift-log append.
# Exit codes: 0=healthy, 1=drift found, 2=harness missing, 3=parse error

set -euo pipefail

HARNESS_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(dirname "$HARNESS_DIR")"
DRIFT_LOG_DIR="$HARNESS_DIR/drift-log"
TODAY=$(date +%Y-%m-%d)
DRIFT_LOG="$DRIFT_LOG_DIR/$TODAY.jsonl"
DRIFT_COUNT=0
EXIT_CODE=0

# ── Colors ──
RED='\033[0;31m'
YELLOW='\033[0;33m'
GREEN='\033[0;32m'
CYAN='\033[0;36m'
NC='\033[0m'

# ── Helpers ──

log_info()  { printf "${CYAN}[INFO]${NC}  %s\n" "$1"; }
log_ok()    { printf "${GREEN}[OK]${NC}    %s\n" "$1"; }
log_warn()  { printf "${YELLOW}[WARN]${NC}  %s\n" "$1"; }
log_drift() { printf "${RED}[DRIFT]${NC} %s\n" "$1"; DRIFT_COUNT=$((DRIFT_COUNT + 1)); }

# ISO timestamp (macOS + GNU compatible)
iso_now() {
  if date -u +%Y-%m-%dT%H:%M:%SZ 2>/dev/null | grep -q 'T'; then
    date -u +%Y-%m-%dT%H:%M:%SZ
  else
    date -u +%Y-%m-%dT%H:%M:%SZ
  fi
}

# Append drift event to drift-log (the ONLY write this script does)
emit_drift() {
  local kind="$1" entity_id="$2" reason="$3" ctx_file="${4:-}" ctx_field="${5:-}"
  mkdir -p "$DRIFT_LOG_DIR"
  local ts
  ts=$(iso_now)
  local entity_json
  if [[ -z "$entity_id" || "$entity_id" == "null" ]]; then
    entity_json='null'
  else
    entity_json=$(printf '"%s"' "$entity_id")
  fi
  printf '{"generator":"harness-session-init","timestamp":"%s","kind":"%s","entityId":%s,"reason":"%s","context":{"file":"%s","field":"%s"}}\n' \
    "$ts" "$kind" "$entity_json" "$reason" "$ctx_file" "$ctx_field" >> "$DRIFT_LOG"
}

# Date comparison: returns days between two YYYY-MM-DD dates
# Uses python3 if available, otherwise basic arithmetic
days_between() {
  local d1="$1" d2="$2"
  if command -v python3 &>/dev/null; then
    python3 -c "from datetime import date; print((date.fromisoformat('$d2') - date.fromisoformat('$d1')).days)"
  else
    # Fallback: macOS date -j or GNU date
    local s1 s2
    if date -j -f "%Y-%m-%d" "$d1" +%s &>/dev/null 2>&1; then
      s1=$(date -j -f "%Y-%m-%d" "$d1" +%s)
      s2=$(date -j -f "%Y-%m-%d" "$d2" +%s)
    else
      s1=$(date -d "$d1" +%s)
      s2=$(date -d "$d2" +%s)
    fi
    echo $(( (s2 - s1) / 86400 ))
  fi
}

# ── Step 1: Verify harness exists ──

log_info "Checking .harness/ integrity..."

REQUIRED_FILES=(
  "features.json"
  "work-items.json"
  "standards.md"
  "progress.md"
  "verification-rules/manifest.json"
  "spec.json"
  "task-pool.json"
  "context-map.json"
  "lineage.jsonl"
  "session-registry.json"
)
MISSING=()
for f in "${REQUIRED_FILES[@]}"; do
  if [[ ! -f "$HARNESS_DIR/$f" ]]; then
    MISSING+=("$f")
  fi
done

if [[ ! -d "$DRIFT_LOG_DIR" ]]; then
  MISSING+=("drift-log/")
fi

if [[ ${#MISSING[@]} -gt 0 ]]; then
  log_warn "Missing harness files: ${MISSING[*]}"
  emit_drift "lint-missing" "" "Missing files: ${MISSING[*]}" "" ""
  EXIT_CODE=2
fi

# ── Step 2: Git recent history ──

if [[ -d "$PROJECT_ROOT/.git" ]]; then
  log_info "Recent git activity (last 5 commits):"
  git -C "$PROJECT_ROOT" log --oneline -5 2>/dev/null || log_warn "git log failed"
  log_info "Active git worktrees:"
  git -C "$PROJECT_ROOT" worktree list 2>/dev/null || log_warn "git worktree list failed"
else
  log_warn "Not a git repository — skipping git history"
fi

# ── Step 3: Parse progress.md header ──

PROGRESS_FILE="$HARNESS_DIR/progress.md"
if [[ -f "$PROGRESS_FILE" ]]; then
  log_info "Parsing progress.md..."
  # Extract JSON block between first ``` pair
  if command -v jq &>/dev/null; then
    PROGRESS_JSON=$(sed -n '/^```json$/,/^```$/p' "$PROGRESS_FILE" | sed '1d;$d')
    CURRENT_FOCUS=$(echo "$PROGRESS_JSON" | jq -r '.currentFocus // "null"')
    CURRENT_ROLE=$(echo "$PROGRESS_JSON" | jq -r '.currentRole // "unknown"')
    AUDIT_STATUS=$(echo "$PROGRESS_JSON" | jq -r '.lastAuditStatus // "unknown"')
    log_ok "Current focus: $CURRENT_FOCUS | Current role: $CURRENT_ROLE | Last audit: $AUDIT_STATUS"
  else
    # Fallback: grep
    CURRENT_FOCUS=$(grep -o '"currentFocus"[[:space:]]*:[[:space:]]*"[^"]*"' "$PROGRESS_FILE" | head -1 | sed 's/.*: *"//;s/"//')
    CURRENT_ROLE=$(grep -o '"currentRole"[[:space:]]*:[[:space:]]*"[^"]*"' "$PROGRESS_FILE" | head -1 | sed 's/.*: *"//;s/"//')
    AUDIT_STATUS=$(grep -o '"lastAuditStatus"[[:space:]]*:[[:space:]]*"[^"]*"' "$PROGRESS_FILE" | head -1 | sed 's/.*: *"//;s/"//')
    log_ok "Current focus: ${CURRENT_FOCUS:-unknown} | Current role: ${CURRENT_ROLE:-unknown} | Last audit: ${AUDIT_STATUS:-unknown}"
  fi
else
  log_warn "progress.md not found"
fi

# ── Step 4: Inspect work-items / task claims ──

TASK_POOL="$HARNESS_DIR/task-pool.json"
if [[ -f "$TASK_POOL" ]] && command -v jq &>/dev/null; then
  log_info "Inspecting claimable task pool..."
  CLAIMABLE_COUNT=$(jq '
    .tasks as $tasks
    | [
        $tasks[]
        | select(.roleHint == "worker")
        | select(.planningStage == "execution-ready")
        | select(.status == "queued")
        | select(.claim.agentId == null)
        | select(
            ((.dependsOn // []) | all(
              . as $dep
              | any($tasks[]; .taskId == $dep and (.status == "completed" or .status == "validated" or .status == "done" or .status == "pass"))
            ))
          )
      ]
    | length
  ' "$TASK_POOL")
  ACTIVE_ORCH_COUNT=$(jq '[.tasks[] | select(.roleHint == "orchestrator" and (.status == "queued" or .status == "active" or .status == "claimed" or .status == "in_progress"))] | length' "$TASK_POOL")
  log_ok "Claimable tasks: $CLAIMABLE_COUNT | Active orchestration tasks: $ACTIVE_ORCH_COUNT"
  if [[ "$ACTIVE_ORCH_COUNT" -gt 0 ]]; then
    log_warn "Control-plane orchestration work exists. New agents should consider orchestrator first."
  fi
else
  log_info "Skipping task-pool inspection (no jq or no task-pool.json)"
fi

# ── Step 4.5: Runner consistency checks ──

RUNNER_STATE="$HARNESS_DIR/state/runner-state.json"
RUNNER_HEARTBEATS="$HARNESS_DIR/state/runner-heartbeats.json"
if [[ -f "$TASK_POOL" ]] && [[ -f "$RUNNER_HEARTBEATS" ]] && command -v jq &>/dev/null; then
  log_info "Checking runner/tmux consistency..."

  # active task without tmux session
  jq -r --slurpfile hb "$RUNNER_HEARTBEATS" '
    ($hb[0].entries // {}) as $entries
    | .tasks[]
    | select(.status == "active" or .status == "claimed" or .status == "in_progress")
    | [.taskId, (.claim.tmuxSession // ($entries[.taskId].tmuxSession // ""))]
    | @tsv
  ' "$TASK_POOL" | while IFS=$'\t' read -r tid tmux_name; do
    if [[ -z "$tmux_name" ]]; then
      log_drift "Active task without tmux session or heartbeat: $tid"
      emit_drift "runner-missing-tmux" "$tid" "active task has no claim.tmuxSession or runner heartbeat session" "task-pool.json" "claim.tmuxSession"
    elif ! tmux has-session -t "$tmux_name" 2>/dev/null; then
      log_drift "Active task with stale tmux session: $tid -> $tmux_name"
      emit_drift "runner-stale-tmux" "$tid" "tmux session missing for active task" "task-pool.json" "claim.tmuxSession"
    fi
  done

  # duplicate session bindings
  jq -r --slurpfile hb "$RUNNER_HEARTBEATS" '
    ($hb[0].entries // {}) as $entries
    | .tasks[]
    | [ .taskId, (.claim.tmuxSession // ($entries[.taskId].tmuxSession // "")) ]
    | select(.[1] != "")
    | @tsv
  ' "$TASK_POOL" | awk -F'\t' '{count[$2]++; tasks[$2]=tasks[$2] "," $1} END {for (s in count) if (count[s] > 1) print s "\t" tasks[s]}' | while IFS=$'\t' read -r tmux_name task_ids; do
    [[ -z "$tmux_name" ]] && continue
    log_drift "Duplicate tmux session binding: $tmux_name -> ${task_ids#,}"
    emit_drift "runner-duplicate-binding" "$tmux_name" "multiple tasks bound to same tmux session" "task-pool.json" "claim.tmuxSession"
  done
fi

# ── Step 5: Run fast read-only-safe verification rules ──

MANIFEST="$HARNESS_DIR/verification-rules/manifest.json"
if [[ -f "$MANIFEST" ]] && command -v jq &>/dev/null; then
  log_info "Running fast read-only-safe verification rules..."
  FAST_RULES=$(jq -r '.rules[] | select(.costTier == "fast" and .readOnlySafe == true) | .id + "|" + .exec' "$MANIFEST")
  while IFS='|' read -r rule_id rule_exec; do
    [[ -z "$rule_id" ]] && continue
    if (cd "$PROJECT_ROOT" && eval "$rule_exec" &>/dev/null); then
      log_ok "$rule_id passed"
    else
      log_drift "$rule_id FAILED"
      emit_drift "verification-fail" "$rule_id" "Rule execution failed" "manifest.json" "$rule_id"
    fi
  done <<< "$FAST_RULES"
else
  log_info "Skipping verification rules (no jq or no manifest.json)"
fi

# ── Step 6: Scan @harness-lint tags for overdue reviews ──

log_info "Scanning @harness-lint tags for drift..."

scan_lint_tags() {
  local file="$1"
  grep -n '@harness-lint:' "$file" 2>/dev/null | while IFS= read -r line; do
    local next_review
    next_review=$(echo "$line" | grep -o 'nextReview=[^ ]*' | sed 's/nextReview=//' | sed 's/ *-->.*//')
    [[ -z "$next_review" ]] && continue

    # Extract just the date part (YYYY-MM-DD)
    local review_date="${next_review:0:10}"
    local overdue
    overdue=$(days_between "$review_date" "$TODAY" 2>/dev/null || echo "0")

    if [[ "$overdue" -gt 0 ]]; then
      local entity_id
      entity_id=$(echo "$line" | grep -o 'id=[^ ]*' | sed 's/id=//')
      log_drift "Overdue by ${overdue}d: ${entity_id:-unknown} in $(basename "$file")"
      emit_drift "review-overdue" "${entity_id:-null}" "nextReview exceeded by ${overdue}d" "$(basename "$file")" "nextReview"
    fi
  done
}

for md_file in "$HARNESS_DIR"/*.md; do
  [[ -f "$md_file" ]] && scan_lint_tags "$md_file"
done

# Also scan features.json lint fields
if [[ -f "$HARNESS_DIR/features.json" ]] && command -v jq &>/dev/null; then
  jq -r '.features[] | select(.lint.nextReview != null) | .id + "|" + .lint.nextReview' "$HARNESS_DIR/features.json" 2>/dev/null | while IFS='|' read -r fid next_review; do
    [[ -z "$fid" ]] && continue
    review_date="${next_review:0:10}"
    overdue=$(days_between "$review_date" "$TODAY" 2>/dev/null || echo "0")
    if [[ "$overdue" -gt 0 ]]; then
      log_drift "Overdue by ${overdue}d: $fid in features.json"
      emit_drift "review-overdue" "$fid" "nextReview exceeded by ${overdue}d" "features.json" "lint.nextReview"
    fi
  done
fi

# ── Step 7: Summary ──

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
if [[ $DRIFT_COUNT -eq 0 && $EXIT_CODE -eq 0 ]]; then
  printf "${GREEN}✓ Harness healthy. No drift detected.${NC}\n"
else
  printf "${RED}⚠ Drift count: %d${NC}\n" "$DRIFT_COUNT"
  [[ $EXIT_CODE -eq 0 ]] && EXIT_CODE=1
fi
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

if [[ -n "${CURRENT_FOCUS:-}" && "$CURRENT_FOCUS" != "null" ]]; then
  log_info "Suggested next step: work on $CURRENT_FOCUS"
fi

exit $EXIT_CODE
