#!/usr/bin/env python3
import json
import re
import sys
from collections import Counter
from datetime import datetime, timezone
from pathlib import Path


def load_json(path: Path):
    return json.loads(path.read_text())


def load_jsonl(path: Path):
    records = []
    if not path.exists():
        return records
    for line in path.read_text().splitlines():
        line = line.strip()
        if not line:
            continue
        records.append(json.loads(line))
    return records


def write_json(path: Path, data):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(data, ensure_ascii=False, indent=2) + "\n")


def load_progress(path: Path):
    text = path.read_text()
    match = re.search(r"```json\s*(\{[\s\S]*?\})\s*```", text)
    if not match:
        raise ValueError(f"missing json block in {path}")
    return json.loads(match.group(1))


def now_iso():
    return datetime.now(timezone.utc).astimezone().isoformat(timespec="seconds")


def active_tasks(tasks):
    return [t for t in tasks if t.get("status") in {"active", "claimed", "in_progress"}]


def sort_key(entry):
    return (entry.get("timestamp") or "", entry.get("id") or "")


def trim_feedback(entry):
    return {
        "id": entry.get("id"),
        "taskId": entry.get("taskId"),
        "sessionId": entry.get("sessionId"),
        "role": entry.get("role"),
        "workerMode": entry.get("workerMode"),
        "feedbackType": entry.get("feedbackType"),
        "severity": entry.get("severity"),
        "source": entry.get("source"),
        "step": entry.get("step"),
        "triggeringAction": entry.get("triggeringAction"),
        "message": entry.get("message"),
        "timestamp": entry.get("timestamp"),
    }


def build_feedback_summary(entries):
    ordered = sorted(entries, key=sort_key)
    errors = [entry for entry in ordered if entry.get("severity") in {"error", "critical"}]
    illegal_actions = [entry for entry in ordered if entry.get("feedbackType") == "illegal_action"]
    recent_failures = [trim_feedback(entry) for entry in errors[-5:]]
    task_summary = {}

    for entry in ordered:
        task_id = entry.get("taskId")
        if not task_id:
            continue
        summary = task_summary.setdefault(
            task_id,
            {
                "taskId": task_id,
                "feedbackCount": 0,
                "errorCount": 0,
                "criticalCount": 0,
                "latestFeedbackType": None,
                "latestSeverity": None,
                "latestMessage": None,
                "latestTimestamp": None,
                "recentFailures": [],
            },
        )
        summary["feedbackCount"] += 1
        if entry.get("severity") in {"error", "critical"}:
            summary["errorCount"] += 1
        if entry.get("severity") == "critical":
            summary["criticalCount"] += 1
        summary["latestFeedbackType"] = entry.get("feedbackType")
        summary["latestSeverity"] = entry.get("severity")
        summary["latestMessage"] = entry.get("message")
        summary["latestTimestamp"] = entry.get("timestamp")

    for task_id in list(task_summary.keys()):
        task_entries = [
            trim_feedback(entry)
            for entry in ordered
            if entry.get("taskId") == task_id and entry.get("severity") in {"error", "critical"}
        ]
        task_summary[task_id]["recentFailures"] = task_entries[-3:]

    return {
        "schemaVersion": "1.0",
        "generator": "harness-architect",
        "generatedAt": now_iso(),
        "feedbackLogPath": ".harness/feedback-log.jsonl",
        "feedbackEventCount": len(ordered),
        "errorCount": len(errors),
        "criticalCount": sum(1 for entry in ordered if entry.get("severity") == "critical"),
        "illegalActionCount": len(illegal_actions),
        "tasksWithRecentFailures": sorted(
            task_id for task_id, summary in task_summary.items() if summary["recentFailures"]
        ),
        "byType": dict(Counter(entry.get("feedbackType", "unknown") for entry in ordered)),
        "bySeverity": dict(Counter(entry.get("severity", "unknown") for entry in ordered)),
        "recentFailures": recent_failures,
        "taskFeedbackSummary": task_summary,
    }


