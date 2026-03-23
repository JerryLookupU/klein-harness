#!/usr/bin/env python3
from __future__ import annotations
"""Unified runner: reconcile requests, route safely, and dispatch tasks."""
import argparse
import json
import subprocess
import sys
import time
from datetime import datetime
from pathlib import Path

from runtime_common import (
    TASK_ACTIVE_STATUSES,
    TASK_COMPLETED_STATUSES,
    age_seconds,
    apply_dirty_state_to_worktree_registry,
    build_compact_log_artifact,
    build_completion_gate,
    build_daemon_summary,
    build_guard_state,
    build_merge_summary,
    build_queue_summary,
    build_task_summary,
    build_todo_summary,
    build_worker_summary,
    collect_dirty_state,
    context_rot_score,
    current_plan_epoch_for_thread,
    emit_follow_up_request,
    ensure_runtime_scaffold,
    evaluate_task_drift_checklist,
    find_task,
    load_merge_queue,
    load_json,
    load_merge_queue,
    load_optional_json,
    load_policy_summary,
    load_worktree_registry,
    now_iso,
    priority_rank,
    process_merge_queue,
    reconcile_requests,
    request_bindings_for_task,
    task_requires_dedicated_worktree,
    update_binding_state,
    update_session_binding,
    upsert_worktree_registry_entry,
    write_log_index,
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


def tmux_session_running(session_name: str) -> bool:
    if not tmux_session_alive(session_name):
        return False
    try:
        result = subprocess.run(
            ["tmux", "list-panes", "-t", session_name, "-F", "#{pane_dead}"],
            capture_output=True,
            text=True,
            check=True,
        )
        return any(line.strip() == "0" for line in result.stdout.splitlines())
    except (subprocess.CalledProcessError, FileNotFoundError):
        return False


def has_live_runner_session(task: dict, heartbeats: dict) -> bool:
    task_id = task.get("taskId")
    if not task_id:
        return False
    heartbeat = heartbeats.get(task_id, {})
    tmux_name = task.get("claim", {}).get("tmuxSession") or heartbeat.get("tmuxSession")
    return bool(tmux_name and tmux_session_running(tmux_name))


def is_terminal_for_dispatch(task: dict) -> bool:
    status = (task.get("status") or "").lower()
    if status in {"completed", "verified", "pass", "skipped"}:
        return True
    verification_status = (task.get("verificationStatus") or "").lower()
    if verification_status in {"pass", "skipped"} and status in TASK_ACTIVE_STATUSES:
        return True
    return False


def is_recoverable_auto_dispatch_allowed(task: dict, heartbeats: dict) -> bool:
    if task.get("status") != "recoverable":
        return True
    if (task.get("verificationStatus") or "").lower() == "fail":
        return False
    if has_live_runner_session(task, heartbeats):
        return False
    return True


def compact_log_entry(log_index: dict, task_id: str):
    logs_by_task = log_index.get("logsByTaskId", {})
    if isinstance(logs_by_task, dict):
        return logs_by_task.get(task_id)
    return None


def evaluate_dispatch_preflight(task: dict, *, request_summary: dict, thread_state: dict, heartbeats: dict, log_index: dict, policy_summary: dict):
    compact_log = compact_log_entry(log_index, task.get("taskId"))
    rot = context_rot_score(task, request_summary, heartbeats, thread_state, compact_log, policy_summary)
    drift = evaluate_task_drift_checklist(
        task,
        latest_plan_epoch=rot.get("latestPlanEpoch"),
        request_summary=request_summary,
        compact_log=compact_log,
    )
    blocked = []
    warnings = []
    force_fresh = False
    if task.get("status") == "superseded":
        blocked.append("task superseded")
    if drift.get("status") == "fail":
        blocked.extend(drift.get("failures", []))
    if rot.get("status") == "downgraded":
        force_fresh = True
        warnings.append("context rot requires fresh execution session")
    elif rot.get("status") == "warning":
        warnings.append("context rot warning")
    if task.get("checkpointRequired"):
        blocked.append(task.get("checkpointReason") or "checkpoint required before dispatch")
    return {
        "ok": not blocked,
        "blocked": blocked,
        "warnings": warnings,
        "forceFresh": force_fresh,
        "contextRot": rot,
        "driftChecklist": drift,
    }


def task_sort_key(task: dict):
    priority = task.get("priority") or "P9"
    return (priority_rank(priority), task.get("taskId") or "")


def find_dispatchable_tasks(tasks: list[dict], preferred_task_ids: list[str] | None = None) -> list[dict]:
    completed_ids = {
        task["taskId"]
        for task in tasks
        if task.get("status") in TASK_COMPLETED_STATUSES
    }
    dispatchable = []
    for task in tasks:
        if task.get("status") not in {"queued", "worktree_prepared"}:
            continue
        if task.get("planningStage") != "execution-ready":
            continue
        if task.get("claim", {}).get("agentId"):
            continue
        if task.get("checkpointRequired"):
            continue
        if task.get("supersededByRequestId"):
            continue
        if task.get("status") in {"superseded", "finishing_then_pause"}:
            continue
        if not all(dep in completed_ids for dep in (task.get("dependsOn") or [])):
            continue
        dispatchable.append(task)
    preferred = preferred_task_ids or []
    dispatchable.sort(key=lambda task: ((task.get("taskId") not in preferred), *task_sort_key(task)))
    return dispatchable


def find_active_tasks(tasks: list[dict]) -> list[dict]:
    return [task for task in tasks if task.get("status") in TASK_ACTIVE_STATUSES]


def find_recoverable_tasks(tasks: list[dict], heartbeats: dict, stale_after_seconds: int) -> list[dict]:
    result = []
    for task in tasks:
        if task.get("status") == "recoverable":
            result.append(task)
            continue
        if task.get("status") not in TASK_ACTIVE_STATUSES:
            continue
        heartbeat = heartbeats.get(task["taskId"], {})
        tmux_name = task.get("claim", {}).get("tmuxSession") or heartbeat.get("tmuxSession")
        if tmux_name and not tmux_session_alive(tmux_name):
            result.append(task)
        elif not tmux_name and task.get("claim", {}).get("boundSessionId"):
            result.append(task)
        else:
            heartbeat_age = age_seconds(heartbeat.get("lastHeartbeatAt"))
            if heartbeat_age is not None and heartbeat_age > stale_after_seconds:
                result.append(task)
    return result


def make_tmux_session_name(task_id: str, project_name: str) -> str:
    safe = project_tmux_fragment(project_name)
    ts = datetime.now().strftime("%m%d-%H%M%S")
    return f"hr-{task_id}-{safe}-{ts}"


def project_tmux_fragment(project_name: str) -> str:
    return project_name[:20].replace(" ", "-")


def session_matches_project(session_name: str, project_name: str) -> bool:
    fragment = project_tmux_fragment(project_name)
    return session_name.startswith("hr-") and f"-{fragment}-" in session_name


def referenced_tmux_sessions(tasks: list[dict], heartbeats: dict) -> set[str]:
    sessions = set()
    for task in tasks:
        task_id = task.get("taskId")
        claim = task.get("claim", {}) if isinstance(task.get("claim"), dict) else {}
        heartbeat = heartbeats.get(task_id, {}) if task_id else {}
        tmux_name = claim.get("tmuxSession") or heartbeat.get("tmuxSession")
        if tmux_name and not str(tmux_name).startswith("print:"):
            sessions.add(tmux_name)
    return sessions


def gc_project_tmux_sessions(root: Path, tasks: list[dict], heartbeats: dict, preserve_sessions: set[str] | None = None) -> list[str]:
    preserve = {
        session
        for session in (preserve_sessions or set())
        if session and not str(session).startswith("print:")
    }
    preserve.update(
        session
        for session in referenced_tmux_sessions(tasks, heartbeats)
        if tmux_session_running(session)
    )
    killed = []
    for session in tmux_list_sessions():
        if not session_matches_project(session, root.name):
            continue
        if session in preserve:
            continue
        subprocess.run(["tmux", "kill-session", "-t", session], capture_output=True)
        if not tmux_session_alive(session):
            killed.append(session)
    return killed


def build_codex_command(task: dict, session_id: str | None, execution_cwd: str) -> list[str]:
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
    model = (
        task.get("executionModel")
        or (task.get("dispatch") or {}).get("executionModel")
        or "gpt-5.3-codex"
    )
    return [
        "codex",
        "exec",
        "--yolo",
        "--skip-git-repo-check",
        "-m",
        model,
        "-C",
        execution_cwd,
        "-",
    ]


def call_prepare_worktree(script_path: Path, project_root: Path, task_id: str) -> dict:
    result = subprocess.run(
        ["python3", str(script_path), "--root", str(project_root), "--task-id", task_id, "--create", "--write-back"],
        capture_output=True,
        text=True,
        check=True,
    )
    return json.loads(result.stdout)


def execution_cwd_for_task(project_root: Path, task: dict) -> Path:
    worktree_rel = task.get("worktreePath")
    if worktree_rel:
        worktree_path = (project_root / worktree_rel).resolve()
        if worktree_path.exists():
            return worktree_path
    return project_root


def environment_summary_for_task(task: dict) -> dict:
    reasons = []
    worktree_status = task.get("worktreeEnvironmentStatus")
    worktree_reason = task.get("worktreeEnvironmentReason")
    diff_status = task.get("diffSummaryStatus")
    diff_reason = task.get("diffSummaryReason")

    if worktree_status == "degraded":
        reasons.append(worktree_reason or "dedicated worktree is not ready")
    elif task.get("worktreeStatus") == "worktree_missing":
        reasons.append("dedicated worktree is missing")

    if diff_status == "degraded":
        reasons.append(diff_reason or "diff baseline is unavailable")

    deduped_reasons = []
    for item in reasons:
        if item and item not in deduped_reasons:
            deduped_reasons.append(item)

    return {
        "status": "degraded" if deduped_reasons else "ready",
        "reasons": deduped_reasons,
        "worktreeStatus": task.get("worktreeStatus"),
        "diffSummaryStatus": diff_status,
    }


def manual_run_guard_override_reasons(guard_state: dict) -> list[str]:
    blockers = list(guard_state.get("blockers") or [])
    if not blockers:
        return []
    allowed = {"unknown dirty worktree blocks automation"}
    if any(item not in allowed for item in blockers):
        return []
    return [item for item in blockers if item in allowed]


def session_binding_for_resume(task: dict, session_registry: dict, session_id: str | None) -> dict | None:
    if not session_id:
        return None
    for collection_name in ("activeBindings", "recoverableBindings"):
        for item in session_registry.get(collection_name, []):
            if item.get("sessionId") == session_id or item.get("lastKnownSessionId") == session_id:
                return item
    claim = task.get("claim", {})
    if claim.get("boundSessionId") == session_id:
        return {
            "taskId": task.get("taskId"),
            "sessionId": session_id,
            "worktreePath": claim.get("worktreePath"),
            "branchName": claim.get("branchName"),
        }
    return None


def resume_safe_for_task(task: dict, route_decision: dict, session_registry: dict) -> tuple[bool, str | None]:
    if route_decision.get("resumeStrategy") != "resume":
        return True, None
    session_id = route_decision.get("preferredResumeSessionId")
    binding = session_binding_for_resume(task, session_registry, session_id)
    if not binding:
        return True, None
    if binding.get("taskId") and binding.get("taskId") != task.get("taskId"):
        return False, "resume session belongs to a different task"
    if binding.get("worktreePath") and task.get("worktreePath") and binding.get("worktreePath") != task.get("worktreePath"):
        return False, "resume session/worktree binding drifted"
    if binding.get("branchName") and task.get("branchName") and binding.get("branchName") != task.get("branchName"):
        return False, "resume session/branch binding drifted"
    return True, None


def call_route_session(script_path: Path, project_root: Path, task_id: str) -> dict:
    result = subprocess.run(
        ["python3", str(script_path), "--root", str(project_root), "--task-id", task_id, "--write-back"],
        capture_output=True,
        text=True,
        check=True,
    )
    return json.loads(result.stdout)


def bound_requests_for_task(project_root: str, task_id: str) -> list[dict]:
    root = Path(project_root)
    request_index = load_optional_json(root / ".harness" / "state" / "request-index.json", {}) or {}
    task_map = load_optional_json(root / ".harness" / "state" / "request-task-map.json", {}) or {}
    requests_by_id = {
        request.get("requestId"): request
        for request in request_index.get("requests", [])
        if request.get("requestId")
    }
    related = []
    for binding in task_map.get("bindings", []):
        if binding.get("taskId") != task_id:
            continue
        request = requests_by_id.get(binding.get("requestId"))
        if request:
            related.append(request)
    related.sort(key=lambda item: (item.get("seq", 0), item.get("createdAt") or ""))
    return related


def prompt_lines(task: dict, route_decision: dict, project_root: str, execution_cwd: str):
    bound_requests = bound_requests_for_task(project_root, task.get("taskId"))
    lines = [
        "使用 klein-harness skill。",
        f"当前项目目录: {project_root}",
        f"执行目录: {execution_cwd}",
        f"当前任务: {task.get('taskId')}",
        f"title: {task.get('title', '-')}",
        f"summary: {task.get('summary', '-')}",
        f"roleHint: {task.get('roleHint', '-')}",
        f"threadKey: {task.get('threadKey', '-')}",
        f"planEpoch: {task.get('planEpoch', '-')}",
        f"branchName: {task.get('branchName', '-')}",
        f"worktreePath: {task.get('worktreePath', '-')}",
        f"integrationBranch: {task.get('integrationBranch', task.get('dispatch', {}).get('integrationBranch', '-'))}",
        f"resumeStrategy: {route_decision.get('resumeStrategy', 'fresh')}",
        f"routingMode: {route_decision.get('routingMode')}",
        f"gateReason: {route_decision.get('gateReason')}",
        f"promptStages: {route_decision.get('promptStages', [])}",
        "",
    ]
    if bound_requests:
        lines.extend([
            "关联请求：",
            *[
                f"- {request.get('requestId')} [{request.get('kind')}/{request.get('status')}] {request.get('goal')}"
                for request in bound_requests[-2:]
            ],
            "",
        ])
    lines.extend([
        "请读取 .harness/state/progress.json、.harness/state/todo-summary.json、.harness/state/completion-gate.json、.harness/state/guard-state.json、.harness/task-pool.json、.harness/session-registry.json。",
        "如果存在 .harness/state/feedback-summary.json，只读取当前 task 最近 3 条高严重度失败。",
        "如果存在 .harness/state/task-summary.json 或 .harness/state/worker-summary.json，先读这些 compact summaries。",
        "如果存在相关 .harness/log-<taskId>.md，默认先读 compact log，不要先扫其他 task 的 raw runner log。",
        "检索顺序默认是：current/runtime/progress/todo-summary/completion-gate/guard-state/request-summary/lineage-index -> task/worker/log summaries -> compact log md -> raw runner log。",
        f"找到任务 {task.get('taskId')}，按其 ownedPaths 和 verificationRuleIds 执行。",
        "执行完成后回写 task-pool.json、state/progress.json、lineage.jsonl、session-registry.json。",
        "progress.md 是由 JSON 派生的人类视图，不要把 Markdown 当机器状态源。",
        "最后额外输出一个很小的 fenced JSON block，格式如下：",
        "```KLEIN_HANDOFF_JSON",
        '{"oneScreenSummary":[],"crossWorkerFacts":[],"decisionsAssumptions":[],"touchedContractsPaths":[],"blockersRisks":[],"verification":[],"openQuestions":[],"tags":[],"evidenceRefs":[]}',
        "```",
        "这个 block 只写 cross-worker relevant facts，不要泄露隐藏推理，不要粘贴大段文件内容。",
    ])
    if task.get("roleHint") == "orchestrator" or task.get("workerMode") in {"audit", "orchestrator"}:
        lines.extend([
            "",
            "控制面约束：",
            "- 什么算待做：只处理当前线程、当前 plan epoch、未 terminal、未 supersede、未被 completion gate 关闭的 actionable todo。",
            "- 什么情况下允许自动改代码：只有 control-plane 明确放行且 guard-state safeToExecute=true 时，才允许后续 worker 自动改业务代码。",
            "- 自动改完后谁来提交/推送：worker 负责本地修改与验证；是否提交/推送遵循 task 和 harness policy，默认不要替 operator 偷推远端。",
            "- 出错、超时、脏工作区怎么处理：先区分 prompt 问题还是 harness 系统问题；保留证据链；不要绕过 spec 偷跑；unknown dirty 默认阻断自动化。",
            "- 怎么知道真的完成：不能只看 exit code，必须同时看 verification、lineage/evidence、completion-gate、todo-summary 与 request/task 状态是否闭环。",
            "- topic drift：如果发现范围漂移、蓝图失效、或当前方案需要重新拆解，先收集证据，再决定发起 audit / replan / blueprint / stop follow-up，不要把新主题直接混进当前 task。",
            "- blueprint / replan 只在计划边界真的变化时触发；轻微实现细节或局部证据补充优先留在当前 task 内处理。",
            "- 如果观察到行为与这些要点不一致，先判定是提示词缺口还是 harness 系统缺口，再决定写结论还是继续修 .harness 控制面。",
            "- 除非 task 明确允许，不要直接修改业务源码；优先修 harness/spec/流程问题。",
        ])
    for failure in route_decision.get("recentFailures", [])[:3]:
        lines.append(
            f"- recentFailure: {failure.get('feedbackType')} [{failure.get('severity')}] {failure.get('message')}"
        )
    return lines


def dispatch_task(root: Path, task: dict, route_decision: dict, project_name: str, state_dir: Path, log_dir: Path, dispatch_mode: str) -> dict:
    task_id = task["taskId"]
    prepare_py = state_dir.parent / "scripts" / "prepare-worktree.py"
    prepared_worktree = None
    if task_requires_dedicated_worktree(task):
        prepared_worktree = call_prepare_worktree(prepare_py, root, task_id)
        task = find_task(load_json(state_dir.parent / "task-pool.json").get("tasks", []), task_id)
    session_id = route_decision.get("preferredResumeSessionId") if route_decision.get("resumeStrategy") == "resume" else None
    tmux_name = make_tmux_session_name(task_id, project_name) if dispatch_mode == "tmux" else f"print:{task_id}"
    dispatch_backend = dispatch_mode
    log_path = log_dir / f"{task_id}.log"
    prompt_path = state_dir / f"runner-prompt-{task_id}.md"
    runner_script = state_dir / f"runner-exec-{task_id}.sh"
    execution_cwd = execution_cwd_for_task(root, task)
    prompt_path.write_text("\n".join(prompt_lines(task, route_decision, str(root), str(execution_cwd))) + "\n")

    codex_cmd = build_codex_command(task, session_id, str(execution_cwd))
    runner_py = Path(__file__).resolve()
    refresh_state_py = state_dir.parent / "scripts" / "refresh-state.py"
    diff_summary_py = state_dir.parent / "scripts" / "diff-summary.py"
    verify_task_py = state_dir.parent / "scripts" / "verify-task.py"
    heartbeat_cmd = f'python3 "{runner_py}" heartbeat "{root}" "{task_id}" "{tmux_name}"'
    finalize_cmd = (
        f'python3 "{runner_py}" finalize "{root}" "{task_id}" '
        f'--tmux-session "{tmux_name}" --runner-status "$runner_status"'
    )
    runner_preamble = [
        f'echo "[runner] task={task_id} started at $(date \'+%Y-%m-%d %H:%M:%S\')"',
        f'echo "[runner] executionCwd={execution_cwd}"',
        f'echo "[runner] worktreePath={task.get("worktreePath") or "."}"',
    ]
    if prepared_worktree:
        runner_preamble.append(f'echo "[runner] worktreeStatus={prepared_worktree.get("status")}"')
        if prepared_worktree.get("environmentStatus") == "degraded":
            runner_preamble.append(
                f'echo "[runner] environmentStatus={prepared_worktree.get("environmentStatus")} reason={prepared_worktree.get("environmentReason")}"'
            )
    runner_preamble.append(
        f'echo "[runner] integrationBranch={task.get("integrationBranch") or task.get("dispatch", {}).get("integrationBranch") or "-"}"'
    )

    runner_script.write_text(
        f"#!/usr/bin/env bash\n"
        f"set -euo pipefail\n"
        f'cd "{execution_cwd}"\n'
        f'exec > >(tee -a "{log_path}") 2>&1\n'
        f'trap \'status=$?; trap - EXIT; {heartbeat_cmd} --phase exited --exit-code \"$status\" || true; exit \"$status\"\' EXIT\n'
        + "\n".join(runner_preamble)
        + "\n"
        f'{heartbeat_cmd} --phase running\n'
        f'runner_status=0\n'
        f'{" ".join(codex_cmd)} < "{prompt_path}" || runner_status=$?\n'
        f'if [[ -f "{diff_summary_py}" ]]; then\n'
        f'  python3 "{diff_summary_py}" --root "{root}" --task-id "{task_id}" --write-back || echo "[runner] diff-summary failed"\n'
        f'fi\n'
        f'if [[ -f "{verify_task_py}" ]]; then\n'
        f'  python3 "{verify_task_py}" --root "{root}" --task-id "{task_id}" --write-back || runner_status=$?\n'
        f'fi\n'
        f'{finalize_cmd} || true\n'
        f'if [[ -f "{refresh_state_py}" ]]; then\n'
        f'  python3 "{refresh_state_py}" "{root}" || echo "[runner] refresh-state failed"\n'
        f'fi\n'
        f'exit "$runner_status"\n'
    )
    runner_script.chmod(0o755)

    if dispatch_mode == "print":
        log_path.write_text(
            "\n".join(
                [
                    f"[runner-preview] task={task_id}",
                    f"[runner-preview] dispatchMode={dispatch_mode}",
                    f"[runner-preview] dispatchBackend={dispatch_backend}",
                    f"[runner-preview] tmuxSession={tmux_name}",
                    f"[runner-preview] resumeStrategy={route_decision.get('resumeStrategy', 'fresh')}",
                    f"[runner-preview] gateReason={route_decision.get('gateReason')}",
                    f"[runner-preview] executionCwd={execution_cwd}",
                    f"[runner-preview] worktreePrepared={json.dumps(prepared_worktree, ensure_ascii=False) if prepared_worktree else 'null'}",
                    f"[runner-preview] promptPath={prompt_path}",
                    f"[runner-preview] ownedPaths={json.dumps(task.get('ownedPaths', []), ensure_ascii=False)}",
                    "",
                    prompt_path.read_text(encoding='utf-8', errors='ignore'),
                ]
            ).rstrip()
            + "\n",
            encoding="utf-8",
        )

    if dispatch_mode == "tmux":
        subprocess.run(["tmux", "new-session", "-d", "-s", tmux_name, f"bash {runner_script}"], check=True)
        subprocess.run(["tmux", "set-option", "-t", tmux_name, "remain-on-exit", "off"], capture_output=True)

    return {
        "taskId": task_id,
        "dispatchMode": dispatch_mode,
        "dispatchBackend": dispatch_backend,
        "tmuxSession": tmux_name,
        "strategy": route_decision.get("resumeStrategy", "fresh"),
        "sessionId": session_id,
        "executionCwd": str(execution_cwd),
        "logPath": str(log_path),
        "promptPath": str(prompt_path),
        "runnerScriptPath": str(runner_script),
        "dispatchedAt": now_iso(),
        "gateReason": route_decision.get("gateReason"),
        "routeDecision": route_decision,
        "preparedWorktree": prepared_worktree,
        "environment": environment_summary_for_task(task),
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


def emit_result_with_refresh(root: Path, payload: dict, exit_code: int = 0) -> int:
    refresh_ok, refresh_error = refresh_hot_state(root)
    if refresh_ok:
        payload["refreshState"] = {"ok": True}
    else:
        payload["refreshState"] = {"ok": False, "error": refresh_error}
    print(json.dumps(payload, ensure_ascii=False, indent=2))
    return exit_code


def write_heartbeat(state_dir: Path, task_id: str, tmux_session: str, phase: str = "running", exit_code: int | None = None):
    hb_path = state_dir / "runner-heartbeats.json"
    hb = load_optional_json(hb_path, {"schemaVersion": "1.0", "generator": "harness-runner", "generatedAt": now_iso(), "entries": {}})
    hb.setdefault("entries", {})[task_id] = {
        "taskId": task_id,
        "tmuxSession": tmux_session,
        "backend": "print" if tmux_session.startswith("print:") else "tmux",
        "backendSession": tmux_session,
        "lastHeartbeatAt": now_iso(),
        "lastKnownPhase": phase,
        "lastExitCode": exit_code,
    }
    hb["generatedAt"] = now_iso()
    write_json(hb_path, hb)


def daemon_paths(state_dir: Path) -> dict:
    return {
        "state": state_dir / "runner-daemon.json",
        "log": state_dir / "runner-daemon.log",
        "session": state_dir / "runner-daemon-tmux-session.txt",
        "script": state_dir / "runner-daemon.sh",
    }


def load_daemon_state(state_dir: Path) -> dict:
    return load_optional_json(
        daemon_paths(state_dir)["state"],
        {
            "schemaVersion": "1.0",
            "generator": "harness-runner",
            "generatedAt": now_iso(),
            "status": "stopped",
            "sessionName": None,
            "intervalSeconds": None,
            "dispatchMode": None,
            "lastTickAt": None,
            "lastRefreshAt": None,
            "lastTickResult": None,
            "lastError": None,
            "logPath": None,
            "restartCount": 0,
            "recentEvents": [],
        },
    )


def write_daemon_state(
    state_dir: Path,
    *,
    status: str,
    session_name: str | None,
    interval: int | None,
    dispatch_mode: str | None,
    log_path: Path | None,
    last_tick_at: str | None = None,
    last_refresh_at: str | None = None,
    last_tick_result: dict | None = None,
    last_error: str | None = None,
):
    current = load_daemon_state(state_dir)
    previous_status = current.get("status")
    current.update(
        {
            "schemaVersion": "1.0",
            "generator": "harness-runner",
            "generatedAt": now_iso(),
            "status": status,
            "sessionName": session_name,
            "intervalSeconds": interval,
            "dispatchMode": dispatch_mode,
            "logPath": str(log_path) if log_path else None,
            "lastTickAt": last_tick_at if last_tick_at is not None else current.get("lastTickAt"),
            "lastRefreshAt": last_refresh_at if last_refresh_at is not None else current.get("lastRefreshAt"),
            "lastTickResult": last_tick_result if last_tick_result is not None else current.get("lastTickResult"),
            "lastError": last_error,
        }
    )
    recent_events = list(current.get("recentEvents", []))
    recent_events.append({
        "status": status,
        "sessionName": session_name,
        "dispatchMode": dispatch_mode,
        "timestamp": now_iso(),
        "lastError": last_error,
    })
    current["recentEvents"] = recent_events[-10:]
    if status == "running" and previous_status != "running":
        current["restartCount"] = int(current.get("restartCount", 0)) + 1
    write_json(daemon_paths(state_dir)["state"], current)


def resolve_daemon_session(state_dir: Path) -> str | None:
    session_path = daemon_paths(state_dir)["session"]
    if not session_path.exists():
        return None
    session_name = session_path.read_text().strip()
    return session_name or None


def claim_dispatched_task(root: Path, task_id: str, run: dict, *, lifecycle_status: str):
    files = ensure_runtime_scaffold(root, generator="harness-runner")
    task_pool_path = files["harness"] / "task-pool.json"
    task_pool = load_json(task_pool_path)
    task = find_task(task_pool.get("tasks", []), task_id)
    claim = task.setdefault("claim", {})
    now = now_iso()
    if task.get("status") in {"queued", "worktree_prepared"}:
        task["status"] = "claimed"
    claim["agentId"] = "harness-runner"
    claim["role"] = task.get("roleHint") or claim.get("role")
    claim["nodeId"] = f"tmux:{run['tmuxSession']}" if run.get("dispatchBackend") == "tmux" else "dispatch:print"
    claim["tmuxSession"] = run.get("tmuxSession")
    claim["dispatchBackend"] = run.get("dispatchBackend") or run.get("dispatchMode")
    claim["boundSessionId"] = run.get("sessionId") or claim.get("boundSessionId")
    claim["boundResumeStrategy"] = run.get("strategy")
    claim["worktreePath"] = task.get("worktreePath")
    claim["branchName"] = task.get("branchName")
    claim["baseRef"] = task.get("baseRef") or (task.get("dispatch") or {}).get("baseRef")
    claim["integrationBranch"] = task.get("integrationBranch") or (task.get("dispatch") or {}).get("integrationBranch")
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
        worktree_path=task.get("worktreePath"),
        branch_name=task.get("branchName"),
        base_ref=task.get("baseRef") or (task.get("dispatch") or {}).get("baseRef"),
        integration_branch=claim.get("integrationBranch"),
        generator="harness-runner",
    )
    if task.get("worktreePath"):
        upsert_worktree_registry_entry(
            root,
            task,
            generator="harness-runner",
            status=lifecycle_status,
            extra={
                "dispatchBackend": claim.get("dispatchBackend"),
                "nodeId": claim.get("nodeId"),
                "tmuxSession": run.get("tmuxSession"),
                "executionCwd": run.get("executionCwd"),
            },
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


def global_guard_state(root: Path, files: dict, *, task_pool: dict, session_registry: dict, heartbeats: dict, runner_state: dict, request_summary: dict, policy_summary: dict):
    queue_summary = build_queue_summary(load_json(files["request_index_path"]), request_summary, generator="harness-runner", policy_summary=policy_summary)
    feedback_summary = load_optional_json(files["feedback_summary_path"], {})
    lineage_index = load_optional_json(files["lineage_index_path"], {})
    worker_summary = build_worker_summary(task_pool, session_registry, runner_state, heartbeats, generator="harness-runner", policy_summary=policy_summary)
    daemon_state = load_daemon_state(files["state_dir"])
    daemon_summary = load_optional_json(files["daemon_summary_path"], {})
    if not daemon_summary:
        daemon_summary = build_daemon_summary(daemon_state, runner_state, worker_summary, generator="harness-runner", policy_summary=policy_summary)
    task_summary = build_task_summary(task_pool, feedback_summary, lineage_index, runner_state, generator="harness-runner", policy_summary=policy_summary)
    merge_queue = load_merge_queue(files, generator="harness-runner")
    merge_summary = load_optional_json(files["merge_summary_path"], {})
    if not merge_summary:
        merge_summary = build_merge_summary(merge_queue, load_worktree_registry(files, generator="harness-runner"), generator="harness-runner")
    dirty_state = collect_dirty_state(root, task_pool, policy_summary)
    worktree_registry = apply_dirty_state_to_worktree_registry(load_worktree_registry(files, generator="harness-runner"), dirty_state, generator="harness-runner")
    todo_summary = build_todo_summary(task_pool, queue_summary, request_summary, merge_summary, dirty_state, generator="harness-runner", policy_summary=policy_summary)
    completion_gate = build_completion_gate(
        load_optional_json(files["harness"] / "spec.json", {}),
        load_optional_json(files["harness"] / "features.json", {}),
        task_pool,
        request_summary,
        merge_summary,
        feedback_summary,
        todo_summary,
        load_optional_json(files["project_meta_path"], {}),
        generator="harness-runner",
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
        generator="harness-runner",
        policy_summary=policy_summary,
    )
    return guard_state


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
    policy_summary = load_policy_summary(files["policy_summary_path"], default_generator="harness-runner")
    reconcile_requests(root, generator="harness-runner")
    state_dir = files["state_dir"]
    log_dir = state_dir / "runner-logs"
    task_pool = load_json(files["harness"] / "task-pool.json")
    route_session_py = state_dir.parent / "scripts" / "route-session.py"
    tasks = task_pool.get("tasks", [])
    heartbeats = load_heartbeats(state_dir)
    session_registry = load_json(files["session_registry_path"])
    request_summary = load_optional_json(files["request_summary_path"], {})
    thread_state = load_optional_json(files["thread_state_path"], {})
    log_index = load_optional_json(files["log_index_path"], {})
    prior_runner_state = load_optional_json(files["runner_state_path"], {})

    process_merge_queue(root, generator="harness-runner")
    task_pool = load_json(files["harness"] / "task-pool.json")
    tasks = task_pool.get("tasks", [])
    daemon_session = resolve_daemon_session(state_dir)
    tmux_gc = gc_project_tmux_sessions(
        root,
        tasks,
        heartbeats,
        preserve_sessions={daemon_session} if daemon_session else None,
    )
    guard_state = global_guard_state(
        root,
        files,
        task_pool=task_pool,
        session_registry=session_registry,
        heartbeats=heartbeats,
        runner_state=prior_runner_state,
        request_summary=request_summary,
        policy_summary=policy_summary,
    )

    active = find_active_tasks(tasks)
    recoverable = find_recoverable_tasks(tasks, heartbeats, policy_summary["heartbeat"]["recoverableAfterSeconds"])
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
            if is_terminal_for_dispatch(task) or not is_recoverable_auto_dispatch_allowed(task, heartbeats):
                continue
            if not guard_state.get("safeToExecute"):
                blocked_routes.append({"taskId": task["taskId"], "gateStatus": "blocked", "gateReason": "; ".join(guard_state.get("blockers", []))})
                continue
            preflight = evaluate_dispatch_preflight(
                task,
                request_summary=request_summary,
                thread_state=thread_state,
                heartbeats=heartbeats,
                log_index=log_index,
                policy_summary=policy_summary,
            )
            if not preflight.get("ok"):
                blocked_routes.append({"taskId": task["taskId"], "gateStatus": "blocked", "gateReason": "; ".join(preflight.get("blocked", []))})
                continue
            route_decision = call_route_session(route_session_py, root, task["taskId"])
            if preflight.get("forceFresh") and route_decision.get("resumeStrategy") == "resume":
                route_decision["resumeStrategy"] = "fresh"
                route_decision["preferredResumeSessionId"] = None
                route_decision["gateReason"] = f"{route_decision.get('gateReason')}; context rot forced fresh session"
            resume_ok, resume_reason = resume_safe_for_task(task, route_decision, session_registry)
            if not resume_ok and route_decision.get("resumeStrategy") == "resume":
                route_decision["resumeStrategy"] = "fresh"
                route_decision["preferredResumeSessionId"] = None
                route_decision["gateReason"] = f"{route_decision.get('gateReason')}; {resume_reason}"
            if not route_decision.get("dispatchReady"):
                blocked_routes.append({"taskId": task["taskId"], "gateStatus": route_decision.get("gateStatus"), "gateReason": route_decision.get("gateReason")})
                emit_blocked_follow_up(root, task, route_decision)
                continue
            run = dispatch_task(root, task, route_decision, root.name, state_dir, log_dir, dispatch_mode)
            claim_dispatched_task(root, task["taskId"], run, lifecycle_status="resumed")
            write_heartbeat(state_dir, task["taskId"], run["tmuxSession"], phase="resumed")
            dispatched.append(run)
        except Exception as exc:
            errors.append({"taskId": task["taskId"], "error": str(exc)})

    if not live_runs and not dispatched and dispatchable:
        task = dispatchable[0]
        try:
            if not guard_state.get("safeToExecute"):
                blocked_routes.append({"taskId": task["taskId"], "gateStatus": "blocked", "gateReason": "; ".join(guard_state.get("blockers", []))})
                write_runner_state(
                    state_dir,
                    trigger,
                    active_runs=live_runs,
                    recoverable_runs=[{"taskId": item["taskId"]} for item in recoverable],
                    stale_runs=stale_runs,
                    dispatchable_ids=[candidate.get("taskId") for candidate in dispatchable[1:]],
                    errors=errors,
                    blocked_routes=blocked_routes,
                )
                return emit_result_with_refresh(root, {"ok": True, "liveRuns": len(live_runs), "dispatched": [], "recoverable": len(recoverable), "dispatchable": len(dispatchable), "blocked": blocked_routes, "errors": errors, "guardState": guard_state, "tmuxGc": tmux_gc})
            preflight = evaluate_dispatch_preflight(
                task,
                request_summary=request_summary,
                thread_state=thread_state,
                heartbeats=heartbeats,
                log_index=log_index,
                policy_summary=policy_summary,
            )
            if not preflight.get("ok"):
                blocked_routes.append({"taskId": task["taskId"], "gateStatus": "blocked", "gateReason": "; ".join(preflight.get("blocked", []))})
                write_runner_state(
                    state_dir,
                    trigger,
                    active_runs=live_runs,
                    recoverable_runs=[{"taskId": item["taskId"]} for item in recoverable],
                    stale_runs=stale_runs,
                    dispatchable_ids=[candidate.get("taskId") for candidate in dispatchable[1:]],
                    errors=errors,
                    blocked_routes=blocked_routes,
                )
                return emit_result_with_refresh(root, {"ok": True, "liveRuns": len(live_runs), "dispatched": [], "recoverable": len(recoverable), "dispatchable": len(dispatchable), "blocked": blocked_routes, "errors": errors, "tmuxGc": tmux_gc})
            route_decision = call_route_session(route_session_py, root, task["taskId"])
            if preflight.get("forceFresh") and route_decision.get("resumeStrategy") == "resume":
                route_decision["resumeStrategy"] = "fresh"
                route_decision["preferredResumeSessionId"] = None
                route_decision["gateReason"] = f"{route_decision.get('gateReason')}; context rot forced fresh session"
            resume_ok, resume_reason = resume_safe_for_task(task, route_decision, session_registry)
            if not resume_ok and route_decision.get("resumeStrategy") == "resume":
                route_decision["resumeStrategy"] = "fresh"
                route_decision["preferredResumeSessionId"] = None
                route_decision["gateReason"] = f"{route_decision.get('gateReason')}; {resume_reason}"
            if not route_decision.get("dispatchReady"):
                blocked_routes.append({"taskId": task["taskId"], "gateStatus": route_decision.get("gateStatus"), "gateReason": route_decision.get("gateReason")})
                emit_blocked_follow_up(root, task, route_decision)
            else:
                run = dispatch_task(root, task, route_decision, root.name, state_dir, log_dir, dispatch_mode)
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
        "guardState": guard_state,
        "tmuxGc": tmux_gc,
        "errors": errors,
    }
    return emit_result_with_refresh(root, result, 0 if not errors else 1)


def cmd_run(root: Path, task_id: str, trigger: str = "shell", dispatch_mode: str = "tmux", lifecycle_status: str = "dispatched"):
    files = ensure_runtime_scaffold(root, generator="harness-runner")
    policy_summary = load_policy_summary(files["policy_summary_path"], default_generator="harness-runner")
    reconcile_requests(root, generator="harness-runner")
    state_dir = files["state_dir"]
    log_dir = state_dir / "runner-logs"
    task_pool = load_json(files["harness"] / "task-pool.json")
    route_session_py = state_dir.parent / "scripts" / "route-session.py"
    task = find_task(task_pool.get("tasks", []), task_id)
    request_summary = load_optional_json(files["request_summary_path"], {})
    thread_state = load_optional_json(files["thread_state_path"], {})
    heartbeats = load_heartbeats(state_dir)
    log_index = load_optional_json(files["log_index_path"], {})
    session_registry = load_json(files["session_registry_path"])
    guard_override_reasons = []
    tmux_gc: list[str] = []

    try:
        process_merge_queue(root, generator="harness-runner")
        task_pool = load_json(files["harness"] / "task-pool.json")
        daemon_session = resolve_daemon_session(state_dir)
        tmux_gc = gc_project_tmux_sessions(
            root,
            task_pool.get("tasks", []),
            heartbeats,
            preserve_sessions={daemon_session} if daemon_session else None,
        )
        task = find_task(task_pool.get("tasks", []), task_id)
        guard_state = global_guard_state(
            root,
            files,
            task_pool=task_pool,
            session_registry=session_registry,
            heartbeats=heartbeats,
            runner_state=load_optional_json(files["runner_state_path"], {}),
            request_summary=request_summary,
            policy_summary=policy_summary,
        )
        if has_live_runner_session(task, heartbeats):
            return emit_result_with_refresh(root, {
                "ok": True,
                "taskId": task_id,
                "status": task.get("status"),
                "verificationStatus": task.get("verificationStatus"),
                "note": "dispatch skipped: live runner session exists",
                "tmuxGc": tmux_gc,
            })
        if is_terminal_for_dispatch(task):
            return emit_result_with_refresh(root, {
                "ok": True,
                "taskId": task_id,
                "status": task.get("status"),
                "verificationStatus": task.get("verificationStatus"),
                "note": "dispatch skipped: task already terminal by current task-pool state",
                "tmuxGc": tmux_gc,
            })
        if not guard_state.get("safeToExecute"):
            guard_override_reasons = manual_run_guard_override_reasons(guard_state)
            if not guard_override_reasons:
                return emit_result_with_refresh(root, {"ok": False, "taskId": task_id, "gateStatus": "blocked", "gateReason": "; ".join(guard_state.get("blockers", [])), "guardState": guard_state, "tmuxGc": tmux_gc}, 1)
        preflight = evaluate_dispatch_preflight(
            task,
            request_summary=request_summary,
            thread_state=thread_state,
            heartbeats=heartbeats,
            log_index=log_index,
            policy_summary=policy_summary,
        )
        if not preflight.get("ok"):
            return emit_result_with_refresh(root, {"ok": False, "taskId": task_id, "gateStatus": "blocked", "gateReason": "; ".join(preflight.get("blocked", [])), "tmuxGc": tmux_gc}, 1)
        route_decision = call_route_session(route_session_py, root, task_id)
        if guard_override_reasons:
            prior_reason = route_decision.get("gateReason")
            override_note = f"manual run override: {'; '.join(guard_override_reasons)}"
            route_decision["gateReason"] = f"{prior_reason}; {override_note}" if prior_reason else override_note
        if preflight.get("forceFresh") and route_decision.get("resumeStrategy") == "resume":
            route_decision["resumeStrategy"] = "fresh"
            route_decision["preferredResumeSessionId"] = None
            route_decision["gateReason"] = f"{route_decision.get('gateReason')}; context rot forced fresh session"
        resume_ok, resume_reason = resume_safe_for_task(task, route_decision, session_registry)
        if not resume_ok and route_decision.get("resumeStrategy") == "resume":
            route_decision["resumeStrategy"] = "fresh"
            route_decision["preferredResumeSessionId"] = None
            route_decision["gateReason"] = f"{route_decision.get('gateReason')}; {resume_reason}"
        if not route_decision.get("dispatchReady"):
            emit_blocked_follow_up(root, task, route_decision)
            return emit_result_with_refresh(root, {
                "ok": False,
                "taskId": task_id,
                "gateStatus": route_decision.get("gateStatus"),
                "gateReason": route_decision.get("gateReason"),
                "needsOrchestrator": route_decision.get("needsOrchestrator"),
                "tmuxGc": tmux_gc,
            }, 1)
        run = dispatch_task(root, task, route_decision, root.name, state_dir, log_dir, dispatch_mode)
        claim_dispatched_task(root, task_id, run, lifecycle_status=lifecycle_status)
        write_heartbeat(state_dir, task_id, run["tmuxSession"], phase=lifecycle_status)
        return emit_result_with_refresh(root, {"ok": True, "dispatched": run, "guardOverrideReasons": guard_override_reasons, "tmuxGc": tmux_gc})
    except Exception as exc:
        return emit_result_with_refresh(root, {"ok": False, "error": str(exc), "tmuxGc": tmux_gc}, 1)


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
    request_summary = load_optional_json(files["request_summary_path"], {})
    thread_state = load_optional_json(files["thread_state_path"], {})
    log_index = load_optional_json(files["log_index_path"], {})
    pre_finalize = evaluate_task_drift_checklist(
        task,
        latest_plan_epoch=current_plan_epoch_for_thread(thread_state, task.get("threadKey")),
        request_summary=request_summary,
        compact_log=compact_log_entry(log_index, task_id),
    )
    failure_class = None

    if runner_status == 0 and verification_status in {"pass", "skipped", None} and pre_finalize.get("status") == "pass":
        merge_required = bool(task.get("mergeRequired") or task.get("handoff", {}).get("mergeRequired"))
        if merge_required:
            merge_result = process_merge_queue(root, task_id=task_id, generator="harness-runner")
            task_pool = load_json(task_pool_path)
            task = find_task(task_pool.get("tasks", []), task_id)
        else:
            task["status"] = "completed"
            task["completedAt"] = now_iso()
            write_json(task_pool_path, task_pool)
        final_session_status = "completed" if task.get("status") == "completed" else task.get("status")
        update_session_binding(
            root,
            task_id=task_id,
            session_id=session_id,
            node_id=task.get("claim", {}).get("nodeId"),
            status=final_session_status,
            worktree_path=task.get("worktreePath"),
            branch_name=task.get("branchName"),
            base_ref=task.get("baseRef") or (task.get("dispatch") or {}).get("baseRef"),
            integration_branch=task.get("integrationBranch") or (task.get("dispatch") or {}).get("integrationBranch"),
            generator="harness-runner",
        )
        if task.get("worktreePath"):
            upsert_worktree_registry_entry(
                root,
                task,
                generator="harness-runner",
                status=task.get("status"),
                cleanup_status=task.get("cleanupStatus"),
                extra={
                    "mergeStatus": task.get("mergeStatus"),
                    "mergedCommit": task.get("mergedCommit"),
                    "conflictPaths": task.get("conflictPaths", []),
                    "tmuxSession": tmux_session,
                },
            )
        if not merge_required or task.get("status") == "completed":
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
        if runner_status != 0:
            failure_class = "dispatch_failure"
        elif verification_status == "fail":
            failure_class = "verification_failure"
        else:
            failure_class = "finalize_guard_failure"
        task["status"] = "recoverable"
        task["recoverableAt"] = now_iso()
        if pre_finalize.get("status") == "fail":
            task["recoveryReason"] = "; ".join(pre_finalize.get("failures", []))
        else:
            task["recoveryReason"] = "verification failed" if verification_status == "fail" else f"runner exited with code {runner_status}"
        write_json(task_pool_path, task_pool)
        update_session_binding(
            root,
            task_id=task_id,
            session_id=session_id,
            node_id=task.get("claim", {}).get("nodeId"),
            status="recoverable",
            error=task.get("recoveryReason"),
            worktree_path=task.get("worktreePath"),
            branch_name=task.get("branchName"),
            base_ref=task.get("baseRef") or (task.get("dispatch") or {}).get("baseRef"),
            integration_branch=task.get("integrationBranch") or (task.get("dispatch") or {}).get("integrationBranch"),
            generator="harness-runner",
        )
        if task.get("worktreePath"):
            upsert_worktree_registry_entry(
                root,
                task,
                generator="harness-runner",
                status="recoverable",
                cleanup_status=task.get("cleanupStatus"),
                extra={
                    "recoveryReason": task.get("recoveryReason"),
                    "tmuxSession": tmux_session,
                },
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

    refreshed_task_pool = load_json(task_pool_path)
    refreshed_task = find_task(refreshed_task_pool.get("tasks", []), task_id)
    refreshed_task_map = load_json(files["request_task_map_path"])
    refreshed_bindings = request_bindings_for_task(refreshed_task_map, task_id)
    primary_binding = refreshed_bindings[-1] if refreshed_bindings else None
    route_decision = (primary_binding or {}).get("route")
    refreshed_environment = environment_summary_for_task(refreshed_task)
    compact_log = build_compact_log_artifact(
        root,
        refreshed_task,
        primary_binding,
        tmux_session,
        route_decision,
        generator="harness-runner",
    )
    write_log_index(root, generator="harness-runner")
    write_heartbeat(files["state_dir"], task_id, tmux_session or f"print:{task_id}", phase=refreshed_task.get("status"), exit_code=runner_status)
    print(
        json.dumps(
            {
                "ok": True,
                "taskId": task_id,
                "finalStatus": refreshed_task.get("status"),
                "mergeStatus": refreshed_task.get("mergeStatus"),
                "verificationStatus": verification_status,
                "failureClass": failure_class,
                "environmentStatus": refreshed_environment.get("status"),
                "environmentReasons": refreshed_environment.get("reasons"),
                "worktreeStatus": refreshed_environment.get("worktreeStatus"),
                "diffSummaryStatus": refreshed_environment.get("diffSummaryStatus"),
                "compactLogPath": str(compact_log["path"].relative_to(root)),
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
        update_session_binding(
            root,
            task_id=task_id,
            session_id=session_id,
            node_id=task.get("claim", {}).get("nodeId"),
            status="running",
            worktree_path=task.get("worktreePath"),
            branch_name=task.get("branchName"),
            base_ref=task.get("baseRef") or (task.get("dispatch") or {}).get("baseRef"),
            integration_branch=task.get("integrationBranch") or (task.get("dispatch") or {}).get("integrationBranch"),
            generator="harness-runner",
        )
        if task.get("worktreePath"):
            upsert_worktree_registry_entry(
                root,
                task,
                generator="harness-runner",
                status="running",
                extra={"tmuxSession": tmux_session, "lastHeartbeatAt": now_iso()},
            )
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
            worktree_path=task.get("worktreePath"),
            branch_name=task.get("branchName"),
            base_ref=task.get("baseRef") or (task.get("dispatch") or {}).get("baseRef"),
            integration_branch=task.get("integrationBranch") or (task.get("dispatch") or {}).get("integrationBranch"),
            generator="harness-runner",
        )
        if task.get("worktreePath"):
            upsert_worktree_registry_entry(
                root,
                task,
                generator="harness-runner",
                status="recoverable",
                extra={"tmuxSession": tmux_session, "lastExitCode": exit_code},
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


def refresh_hot_state(root: Path) -> tuple[bool, str | None]:
    refresh_state_py = root / ".harness" / "scripts" / "refresh-state.py"
    if not refresh_state_py.exists():
        return True, None
    result = subprocess.run(
        ["python3", str(refresh_state_py), str(root)],
        capture_output=True,
        text=True,
    )
    if result.returncode == 0:
        return True, None
    error = result.stderr.strip() or result.stdout.strip() or "refresh-state failed"
    return False, error


def cmd_daemon_foreground(root: Path, interval: int, dispatch_mode: str, session_name: str | None):
    files = ensure_runtime_scaffold(root, generator="harness-runner")
    state_dir = files["state_dir"]
    paths = daemon_paths(state_dir)
    write_daemon_state(
        state_dir,
        status="running",
        session_name=session_name,
        interval=interval,
        dispatch_mode=dispatch_mode,
        log_path=paths["log"],
        last_error=None,
    )
    print(f"[runner-daemon] started interval={interval}s dispatch_mode={dispatch_mode}", flush=True)

    while True:
        tick_ts = now_iso()
        result_payload: dict | None = None
        last_error = None
        tick = subprocess.run(
            [
                "python3",
                str(Path(__file__).resolve()),
                "tick",
                str(root),
                "--trigger",
                "daemon",
                "--dispatch-mode",
                dispatch_mode,
            ],
            capture_output=True,
            text=True,
        )
        stdout = (tick.stdout or "").strip()
        if stdout:
            print(stdout, flush=True)
            try:
                result_payload = json.loads(stdout)
            except json.JSONDecodeError:
                result_payload = {"raw": stdout}
        if tick.returncode != 0:
            last_error = (tick.stderr or "").strip() or "runner tick failed"
            if tick.stderr.strip():
                print(tick.stderr.strip(), file=sys.stderr, flush=True)

        refresh_ok, refresh_error = refresh_hot_state(root)
        refresh_ts = now_iso()
        if not refresh_ok:
            last_error = refresh_error
            if refresh_error:
                print(refresh_error, file=sys.stderr, flush=True)

        write_daemon_state(
            state_dir,
            status="running",
            session_name=session_name,
            interval=interval,
            dispatch_mode=dispatch_mode,
            log_path=paths["log"],
            last_tick_at=tick_ts,
            last_refresh_at=refresh_ts,
            last_tick_result=result_payload,
            last_error=last_error,
        )
        time.sleep(interval)


def cmd_daemon(root: Path, interval: int = 60, dispatch_mode: str = "tmux", foreground: bool = False, replace: bool = False, session_name: str | None = None):
    files = ensure_runtime_scaffold(root, generator="harness-runner")
    state_dir = files["state_dir"]
    paths = daemon_paths(state_dir)

    if foreground:
        try:
            return cmd_daemon_foreground(root, interval, dispatch_mode, session_name)
        except KeyboardInterrupt:
            write_daemon_state(
                state_dir,
                status="stopped",
                session_name=session_name,
                interval=interval,
                dispatch_mode=dispatch_mode,
                log_path=paths["log"],
                last_error="interrupted",
            )
            return 0

    existing = resolve_daemon_session(state_dir)
    task_pool = load_json(files["harness"] / "task-pool.json")
    heartbeats = load_heartbeats(state_dir)
    tmux_gc = gc_project_tmux_sessions(
        root,
        task_pool.get("tasks", []),
        heartbeats,
        preserve_sessions={existing} if existing and not replace else None,
    )
    if existing and tmux_session_running(existing):
        if not replace:
            write_daemon_state(
                state_dir,
                status="running",
                session_name=existing,
                interval=interval,
                dispatch_mode=dispatch_mode,
                log_path=paths["log"],
                last_error=None,
            )
            print(json.dumps({"ok": True, "status": "already-running", "sessionName": existing, "logPath": str(paths["log"]), "tmuxGc": tmux_gc}, ensure_ascii=False, indent=2))
            return 0
        subprocess.run(["tmux", "kill-session", "-t", existing], capture_output=True)

    session = session_name or make_tmux_session_name("daemon", root.name)
    paths["log"].parent.mkdir(parents=True, exist_ok=True)
    paths["log"].touch(exist_ok=True)
    script_body = (
        "#!/usr/bin/env bash\n"
        "set -euo pipefail\n"
        f'cd "{root}"\n'
        f'exec > >(tee -a "{paths["log"]}") 2>&1\n'
        f'python3 "{Path(__file__).resolve()}" daemon "{root}" --foreground --interval {interval} --dispatch-mode {dispatch_mode} --session-name "{session}"\n'
    )
    paths["script"].write_text(script_body)
    paths["script"].chmod(0o755)
    subprocess.run(["tmux", "new-session", "-d", "-s", session, f"bash {paths['script']}"], check=True)
    subprocess.run(["tmux", "set-option", "-t", session, "remain-on-exit", "off"], capture_output=True)
    paths["session"].write_text(session + "\n")
    write_daemon_state(
        state_dir,
        status="running",
        session_name=session,
        interval=interval,
        dispatch_mode=dispatch_mode,
        log_path=paths["log"],
        last_error=None,
    )
    print(json.dumps({"ok": True, "status": "started", "sessionName": session, "logPath": str(paths["log"]), "tmuxGc": tmux_gc}, ensure_ascii=False, indent=2))
    return 0


def cmd_daemon_stop(root: Path):
    files = ensure_runtime_scaffold(root, generator="harness-runner")
    state_dir = files["state_dir"]
    paths = daemon_paths(state_dir)
    session = resolve_daemon_session(state_dir)
    stopped = False
    if session and tmux_session_alive(session):
        subprocess.run(["tmux", "kill-session", "-t", session], capture_output=True)
        stopped = True
    if paths["session"].exists():
        paths["session"].unlink()
    if paths["script"].exists():
        paths["script"].unlink()
    task_pool = load_json(files["harness"] / "task-pool.json")
    heartbeats = load_heartbeats(state_dir)
    tmux_gc = gc_project_tmux_sessions(root, task_pool.get("tasks", []), heartbeats)
    write_daemon_state(
        state_dir,
        status="stopped",
        session_name=None,
        interval=None,
        dispatch_mode=None,
        log_path=paths["log"],
        last_error=None,
    )
    print(json.dumps({"ok": True, "stopped": stopped, "sessionName": session, "tmuxGc": tmux_gc}, ensure_ascii=False, indent=2))
    return 0


def cmd_daemon_status(root: Path):
    files = ensure_runtime_scaffold(root, generator="harness-runner")
    state_dir = files["state_dir"]
    state = load_daemon_state(state_dir)
    session = resolve_daemon_session(state_dir)
    session_alive = bool(session and tmux_session_alive(session))
    result = {
        "daemon": state,
        "sessionAlive": session_alive,
        "sessionRunning": bool(session and tmux_session_running(session)),
        "sessionName": session,
    }
    print(json.dumps(result, ensure_ascii=False, indent=2))
    return 0


def cmd_list(root: Path):
    files = ensure_runtime_scaffold(root, generator="harness-runner")
    result = {
        "runnerState": load_optional_json(files["runner_state_path"], {}),
        "heartbeats": load_heartbeats(files["state_dir"]),
        "daemon": load_daemon_state(files["state_dir"]),
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

    p_daemon = sub.add_parser("daemon", help="start or run a continuous runner daemon")
    p_daemon.add_argument("root", help="project root")
    p_daemon.add_argument("--interval", type=int, default=60)
    p_daemon.add_argument("--dispatch-mode", choices=["tmux", "print"], default="tmux")
    p_daemon.add_argument("--foreground", action="store_true")
    p_daemon.add_argument("--replace", action="store_true")
    p_daemon.add_argument("--session-name")

    p_daemon_stop = sub.add_parser("daemon-stop", help="stop the runner daemon")
    p_daemon_stop.add_argument("root", help="project root")

    p_daemon_status = sub.add_parser("daemon-status", help="show runner daemon status")
    p_daemon_status.add_argument("root", help="project root")

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
    if args.command == "daemon":
        return cmd_daemon(root, args.interval, args.dispatch_mode, args.foreground, args.replace, args.session_name)
    if args.command == "daemon-stop":
        return cmd_daemon_stop(root)
    if args.command == "daemon-status":
        return cmd_daemon_status(root)
    if args.command == "heartbeat":
        return cmd_heartbeat(root, args.task_id, args.tmux_session, args.phase, args.exit_code)
    return 1


if __name__ == "__main__":
    try:
        sys.exit(main() or 0)
    except Exception as exc:
        print(f"runner failed: {exc}", file=sys.stderr)
        sys.exit(1)
