#!/usr/bin/env python3
import argparse
import json
import sys
from pathlib import Path

from runtime_common import (
    SEVERE_ROUTE_FAILURES,
    TASK_ACTIVE_STATUSES,
    ensure_runtime_scaffold,
    find_task,
    load_json,
    load_optional_json,
    record_route_decision,
)


COMPLETED_STATUSES = {"completed", "validated", "done", "pass", "verified"}
CONTROL_PLANE_KINDS = {"orchestration", "replan", "rollback", "lease-recovery"}


def active_tasks(tasks):
    return [task for task in tasks if task.get("status") in TASK_ACTIVE_STATUSES]


def completed_task_ids(tasks):
    return {
        task.get("taskId")
        for task in tasks
        if task.get("status") in COMPLETED_STATUSES
    }


def normalize_glob(path: str) -> str:
    return (path or "").rstrip("/")


def path_overlap(left: str, right: str) -> bool:
    left = normalize_glob(left)
    right = normalize_glob(right)
    if not left or not right:
        return False
    if left == right:
        return True
    if left.endswith("/**"):
        return right.startswith(left[:-3])
    if right.endswith("/**"):
        return left.startswith(right[:-3])
    return False


def owned_paths_overlap(task, other_task) -> bool:
    for left in task.get("ownedPaths", []):
        for right in other_task.get("ownedPaths", []):
            if path_overlap(left, right):
                return True
    return False


def recent_failures_for_task(feedback_summary, task_id: str):
    if not feedback_summary or not task_id:
        return []
    return (
        feedback_summary.get("taskFeedbackSummary", {})
        .get(task_id, {})
        .get("recentFailures", [])
    )


def session_in_use_by_other(session_id: str, task_id: str, tasks, session_registry) -> bool:
    if not session_id:
        return False
    for task in tasks:
        if task.get("taskId") == task_id:
            continue
        if task.get("status") in TASK_ACTIVE_STATUSES and task.get("claim", {}).get("boundSessionId") == session_id:
            return True
    for binding in session_registry.get("activeBindings", []):
        if binding.get("taskId") != task_id and binding.get("sessionId") == session_id:
            return True
    return False


def dedupe(values):
    result = []
    seen = set()
    for value in values:
        if value and value not in seen:
            seen.add(value)
            result.append(value)
    return result


def task_requires_explicit_workspace(task: dict):
    if task.get("kind") in CONTROL_PLANE_KINDS or task.get("roleHint") == "orchestrator":
        return False
    return task.get("kind") != "audit" and task.get("workerMode") != "audit"


