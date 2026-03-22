#!/usr/bin/env python3
import argparse
import json
import re
import sys
from pathlib import Path


def load_json(path: Path):
    return json.loads(path.read_text())


def load_progress(path: Path):
    text = path.read_text()
    match = re.search(r"```json\s*(\{[\s\S]*?\})\s*```", text)
    if not match:
      raise ValueError(f"missing json block in {path}")
    return json.loads(match.group(1))


def load_optional_json(path: Path):
    if path.exists():
        return load_json(path)
    return None


def count_tasks(tasks, statuses):
    return sum(1 for task in tasks if task.get("status") in statuses)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--root", required=True, help="project root containing .harness/")
    args = parser.parse_args()

    root = Path(args.root).resolve()
    harness = root / ".harness"
    state_dir = harness / "state"
    current_state = load_optional_json(state_dir / "current.json")
    runtime_state = load_optional_json(state_dir / "runtime.json")
    feedback_summary = load_optional_json(state_dir / "feedback-summary.json")

    progress = current_state or load_progress(harness / "progress.md")
    task_pool = load_json(harness / "task-pool.json")

    session_registry_path = harness / "session-registry.json"
    session_registry = load_json(session_registry_path) if session_registry_path.exists() else {}

    tasks = task_pool.get("tasks", [])
    if runtime_state:
        active_workers = runtime_state.get("activeWorkerCount", 0)
        active_audit_workers = runtime_state.get("activeAuditWorkerCount", 0)
        active_orchestrators = runtime_state.get("activeOrchestratorCount", 0)
    else:
        active_workers = sum(
            1
            for task in tasks
            if task.get("status") in {"active", "claimed", "in_progress"}
            and task.get("roleHint") == "worker"
        )
        active_audit_workers = sum(
            1
            for task in tasks
            if task.get("status") in {"active", "claimed", "in_progress"}
            and task.get("kind") == "audit"
        )
        active_orchestrators = sum(
            1
            for task in tasks
            if task.get("status") in {"active", "claimed", "in_progress"}
            and task.get("kind") in {"orchestration", "replan", "rollback", "merge", "lease-recovery"}
        )
    current_task_id = progress.get("currentTaskId")
    current_task_title = progress.get("currentTaskTitle", "-")
    current_task_summary = progress.get("currentTaskSummary", "-")
    blockers = progress.get("blockers", [])
    next_actions = progress.get("nextActions", [])

    print("== Harness Status ==")
    print(f"mode: {progress.get('mode', '-')}")
    print(f"focus: {progress.get('currentFocus', '-')}")
    print(f"role: {progress.get('currentRole', '-')}")
    print()
    print(f"current task: {current_task_id or '-'}")
    print(f"title: {current_task_title}")
    print(f"summary: {current_task_summary}")
    active_request = (runtime_state or {}).get("activeRequest", {})
    print(f"active request: {active_request.get('requestId', '-')}")
    print(f"request status: {active_request.get('status', '-')}")
    print()
    print(f"active workers: {active_workers}")
    print(f"active audit workers: {active_audit_workers}")
    print(f"active orchestrator tasks: {active_orchestrators}")
    print(f"active runners: {(runtime_state or {}).get('activeRunnerCount', 0)}")
    print(f"recoverable tasks: {(runtime_state or {}).get('recoverableTaskCount', 0)}")
    print(f"stale runners: {(runtime_state or {}).get('staleRunnerCount', 0)}")
    print(f"blocked routes: {(runtime_state or {}).get('blockedRouteCount', 0)}")
    print(f"verified tasks: {(runtime_state or {}).get('verifiedTaskCount', 0)}")
    print(f"failed verifications: {(runtime_state or {}).get('failingVerificationCount', 0)}")
    print(f"feedback errors: {(runtime_state or {}).get('feedbackErrorCount', (feedback_summary or {}).get('errorCount', 0))}")
    print(f"illegal actions: {(runtime_state or {}).get('illegalActionCount', (feedback_summary or {}).get('illegalActionCount', 0))}")
    print(f"orchestration session: {(runtime_state or {}).get('orchestrationSessionId', session_registry.get('orchestrationSessionId', '-'))}")
    print(f"pending blockers: {len(blockers)}")
    if (runtime_state or {}).get('lastTickAt'):
        print(f"last runner tick: {(runtime_state or {}).get('lastTickAt')}")
    recent_failures = (feedback_summary or {}).get("recentFailures", [])
    if recent_failures:
        latest = recent_failures[-1]
        print(f"latest failure: {latest.get('taskId', '-')} {latest.get('feedbackType', '-')} [{latest.get('severity', '-')}]")
    print()
    print("next actions:")
    if next_actions:
        for action in next_actions:
            print(f"- {action}")
    else:
        print("- none")


if __name__ == "__main__":
    try:
        main()
    except Exception as exc:
        print(f"status example failed: {exc}", file=sys.stderr)
        sys.exit(1)
