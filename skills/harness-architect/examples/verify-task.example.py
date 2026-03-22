#!/usr/bin/env python3
import argparse
import json
import subprocess
import sys
import time
from pathlib import Path

from runtime_common import (
    emit_follow_up_request,
    ensure_runtime_scaffold,
    find_task,
    load_json,
    load_optional_json,
    maybe_complete_request,
    now_iso,
    request_bindings_for_task,
    update_binding_state,
    write_json,
)


def resolve_cwd(root: Path, task: dict):
    worktree_rel = task.get("worktreePath")
    if worktree_rel:
        worktree_path = (root / worktree_rel).resolve()
        if worktree_path.exists():
            return worktree_path, "worktree"
    return root, "root"


def run_rule(rule: dict, cwd: Path):
    started_at = time.time()
    timeout_ms = int(rule.get("timeout") or 30000)
    try:
        result = subprocess.run(
            rule["exec"],
            cwd=cwd,
            shell=True,
            executable="/bin/bash",
            text=True,
            capture_output=True,
            timeout=timeout_ms / 1000,
        )
        duration_ms = int((time.time() - started_at) * 1000)
        return {
            "status": "pass" if result.returncode == 0 else "fail",
            "exitCode": result.returncode,
            "durationMs": duration_ms,
            "stdout": result.stdout[-4000:],
            "stderr": result.stderr[-4000:],
        }
    except subprocess.TimeoutExpired as exc:
        duration_ms = int((time.time() - started_at) * 1000)
        return {
            "status": "fail",
            "exitCode": None,
            "durationMs": duration_ms,
            "stdout": (exc.stdout or "")[-4000:],
            "stderr": ((exc.stderr or "") + "\nTIMEOUT")[-4000:],
        }


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--root", required=True, help="project root containing .harness/")
    parser.add_argument("--task-id", required=True, help="task id to verify")
    parser.add_argument("--write-back", action="store_true", help="write verification status back into task-pool.json and request lifecycle")
    args = parser.parse_args()

    root = Path(args.root).resolve()
    files = ensure_runtime_scaffold(root, generator="harness-verify-task")
    task_pool_path = files["harness"] / "task-pool.json"
    manifest_path = files["harness"] / "verification-rules" / "manifest.json"
    task_pool = load_json(task_pool_path)
    manifest = load_optional_json(manifest_path, {"rules": []})
    task = find_task(task_pool.get("tasks", []), args.task_id)
    rules_by_id = {rule.get("id"): rule for rule in manifest.get("rules", [])}
    result_path = files["verification_dir"] / f"{args.task_id}.json"

    rule_ids = task.get("verificationRuleIds") or []
    cwd, cwd_source = resolve_cwd(root, task)
    results = []
    passed_rule_ids = []
    failed_rule_ids = []
    missing_rule_ids = []

    if not rule_ids:
        overall_status = "skipped"
    else:
        overall_status = "pass"
        for rule_id in rule_ids:
            rule = rules_by_id.get(rule_id)
            if not rule:
                results.append({
                    "ruleId": rule_id,
                    "status": "missing",
                    "title": None,
                    "cwd": str(cwd),
                    "cwdSource": cwd_source,
                })
                missing_rule_ids.append(rule_id)
                overall_status = "fail"
                continue

            run_result = run_rule(rule, cwd)
            results.append({
                "ruleId": rule_id,
                "title": rule.get("title"),
                "type": rule.get("type"),
                "costTier": rule.get("costTier"),
                "readOnlySafe": rule.get("readOnlySafe"),
                "command": rule.get("exec"),
                "cwd": str(cwd),
                "cwdSource": cwd_source,
                **run_result,
            })
            if run_result["status"] == "pass":
                passed_rule_ids.append(rule_id)
            else:
                failed_rule_ids.append(rule_id)
                overall_status = "fail"

    report = {
        "schemaVersion": "1.0",
        "generator": "harness-verify-task",
        "generatedAt": now_iso(),
        "taskId": task.get("taskId"),
        "taskStatus": task.get("status"),
        "worktreePath": task.get("worktreePath"),
        "branchName": task.get("branchName"),
        "diffBase": task.get("diffBase") or task.get("dispatch", {}).get("diffBase"),
        "verificationRuleIds": rule_ids,
        "overallStatus": overall_status,
        "passedRuleIds": passed_rule_ids,
        "failedRuleIds": failed_rule_ids,
        "missingRuleIds": missing_rule_ids,
        "results": results,
    }
    write_json(result_path, report)

    if args.write_back:
        task["verificationStatus"] = overall_status
        task["verificationUpdatedAt"] = report["generatedAt"]
        task["verificationResultPath"] = str(result_path.relative_to(root))
        if overall_status == "pass":
            task["verificationSummary"] = f"{len(passed_rule_ids)}/{len(rule_ids)} rules passed"
            task["status"] = "verified"
        elif overall_status == "skipped":
            task["verificationSummary"] = "no verification rules"
            task["status"] = "verified"
        else:
            task["verificationSummary"] = (
                f"{len(failed_rule_ids) + len(missing_rule_ids)} of {len(rule_ids)} rules failed or missing"
            )
            task["status"] = "recoverable"
        write_json(task_pool_path, task_pool)

        task_map = load_json(files["request_task_map_path"])
        bindings = request_bindings_for_task(task_map, args.task_id)
        for binding in bindings:
            verification = {
                "overallStatus": overall_status,
                "summary": task.get("verificationSummary"),
                "verificationResultPath": task["verificationResultPath"],
            }
            if overall_status in {"pass", "skipped"}:
                update_binding_state(
                    root,
                    binding.get("requestId"),
                    args.task_id,
                    "verified",
                    reason=f"verification {overall_status}",
                    generator="harness-verify-task",
                    session_id=binding.get("lineage", {}).get("sessionId"),
                    verification=verification,
                    worktree_path=task.get("worktreePath"),
                    diff_summary=task.get("diffSummary"),
                )
                maybe_complete_request(root, binding.get("requestId"), generator="harness-verify-task")
                if task.get("kind") != "audit" and task.get("handoff", {}).get("mergeRequired"):
                    emit_follow_up_request(
                        root,
                        kind="audit",
                        goal=f"Audit {args.task_id} after verification {overall_status}",
                        source="runtime:verify",
                        generator="harness-verify-task",
                        parent_request_id=binding.get("requestId"),
                        origin_task_id=args.task_id,
                        origin_session_id=binding.get("lineage", {}).get("sessionId"),
                        reason="verified work requires audit before merge",
                        dedupe_key=f"audit:{args.task_id}:{overall_status}",
                    )
            else:
                update_binding_state(
                    root,
                    binding.get("requestId"),
                    args.task_id,
                    "recoverable",
                    reason="verification failed; runtime should recover or replan",
                    generator="harness-verify-task",
                    session_id=binding.get("lineage", {}).get("sessionId"),
                    verification=verification,
                    worktree_path=task.get("worktreePath"),
                    diff_summary=task.get("diffSummary"),
                    outcome={"status": "verification_failed"},
                )
                emit_follow_up_request(
                    root,
                    kind="replan",
                    goal=f"Replan {args.task_id} after verification failure: {task.get('verificationSummary')}",
                    source="runtime:verify",
                    generator="harness-verify-task",
                    parent_request_id=binding.get("requestId"),
                    origin_task_id=args.task_id,
                    origin_session_id=binding.get("lineage", {}).get("sessionId"),
                    reason="verification failed",
                    dedupe_key=f"replan:{args.task_id}:{task.get('verificationSummary')}",
                )

    print(json.dumps(report, ensure_ascii=False, indent=2))
    return 0 if overall_status in {"pass", "skipped"} else 1


if __name__ == "__main__":
    try:
        sys.exit(main())
    except Exception as exc:
        print(f"verify-task example failed: {exc}", file=sys.stderr)
        sys.exit(1)
