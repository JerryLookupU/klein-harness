#!/usr/bin/env python3
import json
import re
from collections import Counter
from datetime import datetime, timezone
from pathlib import Path


SCHEMA_VERSION = "1.0"
REQUEST_TERMINAL_STATUSES = {"completed", "cancelled"}
TASK_ACTIVE_STATUSES = {"active", "claimed", "in_progress", "dispatched", "running", "resumed"}
TASK_COMPLETED_STATUSES = {"completed", "validated", "done", "pass", "verified"}
SEVERE_ROUTE_FAILURES = {"illegal_action", "path_conflict", "session_conflict", "replan_required"}


def now_iso():
    return datetime.now(timezone.utc).astimezone().isoformat(timespec="seconds")


def load_json(path: Path):
    return json.loads(path.read_text())


def load_optional_json(path: Path, default=None):
    if path.exists():
        return load_json(path)
    return default


def write_json(path: Path, data):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(data, ensure_ascii=False, indent=2) + "\n")


def load_jsonl(path: Path):
    records = []
    if not path.exists():
        return records
    for line in path.read_text().splitlines():
        line = line.strip()
        if line:
            records.append(json.loads(line))
    return records


def append_jsonl(path: Path, data):
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("a", encoding="utf-8") as handle:
        handle.write(json.dumps(data, ensure_ascii=False) + "\n")


def load_progress(path: Path):
    text = path.read_text()
    match = re.search(r"```json\s*(\{[\s\S]*?\})\s*```", text)
    if not match:
        raise ValueError(f"missing json block in {path}")
    return json.loads(match.group(1))


def ensure_runtime_scaffold(root: Path, generator: str = "harness-runtime"):
    root = root.resolve()
    harness = root / ".harness"
    state_dir = harness / "state"
    requests_dir = harness / "requests"
    archive_dir = requests_dir / "archive"
    verification_dir = state_dir / "verification"
    runner_logs_dir = state_dir / "runner-logs"

    for directory in (
        harness,
        state_dir,
        requests_dir,
        archive_dir,
        verification_dir,
        runner_logs_dir,
        harness / "verification-rules",
        harness / "drift-log",
    ):
        directory.mkdir(parents=True, exist_ok=True)

    queue_path = requests_dir / "queue.jsonl"
    feedback_log_path = harness / "feedback-log.jsonl"
    lineage_path = harness / "lineage.jsonl"
    for log_path in (queue_path, feedback_log_path, lineage_path):
        log_path.touch(exist_ok=True)

    timestamp = now_iso()
    request_index_path = state_dir / "request-index.json"
    if not request_index_path.exists():
        write_json(request_index_path, {
            "schemaVersion": SCHEMA_VERSION,
            "generator": generator,
            "generatedAt": timestamp,
            "nextSeq": 1,
            "requests": [],
        })

    request_task_map_path = state_dir / "request-task-map.json"
    if not request_task_map_path.exists():
        write_json(request_task_map_path, {
            "schemaVersion": SCHEMA_VERSION,
            "generator": generator,
            "generatedAt": timestamp,
            "nextBindingSeq": 1,
            "activeRequestId": None,
            "activeTaskId": None,
            "activeSessionId": None,
            "bindings": [],
        })

    project_meta_path = harness / "project-meta.json"
    if not project_meta_path.exists():
        write_json(project_meta_path, {
            "schemaVersion": SCHEMA_VERSION,
            "generator": generator,
            "generatedAt": timestamp,
            "projectRoot": str(root),
            "lifecycle": "initialized",
            "bootstrapStatus": "not_started",
            "requestQueueEnabled": True,
        })

    session_registry_path = harness / "session-registry.json"
    if not session_registry_path.exists():
        write_json(session_registry_path, {
            "schemaVersion": SCHEMA_VERSION,
            "generator": generator,
            "generatedAt": timestamp,
            "orchestrationSessionId": None,
            "orchestrationSessions": [],
            "sessions": [],
            "families": [],
            "routingDecisions": [],
            "activeBindings": [],
            "recoverableBindings": [],
            "lastCompletedByTask": {},
        })

    runner_state_path = state_dir / "runner-state.json"
    if not runner_state_path.exists():
        write_json(runner_state_path, {
            "schemaVersion": SCHEMA_VERSION,
            "generator": "harness-runner",
            "generatedAt": timestamp,
            "lastTickAt": None,
            "lastTrigger": None,
            "activeRuns": [],
            "recoverableRuns": [],
            "staleRuns": [],
            "dispatchableTaskIds": [],
            "blockedRoutes": [],
            "lastErrors": [],
        })

    runner_heartbeats_path = state_dir / "runner-heartbeats.json"
    if not runner_heartbeats_path.exists():
        write_json(runner_heartbeats_path, {
            "schemaVersion": SCHEMA_VERSION,
            "generator": "harness-runner",
            "generatedAt": timestamp,
            "entries": {},
        })

    feedback_summary_path = state_dir / "feedback-summary.json"
    if not feedback_summary_path.exists():
        write_json(feedback_summary_path, {
            "schemaVersion": SCHEMA_VERSION,
            "generator": generator,
            "generatedAt": timestamp,
            "feedbackLogPath": ".harness/feedback-log.jsonl",
            "feedbackEventCount": 0,
            "errorCount": 0,
            "criticalCount": 0,
            "illegalActionCount": 0,
            "tasksWithRecentFailures": [],
            "byType": {},
            "bySeverity": {},
            "recentFailures": [],
            "taskFeedbackSummary": {},
        })

    for path_name in ("current.json", "runtime.json", "blueprint-index.json", "request-summary.json", "lineage-index.json"):
        path = state_dir / path_name
        if not path.exists():
            write_json(path, {
                "schemaVersion": SCHEMA_VERSION,
                "generator": generator,
                "generatedAt": timestamp,
            })

    for snapshot_name in ("audit-requests.json", "replan-requests.json", "stop-requests.json"):
        snapshot_path = harness / snapshot_name
        if not snapshot_path.exists():
            write_json(snapshot_path, {
                "schemaVersion": SCHEMA_VERSION,
                "generator": generator,
                "generatedAt": timestamp,
                "requests": [],
            })

    return {
        "root": root,
        "harness": harness,
        "state_dir": state_dir,
        "requests_dir": requests_dir,
        "queue_path": queue_path,
        "archive_dir": archive_dir,
        "request_index_path": request_index_path,
        "request_task_map_path": request_task_map_path,
        "project_meta_path": project_meta_path,
        "feedback_log_path": feedback_log_path,
        "feedback_summary_path": feedback_summary_path,
        "lineage_path": lineage_path,
        "lineage_index_path": state_dir / "lineage-index.json",
        "request_summary_path": state_dir / "request-summary.json",
        "session_registry_path": session_registry_path,
        "runner_state_path": runner_state_path,
        "runner_heartbeats_path": runner_heartbeats_path,
        "verification_dir": verification_dir,
    }


