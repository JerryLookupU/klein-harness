#!/usr/bin/env python3
import argparse
import json
import subprocess
import sys
from pathlib import Path


def load_json(path: Path):
    return json.loads(path.read_text())


def write_json(path: Path, data):
    path.write_text(json.dumps(data, ensure_ascii=False, indent=2) + "\n")


def run(cmd, cwd=None):
    return subprocess.run(cmd, cwd=cwd, check=True, text=True, capture_output=True)


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
    if not worktree_path.exists():
        raise ValueError(f"worktree does not exist: {worktree_path}")

    stat = run(["git", "diff", "--stat", f"{diff_base}...HEAD"], cwd=worktree_path).stdout.strip()
    names = run(["git", "diff", "--name-only", f"{diff_base}...HEAD"], cwd=worktree_path).stdout.strip().splitlines()
    names = [name for name in names if name]

    summary = {
        "taskId": task["taskId"],
        "branchName": branch_name,
        "worktreePath": str(worktree_path),
        "diffBase": diff_base,
        "changedFiles": len(names),
        "changedPaths": names,
        "diffStat": stat,
    }

    if args.write_back:
        task["diffSummary"] = stat or "no diff"
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
