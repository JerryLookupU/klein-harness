#!/usr/bin/env python3
from __future__ import annotations
import argparse
import json
import sys
from collections import Counter
from pathlib import Path

from runtime_common import (
    apply_dirty_state_to_worktree_registry,
    build_change_summary,
    build_completion_gate,
    build_feedback_summary,
    build_guard_state,
    build_intake_summary,
    build_log_index,
    build_root_cause_summary,
    build_request_id,
    build_request_summary,
    build_thread_state,
    build_todo_summary,
    checkpoint_active_thread_tasks,
    collect_dirty_state,
    ensure_request_index_shape,
    ensure_runtime_scaffold,
    find_request,
    infer_request_impact_class,
    lineage_event,
    load_json,
    load_jsonl,
    load_policy_summary,
    load_optional_json,
    normalize_request_record,
    now_iso,
    normalize_context_paths,
    record_request_thread_state,
    reconcile_requests,
    supersede_queued_thread_tasks,
    update_request_snapshot,
    upsert_request_record,
    update_request_status,
    write_json,
)


def write_request_hot_state(root: Path, files: dict, index: dict):
    task_map = load_json(files["request_task_map_path"])
    task_pool = load_optional_json(files["harness"] / "task-pool.json")
    policy_summary = load_policy_summary(files["policy_summary_path"], default_generator="harness-request")
    feedback_summary = load_optional_json(files["feedback_summary_path"]) or build_feedback_summary(load_jsonl(files["feedback_log_path"]))
    log_index = load_optional_json(files["log_index_path"]) or build_log_index(root)
    request_summary = build_request_summary(index, task_map, task_pool)
    thread_state = build_thread_state(index, task_pool, request_summary, generator="harness-request", policy_summary=policy_summary)
    intake_summary = build_intake_summary(index, thread_state, generator="harness-request", policy_summary=policy_summary)
    change_summary = build_change_summary(index, task_pool, thread_state, generator="harness-request", policy_summary=policy_summary)
    queue_summary = load_optional_json(files["queue_summary_path"], {})
    merge_summary = load_optional_json(files["merge_summary_path"], {})
    daemon_summary = load_optional_json(files["daemon_summary_path"], {})
    worker_summary = load_optional_json(files["worker_summary_path"], {})
    task_summary = load_optional_json(files["task_summary_path"], {})
    worktree_registry = load_optional_json(files["worktree_registry_path"], {})
    dirty_state = collect_dirty_state(root, task_pool, policy_summary)
    worktree_registry = apply_dirty_state_to_worktree_registry(worktree_registry, dirty_state, generator="harness-request")
    todo_summary = build_todo_summary(task_pool, queue_summary, request_summary, merge_summary, dirty_state, generator="harness-request", policy_summary=policy_summary)
    completion_gate = build_completion_gate(
        load_optional_json(files["harness"] / "spec.json", {}),
        load_optional_json(files["harness"] / "features.json", {}),
        task_pool,
        request_summary,
        merge_summary,
        feedback_summary,
        todo_summary,
        load_optional_json(files["project_meta_path"], {}),
        generator="harness-request",
    )
    guard_state = build_guard_state(
        root,
        load_optional_json(files["project_meta_path"], {}),
        queue_summary,
        task_summary,
        worker_summary,
        daemon_summary,
        worktree_registry,
        merge_summary,
        todo_summary,
        completion_gate,
        dirty_state,
        generator="harness-request",
        policy_summary=policy_summary,
    )
    write_json(files["request_summary_path"], request_summary)
    write_json(files["thread_state_path"], thread_state)
    write_json(files["intake_summary_path"], intake_summary)
    write_json(files["change_summary_path"], change_summary)
    write_json(files["worktree_registry_path"], worktree_registry)
    write_json(files["todo_summary_path"], todo_summary)
    write_json(files["completion_gate_path"], completion_gate)
    write_json(files["guard_state_path"], guard_state)
    return request_summary, thread_state, intake_summary, change_summary


