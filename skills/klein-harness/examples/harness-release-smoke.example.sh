#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"

TMP_ROOT="$(mktemp -d)"
CODEX_HOME_DIR="$TMP_ROOT/codex"
PROJECT_ROOT="$TMP_ROOT/release-smoke-project"
AUTO_INIT_ROOT="$TMP_ROOT/auto-init-project"

cleanup() {
  rm -rf "$TMP_ROOT"
}
trap cleanup EXIT

export CODEX_HOME="$CODEX_HOME_DIR"
export PATH="$CODEX_HOME_DIR/bin:$PATH"

"$REPO_ROOT/install.sh" --dest "$CODEX_HOME_DIR/skills" --bin-dir "$CODEX_HOME_DIR/bin" --no-shell-rc --force >/dev/null
AUTO_INIT_JSON="$TMP_ROOT/auto-init-submit.json"
harness-submit "$AUTO_INIT_ROOT" --goal "Auto-init smoke request" --source smoke > "$AUTO_INIT_JSON"
harness-init "$PROJECT_ROOT" >/dev/null

mkdir -p "$PROJECT_ROOT"
git -C "$PROJECT_ROOT" init -b main >/dev/null
git -C "$PROJECT_ROOT" config user.name "Klein Smoke"
git -C "$PROJECT_ROOT" config user.email "smoke@example.com"
cat > "$PROJECT_ROOT/README.md" <<'EOF'
# Release Smoke Project
EOF
cat > "$PROJECT_ROOT/shared-conflict.txt" <<'EOF'
base
EOF
git -C "$PROJECT_ROOT" add README.md shared-conflict.txt
git -C "$PROJECT_ROOT" commit -m "initial smoke baseline" >/dev/null
git -C "$PROJECT_ROOT" branch orch/spec-S-100 >/dev/null

cat > "$PROJECT_ROOT/.harness/state/smoke-extra.log" <<'EOF'
smoke runtime extra evidence
EOF

cat > "$PROJECT_ROOT/.harness/state/progress.json" <<'EOF'
{
  "schemaVersion": "1.0",
  "generator": "smoke-test",
  "generatedAt": "2026-03-22T00:00:00+08:00",
  "mode": "agent-entry",
  "planningStage": "execution-ready",
  "currentFocus": "WI-100",
  "currentRole": "worker",
  "currentTaskId": "T-100",
  "currentTaskTitle": "Apply smoke runtime patch",
  "currentTaskSummary": "Use a minimal task to prove request closure and recover/resume.",
  "blockers": [],
  "nextActions": [
    "Bind submitted request to T-100",
    "Route and preview dispatch",
    "Verify and report closed loop"
  ],
  "lastAuditStatus": "pass",
  "claimSummary": {},
  "legacyFallbackUsed": false
}
EOF

cat > "$PROJECT_ROOT/.harness/spec.json" <<'EOF'
{
  "schemaVersion": "1.0",
  "generator": "smoke-test",
  "generatedAt": "2026-03-22T00:00:00+08:00",
  "specRevision": "S-100",
  "planningStage": "execution-ready",
  "objective": "Close the runtime request loop in smoke coverage",
  "blocks": [
    {
      "id": "TB-100",
      "title": "Smoke runtime block",
      "status": "active",
      "featureIds": ["F-100"]
    }
  ]
}
EOF

cat > "$PROJECT_ROOT/.harness/features.json" <<'EOF'
{
  "schemaVersion": "1.0",
  "generator": "smoke-test",
  "generatedAt": "2026-03-22T00:00:00+08:00",
  "features": [
    {
      "id": "F-100",
      "title": "Smoke runtime closure",
      "verificationStatus": "pass",
      "priority": "P0"
    }
  ]
}
EOF

cat > "$PROJECT_ROOT/.harness/work-items.json" <<'EOF'
{
  "schemaVersion": "1.0",
  "generator": "smoke-test",
  "generatedAt": "2026-03-22T00:00:00+08:00",
  "items": [
    {
      "id": "WI-100",
      "kind": "feature",
      "title": "Apply smoke runtime patch",
      "summary": "Minimal work item for request binding and recovery.",
      "status": "queued",
      "priority": "P0",
      "roleHint": "worker",
      "featureIds": ["F-100"],
      "dependsOn": []
    },
    {
      "id": "WI-101",
      "kind": "bugfix",
      "title": "Repair smoke RCA follow-up",
      "summary": "Minimal repair work item for RCA emission and prevention write-back.",
      "status": "queued",
      "priority": "P0",
      "roleHint": "worker",
      "featureIds": ["F-100"],
      "dependsOn": []
    }
  ]
}
EOF