def build_request_id(seq: int) -> str:
    return f"R-{seq:04d}"


def build_binding_id(seq: int) -> str:
    return f"RB-{seq:04d}"


def normalize_context_paths(root: Path, values: list[str]) -> list[str]:
    result = []
    for value in values:
        path = Path(value)
        if not path.is_absolute():
            path = (root / value).resolve()
        else:
            path = path.resolve()
        if not path.exists():
            raise FileNotFoundError(f"context path not found: {value}")
        result.append(str(path))
    return result


def find_request(requests: list[dict], request_id: str):
    for request in requests:
        if request.get("requestId") == request_id:
            return request
    raise KeyError(f"request not found: {request_id}")


def find_task(tasks: list[dict], task_id: str):
    for task in tasks:
        if task.get("taskId") == task_id:
            return task
    raise KeyError(f"task not found: {task_id}")


def find_binding(bindings: list[dict], request_id: str, task_id: str):
    for binding in bindings:
        if binding.get("requestId") == request_id and binding.get("taskId") == task_id:
            return binding
    return None


def upsert_request_record(index: dict, request: dict):
    requests = index.setdefault("requests", [])
    for idx, existing in enumerate(requests):
        if existing.get("requestId") == request.get("requestId"):
            requests[idx] = request
            return request
    requests.append(request)
    return request


def read_task_pool(harness: Path):
    task_pool_path = harness / "task-pool.json"
    if not task_pool_path.exists():
        return None
    return load_json(task_pool_path)


def request_goal_tokens(text: str):
    return {
        token
        for token in re.findall(r"[a-zA-Z0-9_-]+", (text or "").lower())
        if len(token) >= 3
    }


def task_kind_matches_request(request_kind: str, task: dict):
    kind = (request_kind or "").lower()
    task_kind = (task.get("kind") or "").lower()
    role = (task.get("roleHint") or "").lower()
    worker_mode = (task.get("workerMode") or "").lower()
    if kind == "stop":
        return task.get("status") in TASK_ACTIVE_STATUSES
    if kind == "audit":
        return task_kind == "audit" or worker_mode == "audit"
    if kind in {"replan", "bootstrap"}:
        return role == "orchestrator" or task_kind in {"replan", "orchestration"}
    if kind == "status":
        return True
    if kind in {"analysis", "research"}:
        return role == "orchestrator" or task_kind in {"analysis", "research", "audit"}
    if kind in {"implementation", "change"}:
        return role in {"worker", "orchestrator"} and task_kind != "audit"
    return True


