#!/usr/bin/env python3
"""Unified runner: reconcile task state, decide fresh/resume, dispatch via tmux + codex."""
import argparse
import json
import subprocess
import sys
from datetime import datetime, timezone
from pathlib import Path


def load_json(path: Path):
    return json.loads(path.read_text())


def load_optional_json(path: Path):
    if path.exists():
        return load_json(path)
    return None


def write_json(path: Path, data):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(data, ensure_ascii=False, indent=2) + "\n")


def now_iso():
    return datetime.now(timezone.utc).astimezone().isoformat(timespec="seconds")


def find_task(tasks: list[dict], task_id: str) -> dict | None:
    for task in tasks:
        if task.get("taskId") == task_id:
            return task
    return None


def tmux_session_alive(session_name: str) -> bool:
    try:
        subprocess.run(
            ["tmux", "has-session", "-t", session_name],
            capture_output=True, check=True,
        )
        return True
    except (subprocess.CalledProcessError, FileNotFoundError):
        return False


def tmux_list_sessions() -> list[str]:
    try:
        result = subprocess.run(
            ["tmux", "list-sessions", "-F", "#{session_name}"],
            capture_output=True, text=True, check=True,
        )
        return [s.strip() for s in result.stdout.splitlines() if s.strip()]
    except (subprocess.CalledProcessError, FileNotFoundError):
        return []

def find_dispatchable_tasks(tasks: list[dict]) -> list[dict]:
    """Return tasks that are safe to dispatch: queued, execution-ready, deps met."""
    completed_ids = {
        t["taskId"]
        for t in tasks
        if t.get("status") in ("completed", "validated", "done", "pass")
    }
    result = []
    for t in tasks:
        if t.get("status") != "queued":
            continue
        if t.get("planningStage") != "execution-ready":
            continue
        if t.get("claim", {}).get("agentId"):
            continue
        deps = t.get("dependsOn") or []
        if all(d in completed_ids for d in deps):
            result.append(t)
    return sorted(result, key=lambda x: x.get("priority", "P9"))


def find_active_tasks(tasks: list[dict]) -> list[dict]:
    return [t for t in tasks if t.get("status") in ("active", "claimed", "in_progress")]


def find_recoverable_tasks(tasks: list[dict], heartbeats: dict) -> list[dict]:
    """Tasks marked active but whose tmux session is gone."""
    result = []
    for t in find_active_tasks(tasks):
        tmux_name = (
            t.get("claim", {}).get("tmuxSession")
            or heartbeats.get(t["taskId"], {}).get("tmuxSession")
        )
        if tmux_name and not tmux_session_alive(tmux_name):
            result.append(t)
        elif not tmux_name and t.get("claim", {}).get("boundSessionId"):
            result.append(t)
    return result


def make_tmux_session_name(task_id: str, project_name: str) -> str:
    safe = project_name[:20].replace(" ", "-")
    ts = datetime.now().strftime("%m%d-%H%M%S")
    return f"hr-{task_id}-{safe}-{ts}"


def build_codex_command(session_id: str | None, project_root: str) -> list[str]:
    """Build a fresh or resume codex command."""
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
        ["python3", str(script_path), "--root", str(project_root), "--task-id", task_id],
        capture_output=True, text=True, check=True,
    )
    return json.loads(result.stdout)