cat > "$PROJECT_ROOT/.harness/task-pool.json" <<'EOF'
{
  "schemaVersion": "1.0",
  "generator": "smoke-test",
  "generatedAt": "2026-03-22T00:00:00+08:00",
  "integrationBranch": "orch/spec-S-100",
  "tasks": [
    {
      "taskId": "T-100",
      "workItemId": "WI-100",
      "blockId": "TB-100",
      "kind": "feature",
      "roleHint": "worker",
      "title": "Apply smoke runtime patch",
      "summary": "Minimal worker task used by release smoke.",
      "description": "Dispatch in print mode, force a recoverable exit, then resume and verify.",
      "status": "queued",
      "priority": "P0",
      "dependsOn": [],
      "planningStage": "execution-ready",
      "lineagePath": ["F-100", "WI-100", "T-100"],
      "baseRef": "refs/heads/orch/spec-S-100",
      "branchName": "task/T-100-smoke",
      "worktreePath": ".worktrees/T-100-smoke",
      "diffBase": "refs/heads/orch/spec-S-100",
      "diffSummary": "smoke preview diff",
      "ownedPaths": ["smoke-pass.txt"],
      "verificationRuleIds": ["VR-100"],
      "routingModel": "gpt-5.4",
      "executionModel": "gpt-5.3-codex",
      "resumeStrategy": "resume",
      "preferredResumeSessionId": "sess-worker-100",
      "candidateResumeSessionIds": ["sess-worker-100"],
      "lastKnownSessionId": "sess-worker-100",
      "sessionFamilyId": "SF-F100-WI100",
      "cacheAffinityKey": "feature:F-100|parent:WI-100|role:worker",
      "routingReason": "Smoke task reuses a known-safe worker session to exercise resume flow.",
      "dispatch": {
        "runner": "codex exec",
        "targetKind": "worker-node",
        "targetSelector": "tmux:worker-smoke",
        "entryRole": "worker",
        "taskContextId": "CTX-T-100",
        "worktreePath": ".worktrees/T-100-smoke",
        "branchName": "task/T-100-smoke",
        "baseRef": "refs/heads/orch/spec-S-100",
        "diffBase": "refs/heads/orch/spec-S-100",
        "commandProfile": {
          "standard": "codex exec resume <SESSION_ID> --yolo -m gpt-5.3-codex",
          "localCompat": "codex exec resume <SESSION_ID> --yolo -m gpt-5.3-codex"
        },
        "logPath": ".harness/runtime/worker-smoke.log",
        "heartbeatPath": ".harness/runtime/worker-smoke.heartbeat",
        "maxParallelism": 1,
        "cooldownSeconds": 5
      },
      "handoff": {
        "nextSuggestedWorkItemIds": [],
        "nextSuggestedTaskIds": [],
        "replanOnFail": true,
        "mergeRequired": true,
        "returnToRole": "orchestrator"
      },
      "claim": {
        "agentId": null,
        "role": null,
        "nodeId": null,
        "boundSessionId": null,
        "boundResumeStrategy": null,
        "boundFromTaskId": null,
        "boundAt": null,
        "leasedAt": null,
        "leaseExpiresAt": null
      }
    },
    {
      "taskId": "T-101",
      "workItemId": "WI-101",
      "blockId": "TB-100",
      "kind": "bugfix",
      "roleHint": "worker",
      "title": "Repair smoke RCA follow-up",
      "summary": "Repair request emitted from RCA allocation.",
      "description": "Exercise bug intake -> RCA allocation -> repair request -> verify.",
      "status": "queued",
      "priority": "P0",
      "dependsOn": [],
      "planningStage": "draft",
      "lineagePath": ["F-100", "WI-101", "T-101"],
      "baseRef": "refs/heads/orch/spec-S-100",
      "branchName": "task/T-101-rca-repair",
      "worktreePath": ".worktrees/T-101-rca-repair",
      "diffBase": "refs/heads/orch/spec-S-100",
      "diffSummary": "smoke rca repair diff",
      "ownedPaths": ["smoke-rca-pass.txt"],
      "verificationRuleIds": ["VR-101"],
      "routingModel": "gpt-5.4",
      "executionModel": "gpt-5.3-codex",
      "resumeStrategy": "fresh",
      "preferredResumeSessionId": null,
      "candidateResumeSessionIds": [],
      "lastKnownSessionId": null,
      "sessionFamilyId": "SF-F100-WI101",
      "cacheAffinityKey": "feature:F-100|parent:WI-101|role:worker",
      "routingReason": "Queued repair lane for smoke RCA follow-up.",
      "dispatch": {
        "runner": "codex exec",
        "targetKind": "worker-node",
        "targetSelector": "tmux:worker-smoke",
        "entryRole": "worker",
        "taskContextId": "CTX-T-101",
        "worktreePath": ".worktrees/T-101-rca-repair",
        "branchName": "task/T-101-rca-repair",
        "baseRef": "refs/heads/orch/spec-S-100",
        "diffBase": "refs/heads/orch/spec-S-100",
        "commandProfile": {
          "standard": "codex exec --yolo -m gpt-5.3-codex",
          "localCompat": "codex exec --yolo -m gpt-5.3-codex"
        },
        "logPath": ".harness/runtime/worker-smoke-rca.log",
        "heartbeatPath": ".harness/runtime/worker-smoke-rca.heartbeat",
        "maxParallelism": 1,
        "cooldownSeconds": 5
      },
      "handoff": {
        "nextSuggestedWorkItemIds": [],
        "nextSuggestedTaskIds": [],
        "replanOnFail": true,
        "mergeRequired": false,
        "returnToRole": "orchestrator"
      },
      "claim": {
        "agentId": null,
        "role": null,
        "nodeId": null,
        "boundSessionId": null,
        "boundResumeStrategy": null,
        "boundFromTaskId": null,
        "boundAt": null,
        "leasedAt": null,
        "leaseExpiresAt": null
      }
    }
  ]
}
EOF

cat > "$PROJECT_ROOT/.harness/session-registry.json" <<'EOF'
{
  "schemaVersion": "1.0",
  "generator": "smoke-test",
  "generatedAt": "2026-03-22T00:00:00+08:00",
  "orchestrationSessionId": "orch-session-100",
  "orchestrationSessions": [
    {
      "sessionId": "orch-session-100",
      "model": "gpt-5.4",
      "role": "orchestrator",
      "status": "active",
      "purpose": "smoke routing orchestration",
      "lastUsedAt": "2026-03-22T00:00:00+08:00"
    }
  ],
  "sessions": [
    {
      "sessionId": "sess-worker-100",
      "rootSessionId": "sess-worker-100",
      "parentSessionId": null,
      "branchRootSessionId": "sess-worker-100",
      "branchOfSessionId": null,
      "sessionFamilyId": "SF-F100-WI100",
      "sourceTaskId": "T-100",
      "model": "gpt-5.3-codex",
      "status": "recoverable",
      "lastUsedAt": "2026-03-22T00:00:00+08:00"
    }
  ],
  "families": [
    {
      "sessionFamilyId": "SF-F100-WI100",
      "featureId": "F-100",
      "anchorWorkItemId": "WI-100",
      "cacheAffinityKey": "feature:F-100|parent:WI-100|role:worker"
    }
  ],
  "routingDecisions": [],
  "activeBindings": [],
  "recoverableBindings": [],
  "lastCompletedByTask": {}
}
EOF

