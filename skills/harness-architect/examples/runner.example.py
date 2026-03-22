#!/usr/bin/env python3
"""Unified runner: reconcile requests, route safely, and dispatch tasks."""
import argparse
import json
import subprocess
import sys
from datetime import datetime
from pathlib import Path

from runtime_common import (
    TASK_ACTIVE_STATUSES,
    TASK_COMPLETED_STATUSES,
    emit_follow_up_request,
    ensure_runtime_scaffold,
    find_task,
    load_json,
    load_optional_json,
    now_iso,
    reconcile_requests,
    request_bindings_for_task,
    update_binding_state,
    update_session_binding,
    write_json,
)


def tmux_session_alive(session_name: str) -> bool:
    if not session_name or session_name.startswith("print:"):
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


def task_sort_key(task: dict):
    priority = task.get("priority") or "P9"
    return (priority, task.get("taskId") or "")


def find_dispatchable_tasks(tasks: list[dict], preferred_task_ids: list[str] | None = None) -> list[dict]:
    completed_ids = {
        task["taskId"]
        for task in tasks
        if task.get("status") in TASK_COMPLETED_STATUSES
    }
    dispatchable = []
    for task in tasks:
        if task.get("status") != "queued":
            continue
        if task.get("planningStage") != "execution-ready":
            continue
        if task.get("claim", {}).get("agentId"):
            continue
        if not all(dep in completed_ids for dep in (task.get("dependsOn") or [])):
            continue
        dispatchable.append(task)
    preferred = preferred_task_ids or []
    dispatchable.sort(key=lambda task: ((task.get("taskId") not in preferred), *task_sort_key(task)))
    return dispatchable


def find_active_tasks(tasks: list[dict]) -> list[dict]:
    return [task for task in tasks if task.get("status") in TASK_ACTIVE_STATUSES]


def find_recoverable_tasks(tasks: list[dict], heartbeats: dict) -> list[dict]:
    result = []
    for task in tasks:
        if task.get("status") == "recoverable":
            result.append(task)
            continue
        if task.get("status") not in TASK_ACTIVE_STATUSES:
            continue
        tmux_name = task.get("claim", {}).get("tmuxSession") or heartbeats.get(task["taskId"], {}).get("tmuxSession")
        if tmux_name and not tmux_session_alive(tmux_name):
            result.append(task)
        elif not tmux_name and task.get("claim", {}).get("boundSessionId"):
            result.append(task)
    return result


def make_tmux_session_name(task_id: str, project_name: str) -> str:
    safe = project_name[:20].replace(" ", "-")
    ts = datetime.now().strftime("%m%d-%H%M%S")
    return f"hr-{task_id}-{safe}-{ts}"


def build_codex_command(session_id: str | None, project_root: str) -> list[str]:
    if session_id:
        return [
            "codex",
            "exec",
            "resume",
            session_id,
            "--yolo",
            "--skip-git-repo-check",
            "-",
        ]
    return [
        "codex",
        "exec",
        "--yolo",
        "--skip-git-repo-check",
        "-C",
        project_root,
        "-",
    ]


def call_route_session(script_path: Path, project_root: Path, task_id: str) -> dict:
    result = subprocess.run(
        ["python3", str(script_path), "--root", str(project_root), "--task-id", task_id, "--write-back"],
        capture_output=True,
        text=True,
        check=True,
    )
    return json.loads(result.stdout)


def prompt_lines(task: dict, route_decision: dict, project_root: str):
    lines = [
        "使用 harness-architect skill。",
        f"当前项目目录: {project_root}",
        f"当前任务: {task.get('taskId')}",
        f"title: {task.get('title', '-')}",
        f"summary: {task.get('summary', '-')}",
        f"roleHint: {task.get('roleHint', '-')}",
        f"resumeStrategy: {route_decision.get('resumeStrategy', 'fresh')}",
        f"routingMode: {route_decision.get('routingMode')}",
        f"gateReason: {route_decision.get('gateReason')}",
        f"promptStages: {route_decision.get('promptStages', [])}",
        "",
        "请读取 .harness/progress.md、.harness/task-pool.json、.harness/session-registry.json。",
        "如果存在 .harness/state/feedback-summary.json，只读取当前 task 最近 3 条高严重度失败。",
        f"找到任务 {task.get('taskId')}，按其 ownedPaths 和 verificationRuleIds 执行。",
        "执行完成后回写 task-pool.json、progress.md、lineage.jsonl、session-registry.json。",
    ]
    for failure in route_decision.get("recentFailures", [])[:3]:
        lines.append(
            f"- recentFailure: {failure.get('feedbackType')} [{failure.get('severity')}] {failure.get('message')}"
        )
    return lines