def dispatch_task(task: dict, route_decision: dict, project_root: str,
                  project_name: str, state_dir: Path, log_dir: Path,
                  dispatch_mode: str = "tmux") -> dict:
    """Prepare task execution and optionally launch it in a detached tmux session."""
    task_id = task["taskId"]
    session_id = route_decision.get("preferredResumeSessionId") if route_decision.get("resumeStrategy") == "resume" else None
    tmux_name = make_tmux_session_name(task_id, project_name)
    log_path = log_dir / f"{task_id}.log"
    log_dir.mkdir(parents=True, exist_ok=True)

    codex_cmd = build_codex_command(session_id, project_root)
    runner_py = Path(__file__).resolve()
    refresh_state_py = state_dir.parent / "scripts" / "refresh-state.py"
    diff_summary_py = state_dir.parent / "scripts" / "diff-summary.py"
    verify_task_py = state_dir.parent / "scripts" / "verify-task.py"

    prompt_path = state_dir / f"runner-prompt-{task_id}.md"
    role = task.get("roleHint", "worker")
    prompt_lines = [
        "使用 harness-architect skill。",
        f"当前项目目录: {project_root}",
        f"当前任务: {task_id}",
        f"title: {task.get('title', '-')}",
        f"summary: {task.get('summary', '-')}",
        f"roleHint: {role}",
        f"resumeStrategy: {route_decision.get('resumeStrategy', 'fresh')}",
        f"routingMode: {route_decision.get('routingMode')}",
        f"gateReason: {route_decision.get('gateReason')}",
        f"promptStages: {route_decision.get('promptStages', [])}",
        "",
        "请读取 .harness/progress.md、.harness/task-pool.json、.harness/session-registry.json，",
        "如果存在 .harness/state/feedback-summary.json，只读取当前 task 最近 3 条高严重度失败。",
        f"找到任务 {task_id}，按其 ownedPaths 和 verificationRuleIds 执行。",
        "执行完成后回写 task-pool.json、progress.md、lineage.jsonl、session-registry.json。",
    ]
    for failure in route_decision.get("recentFailures", [])[:3]:
        prompt_lines.append(
            f"- recentFailure: {failure.get('feedbackType')} [{failure.get('severity')}] {failure.get('message')}"
        )
    prompt_path.write_text("\n".join(prompt_lines) + "\n")

    # Build runner script
    runner_script = state_dir / f"runner-exec-{task_id}.sh"
    heartbeat_cmd = (
        f'python3 "{runner_py}" heartbeat "{project_root}" "{task_id}" "{tmux_name}"'
    )
    runner_script.write_text(
        f"#!/usr/bin/env bash\n"
        f"set -euo pipefail\n"
        f'cd "{project_root}"\n'
        f'exec > >(tee -a "{log_path}") 2>&1\n'
        f'trap \'status=$?; trap - EXIT; {heartbeat_cmd} --phase exited --exit-code "$status" || true; exit "$status"\' EXIT\n'
        f'echo "[runner] task={task_id} started at $(date \'+%Y-%m-%d %H:%M:%S\')"\n'
        f'echo "[runner] strategy={"resume" if session_id else "fresh"}"\n'
        f'{heartbeat_cmd} --phase running\n'
        f'runner_status=0\n'
        f'{" ".join(codex_cmd)} < "{prompt_path}" || runner_status=$?\n'
        f'if [[ "$runner_status" -eq 0 ]]; then\n'
        f'  echo "[runner] generating diff summary for {task_id}"\n'
        f'  if [[ -f "{diff_summary_py}" ]]; then\n'
        f'    python3 "{diff_summary_py}" --root "{project_root}" --task-id "{task_id}" --write-back || echo "[runner] diff-summary failed"\n'
        f'  fi\n'
        f'  echo "[runner] running verification for {task_id}"\n'
        f'  if [[ -f "{verify_task_py}" ]]; then\n'
        f'    python3 "{verify_task_py}" --root "{project_root}" --task-id "{task_id}" --write-back || runner_status=$?\n'
        f'  fi\n'
        f'else\n'
        f'  echo "[runner] codex execution failed for {task_id} with exit code $runner_status"\n'
        f'fi\n'
        f'if [[ -f "{refresh_state_py}" ]]; then\n'
        f'  echo "[runner] refreshing hot state..."\n'
        f'  python3 "{refresh_state_py}" "{project_root}" || echo "[runner] refresh-state failed"\n'
        f'fi\n'
        f'echo "[runner] task={task_id} finished at $(date \'+%Y-%m-%d %H:%M:%S\')"\n'
        f'exit "$runner_status"\n'
    )
    runner_script.chmod(0o755)

    if dispatch_mode == "tmux":
        subprocess.run(
            ["tmux", "new-session", "-d", "-s", tmux_name, f"bash {runner_script}"],
            check=True,
        )
        subprocess.run(
            ["tmux", "set-option", "-t", tmux_name, "remain-on-exit", "on"],
            capture_output=True,
        )
    else:
        tmux_name = None

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
    }


def load_heartbeats(state_dir: Path) -> dict:
    hb = load_optional_json(state_dir / "runner-heartbeats.json")
    if not hb or not isinstance(hb, dict):
        return {}
    entries = hb.get("entries", {})
    return entries if isinstance(entries, dict) else {}


def write_runner_state(state_dir: Path, trigger: str, active_runs: list,
                       recoverable_runs: list, stale_runs: list,
                       dispatchable_ids: list, errors: list, blocked_routes: list):
    write_json(state_dir / "runner-state.json", {
        "schemaVersion": "1.0",
        "generator": "harness-runner",
        "generatedAt": now_iso(),
        "lastTickAt": now_iso(),
        "lastTrigger": trigger,
        "activeRuns": active_runs,
        "recoverableRuns": [{"taskId": t["taskId"]} for t in recoverable_runs],
        "staleRuns": stale_runs,
        "dispatchableTaskIds": dispatchable_ids,
        "blockedRoutes": blocked_routes,
        "lastErrors": errors,
    })