cat > "$PROJECT_ROOT/.harness/verification-rules/manifest.json" <<'EOF'
{
  "schemaVersion": "1.0",
  "generator": "smoke-test",
  "generatedAt": "2026-03-22T00:00:00+08:00",
  "rules": [
    {
      "id": "VR-100",
      "title": "Smoke verification rule",
      "type": "shell",
      "costTier": "cheap",
      "readOnlySafe": true,
      "exec": "test -f smoke-pass.txt"
    },
    {
      "id": "VR-101",
      "title": "Smoke RCA repair verification rule",
      "type": "shell",
      "costTier": "cheap",
      "readOnlySafe": true,
      "exec": "test -f smoke-rca-pass.txt"
    }
  ]
}
EOF

SUBMIT_JSON="$TMP_ROOT/submit.json"
DUPLICATE_SUBMIT_JSON="$TMP_ROOT/duplicate-submit.json"
CONTEXT_SUBMIT_JSON="$TMP_ROOT/context-submit.json"
INSPECTION_SUBMIT_JSON="$TMP_ROOT/inspection-submit.json"
APPEND_SUBMIT_JSON="$TMP_ROOT/append-submit.json"
APPEND2_SUBMIT_JSON="$TMP_ROOT/append2-submit.json"
COMPOUND_SUBMIT_JSON="$TMP_ROOT/compound-submit.json"
BUG_SUBMIT_JSON="$TMP_ROOT/bug-submit.json"
RECONCILE_JSON="$TMP_ROOT/reconcile.json"
INSPECTION_RECONCILE_JSON="$TMP_ROOT/inspection-reconcile.json"
APPEND_RECONCILE_JSON="$TMP_ROOT/append-reconcile.json"
APPEND2_RECONCILE_JSON="$TMP_ROOT/append2-reconcile.json"
COMPOUND_RECONCILE_JSON="$TMP_ROOT/compound-reconcile.json"
BUG_RECONCILE_JSON="$TMP_ROOT/bug-reconcile.json"
REPAIR_RECONCILE_JSON="$TMP_ROOT/repair-reconcile.json"
RUN_JSON="$TMP_ROOT/run.json"
RECOVER_JSON="$TMP_ROOT/recover.json"
FINALIZE_JSON="$TMP_ROOT/finalize.json"
REPORT_JSON="$TMP_ROOT/report.json"
RCA_REPORT_JSON="$TMP_ROOT/rca-report.json"
REPAIR_RUN_JSON="$TMP_ROOT/repair-run.json"
REPAIR_FINALIZE_JSON="$TMP_ROOT/repair-finalize.json"
LOG_SEARCH_JSON="$TMP_ROOT/log-search.json"
LOG_SEARCH_DETAIL_JSON="$TMP_ROOT/log-search-detail.json"
OPS_TOP_JSON="$TMP_ROOT/ops-top.json"
OPS_QUEUE_JSON="$TMP_ROOT/ops-queue.json"
OPS_WORKERS_JSON="$TMP_ROOT/ops-workers.json"
OPS_TASK_JSON="$TMP_ROOT/ops-task.json"
OPS_DAEMON_JSON="$TMP_ROOT/ops-daemon.json"
OPS_WORKTREES_JSON="$TMP_ROOT/ops-worktrees.json"
OPS_MERGE_QUEUE_JSON="$TMP_ROOT/ops-merge-queue.json"
OPS_CONFLICTS_JSON="$TMP_ROOT/ops-conflicts.json"
OPS_DOCTOR_JSON="$TMP_ROOT/ops-doctor.json"
OPS_WATCH_TEXT="$TMP_ROOT/ops-watch.txt"
TASKS_JSON="$TMP_ROOT/tasks.json"
TASK_DETAIL_JSON="$TMP_ROOT/task-detail.json"
TASK_LOG_JSON="$TMP_ROOT/task-log.json"
CONTROL_DAEMON_JSON="$TMP_ROOT/control-daemon.json"
CONTROL_ARCHIVE_JSON="$TMP_ROOT/control-archive.json"
UNKNOWN_DIRTY_GUARD_JSON="$TMP_ROOT/unknown-dirty-guard.json"
MANAGED_DIRTY_GUARD_JSON="$TMP_ROOT/managed-dirty-guard.json"
MANAGED_DIRTY_WORKTREE_JSON="$TMP_ROOT/managed-dirty-worktree.json"
CONTROL_PROJECT_ARCHIVE_JSON="$TMP_ROOT/control-project-archive.json"
FINAL_COMPLETION_GATE_JSON="$TMP_ROOT/final-completion-gate.json"

harness-submit "$PROJECT_ROOT" --kind implementation --goal "Apply smoke runtime patch" --source smoke > "$SUBMIT_JSON"
REQUEST_ID="$(python3 - <<'PY' "$SUBMIT_JSON"
import json
import sys
print(json.load(open(sys.argv[1]))["requestId"])
PY
)"