def cmd_submit(args):
    root = Path(args.root).resolve()
    files = ensure_runtime_scaffold(root, generator="harness-request")
    index = load_json(files["request_index_path"])
    ensure_request_index_shape(index)
    task_map = load_json(files["request_task_map_path"])
    task_pool = load_optional_json(files["harness"] / "task-pool.json")
    thread_state = load_optional_json(files["thread_state_path"], index) or index
    seq = int(index.get("nextSeq", 1))
    request_id = build_request_id(seq)
    request = {
        "requestId": request_id,
        "seq": seq,
        "source": args.source,
        "kind": args.kind or "implementation",
        "goal": args.goal,
        "projectRoot": str(root),
        "contextPaths": normalize_context_paths(root, args.context or []),
        "threadKey": args.thread_key,
        "idempotencyKey": args.idempotency_key,
        "priority": args.priority,
        "scope": args.scope,
        "mergePolicy": args.merge_policy,
        "replyPolicy": args.reply_policy,
        "status": "queued",
        "createdAt": now_iso(),
    }

    summary = normalize_request_record(
        {
            **request,
            "summary": args.goal[:120],
            "boundTaskIds": [],
            "bindingIds": [],
            "statusReason": None,
            "updatedAt": request["createdAt"],
        },
        index=index,
        task_map=task_map,
        thread_state=thread_state,
        generator="harness-request",
    )

    impact_classification = "continue_safe"
    impacted_task_ids = []
    superseded_task_ids = []
    checkpoint_task_ids = []
    if task_pool:
        impact_classification, impacted = infer_request_impact_class(summary, task_pool.get("tasks", []), load_optional_json(files["feedback_summary_path"], {}))
        impacted_task_ids = [item.get("taskId") for item in impacted if item.get("taskId")]
        summary["impactClassification"] = impact_classification
        summary["impactedTaskIds"] = impacted_task_ids
        if summary.get("normalizedIntentClass") == "append_change" and impact_classification in {"supersede_queued", "checkpoint_then_replan"}:
            superseded_task_ids = supersede_queued_thread_tasks(root, summary, generator="harness-request")
            checkpoint_task_ids = checkpoint_active_thread_tasks(root, summary, generator="harness-request")
            summary["supersededQueuedTaskIds"] = superseded_task_ids
            summary["checkpointTaskIds"] = checkpoint_task_ids

    with files["queue_path"].open("a", encoding="utf-8") as handle:
        handle.write(json.dumps(summary, ensure_ascii=False) + "\n")

    if summary.get("fusionDecision") in {"duplicate_of_existing", "merged_as_context", "noop"}:
        summary["status"] = "completed"
        summary["effectStatus"] = "closed"
        summary["statusReason"] = summary.get("classificationReason")

    index["nextSeq"] = seq + 1
    index["generatedAt"] = request["createdAt"]
    index["generator"] = "harness-request"
    upsert_request_record(index, summary)
    record_request_thread_state(
        index,
        summary,
        impact_classification=summary.get("impactClassification"),
        impacted_task_ids=impacted_task_ids,
        superseded_task_ids=superseded_task_ids,
        checkpoint_task_ids=checkpoint_task_ids,
        generator="harness-request",
    )
    write_json(files["request_index_path"], index)
    update_request_snapshot(files, summary, generator="harness-request")
    write_request_hot_state(root, files, index)

    lineage_event(
        root,
        "request.submitted",
        "harness-request",
        request_id=request_id,
        detail=args.goal,
        context={
            "kindHint": args.kind,
            "source": args.source,
            "frontDoorClass": summary.get("frontDoorClass"),
            "normalizedIntentClass": summary.get("normalizedIntentClass"),
            "fusionDecision": summary.get("fusionDecision"),
            "threadKey": summary.get("targetThreadKey"),
            "targetPlanEpoch": summary.get("targetPlanEpoch"),
            "impactClassification": summary.get("impactClassification"),
        },
    )

    print(json.dumps({
        "ok": True,
        "requestId": request_id,
        "status": summary.get("status"),
        "frontDoorClass": summary.get("frontDoorClass"),
        "threadKey": summary.get("targetThreadKey"),
        "normalizedIntentClass": summary.get("normalizedIntentClass"),
        "fusionDecision": summary.get("fusionDecision"),
        "targetPlanEpoch": summary.get("targetPlanEpoch"),
        "impactClassification": summary.get("impactClassification"),
        "duplicateOfRequestId": summary.get("duplicateOfRequestId"),
        "mergedIntoRequestId": summary.get("mergedIntoRequestId"),
        "effectRequestIds": summary.get("effectRequestIds", []),
        "queuePath": str(files["queue_path"]),
    }, ensure_ascii=False, indent=2))
    return 0