def dispatch_task(task: dict, route_decision: dict, project_root: str, project_name: str, state_dir: Path, log_dir: Path, dispatch_mode: str) -> dict:
    task_id = task["taskId"]
    session_id = route_decision.get("preferredResumeSessionId") if route_decision.get("resumeStrategy") == "resume" else None
    tmux_name = make_tmux_session_name(task_id, project_name) if dispatch_mode == "tmux" else f"print:{task_id}"
    log_path = log_dir / f"{task_id}.log"
    prompt_path = state_dir / f"runner-prompt-{task_id}.md"
    runner_script = state_dir / f"runner-exec-{task_id}.sh"
    prompt_path.write_text("\n".join(prompt_lines(task, route_decision, project_root)) + "\n")

    codex_cmd = build_codex_command(session_id, project_root)
    runner_py = Path(__file__).resolve()
    refresh_state_py = state_dir.parent / "scripts" / "refresh-state.py"
    diff_summary_py = state_dir.parent / "scripts" / "diff-summary.py"
    verify_task_py = state_dir.parent / "scripts" / "verify-task.py"
    heartbeat_cmd = f'python3 "{runner_py}" heartbeat "{project_root}" "{task_id}" "{tmux_name}"'
    finalize_cmd = (
        f'python3 "{runner_py}" finalize "{project_root}" "{task_id}" '
        f'--tmux-session "{tmux_name}" --runner-status "$runner_status"'
    )

    runner_script.write_text(
        f"#!/usr/bin/env bash\n"
        f"set -euo pipefail\n"
        f'cd "{project_root}"\n'
        f'exec > >(tee -a "{log_path}") 2>&1\n'
        f'trap \'status=$?; trap - EXIT; {heartbeat_cmd} --phase exited --exit-code \"$status\" || true; exit \"$status\"\' EXIT\n'
        f'echo "[runner] task={task_id} started at $(date \'+%Y-%m-%d %H:%M:%S\')"\n'
        f'{heartbeat_cmd} --phase running\n'
        f'runner_status=0\n'
        f'{" ".join(codex_cmd)} < "{prompt_path}" || runner_status=$?\n'
        f'if [[ -f "{diff_summary_py}" ]]; then\n'
        f'  python3 "{diff_summary_py}" --root "{project_root}" --task-id "{task_id}" --write-back || echo "[runner] diff-summary failed"\n'
        f'fi\n'
        f'if [[ -f "{verify_task_py}" ]]; then\n'
        f'  python3 "{verify_task_py}" --root "{project_root}" --task-id "{task_id}" --write-back || runner_status=$?\n'
        f'fi\n'
        f'{finalize_cmd} || true\n'
        f'if [[ -f "{refresh_state_py}" ]]; then\n'
        f'  python3 "{refresh_state_py}" "{project_root}" || echo "[runner] refresh-state failed"\n'
        f'fi\n'
        f'exit "$runner_status"\n'
    )
    runner_script.chmod(0o755)

    if dispatch_mode == "tmux":
        subprocess.run(["tmux", "new-session", "-d", "-s", tmux_name, f"bash {runner_script}"], check=True)
        subprocess.run(["tmux", "set-option", "-t", tmux_name, "remain-on-exit", "on"], capture_output=True)

    return {
        "taskId": task_id,
        "dispatchMode": dispatch_mode,
        "tmuxSession": tmux_name,
        "strategy": route_decision.get("resumeStrategy", "fresh"),
        "sessionId": session_id,
        "logPath": str(log_path),
        "promptPath": str(prompt_path),
        "runnerScriptPath": str(runner_script),
        "dispatchedAt": now_iso(),
        "gateReason": route_decision.get("gateReason"),
        "routeDecision": route_decision,
    }