python3 "$PROJECT_ROOT/.harness/scripts/request.py" reconcile --root "$PROJECT_ROOT" > "$RECONCILE_JSON"
python3 "$PROJECT_ROOT/.harness/scripts/route-session.py" --root "$PROJECT_ROOT" --task-id T-100 --write-back >/dev/null
"$PROJECT_ROOT/.harness/bin/harness-runner" run T-100 "$PROJECT_ROOT" --dispatch-mode print > "$RUN_JSON"
python3 "$PROJECT_ROOT/.harness/scripts/runner.py" heartbeat "$PROJECT_ROOT" T-100 "print:T-100" --phase running >/dev/null
python3 "$PROJECT_ROOT/.harness/scripts/runner.py" heartbeat "$PROJECT_ROOT" T-100 "print:T-100" --phase exited --exit-code 7 >/dev/null
"$PROJECT_ROOT/.harness/bin/harness-runner" recover T-100 "$PROJECT_ROOT" --dispatch-mode print > "$RECOVER_JSON"
touch "$PROJECT_ROOT/.worktrees/T-100-smoke/smoke-pass.txt"
git -C "$PROJECT_ROOT/.worktrees/T-100-smoke" add smoke-pass.txt
git -C "$PROJECT_ROOT/.worktrees/T-100-smoke" commit -m "T-100 smoke patch" >/dev/null
"$PROJECT_ROOT/.harness/bin/harness-verify-task" T-100 "$PROJECT_ROOT" --write-back >/dev/null
python3 "$PROJECT_ROOT/.harness/scripts/runner.py" finalize "$PROJECT_ROOT" T-100 --tmux-session "print:T-100" --runner-status 0 > "$FINALIZE_JSON"
python3 "$PROJECT_ROOT/.harness/scripts/refresh-state.py" "$PROJECT_ROOT" >/dev/null
harness-report "$PROJECT_ROOT" --request-id "$REQUEST_ID" --format json > "$REPORT_JSON"

harness-submit "$PROJECT_ROOT" --goal "Apply smoke runtime patch" --source smoke > "$DUPLICATE_SUBMIT_JSON"
harness-submit "$PROJECT_ROOT" --goal "提供 R-0001 的 smoke 额外日志证据" --context "$PROJECT_ROOT/.harness/state/smoke-extra.log" --source smoke > "$CONTEXT_SUBMIT_JSON"
harness-submit "$PROJECT_ROOT" --goal "检查 R-0001 当前 verify 状态和 compact log" --source smoke > "$INSPECTION_SUBMIT_JSON"
python3 "$PROJECT_ROOT/.harness/scripts/request.py" reconcile --root "$PROJECT_ROOT" > "$INSPECTION_RECONCILE_JSON"
harness-submit "$PROJECT_ROOT" --goal "修改 R-0001 的 smoke 验收并补充 note" --source smoke > "$APPEND_SUBMIT_JSON"
python3 "$PROJECT_ROOT/.harness/scripts/request.py" reconcile --root "$PROJECT_ROOT" > "$APPEND_RECONCILE_JSON"
harness-submit "$PROJECT_ROOT" --goal "再修改 R-0001 的 smoke 验收边界" --source smoke > "$APPEND2_SUBMIT_JSON"
python3 "$PROJECT_ROOT/.harness/scripts/request.py" reconcile --root "$PROJECT_ROOT" > "$APPEND2_RECONCILE_JSON"
harness-submit "$PROJECT_ROOT" --goal "检查 R-0001 当前实现，并且新增 smoke 兼容说明" --source smoke > "$COMPOUND_SUBMIT_JSON"
python3 "$PROJECT_ROOT/.harness/scripts/request.py" reconcile --root "$PROJECT_ROOT" > "$COMPOUND_RECONCILE_JSON"

cat >> "$PROJECT_ROOT/.harness/feedback-log.jsonl" <<'EOF'
{"id":"FB-100","taskId":"T-100","sessionId":"sess-worker-100","role":"worker","workerMode":"execution","feedbackType":"verification_failure","severity":"error","source":"verification","step":"verify","triggeringAction":"post-release smoke bug intake","message":"Smoke task T-100 regressed after verification and now requires RCA allocation.","timestamp":"2026-03-22T00:05:00+08:00"}
EOF

python3 - <<'PY' "$PROJECT_ROOT/.harness/task-pool.json"
import json
import sys
path = sys.argv[1]
data = json.load(open(path))
for task in data.get("tasks", []):
    if task.get("taskId") == "T-101":
        task["planningStage"] = "execution-ready"
json.dump(data, open(path, "w"), ensure_ascii=False, indent=2)
PY

harness-submit "$PROJECT_ROOT" --kind bug --goal "Bug on T-100 after verification failure in smoke runtime patch" --source smoke > "$BUG_SUBMIT_JSON"
BUG_REQUEST_ID="$(python3 - <<'PY' "$BUG_SUBMIT_JSON"
import json
import sys
print(json.load(open(sys.argv[1]))["requestId"])
PY
)"

python3 "$PROJECT_ROOT/.harness/scripts/request.py" reconcile --root "$PROJECT_ROOT" > "$BUG_RECONCILE_JSON"
python3 "$PROJECT_ROOT/.harness/scripts/request.py" reconcile --root "$PROJECT_ROOT" > "$REPAIR_RECONCILE_JSON"
python3 "$PROJECT_ROOT/.harness/scripts/route-session.py" --root "$PROJECT_ROOT" --task-id T-101 --write-back >/dev/null
"$PROJECT_ROOT/.harness/bin/harness-runner" run T-101 "$PROJECT_ROOT" --dispatch-mode print > "$REPAIR_RUN_JSON"
touch "$PROJECT_ROOT/.worktrees/T-101-rca-repair/smoke-rca-pass.txt"
git -C "$PROJECT_ROOT/.worktrees/T-101-rca-repair" add smoke-rca-pass.txt
git -C "$PROJECT_ROOT/.worktrees/T-101-rca-repair" commit -m "T-101 smoke repair" >/dev/null
"$PROJECT_ROOT/.harness/bin/harness-verify-task" T-101 "$PROJECT_ROOT" --write-back >/dev/null
python3 "$PROJECT_ROOT/.harness/scripts/runner.py" finalize "$PROJECT_ROOT" T-101 --tmux-session "print:T-101" --runner-status 0 > "$REPAIR_FINALIZE_JSON"

