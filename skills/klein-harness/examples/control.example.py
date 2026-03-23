#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import subprocess
import sys
from pathlib import Path

from runtime_common import (
    TASK_ACTIVE_STATUSES,
    ensure_runtime_scaffold,
    emit_follow_up_request,
    find_request,
    find_task,
    lineage_event,
    load_json,
    load_optional_json,
    now_iso,
    request_bindings_for_task,
    update_request_snapshot,
    update_request_status,
    write_json,
)


RESET_FIELDS_BY_STAGE = {
    "queued": {
        "status": "queued",
        "dispatchBackend": None,
        "dispatchSessionLabel": None,
        "lastDispatchedAt": None,
        "runnerSession": None,
        "runnerStatus": None,
        "runnerExitCode": None,
        "recoverableAt": None,
        "recoveryReason": None,
        "verificationStatus": None,
        "verificationSummary": None,
        "verificationResultPath": None,
        "mergeStatus": None,
        "mergeQueuedAt": None,
        "mergeCheckedAt": None,
        "mergedAt": None,
        "mergedCommit": None,
        "conflictPaths": None,
        "checkpointRequired": False,
        "checkpointRequested": False,
        "checkpointReason": None,
    },
    "worktree_prepared": {
        "status": "worktree_prepared",
        "dispatchBackend": None,
        "dispatchSessionLabel": None,
        "lastDispatchedAt": None,
        "runnerSession": None,
        "runnerStatus": None,
        "runnerExitCode": None,
        "recoverableAt": None,
        "recoveryReason": None,
        "verificationStatus": None,
        "verificationSummary": None,
        "verificationResultPath": None,
        "mergeStatus": None,
        "mergeQueuedAt": None,
        "mergeCheckedAt": None,
        "mergedAt": None,
        "mergedCommit": None,
        "conflictPaths": None,
        "checkpointRequired": False,
        "checkpointRequested": False,
        "checkpointReason": None,
    },
    "merge_queued": {
        "status": "merge_queued",
        "mergeStatus": "merge_queued",
        "mergeCheckedAt": None,
        "mergedAt": None,
        "mergedCommit": None,
        "conflictPaths": None,
    },
}


def refresh_runtime_state(root: Path) -> None:
    refresh_script = root / ".harness" / "scripts" / "refresh-state.py"
    if refresh_script.exists():
        subprocess.run(["python3", str(refresh_script), str(root)], check=True, stdout=subprocess.DEVNULL)


def latest_request_for_task(root: Path, files: dict, task_id: str) -> dict | None:
    task_map = load_json(files["request_task_map_path"])
    index = load_json(files["request_index_path"])
    requests_by_id = {item.get("requestId"): item for item in index.get("requests", []) if item.get("requestId")}
    bindings = request_bindings_for_task(task_map, task_id)
    if not bindings:
        return None
    latest = bindings[-1]
    return requests_by_id.get(latest.get("requestId"))


def emit(fmt: str, payload: dict) -> None:
    if fmt == "json":
        print(json.dumps(payload, ensure_ascii=False, indent=2))
        return
    lines = [f"ok: {payload.get('ok', True)}"]
    if payload.get("action"):
        lines.append(f"action: {payload['action']}")
    if payload.get("taskId"):
        lines.append(f"taskId: {payload['taskId']}")
    if payload.get("requestId"):
        lines.append(f"requestId: {payload['requestId']}")
    if payload.get("status"):
        lines.append(f"status: {payload['status']}")
    if payload.get("detail"):
        lines.append(f"detail: {payload['detail']}")
    print("\n".join(lines))


def tmux_session_alive(session_name: str) -> bool:
    if not session_name or str(session_name).startswith("print:"):
        return False
    try:
        subprocess.run(["tmux", "has-session", "-t", session_name], capture_output=True, check=True)
        return True
    except (subprocess.CalledProcessError, FileNotFoundError):
        return False


def tmux_list_sessions() -> list[str]:
    try:
        result = subprocess.run(
            ["tmux", "list-sessions", "-F", "#{session_name}"],
            capture_output=True,
            text=True,
            check=True,
        )
        return [line.strip() for line in result.stdout.splitlines() if line.strip()]
    except (subprocess.CalledProcessError, FileNotFoundError):
        return []