def task_status_score(task: dict):
    status = task.get("status")
    if status == "queued":
        return 30
    if status in {"claimed", "active", "in_progress", "dispatched", "running", "resumed"}:
        return 20
    if status in {"verified", "completed", "done", "pass"}:
        return -50
    return 0


def task_priority_score(task: dict):
    priority = task.get("priority") or "P9"
    match = re.match(r"P(\d+)", priority)
    if not match:
        return 0
    return max(0, 20 - int(match.group(1)) * 3)


def candidate_tasks_for_request(request: dict, tasks: list[dict]):
    goal_tokens = request_goal_tokens(request.get("goal"))
    scored = []
    for task in tasks:
        score = task_status_score(task)
        if score < 0:
            continue
        if task_kind_matches_request(request.get("kind"), task):
            score += 50
        haystack = " ".join([
            task.get("taskId") or "",
            task.get("title") or "",
            task.get("summary") or "",
            task.get("kind") or "",
            task.get("roleHint") or "",
        ]).lower()
        token_hits = sum(1 for token in goal_tokens if token in haystack)
        score += token_hits * 8
        score += task_priority_score(task)
        if task.get("taskId") and task.get("taskId").lower() in (request.get("goal") or "").lower():
            score += 25
        scored.append((score, task))

    scored.sort(key=lambda item: (-item[0], item[1].get("taskId") or ""))
    if (request.get("kind") or "").lower() == "stop":
        return [task for score, task in scored if score > 0][:3]
    return [task for score, task in scored if score > 0][:1]


def lineage_event(
    root: Path,
    kind: str,
    generator: str,
    *,
    request_id: str | None = None,
    task_id: str | None = None,
    session_id: str | None = None,
    worktree_path: str | None = None,
    diff_summary: str | None = None,
    verification: dict | None = None,
    outcome: dict | None = None,
    actor: dict | None = None,
    detail: str | None = None,
    reason: str | None = None,
    context: dict | None = None,
):
    files = ensure_runtime_scaffold(root, generator=generator)
    existing = load_jsonl(files["lineage_path"])
    seq = int(existing[-1].get("seq", 0)) + 1 if existing else 1
    event = {
        "seq": seq,
        "timestamp": now_iso(),
        "generator": generator,
        "kind": kind,
        "requestId": request_id,
        "taskId": task_id,
        "sessionId": session_id,
        "worktreePath": worktree_path,
        "diffSummary": diff_summary,
        "verification": verification,
        "outcome": outcome,
        "actor": actor or {"role": "runtime", "id": generator},
        "detail": detail,
        "reason": reason,
        "context": context or {},
    }
    append_jsonl(files["lineage_path"], event)
    return event


def sync_request_from_bindings(request: dict, bindings: list[dict]):
    request["boundTaskIds"] = [binding.get("taskId") for binding in bindings]
    request["bindingIds"] = [binding.get("bindingId") for binding in bindings]
    request["activeTaskId"] = bindings[-1].get("taskId") if bindings else None
    request["sessionIds"] = [
        binding.get("lineage", {}).get("sessionId")
        for binding in bindings
        if binding.get("lineage", {}).get("sessionId")
    ]
    if bindings:
        latest = bindings[-1]
        request["worktreePath"] = latest.get("lineage", {}).get("worktreePath")
        request["diffSummary"] = latest.get("lineage", {}).get("diffSummary")
        request["verificationStatus"] = latest.get("lineage", {}).get("verificationStatus")
        request["verificationResultPath"] = latest.get("lineage", {}).get("verificationResultPath")
        request["outcome"] = latest.get("lineage", {}).get("outcome")
        request["lastBindingStatus"] = latest.get("status")
        request["statusReason"] = latest.get("statusReason")
        request["updatedAt"] = latest.get("updatedAt")
    return request


def update_request_status(index: dict, request_id: str, status: str, *, reason: str | None = None, extra: dict | None = None):
    request = find_request(index.get("requests", []), request_id)
    request["status"] = status
    request["updatedAt"] = now_iso()
    if reason:
        request["statusReason"] = reason
    if extra:
        request.update({key: value for key, value in extra.items() if value is not None})
    return request