def write_heartbeat(state_dir: Path, task_id: str, tmux_session: str,
                    phase: str = "running", exit_code: int | None = None):
    hb_path = state_dir / "runner-heartbeats.json"
    hb = load_optional_json(hb_path) or {"schemaVersion": "1.0", "entries": {}}
    hb["entries"][task_id] = {
        "taskId": task_id,
        "tmuxSession": tmux_session,
        "lastHeartbeatAt": now_iso(),
        "lastKnownPhase": phase,
        "lastExitCode": exit_code,
    }
    write_json(hb_path, hb)


def claim_dispatched_task(harness: Path, task_id: str, run: dict):
    task_pool_path = harness / "task-pool.json"
    task_pool = load_json(task_pool_path)
    task = find_task(task_pool.get("tasks", []), task_id)
    if not task:
        raise KeyError(f"task not found: {task_id}")

    claim = task.setdefault("claim", {})
    now = now_iso()
    if task.get("status") == "queued":
        task["status"] = "claimed"
    claim["agentId"] = "harness-runner"
    claim["role"] = task.get("roleHint") or claim.get("role")
    claim["nodeId"] = f"tmux:{run['tmuxSession']}" if run.get("tmuxSession") else "dispatch:print"
    claim["tmuxSession"] = run.get("tmuxSession")
    claim["boundSessionId"] = run.get("sessionId") or claim.get("boundSessionId")
    claim["boundResumeStrategy"] = run.get("strategy")
    claim["boundAt"] = now
    claim["leasedAt"] = now
    if run.get("sessionId"):
        task["lastKnownSessionId"] = run["sessionId"]
    write_json(task_pool_path, task_pool)


