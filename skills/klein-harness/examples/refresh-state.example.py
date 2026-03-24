#!/usr/bin/env python3
from __future__ import annotations
import json
import sys
from collections import Counter
from pathlib import Path

from runtime_common import (
    ACTIVE_TASK_COLUMNS,
    LINEAGE_ACTIVE_BINDING_COLUMNS,
    REQUEST_BINDING_COLUMNS,
    align_flat_records,
    TASK_ACTIVE_STATUSES,
    build_change_summary,
    build_completion_gate,
    build_guard_state,
    build_feedback_summary,
    build_intake_summary,
    build_lineage_index,
    build_log_index,
    build_merge_summary,
    build_policy_summary,
    build_progress_summary,
    build_queue_summary,
    build_research_summary,
    build_root_cause_summary,
    build_research_index,
    build_request_summary,
    build_task_summary,
    build_thread_state,
    build_todo_summary,
    build_worker_summary,
    collect_dirty_state,
    apply_dirty_state_to_worktree_registry,
    context_rot_score,
    build_daemon_summary,
    evaluate_task_drift_checklist,
    ensure_runtime_scaffold,
    load_json,
    load_jsonl,
    load_optional_json,
    load_policy_summary,
    load_runner_daemon_state,
    load_runner_heartbeats,
    load_merge_queue,
    load_worktree_registry,
    maybe_complete_request,
    now_iso,
    read_progress_state,
    reconcile_requests,
    write_progress_projection,
    write_json,
)


def active_tasks(tasks):
    return [task for task in tasks if task.get("status") in TASK_ACTIVE_STATUSES]


RUN_RECORD_COLUMNS = [
    "taskId",
    "dispatchMode",
    "dispatchBackend",
    "tmuxSession",
    "strategy",
    "sessionId",
    "executionCwd",
    "logPath",
    "promptPath",
    "runnerScriptPath",
    "dispatchedAt",
    "gateReason",
]

ROT_RECORD_COLUMNS = [
    "taskId",
    "threadKey",
    "planEpoch",
    "contextRotScore",
    "contextRotStatus",
    "contextRotReasons",
]

DRIFT_RECORD_COLUMNS = [
    "taskId",
    "threadKey",
    "planEpoch",
    "failures",
]