def ensure_binding(
    root: Path,
    request_id: str,
    task: dict,
    *,
    status: str = "bound",
    reason: str,
    generator: str,
):
    files = ensure_runtime_scaffold(root, generator=generator)
    index = load_json(files["request_index_path"])
    task_map = load_json(files["request_task_map_path"])
    binding = find_binding(task_map.get("bindings", []), request_id, task.get("taskId"))
    timestamp = now_iso()

    if binding is None:
        binding_id = build_binding_id(int(task_map.get("nextBindingSeq", 1)))
        binding = {
            "bindingId": binding_id,
            "requestId": request_id,
            "taskId": task.get("taskId"),
            "workItemId": task.get("workItemId"),
            "taskTitle": task.get("title"),
            "taskKind": task.get("kind"),
            "status": status,
            "statusReason": reason,
            "bindingReason": reason,
            "createdAt": timestamp,
            "updatedAt": timestamp,
            "lineage": {
                "sessionId": task.get("claim", {}).get("boundSessionId") or task.get("preferredResumeSessionId"),
                "worktreePath": task.get("worktreePath"),
                "diffBase": task.get("diffBase") or task.get("dispatch", {}).get("diffBase"),
                "diffSummary": task.get("diffSummary"),
                "verificationStatus": task.get("verificationStatus"),
                "verificationSummary": task.get("verificationSummary"),
                "verificationResultPath": task.get("verificationResultPath"),
                "outcome": task.get("outcome"),
            },
            "route": None,
            "history": [{
                "status": status,
                "reason": reason,
                "timestamp": timestamp,
                "generator": generator,
            }],
        }
        task_map.setdefault("bindings", []).append(binding)
        task_map["nextBindingSeq"] = int(task_map.get("nextBindingSeq", 1)) + 1
    else:
        binding["updatedAt"] = timestamp
        binding["status"] = status
        binding["statusReason"] = reason
        binding.setdefault("history", []).append({
            "status": status,
            "reason": reason,
            "timestamp": timestamp,
            "generator": generator,
        })

    task_map["generatedAt"] = timestamp
    task_map["generator"] = generator
    request = update_request_status(index, request_id, status, reason=reason)
    sync_request_from_bindings(
        request,
        [item for item in task_map.get("bindings", []) if item.get("requestId") == request_id],
    )
    index["generatedAt"] = timestamp
    index["generator"] = generator
    write_json(files["request_task_map_path"], task_map)
    write_json(files["request_index_path"], index)

    lineage_event(
        root,
        f"request.{status}",
        generator,
        request_id=request_id,
        task_id=task.get("taskId"),
        session_id=binding.get("lineage", {}).get("sessionId"),
        worktree_path=binding.get("lineage", {}).get("worktreePath"),
        diff_summary=binding.get("lineage", {}).get("diffSummary"),
        outcome=binding.get("lineage", {}).get("outcome"),
        detail=reason,
    )
    return binding


def update_binding_state(
    root: Path,
    request_id: str,
    task_id: str,
    status: str,
    *,
    reason: str,
    generator: str,
    session_id: str | None = None,
    route: dict | None = None,
    verification: dict | None = None,
    outcome: dict | None = None,
    worktree_path: str | None = None,
    diff_summary: str | None = None,
):
    files = ensure_runtime_scaffold(root, generator=generator)
    index = load_json(files["request_index_path"])
    task_map = load_json(files["request_task_map_path"])
    task_pool = read_task_pool(files["harness"])
    binding = find_binding(task_map.get("bindings", []), request_id, task_id)
    if binding is None:
        if task_pool is None:
            raise KeyError(f"binding not found: {request_id} -> {task_id}")
        task = find_task(task_pool.get("tasks", []), task_id)
        binding = ensure_binding(root, request_id, task, status="bound", reason="late binding recovery", generator=generator)
        task_map = load_json(files["request_task_map_path"])
        binding = find_binding(task_map.get("bindings", []), request_id, task_id)

    timestamp = now_iso()
    binding["status"] = status
    binding["statusReason"] = reason
    binding["updatedAt"] = timestamp
    binding.setdefault("history", []).append({
        "status": status,
        "reason": reason,
        "timestamp": timestamp,
        "generator": generator,
    })
    lineage = binding.setdefault("lineage", {})
    if session_id is not None:
        lineage["sessionId"] = session_id
    if worktree_path is not None:
        lineage["worktreePath"] = worktree_path
    if diff_summary is not None:
        lineage["diffSummary"] = diff_summary
    if verification:
        lineage["verificationStatus"] = verification.get("overallStatus")
        lineage["verificationSummary"] = verification.get("summary")
        lineage["verificationResultPath"] = verification.get("verificationResultPath")
    if outcome:
        lineage["outcome"] = outcome
    if route:
        binding["route"] = route

    request = update_request_status(index, request_id, status, reason=reason, extra={
        "activeTaskId": task_id,
        "sessionId": lineage.get("sessionId"),
        "worktreePath": lineage.get("worktreePath"),
        "diffSummary": lineage.get("diffSummary"),
        "verificationStatus": lineage.get("verificationStatus"),
        "verificationResultPath": lineage.get("verificationResultPath"),
        "outcome": lineage.get("outcome"),
    })
    sync_request_from_bindings(
        request,
        [item for item in task_map.get("bindings", []) if item.get("requestId") == request_id],
    )
    index["generatedAt"] = timestamp
    index["generator"] = generator
    task_map["generatedAt"] = timestamp
    task_map["generator"] = generator
    write_json(files["request_index_path"], index)
    write_json(files["request_task_map_path"], task_map)

    lineage_event(
        root,
        f"request.{status}",
        generator,
        request_id=request_id,
        task_id=task_id,
        session_id=lineage.get("sessionId"),
        worktree_path=lineage.get("worktreePath"),
        diff_summary=lineage.get("diffSummary"),
        verification=verification,
        outcome=outcome,
        detail=reason,
        context={"route": route} if route else {},
    )
    return binding