cat > "$PROJECT_ROOT/.harness/research/smoke-runtime-scan.md" <<'EOF'
---
schemaVersion: "1.0"
generator: "smoke-test"
generatedAt: "2026-03-22T00:06:00+08:00"
slug: "smoke-runtime-scan"
researchMode: "targeted"
question: "Does the smoke runtime need extra targeted operator evidence before a blueprint draft?"
sources:
  - "repo:.harness/task-pool.json"
  - "repo:.harness/log-T-100.md"
---

## Summary

- Compact logs expose enough context for downstream workers.
- Raw runner logs remain necessary only for targeted evidence windows.

## Findings

- The finalize path emits a shareable handoff surface.

## Recommendation

- Use compact log summaries as the default blueprint input and fall back to raw evidence only on demand.
EOF

python3 "$PROJECT_ROOT/.harness/scripts/refresh-state.py" "$PROJECT_ROOT" >/dev/null
harness-report "$PROJECT_ROOT" --request-id "$BUG_REQUEST_ID" --format json > "$RCA_REPORT_JSON"
"$PROJECT_ROOT/.harness/bin/harness-log-search" "$PROJECT_ROOT" --task-id T-100 --keyword smoke --json > "$LOG_SEARCH_JSON"
"$PROJECT_ROOT/.harness/bin/harness-log-search" "$PROJECT_ROOT" --task-id T-100 --keyword smoke --detail --json > "$LOG_SEARCH_DETAIL_JSON"
echo "operator unknown dirty" > "$PROJECT_ROOT/manual-unknown-dirty.txt"
python3 "$PROJECT_ROOT/.harness/scripts/refresh-state.py" "$PROJECT_ROOT" >/dev/null
cp "$PROJECT_ROOT/.harness/state/guard-state.json" "$UNKNOWN_DIRTY_GUARD_JSON"
rm -f "$PROJECT_ROOT/manual-unknown-dirty.txt"
echo "managed dirty" > "$PROJECT_ROOT/.worktrees/T-100-smoke/managed-dirty.txt"
python3 "$PROJECT_ROOT/.harness/scripts/refresh-state.py" "$PROJECT_ROOT" >/dev/null
cp "$PROJECT_ROOT/.harness/state/guard-state.json" "$MANAGED_DIRTY_GUARD_JSON"
cp "$PROJECT_ROOT/.harness/state/worktree-registry.json" "$MANAGED_DIRTY_WORKTREE_JSON"
rm -f "$PROJECT_ROOT/.worktrees/T-100-smoke/managed-dirty.txt"
python3 "$PROJECT_ROOT/.harness/scripts/refresh-state.py" "$PROJECT_ROOT" >/dev/null
"$PROJECT_ROOT/.harness/bin/harness-runner" daemon "$PROJECT_ROOT" --interval 1 --dispatch-mode print --replace >/dev/null
sleep 2
python3 "$PROJECT_ROOT/.harness/scripts/refresh-state.py" "$PROJECT_ROOT" >/dev/null
"$PROJECT_ROOT/.harness/bin/harness-ops" "$PROJECT_ROOT" --format json top > "$OPS_TOP_JSON"
"$PROJECT_ROOT/.harness/bin/harness-ops" "$PROJECT_ROOT" --format json queue > "$OPS_QUEUE_JSON"
"$PROJECT_ROOT/.harness/bin/harness-ops" "$PROJECT_ROOT" --format json workers > "$OPS_WORKERS_JSON"
"$PROJECT_ROOT/.harness/bin/harness-ops" "$PROJECT_ROOT" --format json task T-100 > "$OPS_TASK_JSON"
"$PROJECT_ROOT/.harness/bin/harness-ops" "$PROJECT_ROOT" --format json daemon status > "$OPS_DAEMON_JSON"
"$PROJECT_ROOT/.harness/bin/harness-ops" "$PROJECT_ROOT" --format json worktrees > "$OPS_WORKTREES_JSON"
"$PROJECT_ROOT/.harness/bin/harness-ops" "$PROJECT_ROOT" --format json merge-queue > "$OPS_MERGE_QUEUE_JSON"
"$PROJECT_ROOT/.harness/bin/harness-ops" "$PROJECT_ROOT" --format json conflicts > "$OPS_CONFLICTS_JSON"
"$PROJECT_ROOT/.harness/bin/harness-ops" "$PROJECT_ROOT" --format json doctor > "$OPS_DOCTOR_JSON"
"$PROJECT_ROOT/.harness/bin/harness-ops" "$PROJECT_ROOT" watch --view top --count 1 > "$OPS_WATCH_TEXT"
harness-tasks "$PROJECT_ROOT" tasks --format json > "$TASKS_JSON"
harness-task "$PROJECT_ROOT" T-100 --format json > "$TASK_DETAIL_JSON"
harness-task "$PROJECT_ROOT" T-100 logs --detail --format json > "$TASK_LOG_JSON"
harness-control "$PROJECT_ROOT" daemon status --format json > "$CONTROL_DAEMON_JSON"
harness-control "$PROJECT_ROOT" task T-100 archive --reason "smoke archive" --format json > "$CONTROL_ARCHIVE_JSON"
harness-control "$PROJECT_ROOT" project archive --reason "smoke project archive" --format json > "$CONTROL_PROJECT_ARCHIVE_JSON"
python3 "$PROJECT_ROOT/.harness/scripts/refresh-state.py" "$PROJECT_ROOT" >/dev/null
cp "$PROJECT_ROOT/.harness/state/completion-gate.json" "$FINAL_COMPLETION_GATE_JSON"
"$PROJECT_ROOT/.harness/bin/harness-runner" daemon-stop "$PROJECT_ROOT" >/dev/null