def build_report_payload(root: Path, request_id: str | None):
    files = ensure_runtime_scaffold(root, generator="harness-report")
    index = load_json(files["request_index_path"])
    task_map = load_json(files["request_task_map_path"])
    current = load_optional_json(files["state_dir"] / "current.json", {})
    runtime = load_optional_json(files["state_dir"] / "runtime.json", {})
    queue_summary = load_optional_json(files["queue_summary_path"], {})
    intake_summary = load_optional_json(files["intake_summary_path"], {})
    thread_state = load_optional_json(files["thread_state_path"], {})
    change_summary = load_optional_json(files["change_summary_path"], {})
    todo_summary = load_optional_json(files["todo_summary_path"], {})
    completion_gate = load_optional_json(files["completion_gate_path"], {})
    guard_state = load_optional_json(files["guard_state_path"], {})
    task_summary = load_optional_json(files["task_summary_path"], {})
    worker_summary = load_optional_json(files["worker_summary_path"], {})
    daemon_summary = load_optional_json(files["daemon_summary_path"], {})
    worktree_registry = load_optional_json(files["worktree_registry_path"], {})
    merge_summary = load_optional_json(files["merge_summary_path"], {})
    request_summary = load_optional_json(files["request_summary_path"]) or build_request_summary(index, task_map, None)
    lineage_index = load_optional_json(files["lineage_index_path"], {})
    root_cause_summary = load_optional_json(files["root_cause_summary_path"]) or build_root_cause_summary(load_jsonl(files["root_cause_log_path"]))
    session_registry = load_optional_json(files["session_registry_path"], {})
    project_meta = load_optional_json(files["project_meta_path"], {})
    requests = index.get("requests", [])
    selected = find_request(requests, request_id) if request_id else None
    active_request = selected or request_summary.get("activeRequest")
    active_binding = None
    if active_request:
        active_binding = next(
            (
                binding for binding in request_summary.get("bindings", [])
                if binding.get("requestId") == active_request.get("requestId")
            ),
            None,
        )

    return {
        "projectRoot": str(root),
        "projectLifecycle": project_meta.get("lifecycle"),
        "bootstrapStatus": project_meta.get("bootstrapStatus"),
        "requestCounts": request_summary.get("requestCounts") or dict(Counter(request.get("status", "unknown") for request in requests)),
        "totalRequests": len(requests),
        "selectedRequest": selected,
        "activeRequest": active_request,
        "recentRequests": request_summary.get("recentRequests", requests[-5:]),
        "requestBindings": request_summary.get("bindings", []),
        "activeBinding": active_binding,
        "currentFocus": current.get("currentFocus"),
        "currentRole": current.get("currentRole"),
        "currentTaskId": current.get("currentTaskId"),
        "currentTaskTitle": current.get("currentTaskTitle"),
        "activeTaskCount": runtime.get("activeTaskCount", 0),
        "activeRunnerCount": runtime.get("activeRunnerCount", 0),
        "queueDepth": queue_summary.get("queueDepth", 0),
        "intakeSummary": intake_summary,
        "threadState": thread_state,
        "changeSummary": change_summary,
        "todoSummary": todo_summary,
        "completionGate": completion_gate,
        "guardState": guard_state,
        "recoverableTaskCount": runtime.get("recoverableTaskCount", 0),
        "recoverableRequestCount": request_summary.get("recoverableRequestCount", 0),
        "blockedRequestCount": request_summary.get("blockedRequestCount", 0),
        "verifiedTaskCount": runtime.get("verifiedTaskCount", 0),
        "failingVerificationCount": runtime.get("failingVerificationCount", 0),
        "taskSummary": task_summary,
        "workerSummary": worker_summary,
        "daemonSummary": daemon_summary,
        "worktreeRegistry": worktree_registry,
        "mergeSummary": merge_summary,
        "lastTickAt": runtime.get("lastTickAt"),
        "lastTrigger": runtime.get("lastTrigger"),
        "orchestrationSessionId": runtime.get("orchestrationSessionId") or session_registry.get("orchestrationSessionId"),
        "lineageEventCount": lineage_index.get("eventCount", 0),
        "lineage": lineage_index.get("requests", {}).get(active_request.get("requestId")) if active_request else None,
        "rootCauseSummary": root_cause_summary,
        "openRootCauseItems": root_cause_summary.get("openItems", []),
        "bugsMissingLineageCorrelation": root_cause_summary.get("bugsMissingLineageCorrelation", []),
    }