def request_bindings_for_task(task_map: dict, task_id: str):
    return [binding for binding in task_map.get("bindings", []) if binding.get("taskId") == task_id]


def request_bindings_for_request(task_map: dict, request_id: str):
    return [binding for binding in task_map.get("bindings", []) if binding.get("requestId") == request_id]


def maybe_complete_request(root: Path, request_id: str, *, generator: str):
    files = ensure_runtime_scaffold(root, generator=generator)
    index = load_json(files["request_index_path"])
    task_map = load_json(files["request_task_map_path"])
    bindings = request_bindings_for_request(task_map, request_id)
    if not bindings:
        return False
    statuses = {binding.get("status") for binding in bindings}
    if statuses == {"completed"}:
        return False
    if statuses and statuses <= {"verified", "completed"}:
        latest = bindings[-1]
        update_binding_state(
            root,
            request_id,
            latest.get("taskId"),
            "completed",
            reason="all bound tasks verified; request loop closed",
            generator=generator,
            session_id=latest.get("lineage", {}).get("sessionId"),
            worktree_path=latest.get("lineage", {}).get("worktreePath"),
            diff_summary=latest.get("lineage", {}).get("diffSummary"),
            verification={
                "overallStatus": latest.get("lineage", {}).get("verificationStatus"),
                "summary": latest.get("lineage", {}).get("verificationSummary"),
                "verificationResultPath": latest.get("lineage", {}).get("verificationResultPath"),
            },
            outcome={"status": "completed"},
        )
        return True
    return False


def record_route_decision(root: Path, task_id: str, decision: dict, *, generator: str):
    files = ensure_runtime_scaffold(root, generator=generator)
    session_registry = load_json(files["session_registry_path"])
    decisions = [item for item in session_registry.get("routingDecisions", []) if item.get("taskId") != task_id]
    decisions.append({**decision, "routedAt": now_iso()})
    session_registry["routingDecisions"] = decisions[-50:]
    session_registry["generatedAt"] = now_iso()
    session_registry["generator"] = generator
    write_json(files["session_registry_path"], session_registry)

    task_map = load_json(files["request_task_map_path"])
    for binding in request_bindings_for_task(task_map, task_id):
        update_binding_state(
            root,
            binding.get("requestId"),
            task_id,
            "blocked" if decision.get("gateStatus") == "blocked" else binding.get("status"),
            reason=decision.get("gateReason") or "route decision recorded",
            generator=generator,
            session_id=decision.get("preferredResumeSessionId"),
            route=decision,
        )

    lineage_event(
        root,
        "route.decided",
        generator,
        task_id=task_id,
        session_id=decision.get("preferredResumeSessionId"),
        worktree_path=decision.get("worktreePath"),
        detail=decision.get("gateReason"),
        context={"decision": decision},
    )


def update_session_binding(
    root: Path,
    *,
    task_id: str,
    session_id: str | None,
    node_id: str | None,
    status: str,
    bound_from_task_id: str | None = None,
    error: str | None = None,
    generator: str,
):
    files = ensure_runtime_scaffold(root, generator=generator)
    registry = load_json(files["session_registry_path"])
    active = [
        item for item in registry.get("activeBindings", [])
        if item.get("taskId") != task_id and item.get("sessionId") != session_id
    ]
    recoverable = [item for item in registry.get("recoverableBindings", []) if item.get("taskId") != task_id]
    timestamp = now_iso()

    if status in {"dispatched", "running", "resumed"} and session_id:
        active.append({
            "taskId": task_id,
            "sessionId": session_id,
            "nodeId": node_id,
            "boundFromTaskId": bound_from_task_id,
            "updatedAt": timestamp,
        })
    elif status == "recoverable" and session_id:
        recoverable.append({
            "taskId": task_id,
            "sessionId": session_id,
            "lastKnownSessionId": session_id,
            "error": error,
            "recordedAt": timestamp,
        })
    elif status == "completed" and session_id:
        registry.setdefault("lastCompletedByTask", {})[task_id] = session_id

    registry["activeBindings"] = active
    registry["recoverableBindings"] = recoverable
    registry["generatedAt"] = timestamp
    registry["generator"] = generator
    write_json(files["session_registry_path"], registry)
    return registry