def project_tmux_fragment(project_name: str) -> str:
    return project_name[:20].replace(" ", "-")


def session_matches_project(session_name: str, project_name: str) -> bool:
    fragment = project_tmux_fragment(project_name)
    return session_name.startswith("hr-") and f"-{fragment}-" in session_name


def list_runtime_cleanup_candidates(root: Path) -> list[Path]:
    state_dir = root / ".harness" / "state"
    candidates = []
    for pattern in ("runner-exec-*.sh", "runner-prompt-*.md"):
        candidates.extend(sorted(state_dir.glob(pattern)))
    daemon_state_path = state_dir / "runner-daemon.json"
    daemon_script_path = state_dir / "runner-daemon.sh"
    daemon_session_path = state_dir / "runner-daemon-tmux-session.txt"
    daemon_running = False
    if daemon_state_path.exists():
        try:
            daemon_running = json.loads(daemon_state_path.read_text()).get("status") == "running"
        except json.JSONDecodeError:
            daemon_running = False
    if not daemon_running:
        for path in (daemon_script_path, daemon_session_path):
            if path.exists():
                candidates.append(path)
    return candidates


def referenced_tmux_sessions(tasks: list[dict], heartbeats: dict, daemon_session: str | None, daemon_running: bool) -> set[str]:
    sessions = set()
    for task in tasks:
        task_id = task.get("taskId")
        claim = task.get("claim", {}) if isinstance(task.get("claim"), dict) else {}
        heartbeat = heartbeats.get(task_id, {}) if task_id else {}
        tmux_name = claim.get("tmuxSession") or heartbeat.get("tmuxSession")
        if tmux_name and tmux_session_alive(tmux_name):
            sessions.add(tmux_name)
    if daemon_running and daemon_session and tmux_session_alive(daemon_session):
        sessions.add(daemon_session)
    return sessions


def classify_stale_heartbeats(tasks: list[dict], heartbeats: dict) -> list[str]:
    tasks_by_id = {task.get("taskId"): task for task in tasks if task.get("taskId")}
    stale = []
    for task_id, heartbeat in heartbeats.items():
        task = tasks_by_id.get(task_id)
        status = (task or {}).get("status")
        tmux_name = heartbeat.get("tmuxSession")
        alive = tmux_session_alive(tmux_name)
        if status not in TASK_ACTIVE_STATUSES or not alive:
            stale.append(task_id)
    return stale


