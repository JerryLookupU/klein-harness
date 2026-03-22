#!/usr/bin/env python3
import argparse
import json
import sys
from pathlib import Path


def load_json(path: Path):
    return json.loads(path.read_text())


def load_optional_json(path: Path):
    if path.exists():
        return load_json(path)
    return None


def find_task(tasks, task_id: str):
    for task in tasks:
        if task.get("taskId") == task_id:
            return task
    raise KeyError(f"task not found: {task_id}")


def recent_failures(feedback_summary, task_id: str):
    if not feedback_summary:
        return []
    return (
        feedback_summary.get("taskFeedbackSummary", {})
        .get(task_id, {})
        .get("recentFailures", [])
    )


def render_worker_start(task, failures):
    lines = [
        "你是 `worker`。",
        f"当前模型是 `{task.get('executionModel', 'gpt-5.3-codex')}`。",
        "你不是 `planner`。",
        "你不是 `orchestrator`。",
        "pre-worker gate 已由程序执行；不要自行重做路由判断。",
        "不要自行改编排。",
        "不要自行改 session routing。",
        f"当前任务: {task.get('taskId')}",
        f"title: {task.get('title', '-')}",
        f"summary: {task.get('summary', '-')}",
        "先做这几步：",
        "1. 读 progress/task-pool/session-registry。",
        "2. 找到当前 task。",
        "3. 如果存在 feedback-summary，只读当前 task 最近 3 条高严重度失败。",
        "4. 只按当前 task 执行。",
    ]
    for failure in failures:
        lines.append(
            f"- recentFailure: {failure.get('feedbackType')} [{failure.get('severity')}] {failure.get('message')}"
        )
    return "\n".join(lines)


def render_worker_execute(task, failures):
    fields = [
        f"ownedPaths: {task.get('ownedPaths', [])}",
        f"worktreePath: {task.get('worktreePath')}",
        f"diffBase: {task.get('diffBase')}",
        f"verificationRuleIds: {task.get('verificationRuleIds', [])}",
        f"resumeStrategy: {task.get('resumeStrategy')}",
        f"preferredResumeSessionId: {task.get('preferredResumeSessionId')}",
    ]
    lines = ["执行边界：", *fields]
    for failure in failures:
        lines.append(
            f"avoidRepeat: {failure.get('feedbackType')} [{failure.get('severity')}] {failure.get('message')}"
        )
    return "\n".join(lines)


def render_worker_recover(task):
    lines = [
        "立即停止条件：",
        "- 路径冲突",
        "- 前提失效",
        "- 需要 rollback",
        "- 需要 stop 其他 task",
        "- ownedPaths 不清楚",
        "- 依赖未满足",
        "出错后按顺序：",
        "1. 写 replan/stop request。",
        "2. 回写 lastKnownSessionId 和失败原因。",
        "3. 把 task 置为 pause_requested 或 finishing_then_pause。",
        "4. 停止继续扩写。",
    ]
    return "\n".join(lines)


def render_audit(task):
    lines = [
        "你是 `worker`。",
        "你的 `workerMode = audit`。",
        "不要直接修改业务源码。",
        f"当前任务: {task.get('taskId')}",
        f"title: {task.get('title', '-')}",
        f"summary: {task.get('summary', '-')}",
        f"reviewOfTaskIds: {task.get('reviewOfTaskIds', [])}",
        f"auditScope: {task.get('auditScope')}",
        f"branchName: {task.get('branchName')}",
        f"diffBase: {task.get('diffBase')}",
        "先做这几步：",
        "1. 读 progress/task-pool/lineage/audit-report。",
        "2. 优先对 diffBase...branchName 做证据采样。",
        "3. 写 audit-report 和 auditVerdict。",
    ]
    return "\n".join(lines)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--root", required=True)
    parser.add_argument("--task-id", required=True)
    parser.add_argument("--role", required=True, choices=["worker", "audit"])
    parser.add_argument("--stage", required=True, choices=["start", "execute", "recover", "audit"])
    args = parser.parse_args()

    root = Path(args.root).resolve()
    task_pool = load_json(root / ".harness" / "task-pool.json")
    feedback_summary = load_optional_json(root / ".harness" / "state" / "feedback-summary.json")
    task = find_task(task_pool.get("tasks", []), args.task_id)
    failures = recent_failures(feedback_summary, args.task_id)

    if args.role == "audit" or task.get("kind") == "audit" or args.stage == "audit":
        print(render_audit(task))
        return

    if args.stage == "start":
        print(render_worker_start(task, failures))
    elif args.stage == "execute":
        print(render_worker_execute(task, failures))
    elif args.stage == "recover":
        print(render_worker_recover(task))
    else:
        raise ValueError(f"unsupported stage: {args.stage}")


if __name__ == "__main__":
    try:
        main()
    except Exception as exc:
        print(f"render-prompt example failed: {exc}", file=sys.stderr)
        sys.exit(1)