def emit_follow_up_request(
    root: Path,
    *,
    kind: str,
    goal: str,
    source: str,
    generator: str,
    parent_request_id: str | None = None,
    origin_task_id: str | None = None,
    origin_session_id: str | None = None,
    reason: str | None = None,
    dedupe_key: str | None = None,
):
    files = ensure_runtime_scaffold(root, generator=generator)
    index = load_json(files["request_index_path"])
    existing = next(
        (
            request for request in index.get("requests", [])
            if request.get("dedupeKey") == dedupe_key and request.get("status") not in REQUEST_TERMINAL_STATUSES
        ),
        None,
    )
    if existing is not None:
        return existing

    seq = int(index.get("nextSeq", 1))
    request_id = build_request_id(seq)
    request = {
        "requestId": request_id,
        "seq": seq,
        "source": source,
        "kind": kind,
        "goal": goal,
        "projectRoot": str(root.resolve()),
        "contextPaths": [],
        "threadKey": None,
        "priority": "P0" if kind in {"replan", "stop"} else "P1",
        "scope": "project",
        "mergePolicy": "append",
        "replyPolicy": "summary",
        "status": "queued",
        "createdAt": now_iso(),
        "parentRequestId": parent_request_id,
        "originTaskId": origin_task_id,
        "originSessionId": origin_session_id,
        "statusReason": reason,
        "dedupeKey": dedupe_key,
    }
    append_jsonl(files["queue_path"], request)
    index["nextSeq"] = seq + 1
    index["generatedAt"] = now_iso()
    index["generator"] = generator
    index.setdefault("requests", []).append(request)
    write_json(files["request_index_path"], index)

    if kind in {"audit", "replan", "stop"}:
        snapshot_path = files["harness"] / f"{kind}-requests.json"
        snapshot = load_optional_json(snapshot_path, {
            "schemaVersion": SCHEMA_VERSION,
            "generator": generator,
            "generatedAt": now_iso(),
            "requests": [],
        })
        snapshot.setdefault("requests", []).append({
            "requestId": request_id,
            "goal": goal,
            "source": source,
            "originTaskId": origin_task_id,
            "originSessionId": origin_session_id,
            "createdAt": request["createdAt"],
        })
        snapshot["generatedAt"] = now_iso()
        snapshot["generator"] = generator
        write_json(snapshot_path, snapshot)

    lineage_event(
        root,
        "request.follow_up_emitted",
        generator,
        request_id=request_id,
        task_id=origin_task_id,
        session_id=origin_session_id,
        detail=goal,
        reason=reason,
        context={
            "parentRequestId": parent_request_id,
            "kind": kind,
        },
    )
    return request


def reconcile_requests(root: Path, *, generator: str = "harness-reconcile"):
    files = ensure_runtime_scaffold(root, generator=generator)
    index = load_json(files["request_index_path"])
    task_map = load_json(files["request_task_map_path"])
    task_pool = read_task_pool(files["harness"])
    tasks = task_pool.get("tasks", []) if task_pool else []
    bound = []
    blocked = []

    for request in index.get("requests", []):
        if request.get("status") in REQUEST_TERMINAL_STATUSES:
            continue
        if request_bindings_for_request(task_map, request.get("requestId")):
            continue
        candidates = candidate_tasks_for_request(request, tasks)
        if not candidates:
            update_request_status(
                index,
                request.get("requestId"),
                "blocked",
                reason="no compatible task available for current runtime state",
            )
            blocked.append(request.get("requestId"))
            lineage_event(
                root,
                "request.blocked",
                generator,
                request_id=request.get("requestId"),
                detail="no compatible task available for current runtime state",
            )
            continue
        for task in candidates:
            ensure_binding(
                root,
                request.get("requestId"),
                task,
                status="bound",
                reason=f"bound by reconcile to {task.get('taskId')}",
                generator=generator,
            )
            bound.append({
                "requestId": request.get("requestId"),
                "taskId": task.get("taskId"),
            })

    index = load_json(files["request_index_path"])
    index["generatedAt"] = now_iso()
    index["generator"] = generator
    write_json(files["request_index_path"], index)
    return {
        "bound": bound,
        "blocked": blocked,
        "requestCount": len(index.get("requests", [])),
    }


