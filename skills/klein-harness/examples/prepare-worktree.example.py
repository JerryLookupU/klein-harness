#!/usr/bin/env python3
from __future__ import annotations
import argparse
import json
import subprocess
import sys
from pathlib import Path

from runtime_common import (
    ensure_runtime_scaffold,
    find_task,
    integration_branch_for_task,
    lineage_event,
    load_json,
    now_iso,
    read_task_pool,
    task_requires_dedicated_worktree,
    upsert_worktree_registry_entry,
    write_json,
)


def run(cmd: list[str], *, cwd: Path, check: bool = True):
    return subprocess.run(cmd, cwd=str(cwd), check=check, text=True, capture_output=True)


def git_head_available(root: Path) -> bool:
    try:
        run(["git", "rev-parse", "--verify", "HEAD"], cwd=root)
        return True
    except subprocess.CalledProcessError:
        return False


def git_ref_exists(root: Path, ref_name: str) -> bool:
    try:
        run(["git", "rev-parse", "--verify", ref_name], cwd=root)
        return True
    except subprocess.CalledProcessError:
        return False


def ref_is_unborn(ref_name: str | None) -> bool:
    return (ref_name or "").strip() == "UNBORN_HEAD"


def worktree_is_git_checkout(path: Path) -> bool:
    return path.exists() and (path / ".git").exists()


def summarize_git_error(exc: subprocess.CalledProcessError) -> str:
    stderr = (exc.stderr or "").strip()
    stdout = (exc.stdout or "").strip()
    return stderr or stdout or str(exc)


def create_or_attach_worktree(root: Path, *, worktree_path: Path, branch_name: str, base_ref: str) -> dict:
    if worktree_is_git_checkout(worktree_path):
        return {"created": False, "attached": False}
    worktree_path.parent.mkdir(parents=True, exist_ok=True)
    if git_ref_exists(root, branch_name):
        run(["git", "worktree", "add", str(worktree_path), branch_name], cwd=root)
        return {"created": False, "attached": True}
    run(["git", "worktree", "add", str(worktree_path), "-b", branch_name, base_ref], cwd=root)
    return {"created": True, "attached": True}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--root", required=True, help="project root containing .harness/")
    parser.add_argument("--task-id", required=True, help="task id to prepare")
    parser.add_argument("--create", action="store_true", help="create the worktree if missing")
    parser.add_argument("--write-back", action="store_true", help="persist worktree-prepared metadata into task-pool.json and worktree registry")
    args = parser.parse_args()

    root = Path(args.root).resolve()
    files = ensure_runtime_scaffold(root, generator="harness-prepare-worktree")
    task_pool_path = files["harness"] / "task-pool.json"
    task_pool = load_json(task_pool_path)
    task = find_task(task_pool.get("tasks", []), args.task_id)

    branch_name = task.get("branchName")
    worktree_rel = task.get("worktreePath")
    base_ref = task.get("baseRef") or task.get("dispatch", {}).get("baseRef")
    integration_branch = integration_branch_for_task(task, read_task_pool(files["harness"]))
    dedicated = task_requires_dedicated_worktree(task, task_pool.get("tasks", []))
    if not branch_name or not worktree_rel:
        raise ValueError("task missing branchName or worktreePath")
    if dedicated and not base_ref:
        raise ValueError("task missing baseRef")

    try:
        run(["git", "rev-parse", "--show-toplevel"], cwd=root)
    except subprocess.CalledProcessError as exc:
        raise ValueError(f"not a git repository: {root}") from exc

    worktree_path = (root / worktree_rel).resolve()
    exists = worktree_is_git_checkout(worktree_path)
    create_result = {"created": False, "attached": False}
    environment_status = "ready"
    environment_reason = None
    if dedicated and args.create and not exists:
        if ref_is_unborn(base_ref) or not git_head_available(root):
            environment_status = "degraded"
            environment_reason = "git HEAD is unborn; dedicated worktree creation is deferred until the repo has an initial commit"
        else:
            try:
                create_result = create_or_attach_worktree(root, worktree_path=worktree_path, branch_name=branch_name, base_ref=base_ref)
                exists = worktree_is_git_checkout(worktree_path)
            except subprocess.CalledProcessError as exc:
                environment_status = "degraded"
                environment_reason = f"git worktree add failed: {summarize_git_error(exc)}"

    if dedicated and not exists and environment_reason is None:
        environment_status = "degraded"
        environment_reason = "dedicated worktree is missing; execution falls back to the repo root until the worktree is prepared"

    timestamp = now_iso()
    status = "worktree_prepared" if exists else "worktree_missing"
    result = {
        "schemaVersion": "1.0",
        "generator": "harness-prepare-worktree",
        "generatedAt": timestamp,
        "taskId": task["taskId"],
        "branchName": branch_name,
        "worktreePath": str(worktree_path),
        "worktreePathRelative": worktree_rel,
        "baseRef": base_ref,
        "integrationBranch": integration_branch,
        "requiresDedicatedWorktree": dedicated,
        "exists": exists,
        "created": create_result["created"],
        "attached": create_result["attached"],
        "status": status,
        "environmentStatus": environment_status,
        "environmentReason": environment_reason,
        "recommendedCwd": str(worktree_path if exists else root),
        "commands": {
            "create": f"git worktree add {worktree_path} -b {branch_name} {base_ref}" if base_ref else None,
            "status": f"git -C {worktree_path} status --short" if exists else None,
        },
    }

    if args.write_back:
        task["worktreePreparedAt"] = timestamp if exists else task.get("worktreePreparedAt")
        task["worktreeStatus"] = status
        task["worktreeEnvironmentStatus"] = environment_status
        task["worktreeEnvironmentReason"] = environment_reason
        task["integrationBranch"] = integration_branch
        task["executionCwd"] = worktree_rel if exists else "."
        if exists and task.get("status") in {"queued", "bound"}:
            task["status"] = "worktree_prepared"
        write_json(task_pool_path, task_pool)
        upsert_worktree_registry_entry(
            root,
            task,
            generator="harness-prepare-worktree",
            status=status,
            extra={
                "requiresDedicatedWorktree": dedicated,
                "executionCwd": task.get("executionCwd"),
                "integrationBranch": integration_branch,
                "environmentStatus": environment_status,
                "environmentReason": environment_reason,
            },
        )
        lineage_event(
            root,
            "task.worktree_prepared" if exists else "task.worktree_missing",
            "harness-prepare-worktree",
            task_id=task.get("taskId"),
            worktree_path=worktree_rel,
            detail=status,
            context={
                "branchName": branch_name,
                "baseRef": base_ref,
                "integrationBranch": integration_branch,
                "created": create_result["created"],
                "attached": create_result["attached"],
                "environmentStatus": environment_status,
                "environmentReason": environment_reason,
            },
        )
        result["wroteBack"] = True
    else:
        result["wroteBack"] = False

    print(json.dumps(result, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    try:
        main()
    except Exception as exc:
        print(f"prepare-worktree example failed: {exc}", file=sys.stderr)
        sys.exit(1)
