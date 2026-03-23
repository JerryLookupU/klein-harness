#!/usr/bin/env python3
from __future__ import annotations
import argparse
import json
import subprocess
import sys
from datetime import datetime, timezone
from pathlib import Path


def load_json(path: Path):
    return json.loads(path.read_text())


def write_json(path: Path, data):
    path.write_text(json.dumps(data, ensure_ascii=False, indent=2) + "\n")


def now_iso() -> str:
    return datetime.now(timezone.utc).astimezone().isoformat(timespec="seconds")


def run(cmd, cwd=None):
    return subprocess.run(cmd, cwd=cwd, check=True, text=True, capture_output=True)


def git_ref_exists(root: Path, ref_name: str) -> bool:
    try:
        run(["git", "rev-parse", "--verify", ref_name], cwd=root)
        return True
    except subprocess.CalledProcessError:
        return False


def git_head_available(root: Path) -> bool:
    try:
        run(["git", "rev-parse", "--verify", "HEAD"], cwd=root)
        return True
    except subprocess.CalledProcessError:
        return False


def ref_is_unborn(ref_name: str | None) -> bool:
    return (ref_name or "").strip() == "UNBORN_HEAD"


def summarize_git_error(exc: subprocess.CalledProcessError) -> str:
    stderr = (exc.stderr or "").strip()
    stdout = (exc.stdout or "").strip()
    return stderr or stdout or str(exc)


def find_task(tasks, task_id: str):
    for task in tasks:
        if task.get("taskId") == task_id:
            return task
    raise KeyError(f"task not found: {task_id}")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--root", required=True, help="project root containing .harness/")
    parser.add_argument("--task-id", required=True, help="task id to summarize")
    parser.add_argument("--write-back", action="store_true", help="write diffSummary back into task-pool.json")
    args = parser.parse_args()

    root = Path(args.root).resolve()
    harness = root / ".harness"
    task_pool_path = harness / "task-pool.json"
    task_pool = load_json(task_pool_path)
    task = find_task(task_pool.get("tasks", []), args.task_id)

    worktree_rel = task.get("worktreePath")
    branch_name = task.get("branchName")
    diff_base = task.get("diffBase") or task.get("dispatch", {}).get("diffBase")
    if not worktree_rel or not diff_base:
        raise ValueError("task missing worktreePath or diffBase")

    worktree_path = (root / worktree_rel).resolve()
    generated_at = now_iso()
    comparison_mode = None
    comparison_ref = None
    status = "ok"
    reason = None
    stat = ""
    names = []

    if ref_is_unborn(diff_base) or not git_head_available(root):
        status = "degraded"
        reason = "diff base is unavailable because repository HEAD is unborn"
    elif worktree_path.exists():
        comparison_mode = "worktree-head"
        comparison_ref = "HEAD"
        try:
            stat = run(["git", "diff", "--stat", f"{diff_base}...HEAD"], cwd=worktree_path).stdout.strip()
            names = run(["git", "diff", "--name-only", f"{diff_base}...HEAD"], cwd=worktree_path).stdout.strip().splitlines()
            names = [name for name in names if name]
        except subprocess.CalledProcessError as exc:
            status = "degraded"
            reason = f"git diff failed in worktree: {summarize_git_error(exc)}"
            comparison_mode = None
            comparison_ref = None
    elif branch_name and git_ref_exists(root, branch_name):
        comparison_mode = "branch-ref"
        comparison_ref = branch_name
        try:
            stat = run(["git", "diff", "--stat", f"{diff_base}...{branch_name}"], cwd=root).stdout.strip()
            names = run(["git", "diff", "--name-only", f"{diff_base}...{branch_name}"], cwd=root).stdout.strip().splitlines()
            names = [name for name in names if name]
        except subprocess.CalledProcessError as exc:
            status = "degraded"
            reason = f"git diff failed against branch ref: {summarize_git_error(exc)}"
            comparison_mode = None
            comparison_ref = None
    else:
        status = "degraded"
        reason = f"worktree is missing and branch ref is unavailable: {worktree_path}"

    summary = {
        "schemaVersion": "1.0",
        "generator": "harness-diff-summary",
        "generatedAt": generated_at,
        "taskId": task["taskId"],
        "branchName": branch_name,
        "worktreePath": str(worktree_path),
        "diffBase": diff_base,
        "status": status,
        "reason": reason,
        "comparisonMode": comparison_mode,
        "comparisonRef": comparison_ref,
        "changedFiles": len(names),
        "changedPaths": names,
        "diffStat": stat or None,
    }

    if args.write_back:
        task["diffSummaryGeneratedAt"] = summary["generatedAt"]
        task["diffSummaryStatus"] = status
        task["diffSummaryReason"] = reason
        if status == "ok":
            task["diffSummary"] = stat or "no diff"
        else:
            task["diffSummary"] = None
        write_json(task_pool_path, task_pool)
        summary["wroteBack"] = True
    else:
        summary["wroteBack"] = False

    print(json.dumps(summary, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    try:
        main()
    except Exception as exc:
        print(f"diff-summary example failed: {exc}", file=sys.stderr)
        sys.exit(1)