def build_feedback_summary(entries):
    ordered = sorted(entries, key=lambda item: (item.get("timestamp") or "", item.get("id") or ""))
    errors = [entry for entry in ordered if entry.get("severity") in {"error", "critical"}]
    illegal_actions = [entry for entry in ordered if entry.get("feedbackType") == "illegal_action"]
    task_summary = {}

    def trim(entry):
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

    for entry in ordered:
        task_id = entry.get("taskId")
        if not task_id:
            continue
        summary = task_summary.setdefault(task_id, {
            "taskId": task_id,
            "feedbackCount": 0,
            "errorCount": 0,
            "criticalCount": 0,
            "latestFeedbackType": None,
            "latestSeverity": None,
            "latestMessage": None,
            "latestTimestamp": None,
            "recentFailures": [],
        })
        summary["feedbackCount"] += 1
        if entry.get("severity") in {"error", "critical"}:
            summary["errorCount"] += 1
        if entry.get("severity") == "critical":
            summary["criticalCount"] += 1
        summary["latestFeedbackType"] = entry.get("feedbackType")
        summary["latestSeverity"] = entry.get("severity")
        summary["latestMessage"] = entry.get("message")
        summary["latestTimestamp"] = entry.get("timestamp")

    for task_id, summary in task_summary.items():
        task_entries = [
            trim(entry)
            for entry in ordered
            if entry.get("taskId") == task_id and entry.get("severity") in {"error", "critical"}
        ]
        summary["recentFailures"] = task_entries[-3:]

    return {
        "schemaVersion": SCHEMA_VERSION,
        "generator": "harness-architect",
        "generatedAt": now_iso(),
        "feedbackLogPath": ".harness/feedback-log.jsonl",
        "feedbackEventCount": len(ordered),
        "errorCount": len(errors),
        "criticalCount": sum(1 for entry in ordered if entry.get("severity") == "critical"),
        "illegalActionCount": len(illegal_actions),
        "tasksWithRecentFailures": sorted(task_id for task_id, summary in task_summary.items() if summary["recentFailures"]),
        "byType": dict(Counter(entry.get("feedbackType", "unknown") for entry in ordered)),
        "bySeverity": dict(Counter(entry.get("severity", "unknown") for entry in ordered)),
        "recentFailures": [trim(entry) for entry in errors[-5:]],
        "taskFeedbackSummary": task_summary,
    }


def build_request_summary(index: dict, task_map: dict, task_pool: dict | None = None):
    tasks_by_id = {
        task.get("taskId"): task
        for task in (task_pool or {}).get("tasks", [])
        if task.get("taskId")
    }
    requests = index.get("requests", [])
    counts = dict(Counter(request.get("status", "unknown") for request in requests))
    active = [request for request in requests if request.get("status") not in REQUEST_TERMINAL_STATUSES]
    active.sort(
        key=lambda request: (
            {
                "running": 0,
                "resumed": 1,
                "recoverable": 2,
                "dispatched": 3,
                "bound": 4,
                "queued": 5,
                "blocked": 6,
                "verified": 7,
            }.get(request.get("status"), 9),
            request.get("seq", 0),
        )
    )
    active_request = active[0] if active else None
    bindings = task_map.get("bindings", [])
    binding_summary = []
    for binding in bindings[-20:]:
        task = tasks_by_id.get(binding.get("taskId"), {})
        binding_summary.append({
            "bindingId": binding.get("bindingId"),
            "requestId": binding.get("requestId"),
            "requestStatus": next(
                (
                    request.get("status")
                    for request in requests
                    if request.get("requestId") == binding.get("requestId")
                ),
                None,
            ),
            "taskId": binding.get("taskId"),
            "taskTitle": binding.get("taskTitle") or task.get("title"),
            "bindingStatus": binding.get("status"),
            "branchName": task.get("branchName"),
            "worktreePath": binding.get("lineage", {}).get("worktreePath") or task.get("worktreePath"),
            "sessionId": binding.get("lineage", {}).get("sessionId"),
            "diffSummary": binding.get("lineage", {}).get("diffSummary") or task.get("diffSummary"),
            "verificationStatus": binding.get("lineage", {}).get("verificationStatus") or task.get("verificationStatus"),
            "verificationResultPath": binding.get("lineage", {}).get("verificationResultPath") or task.get("verificationResultPath"),
            "updatedAt": binding.get("updatedAt"),
        })

    return {
        "schemaVersion": SCHEMA_VERSION,
        "generator": "harness-architect",
        "generatedAt": now_iso(),
        "requestCounts": counts,
        "totalRequests": len(requests),
        "activeRequest": active_request,
        "recentRequests": requests[-5:],
        "bindings": binding_summary,
        "boundRequestCount": counts.get("bound", 0),
        "runningRequestCount": counts.get("running", 0),
        "recoverableRequestCount": counts.get("recoverable", 0),
        "completedRequestCount": counts.get("completed", 0),
        "blockedRequestCount": counts.get("blocked", 0),
    }