def load_heartbeats(state_dir: Path) -> dict:
    data = load_optional_json(state_dir / "runner-heartbeats.json", {})
    entries = data.get("entries", {}) if isinstance(data, dict) else {}
    return entries if isinstance(entries, dict) else {}


def write_runner_state(state_dir: Path, trigger: str, active_runs: list, recoverable_runs: list, stale_runs: list, dispatchable_ids: list, errors: list, blocked_routes: list):
    write_json(state_dir / "runner-state.json", {
        "schemaVersion": "1.0",
        "generator": "harness-runner",
        "generatedAt": now_iso(),
        "lastTickAt": now_iso(),
        "lastTrigger": trigger,
        "activeRuns": active_runs,
        "recoverableRuns": recoverable_runs,
        "staleRuns": stale_runs,
        "dispatchableTaskIds": dispatchable_ids,
        "blockedRoutes": blocked_routes,
        "lastErrors": errors,
    })


def write_heartbeat(state_dir: Path, task_id: str, tmux_session: str, phase: str = "running", exit_code: int | None = None):
    hb_path = state_dir / "runner-heartbeats.json"
    hb = load_optional_json(hb_path, {"schemaVersion": "1.0", "generator": "harness-runner", "generatedAt": now_iso(), "entries": {}})
    hb.setdefault("entries", {})[task_id] = {
        "taskId": task_id,
        "tmuxSession": tmux_session,
        "lastHeartbeatAt": now_iso(),
        "lastKnownPhase": phase,
        "lastExitCode": exit_code,
    }
    hb["generatedAt"] = now_iso()
    write_json(hb_path, hb)


def claim_dispatched_task(root: Path, task_id: str, run: dict, *, lifecycle_status: str):
    files = ensure_runtime_scaffold(root, generator="harness-runner")
    task_pool_path = files["harness"] / "task-pool.json"
    task_pool = load_json(task_pool_path)
    task = find_task(task_pool.get("tasks", []), task_id)
    claim = task.setdefault("claim", {})
    now = now_iso()
    if task.get("status") == "queued":
        task["status"] = "claimed"
    claim["agentId"] = "harness-runner"
    claim["role"] = task.get("roleHint") or claim.get("role")
    claim["nodeId"] = f"tmux:{run['tmuxSession']}" if run.get("dispatchMode") == "tmux" else "dispatch:print"
    claim["tmuxSession"] = run.get("tmuxSession")
    claim["boundSessionId"] = run.get("sessionId") or claim.get("boundSessionId")
    claim["boundResumeStrategy"] = run.get("strategy")
    claim["boundAt"] = now
    claim["leasedAt"] = now
    if run.get("sessionId"):
        task["lastKnownSessionId"] = run["sessionId"]
    task["status"] = lifecycle_status
    task["lastDispatchAt"] = now
    write_json(task_pool_path, task_pool)

    update_session_binding(
        root,
        task_id=task_id,
        session_id=run.get("sessionId"),
        node_id=claim.get("nodeId"),
        status=lifecycle_status,
        bound_from_task_id=claim.get("boundFromTaskId"),
        generator="harness-runner",
    )

    task_map = load_json(files["request_task_map_path"])
    for binding in request_bindings_for_task(task_map, task_id):
        update_binding_state(
            root,
            binding.get("requestId"),
            task_id,
            lifecycle_status,
            reason=f"runner {lifecycle_status}",
            generator="harness-runner",
            session_id=run.get("sessionId"),
            route=run.get("routeDecision"),
            worktree_path=task.get("worktreePath"),
            diff_summary=task.get("diffSummary"),
        )