def format_report_text(payload: dict):
    lines = [
        f"project: {payload.get('projectRoot')}",
        f"lifecycle: {payload.get('projectLifecycle')}",
        f"bootstrapStatus: {payload.get('bootstrapStatus')}",
        f"totalRequests: {payload.get('totalRequests')}",
        f"requestCounts: {payload.get('requestCounts')}",
        f"currentFocus: {payload.get('currentFocus')}",
        f"currentRole: {payload.get('currentRole')}",
        f"currentTask: {payload.get('currentTaskId')} {payload.get('currentTaskTitle')}",
        f"activeTaskCount: {payload.get('activeTaskCount')}",
        f"activeRunnerCount: {payload.get('activeRunnerCount')}",
        f"queueDepth: {payload.get('queueDepth')}",
        f"frontDoorClasses: {payload.get('intakeSummary', {}).get('byFrontDoorClass', {})}",
        f"duplicateRequestCount: {payload.get('intakeSummary', {}).get('duplicateCount', 0)}",
        f"contextMergeCount: {payload.get('intakeSummary', {}).get('contextMergeCount', 0)}",
        f"inspectionOverlayCount: {payload.get('intakeSummary', {}).get('inspectionOverlayCount', 0)}",
        f"todoActionableCount: {payload.get('todoSummary', {}).get('actionableTodoCount', 0)}",
        f"completionGateStatus: {payload.get('completionGate', {}).get('status')}",
        f"guardStatus: {payload.get('guardState', {}).get('status')}",
        f"pendingCheckpointCount: {payload.get('guardState', {}).get('pendingCheckpointCount', 0)}",
        f"unknownDirtyCount: {payload.get('guardState', {}).get('unknownDirtyCount', 0)}",
        f"recoverableTaskCount: {payload.get('recoverableTaskCount')}",
        f"recoverableRequestCount: {payload.get('recoverableRequestCount')}",
        f"blockedRequestCount: {payload.get('blockedRequestCount')}",
        f"verifiedTaskCount: {payload.get('verifiedTaskCount')}",
        f"failingVerificationCount: {payload.get('failingVerificationCount')}",
        f"runtimeHealth: {payload.get('daemonSummary', {}).get('runtimeHealth')}",
        f"dispatchBackendDefault: {payload.get('daemonSummary', {}).get('dispatchBackendDefault')}",
        f"workerCount: {payload.get('workerSummary', {}).get('workerCount')}",
        f"activeWorktreeCount: {len(payload.get('worktreeRegistry', {}).get('worktrees', []))}",
        f"mergeQueueDepth: {payload.get('mergeSummary', {}).get('queueDepth', 0)}",
        f"readyToMergeCount: {payload.get('mergeSummary', {}).get('readyToMergeCount', 0)}",
        f"mergeConflictCount: {payload.get('mergeSummary', {}).get('conflictCount', 0)}",
        f"lineageEventCount: {payload.get('lineageEventCount')}",
        f"rootCauseCount: {payload.get('rootCauseSummary', {}).get('rcaCount', 0)}",
        f"openRootCauseCount: {payload.get('rootCauseSummary', {}).get('openCount', 0)}",
    ]
    active_request = payload.get("selectedRequest") or payload.get("activeRequest")
    if active_request:
        lines.extend([
            "",
            f"activeRequest: {active_request.get('requestId')}",
            f"requestKind: {active_request.get('kind')}",
            f"kindHint: {active_request.get('kindHint')}",
            f"requestStatus: {active_request.get('status')}",
            f"requestGoal: {active_request.get('goal')}",
            f"frontDoorClass: {active_request.get('frontDoorClass')}",
            f"normalizedIntentClass: {active_request.get('normalizedIntentClass')}",
            f"fusionDecision: {active_request.get('fusionDecision')}",
            f"threadKey: {active_request.get('threadKey')}",
            f"targetPlanEpoch: {active_request.get('targetPlanEpoch')}",
            f"impactClassification: {active_request.get('impactClassification')}",
        ])
        if active_request.get("rcaId"):
            lines.append(f"rcaId: {active_request.get('rcaId')}")
            lines.append(f"primaryCauseDimension: {active_request.get('primaryCauseDimension')}")
        active_binding = payload.get("activeBinding")
        if active_binding:
            lines.extend([
                f"boundTask: {active_binding.get('taskId')} {active_binding.get('taskTitle')}",
                f"bindingStatus: {active_binding.get('bindingStatus')}",
                f"boundSession: {active_binding.get('sessionId')}",
                f"worktreePath: {active_binding.get('worktreePath')}",
                f"mergeStatus: {active_binding.get('outcome', {}).get('status') or active_binding.get('bindingStatus')}",
                f"verification: {active_binding.get('verificationStatus')}",
                f"verificationResultPath: {active_binding.get('verificationResultPath')}",
                f"diffSummary: {active_binding.get('diffSummary')}",
            ])
    if payload.get("openRootCauseItems"):
        lines.extend(["", "openRootCauseItems:"])
        for item in payload["openRootCauseItems"][:5]:
            lines.append(
                f"- {item.get('rcaId')} {item.get('primaryCauseDimension')} owner={item.get('ownerRole')} status={item.get('status')}"
            )
    elif payload.get("recentRequests"):
        lines.append("")
        lines.append("recentRequests:")
        for request in payload["recentRequests"]:
            lines.append(f"- {request.get('requestId')} [{request.get('status')}] {request.get('kind')} {request.get('goal')}")
    return "\n".join(lines)