def build_lineage_index(lineage_entries: list[dict], task_pool: dict | None, task_map: dict):
    tasks_by_id = {
        task.get("taskId"): task
        for task in (task_pool or {}).get("tasks", [])
        if task.get("taskId")
    }
    by_request = {}
    by_task = {}
    latest_by_request = {}
    latest_by_task = {}
    latest_by_session = {}
    for event in lineage_entries:
        request_id = event.get("requestId")
        task_id = event.get("taskId")
        session_id = event.get("sessionId")
        if request_id:
            summary = by_request.setdefault(request_id, {
                "requestId": request_id,
                "taskIds": [],
                "sessionIds": [],
                "worktreePaths": [],
                "verificationStatuses": [],
                "outcomes": [],
                "lastEventKind": None,
                "lastEventAt": None,
            })
            if task_id and task_id not in summary["taskIds"]:
                summary["taskIds"].append(task_id)
            if event.get("sessionId") and event.get("sessionId") not in summary["sessionIds"]:
                summary["sessionIds"].append(event.get("sessionId"))
            if event.get("worktreePath") and event.get("worktreePath") not in summary["worktreePaths"]:
                summary["worktreePaths"].append(event.get("worktreePath"))
            verification_status = (event.get("verification") or {}).get("overallStatus")
            if verification_status and verification_status not in summary["verificationStatuses"]:
                summary["verificationStatuses"].append(verification_status)
            outcome_status = (event.get("outcome") or {}).get("status")
            if outcome_status and outcome_status not in summary["outcomes"]:
                summary["outcomes"].append(outcome_status)
            summary["lastEventKind"] = event.get("kind")
            summary["lastEventAt"] = event.get("timestamp")
            latest_by_request[request_id] = {
                "kind": event.get("kind"),
                "timestamp": event.get("timestamp"),
                "taskId": task_id,
                "sessionId": session_id,
            }
        if task_id:
            task = tasks_by_id.get(task_id, {})
            summary = by_task.setdefault(task_id, {
                "taskId": task_id,
                "requestIds": [],
                "sessionIds": [],
                "worktreePath": task.get("worktreePath"),
                "diffSummary": task.get("diffSummary"),
                "verificationStatus": task.get("verificationStatus"),
                "outcome": task.get("outcome"),
                "lastEventKind": None,
                "lastEventAt": None,
            })
            if request_id and request_id not in summary["requestIds"]:
                summary["requestIds"].append(request_id)
            if event.get("sessionId") and event.get("sessionId") not in summary["sessionIds"]:
                summary["sessionIds"].append(event.get("sessionId"))
            if event.get("worktreePath"):
                summary["worktreePath"] = event.get("worktreePath")
            if event.get("diffSummary"):
                summary["diffSummary"] = event.get("diffSummary")
            if (event.get("verification") or {}).get("overallStatus"):
                summary["verificationStatus"] = event.get("verification", {}).get("overallStatus")
            if event.get("outcome"):
                summary["outcome"] = event.get("outcome")
            summary["lastEventKind"] = event.get("kind")
            summary["lastEventAt"] = event.get("timestamp")
            latest_by_task[task_id] = {
                "kind": event.get("kind"),
                "timestamp": event.get("timestamp"),
                "requestId": request_id,
                "sessionId": session_id,
            }
        if session_id:
            latest_by_session[session_id] = {
                "kind": event.get("kind"),
                "timestamp": event.get("timestamp"),
                "requestId": request_id,
                "taskId": task_id,
                "worktreePath": event.get("worktreePath"),
            }

    for binding in task_map.get("bindings", []):
        task_id = binding.get("taskId")
        request_id = binding.get("requestId")
        if not task_id or not request_id:
            continue
        request_summary = by_request.setdefault(request_id, {
            "requestId": request_id,
            "taskIds": [],
            "sessionIds": [],
            "worktreePaths": [],
            "verificationStatuses": [],
            "outcomes": [],
            "lastEventKind": None,
            "lastEventAt": None,
        })
        if task_id not in request_summary["taskIds"]:
            request_summary["taskIds"].append(task_id)

    active_bindings = []
    for binding in task_map.get("bindings", []):
        if binding.get("status") in REQUEST_TERMINAL_STATUSES:
            continue
        active_bindings.append(
            {
                "bindingId": binding.get("bindingId"),
                "requestId": binding.get("requestId"),
                "taskId": binding.get("taskId"),
                "status": binding.get("status"),
                "sessionId": binding.get("lineage", {}).get("sessionId"),
                "worktreePath": binding.get("lineage", {}).get("worktreePath"),
                "diffSummary": binding.get("lineage", {}).get("diffSummary"),
                "verificationStatus": binding.get("lineage", {}).get("verificationStatus"),
                "outcome": binding.get("lineage", {}).get("outcome"),
            }
        )

    return {
        "schemaVersion": SCHEMA_VERSION,
        "generator": "harness-architect",
        "generatedAt": now_iso(),
        "eventCount": len(lineage_entries),
        "lastSeq": lineage_entries[-1].get("seq") if lineage_entries else 0,
        "recentEvents": lineage_entries[-10:],
        "activeBindings": active_bindings,
        "latestByRequest": latest_by_request,
        "latestByTask": latest_by_task,
        "latestBySession": latest_by_session,
        "requests": by_request,
        "tasks": by_task,
    }