def route_task(task, tasks, session_registry, feedback_summary):
    orchestration_session_id = session_registry.get("orchestrationSessionId")
    reasons = []
    needs_orchestrator = False
    claimable = True
    prompt_stages = ["audit"] if (task.get("kind") == "audit" or task.get("workerMode") == "audit") else ["start", "execute", "recover"]
    recent_failures = recent_failures_for_task(feedback_summary, task.get("taskId"))
    severe_recent = [
        failure for failure in recent_failures
        if failure.get("feedbackType") in SEVERE_ROUTE_FAILURES
    ]

    if not orchestration_session_id:
        needs_orchestrator = True
        reasons.append("missing orchestrationSessionId in session-registry.json")

    status = task.get("status")
    role_hint = task.get("roleHint")
    planning_stage = task.get("planningStage")
    claim = task.get("claim", {})
    active = active_tasks(tasks)
    completed_ids = completed_task_ids(tasks)

    if role_hint == "worker" and status == "queued" and planning_stage != "execution-ready":
        claimable = False
        reasons.append(f"planningStage={planning_stage} is not execution-ready")

    unmet_deps = [
        dep for dep in task.get("dependsOn", [])
        if dep not in completed_ids and dep != task.get("taskId")
    ]
    if unmet_deps and status == "queued":
        claimable = False
        reasons.append(f"dependencies not completed: {unmet_deps}")

    if claim.get("agentId") and status in TASK_ACTIVE_STATUSES:
        reasons.append(f"task already claimed by {claim.get('agentId')}")

    conflicting_tasks = [
        other.get("taskId")
        for other in active
        if other.get("taskId") != task.get("taskId")
        and other.get("roleHint") == "worker"
        and owned_paths_overlap(task, other)
    ]
    if conflicting_tasks:
        claimable = False
        reasons.append(f"ownedPaths overlap with active tasks: {conflicting_tasks}")

    if severe_recent:
        claimable = False
        needs_orchestrator = True
        reasons.append(
            "recent failures require review: "
            + ", ".join(failure.get("feedbackType", "unknown") for failure in severe_recent)
        )

    if task_requires_explicit_workspace(task):
        if not task.get("worktreePath"):
            claimable = False
            reasons.append("worker task missing worktreePath")
        if not (task.get("diffBase") or task.get("dispatch", {}).get("diffBase")):
            claimable = False
            reasons.append("worker task missing diffBase")
        if not task.get("ownedPaths"):
            claimable = False
            reasons.append("worker task missing ownedPaths")

    candidate_sessions = dedupe([
        claim.get("boundSessionId"),
        task.get("preferredResumeSessionId"),
        *task.get("candidateResumeSessionIds", []),
        task.get("lastKnownSessionId"),
    ])
    safe_candidates = [
        session_id
        for session_id in candidate_sessions
        if not session_in_use_by_other(session_id, task.get("taskId"), tasks, session_registry)
    ]
    contested_candidates = [session_id for session_id in candidate_sessions if session_id not in safe_candidates]

    resume_strategy = "fresh"
    preferred_resume_session_id = None
    if task.get("resumeStrategy") == "resume":
        preferred = task.get("preferredResumeSessionId")
        if contested_candidates:
            claimable = False
            needs_orchestrator = True
            reasons.append(f"resume candidates already owned elsewhere: {contested_candidates}")
        elif preferred and preferred in safe_candidates:
            resume_strategy = "resume"
            preferred_resume_session_id = preferred
            reasons.append("preferred resume session is safe to reuse")
        elif len(safe_candidates) == 1:
            resume_strategy = "resume"
            preferred_resume_session_id = safe_candidates[0]
            reasons.append("single safe resume candidate selected programmatically")
        elif len(safe_candidates) > 1:
            claimable = False
            needs_orchestrator = True
            reasons.append(f"multiple safe resume candidates require orchestration review: {safe_candidates}")
        else:
            claimable = False
            needs_orchestrator = True
            reasons.append("no safe resume candidate found for resume-only task")

    gate_status = "claimable"
    if needs_orchestrator:
        gate_status = "orchestrator_review"
    elif not claimable:
        gate_status = "blocked"

    return {
        "taskId": task["taskId"],
        "routingMode": "llm-fallback" if needs_orchestrator else "programmatic",
        "needsOrchestrator": needs_orchestrator,
        "dispatchReady": claimable and not needs_orchestrator,
        "gateStatus": gate_status,
        "gateReason": "; ".join(reasons) if reasons else "program gate passed",
        "routingModel": task.get("routingModel", "gpt-5.4"),
        "executionModel": task.get("executionModel", "gpt-5.3-codex"),
        "orchestrationSessionId": orchestration_session_id,
        "resumeStrategy": resume_strategy,
        "preferredResumeSessionId": preferred_resume_session_id,
        "candidateResumeSessionIds": safe_candidates,
        "contestedResumeSessionIds": contested_candidates,
        "sessionFamilyId": task.get("sessionFamilyId"),
        "cacheAffinityKey": task.get("cacheAffinityKey"),
        "routingReason": task.get("routingReason"),
        "promptStages": prompt_stages if claimable and not needs_orchestrator else [],
        "recentFailures": recent_failures,
        "claimBinding": {
            "boundSessionId": claim.get("boundSessionId"),
            "boundResumeStrategy": claim.get("boundResumeStrategy"),
            "boundFromTaskId": claim.get("boundFromTaskId"),
        },
        "worktreePath": task.get("worktreePath"),
        "branchName": task.get("branchName"),
        "ownedPaths": task.get("ownedPaths", []),
        "forbiddenPaths": task.get("forbiddenPaths", []),
        "targetSelector": task.get("dispatch", {}).get("targetSelector"),
        "diffBase": task.get("diffBase") or task.get("dispatch", {}).get("diffBase"),
        "commandProfile": task.get("dispatch", {}).get("commandProfile", {}),
    }


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--root", required=True, help="project root containing .harness/")
    parser.add_argument("--task-id", required=True, help="task id to route")
    parser.add_argument("--write-back", action="store_true", help="persist the routing decision into session-registry.json and lineage")
    args = parser.parse_args()

    root = Path(args.root).resolve()
    files = ensure_runtime_scaffold(root, generator="harness-route-session")
    task_pool = load_json(files["harness"] / "task-pool.json")
    session_registry = load_json(files["session_registry_path"])
    feedback_summary = load_optional_json(files["feedback_summary_path"])

    task = find_task(task_pool.get("tasks", []), args.task_id)
    decision = route_task(task, task_pool.get("tasks", []), session_registry, feedback_summary)
    if args.write_back:
        record_route_decision(root, args.task_id, decision, generator="harness-route-session")
    print(json.dumps(decision, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    try:
        main()
    except Exception as exc:
        print(f"route-session example failed: {exc}", file=sys.stderr)
        sys.exit(1)