python3 - <<'PY' "$PROJECT_ROOT" "$AUTO_INIT_ROOT" "$AUTO_INIT_JSON" "$REQUEST_ID" "$BUG_REQUEST_ID" "$RECONCILE_JSON" "$RUN_JSON" "$RECOVER_JSON" "$FINALIZE_JSON" "$REPORT_JSON" "$DUPLICATE_SUBMIT_JSON" "$CONTEXT_SUBMIT_JSON" "$INSPECTION_SUBMIT_JSON" "$APPEND_SUBMIT_JSON" "$APPEND2_SUBMIT_JSON" "$COMPOUND_SUBMIT_JSON" "$INSPECTION_RECONCILE_JSON" "$APPEND_RECONCILE_JSON" "$APPEND2_RECONCILE_JSON" "$COMPOUND_RECONCILE_JSON" "$BUG_RECONCILE_JSON" "$REPAIR_RECONCILE_JSON" "$REPAIR_RUN_JSON" "$REPAIR_FINALIZE_JSON" "$RCA_REPORT_JSON" "$LOG_SEARCH_JSON" "$LOG_SEARCH_DETAIL_JSON" "$OPS_TOP_JSON" "$OPS_QUEUE_JSON" "$OPS_WORKERS_JSON" "$OPS_TASK_JSON" "$OPS_DAEMON_JSON" "$OPS_WORKTREES_JSON" "$OPS_MERGE_QUEUE_JSON" "$OPS_CONFLICTS_JSON" "$OPS_DOCTOR_JSON" "$OPS_WATCH_TEXT" "$TASKS_JSON" "$TASK_DETAIL_JSON" "$TASK_LOG_JSON" "$CONTROL_DAEMON_JSON" "$CONTROL_ARCHIVE_JSON" "$UNKNOWN_DIRTY_GUARD_JSON" "$MANAGED_DIRTY_GUARD_JSON" "$MANAGED_DIRTY_WORKTREE_JSON" "$CONTROL_PROJECT_ARCHIVE_JSON" "$FINAL_COMPLETION_GATE_JSON"
import json
import sys
from pathlib import Path

project_root = Path(sys.argv[1])
auto_init_root = Path(sys.argv[2])
auto_init_submit = json.load(open(sys.argv[3]))
request_id = sys.argv[4]
bug_request_id = sys.argv[5]
reconcile = json.load(open(sys.argv[6]))
run_payload = json.load(open(sys.argv[7]))
recover_payload = json.load(open(sys.argv[8]))
finalize_payload = json.load(open(sys.argv[9]))
report = json.load(open(sys.argv[10]))
duplicate_submit = json.load(open(sys.argv[11]))
context_submit = json.load(open(sys.argv[12]))
inspection_submit = json.load(open(sys.argv[13]))
append_submit = json.load(open(sys.argv[14]))
append2_submit = json.load(open(sys.argv[15]))
compound_submit = json.load(open(sys.argv[16]))
inspection_reconcile = json.load(open(sys.argv[17]))
append_reconcile = json.load(open(sys.argv[18]))
append2_reconcile = json.load(open(sys.argv[19]))
compound_reconcile = json.load(open(sys.argv[20]))
bug_reconcile = json.load(open(sys.argv[21]))
repair_reconcile = json.load(open(sys.argv[22]))
repair_run = json.load(open(sys.argv[23]))
repair_finalize = json.load(open(sys.argv[24]))
rca_report = json.load(open(sys.argv[25]))
log_search = json.load(open(sys.argv[26]))
log_search_detail = json.load(open(sys.argv[27]))
ops_top = json.load(open(sys.argv[28]))
ops_queue = json.load(open(sys.argv[29]))
ops_workers = json.load(open(sys.argv[30]))
ops_task = json.load(open(sys.argv[31]))
ops_daemon = json.load(open(sys.argv[32]))
ops_worktrees = json.load(open(sys.argv[33]))
ops_merge_queue = json.load(open(sys.argv[34]))
ops_conflicts = json.load(open(sys.argv[35]))
ops_doctor = json.load(open(sys.argv[36]))
ops_watch_text = Path(sys.argv[37]).read_text()
tasks_view = json.load(open(sys.argv[38]))
task_detail = json.load(open(sys.argv[39]))
task_log = json.load(open(sys.argv[40]))
control_daemon = json.load(open(sys.argv[41]))
control_archive = json.load(open(sys.argv[42]))
unknown_dirty_guard = json.load(open(sys.argv[43]))
managed_dirty_guard = json.load(open(sys.argv[44]))
managed_dirty_worktree = json.load(open(sys.argv[45]))
control_project_archive = json.load(open(sys.argv[46]))
final_completion_gate = json.load(open(sys.argv[47]))

assert auto_init_submit["ok"] is True
assert (auto_init_root / ".harness").exists()

assert reconcile["bound"], "request should bind to at least one task"
assert reconcile["bound"][0]["requestId"] == request_id

dispatched = run_payload["dispatched"]
assert dispatched["taskId"] == "T-100"
assert dispatched["dispatchMode"] == "print"
assert dispatched["executionCwd"].endswith(".worktrees/T-100-smoke")
assert dispatched["routeDecision"]["resumeStrategy"] == "resume"
assert dispatched["routeDecision"]["gateStatus"] == "claimable"

recover = recover_payload["dispatched"]
assert recover["taskId"] == "T-100"
assert recover["routeDecision"]["resumeStrategy"] == "resume"
assert finalize_payload["taskId"] == "T-100"
assert finalize_payload["finalStatus"] == "completed"
assert finalize_payload["compactLogPath"] == ".harness/log-T-100.md"

