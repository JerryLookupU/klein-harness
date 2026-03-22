#!/usr/bin/env python3
import json
import sys
from collections import Counter
from pathlib import Path

from runtime_common import (
    TASK_ACTIVE_STATUSES,
    build_feedback_summary,
    build_lineage_index,
    build_request_summary,
    ensure_runtime_scaffold,
    load_json,
    load_jsonl,
    load_optional_json,
    load_progress,
    maybe_complete_request,
    now_iso,
    reconcile_requests,
    write_json,
)


def active_tasks(tasks):
    return [task for task in tasks if task.get("status") in TASK_ACTIVE_STATUSES]


def main():
    if len(sys.argv) != 2:
        print(f"usage: {sys.argv[0]} <ROOT>", file=sys.stderr)
        sys.exit(1)

    root = Path(sys.argv[1]).resolve()
    files = ensure_runtime_scaffold(root, generator="harness-refresh-state")
    reconcile_requests(root, generator="harness-refresh-state")

    progress = load_progress(files["harness"] / "progress.md")
    task_pool = load_json(files["harness"] / "task-pool.json")
    work_items = load_json(files["harness"] / "work-items.json")
    spec = load_json(files["harness"] / "spec.json")
    session_registry = load_json(files["session_registry_path"])
    feedback_entries = load_jsonl(files["feedback_log_path"])
    feedback_summary = build_feedback_summary(feedback_entries)
    runner_state = load_json(files["runner_state_path"])
    request_index = load_json(files["request_index_path"])
    request_task_map = load_json(files["request_task_map_path"])
    lineage_entries = load_jsonl(files["lineage_path"])

    for request in request_index.get("requests", []):
        maybe_complete_request(root, request.get("requestId"), generator="harness-refresh-state")
    request_index = load_json(files["request_index_path"])
    request_task_map = load_json(files["request_task_map_path"])

    tasks = task_pool.get("tasks", [])
    items = work_items.get("items", [])
    active = active_tasks(tasks)

    request_summary = build_request_summary(request_index, request_task_map, task_pool)
    lineage_index = build_lineage_index(lineage_entries, task_pool, request_task_map)
    active_request = request_summary.get("activeRequest") or {}
    active_binding = next(
        (
            binding for binding in request_summary.get("bindings", [])
            if binding.get("requestId") == active_request.get("requestId")
        ),
        None,
    )

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
        "currentRequestId": active_request.get("requestId"),
        "currentRequestKind": active_request.get("kind"),
        "currentRequestStatus": active_request.get("status"),
        "currentBindingId": active_binding.get("bindingId") if active_binding else None,
        "currentSessionId": active_binding.get("sessionId") if active_binding else None,
        "currentVerificationStatus": active_binding.get("verificationStatus") if active_binding else None,
        "activeRequestId": active_request.get("requestId"),
        "activeRequestKind": active_request.get("kind"),
        "activeRequestStatus": active_request.get("status"),
        "activeRequestTaskId": active_binding.get("taskId") if active_binding else None,
        "activeRequestSessionId": active_binding.get("sessionId") if active_binding else None,
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
        "activeWorkerCount": sum(1 for task in active if task.get("roleHint") == "worker"),
        "activeAuditWorkerCount": sum(1 for task in active if task.get("kind") == "audit"),
        "activeOrchestratorCount": sum(
            1
            for task in active
            if task.get("kind") in {"orchestration", "replan", "rollback", "merge", "lease-recovery"}
        ),
        "activeTasks": [
            {
                "taskId": task.get("taskId"),
                "kind": task.get("kind"),
                "roleHint": task.get("roleHint"),
                "workerMode": task.get("workerMode"),
                "title": task.get("title"),
                "summary": task.get("summary"),
                "nodeId": task.get("claim", {}).get("nodeId"),
                "boundSessionId": task.get("claim", {}).get("boundSessionId"),
                "branchName": task.get("branchName"),
                "worktreePath": task.get("worktreePath"),
                "recentFailures": feedback_summary.get("taskFeedbackSummary", {}).get(task.get("taskId"), {}).get("recentFailures", []),
            }
            for task in active
        ],
        "activeRunnerCount": len(runner_state.get("activeRuns", [])),
        "recoverableTaskCount": len(runner_state.get("recoverableRuns", [])),
        "staleRunnerCount": len(runner_state.get("staleRuns", [])),
        "blockedRouteCount": len(runner_state.get("blockedRoutes", [])),
        "verifiedTaskCount": sum(1 for task in tasks if task.get("verificationStatus") in {"pass", "skipped"}),
        "failingVerificationCount": sum(1 for task in tasks if task.get("verificationStatus") == "fail"),
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
        "requestCounts": request_summary.get("requestCounts", {}),
        "activeRequest": active_request,
        "activeBinding": active_binding,
        "boundRequestCount": request_summary.get("boundRequestCount", 0),
        "runningRequestCount": request_summary.get("runningRequestCount", 0),
        "recoverableRequestCount": request_summary.get("recoverableRequestCount", 0),
        "blockedRequestCount": request_summary.get("blockedRequestCount", 0),
        "completedRequestCount": request_summary.get("completedRequestCount", 0),
        "lineageEventCount": lineage_index.get("eventCount", 0),
        "lineageLastSeq": lineage_index.get("lastSeq", 0),
        "activeLineageBindings": lineage_index.get("activeBindings", []),
        "lineageRequestCount": len(lineage_index.get("requests", {})),
        "requestBindings": request_summary.get("bindings", []),
    }

    blocks = {}
    for block in spec.get("blocks", []):
        block_id = block.get("id")
        block_items = [item for item in items if set(item.get("featureIds", [])) & set(block.get("featureIds", []))]
        block_tasks = [task for task in tasks if task.get("blockId") == block_id]
        blocks[block_id] = {
            "title": block.get("title"),
            "status": block.get("status"),
            "featureIds": block.get("featureIds", []),
            "workItemIds": [item.get("id") for item in block_items],
            "taskIds": [task.get("taskId") for task in block_tasks],
        }

    blueprint_index = {
        "schemaVersion": "1.0",
        "generator": "harness-architect",
        "generatedAt": now_iso(),
        "specRevision": spec.get("specRevision"),
        "planningStage": spec.get("planningStage"),
        "objective": spec.get("objective"),
        "integrationBranch": task_pool.get("integrationBranch"),
        "taskStatusCounts": dict(Counter(task.get("status", "unknown") for task in tasks)),
        "blocks": blocks,
    }

    write_json(files["state_dir"] / "current.json", current_state)
    write_json(files["state_dir"] / "runtime.json", runtime_state)
    write_json(files["state_dir"] / "blueprint-index.json", blueprint_index)
    write_json(files["feedback_summary_path"], feedback_summary)
    write_json(files["request_summary_path"], request_summary)
    write_json(files["lineage_index_path"], lineage_index)

    print(json.dumps({"ok": True, "stateDir": str(files["state_dir"])}, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    try:
        main()
    except Exception as exc:
        print(f"refresh-state example failed: {exc}", file=sys.stderr)
        sys.exit(1)