def preferred_request_task_ids(root: Path) -> list[str]:
    files = ensure_runtime_scaffold(root, generator="harness-runner")
    request_summary = load_optional_json(files["request_summary_path"], {})
    return [
        binding.get("taskId")
        for binding in request_summary.get("bindings", [])
        if binding.get("requestStatus") in {"bound", "recoverable", "resumed", "queued"}
    ]


def blocked_follow_up_kind(task: dict, route_decision: dict):
    reason = (route_decision.get("gateReason") or "").lower()
    if "session" in reason:
        return "stop"
    if task.get("kind") == "audit":
        return "audit"
    return "replan"


def emit_blocked_follow_up(root: Path, task: dict, route_decision: dict):
    kind = blocked_follow_up_kind(task, route_decision)
    return emit_follow_up_request(
        root,
        kind=kind,
        goal=f"{kind} follow-up for {task.get('taskId')}: {route_decision.get('gateReason')}",
        source="runtime:runner",
        generator="harness-runner",
        origin_task_id=task.get("taskId"),
        origin_session_id=route_decision.get("preferredResumeSessionId"),
        reason=route_decision.get("gateReason"),
        dedupe_key=f"{kind}:{task.get('taskId')}:{route_decision.get('gateReason')}",
    )


def cmd_tick(root: Path, trigger: str = "shell", dispatch_mode: str = "tmux"):
    files = ensure_runtime_scaffold(root, generator="harness-runner")
    reconcile_requests(root, generator="harness-runner")
    state_dir = files["state_dir"]
    log_dir = state_dir / "runner-logs"
    task_pool = load_json(files["harness"] / "task-pool.json")
    route_session_py = state_dir.parent / "scripts" / "route-session.py"
    tasks = task_pool.get("tasks", [])
    heartbeats = load_heartbeats(state_dir)

    active = find_active_tasks(tasks)
    recoverable = find_recoverable_tasks(tasks, heartbeats)
    dispatchable = find_dispatchable_tasks(tasks, preferred_request_task_ids(root))

    live_runs = []
    stale_runs = []
    for task in active:
        tmux_name = task.get("claim", {}).get("tmuxSession") or heartbeats.get(task["taskId"], {}).get("tmuxSession")
        if tmux_name and tmux_session_alive(tmux_name):
            live_runs.append({"taskId": task["taskId"], "tmuxSession": tmux_name, "status": "running"})
        elif tmux_name:
            stale_runs.append({"taskId": task["taskId"], "tmuxSession": tmux_name, "status": "stale"})

    errors = []
    dispatched = []
    blocked_routes = []

    for task in recoverable:
        try:
            route_decision = call_route_session(route_session_py, root, task["taskId"])
            if not route_decision.get("dispatchReady"):
                blocked_routes.append({"taskId": task["taskId"], "gateStatus": route_decision.get("gateStatus"), "gateReason": route_decision.get("gateReason")})
                emit_blocked_follow_up(root, task, route_decision)
                continue
            run = dispatch_task(task, route_decision, str(root), root.name, state_dir, log_dir, dispatch_mode)
            claim_dispatched_task(root, task["taskId"], run, lifecycle_status="resumed")
            write_heartbeat(state_dir, task["taskId"], run["tmuxSession"], phase="resumed")
            dispatched.append(run)
        except Exception as exc:
            errors.append({"taskId": task["taskId"], "error": str(exc)})

    if not live_runs and not dispatched and dispatchable:
        task = dispatchable[0]
        try:
            route_decision = call_route_session(route_session_py, root, task["taskId"])
            if not route_decision.get("dispatchReady"):
                blocked_routes.append({"taskId": task["taskId"], "gateStatus": route_decision.get("gateStatus"), "gateReason": route_decision.get("gateReason")})
                emit_blocked_follow_up(root, task, route_decision)
            else:
                run = dispatch_task(task, route_decision, str(root), root.name, state_dir, log_dir, dispatch_mode)
                claim_dispatched_task(root, task["taskId"], run, lifecycle_status="dispatched")
                write_heartbeat(state_dir, task["taskId"], run["tmuxSession"], phase="dispatched")
                dispatched.append(run)
        except Exception as exc:
            errors.append({"taskId": task["taskId"], "error": str(exc)})

    latest_tasks = load_json(files["harness"] / "task-pool.json").get("tasks", [])
    latest_dispatchable = find_dispatchable_tasks(latest_tasks, preferred_request_task_ids(root))
    write_runner_state(
        state_dir,
        trigger,
        active_runs=live_runs + dispatched,
        recoverable_runs=[{"taskId": task["taskId"]} for task in recoverable if task["taskId"] not in {run["taskId"] for run in dispatched}],
        stale_runs=stale_runs,
        dispatchable_ids=[task["taskId"] for task in latest_dispatchable],
        errors=errors,
        blocked_routes=blocked_routes,
    )

    result = {
        "ok": not errors,
        "liveRuns": len(live_runs),
        "dispatched": dispatched,
        "recoverable": len(recoverable),
        "dispatchable": len(latest_dispatchable),
        "blocked": blocked_routes,
        "errors": errors,
    }
    print(json.dumps(result, ensure_ascii=False, indent=2))
    return 0 if not errors else 1


