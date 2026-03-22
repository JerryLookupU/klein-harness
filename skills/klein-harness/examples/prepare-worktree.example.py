#!/usr/bin/env python3
import argparse
import json
import subprocess
import sys
from pathlib import Path


def load_json(path: Path):
    return json.loads(path.read_text())


def run(cmd, cwd=None, check=True):
    return subprocess.run(cmd, cwd=cwd, check=check, text=True, capture_output=True)


def find_task(tasks, task_id: str):
    for task in tasks:
        if task.get("taskId") == task_id:
            return task
    raise KeyError(f"task not found: {task_id}")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--root", required=True, help="project root containing .harness/")
    parser.add_argument("--task-id", required=True, help="task id to prepare")
    parser.add_argument("--create", action="store_true", help="create the worktree if missing")
    args = parser.parse_args()

    root = Path(args.root).resolve()
    harness = root / ".harness"
    task_pool = load_json(harness / "task-pool.json")
    task = find_task(task_pool.get("tasks", []), args.task_id)

    branch_name = task.get("branchName")
    worktree_rel = task.get("worktreePath")
    base_ref = task.get("baseRef") or task.get("dispatch", {}).get("baseRef")
    if not branch_name or not worktree_rel:
        raise ValueError("task missing branchName or worktreePath")
    if not base_ref:
        raise ValueError("task missing baseRef")

    git_dir = root / ".git"
    if not git_dir.exists():
        raise ValueError(f"not a git repository: {root}")

    worktree_path = (root / worktree_rel).resolve()
    exists = worktree_path.exists()

    if args.create and not exists:
        worktree_path.parent.mkdir(parents=True, exist_ok=True)
        run(["git", "worktree", "add", str(worktree_path), "-b", branch_name, base_ref], cwd=root)
        exists = True

    result = {
        "taskId": task["taskId"],
        "branchName": branch_name,
        "worktreePath": str(worktree_path),
        "baseRef": base_ref,
        "exists": exists,
        "created": bool(args.create and exists),
        "recommendedCwd": str(worktree_path),
        "commands": {
            "create": f"git worktree add {worktree_path} -b {branch_name} {base_ref}",
            "status": f"git -C {worktree_path} status --short",
        },
    }

    print(json.dumps(result, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    try:
        main()
    except Exception as exc:
        print(f"prepare-worktree example failed: {exc}", file=sys.stderr)
        sys.exit(1)