def cmd_project_tidy_worktrees(root: Path, files: dict, *, fmt: str, dry_run: bool) -> int:
    candidates = list_runtime_cleanup_candidates(root)
    state_dir = files["state_dir"]
    task_pool = load_json(files["harness"] / "task-pool.json")
    tasks = task_pool.get("tasks", [])
    heartbeat_payload = load_optional_json(state_dir / "runner-heartbeats.json", {})
    heartbeat_entries = heartbeat_payload.get("entries", {}) if isinstance(heartbeat_payload, dict) else {}
    daemon_state = load_optional_json(state_dir / "runner-daemon.json", {})
    daemon_running = isinstance(daemon_state, dict) and daemon_state.get("status") == "running"
    daemon_session_path = state_dir / "runner-daemon-tmux-session.txt"
    daemon_session = daemon_session_path.read_text(encoding="utf-8").strip() if daemon_session_path.exists() else None

    removed = []
    killed_sessions = []
    stale_heartbeat_task_ids = classify_stale_heartbeats(tasks, heartbeat_entries)
    preserve_sessions = referenced_tmux_sessions(tasks, heartbeat_entries, daemon_session, daemon_running)
    orphan_sessions = [
        session
        for session in tmux_list_sessions()
        if session_matches_project(session, root.name) and session not in preserve_sessions
    ]
    git_prune = subprocess.run(
        ["git", "-C", str(root), "worktree", "prune"],
        check=False,
        text=True,
        capture_output=True,
    )
    if not dry_run:
        for session in orphan_sessions:
            subprocess.run(["tmux", "kill-session", "-t", session], capture_output=True)
            if not tmux_session_alive(session):
                killed_sessions.append(session)
        for path in candidates:
            if path.is_file():
                path.unlink(missing_ok=True)
                removed.append(str(path.relative_to(root)))
            elif path.is_dir():
                removed.append(str(path.relative_to(root)))
        if stale_heartbeat_task_ids:
            cleaned_entries = {
                task_id: entry
                for task_id, entry in heartbeat_entries.items()
                if task_id not in set(stale_heartbeat_task_ids)
            }
            write_json(
                state_dir / "runner-heartbeats.json",
                {
                    "schemaVersion": "1.0",
                    "generator": "harness-control",
                    "generatedAt": now_iso(),
                    "entries": cleaned_entries,
                },
            )
        refresh_runtime_state(root)
    else:
        removed = [str(path.relative_to(root)) for path in candidates]
        killed_sessions = orphan_sessions
    payload = {
        "ok": git_prune.returncode == 0,
        "action": "tidy-worktrees",
        "status": "dry-run" if dry_run else "applied",
        "detail": (
            f"removedArtifacts={len(removed)} "
            f"killedTmuxSessions={len(killed_sessions)} "
            f"clearedHeartbeats={len(stale_heartbeat_task_ids)} "
            f"gitPruneExit={git_prune.returncode}"
        ),
        "removedArtifacts": removed,
        "killedTmuxSessions": killed_sessions,
        "clearedHeartbeatTaskIds": stale_heartbeat_task_ids,
        "gitWorktreePrune": {
            "exitCode": git_prune.returncode,
            "stdout": (git_prune.stdout or "").strip(),
            "stderr": (git_prune.stderr or "").strip(),
        },
    }
    emit(fmt, payload)
    return 0 if git_prune.returncode == 0 else 1


def cmd_task(args) -> int:
    root = Path(args.root).resolve()
    files = ensure_runtime_scaffold(root, generator="harness-control")
    task_pool = load_json(files["harness"] / "task-pool.json")
    task = find_task(task_pool.get("tasks", []), args.task_id)

    if args.action == "checkpoint":
        task["checkpointRequired"] = True
        task["checkpointRequested"] = True
        task["checkpointReason"] = args.reason or "checkpoint requested by harness-control"
        task["updatedAt"] = now_iso()
        write_json(files["harness"] / "task-pool.json", task_pool)
        lineage_event(root, "task.checkpoint_requested", "harness-control", task_id=args.task_id, detail=task["checkpointReason"])
        refresh_runtime_state(root)
        emit(args.format, {"ok": True, "action": "checkpoint", "taskId": args.task_id, "status": task.get("status"), "detail": task["checkpointReason"]})
        return 0

    if args.action == "archive":
        task["cleanupStatus"] = "archived"
        task["archivedAt"] = now_iso()
        task["updatedAt"] = now_iso()
        write_json(files["harness"] / "task-pool.json", task_pool)
        lineage_event(root, "task.archived", "harness-control", task_id=args.task_id, detail=args.reason or "archived by operator")
        refresh_runtime_state(root)
        emit(args.format, {"ok": True, "action": "archive", "taskId": args.task_id, "status": task.get("status"), "detail": task.get("cleanupStatus")})
        return 0

    if args.action == "stop":
        request = latest_request_for_task(root, files, args.task_id)
        follow_up = emit_follow_up_request(
            root,
            kind="stop",
            goal=args.goal or f"Stop task {args.task_id} safely",
            source="runtime:control",
            generator="harness-control",
            parent_request_id=(request or {}).get("requestId"),
            origin_task_id=args.task_id,
            reason=args.reason or "operator requested stop",
            dedupe_key=f"control-stop:{args.task_id}:{args.goal or 'default'}",
            thread_key=task.get("threadKey") or (request or {}).get("threadKey"),
            target_plan_epoch=task.get("planEpoch") or (request or {}).get("targetPlanEpoch"),
        )
        refresh_runtime_state(root)
        emit(args.format, {"ok": True, "action": "stop", "taskId": args.task_id, "requestId": follow_up.get("requestId"), "status": follow_up.get("status"), "detail": follow_up.get("goal")})
        return 0

    if args.action == "restart":
        stage = args.from_stage or "queued"
        if stage not in RESET_FIELDS_BY_STAGE:
            raise ValueError(f"unsupported restart stage: {stage}")
        task.update({key: value for key, value in RESET_FIELDS_BY_STAGE[stage].items()})
        task["restartRequestedAt"] = now_iso()
        task["restartReason"] = args.reason or f"operator restart from {stage}"
        write_json(files["harness"] / "task-pool.json", task_pool)
        lineage_event(root, "task.restart_staged", "harness-control", task_id=args.task_id, detail=task["restartReason"], context={"fromStage": stage})
        refresh_runtime_state(root)
        emit(args.format, {"ok": True, "action": "restart", "taskId": args.task_id, "status": task.get("status"), "detail": stage})
        return 0

    raise ValueError(f"unsupported task action: {args.action}")