def cmd_run(root: Path, task_id: str, trigger: str = "shell", dispatch_mode: str = "tmux", lifecycle_status: str = "dispatched"):
    files = ensure_runtime_scaffold(root, generator="harness-runner")
    reconcile_requests(root, generator="harness-runner")
    state_dir = files["state_dir"]
    log_dir = state_dir / "runner-logs"
    task_pool = load_json(files["harness"] / "task-pool.json")
    route_session_py = state_dir.parent / "scripts" / "route-session.py"
    task = find_task(task_pool.get("tasks", []), task_id)

    try:
        route_decision = call_route_session(route_session_py, root, task_id)
        if not route_decision.get("dispatchReady"):
            emit_blocked_follow_up(root, task, route_decision)
            print(json.dumps({
                "ok": False,
                "taskId": task_id,
                "gateStatus": route_decision.get("gateStatus"),
                "gateReason": route_decision.get("gateReason"),
                "needsOrchestrator": route_decision.get("needsOrchestrator"),
            }, ensure_ascii=False, indent=2))
            return 1
        run = dispatch_task(task, route_decision, str(root), root.name, state_dir, log_dir, dispatch_mode)
        claim_dispatched_task(root, task_id, run, lifecycle_status=lifecycle_status)
        write_heartbeat(state_dir, task_id, run["tmuxSession"], phase=lifecycle_status)
        print(json.dumps({"ok": True, "dispatched": run}, ensure_ascii=False, indent=2))
        return 0
    except Exception as exc:
        print(json.dumps({"ok": False, "error": str(exc)}, ensure_ascii=False, indent=2))
        return 1


def cmd_recover(root: Path, task_id: str, trigger: str = "shell", dispatch_mode: str = "tmux"):
    return cmd_run(root, task_id, trigger, dispatch_mode, lifecycle_status="resumed")


