#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"

TMP_ROOT="$(mktemp -d)"
CODEX_HOME_DIR="$TMP_ROOT/codex"
PROJECT_ROOT="$TMP_ROOT/release-smoke-project"

cleanup() {
  rm -rf "$TMP_ROOT"
}
trap cleanup EXIT

export CODEX_HOME="$CODEX_HOME_DIR"
export PATH="$CODEX_HOME_DIR/bin:$PATH"

"$REPO_ROOT/install.sh" --dest "$CODEX_HOME_DIR/skills" --bin-dir "$CODEX_HOME_DIR/bin" --no-shell-rc --force >/dev/null
harness-init "$PROJECT_ROOT" >/dev/null

mkdir -p "$PROJECT_ROOT/.harness/.worktrees/T-100-smoke"
touch "$PROJECT_ROOT/.harness/.worktrees/T-100-smoke/smoke-pass.txt"

cat > "$PROJECT_ROOT/.harness/progress.md" <<'EOF'
# Smoke Progress

```json
{
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
  "lastAuditStatus": "pass"
}
```
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
      "worktreePath": ".harness/.worktrees/T-100-smoke",
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
        "worktreePath": ".harness/.worktrees/T-100-smoke",
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
    }
  ]
}
EOF

SUBMIT_JSON="$TMP_ROOT/submit.json"
RECONCILE_JSON="$TMP_ROOT/reconcile.json"
RUN_JSON="$TMP_ROOT/run.json"
RECOVER_JSON="$TMP_ROOT/recover.json"
REPORT_JSON="$TMP_ROOT/report.json"

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
"$PROJECT_ROOT/.harness/bin/harness-verify-task" T-100 "$PROJECT_ROOT" --write-back >/dev/null
python3 "$PROJECT_ROOT/.harness/scripts/refresh-state.py" "$PROJECT_ROOT" >/dev/null
harness-report "$PROJECT_ROOT" --request-id "$REQUEST_ID" --format json > "$REPORT_JSON"

python3 - <<'PY' "$PROJECT_ROOT" "$REQUEST_ID" "$RECONCILE_JSON" "$RUN_JSON" "$RECOVER_JSON" "$REPORT_JSON"
import json
import sys
from pathlib import Path

project_root = Path(sys.argv[1])
request_id = sys.argv[2]
reconcile = json.load(open(sys.argv[3]))
run_payload = json.load(open(sys.argv[4]))
recover_payload = json.load(open(sys.argv[5]))
report = json.load(open(sys.argv[6]))

assert reconcile["bound"], "request should bind to at least one task"
assert reconcile["bound"][0]["requestId"] == request_id

dispatched = run_payload["dispatched"]
assert dispatched["taskId"] == "T-100"
assert dispatched["dispatchMode"] == "print"
assert dispatched["routeDecision"]["resumeStrategy"] == "resume"
assert dispatched["routeDecision"]["gateStatus"] == "claimable"

recover = recover_payload["dispatched"]
assert recover["taskId"] == "T-100"
assert recover["routeDecision"]["resumeStrategy"] == "resume"

request_map = json.load(open(project_root / ".harness/state/request-task-map.json"))
binding = next(item for item in request_map["bindings"] if item["requestId"] == request_id)
history_statuses = [entry["status"] for entry in binding["history"]]
assert "bound" in history_statuses
assert "dispatched" in history_statuses
assert "running" in history_statuses
assert "recoverable" in history_statuses
assert "resumed" in history_statuses
assert "verified" in history_statuses
assert "completed" in history_statuses

report_request = report["selectedRequest"]
assert report_request["requestId"] == request_id
assert report_request["status"] == "completed"
assert report["activeBinding"]["taskId"] == "T-100"
assert report["activeBinding"]["verificationStatus"] == "pass"
assert report["activeBinding"]["sessionId"] == "sess-worker-100"

request_index = json.load(open(project_root / ".harness/state/request-index.json"))
assert any(item["kind"] == "audit" for item in request_index["requests"]), "verification should emit an audit follow-up request"

lineage_index = json.load(open(project_root / ".harness/state/lineage-index.json"))
assert lineage_index["eventCount"] > 0
assert request_id in lineage_index["requests"]
PY

echo "release smoke passed"