def cmd_request(args) -> int:
    root = Path(args.root).resolve()
    files = ensure_runtime_scaffold(root, generator="harness-control")
    index = load_json(files["request_index_path"])
    request = find_request(index.get("requests", []), args.request_id)

    if args.action == "cancel":
        update_request_status(index, args.request_id, "cancelled", reason=args.reason or "cancelled by harness-control")
        index["generatedAt"] = now_iso()
        index["generator"] = "harness-control"
        write_json(files["request_index_path"], index)
        update_request_snapshot(files, request, generator="harness-control")
        refresh_runtime_state(root)
        lineage_event(root, "request.cancelled", "harness-control", request_id=args.request_id, detail=args.reason or "cancelled by operator")
        emit(args.format, {"ok": True, "action": "cancel", "requestId": args.request_id, "status": "cancelled"})
        return 0

    raise ValueError(f"unsupported request action: {args.action}")


def cmd_project(args) -> int:
    root = Path(args.root).resolve()
    files = ensure_runtime_scaffold(root, generator="harness-control")
    project_meta = load_json(files["project_meta_path"])

    if args.action == "archive":
        project_meta["lifecycle"] = "archived"
        project_meta["archivedAt"] = now_iso()
        project_meta["archiveReason"] = args.reason or "archived by harness-control"
        project_meta["generator"] = "harness-control"
        project_meta["generatedAt"] = now_iso()
        write_json(files["project_meta_path"], project_meta)
        lineage_event(root, "project.archived", "harness-control", detail=project_meta["archiveReason"])
        refresh_runtime_state(root)
        emit(args.format, {"ok": True, "action": "archive", "status": project_meta.get("lifecycle"), "detail": project_meta.get("archiveReason")})
        return 0

    if args.action == "tidy-worktrees":
        return cmd_project_tidy_worktrees(root, files, fmt=args.format, dry_run=args.dry_run)

    raise ValueError(f"unsupported project action: {args.action}")


def main() -> int:
    parser = argparse.ArgumentParser(description="thin control actions for Klein-Harness")
    parser.add_argument("--root", required=True)
    parser.add_argument("--format", choices=["text", "json"], default="text")
    sub = parser.add_subparsers(dest="command", required=True)

    p_task = sub.add_parser("task")
    p_task.add_argument("task_id")
    p_task.add_argument("action", choices=["checkpoint", "archive", "stop", "restart"])
    p_task.add_argument("--from-stage")
    p_task.add_argument("--reason")
    p_task.add_argument("--goal")

    p_request = sub.add_parser("request")
    p_request.add_argument("request_id")
    p_request.add_argument("action", choices=["cancel"])
    p_request.add_argument("--reason")

    p_project = sub.add_parser("project")
    p_project.add_argument("action", choices=["archive", "tidy-worktrees"])
    p_project.add_argument("--reason")
    p_project.add_argument("--dry-run", action="store_true")

    args = parser.parse_args()
    if args.command == "task":
        return cmd_task(args)
    if args.command == "request":
        return cmd_request(args)
    if args.command == "project":
        return cmd_project(args)
    return 1


if __name__ == "__main__":
    try:
        sys.exit(main())
    except Exception as exc:
        print(f"harness-control failed: {exc}", file=sys.stderr)
        sys.exit(1)