def cmd_tick(root: Path, trigger: str = "shell", dispatch_mode: str = "tmux"):
    harness = root / ".harness"
    state_dir = harness / "state"
    log_dir = state_dir / "runner-logs"
    project_name = root.name

    task_pool = load_json(harness / "task-pool.json")
    heartbeats = load_heartbeats(state_dir)
    route_session_py = state_dir.parent / "scripts" / "route-session.py"
    tasks = task_pool.get("tasks", [])

    active = find_active_tasks(tasks)
    recoverable = find_recoverable_tasks(tasks, heartbeats)
    dispatchable = find_dispatchable_tasks(tasks)

    # Check for truly live active runs
    live_runs = []
    stale_runs = []
    for t in active:
        tmux_name = (
            t.get("claim", {}).get("tmuxSession")
            or heartbeats.get(t["taskId"], {}).get("tmuxSession")
        )
        if tmux_name and tmux_session_alive(tmux_name):
            live_runs.append({
                "taskId": t["taskId"],
                "tmuxSession": tmux_name,
                "status": "running",
            })
        elif tmux_name:
            stale_runs.append({
                "taskId": t["taskId"],
                "tmuxSession": tmux_name,
                "status": "stale",
            })

    errors = []
    dispatched = []
    blocked_routes = []

    # Priority 1: recover stale tasks
    for t in recoverable:
        try:
            route_decision = call_route_session(route_session_py, root, t["taskId"])
            if not route_decision.get("dispatchReady"):
                blocked_routes.append({
                    "taskId": t["taskId"],
                    "gateStatus": route_decision.get("gateStatus"),
                    "gateReason": route_decision.get("gateReason"),
                })
                continue
            run = dispatch_task(t, route_decision, str(root), project_name, state_dir, log_dir, dispatch_mode)
            if dispatch_mode == "tmux":
                claim_dispatched_task(harness, t["taskId"], run)
                write_heartbeat(state_dir, t["taskId"], run["tmuxSession"])
            dispatched.append(run)
        except Exception as exc:
            errors.append({"taskId": t["taskId"], "error": str(exc)})

    # Priority 2: dispatch new tasks if no active work and slots available
    if not live_runs and not dispatched and dispatchable:
        t = dispatchable[0]
        try:
            route_decision = call_route_session(route_session_py, root, t["taskId"])
            if not route_decision.get("dispatchReady"):
                blocked_routes.append({
                    "taskId": t["taskId"],
                    "gateStatus": route_decision.get("gateStatus"),
                    "gateReason": route_decision.get("gateReason"),
                })
            else:
                run = dispatch_task(t, route_decision, str(root), project_name, state_dir, log_dir, dispatch_mode)
                if dispatch_mode == "tmux":
                    claim_dispatched_task(harness, t["taskId"], run)
                    write_heartbeat(state_dir, t["taskId"], run["tmuxSession"])
                dispatched.append(run)
        except Exception as exc:
            errors.append({"taskId": t["taskId"], "error": str(exc)})

    latest_tasks = load_json(harness / "task-pool.json").get("tasks", [])
    latest_dispatchable = find_dispatchable_tasks(latest_tasks)

    write_runner_state(
        state_dir, trigger,
        active_runs=live_runs + dispatched,
        recoverable_runs=[r for r in recoverable if r["taskId"] not in {d["taskId"] for d in dispatched}],
        stale_runs=stale_runs,
        dispatchable_ids=[t["taskId"] for t in latest_dispatchable],
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


def cmd_run(root: Path, task_id: str, trigger: str = "shell", dispatch_mode: str = "tmux"):
    harness = root / ".harness"
    state_dir = harness / "state"
    log_dir = state_dir / "runner-logs"
    project_name = root.name

    task_pool = load_json(harness / "task-pool.json")
    route_session_py = state_dir.parent / "scripts" / "route-session.py"
    tasks = task_pool.get("tasks", [])

    task = None
    for t in tasks:
        if t.get("taskId") == task_id:
            task = t
            break
    if not task:
        print(json.dumps({"ok": False, "error": f"task not found: {task_id}"}))
        return 1

    try:
        route_decision = call_route_session(route_session_py, root, task_id)
        if not route_decision.get("dispatchReady"):
            print(json.dumps({
                "ok": False,
                "taskId": task_id,
                "gateStatus": route_decision.get("gateStatus"),
                "gateReason": route_decision.get("gateReason"),
                "needsOrchestrator": route_decision.get("needsOrchestrator"),
            }, ensure_ascii=False, indent=2))
            return 1
        run = dispatch_task(task, route_decision, str(root), project_name, state_dir, log_dir, dispatch_mode)
        if dispatch_mode == "tmux":
            claim_dispatched_task(harness, task_id, run)
            write_heartbeat(state_dir, task_id, run["tmuxSession"])
        print(json.dumps({"ok": True, "dispatched": run}, ensure_ascii=False, indent=2))
        return 0
    except Exception as exc:
        print(json.dumps({"ok": False, "error": str(exc)}))
        return 1


def cmd_recover(root: Path, task_id: str, trigger: str = "shell", dispatch_mode: str = "tmux"):
    return cmd_run(root, task_id, trigger, dispatch_mode)


def cmd_heartbeat(root: Path, task_id: str, tmux_session: str,
                  phase: str = "running", exit_code: int | None = None):
    harness = root / ".harness"
    state_dir = harness / "state"
    write_heartbeat(state_dir, task_id, tmux_session, phase=phase, exit_code=exit_code)
    return 0


def cmd_list(root: Path):
    harness = root / ".harness"
    state_dir = harness / "state"
    runner_state = load_optional_json(state_dir / "runner-state.json")
    heartbeats = load_heartbeats(state_dir)
    tmux_sessions = tmux_list_sessions()

    result = {
        "runnerState": runner_state,
        "heartbeats": heartbeats,
        "tmuxSessions": tmux_sessions,
    }
    print(json.dumps(result, ensure_ascii=False, indent=2))
    return 0


def main():
    parser = argparse.ArgumentParser(description="harness-runner: unified task executor")
    sub = parser.add_subparsers(dest="command")

    p_tick = sub.add_parser("tick", help="reconcile and dispatch")
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

    p_heartbeat = sub.add_parser("heartbeat", help="internal heartbeat writer")
    p_heartbeat.add_argument("root", help="project root")
    p_heartbeat.add_argument("task_id", help="task id")
    p_heartbeat.add_argument("tmux_session", help="tmux session")
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
    elif args.command == "run":
        return cmd_run(root, args.task_id, args.trigger, args.dispatch_mode)
    elif args.command == "recover":
        return cmd_recover(root, args.task_id, args.trigger, args.dispatch_mode)
    elif args.command == "list":
        return cmd_list(root)
    elif args.command == "heartbeat":
        return cmd_heartbeat(root, args.task_id, args.tmux_session, args.phase, args.exit_code)
    return 1


if __name__ == "__main__":
    try:
        sys.exit(main() or 0)
    except Exception as exc:
        print(f"runner failed: {exc}", file=sys.stderr)
        sys.exit(1)