request_map = json.load(open(project_root / ".harness/state/request-task-map.json"))
binding = next(item for item in request_map["bindings"] if item["requestId"] == request_id)
history_statuses = [entry["status"] for entry in binding["history"]]
assert "bound" in history_statuses
assert "dispatched" in history_statuses
assert "running" in history_statuses
assert "recoverable" in history_statuses
assert "resumed" in history_statuses
assert "verified" in history_statuses
assert "merge_queued" in history_statuses
assert "merge_checked" in history_statuses or "merged" in history_statuses
assert "completed" in history_statuses

report_request = report["selectedRequest"]
assert report_request["requestId"] == request_id
assert report_request["status"] == "completed"
assert report["activeBinding"]["taskId"] == "T-100"
assert report["activeBinding"]["verificationStatus"] == "pass"
assert report["activeBinding"]["sessionId"] == "sess-worker-100"

assert duplicate_submit["normalizedIntentClass"] == "duplicate_or_noop"
assert duplicate_submit["frontDoorClass"] == "duplicate_or_context"
assert duplicate_submit["fusionDecision"] == "duplicate_of_existing"
assert duplicate_submit["status"] == "completed"

assert context_submit["normalizedIntentClass"] == "context_enrichment"
assert context_submit["frontDoorClass"] == "duplicate_or_context"
assert context_submit["fusionDecision"] == "merged_as_context"
assert context_submit["status"] == "completed"

assert inspection_submit["normalizedIntentClass"] == "inspection"
assert inspection_submit["frontDoorClass"] in {"inspection", "advisory_read_only"}
assert inspection_submit["fusionDecision"] == "inspection_overlay"
assert any(item["mode"] == "inspection_overlay" for item in inspection_reconcile["internalized"])

assert append_submit["normalizedIntentClass"] == "append_change"
assert append_submit["frontDoorClass"] == "work_order"
assert append_submit["fusionDecision"] == "append_requires_replan"
assert any(item["mode"] == "append_requires_replan" for item in append_reconcile["internalized"])

assert append2_submit["normalizedIntentClass"] == "append_change"
assert append2_submit["fusionDecision"] == "append_requires_replan"
assert any(item["mode"] == "append_requires_replan" for item in append2_reconcile["internalized"])

assert compound_submit["normalizedIntentClass"] == "compound_split"
assert compound_submit["fusionDecision"] == "compound_split_created"
compound_internal = next(item for item in compound_reconcile["internalized"] if item["requestId"] == compound_submit["requestId"])
assert compound_internal["followUpRequestIds"], "compound submission should create internal follow-up requests"

request_index = json.load(open(project_root / ".harness/state/request-index.json"))
assert any(item["kind"] == "audit" for item in request_index["requests"]), "verification should emit an audit follow-up request"

lineage_index = json.load(open(project_root / ".harness/state/lineage-index.json"))
assert lineage_index["eventCount"] > 0
assert request_id in lineage_index["requests"]

raw_log_path = project_root / ".harness/state/runner-logs/T-100.log"
compact_log_path = project_root / ".harness/log-T-100.md"
assert raw_log_path.exists(), "raw runner log should still exist"
assert compact_log_path.exists(), "compact handoff log should exist after finalize"
compact_text = compact_log_path.read_text()
assert "One-screen summary" in compact_text
assert "Cross-worker relevant facts" in compact_text

progress_json = json.load(open(project_root / ".harness/state/progress.json"))
progress_md_text = (project_root / ".harness/progress.md").read_text()
assert progress_json["currentTaskId"] == "T-100"
assert "```json" not in progress_md_text
assert "rendered from `.harness/state/progress.json`" in progress_md_text

log_index = json.load(open(project_root / ".harness/state/log-index.json"))
assert log_index["compactLogCount"] >= 2
assert "T-100" in log_index["logsByTaskId"]
assert any(item["taskId"] == "T-100" for item in log_search["matches"])
assert log_search["matchCount"] >= 1
assert log_search_detail["matches"][0]["detailWindows"], "detail mode should return raw evidence windows"

assert any(item["requestId"] == bug_request_id and item["rcaId"] for item in bug_reconcile["bound"]), "bug request should allocate RCA"
repair_request = next(
    item for item in request_index["requests"]
    if item.get("parentRequestId") == bug_request_id and item.get("source") == "runtime:rca"
)
assert repair_request["kind"] == "implementation"

repair_bound = next(item for item in repair_reconcile["bound"] if item["requestId"] == repair_request["requestId"])
assert repair_bound["taskId"] == "T-101"

repair_dispatched = repair_run["dispatched"]
assert repair_dispatched["taskId"] == "T-101"
assert repair_dispatched["dispatchMode"] == "print"
assert repair_finalize["taskId"] == "T-101"
assert repair_finalize["compactLogPath"] == ".harness/log-T-101.md"

root_cause_log = [json.loads(line) for line in open(project_root / ".harness/root-cause-log.jsonl") if line.strip()]
latest_by_rca = {}
for entry in root_cause_log:
    latest_by_rca[entry["rcaId"]] = entry
latest_records = list(latest_by_rca.values())
assert latest_records, "root cause log should contain RCA records"
latest_rca = latest_records[-1]
assert latest_rca["primaryCauseDimension"] == "verification_guardrail"
assert latest_rca["ownerRole"] == "verifier/architect"
assert latest_rca["repairMode"] == "test-fix"
assert latest_rca["status"] == "repaired"
assert latest_rca["repairRequestId"] == repair_request["requestId"]
assert latest_rca["preventionAction"]

root_cause_summary = json.load(open(project_root / ".harness/state/root-cause-summary.json"))
assert root_cause_summary["rcaCount"] >= 1
assert root_cause_summary["openCount"] == 0
assert root_cause_summary["byPrimaryCauseDimension"]["verification_guardrail"] >= 1
assert root_cause_summary["byOwnerRole"]["verifier/architect"] >= 1
assert not root_cause_summary["bugsMissingLineageCorrelation"]

