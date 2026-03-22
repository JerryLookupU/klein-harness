#!/usr/bin/env python3
import argparse
import json
import sys
from collections import Counter
from pathlib import Path

from runtime_common import (
    build_request_id,
    build_request_summary,
    ensure_runtime_scaffold,
    find_request,
    lineage_event,
    load_json,
    load_optional_json,
    now_iso,
    normalize_context_paths,
    reconcile_requests,
    upsert_request_record,
    update_request_status,
    write_json,
)


def cmd_submit(args):
    root = Path(args.root).resolve()
    files = ensure_runtime_scaffold(root, generator="harness-request")
    index = load_json(files["request_index_path"])
    seq = int(index.get("nextSeq", 1))
    request_id = build_request_id(seq)
    request = {
        "requestId": request_id,
        "seq": seq,
        "source": args.source,
        "kind": args.kind,
        "goal": args.goal,
        "projectRoot": str(root),
        "contextPaths": normalize_context_paths(root, args.context or []),
        "threadKey": args.thread_key,
        "priority": args.priority,
        "scope": args.scope,
        "mergePolicy": args.merge_policy,
        "replyPolicy": args.reply_policy,
        "status": "queued",
        "createdAt": now_iso(),
    }

    with files["queue_path"].open("a", encoding="utf-8") as handle:
        handle.write(json.dumps(request, ensure_ascii=False) + "\n")

    summary = {
        **request,
        "summary": args.goal[:120],
        "boundTaskIds": [],
        "bindingIds": [],
        "statusReason": None,
        "updatedAt": request["createdAt"],
    }
    index["nextSeq"] = seq + 1
    index["generatedAt"] = request["createdAt"]
    index["generator"] = "harness-request"
    upsert_request_record(index, summary)
    write_json(files["request_index_path"], index)
    write_json(files["request_summary_path"], build_request_summary(index, load_json(files["request_task_map_path"]), None))

    lineage_event(
        root,
        "request.submitted",
        "harness-request",
        request_id=request_id,
        detail=args.goal,
        context={"kind": args.kind, "source": args.source},
    )

    print(json.dumps({
        "ok": True,
        "requestId": request_id,
        "status": "queued",
        "queuePath": str(files["queue_path"]),
    }, ensure_ascii=False, indent=2))
    return 0


def build_report_payload(root: Path, request_id: str | None):
    files = ensure_runtime_scaffold(root, generator="harness-report")
    index = load_json(files["request_index_path"])
    task_map = load_json(files["request_task_map_path"])
    current = load_optional_json(files["state_dir"] / "current.json", {})
    runtime = load_optional_json(files["state_dir"] / "runtime.json", {})
    request_summary = load_optional_json(files["request_summary_path"]) or build_request_summary(index, task_map, None)
    lineage_index = load_optional_json(files["lineage_index_path"], {})
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
        "recoverableTaskCount": runtime.get("recoverableTaskCount", 0),
        "recoverableRequestCount": request_summary.get("recoverableRequestCount", 0),
        "blockedRequestCount": request_summary.get("blockedRequestCount", 0),
        "verifiedTaskCount": runtime.get("verifiedTaskCount", 0),
        "failingVerificationCount": runtime.get("failingVerificationCount", 0),
        "lastTickAt": runtime.get("lastTickAt"),
        "lastTrigger": runtime.get("lastTrigger"),
        "orchestrationSessionId": runtime.get("orchestrationSessionId") or session_registry.get("orchestrationSessionId"),
        "lineageEventCount": lineage_index.get("eventCount", 0),
        "lineage": lineage_index.get("requests", {}).get(active_request.get("requestId")) if active_request else None,
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
        f"recoverableTaskCount: {payload.get('recoverableTaskCount')}",
        f"recoverableRequestCount: {payload.get('recoverableRequestCount')}",
        f"blockedRequestCount: {payload.get('blockedRequestCount')}",
        f"verifiedTaskCount: {payload.get('verifiedTaskCount')}",
        f"failingVerificationCount: {payload.get('failingVerificationCount')}",
        f"lineageEventCount: {payload.get('lineageEventCount')}",
    ]
    active_request = payload.get("selectedRequest") or payload.get("activeRequest")
    if active_request:
        lines.extend([
            "",
            f"activeRequest: {active_request.get('requestId')}",
            f"requestKind: {active_request.get('kind')}",
            f"requestStatus: {active_request.get('status')}",
            f"requestGoal: {active_request.get('goal')}",
        ])
        active_binding = payload.get("activeBinding")
        if active_binding:
            lines.extend([
                f"boundTask: {active_binding.get('taskId')} {active_binding.get('taskTitle')}",
                f"bindingStatus: {active_binding.get('bindingStatus')}",
                f"boundSession: {active_binding.get('sessionId')}",
                f"worktreePath: {active_binding.get('worktreePath')}",
                f"verification: {active_binding.get('verificationStatus')}",
                f"verificationResultPath: {active_binding.get('verificationResultPath')}",
                f"diffSummary: {active_binding.get('diffSummary')}",
            ])
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
    task_map = load_json(files["request_task_map_path"])
    task_pool = load_optional_json(files["harness"] / "task-pool.json")
    write_json(files["request_summary_path"], build_request_summary(index, task_map, task_pool))
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
    write_json(files["request_summary_path"], build_request_summary(index, load_json(files["request_task_map_path"]), load_optional_json(files["harness"] / "task-pool.json")))
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
    p_submit.add_argument("--kind", required=True)
    p_submit.add_argument("--goal", required=True)
    p_submit.add_argument("--source", default="shell")
    p_submit.add_argument("--context", action="append", default=[])
    p_submit.add_argument("--thread-key")
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