def cmd_report(args):
    root = Path(args.root).resolve()
    payload = build_report_payload(root, args.request_id)
    if args.format == "text":
        print(format_report_text(payload))
    else:
        print(json.dumps(payload, ensure_ascii=False, indent=2))
    return 0


def cmd_reconcile(args):
    root = Path(args.root).resolve()
    result = reconcile_requests(root, generator="harness-reconcile")
    files = ensure_runtime_scaffold(root, generator="harness-reconcile")
    index = load_json(files["request_index_path"])
    write_request_hot_state(root, files, index)
    print(json.dumps({"ok": True, **result}, ensure_ascii=False, indent=2))
    return 0


def cmd_cancel(args):
    root = Path(args.root).resolve()
    files = ensure_runtime_scaffold(root, generator="harness-request")
    index = load_json(files["request_index_path"])
    update_request_status(index, args.request_id, "cancelled", reason=args.reason or "cancelled by operator")
    index["generatedAt"] = now_iso()
    index["generator"] = "harness-request"
    write_json(files["request_index_path"], index)
    update_request_snapshot(files, find_request(index.get("requests", []), args.request_id), generator="harness-request")
    write_request_hot_state(root, files, index)
    lineage_event(
        root,
        "request.cancelled",
        "harness-request",
        request_id=args.request_id,
        detail=args.reason or "cancelled by operator",
    )
    print(json.dumps({"ok": True, "requestId": args.request_id, "status": "cancelled"}, ensure_ascii=False, indent=2))
    return 0


def main():
    parser = argparse.ArgumentParser(description="request intake and closed-loop lifecycle tools")
    sub = parser.add_subparsers(dest="command")

    p_submit = sub.add_parser("submit", help="append a request into the project queue")
    p_submit.add_argument("--root", required=True)
    p_submit.add_argument("--kind")
    p_submit.add_argument("--goal", required=True)
    p_submit.add_argument("--source", default="shell")
    p_submit.add_argument("--context", action="append", default=[])
    p_submit.add_argument("--thread-key")
    p_submit.add_argument("--idempotency-key")
    p_submit.add_argument("--priority", default="P2")
    p_submit.add_argument("--scope", default="project")
    p_submit.add_argument("--merge-policy", default="append")
    p_submit.add_argument("--reply-policy", default="summary")

    p_report = sub.add_parser("report", help="summarize request queue, binding, lineage, and runtime state")
    p_report.add_argument("--root", required=True)
    p_report.add_argument("--request-id")
    p_report.add_argument("--format", default="text", choices=["text", "json"])

    p_reconcile = sub.add_parser("reconcile", help="bind queued requests to current tasks")
    p_reconcile.add_argument("--root", required=True)

    p_cancel = sub.add_parser("cancel", help="cancel a queued or active request")
    p_cancel.add_argument("--root", required=True)
    p_cancel.add_argument("--request-id", required=True)
    p_cancel.add_argument("--reason")

    args = parser.parse_args()
    if args.command == "submit":
        return cmd_submit(args)
    if args.command == "report":
        return cmd_report(args)
    if args.command == "reconcile":
        return cmd_reconcile(args)
    if args.command == "cancel":
        return cmd_cancel(args)
    parser.print_help()
    return 1


if __name__ == "__main__":
    try:
        sys.exit(main())
    except Exception as exc:
        print(f"request example failed: {exc}", file=sys.stderr)
        sys.exit(1)