def main():
    if len(sys.argv) != 2:
        print(f"usage: {sys.argv[0]} <ROOT>", file=sys.stderr)
        sys.exit(1)

    root = Path(sys.argv[1]).resolve()
    files = ensure_runtime_scaffold(root, generator="harness-refresh-state")
    reconcile_requests(root, generator="harness-refresh-state")

    policy_summary = load_policy_summary(files["policy_summary_path"], default_generator="harness-refresh-state")
    progress = read_progress_state(files, generator="harness-refresh-state")
    task_pool = load_json(files["harness"] / "task-pool.json")
    work_items = load_json(files["harness"] / "work-items.json")
    spec = load_json(files["harness"] / "spec.json")
    features = load_json(files["harness"] / "features.json")
    session_registry = load_json(files["session_registry_path"])
    feedback_entries = load_jsonl(files["feedback_log_path"])
    feedback_summary = build_feedback_summary(feedback_entries)
    root_cause_entries = load_jsonl(files["root_cause_log_path"])
    root_cause_summary = build_root_cause_summary(root_cause_entries)
    log_index = build_log_index(root)
    research_index = build_research_index(root)
    runner_state = load_json(files["runner_state_path"])
    heartbeats = load_runner_heartbeats(files["state_dir"])
    daemon_state = load_runner_daemon_state(files["state_dir"])
    worktree_registry = load_worktree_registry(files, generator="harness-refresh-state")
    merge_queue = load_merge_queue(files, generator="harness-refresh-state")
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
    thread_state = build_thread_state(
        request_index,
        task_pool,
        request_summary,
        generator="klein-harness",
        policy_summary=policy_summary,
    )
    intake_summary = build_intake_summary(request_index, thread_state, generator="klein-harness", policy_summary=policy_summary)
    change_summary = build_change_summary(request_index, task_pool, thread_state, generator="klein-harness", policy_summary=policy_summary)
    queue_summary = build_queue_summary(request_index, request_summary, generator="klein-harness", policy_summary=policy_summary)
    task_summary = build_task_summary(task_pool, feedback_summary, lineage_index, runner_state, generator="klein-harness", policy_summary=policy_summary)
    worker_summary = build_worker_summary(task_pool, session_registry, runner_state, heartbeats, generator="klein-harness", policy_summary=policy_summary)
    daemon_summary = build_daemon_summary(daemon_state, runner_state, worker_summary, generator="klein-harness", policy_summary=policy_summary)
    merge_summary = build_merge_summary(merge_queue, worktree_registry, generator="klein-harness")
    research_summary = build_research_summary(research_index, generator="klein-harness")
    project_meta = load_optional_json(files["project_meta_path"], {})
    dirty_state = collect_dirty_state(root, task_pool, policy_summary)
    worktree_registry = apply_dirty_state_to_worktree_registry(worktree_registry, dirty_state, generator="klein-harness")
    todo_summary = build_todo_summary(
        task_pool,
        queue_summary,
        request_summary,
        merge_summary,
        dirty_state,
        generator="klein-harness",
        policy_summary=policy_summary,
    )
    completion_gate = build_completion_gate(
        spec,
        features,
        task_pool,
        request_summary,
        merge_summary,
        feedback_summary,
        todo_summary,
        project_meta,
        generator="klein-harness",
    )
    guard_state = build_guard_state(
        root,
        project_meta,
        queue_summary,
        task_summary,
        worker_summary,
        daemon_summary,
        worktree_registry,
        merge_summary,
        todo_summary,
        completion_gate,
        dirty_state,
        generator="klein-harness",
        policy_summary=policy_summary,
    )
    progress = build_progress_summary(
        progress,
        request_summary,
        task_summary,
        worker_summary,
        daemon_summary,
        todo_summary,
        generator="klein-harness",
    )
    progress["todoActionableCount"] = todo_summary.get("actionableTodoCount", 0)
    progress["completionGateStatus"] = completion_gate.get("status")
    progress["guardStatus"] = guard_state.get("status")
    progress["pendingCheckpointCount"] = guard_state.get("pendingCheckpointCount", 0)
    progress["unknownDirtyCount"] = guard_state.get("unknownDirtyCount", 0)
    progress["blockers"] = guard_state.get("blockers", [])
    active_request = request_summary.get("activeRequest") or {}
    active_binding = next(
        (
            binding for binding in request_summary.get("bindings", [])
            if binding.get("requestId") == active_request.get("requestId")
        ),
        None,
    )
    next_task_id = (
        (active_binding or {}).get("taskId")
        or next((task_id for task_id in todo_summary.get("nextTaskIds", []) if task_id), None)
    )
    next_todo = next((item for item in todo_summary.get("todoItems", []) if item.get("taskId") == next_task_id), None)
    next_actions = []
    if guard_state.get("unknownDirtyCount", 0):
        next_actions.append("keep safeToExecute=false until unknown dirty is classified and resolved")
    if next_task_id:
        action = f"run {next_task_id}"
        if guard_state.get("safeToExecute") is False and (next_todo or {}).get("roleHint") == "orchestrator":
            action += " as control-plane planning only; do not mutate business code"
        next_actions.append(action)
    progress["nextActions"] = next_actions

    current_state = {
        "schemaVersion": "1.0",
        "generator": "klein-harness",
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
        "currentIntentClass": active_request.get("normalizedIntentClass"),
        "currentFusionDecision": active_request.get("fusionDecision"),
        "currentThreadKey": active_request.get("targetThreadKey") or active_request.get("threadKey"),
        "currentPlanEpoch": active_request.get("targetPlanEpoch"),
        "currentBindingId": active_binding.get("bindingId") if active_binding else None,
        "currentSessionId": active_binding.get("sessionId") if active_binding else None,
        "currentVerificationStatus": active_binding.get("verificationStatus") if active_binding else None,
        "activeRequestId": active_request.get("requestId"),
        "activeRequestKind": active_request.get("kind"),
        "activeRequestStatus": active_request.get("status"),
        "activeRequestTaskId": active_binding.get("taskId") if active_binding else None,
        "activeRequestSessionId": active_binding.get("sessionId") if active_binding else None,
        "blockers": progress.get("blockers", []),
        "blockerCount": len(progress.get("blockers", [])),
        "blockersCsv": " | ".join(progress.get("blockers", [])[:10]) if progress.get("blockers") else None,
        "nextActions": progress.get("nextActions", []),
        "nextActionCount": len(progress.get("nextActions", [])),
        "nextActionsCsv": " | ".join(progress.get("nextActions", [])[:10]) if progress.get("nextActions") else None,
        "lastAuditStatus": progress.get("lastAuditStatus"),
        "recentFailureDigest": feedback_summary.get("recentFailures", [])[-3:],
        "recentFailureColumns": ["id", "taskId", "sessionId", "feedbackType", "severity", "message", "timestamp"],
        "recentFailureRecords": align_flat_records(
            feedback_summary.get("recentFailures", [])[-3:],
            ["id", "taskId", "sessionId", "feedbackType", "severity", "message", "timestamp"],
        ),
        "recentIllegalActionTaskIds": sorted(
            {
                entry.get("taskId")
                for entry in feedback_summary.get("recentFailures", [])
                if entry.get("feedbackType") == "illegal_action" and entry.get("taskId")
            }
        ),
        "recentIllegalActionTaskIdsCsv": ", ".join(
            sorted(
                {
                    entry.get("taskId")
                    for entry in feedback_summary.get("recentFailures", [])
                    if entry.get("feedbackType") == "illegal_action" and entry.get("taskId")
                }
            )
        ) or None,
        "openRootCauseCount": root_cause_summary.get("openCount", 0),
        "latestRootCauseDimension": next(
            (item.get("primaryCauseDimension") for item in reversed(root_cause_summary.get("recentAllocations", [])) if item.get("primaryCauseDimension")),
            None,
        ),
        "compactLogCount": log_index.get("compactLogCount", 0),
        "researchMemoCount": research_index.get("memoCount", 0),
        "queueDepth": queue_summary.get("queueDepth", 0),
        "activeWorktreeCount": len(worktree_registry.get("worktrees", [])),
        "mergeQueueDepth": merge_summary.get("queueDepth", 0),
        "mergeConflictCount": merge_summary.get("conflictCount", 0),
        "readyToMergeCount": merge_summary.get("readyToMergeCount", 0),
        "duplicateRequestCount": intake_summary.get("duplicateCount", 0),
        "contextMergeCount": intake_summary.get("contextMergeCount", 0),
        "inspectionOverlayCount": intake_summary.get("inspectionOverlayCount", 0),
        "runtimeHealth": daemon_summary.get("runtimeHealth"),
        "dispatchBackendDefault": daemon_summary.get("dispatchBackendDefault"),
        "guardStatus": guard_state.get("status"),
        "completionGateStatus": completion_gate.get("status"),
        "todoActionableCount": todo_summary.get("actionableTodoCount", 0),
        "pendingCheckpointCount": guard_state.get("pendingCheckpointCount", 0),
        "unknownDirtyCount": guard_state.get("unknownDirtyCount", 0),
    }

    rot_entries = []
    drift_failures = []
    for task in tasks:
        compact_log = log_index.get("logsByTaskId", {}).get(task.get("taskId")) if isinstance(log_index.get("logsByTaskId"), dict) else None
        rot = context_rot_score(task, request_summary, heartbeats, thread_state, compact_log, policy_summary)
        drift = evaluate_task_drift_checklist(task, latest_plan_epoch=rot.get("latestPlanEpoch"), request_summary=request_summary, compact_log=compact_log)
        rot_entries.append(
            {
                "taskId": task.get("taskId"),
                "threadKey": task.get("threadKey"),
                "planEpoch": task.get("planEpoch"),
                "contextRotScore": rot.get("score"),
                "contextRotStatus": rot.get("status"),
                "contextRotReasons": rot.get("reasons"),
            }
        )
        if drift.get("failures"):
            drift_failures.append(
                {
                    "taskId": task.get("taskId"),
                    "threadKey": task.get("threadKey"),
                    "planEpoch": task.get("planEpoch"),
                    "failures": drift.get("failures"),
                }
            )

    active_task_records = [
        {
            "taskId": task.get("taskId"),
            "kind": task.get("kind"),
            "roleHint": task.get("roleHint"),
            "status": task.get("status"),
            "threadKey": task.get("threadKey"),
            "planEpoch": task.get("planEpoch"),
            "title": task.get("title"),
            "nodeId": task.get("claim", {}).get("nodeId"),
            "branchName": task.get("branchName"),
            "baseRef": task.get("baseRef") or (task.get("dispatch") or {}).get("baseRef"),
            "worktreePath": task.get("worktreePath"),
            "worktreeStatus": task.get("worktreeStatus"),
            "integrationBranch": task.get("integrationBranch") or task_pool.get("integrationBranch"),
            "mergeStatus": task.get("mergeStatus"),
            "cleanupStatus": task.get("cleanupStatus"),
            "dispatchBackend": task.get("claim", {}).get("dispatchBackend"),
            "boundSessionId": task.get("claim", {}).get("boundSessionId"),
        }
        for task in active
    ]
    active_run_records = align_flat_records(runner_state.get("activeRuns", []), RUN_RECORD_COLUMNS)
    recoverable_run_records = align_flat_records(runner_state.get("recoverableRuns", []), RUN_RECORD_COLUMNS)
    stale_run_records = align_flat_records(runner_state.get("staleRuns", []), RUN_RECORD_COLUMNS)
    rot_record_rows = align_flat_records(rot_entries, ROT_RECORD_COLUMNS)
    drift_record_rows = align_flat_records(drift_failures, DRIFT_RECORD_COLUMNS)
    active_binding_record = next(
        (
            record
            for record in request_summary.get("bindingRecords", [])
            if active_binding and record.get("bindingId") == active_binding.get("bindingId")
        ),
        None,
    )
    active_lineage_binding_record = next(
        (
            record
            for record in lineage_index.get("activeBindingRecords", [])
            if active_binding and record.get("bindingId") == active_binding.get("bindingId")
        ),
        None,
    )

    runtime_state = {
        "schemaVersion": "1.0",
        "generator": "klein-harness",
        "generatedAt": now_iso(),
        "orchestrationSessionId": session_registry.get("orchestrationSessionId"),
        "activeTaskCount": len(active),
        "activeWorkerCount": worker_summary.get("workerCount", 0),
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
                "integrationBranch": task.get("integrationBranch") or task_pool.get("integrationBranch"),
                "mergeStatus": task.get("mergeStatus"),
                "recentFailures": feedback_summary.get("taskFeedbackSummary", {}).get(task.get("taskId"), {}).get("recentFailures", []),
            }
            for task in active
        ],
        "activeTaskColumns": ACTIVE_TASK_COLUMNS,
        "activeTaskRecords": align_flat_records(active_task_records, ACTIVE_TASK_COLUMNS),
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
        "activeRunColumns": RUN_RECORD_COLUMNS,
        "activeRunRecords": active_run_records,
        "recoverableRuns": runner_state.get("recoverableRuns", []),
        "recoverableRunColumns": RUN_RECORD_COLUMNS,
        "recoverableRunRecords": recoverable_run_records,
        "staleRuns": runner_state.get("staleRuns", []),
        "staleRunColumns": RUN_RECORD_COLUMNS,
        "staleRunRecords": stale_run_records,
        "lastTickAt": runner_state.get("lastTickAt"),
        "lastTrigger": runner_state.get("lastTrigger"),
        "requestCounts": request_summary.get("requestCounts", {}),
        "requestCountRecords": request_summary.get("requestCountRecords", []),
        "activeRequest": active_request,
        "activeRequestRecord": request_summary.get("activeRequestRecord"),
        "activeBinding": active_binding,
        "activeBindingRecord": active_binding_record,
        "activeThreadKey": active_request.get("targetThreadKey") or active_request.get("threadKey"),
        "activePlanEpoch": active_request.get("targetPlanEpoch"),
        "boundRequestCount": request_summary.get("boundRequestCount", 0),
        "runningRequestCount": request_summary.get("runningRequestCount", 0),
        "recoverableRequestCount": request_summary.get("recoverableRequestCount", 0),
        "blockedRequestCount": request_summary.get("blockedRequestCount", 0),
        "completedRequestCount": request_summary.get("completedRequestCount", 0),
        "duplicateRequestCount": intake_summary.get("duplicateCount", 0),
        "contextMergeCount": intake_summary.get("contextMergeCount", 0),
        "inspectionOverlayCount": intake_summary.get("inspectionOverlayCount", 0),
        "compoundSplitCount": intake_summary.get("compoundSplitCount", 0),
        "lineageEventCount": lineage_index.get("eventCount", 0),
        "lineageLastSeq": lineage_index.get("lastSeq", 0),
        "activeLineageBindings": lineage_index.get("activeBindings", []),
        "activeLineageBindingColumns": LINEAGE_ACTIVE_BINDING_COLUMNS,
        "activeLineageBindingRecords": lineage_index.get("activeBindingRecords", []),
        "activeLineageBindingRecord": active_lineage_binding_record,
        "lineageRequestCount": len(lineage_index.get("requests", {})),
        "requestBindings": request_summary.get("bindings", []),
        "requestBindingColumns": REQUEST_BINDING_COLUMNS,
        "requestBindingRecords": request_summary.get("bindingRecords", []),
        "rootCauseCount": root_cause_summary.get("rcaCount", 0),
        "openRootCauseCount": root_cause_summary.get("openCount", 0),
        "underdeterminedRootCauseCount": root_cause_summary.get("underdeterminedCount", 0),
        "rootCauseByDimension": root_cause_summary.get("byPrimaryCauseDimension", {}),
        "rootCauseByOwner": root_cause_summary.get("byOwnerRole", {}),
        "openRootCauseItems": root_cause_summary.get("openItems", []),
        "bugsMissingLineageCorrelation": root_cause_summary.get("bugsMissingLineageCorrelation", []),
        "compactLogCount": log_index.get("compactLogCount", 0),
        "recentHighSignalLogs": log_index.get("recentHighSignalLogs", []),
        "openLogBlockers": log_index.get("openBlockers", []),
        "recurringLogTags": log_index.get("recurringTags", {}),
        "worktreeRegistry": worktree_registry,
        "mergeQueue": merge_queue,
        "mergeSummary": merge_summary,
        "activeWorktrees": worktree_registry.get("worktrees", []),
        "mergeQueueDepth": merge_summary.get("queueDepth", 0),
        "mergeConflictCount": merge_summary.get("conflictCount", 0),
        "readyToMergeCount": merge_summary.get("readyToMergeCount", 0),
        "researchMemoCount": research_index.get("memoCount", 0),
        "researchModes": research_index.get("researchModes", {}),
        "recentResearchMemos": research_index.get("recentMemos", []),
        "queueDepth": queue_summary.get("queueDepth", 0),
        "intakeSummary": intake_summary,
        "threadState": thread_state,
        "changeSummary": change_summary,
        "todoSummary": todo_summary,
        "completionGate": completion_gate,
        "guardState": guard_state,
        "contextRotWarnings": [item for item in rot_entries if item.get("contextRotStatus") in {"warning", "downgraded"}][:10],
        "contextRotWarningColumns": ROT_RECORD_COLUMNS,
        "contextRotWarningRecords": [item for item in rot_record_rows if item.get("contextRotStatus") in {"warning", "downgraded"}][:10],
        "driftChecklistFailures": drift_failures[:10],
        "driftChecklistFailureColumns": DRIFT_RECORD_COLUMNS,
        "driftChecklistFailureRecords": drift_record_rows[:10],
        "runtimeHealth": daemon_summary.get("runtimeHealth"),
        "dispatchBackendDefault": daemon_summary.get("dispatchBackendDefault"),
        "workerBackendCounts": worker_summary.get("dispatchBackendCounts", {}),
        "workerBackendCountRecords": daemon_summary.get("workerBackendHealthRecords", []),
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
        "generator": "klein-harness",
        "generatedAt": now_iso(),
        "specRevision": spec.get("specRevision"),
        "planningStage": spec.get("planningStage"),
        "objective": spec.get("objective"),
        "integrationBranch": task_pool.get("integrationBranch"),
        "mergeQueueDepth": merge_summary.get("queueDepth", 0),
        "mergeConflictCount": merge_summary.get("conflictCount", 0),
        "readyToMergeCount": merge_summary.get("readyToMergeCount", 0),
        "taskStatusCounts": dict(Counter(task.get("status", "unknown") for task in tasks)),
        "compactLogCount": log_index.get("compactLogCount", 0),
        "researchMemoCount": research_index.get("memoCount", 0),
        "researchModes": research_index.get("researchModes", {}),
        "completionGateStatus": completion_gate.get("status"),
        "todoActionableCount": todo_summary.get("actionableTodoCount", 0),
        "blocks": blocks,
    }

    project_meta["schemaVersion"] = "1.0"
    project_meta["generator"] = "klein-harness"
    project_meta["generatedAt"] = now_iso()
    project_meta["lastCompletionGateStatus"] = completion_gate.get("status")
    project_meta["retireEligible"] = completion_gate.get("retireEligible", False)
    write_json(files["project_meta_path"], project_meta)

    write_progress_projection(files, progress, generator="klein-harness")
    write_json(files["state_dir"] / "current.json", current_state)
    write_json(files["state_dir"] / "runtime.json", runtime_state)
    write_json(files["state_dir"] / "blueprint-index.json", blueprint_index)
    write_json(files["feedback_summary_path"], feedback_summary)
    write_json(files["root_cause_summary_path"], root_cause_summary)
    write_json(files["log_index_path"], log_index)
    write_json(files["worktree_registry_path"], worktree_registry)
    write_json(files["merge_queue_path"], merge_queue)
    write_json(files["merge_summary_path"], merge_summary)
    write_json(files["intake_summary_path"], intake_summary)
    write_json(files["thread_state_path"], thread_state)
    write_json(files["change_summary_path"], change_summary)
    write_json(files["todo_summary_path"], todo_summary)
    write_json(files["completion_gate_path"], completion_gate)
    write_json(files["guard_state_path"], guard_state)
    write_json(files["queue_summary_path"], queue_summary)
    write_json(files["task_summary_path"], task_summary)
    write_json(files["worker_summary_path"], worker_summary)
    write_json(files["daemon_summary_path"], daemon_summary)
    write_json(files["policy_summary_path"], policy_summary)
    write_json(files["research_index_path"], research_index)
    write_json(files["research_summary_path"], research_summary)
    write_json(files["request_summary_path"], request_summary)
    write_json(files["lineage_index_path"], lineage_index)

    print(json.dumps({"ok": True, "stateDir": str(files["state_dir"])}, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    try:
        main()
    except Exception as exc:
        print(f"refresh-state example failed: {exc}", file=sys.stderr)
        sys.exit(1)