def cmd_finalize(root: Path, task_id: str, runner_status: int, tmux_session: str | None):
    files = ensure_runtime_scaffold(root, generator="harness-runner")
    task_pool_path = files["harness"] / "task-pool.json"
    task_pool = load_json(task_pool_path)
    task = find_task(task_pool.get("tasks", []), task_id)
    if task is None:
        print(json.dumps({"ok": False, "error": f"task not found: {task_id}"}, ensure_ascii=False, indent=2))
        return 1

    task_map = load_json(files["request_task_map_path"])
    bindings = request_bindings_for_task(task_map, task_id)
    session_id = task.get("claim", {}).get("boundSessionId") or task.get("lastKnownSessionId")
    verification_status = task.get("verificationStatus")

    if runner_status == 0 and verification_status in {"pass", "skipped", None}:
        task["status"] = "completed"
        task["completedAt"] = now_iso()
        write_json(task_pool_path, task_pool)
        update_session_binding(
            root,
            task_id=task_id,
            session_id=session_id,
            node_id=task.get("claim", {}).get("nodeId"),
            status="completed",
            generator="harness-runner",
        )
        for binding in bindings:
            update_binding_state(
                root,
                binding.get("requestId"),
                task_id,
                "completed",
                reason="runner finalized completed task",
                generator="harness-runner",
                session_id=session_id,
                worktree_path=task.get("worktreePath"),
                diff_summary=task.get("diffSummary"),
                verification={
                    "overallStatus": verification_status or "skipped",
                    "summary": task.get("verificationSummary"),
                    "verificationResultPath": task.get("verificationResultPath"),
                },
                outcome={"status": "completed"},
            )
    else:
        task["status"] = "recoverable"
        task["recoverableAt"] = now_iso()
        task["recoveryReason"] = "verification failed" if verification_status == "fail" else f"runner exited with code {runner_status}"
        write_json(task_pool_path, task_pool)
        update_session_binding(
            root,
            task_id=task_id,
            session_id=session_id,
            node_id=task.get("claim", {}).get("nodeId"),
            status="recoverable",
            error=task.get("recoveryReason"),
            generator="harness-runner",
        )
        for binding in bindings:
            update_binding_state(
                root,
                binding.get("requestId"),
                task_id,
                "recoverable",
                reason=task.get("recoveryReason"),
                generator="harness-runner",
                session_id=session_id,
                worktree_path=task.get("worktreePath"),
                diff_summary=task.get("diffSummary"),
                verification={
                    "overallStatus": verification_status,
                    "summary": task.get("verificationSummary"),
                    "verificationResultPath": task.get("verificationResultPath"),
                },
                outcome={"status": "recoverable", "runnerStatus": runner_status},
            )
            emit_follow_up_request(
                root,
                kind="replan",
                goal=f"Replan {task_id} after runner finalize failure",
                source="runtime:runner",
                generator="harness-runner",
                parent_request_id=binding.get("requestId"),
                origin_task_id=task_id,
                origin_session_id=session_id,
                reason=task.get("recoveryReason"),
                dedupe_key=f"replan:{task_id}:{task.get('recoveryReason')}",
            )

    write_heartbeat(files["state_dir"], task_id, tmux_session or f"print:{task_id}", phase=task.get("status"), exit_code=runner_status)
    print(
        json.dumps(
            {
                "ok": True,
                "taskId": task_id,
                "finalStatus": task.get("status"),
                "verificationStatus": verification_status,
            },
            ensure_ascii=False,
            indent=2,
        )
    )
    return 0


def cmd_heartbeat(root: Path, task_id: str, tmux_session: str, phase: str = "running", exit_code: int | None = None):
    files = ensure_runtime_scaffold(root, generator="harness-runner")
    write_heartbeat(files["state_dir"], task_id, tmux_session, phase=phase, exit_code=exit_code)
    task_pool = load_optional_json(files["harness"] / "task-pool.json", {"tasks": []})
    task = next((item for item in task_pool.get("tasks", []) if item.get("taskId") == task_id), None)
    if task is None:
        return 0
    task_map = load_json(files["request_task_map_path"])
    bindings = request_bindings_for_task(task_map, task_id)
    session_id = task.get("claim", {}).get("boundSessionId")

    if phase == "running":
        update_session_binding(root, task_id=task_id, session_id=session_id, node_id=task.get("claim", {}).get("nodeId"), status="running", generator="harness-runner")
        for binding in bindings:
            update_binding_state(
                root,
                binding.get("requestId"),
                task_id,
                "running",
                reason="runner heartbeat: task is running",
                generator="harness-runner",
                session_id=session_id,
                worktree_path=task.get("worktreePath"),
                diff_summary=task.get("diffSummary"),
            )
    elif phase == "exited" and exit_code not in {None, 0}:
        update_session_binding(
            root,
            task_id=task_id,
            session_id=session_id,
            node_id=task.get("claim", {}).get("nodeId"),
            status="recoverable",
            error=f"runner exited with code {exit_code}",
            generator="harness-runner",
        )
        for binding in bindings:
            update_binding_state(
                root,
                binding.get("requestId"),
                task_id,
                "recoverable",
                reason=f"runner exited with code {exit_code}",
                generator="harness-runner",
                session_id=session_id,
                worktree_path=task.get("worktreePath"),
                diff_summary=task.get("diffSummary"),
                outcome={"status": "recoverable", "exitCode": exit_code},
            )
            emit_follow_up_request(
                root,
                kind="replan",
                goal=f"Recover {task_id} after runner exit {exit_code}",
                source="runtime:runner",
                generator="harness-runner",
                parent_request_id=binding.get("requestId"),
                origin_task_id=task_id,
                origin_session_id=session_id,
                reason=f"runner exited with code {exit_code}",
                dedupe_key=f"recover:{task_id}:{exit_code}",
            )
    return 0