bug_request = next(item for item in request_index["requests"] if item["requestId"] == bug_request_id)
assert bug_request["status"]

assert rca_report["selectedRequest"]["requestId"] == bug_request_id
assert rca_report["rootCauseSummary"]["rcaCount"] >= 1
assert rca_report["rootCauseSummary"]["openCount"] == 0

research_index = json.load(open(project_root / ".harness/state/research-index.json"))
assert research_index["memoCount"] >= 1
assert research_index["researchModes"]["targeted"] >= 1
assert "smoke-runtime-scan" in research_index["bySlug"]

queue_summary = json.load(open(project_root / ".harness/state/queue-summary.json"))
intake_summary = json.load(open(project_root / ".harness/state/intake-summary.json"))
thread_state = json.load(open(project_root / ".harness/state/thread-state.json"))
change_summary = json.load(open(project_root / ".harness/state/change-summary.json"))
task_summary = json.load(open(project_root / ".harness/state/task-summary.json"))
worker_summary = json.load(open(project_root / ".harness/state/worker-summary.json"))
daemon_summary = json.load(open(project_root / ".harness/state/daemon-summary.json"))
worktree_registry = json.load(open(project_root / ".harness/state/worktree-registry.json"))
merge_queue = json.load(open(project_root / ".harness/state/merge-queue.json"))
merge_summary = json.load(open(project_root / ".harness/state/merge-summary.json"))
policy_summary = json.load(open(project_root / ".harness/state/policy-summary.json"))
research_summary = json.load(open(project_root / ".harness/state/research-summary.json"))
todo_summary = json.load(open(project_root / ".harness/state/todo-summary.json"))
completion_gate = json.load(open(project_root / ".harness/state/completion-gate.json"))
guard_state = json.load(open(project_root / ".harness/state/guard-state.json"))

assert queue_summary["totalRequests"] >= 2
assert intake_summary["byFrontDoorClass"]
assert intake_summary["duplicateCount"] >= 1
assert intake_summary["contextMergeCount"] >= 1
assert intake_summary["inspectionOverlayCount"] >= 1
assert change_summary["appendChangeCount"] >= 2
assert thread_state["contextRotWarningCount"] >= 1
thread_entry = thread_state["threads"][report_request["threadKey"]]
assert thread_entry["mergedContextRefs"], "merged context should materialize into thread state"
assert thread_entry["contextDigest"], "thread state should keep a compact context digest"
assert "taskStatusCounts" in task_summary
assert "workerNodes" in worker_summary
assert any(item["taskId"] == "T-100" for item in worktree_registry["worktrees"])
assert "items" in merge_queue
assert "queueDepth" in merge_summary
assert daemon_summary["dispatchBackendDefault"] == "print"
assert daemon_summary["runtimeHealth"] in {"healthy", "degraded"}
assert policy_summary["dispatch"]["defaultBackend"] == "tmux"
assert research_summary["memoCount"] >= 1
assert report["intakeSummary"]["byFrontDoorClass"]
assert "actionableTodoCount" in todo_summary
assert completion_gate["status"] in {"open", "satisfied", "retired"}
assert guard_state["status"] in {"ready", "blocked", "retire_ready", "archived"}
assert guard_state["pendingCheckpointCount"] >= 0

assert ops_top["dispatchBackendDefault"] == "print"
assert ops_top["runtimeHealth"] in {"healthy", "degraded"}
assert ops_top["guardStatus"] in {"ready", "blocked", "retire_ready", "archived"}
assert ops_top["completionGateStatus"] in {"open", "satisfied", "retired"}
assert ops_queue["queueDepth"] >= 0
assert "dispatchBackendCounts" in ops_workers
assert ops_task["taskId"] == "T-100"
assert ops_task["worktreePath"] == ".worktrees/T-100-smoke"
assert ops_task["status"]
assert "mergeStatus" in ops_task
assert ops_daemon["dispatchBackendDefault"] == "print"
assert any(item["taskId"] == "T-100" for item in ops_worktrees["worktrees"])
assert "recentMerged" in ops_merge_queue
assert ops_conflicts["conflictCount"] >= 0
assert "workerBackendHealth" in ops_daemon
assert ops_doctor["ok"] in {True, False}
assert "Harness Ops Top" in ops_watch_text

assert tasks_view["taskStatusCounts"]
assert task_detail["taskId"] == "T-100"
assert task_detail["threadKey"]
assert task_log["frontMatter"]["taskId"] == "T-100"
assert control_daemon["dispatchBackendDefault"] == "print"
assert control_archive["action"] == "archive"
assert control_archive["taskId"] == "T-100"
assert unknown_dirty_guard["unknownDirtyCount"] >= 1
assert unknown_dirty_guard["safeToExecute"] is False
assert any("unknown dirty worktree" in item for item in unknown_dirty_guard["blockers"])
assert managed_dirty_guard["systemOwnedDirtyCount"] >= 1
assert managed_dirty_guard["pendingCheckpointCount"] >= 1
managed_worktree = next(item for item in managed_dirty_worktree["worktrees"] if item["taskId"] == "T-100")
assert managed_worktree["dirtyState"] == "system_owned_dirty"
assert managed_worktree["pendingCheckpoint"] is True
assert control_project_archive["action"] == "archive"
assert control_project_archive["status"] == "archived"
assert final_completion_gate["status"] == "retired"
assert final_completion_gate["retired"] is True

archived_task_pool = json.load(open(project_root / ".harness/task-pool.json"))
archived_task = next(task for task in archived_task_pool["tasks"] if task["taskId"] == "T-100")
assert archived_task["cleanupStatus"] == "archived"
project_meta = json.load(open(project_root / ".harness/project-meta.json"))
assert project_meta["lifecycle"] == "archived"
PY

echo "release smoke passed"