def main():
    if len(sys.argv) != 2:
        print(f"usage: {sys.argv[0]} <ROOT>", file=sys.stderr)
        sys.exit(1)

    root = Path(sys.argv[1]).resolve()
    harness = root / ".harness"
    state_dir = harness / "state"

    progress = load_progress(harness / "progress.md")
    task_pool = load_json(harness / "task-pool.json")
    work_items = load_json(harness / "work-items.json")
    spec = load_json(harness / "spec.json")
    session_registry_path = harness / "session-registry.json"
    session_registry = load_json(session_registry_path) if session_registry_path.exists() else {}
    feedback_log_path = harness / "feedback-log.jsonl"
    feedback_entries = load_jsonl(feedback_log_path)
    feedback_summary = build_feedback_summary(feedback_entries)
    runner_state_path = state_dir / "runner-state.json"
    runner_state = load_json(runner_state_path) if runner_state_path.exists() else {}

    tasks = task_pool.get("tasks", [])
    items = work_items.get("items", [])
    active = active_tasks(tasks)

    current_state = {
        "schemaVersion": "1.0",
        "generator": "harness-architect",
        "generatedAt": now_iso(),
        "mode": progress.get("mode"),
        "planningStage": progress.get("planningStage"),
        "currentFocus": progress.get("currentFocus"),
        "currentRole": progress.get("currentRole"),
        "currentTaskId": progress.get("currentTaskId"),
        "currentTaskTitle": progress.get("currentTaskTitle"),
        "currentTaskSummary": progress.get("currentTaskSummary"),
        "blockers": progress.get("blockers", []),
        "nextActions": progress.get("nextActions", []),
        "lastAuditStatus": progress.get("lastAuditStatus"),
        "recentFailureDigest": feedback_summary.get("recentFailures", [])[-3:],
        "recentIllegalActionTaskIds": sorted(
            {
                entry.get("taskId")
                for entry in feedback_summary.get("recentFailures", [])
                if entry.get("feedbackType") == "illegal_action" and entry.get("taskId")
            }
        ),
    }

    runtime_state = {
        "schemaVersion": "1.0",
        "generator": "harness-architect",
        "generatedAt": now_iso(),
        "orchestrationSessionId": session_registry.get("orchestrationSessionId"),
        "activeTaskCount": len(active),
        "activeWorkerCount": sum(1 for t in active if t.get("roleHint") == "worker"),
        "activeAuditWorkerCount": sum(1 for t in active if t.get("kind") == "audit"),
        "activeOrchestratorCount": sum(1 for t in active if t.get("kind") in {"orchestration", "replan", "rollback", "merge", "lease-recovery"}),
        "activeTasks": [
            {
                "taskId": t.get("taskId"),
                "kind": t.get("kind"),
                "roleHint": t.get("roleHint"),
                "workerMode": t.get("workerMode"),
                "title": t.get("title"),
                "summary": t.get("summary"),
                "nodeId": t.get("claim", {}).get("nodeId"),
                "boundSessionId": t.get("claim", {}).get("boundSessionId"),
                "branchName": t.get("branchName"),
                "worktreePath": t.get("worktreePath"),
                "recentFailures": feedback_summary.get("taskFeedbackSummary", {})
                .get(t.get("taskId"), {})
                .get("recentFailures", []),
            }
            for t in active
        ],
        "activeRunnerCount": len(runner_state.get("activeRuns", [])),
        "recoverableTaskCount": len(runner_state.get("recoverableRuns", [])),
        "staleRunnerCount": len(runner_state.get("staleRuns", [])),
        "blockedRouteCount": len(runner_state.get("blockedRoutes", [])),
        "verifiedTaskCount": sum(1 for t in tasks if t.get("verificationStatus") == "pass"),
        "failingVerificationCount": sum(1 for t in tasks if t.get("verificationStatus") == "fail"),
        "feedbackEventCount": feedback_summary.get("feedbackEventCount", 0),
        "feedbackErrorCount": feedback_summary.get("errorCount", 0),
        "feedbackCriticalCount": feedback_summary.get("criticalCount", 0),
        "illegalActionCount": feedback_summary.get("illegalActionCount", 0),
        "tasksWithRecentFailures": feedback_summary.get("tasksWithRecentFailures", []),
        "recentFailures": feedback_summary.get("recentFailures", []),
        "activeRuns": runner_state.get("activeRuns", []),
        "recoverableRuns": runner_state.get("recoverableRuns", []),
        "staleRuns": runner_state.get("staleRuns", []),
        "lastTickAt": runner_state.get("lastTickAt"),
        "lastTrigger": runner_state.get("lastTrigger"),
    }

    blocks = {}
    for block in spec.get("blocks", []):
        block_id = block.get("id")
        block_items = [w for w in items if set(w.get("featureIds", [])) & set(block.get("featureIds", []))]
        block_tasks = [t for t in tasks if t.get("blockId") == block_id]
        blocks[block_id] = {
            "title": block.get("title"),
            "status": block.get("status"),
            "featureIds": block.get("featureIds", []),
            "workItemIds": [w.get("id") for w in block_items],
            "taskIds": [t.get("taskId") for t in block_tasks],
        }

    blueprint_index = {
        "schemaVersion": "1.0",
        "generator": "harness-architect",
        "generatedAt": now_iso(),
        "specRevision": spec.get("specRevision"),
        "planningStage": spec.get("planningStage"),
        "objective": spec.get("objective"),
        "integrationBranch": task_pool.get("integrationBranch"),
        "taskStatusCounts": dict(Counter(t.get("status", "unknown") for t in tasks)),
        "blocks": blocks,
    }

    write_json(state_dir / "current.json", current_state)
    write_json(state_dir / "runtime.json", runtime_state)
    write_json(state_dir / "blueprint-index.json", blueprint_index)
    write_json(state_dir / "feedback-summary.json", feedback_summary)

    print(json.dumps({"ok": True, "stateDir": str(state_dir)}, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    try:
        main()
    except Exception as exc:
        print(f"refresh-state example failed: {exc}", file=sys.stderr)
        sys.exit(1)