def cmd_list(root: Path):
    files = ensure_runtime_scaffold(root, generator="harness-runner")
    result = {
        "runnerState": load_optional_json(files["runner_state_path"], {}),
        "heartbeats": load_heartbeats(files["state_dir"]),
        "tmuxSessions": tmux_list_sessions(),
    }
    print(json.dumps(result, ensure_ascii=False, indent=2))
    return 0


def main():
    parser = argparse.ArgumentParser(description="harness-runner: unified request-aware task executor")
    sub = parser.add_subparsers(dest="command")

    p_tick = sub.add_parser("tick", help="reconcile requests and dispatch work")
    p_tick.add_argument("root", help="project root")
    p_tick.add_argument("--trigger", default="shell")
    p_tick.add_argument("--dispatch-mode", choices=["tmux", "print"], default="tmux")

    p_run = sub.add_parser("run", help="force-dispatch a specific task")
    p_run.add_argument("task_id", help="task id")
    p_run.add_argument("root", help="project root")
    p_run.add_argument("--trigger", default="shell")
    p_run.add_argument("--dispatch-mode", choices=["tmux", "print"], default="tmux")

    p_recover = sub.add_parser("recover", help="recover a specific task")
    p_recover.add_argument("task_id", help="task id")
    p_recover.add_argument("root", help="project root")
    p_recover.add_argument("--trigger", default="shell")
    p_recover.add_argument("--dispatch-mode", choices=["tmux", "print"], default="tmux")

    p_list = sub.add_parser("list", help="list runner state and tmux sessions")
    p_list.add_argument("root", help="project root")

    p_finalize = sub.add_parser("finalize", help="finalize a dispatched task after execution")
    p_finalize.add_argument("root", help="project root")
    p_finalize.add_argument("task_id", help="task id")
    p_finalize.add_argument("--tmux-session")
    p_finalize.add_argument("--runner-status", type=int, default=0)

    p_heartbeat = sub.add_parser("heartbeat", help="internal heartbeat writer")
    p_heartbeat.add_argument("root", help="project root")
    p_heartbeat.add_argument("task_id", help="task id")
    p_heartbeat.add_argument("tmux_session", help="tmux or print session label")
    p_heartbeat.add_argument("--phase", default="running")
    p_heartbeat.add_argument("--exit-code", type=int)

    args = parser.parse_args()
    if not args.command:
        parser.print_help()
        return 1

    root = Path(args.root).resolve()
    if not (root / ".harness").is_dir():
        print(f"error: .harness not found in {root}", file=sys.stderr)
        return 2

    if args.command == "tick":
        return cmd_tick(root, args.trigger, args.dispatch_mode)
    if args.command == "run":
        return cmd_run(root, args.task_id, args.trigger, args.dispatch_mode)
    if args.command == "recover":
        return cmd_recover(root, args.task_id, args.trigger, args.dispatch_mode)
    if args.command == "finalize":
        return cmd_finalize(root, args.task_id, args.runner_status, args.tmux_session)
    if args.command == "list":
        return cmd_list(root)
    if args.command == "heartbeat":
        return cmd_heartbeat(root, args.task_id, args.tmux_session, args.phase, args.exit_code)
    return 1


if __name__ == "__main__":
    try:
        sys.exit(main() or 0)
    except Exception as exc:
        print(f"runner failed: {exc}", file=sys.stderr)
        sys.exit(1)
