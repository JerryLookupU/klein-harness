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
mkdir -p "$PROJECT_ROOT/.harness/.worktrees/T-101-rca-repair"
touch "$PROJECT_ROOT/.harness/.worktrees/T-101-rca-repair/smoke-rca-pass.txt"

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
      "planningStage": "execution-ready",
      "lineagePath": ["F-100", "WI-101", "T-101"],
      "baseRef": "refs/heads/orch/spec-S-100",
      "branchName": "task/T-101-rca-repair",
      "worktreePath": ".harness/.worktrees/T-101-rca-repair",
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
        "worktreePath": ".harness/.worktrees/T-101-rca-repair",
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
BUG_SUBMIT_JSON="$TMP_ROOT/bug-submit.json"
RECONCILE_JSON="$TMP_ROOT/reconcile.json"
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
python3 "$PROJECT_ROOT/.harness/scripts/runner.py" finalize "$PROJECT_ROOT" T-100 --tmux-session "print:T-100" --runner-status 0 > "$FINALIZE_JSON"
python3 "$PROJECT_ROOT/.harness/scripts/refresh-state.py" "$PROJECT_ROOT" >/dev/null
harness-report "$PROJECT_ROOT" --request-id "$REQUEST_ID" --format json > "$REPORT_JSON"

cat >> "$PROJECT_ROOT/.harness/feedback-log.jsonl" <<'EOF'
{"id":"FB-100","taskId":"T-100","sessionId":"sess-worker-100","role":"worker","workerMode":"execution","feedbackType":"verification_failure","severity":"error","source":"verification","step":"verify","triggeringAction":"post-release smoke bug intake","message":"Smoke task T-100 regressed after verification and now requires RCA allocation.","timestamp":"2026-03-22T00:05:00+08:00"}
EOF

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

python3 - <<'PY' "$PROJECT_ROOT" "$REQUEST_ID" "$BUG_REQUEST_ID" "$RECONCILE_JSON" "$RUN_JSON" "$RECOVER_JSON" "$FINALIZE_JSON" "$REPORT_JSON" "$BUG_RECONCILE_JSON" "$REPAIR_RECONCILE_JSON" "$REPAIR_RUN_JSON" "$REPAIR_FINALIZE_JSON" "$RCA_REPORT_JSON" "$LOG_SEARCH_JSON" "$LOG_SEARCH_DETAIL_JSON"
import json
import sys
from pathlib import Path

project_root = Path(sys.argv[1])
request_id = sys.argv[2]
bug_request_id = sys.argv[3]
reconcile = json.load(open(sys.argv[4]))
run_payload = json.load(open(sys.argv[5]))
recover_payload = json.load(open(sys.argv[6]))
finalize_payload = json.load(open(sys.argv[7]))
report = json.load(open(sys.argv[8]))
bug_reconcile = json.load(open(sys.argv[9]))
repair_reconcile = json.load(open(sys.argv[10]))
repair_run = json.load(open(sys.argv[11]))
repair_finalize = json.load(open(sys.argv[12]))
rca_report = json.load(open(sys.argv[13]))
log_search = json.load(open(sys.argv[14]))
log_search_detail = json.load(open(sys.argv[15]))

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

raw_log_path = project_root / ".harness/state/runner-logs/T-100.log"
compact_log_path = project_root / ".harness/log-T-100.md"
assert raw_log_path.exists(), "raw runner log should still exist"
assert compact_log_path.exists(), "compact handoff log should exist after finalize"
compact_text = compact_log_path.read_text()
assert "One-screen summary" in compact_text
assert "Cross-worker relevant facts" in compact_text

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
assert bug_request["status"] == "completed"

assert rca_report["selectedRequest"]["requestId"] == bug_request_id
assert rca_report["rootCauseSummary"]["rcaCount"] >= 1
assert rca_report["rootCauseSummary"]["openCount"] == 0

research_index = json.load(open(project_root / ".harness/state/research-index.json"))
assert research_index["memoCount"] >= 1
assert research_index["researchModes"]["targeted"] >= 1
assert "smoke-runtime-scan" in research_index["bySlug"]
PY

echo "release smoke passed"
