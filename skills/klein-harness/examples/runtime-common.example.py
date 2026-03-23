#!/usr/bin/env python3
from __future__ import annotations
import hashlib
import json
import os
import re
import subprocess
import tempfile
from collections import Counter, defaultdict
from datetime import datetime, timezone
from pathlib import Path


SCHEMA_VERSION = "1.0"
REQUEST_TERMINAL_STATUSES = {"completed", "cancelled"}
TASK_ACTIVE_STATUSES = {"active", "claimed", "in_progress", "dispatched", "running", "resumed"}
TASK_COMPLETED_STATUSES = {"completed", "validated", "done", "pass", "verified"}
TASK_SUPERSEDED_STATUSES = {"superseded", "finishing_then_pause"}
SEVERE_ROUTE_FAILURES = {"illegal_action", "path_conflict", "session_conflict", "replan_required"}
REQUEST_SNAPSHOT_KINDS = {"audit", "replan", "stop", "bug", "feedback", "rca"}
RCA_TERMINAL_STATUSES = {"repaired", "closed"}
INTENT_CLASSES = {
    "duplicate_or_noop",
    "context_enrichment",
    "inspection",
    "append_change",
    "fresh_work",
    "compound_split",
    "ambiguous_needs_orchestrator",
}
FRONT_DOOR_CLASSES = {
    "conversational_help",
    "advisory_read_only",
    "inspection",
    "work_order",
    "duplicate_or_context",
}
FUSION_DECISIONS = {
    "accepted_new_thread",
    "accepted_existing_thread",
    "duplicate_of_existing",
    "merged_as_context",
    "inspection_overlay",
    "append_requires_replan",
    "compound_split_created",
    "noop",
}
IMPACT_CLASSES = {
    "continue_safe",
    "continue_with_note",
    "checkpoint_then_replan",
    "supersede_queued",
    "inspection_only_overlay",
}
ROT_SESSION_STATUSES = {"fresh", "healthy", "warning", "downgraded", "superseded", "archived"}
RCA_DIMENSIONS = {
    "spec_acceptance",
    "blueprint_decomposition",
    "routing_session",
    "execution_change",
    "verification_guardrail",
    "runtime_tooling",
    "environment_dependency",
    "merge_handoff",
    "underdetermined",
}
CAUSE_OWNER_ROLE = {
    "spec_acceptance": "architect/product",
    "blueprint_decomposition": "orchestrator/blueprint-architect",
    "routing_session": "runtime/orchestrator",
    "execution_change": "worker",
    "verification_guardrail": "verifier/architect",
    "runtime_tooling": "runtime-maintainer",
    "environment_dependency": "operator/external",
    "merge_handoff": "audit/orchestrator",
    "underdetermined": "architect/orchestrator",
}
CAUSE_REPAIR_MODE = {
    "spec_acceptance": "replan/clarification",
    "blueprint_decomposition": "replan",
    "routing_session": "stop/route-fix",
    "execution_change": "bugfix",
    "verification_guardrail": "test-fix",
    "runtime_tooling": "harness-fix",
    "environment_dependency": "ops-fix",
    "merge_handoff": "audit/merge-fix",
    "underdetermined": "audit/research",
}
CAUSE_PREVENTION_TARGET = {
    "spec_acceptance": ".harness/spec.json",
    "blueprint_decomposition": ".harness/work-items.json",
    "routing_session": ".harness/session-registry.json",
    "execution_change": ".harness/standards.md",
    "verification_guardrail": ".harness/verification-rules/manifest.json",
    "runtime_tooling": ".harness/templates/AGENTS.template.md",
    "environment_dependency": ".harness/notes/environment.md",
    "merge_handoff": ".harness/audit-report.md",
    "underdetermined": ".harness/audit-report.md",
}
DEFAULT_POLICY_SUMMARY = {
    "schemaVersion": SCHEMA_VERSION,
    "generator": "klein-harness",
    "generatedAt": None,
    "heartbeat": {
        "staleAfterSeconds": 180,
        "recoverableAfterSeconds": 45,
        "maxRecentHeartbeats": 20,
    },
    "retry": {
        "maxAttempts": 3,
        "backoffSeconds": [15, 60, 300],
    },
    "queue": {
        "priorityOrder": ["P0", "P1", "P2", "P3", "P4", "P5", "P6", "P7", "P8", "P9"],
        "maxRecentRequests": 10,
        "maxBlockedItems": 10,
    },
    "daemon": {
        "defaultIntervalSeconds": 60,
        "maxRecentErrors": 5,
        "maxRecentEvents": 10,
        "degradedAfterSeconds": 180,
    },
    "dispatch": {
        "defaultBackend": "tmux",
        "supportedBackends": ["tmux", "print"],
        "printIsNonExecuting": True,
        "backendHealthThresholds": {
            "tmuxSessionMissing": "error",
            "printPreview": "info",
        },
    },
    "hotState": {
        "maxRecentFailures": 5,
        "maxTaskFailures": 3,
        "maxActiveItems": 10,
        "maxRecentLogs": 10,
        "maxOpenBlockers": 10,
    },
    "intake": {
        "maxRecentSubmissions": 15,
        "maxClassificationEvents": 15,
        "maxRecentThreads": 10,
        "maxRecentChanges": 10,
        "duplicateCanonicalGoalWindow": 20,
        "tokenOverlapThreshold": 0.6,
    },
    "threading": {
        "defaultInitialPlanEpoch": 1,
        "maxRecentThreadEvents": 10,
        "maxRecentChanges": 10,
    },
    "contextRot": {
        "warnScore": 4,
        "freshSessionScore": 7,
        "maxResumeCount": 3,
        "staleSummaryAfterSeconds": 1800,
        "sessionAgeWarnSeconds": 7200,
        "sessionAgeFreshSeconds": 21600,
    },
    "worktree": {
        "defaultBackend": "git-worktree",
        "registryWindow": 20,
        "prepareRequiredForCode": True,
        "cleanupRetentionStatuses": ["merged", "superseded", "abandoned", "merge_conflict"],
    },
    "merge": {
        "queueWindow": 20,
        "summaryWindow": 10,
        "defaultCleanupStatus": "retained",
        "mergedCleanupStatus": "ready_to_reclaim",
        "conflictImpactStatus": "unsafe-conflict",
        "cleanImpactStatus": "non-conflicting",
    },
}


def now_iso():
    return datetime.now(timezone.utc).astimezone().isoformat(timespec="seconds")


def load_json(path: Path):
    return json.loads(path.read_text())


def load_optional_json(path: Path, default=None):
    if path.exists():
        try:
            return load_json(path)
        except json.JSONDecodeError:
            return default
    return default


def write_json(path: Path, data):
    path.parent.mkdir(parents=True, exist_ok=True)
    payload = json.dumps(data, ensure_ascii=False, indent=2) + "\n"
    with tempfile.NamedTemporaryFile("w", encoding="utf-8", dir=path.parent, prefix=f".{path.name}.", suffix=".tmp", delete=False) as handle:
        handle.write(payload)
        tmp_name = handle.name
    os.replace(tmp_name, path)


def load_jsonl(path: Path):
    records = []
    if not path.exists():
        return records
    for line in path.read_text().splitlines():
        line = line.strip()
        if line:
            records.append(json.loads(line))
    return records


def append_jsonl(path: Path, data):
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("a", encoding="utf-8") as handle:
        handle.write(json.dumps(data, ensure_ascii=False) + "\n")


def load_progress(path: Path):
    text = path.read_text()
    match = re.search(r"```json\s*(\{[\s\S]*?\})\s*```", text)
    if not match:
        raise ValueError(f"missing json block in {path}")
    return json.loads(match.group(1))


def priority_rank(priority: str | None, order: list[str] | None = None) -> int:
    priority_order = order or DEFAULT_POLICY_SUMMARY["queue"]["priorityOrder"]
    if priority in priority_order:
        return priority_order.index(priority)
    return len(priority_order)


def parse_iso(value: str | None):
    if not value or not isinstance(value, str):
        return None
    try:
        return datetime.fromisoformat(value)
    except ValueError:
        return None


def age_seconds(value: str | None) -> int | None:
    parsed = parse_iso(value)
    if parsed is None:
        return None
    now = datetime.now(parsed.tzinfo or timezone.utc)
    return max(0, int((now - parsed).total_seconds()))


def bounded(items, limit: int):
    return list(items or [])[:limit]


def default_progress_state() -> dict:
    return {
        "schemaVersion": SCHEMA_VERSION,
        "generator": "klein-harness",
        "generatedAt": now_iso(),
        "mode": "bootstrap",
        "planningStage": "draft",
        "currentFocus": None,
        "currentRole": "orchestrator",
        "currentTaskId": None,
        "currentTaskTitle": None,
        "currentTaskSummary": None,
        "blockers": [],
        "nextActions": [],
        "lastAuditStatus": "unknown",
        "claimSummary": {},
        "legacyFallbackUsed": False,
    }


def render_progress_markdown(progress: dict) -> str:
    lines = [
        "# Harness Progress",
        "",
        f"- mode: {progress.get('mode') or '-'}",
        f"- planningStage: {progress.get('planningStage') or '-'}",
        f"- currentFocus: {progress.get('currentFocus') or '-'}",
        f"- currentRole: {progress.get('currentRole') or '-'}",
        f"- currentTaskId: {progress.get('currentTaskId') or '-'}",
        f"- currentTaskTitle: {progress.get('currentTaskTitle') or '-'}",
        f"- lastAuditStatus: {progress.get('lastAuditStatus') or '-'}",
        "",
        "## Summary",
        progress.get("currentTaskSummary") or "No current task summary.",
        "",
        "## Blockers",
    ]
    blockers = progress.get("blockers", [])
    if blockers:
        lines.extend(f"- {item}" for item in blockers[:10])
    else:
        lines.append("- none")
    lines.extend(["", "## Next Actions"])
    next_actions = progress.get("nextActions", [])
    if next_actions:
        lines.extend(f"- {item}" for item in next_actions[:10])
    else:
        lines.append("- none")
    lines.extend([
        "",
        "## Machine Source",
        "- This file is rendered from `.harness/state/progress.json`.",
        "- Do not parse machine state from this Markdown surface.",
    ])
    return "\n".join(lines).rstrip() + "\n"


def write_progress_projection(files: dict, progress: dict, *, generator: str):
    snapshot = default_progress_state()
    snapshot.update(progress or {})
    snapshot["schemaVersion"] = SCHEMA_VERSION
    snapshot["generator"] = generator
    snapshot["generatedAt"] = now_iso()
    write_json(files["progress_state_path"], snapshot)
    files["progress_markdown_path"].write_text(render_progress_markdown(snapshot), encoding="utf-8")
    return snapshot


def read_progress_state(files: dict, *, generator: str):
    progress_json = files["progress_state_path"]
    progress_md = files["progress_markdown_path"]
    if progress_json.exists():
        data = load_json(progress_json)
        data.setdefault("legacyFallbackUsed", False)
        return data
    if progress_md.exists():
        legacy = load_progress(progress_md)
        legacy["legacyFallbackUsed"] = True
        legacy["deprecatedSource"] = ".harness/progress.md"
        return write_progress_projection(files, legacy, generator=generator)
    return write_progress_projection(files, default_progress_state(), generator=generator)


def render_research_markdown(summary: dict) -> str:
    lines = [
        "# Research Summary",
        "",
        f"- memoCount: {summary.get('memoCount', 0)}",
        f"- researchModes: {summary.get('researchModes', {})}",
        "",
        "## Recent Memos",
    ]
    recent = summary.get("recentMemos", [])
    if recent:
        for item in recent[:10]:
            lines.append(
                f"- {item.get('slug')} [{item.get('researchMode')}] {item.get('path')}"
            )
    else:
        lines.append("- none")
    return "\n".join(lines).rstrip() + "\n"


def build_policy_summary(generator: str = "klein-harness") -> dict:
    policy = json.loads(json.dumps(DEFAULT_POLICY_SUMMARY))
    policy["generator"] = generator
    policy["generatedAt"] = now_iso()
    return policy


def load_policy_summary(path: Path | None, default_generator: str = "klein-harness") -> dict:
    if path and path.exists():
        policy = load_json(path)
        policy.setdefault("dispatch", DEFAULT_POLICY_SUMMARY["dispatch"])
        policy.setdefault("heartbeat", DEFAULT_POLICY_SUMMARY["heartbeat"])
        policy.setdefault("retry", DEFAULT_POLICY_SUMMARY["retry"])
        policy.setdefault("queue", DEFAULT_POLICY_SUMMARY["queue"])
        policy.setdefault("daemon", DEFAULT_POLICY_SUMMARY["daemon"])
        policy.setdefault("hotState", DEFAULT_POLICY_SUMMARY["hotState"])
        policy.setdefault("intake", DEFAULT_POLICY_SUMMARY["intake"])
        policy.setdefault("threading", DEFAULT_POLICY_SUMMARY["threading"])
        policy.setdefault("contextRot", DEFAULT_POLICY_SUMMARY["contextRot"])
        return policy
    return build_policy_summary(generator=default_generator)


def load_runner_heartbeats(state_dir: Path) -> dict:
    data = load_optional_json(state_dir / "runner-heartbeats.json", {})
    entries = data.get("entries", {}) if isinstance(data, dict) else {}
    return entries if isinstance(entries, dict) else {}


def runner_daemon_paths(state_dir: Path) -> dict:
    return {
        "state": state_dir / "runner-daemon.json",
        "log": state_dir / "runner-daemon.log",
        "session": state_dir / "runner-daemon-tmux-session.txt",
        "script": state_dir / "runner-daemon.sh",
    }


def load_runner_daemon_state(state_dir: Path) -> dict:
    return load_optional_json(
        runner_daemon_paths(state_dir)["state"],
        {
            "schemaVersion": SCHEMA_VERSION,
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


def backend_kind_from_label(label: str | None, dispatch_backend: str | None = None) -> str:
    if dispatch_backend:
        return dispatch_backend
    if not label:
        return "unknown"
    if label.startswith("print:") or label.startswith("dispatch:print"):
        return "print"
    if label.startswith("tmux:"):
        return "tmux"
    return "unknown"


def backend_session_label(label: str | None) -> str | None:
    if not label:
        return None
    if label.startswith("tmux:"):
        return label.split(":", 1)[1]
    return label


def tmux_session_alive(session_name: str | None) -> bool:
    if not session_name or session_name.startswith("print:"):
        return False
    try:
        subprocess.run(["tmux", "has-session", "-t", session_name], capture_output=True, check=True)
        return True
    except (subprocess.CalledProcessError, FileNotFoundError):
        return False


def assess_backend_health(dispatch_backend: str, backend_session: str | None) -> dict:
    if dispatch_backend == "print":
        return {
            "dispatchBackend": "print",
            "backendHealth": "non-executing",
            "backendReachable": True,
            "backendSession": backend_session,
        }
    if dispatch_backend == "tmux":
        alive = tmux_session_alive(backend_session)
        return {
            "dispatchBackend": "tmux",
            "backendHealth": "healthy" if alive else "missing",
            "backendReachable": alive,
            "backendSession": backend_session,
        }
    return {
        "dispatchBackend": dispatch_backend or "unknown",
        "backendHealth": "unknown",
        "backendReachable": None,
        "backendSession": backend_session,
    }


def ensure_runtime_scaffold(root: Path, generator: str = "harness-runtime"):
    root = root.resolve()
    harness = root / ".harness"
    state_dir = harness / "state"
    requests_dir = harness / "requests"
    archive_dir = requests_dir / "archive"
    research_dir = harness / "research"
    integration_dir = harness / ".integration"
    verification_dir = state_dir / "verification"
    runner_logs_dir = state_dir / "runner-logs"

    for directory in (
        harness,
        state_dir,
        requests_dir,
        archive_dir,
        research_dir,
        integration_dir,
        verification_dir,
        runner_logs_dir,
        harness / "verification-rules",
        harness / "drift-log",
    ):
        directory.mkdir(parents=True, exist_ok=True)

    queue_path = requests_dir / "queue.jsonl"
    feedback_log_path = harness / "feedback-log.jsonl"
    lineage_path = harness / "lineage.jsonl"
    root_cause_log_path = harness / "root-cause-log.jsonl"
    for log_path in (queue_path, feedback_log_path, lineage_path, root_cause_log_path):
        log_path.touch(exist_ok=True)

    timestamp = now_iso()
    request_index_path = state_dir / "request-index.json"
    if not request_index_path.exists():
        write_json(request_index_path, {
            "schemaVersion": SCHEMA_VERSION,
            "generator": generator,
            "generatedAt": timestamp,
            "nextSeq": 1,
            "requests": [],
        })

    request_task_map_path = state_dir / "request-task-map.json"
    if not request_task_map_path.exists():
        write_json(request_task_map_path, {
            "schemaVersion": SCHEMA_VERSION,
            "generator": generator,
            "generatedAt": timestamp,
            "nextBindingSeq": 1,
            "activeRequestId": None,
            "activeTaskId": None,
            "activeSessionId": None,
            "bindings": [],
        })

    project_meta_path = harness / "project-meta.json"
    if not project_meta_path.exists():
        write_json(project_meta_path, {
            "schemaVersion": SCHEMA_VERSION,
            "generator": generator,
            "generatedAt": timestamp,
            "projectRoot": str(root),
            "lifecycle": "initialized",
            "bootstrapStatus": "not_started",
            "requestQueueEnabled": True,
        })

    session_registry_path = harness / "session-registry.json"
    if not session_registry_path.exists():
        write_json(session_registry_path, {
            "schemaVersion": SCHEMA_VERSION,
            "generator": generator,
            "generatedAt": timestamp,
            "orchestrationSessionId": None,
            "orchestrationSessions": [],
            "sessions": [],
            "families": [],
            "routingDecisions": [],
            "activeBindings": [],
            "recoverableBindings": [],
            "lastCompletedByTask": {},
        })

    runner_state_path = state_dir / "runner-state.json"
    if not runner_state_path.exists():
        write_json(runner_state_path, {
            "schemaVersion": SCHEMA_VERSION,
            "generator": "harness-runner",
            "generatedAt": timestamp,
            "lastTickAt": None,
            "lastTrigger": None,
            "activeRuns": [],
            "recoverableRuns": [],
            "staleRuns": [],
            "dispatchableTaskIds": [],
            "blockedRoutes": [],
            "lastErrors": [],
        })

    runner_heartbeats_path = state_dir / "runner-heartbeats.json"
    if not runner_heartbeats_path.exists():
        write_json(runner_heartbeats_path, {
            "schemaVersion": SCHEMA_VERSION,
            "generator": "harness-runner",
            "generatedAt": timestamp,
            "entries": {},
        })

    feedback_summary_path = state_dir / "feedback-summary.json"
    if not feedback_summary_path.exists():
        write_json(feedback_summary_path, {
            "schemaVersion": SCHEMA_VERSION,
            "generator": generator,
            "generatedAt": timestamp,
            "feedbackLogPath": ".harness/feedback-log.jsonl",
            "feedbackEventCount": 0,
            "errorCount": 0,
            "criticalCount": 0,
            "illegalActionCount": 0,
            "tasksWithRecentFailures": [],
            "byType": {},
            "bySeverity": {},
            "recentFailures": [],
            "taskFeedbackSummary": {},
        })

    root_cause_summary_path = state_dir / "root-cause-summary.json"
    if not root_cause_summary_path.exists():
        write_json(root_cause_summary_path, {
            "schemaVersion": SCHEMA_VERSION,
            "generator": generator,
            "generatedAt": timestamp,
            "rootCauseLogPath": ".harness/root-cause-log.jsonl",
            "rcaCount": 0,
            "openCount": 0,
            "underdeterminedCount": 0,
            "byPrimaryCauseDimension": {},
            "byOwnerRole": {},
            "openItems": [],
            "recurringRootCauses": [],
            "bugsMissingLineageCorrelation": [],
            "recentAllocations": [],
        })

    progress_state_path = state_dir / "progress.json"
    progress_markdown_path = harness / "progress.md"
    if not progress_state_path.exists() and not progress_markdown_path.exists():
        write_json(progress_state_path, default_progress_state())
    if progress_state_path.exists() and not progress_markdown_path.exists():
        progress_markdown_path.write_text(render_progress_markdown(load_json(progress_state_path)), encoding="utf-8")

    for path_name in (
        "current.json",
        "runtime.json",
        "blueprint-index.json",
        "request-summary.json",
        "lineage-index.json",
        "intake-summary.json",
        "thread-state.json",
        "change-summary.json",
        "queue-summary.json",
        "task-summary.json",
        "worker-summary.json",
        "daemon-summary.json",
        "policy-summary.json",
        "research-summary.json",
        "merge-summary.json",
        "worktree-registry.json",
        "merge-queue.json",
    ):
        path = state_dir / path_name
        if not path.exists():
            write_json(path, {
                "schemaVersion": SCHEMA_VERSION,
                "generator": generator,
                "generatedAt": timestamp,
            })

    log_index_path = state_dir / "log-index.json"
    if not log_index_path.exists():
        write_json(log_index_path, {
            "schemaVersion": SCHEMA_VERSION,
            "generator": generator,
            "generatedAt": timestamp,
            "compactLogCount": 0,
            "logsByTaskId": {},
            "logsByRequestId": {},
            "logsBySessionId": {},
            "recentHighSignalLogs": [],
            "openBlockers": [],
            "recurringTags": {},
        })

    research_index_path = state_dir / "research-index.json"
    if not research_index_path.exists():
        write_json(research_index_path, {
            "schemaVersion": SCHEMA_VERSION,
            "generator": generator,
            "generatedAt": timestamp,
            "memoCount": 0,
            "researchModes": {},
            "recentMemos": [],
            "bySlug": {},
        })

    for snapshot_name in (
        "audit-requests.json",
        "replan-requests.json",
        "stop-requests.json",
        "bug-requests.json",
        "feedback-requests.json",
        "rca-requests.json",
    ):
        snapshot_path = harness / snapshot_name
        if not snapshot_path.exists():
            write_json(snapshot_path, {
                "schemaVersion": SCHEMA_VERSION,
                "generator": generator,
                "generatedAt": timestamp,
                "requests": [],
            })

    return {
        "root": root,
        "harness": harness,
        "state_dir": state_dir,
        "progress_state_path": progress_state_path,
        "progress_markdown_path": progress_markdown_path,
        "requests_dir": requests_dir,
        "queue_path": queue_path,
        "archive_dir": archive_dir,
        "research_dir": research_dir,
        "request_index_path": request_index_path,
        "request_task_map_path": request_task_map_path,
        "project_meta_path": project_meta_path,
        "feedback_log_path": feedback_log_path,
        "feedback_summary_path": feedback_summary_path,
        "root_cause_log_path": root_cause_log_path,
        "root_cause_summary_path": root_cause_summary_path,
        "queue_summary_path": state_dir / "queue-summary.json",
        "intake_summary_path": state_dir / "intake-summary.json",
        "thread_state_path": state_dir / "thread-state.json",
        "change_summary_path": state_dir / "change-summary.json",
        "task_summary_path": state_dir / "task-summary.json",
        "worker_summary_path": state_dir / "worker-summary.json",
        "daemon_summary_path": state_dir / "daemon-summary.json",
        "policy_summary_path": state_dir / "policy-summary.json",
        "research_summary_path": state_dir / "research-summary.json",
        "lineage_path": lineage_path,
        "lineage_index_path": state_dir / "lineage-index.json",
        "log_index_path": log_index_path,
        "research_index_path": research_index_path,
        "request_summary_path": state_dir / "request-summary.json",
        "merge_summary_path": state_dir / "merge-summary.json",
        "merge_queue_path": state_dir / "merge-queue.json",
        "worktree_registry_path": state_dir / "worktree-registry.json",
        "session_registry_path": session_registry_path,
        "runner_state_path": runner_state_path,
        "runner_heartbeats_path": runner_heartbeats_path,
        "verification_dir": verification_dir,
        "integration_dir": integration_dir,
    }


def build_request_id(seq: int) -> str:
    return f"R-{seq:04d}"


def build_binding_id(seq: int) -> str:
    return f"RB-{seq:04d}"


def build_rca_id(seq: int) -> str:
    return f"RCA-{seq:04d}"


def request_snapshot_path(files: dict, kind: str) -> Path | None:
    if kind not in REQUEST_SNAPSHOT_KINDS:
        return None
    return files["harness"] / f"{kind}-requests.json"


def update_request_snapshot(files: dict, request: dict, *, generator: str):
    snapshot_path = request_snapshot_path(files, request.get("kind") or "")
    if snapshot_path is None:
        return None
    snapshot = load_optional_json(snapshot_path, {
        "schemaVersion": SCHEMA_VERSION,
        "generator": generator,
        "generatedAt": now_iso(),
        "requests": [],
    })
    requests = [item for item in snapshot.get("requests", []) if item.get("requestId") != request.get("requestId")]
    requests.append({
        "requestId": request.get("requestId"),
        "kind": request.get("kind"),
        "goal": request.get("goal"),
        "source": request.get("source"),
        "status": request.get("status"),
        "parentRequestId": request.get("parentRequestId"),
        "originTaskId": request.get("originTaskId"),
        "originSessionId": request.get("originSessionId"),
        "createdAt": request.get("createdAt"),
        "updatedAt": request.get("updatedAt"),
    })
    snapshot["requests"] = requests[-50:]
    snapshot["generatedAt"] = now_iso()
    snapshot["generator"] = generator
    write_json(snapshot_path, snapshot)
    return snapshot


def compact_log_path(root: Path, task_id: str) -> Path:
    return root / ".harness" / f"log-{task_id}.md"


def front_matter_value(value):
    if value is None:
        return "null"
    if isinstance(value, bool):
        return "true" if value else "false"
    if isinstance(value, (int, float)):
        return str(value)
    return json.dumps(value, ensure_ascii=False)


def dump_front_matter(data: dict) -> str:
    lines = ["---"]
    for key, value in data.items():
        lines.append(f"{key}: {front_matter_value(value)}")
    lines.append("---")
    return "\n".join(lines)


def parse_front_matter(markdown_text: str) -> tuple[dict, str]:
    if not markdown_text.startswith("---\n"):
        return {}, markdown_text
    end = markdown_text.find("\n---\n", 4)
    if end == -1:
        return {}, markdown_text
    block = markdown_text[4:end]
    body = markdown_text[end + 5 :]
    data = {}
    current_list_key = None
    for line in block.splitlines():
        stripped = line.strip()
        if current_list_key and stripped.startswith("- "):
            data.setdefault(current_list_key, []).append(stripped[2:].strip().strip('"'))
            continue
        if ":" not in line:
            continue
        key, raw_value = line.split(":", 1)
        value = raw_value.strip()
        if not value:
            data[key.strip()] = []
            current_list_key = key.strip()
            continue
        current_list_key = None
        try:
            data[key.strip()] = json.loads(value)
        except json.JSONDecodeError:
            data[key.strip()] = value.strip('"')
    return data, body.lstrip("\n")


def markdown_section(body: str, title: str) -> str:
    pattern = rf"^#{{1,2}} {re.escape(title)}\n([\s\S]*?)(?=^#{{1,2}} |\Z)"
    match = re.search(pattern, body, flags=re.MULTILINE)
    if not match:
        return ""
    return match.group(1).strip()


def extract_handoff_json(raw_text: str) -> dict | None:
    patterns = [
        r"```KLEIN_HANDOFF_JSON\s*(\{[\s\S]*?\})\s*```",
        r"```json\s+KLEIN_HANDOFF_JSON\s*(\{[\s\S]*?\})\s*```",
    ]
    for pattern in patterns:
        match = re.search(pattern, raw_text)
        if not match:
            continue
        try:
            parsed = json.loads(match.group(1))
            return parsed if isinstance(parsed, dict) else None
        except json.JSONDecodeError:
            continue
    return None


def detect_log_severity(raw_text: str, verification_report: dict | None, task: dict) -> str:
    lowered = raw_text.lower()
    if (verification_report or {}).get("overallStatus") == "fail" or task.get("status") == "recoverable":
        return "error"
    if any(token in lowered for token in ("error", "failed", "warning", "warn", "timeout", "recoverable")):
        return "warn"
    return "info"


def log_tags(task: dict, route_decision: dict | None, verification_report: dict | None, severity: str, handoff: dict | None) -> list[str]:
    tags = []
    if task.get("kind"):
        tags.append(f"kind:{task.get('kind')}")
    if task.get("roleHint"):
        tags.append(f"role:{task.get('roleHint')}")
    if severity != "info":
        tags.append(f"severity:{severity}")
    if task.get("status"):
        tags.append(f"status:{task.get('status')}")
    if route_decision and route_decision.get("resumeStrategy"):
        tags.append(f"resume:{route_decision.get('resumeStrategy')}")
    if verification_report and verification_report.get("overallStatus"):
        tags.append(f"verify:{verification_report.get('overallStatus')}")
    for candidate in (handoff or {}).get("tags", []) or []:
        if isinstance(candidate, str) and candidate.strip():
            tags.append(candidate.strip())
    return list(dict.fromkeys(tags))


def slim_text_lines(text: str, *, limit: int) -> list[str]:
    lines = [line.strip() for line in (text or "").splitlines() if line.strip()]
    return lines[:limit]


def normalize_string_list(value, *, limit: int) -> list[str]:
    result = []
    if isinstance(value, list):
        candidates = value
    elif isinstance(value, str):
        candidates = [item.strip("- ").strip() for item in value.splitlines()]
    else:
        candidates = []
    for item in candidates:
        if not isinstance(item, str):
            continue
        cleaned = item.strip()
        if cleaned:
            result.append(cleaned)
    return result[:limit]


def fallback_handoff_summary(task: dict, route_decision: dict | None, raw_text: str, verification_report: dict | None):
    tail_lines = slim_text_lines("\n".join(raw_text.splitlines()[-40:]), limit=8)
    changed_paths = []
    diff_summary = task.get("diffSummary") or ""
    if diff_summary:
        changed_paths.append(diff_summary)
    facts = [
        f"task status: {task.get('status')}",
        f"resume strategy: {(route_decision or {}).get('resumeStrategy') or task.get('resumeStrategy')}",
        f"verification: {(verification_report or {}).get('overallStatus') or task.get('verificationStatus') or 'unknown'}",
    ]
    if task.get("worktreePath"):
        facts.append(f"worktree: {task.get('worktreePath')}")
    facts.extend(tail_lines[:2])
    blockers = []
    if task.get("status") == "recoverable":
        blockers.append(task.get("recoveryReason") or "task is recoverable")
    if (verification_report or {}).get("overallStatus") == "fail":
        blockers.append(task.get("verificationSummary") or "verification failed")
    verification_notes = []
    if verification_report:
        verification_notes.append(f"overallStatus: {verification_report.get('overallStatus')}")
        if verification_report.get("failedRuleIds"):
            verification_notes.append(f"failedRuleIds: {verification_report.get('failedRuleIds')}")
    return {
        "oneScreenSummary": facts[:8],
        "crossWorkerFacts": facts[:6],
        "decisionsAssumptions": [
            f"routingMode: {(route_decision or {}).get('routingMode')}",
            f"gateReason: {(route_decision or {}).get('gateReason')}",
        ],
        "touchedContractsPaths": normalize_string_list(task.get("ownedPaths", []), limit=6) or changed_paths[:6],
        "blockersRisks": blockers[:6],
        "verification": verification_notes[:6],
        "evidenceRefs": [],
        "openQuestions": [],
        "tags": [],
    }


def read_text_tail(path: Path, *, max_lines: int = 120) -> str:
    if not path.exists():
        return ""
    lines = path.read_text(encoding="utf-8", errors="ignore").splitlines()
    return "\n".join(lines[-max_lines:])


def build_compact_log_artifact(root: Path, task: dict, binding: dict | None, tmux_session: str | None, route_decision: dict | None, *, generator: str) -> dict:
    files = ensure_runtime_scaffold(root, generator=generator)
    task_id = task.get("taskId")
    raw_log_rel = f".harness/state/runner-logs/{task_id}.log"
    raw_log_path = files["state_dir"] / "runner-logs" / f"{task_id}.log"
    prompt_rel = f".harness/state/runner-prompt-{task_id}.md"
    prompt_path = files["state_dir"] / f"runner-prompt-{task_id}.md"
    verification_rel = task.get("verificationResultPath")
    verification_report = read_verification_report(root, verification_rel)
    raw_tail = read_text_tail(raw_log_path, max_lines=120)
    handoff = extract_handoff_json(raw_tail)
    fallback = fallback_handoff_summary(task, route_decision, raw_tail, verification_report)
    source = handoff or {}
    severity = detect_log_severity(raw_tail, verification_report, task)
    tags = log_tags(task, route_decision, verification_report, severity, source)
    binding_lineage = (binding or {}).get("lineage", {})
    one_screen = normalize_string_list(source.get("oneScreenSummary"), limit=8) or fallback["oneScreenSummary"]
    facts = normalize_string_list(source.get("crossWorkerFacts"), limit=6) or fallback["crossWorkerFacts"]
    decisions = normalize_string_list(source.get("decisionsAssumptions"), limit=6) or fallback["decisionsAssumptions"]
    touched = normalize_string_list(source.get("touchedContractsPaths"), limit=6) or fallback["touchedContractsPaths"]
    blockers = normalize_string_list(source.get("blockersRisks"), limit=6) or fallback["blockersRisks"]
    verification_notes = normalize_string_list(source.get("verification"), limit=6) or fallback["verification"]
    evidence_refs = normalize_string_list(source.get("evidenceRefs"), limit=8)
    if raw_log_path.exists():
        evidence_refs.append(raw_log_rel)
    if prompt_path.exists():
        evidence_refs.append(prompt_rel)
    if verification_rel:
        evidence_refs.append(verification_rel)
    evidence_refs = list(dict.fromkeys(evidence_refs))
    open_questions = normalize_string_list(source.get("openQuestions"), limit=6)
    compact_path = compact_log_path(root, task_id)
    front_matter = {
        "schemaVersion": SCHEMA_VERSION,
        "generator": "harness-log-compact",
        "generatedAt": now_iso(),
        "taskId": task_id,
        "requestId": (binding or {}).get("requestId"),
        "bindingId": (binding or {}).get("bindingId"),
        "sessionId": binding_lineage.get("sessionId") or task.get("claim", {}).get("boundSessionId") or task.get("lastKnownSessionId"),
        "tmuxSession": tmux_session or task.get("claim", {}).get("tmuxSession") or f"print:{task_id}",
        "roleHint": task.get("roleHint"),
        "kind": task.get("kind"),
        "status": task.get("status"),
        "shareability": "cross-worker",
        "rawLogPath": raw_log_rel,
        "promptPath": prompt_rel if prompt_path.exists() else None,
        "verificationResultPath": verification_rel,
        "diffSummaryPath": None,
        "ownedPaths": task.get("ownedPaths", []),
        "tags": tags,
        "severity": severity,
    }
    body_lines = [
        "# One-screen summary",
        *([line for line in one_screen] or ["No summary emitted."]),
        "",
        "## Cross-worker relevant facts",
        *([f"- {line}" for line in facts] or ["- none"]),
        "",
        "## Decisions and assumptions",
        *([f"- {line}" for line in decisions] or ["- none"]),
        "",
        "## Touched contracts / paths",
        *([f"- {line}" for line in touched] or ["- none"]),
        "",
        "## Blockers / risks",
        *([f"- {line}" for line in blockers] or ["- none"]),
        "",
        "## Verification",
        *([f"- {line}" for line in verification_notes] or ["- none"]),
        "",
        "## Evidence refs",
        *([f"- {line}" for line in evidence_refs] or ["- none"]),
    ]
    if open_questions:
        body_lines.extend(["", "## Open questions", *[f"- {line}" for line in open_questions]])
    compact_path.write_text(dump_front_matter(front_matter) + "\n\n" + "\n".join(body_lines).rstrip() + "\n", encoding="utf-8")
    return {
        "path": compact_path,
        "frontMatter": front_matter,
        "body": "\n".join(body_lines),
    }


def collect_compact_log_entries(root: Path):
    harness = root / ".harness"
    entries = []
    for path in sorted(harness.glob("log-*.md")):
        front_matter, body = parse_front_matter(path.read_text(encoding="utf-8", errors="ignore"))
        entries.append({
            "path": str(path.relative_to(root)),
            "frontMatter": front_matter,
            "body": body,
            "oneScreenSummary": markdown_section(body, "One-screen summary").splitlines()[:8],
            "facts": [line.lstrip("- ").strip() for line in markdown_section(body, "Cross-worker relevant facts").splitlines() if line.strip()],
            "blockers": [line.lstrip("- ").strip() for line in markdown_section(body, "Blockers / risks").splitlines() if line.strip() and line.strip() != "- none"],
            "evidenceRefs": [line.lstrip("- ").strip() for line in markdown_section(body, "Evidence refs").splitlines() if line.strip()],
        })
    return entries


def build_log_index(root: Path):
    entries = collect_compact_log_entries(root)
    by_task = {}
    by_request = {}
    by_session = {}
    high_signal = []
    blockers = []
    tag_counter = Counter()
    for entry in entries:
        meta = entry["frontMatter"]
        task_id = meta.get("taskId")
        request_id = meta.get("requestId")
        session_id = meta.get("sessionId")
        if task_id:
            by_task[task_id] = {
                "taskId": task_id,
                "path": entry["path"],
                "severity": meta.get("severity"),
                "status": meta.get("status"),
                "tags": meta.get("tags", []),
                "requestId": request_id,
                "sessionId": session_id,
                "summary": entry["oneScreenSummary"][:3],
            }
        if request_id:
            by_request.setdefault(request_id, []).append(task_id)
        if session_id:
            by_session.setdefault(session_id, []).append(task_id)
        for tag in meta.get("tags", []) or []:
            tag_counter[tag] += 1
        if meta.get("severity") in {"warn", "error"}:
            high_signal.append({
                "taskId": task_id,
                "requestId": request_id,
                "path": entry["path"],
                "severity": meta.get("severity"),
                "status": meta.get("status"),
                "summary": entry["oneScreenSummary"][:3],
            })
        if entry["blockers"]:
            blockers.append({
                "taskId": task_id,
                "requestId": request_id,
                "path": entry["path"],
                "blockers": entry["blockers"][:3],
            })
    return {
        "schemaVersion": SCHEMA_VERSION,
        "generator": "harness-log-index",
        "generatedAt": now_iso(),
        "compactLogCount": len(entries),
        "logsByTaskId": by_task,
        "logsByRequestId": by_request,
        "logsBySessionId": by_session,
        "recentHighSignalLogs": high_signal[-10:],
        "openBlockers": blockers[-10:],
        "recurringTags": {tag: count for tag, count in tag_counter.items() if count > 1},
    }


def write_log_index(root: Path, *, generator: str):
    files = ensure_runtime_scaffold(root, generator=generator)
    index = build_log_index(root)
    index["generator"] = generator
    index["generatedAt"] = now_iso()
    write_json(files["log_index_path"], index)
    return index


def extract_raw_log_windows(path: Path, *, keywords: list[str] | None = None, task_id: str | None = None, window: int = 8, limit: int = 3):
    if not path.exists():
        return []
    lines = path.read_text(encoding="utf-8", errors="ignore").splitlines()
    lowered_keywords = [keyword.lower() for keyword in (keywords or []) if keyword]
    hits = []
    for idx, line in enumerate(lines):
        lowered = line.lower()
        if lowered_keywords and not any(keyword in lowered for keyword in lowered_keywords):
            continue
        if not lowered_keywords and task_id and task_id not in line:
            continue
        start = max(0, idx - window)
        end = min(len(lines), idx + window + 1)
        snippet = "\n".join(lines[start:end]).strip()
        if snippet:
            hits.append({
                "lineStart": start + 1,
                "lineEnd": end,
                "snippet": snippet,
            })
        if len(hits) >= limit:
            break
    if not hits and lines:
        tail_start = max(0, len(lines) - (window * 2))
        hits.append({
            "lineStart": tail_start + 1,
            "lineEnd": len(lines),
            "snippet": "\n".join(lines[tail_start:]).strip(),
        })
    return hits


def parse_research_memo(path: Path):
    text = path.read_text(encoding="utf-8", errors="ignore")
    front_matter, body = parse_front_matter(text)
    slug = path.stem
    return {
        "slug": slug,
        "path": str(path),
        "frontMatter": front_matter,
        "body": body,
        "title": front_matter.get("title") or slug,
        "researchMode": front_matter.get("researchMode") or "targeted",
        "question": front_matter.get("question"),
        "sources": front_matter.get("sources", []),
        "summary": markdown_section(body, "Summary").splitlines()[:4] or [line.strip() for line in body.splitlines() if line.strip()][:4],
    }


def build_research_index(root: Path):
    research_dir = root / ".harness" / "research"
    memos = [parse_research_memo(path) for path in sorted(research_dir.glob("*.md"))]
    return {
        "schemaVersion": SCHEMA_VERSION,
        "generator": "klein-harness",
        "generatedAt": now_iso(),
        "memoCount": len(memos),
        "researchModes": dict(Counter(memo.get("researchMode", "none") for memo in memos)),
        "recentMemos": [
            {
                "slug": memo.get("slug"),
                "title": memo.get("title"),
                "researchMode": memo.get("researchMode"),
                "path": str(Path(memo.get("path")).relative_to(root)),
                "summary": memo.get("summary", [])[:2],
            }
            for memo in memos[-10:]
        ],
        "bySlug": {
            memo.get("slug"): {
                "title": memo.get("title"),
                "researchMode": memo.get("researchMode"),
                "path": str(Path(memo.get("path")).relative_to(root)),
                "question": memo.get("question"),
                "sources": memo.get("sources", []),
            }
            for memo in memos
        },
    }


def write_research_index(root: Path, *, generator: str):
    files = ensure_runtime_scaffold(root, generator=generator)
    index = build_research_index(root)
    index["generator"] = generator
    index["generatedAt"] = now_iso()
    write_json(files["research_index_path"], index)
    return index


def build_research_summary(research_index: dict, *, generator: str) -> dict:
    return {
        "schemaVersion": SCHEMA_VERSION,
        "generator": generator,
        "generatedAt": now_iso(),
        "memoCount": research_index.get("memoCount", 0),
        "researchModes": research_index.get("researchModes", {}),
        "recentMemos": bounded(research_index.get("recentMemos", []), DEFAULT_POLICY_SUMMARY["hotState"]["maxRecentLogs"]),
        "bySlug": research_index.get("bySlug", {}),
    }


def build_queue_summary(request_index: dict, request_summary: dict, *, generator: str, policy_summary: dict) -> dict:
    requests = request_index.get("requests", [])
    queued = [item for item in requests if item.get("status") == "queued"]
    bound = [item for item in requests if item.get("status") == "bound"]
    blocked = [item for item in requests if item.get("status") == "blocked"]
    recent_limit = policy_summary["queue"]["maxRecentRequests"]
    queued_sorted = sorted(
        queued,
        key=lambda item: (priority_rank(item.get("priority"), policy_summary["queue"]["priorityOrder"]), item.get("createdAt") or ""),
    )
    return {
        "schemaVersion": SCHEMA_VERSION,
        "generator": generator,
        "generatedAt": now_iso(),
        "totalRequests": len(requests),
        "queueDepth": len(queued),
        "queuedRequestCount": len(queued),
        "boundRequestCount": request_summary.get("boundRequestCount", 0),
        "runningRequestCount": request_summary.get("runningRequestCount", 0),
        "recoverableRequestCount": request_summary.get("recoverableRequestCount", 0),
        "blockedRequestCount": request_summary.get("blockedRequestCount", 0),
        "completedRequestCount": request_summary.get("completedRequestCount", 0),
        "duplicateRequestCount": request_summary.get("duplicateRequestCount", 0),
        "contextMergeCount": request_summary.get("contextMergeCount", 0),
        "inspectionOverlayCount": request_summary.get("inspectionOverlayCount", 0),
        "cancelledRequestCount": sum(1 for item in requests if item.get("status") == "cancelled"),
        "queuedByKind": dict(Counter(item.get("kind", "unknown") for item in queued)),
        "queuedByPriority": dict(Counter(item.get("priority", "unknown") for item in queued)),
        "oldestQueuedAt": queued_sorted[0].get("createdAt") if queued_sorted else None,
        "activeQueueHead": queued_sorted[0] if queued_sorted else None,
        "unboundQueuedRequestCount": sum(1 for item in queued if not item.get("boundTaskIds")),
        "recentQueuedRequests": bounded(
            [
                {
                    "requestId": item.get("requestId"),
                    "kind": item.get("kind"),
                    "normalizedIntentClass": item.get("normalizedIntentClass"),
                    "fusionDecision": item.get("fusionDecision"),
                    "priority": item.get("priority"),
                    "goal": item.get("goal"),
                    "threadKey": thread_key_from_request(item),
                    "targetPlanEpoch": item.get("targetPlanEpoch"),
                    "createdAt": item.get("createdAt"),
                }
                for item in queued_sorted
            ],
            recent_limit,
        ),
        "recentBlockedRequests": bounded(
            [
                {
                    "requestId": item.get("requestId"),
                    "kind": item.get("kind"),
                    "normalizedIntentClass": item.get("normalizedIntentClass"),
                    "fusionDecision": item.get("fusionDecision"),
                    "goal": item.get("goal"),
                    "statusReason": item.get("statusReason"),
                    "updatedAt": item.get("updatedAt"),
                }
                for item in blocked
            ],
            policy_summary["queue"]["maxBlockedItems"],
        ),
        "recentBoundRequests": bounded(
            [
                {
                    "requestId": item.get("requestId"),
                    "kind": item.get("kind"),
                    "boundTaskIds": item.get("boundTaskIds", []),
                    "bindingIds": item.get("bindingIds", []),
                    "updatedAt": item.get("updatedAt"),
                }
                for item in bound
            ],
            recent_limit,
        ),
    }


def build_task_summary(task_pool: dict, feedback_summary: dict, lineage_index: dict, runner_state: dict, *, generator: str, policy_summary: dict) -> dict:
    tasks = task_pool.get("tasks", [])
    hot_limit = policy_summary["hotState"]["maxActiveItems"]
    dispatchable_ids = bounded(runner_state.get("dispatchableTaskIds", []), hot_limit)
    recoverable_ids = bounded([item.get("taskId") for item in runner_state.get("recoverableRuns", []) if item.get("taskId")], hot_limit)
    blocked_routes = bounded(runner_state.get("blockedRoutes", []), hot_limit)
    task_feedback = feedback_summary.get("taskFeedbackSummary", {})
    return {
        "schemaVersion": SCHEMA_VERSION,
        "generator": generator,
        "generatedAt": now_iso(),
        "totalTaskCount": len(tasks),
        "taskStatusCounts": dict(Counter(task.get("status", "unknown") for task in tasks)),
        "taskKindCounts": dict(Counter(task.get("kind", "unknown") for task in tasks)),
        "threadKeyCount": len({task.get("threadKey") for task in tasks if task.get("threadKey")}),
        "planEpochs": dict(Counter(str(task.get("planEpoch")) for task in tasks if task.get("planEpoch") is not None)),
        "roleHintCounts": dict(Counter(task.get("roleHint", "unknown") for task in tasks)),
        "workerModeCounts": dict(Counter(task.get("workerMode", "unknown") for task in tasks if task.get("workerMode"))),
        "worktreeCount": sum(1 for task in tasks if task.get("worktreePath")),
        "worktreePreparedCount": sum(1 for task in tasks if task.get("worktreePreparedAt")),
        "mergeStatusCounts": dict(Counter(task.get("mergeStatus", "none") for task in tasks if task.get("mergeStatus"))),
        "dispatchableTaskIds": dispatchable_ids,
        "recoverableTaskIds": recoverable_ids,
        "supersededTaskIds": bounded([task.get("taskId") for task in tasks if task.get("status") in TASK_SUPERSEDED_STATUSES], hot_limit),
        "blockedRoutes": blocked_routes,
        "verifiedTaskCount": sum(1 for task in tasks if task.get("verificationStatus") in {"pass", "skipped"}),
        "failingVerificationCount": sum(1 for task in tasks if task.get("verificationStatus") == "fail"),
        "tasksWithRecentFailures": bounded(
            [
                {
                    "taskId": task_id,
                    "latestFeedbackType": summary.get("latestFeedbackType"),
                    "latestSeverity": summary.get("latestSeverity"),
                    "latestMessage": summary.get("latestMessage"),
                }
                for task_id, summary in task_feedback.items()
                if summary.get("recentFailures")
            ],
            hot_limit,
        ),
        "activeTasks": bounded(
            [
                {
                    "taskId": task.get("taskId"),
                    "kind": task.get("kind"),
                    "roleHint": task.get("roleHint"),
                    "status": task.get("status"),
                    "threadKey": task.get("threadKey"),
                    "planEpoch": task.get("planEpoch"),
                    "title": task.get("title"),
                    "nodeId": task.get("claim", {}).get("nodeId"),
                    "branchName": task.get("branchName"),
                    "baseRef": task.get("baseRef") or (task.get("dispatch") or {}).get("baseRef"),
                    "worktreePath": task.get("worktreePath"),
                    "worktreeStatus": task.get("worktreeStatus"),
                    "integrationBranch": integration_branch_for_task(task, task_pool),
                    "mergeStatus": task.get("mergeStatus"),
                    "cleanupStatus": task.get("cleanupStatus"),
                    "dispatchBackend": task.get("claim", {}).get("dispatchBackend"),
                    "boundSessionId": task.get("claim", {}).get("boundSessionId"),
                }
                for task in tasks
                if task.get("status") in TASK_ACTIVE_STATUSES
            ],
            hot_limit,
        ),
        "lineageTaskCount": len(lineage_index.get("tasks", {})),
    }


def build_worker_summary(task_pool: dict, session_registry: dict, runner_state: dict, heartbeats: dict, *, generator: str, policy_summary: dict) -> dict:
    tasks = {task.get("taskId"): task for task in task_pool.get("tasks", [])}
    entries = []
    seen = set()
    for source in (runner_state.get("activeRuns", []), runner_state.get("recoverableRuns", []), runner_state.get("staleRuns", [])):
        for item in source:
            task_id = item.get("taskId")
            if task_id and task_id not in seen:
                seen.add(task_id)
    for task_id in seen:
        task = tasks.get(task_id, {})
        heartbeat = heartbeats.get(task_id, {})
        claim = task.get("claim", {})
        dispatch_backend = (
            claim.get("dispatchBackend")
            or next((run.get("dispatchBackend") or run.get("dispatchMode") for run in runner_state.get("activeRuns", []) if run.get("taskId") == task_id), None)
            or backend_kind_from_label(claim.get("nodeId") or heartbeat.get("tmuxSession"), None)
        )
        backend_session = heartbeat.get("backendSession") or heartbeat.get("tmuxSession") or claim.get("tmuxSession")
        backend_status = assess_backend_health(dispatch_backend or "unknown", backend_session)
        heartbeat_age = age_seconds(heartbeat.get("lastHeartbeatAt"))
        stale_after = policy_summary["heartbeat"]["staleAfterSeconds"]
        node_health = "stale" if heartbeat_age is not None and heartbeat_age > stale_after else "healthy"
        if task.get("status") == "recoverable":
            node_health = "recoverable"
        if backend_status.get("dispatchBackend") == "print":
            node_health = "non-executing"
        entries.append(
            {
                "taskId": task_id,
                "threadKey": task.get("threadKey"),
                "planEpoch": task.get("planEpoch"),
                "roleHint": task.get("roleHint"),
                "kind": task.get("kind"),
                "status": task.get("status"),
                "workerMode": task.get("workerMode"),
                "nodeId": claim.get("nodeId"),
                "dispatchBackend": dispatch_backend,
                "backendSession": backend_session,
                "backendHealth": backend_status.get("backendHealth"),
                "backendReachable": backend_status.get("backendReachable"),
                "nodeHealth": node_health,
                "boundSessionId": claim.get("boundSessionId"),
                "branchName": task.get("branchName"),
                "baseRef": task.get("baseRef") or (task.get("dispatch") or {}).get("baseRef"),
                "worktreePath": task.get("worktreePath"),
                "integrationBranch": integration_branch_for_task(task, task_pool),
                "mergeStatus": task.get("mergeStatus"),
                "lastHeartbeatAt": heartbeat.get("lastHeartbeatAt"),
                "heartbeatAgeSeconds": heartbeat_age,
                "lastKnownPhase": heartbeat.get("lastKnownPhase"),
                "lastExitCode": heartbeat.get("lastExitCode"),
            }
        )
    hot_limit = policy_summary["hotState"]["maxActiveItems"]
    active_bindings = bounded(session_registry.get("activeBindings", []), hot_limit)
    return {
        "schemaVersion": SCHEMA_VERSION,
        "generator": generator,
        "generatedAt": now_iso(),
        "workerCount": len(entries),
        "healthyWorkerCount": sum(1 for item in entries if item.get("nodeHealth") == "healthy"),
        "staleWorkerCount": sum(1 for item in entries if item.get("nodeHealth") == "stale"),
        "recoverableWorkerCount": sum(1 for item in entries if item.get("nodeHealth") == "recoverable"),
        "nonExecutingWorkerCount": sum(1 for item in entries if item.get("nodeHealth") == "non-executing"),
        "dispatchBackendCounts": dict(Counter(item.get("dispatchBackend", "unknown") for item in entries)),
        "workerNodes": bounded(entries, hot_limit),
        "activeBindings": active_bindings,
        "runtimeToWorkerMap": bounded(
            [
                {
                    "taskId": item.get("taskId"),
                    "dispatchBackend": item.get("dispatchBackend"),
                    "nodeId": item.get("nodeId"),
                    "backendSession": item.get("backendSession"),
                    "boundSessionId": item.get("boundSessionId"),
                    "worktreePath": item.get("worktreePath"),
                    "branchName": item.get("branchName"),
                    "nodeHealth": item.get("nodeHealth"),
                }
                for item in entries
            ],
            hot_limit,
        ),
    }


def build_daemon_summary(daemon_state: dict, runner_state: dict, worker_summary: dict, *, generator: str, policy_summary: dict) -> dict:
    degraded_after = policy_summary["daemon"]["degradedAfterSeconds"]
    tick_age = age_seconds(daemon_state.get("lastTickAt"))
    runtime_health = "stopped"
    if daemon_state.get("status") == "running":
        runtime_health = "healthy"
        if tick_age is None or tick_age > degraded_after or daemon_state.get("lastError"):
            runtime_health = "degraded"
    session_name = daemon_state.get("sessionName")
    backend_default = daemon_state.get("dispatchMode") or policy_summary["dispatch"]["defaultBackend"]
    session_alive = None
    if session_name:
        session_alive = tmux_session_alive(session_name)
    return {
        "schemaVersion": SCHEMA_VERSION,
        "generator": generator,
        "generatedAt": now_iso(),
        "status": daemon_state.get("status"),
        "runtimeHealth": runtime_health,
        "dispatchBackendDefault": backend_default,
        "backendAware": True,
        "sessionName": session_name,
        "sessionAlive": session_alive,
        "intervalSeconds": daemon_state.get("intervalSeconds"),
        "lastTickAt": daemon_state.get("lastTickAt"),
        "lastTickAgeSeconds": tick_age,
        "lastRefreshAt": daemon_state.get("lastRefreshAt"),
        "lastError": daemon_state.get("lastError"),
        "restartCount": daemon_state.get("restartCount", 0),
        "recentEvents": bounded(daemon_state.get("recentEvents", []), policy_summary["daemon"]["maxRecentErrors"]),
        "activeRunnerCount": len(runner_state.get("activeRuns", [])),
        "recoverableTaskCount": len(runner_state.get("recoverableRuns", [])),
        "staleRunnerCount": len(runner_state.get("staleRuns", [])),
        "blockedRouteCount": len(runner_state.get("blockedRoutes", [])),
        "backendState": {
            "defaultDispatchBackend": backend_default,
            "workerBackendCounts": worker_summary.get("dispatchBackendCounts", {}),
        },
        "workerBackendHealth": worker_summary.get("dispatchBackendCounts", {}),
        "runtimeToWorkerMap": worker_summary.get("runtimeToWorkerMap", []),
    }


def build_progress_summary(
    progress: dict,
    request_summary: dict,
    task_summary: dict,
    worker_summary: dict,
    daemon_summary: dict,
    *,
    generator: str,
) -> dict:
    snapshot = default_progress_state()
    snapshot.update(progress or {})
    snapshot.update(
        {
            "schemaVersion": SCHEMA_VERSION,
            "generator": generator,
            "generatedAt": now_iso(),
            "queueDepth": request_summary.get("requestCounts", {}).get("queued", 0),
            "activeTaskCount": task_summary.get("taskStatusCounts", {}).get("running", 0) + task_summary.get("taskStatusCounts", {}).get("dispatched", 0) + task_summary.get("taskStatusCounts", {}).get("resumed", 0) + task_summary.get("taskStatusCounts", {}).get("active", 0) + task_summary.get("taskStatusCounts", {}).get("claimed", 0) + task_summary.get("taskStatusCounts", {}).get("in_progress", 0),
            "recoverableTaskCount": len(task_summary.get("recoverableTaskIds", [])),
            "supersededTaskCount": len(task_summary.get("supersededTaskIds", [])),
            "activeWorkerCount": worker_summary.get("workerCount", 0),
            "runtimeHealth": daemon_summary.get("runtimeHealth"),
            "dispatchBackendDefault": daemon_summary.get("dispatchBackendDefault"),
        }
    )
    return snapshot


def normalize_context_paths(root: Path, values: list[str]) -> list[str]:
    result = []
    for value in values:
        path = Path(value)
        if not path.is_absolute():
            path = (root / value).resolve()
        else:
            path = path.resolve()
        if not path.exists():
            raise FileNotFoundError(f"context path not found: {value}")
        result.append(str(path))
    return result


def find_request(requests: list[dict], request_id: str):
    for request in requests:
        if request.get("requestId") == request_id:
            return request
    raise KeyError(f"request not found: {request_id}")


def find_task(tasks: list[dict], task_id: str):
    for task in tasks:
        if task.get("taskId") == task_id:
            return task
    raise KeyError(f"task not found: {task_id}")


def find_binding(bindings: list[dict], request_id: str, task_id: str):
    for binding in bindings:
        if binding.get("requestId") == request_id and binding.get("taskId") == task_id:
            return binding
    return None


def upsert_request_record(index: dict, request: dict):
    requests = index.setdefault("requests", [])
    for idx, existing in enumerate(requests):
        if existing.get("requestId") == request.get("requestId"):
            requests[idx] = request
            return request
    requests.append(request)
    return request


def read_task_pool(harness: Path):
    task_pool_path = harness / "task-pool.json"
    if not task_pool_path.exists():
        return None
    return load_json(task_pool_path)


def sanitize_ref_fragment(value: str) -> str:
    text = re.sub(r"[^A-Za-z0-9._-]+", "-", value or "").strip("-")
    return text or "default"


def task_merge_required(task: dict) -> bool:
    if task.get("mergeRequired") is not None:
        return bool(task.get("mergeRequired"))
    return bool(task.get("handoff", {}).get("mergeRequired"))


def task_requires_dedicated_worktree(task: dict, tasks: list[dict] | None = None) -> bool:
    kind = (task.get("kind") or "").lower()
    role = (task.get("roleHint") or "").lower()
    worker_mode = (task.get("workerMode") or "").lower()
    owned_paths = task.get("ownedPaths") or []
    code_like = any(not str(path).startswith(".harness/") for path in owned_paths)
    active_code_workers = [
        other for other in (tasks or [])
        if other.get("taskId") != task.get("taskId")
        and other.get("status") in TASK_ACTIVE_STATUSES
        and any(not str(path).startswith(".harness/") for path in (other.get("ownedPaths") or []))
    ]
    if role == "orchestrator" or kind in {"audit", "analysis", "research", "replan", "rollback", "merge", "lease-recovery"}:
        return False
    if worker_mode == "audit":
        return False
    if task_merge_required(task):
        return True
    if code_like:
        return True
    if len(owned_paths) > 1:
        return True
    if active_code_workers:
        return True
    return False


def control_plane_only_task(task: dict) -> bool:
    return not task_requires_dedicated_worktree(task)


def default_worktree_registry(*, generator: str) -> dict:
    return {
        "schemaVersion": SCHEMA_VERSION,
        "generator": generator,
        "generatedAt": now_iso(),
        "worktrees": [],
    }


def load_worktree_registry(files: dict, *, generator: str) -> dict:
    return load_optional_json(files["worktree_registry_path"], default_worktree_registry(generator=generator)) or default_worktree_registry(generator=generator)


def default_merge_queue(*, generator: str) -> dict:
    return {
        "schemaVersion": SCHEMA_VERSION,
        "generator": generator,
        "generatedAt": now_iso(),
        "integrationBranch": None,
        "items": [],
    }


def load_merge_queue(files: dict, *, generator: str) -> dict:
    return load_optional_json(files["merge_queue_path"], default_merge_queue(generator=generator)) or default_merge_queue(generator=generator)


def default_merge_summary(*, generator: str) -> dict:
    return {
        "schemaVersion": SCHEMA_VERSION,
        "generator": generator,
        "generatedAt": now_iso(),
        "integrationBranch": None,
        "queueDepth": 0,
        "readyToMergeCount": 0,
        "conflictCount": 0,
        "mergedCount": 0,
        "openConflicts": [],
        "readyToMerge": [],
        "recentMerged": [],
        "supersededCandidates": [],
    }


def integration_branch_for_task(task: dict, task_pool: dict | None = None) -> str | None:
    return (
        task.get("integrationBranch")
        or (task.get("dispatch") or {}).get("integrationBranch")
        or (task_pool or {}).get("integrationBranch")
        or task.get("baseRef")
        or (task.get("dispatch") or {}).get("baseRef")
    )


def run_command(cmd: list[str], *, cwd: Path, check: bool = True):
    return subprocess.run(cmd, cwd=str(cwd), check=check, text=True, capture_output=True)


def git_ref_exists(root: Path, ref_name: str) -> bool:
    try:
        run_command(["git", "rev-parse", "--verify", ref_name], cwd=root)
        return True
    except subprocess.CalledProcessError:
        return False


def ensure_integration_branch(root: Path, integration_branch: str, base_ref: str | None) -> bool:
    if not integration_branch:
        return False
    if git_ref_exists(root, integration_branch):
        return True
    seed_ref = base_ref or "HEAD"
    try:
        run_command(["git", "branch", integration_branch, seed_ref], cwd=root)
        return True
    except subprocess.CalledProcessError:
        return False


def integration_worktree_path(root: Path, integration_branch: str) -> Path:
    return root / ".harness" / ".integration" / sanitize_ref_fragment(integration_branch)


def ensure_integration_worktree(root: Path, integration_branch: str, base_ref: str | None) -> Path | None:
    if not integration_branch:
        return None
    if not ensure_integration_branch(root, integration_branch, base_ref):
        return None
    path = integration_worktree_path(root, integration_branch)
    if path.exists():
        return path
    path.parent.mkdir(parents=True, exist_ok=True)
    try:
        run_command(["git", "worktree", "add", str(path), integration_branch], cwd=root)
    except subprocess.CalledProcessError:
        return None
    return path


def upsert_worktree_registry_entry(root: Path, task: dict, *, generator: str, status: str, cleanup_status: str | None = None, extra: dict | None = None) -> dict:
    files = ensure_runtime_scaffold(root, generator=generator)
    registry = load_worktree_registry(files, generator=generator)
    worktrees = [item for item in registry.get("worktrees", []) if item.get("taskId") != task.get("taskId")]
    entry = {
        "taskId": task.get("taskId"),
        "threadKey": task.get("threadKey"),
        "planEpoch": task.get("planEpoch"),
        "branchName": task.get("branchName"),
        "worktreePath": task.get("worktreePath"),
        "baseRef": task.get("baseRef") or (task.get("dispatch") or {}).get("baseRef"),
        "diffBase": task.get("diffBase") or (task.get("dispatch") or {}).get("diffBase"),
        "integrationBranch": integration_branch_for_task(task, read_task_pool(files["harness"])),
        "mergeRequired": task_merge_required(task),
        "status": status,
        "cleanupStatus": cleanup_status or task.get("cleanupStatus") or DEFAULT_POLICY_SUMMARY["merge"]["defaultCleanupStatus"],
        "preparedAt": task.get("worktreePreparedAt"),
        "updatedAt": now_iso(),
    }
    if extra:
        entry.update(extra)
    worktrees.append(entry)
    registry["worktrees"] = bounded(worktrees, DEFAULT_POLICY_SUMMARY["worktree"]["registryWindow"])
    registry["generatedAt"] = now_iso()
    registry["generator"] = generator
    write_json(files["worktree_registry_path"], registry)
    return entry


def merge_queue_entry(task: dict, request_id: str | None, *, merge_status: str, integration_branch: str | None, extra: dict | None = None) -> dict:
    entry = {
        "taskId": task.get("taskId"),
        "requestId": request_id,
        "threadKey": task.get("threadKey"),
        "planEpoch": task.get("planEpoch"),
        "branchName": task.get("branchName"),
        "worktreePath": task.get("worktreePath"),
        "baseRef": task.get("baseRef") or (task.get("dispatch") or {}).get("baseRef"),
        "integrationBranch": integration_branch,
        "mergeRequired": task_merge_required(task),
        "mergeStatus": merge_status,
        "mergeCheckedAt": None,
        "conflictPaths": [],
        "supersededByEpoch": None,
        "lastAuditVerdict": task.get("auditVerdict"),
        "mergedCommit": None,
        "cleanupStatus": task.get("cleanupStatus") or DEFAULT_POLICY_SUMMARY["merge"]["defaultCleanupStatus"],
        "queuedAt": now_iso(),
        "updatedAt": now_iso(),
    }
    if extra:
        entry.update(extra)
    return entry


def upsert_merge_queue_entry(root: Path, task: dict, request_id: str | None, *, generator: str, merge_status: str, extra: dict | None = None) -> dict:
    files = ensure_runtime_scaffold(root, generator=generator)
    queue = load_merge_queue(files, generator=generator)
    task_pool = read_task_pool(files["harness"]) or {}
    integration_branch = integration_branch_for_task(task, task_pool)
    items = [item for item in queue.get("items", []) if item.get("taskId") != task.get("taskId")]
    prior = next((item for item in queue.get("items", []) if item.get("taskId") == task.get("taskId")), None)
    entry = merge_queue_entry(task, request_id, merge_status=merge_status, integration_branch=integration_branch, extra=extra)
    if prior:
        entry["queuedAt"] = prior.get("queuedAt") or entry["queuedAt"]
    items.append(entry)
    queue["items"] = bounded(items, DEFAULT_POLICY_SUMMARY["merge"]["queueWindow"])
    queue["integrationBranch"] = integration_branch
    queue["generatedAt"] = now_iso()
    queue["generator"] = generator
    write_json(files["merge_queue_path"], queue)
    return entry


def build_merge_summary(merge_queue: dict, worktree_registry: dict, *, generator: str) -> dict:
    items = merge_queue.get("items", [])
    return {
        "schemaVersion": SCHEMA_VERSION,
        "generator": generator,
        "generatedAt": now_iso(),
        "integrationBranch": merge_queue.get("integrationBranch"),
        "queueDepth": len(items),
        "readyToMergeCount": sum(1 for item in items if item.get("mergeStatus") in {"merge_queued", "merge_checked"}),
        "conflictCount": sum(1 for item in items if item.get("mergeStatus") == "merge_conflict"),
        "mergedCount": sum(1 for item in items if item.get("mergeStatus") == "merged"),
        "openConflicts": bounded(
            [
                {
                    "taskId": item.get("taskId"),
                    "requestId": item.get("requestId"),
                    "branchName": item.get("branchName"),
                    "integrationBranch": item.get("integrationBranch"),
                    "conflictPaths": item.get("conflictPaths", []),
                }
                for item in items
                if item.get("mergeStatus") == "merge_conflict"
            ],
            DEFAULT_POLICY_SUMMARY["merge"]["summaryWindow"],
        ),
        "readyToMerge": bounded(
            [
                {
                    "taskId": item.get("taskId"),
                    "requestId": item.get("requestId"),
                    "branchName": item.get("branchName"),
                    "integrationBranch": item.get("integrationBranch"),
                    "planEpoch": item.get("planEpoch"),
                }
                for item in items
                if item.get("mergeStatus") in {"merge_queued", "merge_checked"}
            ],
            DEFAULT_POLICY_SUMMARY["merge"]["summaryWindow"],
        ),
        "recentMerged": bounded(
            [
                {
                    "taskId": item.get("taskId"),
                    "branchName": item.get("branchName"),
                    "integrationBranch": item.get("integrationBranch"),
                    "mergedCommit": item.get("mergedCommit"),
                    "updatedAt": item.get("updatedAt"),
                }
                for item in items
                if item.get("mergeStatus") == "merged"
            ],
            DEFAULT_POLICY_SUMMARY["merge"]["summaryWindow"],
        ),
        "supersededCandidates": bounded(
            [
                {
                    "taskId": item.get("taskId"),
                    "branchName": item.get("branchName"),
                    "planEpoch": item.get("planEpoch"),
                    "supersededByEpoch": item.get("supersededByEpoch"),
                }
                for item in items
                if item.get("supersededByEpoch")
            ],
            DEFAULT_POLICY_SUMMARY["merge"]["summaryWindow"],
        ),
        "activeWorktrees": bounded(worktree_registry.get("worktrees", []), DEFAULT_POLICY_SUMMARY["worktree"]["registryWindow"]),
    }


def merge_conflict_paths(root: Path, integration_branch: str, branch_name: str) -> list[str]:
    merge_base = run_command(["git", "merge-base", integration_branch, branch_name], cwd=root).stdout.strip()
    if not merge_base:
        return []
    branch_paths = {
        line.strip()
        for line in run_command(["git", "diff", "--name-only", f"{merge_base}...{branch_name}"], cwd=root).stdout.splitlines()
        if line.strip()
    }
    integration_paths = {
        line.strip()
        for line in run_command(["git", "diff", "--name-only", f"{merge_base}...{integration_branch}"], cwd=root).stdout.splitlines()
        if line.strip()
    }
    return sorted(branch_paths & integration_paths)


def merge_preflight(task: dict, *, latest_plan_epoch: int | None) -> list[str]:
    failures = []
    if task.get("status") not in {"verified", "merge_queued", "merge_checked", "merged"}:
        failures.append("task not verified")
    if task.get("supersededByRequestId"):
        failures.append("task superseded")
    if latest_plan_epoch is not None and task.get("planEpoch") and int(task.get("planEpoch") or 0) < int(latest_plan_epoch or 0):
        failures.append("task not on latest valid plan epoch")
    if not task.get("diffSummary"):
        failures.append("diff summary missing")
    if not task.get("branchName") or not task.get("worktreePath"):
        failures.append("branch/worktree binding incomplete")
    if not (task.get("baseRef") or (task.get("dispatch") or {}).get("baseRef")):
        failures.append("base ref missing")
    return failures


def merge_follow_up_kind(task: dict, conflict_paths: list[str]) -> str:
    if task.get("supersededByRequestId"):
        return "stop"
    if len(conflict_paths) > 6:
        return "replan"
    return "audit"


def emit_merge_conflict_follow_up(root: Path, task: dict, conflict_paths: list[str], *, generator: str) -> dict:
    kind = merge_follow_up_kind(task, conflict_paths)
    goal = f"Merge resolution requested for {task.get('taskId')}: {', '.join(conflict_paths[:5]) or 'conflict detected'}"
    return emit_follow_up_request(
        root,
        kind=kind,
        goal=goal,
        source="runtime:merge",
        generator=generator,
        origin_task_id=task.get("taskId"),
        reason="merge conflict detected during local integration preview",
        dedupe_key=f"{kind}:merge-conflict:{task.get('taskId')}:{stable_short_hash(*conflict_paths)}",
        thread_key=task.get("threadKey"),
        target_plan_epoch=task.get("planEpoch"),
    )


def process_merge_queue(root: Path, *, task_id: str | None = None, generator: str) -> dict:
    files = ensure_runtime_scaffold(root, generator=generator)
    task_pool_path = files["harness"] / "task-pool.json"
    task_pool = load_json(task_pool_path)
    request_summary = load_optional_json(files["request_summary_path"], {})
    thread_state = load_optional_json(files["thread_state_path"], {})
    merge_queue = load_merge_queue(files, generator=generator)
    queue_items = merge_queue.get("items", [])
    if task_id:
        queue_items = [item for item in queue_items if item.get("taskId") == task_id]
    results = []
    for item in queue_items:
        task = next((candidate for candidate in task_pool.get("tasks", []) if candidate.get("taskId") == item.get("taskId")), None)
        if task is None:
            continue
        thread_key = task.get("threadKey")
        latest_epoch = None
        if thread_key:
            latest_epoch = ((thread_state.get("threads") or {}).get(thread_key) or {}).get("currentPlanEpoch")
        failures = merge_preflight(task, latest_plan_epoch=latest_epoch)
        if failures:
            status = "merge_conflict" if "task not on latest valid plan epoch" in failures else "merge_resolution_requested"
            task["status"] = status
            task["mergeStatus"] = status
            task["mergeCheckedAt"] = now_iso()
            task["mergeFailureReasons"] = failures
            extra = {
                "mergeCheckedAt": task["mergeCheckedAt"],
                "conflictPaths": [],
                "supersededByEpoch": latest_epoch if latest_epoch and task.get("planEpoch") and int(task.get("planEpoch") or 0) < int(latest_epoch or 0) else None,
            }
            upsert_merge_queue_entry(root, task, item.get("requestId"), generator=generator, merge_status=status, extra=extra)
            upsert_worktree_registry_entry(
                root,
                task,
                generator=generator,
                status=status,
                extra={
                    "mergeStatus": status,
                    "mergeFailureReasons": failures[:5],
                    "supersededByEpoch": extra.get("supersededByEpoch"),
                },
            )
            for binding in request_bindings_for_task(load_json(files["request_task_map_path"]), task.get("taskId")):
                update_binding_state(
                    root,
                    binding.get("requestId"),
                    task.get("taskId"),
                    status,
                    reason="merge preflight blocked local integration",
                    generator=generator,
                    session_id=binding.get("lineage", {}).get("sessionId"),
                    worktree_path=task.get("worktreePath"),
                    diff_summary=task.get("diffSummary"),
                    outcome={"status": status, "failures": failures[:5]},
                )
            results.append({"taskId": task.get("taskId"), "mergeStatus": status, "failures": failures})
            continue
        integration_branch = integration_branch_for_task(task, task_pool)
        if not integration_branch or not ensure_integration_branch(root, integration_branch, task.get("baseRef") or (task.get("dispatch") or {}).get("baseRef")):
            status = "merge_resolution_requested"
            task["status"] = status
            task["mergeStatus"] = status
            failures = ["integration branch unavailable"]
            task["mergeFailureReasons"] = failures
            upsert_merge_queue_entry(root, task, item.get("requestId"), generator=generator, merge_status=status, extra={"mergeCheckedAt": now_iso()})
            upsert_worktree_registry_entry(
                root,
                task,
                generator=generator,
                status=status,
                extra={"mergeStatus": status, "mergeFailureReasons": failures},
            )
            results.append({"taskId": task.get("taskId"), "mergeStatus": status, "failures": failures})
            continue
        conflict_paths = merge_conflict_paths(root, integration_branch, task.get("branchName"))
        if conflict_paths:
            task["status"] = "merge_conflict"
            task["mergeStatus"] = "merge_conflict"
            task["mergeCheckedAt"] = now_iso()
            task["conflictPaths"] = conflict_paths
            follow_up = emit_merge_conflict_follow_up(root, task, conflict_paths, generator=generator)
            upsert_merge_queue_entry(
                root,
                task,
                item.get("requestId"),
                generator=generator,
                merge_status="merge_conflict",
                extra={
                    "mergeCheckedAt": task["mergeCheckedAt"],
                    "conflictPaths": conflict_paths,
                    "followUpRequestId": follow_up.get("requestId"),
                    "impactClassification": DEFAULT_POLICY_SUMMARY["merge"]["conflictImpactStatus"],
                },
            )
            upsert_worktree_registry_entry(
                root,
                task,
                generator=generator,
                status="merge_conflict",
                extra={
                    "mergeStatus": "merge_conflict",
                    "conflictPaths": conflict_paths,
                    "impactClassification": DEFAULT_POLICY_SUMMARY["merge"]["conflictImpactStatus"],
                },
            )
            for binding in request_bindings_for_task(load_json(files["request_task_map_path"]), task.get("taskId")):
                update_binding_state(
                    root,
                    binding.get("requestId"),
                    task.get("taskId"),
                    "merge_conflict",
                    reason="merge preview detected local conflict",
                    generator=generator,
                    session_id=binding.get("lineage", {}).get("sessionId"),
                    worktree_path=task.get("worktreePath"),
                    diff_summary=task.get("diffSummary"),
                    outcome={
                        "status": "merge_conflict",
                        "impactClassification": DEFAULT_POLICY_SUMMARY["merge"]["conflictImpactStatus"],
                        "conflictPaths": conflict_paths,
                        "followUpRequestId": follow_up.get("requestId"),
                    },
                )
            lineage_event(
                root,
                "task.merge_conflict",
                generator,
                request_id=item.get("requestId"),
                task_id=task.get("taskId"),
                worktree_path=task.get("worktreePath"),
                diff_summary=task.get("diffSummary"),
                detail="merge conflict detected",
                context={"conflictPaths": conflict_paths, "integrationBranch": integration_branch, "branchName": task.get("branchName")},
            )
            results.append({"taskId": task.get("taskId"), "mergeStatus": "merge_conflict", "conflictPaths": conflict_paths})
            continue
        task["status"] = "merge_checked"
        task["mergeStatus"] = "merge_checked"
        task["mergeCheckedAt"] = now_iso()
        upsert_merge_queue_entry(
            root,
            task,
            item.get("requestId"),
            generator=generator,
            merge_status="merge_checked",
            extra={
                "mergeCheckedAt": task["mergeCheckedAt"],
                "impactClassification": DEFAULT_POLICY_SUMMARY["merge"]["cleanImpactStatus"],
            },
        )
        upsert_worktree_registry_entry(
            root,
            task,
            generator=generator,
            status="merge_checked",
            extra={
                "mergeStatus": "merge_checked",
                "integrationBranch": integration_branch,
                "impactClassification": DEFAULT_POLICY_SUMMARY["merge"]["cleanImpactStatus"],
            },
        )
        for binding in request_bindings_for_task(load_json(files["request_task_map_path"]), task.get("taskId")):
            update_binding_state(
                root,
                binding.get("requestId"),
                task.get("taskId"),
                "merge_checked",
                reason="local merge preview is clean",
                generator=generator,
                session_id=binding.get("lineage", {}).get("sessionId"),
                worktree_path=task.get("worktreePath"),
                diff_summary=task.get("diffSummary"),
                outcome={
                    "status": "merge_checked",
                    "impactClassification": DEFAULT_POLICY_SUMMARY["merge"]["cleanImpactStatus"],
                    "integrationBranch": integration_branch,
                },
            )
        lineage_event(
            root,
            "task.merge_checked",
            generator,
            request_id=item.get("requestId"),
            task_id=task.get("taskId"),
            worktree_path=task.get("worktreePath"),
            diff_summary=task.get("diffSummary"),
            detail="local merge preview clean",
            context={"integrationBranch": integration_branch, "branchName": task.get("branchName")},
        )
        integration_worktree = ensure_integration_worktree(root, integration_branch, task.get("baseRef") or (task.get("dispatch") or {}).get("baseRef"))
        if integration_worktree is None:
            task["status"] = "merge_resolution_requested"
            task["mergeStatus"] = "merge_resolution_requested"
            upsert_merge_queue_entry(root, task, item.get("requestId"), generator=generator, merge_status="merge_resolution_requested", extra={"mergeCheckedAt": now_iso()})
            results.append({"taskId": task.get("taskId"), "mergeStatus": "merge_resolution_requested", "failures": ["integration worktree unavailable"]})
            continue
        try:
            run_command(["git", "merge", "--no-ff", task.get("branchName"), "-m", f"Merge {task.get('taskId')} via klein-harness"], cwd=integration_worktree)
            merged_commit = run_command(["git", "rev-parse", "HEAD"], cwd=integration_worktree).stdout.strip()
            task["status"] = "completed"
            task["mergeStatus"] = "merged"
            task["mergedAt"] = now_iso()
            task["mergedCommit"] = merged_commit
            task["cleanupStatus"] = DEFAULT_POLICY_SUMMARY["merge"]["mergedCleanupStatus"]
            upsert_worktree_registry_entry(
                root,
                task,
                generator=generator,
                status="merged",
                cleanup_status=DEFAULT_POLICY_SUMMARY["merge"]["mergedCleanupStatus"],
                extra={"mergedCommit": merged_commit},
            )
            upsert_merge_queue_entry(
                root,
                task,
                item.get("requestId"),
                generator=generator,
                merge_status="merged",
                extra={
                    "mergeCheckedAt": now_iso(),
                    "mergedCommit": merged_commit,
                    "impactClassification": DEFAULT_POLICY_SUMMARY["merge"]["cleanImpactStatus"],
                    "cleanupStatus": DEFAULT_POLICY_SUMMARY["merge"]["mergedCleanupStatus"],
                },
            )
            lineage_event(
                root,
                "task.merged",
                generator,
                request_id=item.get("requestId"),
                task_id=task.get("taskId"),
                worktree_path=task.get("worktreePath"),
                diff_summary=task.get("diffSummary"),
                outcome={"status": "merged", "mergedCommit": merged_commit},
                context={"integrationBranch": integration_branch, "branchName": task.get("branchName")},
            )
            for binding in request_bindings_for_task(load_json(files["request_task_map_path"]), task.get("taskId")):
                update_binding_state(
                    root,
                    binding.get("requestId"),
                    task.get("taskId"),
                    "merged",
                    reason="local integration merge succeeded",
                    generator=generator,
                    session_id=binding.get("lineage", {}).get("sessionId"),
                    worktree_path=task.get("worktreePath"),
                    diff_summary=task.get("diffSummary"),
                    verification={
                        "overallStatus": task.get("verificationStatus"),
                        "summary": task.get("verificationSummary"),
                        "verificationResultPath": task.get("verificationResultPath"),
                    },
                    outcome={"status": "merged", "mergedCommit": merged_commit},
                )
                maybe_complete_request(root, binding.get("requestId"), generator=generator)
            results.append({"taskId": task.get("taskId"), "mergeStatus": "merged", "mergedCommit": merged_commit})
        except subprocess.CalledProcessError:
            conflict_paths = [
                line.strip()
                for line in run_command(["git", "diff", "--name-only", "--diff-filter=U"], cwd=integration_worktree, check=False).stdout.splitlines()
                if line.strip()
            ]
            run_command(["git", "merge", "--abort"], cwd=integration_worktree, check=False)
            task["status"] = "merge_conflict"
            task["mergeStatus"] = "merge_conflict"
            task["conflictPaths"] = conflict_paths
            task["mergeCheckedAt"] = now_iso()
            follow_up = emit_merge_conflict_follow_up(root, task, conflict_paths, generator=generator)
            upsert_merge_queue_entry(
                root,
                task,
                item.get("requestId"),
                generator=generator,
                merge_status="merge_conflict",
                extra={
                    "mergeCheckedAt": task["mergeCheckedAt"],
                    "conflictPaths": conflict_paths,
                    "followUpRequestId": follow_up.get("requestId"),
                    "impactClassification": DEFAULT_POLICY_SUMMARY["merge"]["conflictImpactStatus"],
                },
            )
            upsert_worktree_registry_entry(
                root,
                task,
                generator=generator,
                status="merge_conflict",
                extra={
                    "mergeStatus": "merge_conflict",
                    "conflictPaths": conflict_paths,
                    "impactClassification": DEFAULT_POLICY_SUMMARY["merge"]["conflictImpactStatus"],
                },
            )
            for binding in request_bindings_for_task(load_json(files["request_task_map_path"]), task.get("taskId")):
                update_binding_state(
                    root,
                    binding.get("requestId"),
                    task.get("taskId"),
                    "merge_conflict",
                    reason="local integration merge failed with conflicts",
                    generator=generator,
                    session_id=binding.get("lineage", {}).get("sessionId"),
                    worktree_path=task.get("worktreePath"),
                    diff_summary=task.get("diffSummary"),
                    outcome={
                        "status": "merge_conflict",
                        "conflictPaths": conflict_paths,
                        "followUpRequestId": follow_up.get("requestId"),
                    },
                )
            lineage_event(
                root,
                "task.merge_conflict",
                generator,
                request_id=item.get("requestId"),
                task_id=task.get("taskId"),
                worktree_path=task.get("worktreePath"),
                diff_summary=task.get("diffSummary"),
                detail="local merge failed with conflicts",
                context={"conflictPaths": conflict_paths, "integrationBranch": integration_branch, "branchName": task.get("branchName")},
            )
            results.append({"taskId": task.get("taskId"), "mergeStatus": "merge_conflict", "conflictPaths": conflict_paths})
    write_json(task_pool_path, task_pool)
    worktree_registry = load_worktree_registry(files, generator=generator)
    merge_summary = build_merge_summary(load_merge_queue(files, generator=generator), worktree_registry, generator=generator)
    write_json(files["merge_summary_path"], merge_summary)
    return {
        "schemaVersion": SCHEMA_VERSION,
        "generator": generator,
        "generatedAt": now_iso(),
        "results": results,
    }


def request_goal_tokens(text: str):
    return {
        token
        for token in re.findall(r"[a-zA-Z0-9_-]+", (text or "").lower())
        if len(token) >= 3
    }


def canonicalize_goal_text(text: str) -> str:
    normalized = re.sub(r"\s+", " ", (text or "").strip().lower())
    normalized = re.sub(r"[，。、“”‘’；;：:,.!?！？（）()\[\]{}]+", " ", normalized)
    normalized = re.sub(r"\s+", " ", normalized).strip()
    return normalized


def stable_short_hash(*parts: str) -> str:
    payload = "\n".join(part or "" for part in parts)
    return hashlib.sha256(payload.encode("utf-8")).hexdigest()[:16]


def explicit_ids_from_text(text: str, pattern: str) -> list[str]:
    if not text:
        return []
    return list(dict.fromkeys(re.findall(pattern, text)))


def evidence_fingerprint(goal: str, context_paths: list[str]) -> str:
    normalized_paths = ",".join(sorted(context_paths or []))
    return stable_short_hash(canonicalize_goal_text(goal), normalized_paths)


def idempotency_key_for_submission(thread_key: str | None, explicit_key: str | None, canonical_goal_hash: str, evidence_fp: str) -> str:
    if explicit_key:
        return explicit_key
    return stable_short_hash(thread_key or "thread:unbound", canonical_goal_hash, evidence_fp)


def token_overlap_ratio(left: set[str], right: set[str]) -> float:
    if not left or not right:
        return 0.0
    union = left | right
    if not union:
        return 0.0
    return len(left & right) / len(union)


def thread_key_from_request(request: dict) -> str | None:
    return request.get("threadKey") or request.get("targetThreadKey")


def request_effect_terminal(request: dict) -> bool:
    if request.get("status") in REQUEST_TERMINAL_STATUSES:
        return True
    return request.get("fusionDecision") in {"duplicate_of_existing", "merged_as_context", "noop"} and request.get("effectStatus") == "closed"


def detect_compound_segments(goal: str) -> list[str]:
    text = (goal or "").strip()
    if not text:
        return []
    separators = [
        r"\s+and also\s+",
        r"\s+also\s+",
        r"\s*(?:,|，|、)?\s*同时\s*",
        r"\s*(?:,|，|、)?\s*并且\s*",
        r"\s*(?:,|，|、)?\s*另外再\s*",
        r"\s*(?:,|，|、)?\s*另外\s*",
        r"\s*(?:,|，|、)?\s*然后\s*",
    ]
    for separator in separators:
        parts = [part.strip(" ，。;；") for part in re.split(separator, text, flags=re.IGNORECASE) if part.strip(" ，。;；")]
        if len(parts) > 1:
            return parts[:4]
    return [text]


def classify_goal_clause(goal: str, *, has_context: bool) -> str:
    text = canonicalize_goal_text(goal)
    inspection_keywords = {
        "check", "inspect", "review", "audit", "verify", "triage", "analyze", "analyse",
        "compare", "explain", "reproduce", "诊断", "检查", "排查", "分析", "复核", "审计", "验证", "解释",
    }
    context_keywords = {
        "context", "evidence", "clarify", "constraint", "logs", "extra info", "trace", "补充", "额外信息", "上下文",
        "约束", "线索", "日志", "证据", "说明补充",
    }
    change_keywords = {
        "add", "implement", "fix", "change", "update", "modify", "append", "support", "refactor",
        "新增", "实现", "修复", "修改", "调整", "补充", "支持", "重构", "继续做", "完善",
    }
    tokens = set(text.split())
    has_inspection = bool(tokens & inspection_keywords) or any(keyword in text for keyword in inspection_keywords)
    has_context_only = has_context and not has_inspection and not any(keyword in text for keyword in change_keywords)
    has_change = bool(tokens & change_keywords) or any(keyword in text for keyword in change_keywords)
    if has_inspection and re.search(r"(当前|现有)\s*实现|current implementation|existing implementation", text):
        has_change = False
    if has_context_only:
        return "context_enrichment"
    if has_inspection and has_change:
        return "compound_split"
    if has_inspection:
        return "inspection"
    if has_change:
        return "append_change"
    if has_context:
        return "context_enrichment"
    return "fresh_work"


def classify_front_door_semantics(request: dict, normalized_intent: str) -> str:
    text = (request.get("goal") or "").strip().lower()
    kind_hint = (request.get("kind") or "").lower()
    help_keywords = {"help", "usage", "how", "what", "why", "说明", "怎么", "如何", "是什么", "为什么"}
    advisory_keywords = {"compare", "difference", "overview", "summarize", "summary", "建议", "比较", "概览", "总结"}
    if normalized_intent in {"duplicate_or_noop", "context_enrichment"}:
        return "duplicate_or_context"
    if normalized_intent == "inspection":
        if any(keyword in text for keyword in help_keywords | advisory_keywords) or kind_hint in {"status", "analysis", "research"}:
            return "advisory_read_only"
        return "inspection"
    if normalized_intent == "ambiguous_needs_orchestrator" and any(keyword in text for keyword in help_keywords):
        return "conversational_help"
    return "work_order"


def existing_open_requests(index: dict) -> list[dict]:
    return [request for request in index.get("requests", []) if not request_effect_terminal(request)]


def correlate_existing_thread(request: dict, existing: list[dict], task_map: dict) -> tuple[str | None, dict | None, str | None]:
    explicit_thread_key = request.get("threadKey")
    if explicit_thread_key:
        match = next((item for item in reversed(existing) if thread_key_from_request(item) == explicit_thread_key), None)
        return explicit_thread_key, match, "explicit thread key"

    goal = request.get("goal") or ""
    explicit_request_ids = explicit_ids_from_text(goal, r"\bR-\d+\b")
    for request_id in explicit_request_ids:
        match = next((item for item in existing if item.get("requestId") == request_id), None)
        if match:
            return thread_key_from_request(match) or f"thread:{match['requestId']}", match, f"goal references {request_id}"

    explicit_task_ids = explicit_ids_from_text(goal, r"\bT-\d+\b")
    if explicit_task_ids:
        task_id = explicit_task_ids[0]
        match_binding = next((binding for binding in reversed(task_map.get("bindings", [])) if binding.get("taskId") == task_id), None)
        if match_binding:
            match = next((item for item in existing if item.get("requestId") == match_binding.get("requestId")), None)
            if match:
                return thread_key_from_request(match) or f"thread:{match['requestId']}", match, f"goal references {task_id}"

    incoming_tokens = request_goal_tokens(goal)
    best = None
    best_ratio = 0.0
    for candidate in existing[-20:]:
        candidate_tokens = request_goal_tokens(candidate.get("goal"))
        ratio = token_overlap_ratio(incoming_tokens, candidate_tokens)
        if ratio > best_ratio:
            best_ratio = ratio
            best = candidate
    if best and best_ratio >= DEFAULT_POLICY_SUMMARY["intake"]["tokenOverlapThreshold"]:
        return thread_key_from_request(best) or f"thread:{best['requestId']}", best, f"token overlap {best_ratio:.2f}"
    return None, None, None


def open_requests_for_thread(index: dict, thread_key: str | None) -> list[dict]:
    if not thread_key:
        return []
    return [
        request for request in index.get("requests", [])
        if thread_key_from_request(request) == thread_key and not request_effect_terminal(request)
    ]


def recent_requests_for_thread(index: dict, thread_key: str | None, *, limit: int | None = None) -> list[dict]:
    if not thread_key:
        return []
    requests = [
        request for request in index.get("requests", [])
        if thread_key_from_request(request) == thread_key
    ]
    if limit is None:
        limit = DEFAULT_POLICY_SUMMARY["intake"]["duplicateCanonicalGoalWindow"]
    return requests[-limit:]


def current_plan_epoch_for_thread(thread_state: dict | None, thread_key: str | None) -> int:
    if not thread_state or not thread_key:
        return DEFAULT_POLICY_SUMMARY["threading"]["defaultInitialPlanEpoch"]
    thread = (thread_state.get("threads") or {}).get(thread_key)
    if not thread:
        return DEFAULT_POLICY_SUMMARY["threading"]["defaultInitialPlanEpoch"]
    return int(thread.get("currentPlanEpoch") or DEFAULT_POLICY_SUMMARY["threading"]["defaultInitialPlanEpoch"])


def classify_submission(request: dict, *, index: dict, task_map: dict, thread_state: dict | None = None) -> dict:
    kind_hint = (request.get("kind") or "").lower()
    runtime_internal_request = (request.get("source") or "").startswith("runtime:") and kind_hint in {
        "replan",
        "audit",
        "analysis",
        "status",
        "stop",
        "research",
        "implementation",
    }
    canonical_goal_hash = stable_short_hash(canonicalize_goal_text(request.get("goal")))
    evidence_fp = evidence_fingerprint(request.get("goal") or "", request.get("contextPaths") or [])
    recent_requests = [
        item
        for item in bounded(
            index.get("requests", []),
            DEFAULT_POLICY_SUMMARY["intake"]["duplicateCanonicalGoalWindow"],
        )
        if item.get("requestId") != request.get("requestId")
        and item.get("submissionId") != request.get("submissionId")
    ]
    thread_key, target_request, correlation_reason = correlate_existing_thread(request, recent_requests, task_map)
    if thread_key is None:
        thread_key = f"thread:{canonical_goal_hash[:12]}"
    derived_idempotency_key = idempotency_key_for_submission(
        thread_key,
        request.get("idempotencyKey"),
        canonical_goal_hash,
        evidence_fp,
    )
    clause_segments = detect_compound_segments(request.get("goal") or "")
    clause_kinds = [classify_goal_clause(segment, has_context=bool(request.get("contextPaths"))) for segment in clause_segments]
    has_multiple_kinds = len(set(clause_kinds)) > 1
    initial_intent = clause_kinds[0] if clause_kinds else "fresh_work"
    if has_multiple_kinds:
        normalized_intent = "compound_split"
    elif runtime_internal_request and kind_hint in {"audit", "analysis", "status"}:
        normalized_intent = "fresh_work"
    elif runtime_internal_request:
        normalized_intent = "fresh_work"
    elif target_request and initial_intent == "fresh_work":
        normalized_intent = "append_change"
    else:
        normalized_intent = initial_intent

    existing_thread_requests = recent_requests_for_thread(index, thread_key)
    duplicate_of = next(
        (
            item for item in existing_thread_requests
            if item.get("requestId") != request.get("requestId")
            and item.get("submissionId") != request.get("submissionId")
            if item.get("idempotencyKey") == derived_idempotency_key
            or (
                item.get("canonicalGoalHash") == canonical_goal_hash
                and item.get("evidenceFingerprint") == evidence_fp
            )
        ),
        None,
    )
    if (
        not duplicate_of
        and target_request
        and target_request.get("canonicalGoalHash") == canonical_goal_hash
        and evidence_fp != target_request.get("evidenceFingerprint")
        and bool(request.get("contextPaths"))
    ):
        normalized_intent = "context_enrichment"

    if duplicate_of:
        fusion_decision = "duplicate_of_existing"
        normalized_intent = "duplicate_or_noop"
        effect_status = "closed"
        target_plan_epoch = duplicate_of.get("targetPlanEpoch") or duplicate_of.get("planEpoch") or current_plan_epoch_for_thread(thread_state, thread_key)
        duplicate_of_request_id = duplicate_of.get("requestId")
        merged_into_request_id = None
    elif normalized_intent == "context_enrichment" and target_request:
        fusion_decision = "merged_as_context"
        effect_status = "closed"
        target_plan_epoch = current_plan_epoch_for_thread(thread_state, thread_key)
        duplicate_of_request_id = None
        merged_into_request_id = target_request.get("requestId")
    elif normalized_intent == "inspection":
        fusion_decision = "inspection_overlay"
        effect_status = "open"
        target_plan_epoch = current_plan_epoch_for_thread(thread_state, thread_key)
        duplicate_of_request_id = None
        merged_into_request_id = target_request.get("requestId") if target_request else None
    elif normalized_intent == "compound_split":
        fusion_decision = "compound_split_created"
        effect_status = "open"
        target_plan_epoch = current_plan_epoch_for_thread(thread_state, thread_key) + (1 if target_request else 0)
        duplicate_of_request_id = None
        merged_into_request_id = target_request.get("requestId") if target_request else None
    elif runtime_internal_request and target_request:
        fusion_decision = "accepted_existing_thread"
        effect_status = "open"
        target_plan_epoch = current_plan_epoch_for_thread(thread_state, thread_key)
        duplicate_of_request_id = None
        merged_into_request_id = target_request.get("requestId")
    elif normalized_intent == "append_change" and target_request:
        fusion_decision = "append_requires_replan"
        effect_status = "open"
        target_plan_epoch = current_plan_epoch_for_thread(thread_state, thread_key) + 1
        duplicate_of_request_id = None
        merged_into_request_id = target_request.get("requestId")
    elif target_request:
        fusion_decision = "accepted_existing_thread"
        effect_status = "open"
        target_plan_epoch = current_plan_epoch_for_thread(thread_state, thread_key)
        duplicate_of_request_id = None
        merged_into_request_id = target_request.get("requestId")
    else:
        fusion_decision = "accepted_new_thread"
        effect_status = "open"
        target_plan_epoch = current_plan_epoch_for_thread(thread_state, thread_key)
        duplicate_of_request_id = None
        merged_into_request_id = None

    if normalized_intent not in INTENT_CLASSES:
        normalized_intent = "ambiguous_needs_orchestrator"
    if fusion_decision not in FUSION_DECISIONS:
        fusion_decision = "accepted_new_thread"

    compound_group_id = stable_short_hash(thread_key, canonical_goal_hash, "compound") if normalized_intent == "compound_split" else None
    if normalized_intent == "ambiguous_needs_orchestrator":
        classification_reason = "deterministic rules could not safely classify request"
    else:
        reason_parts = [normalized_intent.replace("_", " "), fusion_decision.replace("_", " ")]
        if correlation_reason:
            reason_parts.append(correlation_reason)
        classification_reason = "; ".join(reason_parts)
    front_door_class = classify_front_door_semantics(request, normalized_intent)
    if front_door_class not in FRONT_DOOR_CLASSES:
        front_door_class = "work_order"
    return {
        "normalizedIntentClass": normalized_intent,
        "frontDoorClass": front_door_class,
        "fusionDecision": fusion_decision,
        "threadKey": request.get("threadKey") or thread_key,
        "targetThreadKey": thread_key,
        "canonicalGoalHash": canonical_goal_hash,
        "evidenceFingerprint": evidence_fp,
        "idempotencyKey": derived_idempotency_key,
        "duplicateOfRequestId": duplicate_of_request_id,
        "mergedIntoRequestId": merged_into_request_id,
        "compoundGroupId": compound_group_id,
        "targetPlanEpoch": target_plan_epoch,
        "classificationReason": classification_reason,
        "internalIntents": list(dict.fromkeys(clause_kinds)),
        "effectStatus": effect_status,
        "effectiveKind": (
            "audit" if normalized_intent == "inspection"
            else "analysis" if normalized_intent == "context_enrichment"
            else (request.get("kind") or "implementation")
        ),
    }


def resolve_task_thread_key(task: dict, request_index: dict | None = None, task_map: dict | None = None) -> str | None:
    if task.get("threadKey"):
        return task.get("threadKey")
    if not request_index or not task_map:
        return None
    requests_by_id = {
        request.get("requestId"): request
        for request in (request_index or {}).get("requests", [])
        if request.get("requestId")
    }
    for binding in reversed((task_map or {}).get("bindings", [])):
        if binding.get("taskId") != task.get("taskId"):
            continue
        request = requests_by_id.get(binding.get("requestId"))
        if request and thread_key_from_request(request):
            return thread_key_from_request(request)
    return None


def resolve_task_plan_epoch(task: dict, request_index: dict | None = None, task_map: dict | None = None) -> int:
    if task.get("planEpoch") is not None:
        return int(task.get("planEpoch") or DEFAULT_POLICY_SUMMARY["threading"]["defaultInitialPlanEpoch"])
    if not request_index or not task_map:
        return DEFAULT_POLICY_SUMMARY["threading"]["defaultInitialPlanEpoch"]
    requests_by_id = {
        request.get("requestId"): request
        for request in (request_index or {}).get("requests", [])
        if request.get("requestId")
    }
    for binding in reversed((task_map or {}).get("bindings", [])):
        if binding.get("taskId") != task.get("taskId"):
            continue
        request = requests_by_id.get(binding.get("requestId"))
        if request and request.get("targetPlanEpoch") is not None:
            return int(request.get("targetPlanEpoch") or DEFAULT_POLICY_SUMMARY["threading"]["defaultInitialPlanEpoch"])
    return DEFAULT_POLICY_SUMMARY["threading"]["defaultInitialPlanEpoch"]


def active_task_ids_for_thread(tasks: list[dict], thread_key: str, request_index: dict, task_map: dict) -> list[str]:
    result = []
    for task in tasks:
        if task.get("status") not in TASK_ACTIVE_STATUSES:
            continue
        if resolve_task_thread_key(task, request_index, task_map) == thread_key:
            result.append(task.get("taskId"))
    return [item for item in result if item]


def queued_task_ids_for_thread(tasks: list[dict], thread_key: str, request_index: dict, task_map: dict) -> list[str]:
    result = []
    for task in tasks:
        if task.get("status") != "queued":
            continue
        if resolve_task_thread_key(task, request_index, task_map) == thread_key:
            result.append(task.get("taskId"))
    return [item for item in result if item]


def analysis_follow_up_kind(request: dict) -> str:
    normalized_intent = request.get("normalizedIntentClass")
    if normalized_intent == "inspection":
        return "audit"
    if normalized_intent == "compound_split":
        return "audit"
    if normalized_intent == "ambiguous_needs_orchestrator":
        return "analysis"
    if normalized_intent == "append_change":
        return "replan"
    return request.get("effectiveKind") or request.get("kind") or "implementation"


def analyze_inflight_impact(
    request: dict,
    *,
    task_pool: dict | None,
    request_index: dict,
    task_map: dict,
) -> dict:
    tasks = (task_pool or {}).get("tasks", [])
    thread_key = thread_key_from_request(request)
    impacted = []
    unaffected_active = []
    superseded = []
    if thread_key:
        for task in tasks:
            task_thread = resolve_task_thread_key(task, request_index, task_map)
            if task_thread != thread_key:
                if task.get("status") in TASK_ACTIVE_STATUSES:
                    unaffected_active.append(task.get("taskId"))
                continue
            task_epoch = resolve_task_plan_epoch(task, request_index, task_map)
            target_epoch = int(request.get("targetPlanEpoch") or task_epoch or DEFAULT_POLICY_SUMMARY["threading"]["defaultInitialPlanEpoch"])
            impacted.append(task.get("taskId"))
            if task.get("status") == "queued" and task_epoch < target_epoch:
                superseded.append(task.get("taskId"))

    normalized_intent = request.get("normalizedIntentClass")
    if normalized_intent == "inspection":
        impact = "inspection_only_overlay"
    elif superseded:
        impact = "supersede_queued"
    elif normalized_intent in {"append_change", "compound_split"} and active_task_ids_for_thread(tasks, thread_key, request_index, task_map):
        impact = "checkpoint_then_replan"
    elif impacted:
        impact = "continue_with_note"
    else:
        impact = "continue_safe"

    return {
        "impactClassification": impact,
        "impactedTaskIds": [item for item in impacted if item],
        "unaffectedActiveTaskIds": [item for item in unaffected_active if item],
        "supersededQueuedTaskIds": [item for item in superseded if item],
    }


def apply_impact_to_task_pool(
    root: Path,
    request: dict,
    impact: dict,
    *,
    generator: str,
):
    files = ensure_runtime_scaffold(root, generator=generator)
    task_pool = read_task_pool(files["harness"])
    if not task_pool:
        return None
    tasks = task_pool.get("tasks", [])
    changed = False
    target_epoch = int(request.get("targetPlanEpoch") or DEFAULT_POLICY_SUMMARY["threading"]["defaultInitialPlanEpoch"])
    for task in tasks:
        task_id = task.get("taskId")
        if task_id in set(impact.get("supersededQueuedTaskIds", [])):
            task["status"] = "superseded"
            task["supersededByRequestId"] = request.get("requestId")
            task["supersededAt"] = now_iso()
            changed = True
        elif task_id in set(impact.get("impactedTaskIds", [])) and task.get("status") in TASK_ACTIVE_STATUSES:
            task["checkpointRequested"] = True
            task["checkpointReason"] = f"request {request.get('requestId')} requires epoch {target_epoch}"
            task["nextPlanEpoch"] = target_epoch
            changed = True
    if changed:
        write_json(files["harness"] / "task-pool.json", task_pool)
        lineage_event(
            root,
            "thread.impact_applied",
            generator,
            request_id=request.get("requestId"),
            detail=impact.get("impactClassification"),
            context=impact,
        )
    return task_pool


def build_thread_state(
    root: Path,
    request_index: dict,
    task_map: dict,
    task_pool: dict | None,
    session_registry: dict | None,
    feedback_summary: dict | None,
    log_index: dict | None,
    *,
    generator: str,
    policy_summary: dict,
):
    tasks = (task_pool or {}).get("tasks", [])
    threads: dict[str, dict] = {}
    recent_limit = policy_summary["threading"]["maxRecentThreadEvents"]
    hot_limit = policy_summary["thread"]["maxActiveTaskIds"] if "thread" in policy_summary else DEFAULT_POLICY_SUMMARY["hotState"]["maxActiveItems"]
    requests = sorted(request_index.get("requests", []), key=lambda item: item.get("seq", 0))
    task_ids_by_thread: dict[str, set[str]] = defaultdict(set)

    for request in requests:
        thread_key = thread_key_from_request(request)
        if not thread_key:
            continue
        thread = threads.setdefault(
            thread_key,
            {
                "threadKey": thread_key,
                "rootRequestId": request.get("requestId"),
                "latestRequestId": request.get("requestId"),
                "currentPlanEpoch": int(request.get("targetPlanEpoch") or request.get("planEpoch") or DEFAULT_POLICY_SUMMARY["threading"]["defaultInitialPlanEpoch"]),
                "latestValidPlanEpoch": int(request.get("targetPlanEpoch") or request.get("planEpoch") or DEFAULT_POLICY_SUMMARY["threading"]["defaultInitialPlanEpoch"]),
                "recentRequestIds": [],
                "requestCount": 0,
                "appendChangeCount": 0,
                "inspectionCount": 0,
                "contextEnrichmentCount": 0,
                "duplicateCount": 0,
                "compoundCount": 0,
                "openRequestCount": 0,
                "lastFusionDecision": None,
                "lastImpactClassification": None,
                "lastClassification": None,
                "latestAppendRequestId": None,
                "latestInspectionRequestId": None,
                "activeTaskIds": [],
                "queuedTaskIds": [],
                "recoverableTaskIds": [],
                "completedTaskIds": [],
                "supersededTaskIds": [],
                "activeSessionIds": [],
                "contextRotScore": 0,
                "contextRotStatus": "fresh",
                "contextRotReasons": [],
                "unresolvedFailureCount": 0,
                "lastUpdatedAt": request.get("updatedAt") or request.get("createdAt"),
            },
        )
        thread["latestRequestId"] = request.get("requestId")
        thread["requestCount"] += 1
        thread["currentPlanEpoch"] = max(thread["currentPlanEpoch"], int(request.get("targetPlanEpoch") or request.get("planEpoch") or 1))
        thread["latestValidPlanEpoch"] = thread["currentPlanEpoch"]
        thread["lastFusionDecision"] = request.get("fusionDecision") or thread.get("lastFusionDecision")
        thread["lastClassification"] = request.get("normalizedIntentClass") or thread.get("lastClassification")
        thread["lastImpactClassification"] = request.get("impactClassification") or thread.get("lastImpactClassification")
        thread["lastUpdatedAt"] = request.get("updatedAt") or request.get("createdAt") or thread.get("lastUpdatedAt")
        if request.get("status") not in REQUEST_TERMINAL_STATUSES:
            thread["openRequestCount"] += 1
        if request.get("normalizedIntentClass") == "append_change":
            thread["appendChangeCount"] += 1
            thread["latestAppendRequestId"] = request.get("requestId")
        elif request.get("normalizedIntentClass") == "inspection":
            thread["inspectionCount"] += 1
            thread["latestInspectionRequestId"] = request.get("requestId")
        elif request.get("normalizedIntentClass") == "context_enrichment":
            thread["contextEnrichmentCount"] += 1
        elif request.get("normalizedIntentClass") == "duplicate_or_noop":
            thread["duplicateCount"] += 1
        elif request.get("normalizedIntentClass") == "compound_split":
            thread["compoundCount"] += 1
        request_ids = thread.setdefault("recentRequestIds", [])
        if request.get("requestId"):
            request_ids.append(request.get("requestId"))
            thread["recentRequestIds"] = request_ids[-recent_limit:]

    requests_by_id = {
        request.get("requestId"): request
        for request in requests
        if request.get("requestId")
    }
    for task in tasks:
        thread_key = resolve_task_thread_key(task, request_index, task_map)
        if not thread_key:
            continue
        thread = threads.setdefault(
            thread_key,
            {
                "threadKey": thread_key,
                "rootRequestId": None,
                "latestRequestId": None,
                "currentPlanEpoch": resolve_task_plan_epoch(task, request_index, task_map),
                "latestValidPlanEpoch": resolve_task_plan_epoch(task, request_index, task_map),
                "recentRequestIds": [],
                "requestCount": 0,
                "appendChangeCount": 0,
                "inspectionCount": 0,
                "contextEnrichmentCount": 0,
                "duplicateCount": 0,
                "compoundCount": 0,
                "openRequestCount": 0,
                "lastFusionDecision": None,
                "lastImpactClassification": None,
                "lastClassification": None,
                "latestAppendRequestId": None,
                "latestInspectionRequestId": None,
                "activeTaskIds": [],
                "queuedTaskIds": [],
                "recoverableTaskIds": [],
                "completedTaskIds": [],
                "supersededTaskIds": [],
                "activeSessionIds": [],
                "contextRotScore": 0,
                "contextRotStatus": "fresh",
                "contextRotReasons": [],
                "unresolvedFailureCount": 0,
                "lastUpdatedAt": None,
            },
        )
        task_id = task.get("taskId")
        if task_id:
            task_ids_by_thread[thread_key].add(task_id)
        task_epoch = resolve_task_plan_epoch(task, request_index, task_map)
        thread["currentPlanEpoch"] = max(thread["currentPlanEpoch"], task_epoch)
        thread["latestValidPlanEpoch"] = thread["currentPlanEpoch"]
        status = task.get("status")
        if status in TASK_ACTIVE_STATUSES:
            thread["activeTaskIds"].append(task_id)
        elif status == "queued":
            thread["queuedTaskIds"].append(task_id)
        elif status == "recoverable":
            thread["recoverableTaskIds"].append(task_id)
        elif status in TASK_COMPLETED_STATUSES:
            thread["completedTaskIds"].append(task_id)
        elif status in TASK_SUPERSEDED_STATUSES or status == "superseded":
            thread["supersededTaskIds"].append(task_id)
        session_id = task.get("claim", {}).get("boundSessionId") or task.get("lastKnownSessionId")
        if session_id:
            thread["activeSessionIds"].append(session_id)

    feedback_by_task = (feedback_summary or {}).get("taskFeedbackSummary", {})
    logs_by_task = (log_index or {}).get("logsByTaskId", {})
    reuse_history = (session_registry or {}).get("reuseHistory", [])
    sessions_by_id = {
        session.get("sessionId"): session
        for session in (session_registry or {}).get("sessions", [])
        if session.get("sessionId")
    }
    for thread_key, thread in threads.items():
        task_ids = set(task_ids_by_thread.get(thread_key, set()))
        unresolved_failures = 0
        for task_id in task_ids:
            unresolved_failures += len(feedback_by_task.get(task_id, {}).get("recentFailures", []))
        thread["unresolvedFailureCount"] = unresolved_failures
        resume_count = sum(1 for item in reuse_history if item.get("taskId") in task_ids and item.get("strategy") == "resume")
        thread["resumeCount"] = resume_count
        divergence_count = sum(
            1
            for task in tasks
            if resolve_task_thread_key(task, request_index, task_map) == thread_key
            and resolve_task_plan_epoch(task, request_index, task_map) < thread["currentPlanEpoch"]
            and task.get("status") not in TASK_COMPLETED_STATUSES | {"superseded"}
        )
        thread["epochDivergenceCount"] = divergence_count
        missing_compression = sum(1 for task_id in task_ids if task_id and task_id not in logs_by_task)
        thread["missingCompressionCount"] = missing_compression
        latest_session_time = None
        for session_id in set(thread.get("activeSessionIds", [])):
            session = sessions_by_id.get(session_id, {})
            last_used = parse_iso(session.get("lastUsedAt"))
            if last_used and (latest_session_time is None or last_used > latest_session_time):
                latest_session_time = last_used
        session_age = None
        if latest_session_time is not None:
            session_age = max(0, int((datetime.now(latest_session_time.tzinfo or timezone.utc) - latest_session_time).total_seconds()))
        thread["sessionAgeSeconds"] = session_age
        stale_summary_age = age_seconds(thread.get("lastUpdatedAt"))
        score = 0
        reasons = []
        if resume_count > policy_summary["contextRot"]["maxResumeCount"]:
            score += 2
            reasons.append("resume count high")
        if thread["appendChangeCount"] > policy_summary["contextRot"].get("maxAppendedChanges", 3):
            score += 2
            reasons.append("append change count high")
        if divergence_count > 0:
            score += 2
            reasons.append("thread has older-epoch tasks")
        if unresolved_failures > policy_summary["contextRot"].get("maxUnresolvedFailures", 3):
            score += 2
            reasons.append("unresolved failures high")
        if stale_summary_age is not None and stale_summary_age > policy_summary["contextRot"]["staleSummaryAfterSeconds"]:
            score += 1
            reasons.append("thread summary stale")
        if missing_compression > 0:
            score += 1
            reasons.append("missing compact checkpoints")
        if session_age is not None and session_age > policy_summary["contextRot"].get("sessionAgeWarnSeconds", 7200):
            score += 2
            reasons.append("session age high")
        if session_age is not None and session_age > policy_summary["contextRot"].get("sessionAgeFreshSeconds", 21600):
            score += 2
            reasons.append("session age exceeds fresh threshold")
        thread["contextRotScore"] = score
        if score >= policy_summary["contextRot"]["freshSessionScore"]:
            thread["contextRotStatus"] = "downgraded"
        elif score >= policy_summary["contextRot"]["warnScore"]:
            thread["contextRotStatus"] = "warning"
        else:
            thread["contextRotStatus"] = "healthy" if score else "fresh"
        thread["contextRotReasons"] = reasons[:5]
        thread["activeTaskIds"] = bounded(list(dict.fromkeys(thread.get("activeTaskIds", []))), hot_limit)
        thread["queuedTaskIds"] = bounded(list(dict.fromkeys(thread.get("queuedTaskIds", []))), hot_limit)
        thread["recoverableTaskIds"] = bounded(list(dict.fromkeys(thread.get("recoverableTaskIds", []))), hot_limit)
        thread["completedTaskIds"] = bounded(list(dict.fromkeys(thread.get("completedTaskIds", []))), hot_limit)
        thread["supersededTaskIds"] = bounded(list(dict.fromkeys(thread.get("supersededTaskIds", []))), hot_limit)
        thread["activeSessionIds"] = bounded(list(dict.fromkeys(thread.get("activeSessionIds", []))), hot_limit)

    active_threads = [
        {
            "threadKey": key,
            "currentPlanEpoch": value.get("currentPlanEpoch"),
            "contextRotScore": value.get("contextRotScore"),
            "contextRotStatus": value.get("contextRotStatus"),
            "activeTaskIds": value.get("activeTaskIds", []),
            "openRequestCount": value.get("openRequestCount", 0),
            "lastFusionDecision": value.get("lastFusionDecision"),
        }
        for key, value in threads.items()
        if value.get("openRequestCount") or value.get("activeTaskIds")
    ]
    active_threads.sort(key=lambda item: (-len(item.get("activeTaskIds", [])), -(item.get("openRequestCount") or 0), item.get("threadKey") or ""))
    return {
        "schemaVersion": SCHEMA_VERSION,
        "generator": generator,
        "generatedAt": now_iso(),
        "threadCount": len(threads),
        "activeThreadCount": len(active_threads),
        "activeThreadKey": active_threads[0].get("threadKey") if active_threads else None,
        "threads": threads,
        "activeThreads": bounded(active_threads, policy_summary["intake"]["maxRecentThreads"]),
        "contextRotWarnings": bounded(
            [
                {
                    "threadKey": key,
                    "contextRotScore": value.get("contextRotScore"),
                    "contextRotStatus": value.get("contextRotStatus"),
                    "reasons": value.get("contextRotReasons", []),
                }
                for key, value in threads.items()
                if value.get("contextRotScore", 0) >= policy_summary["contextRot"]["warnScore"]
            ],
            policy_summary["intake"]["maxRecentThreads"],
        ),
    }


def build_intake_summary(index: dict, thread_state: dict | None, *, generator: str, policy_summary: dict) -> dict:
    requests = index.get("requests", [])
    recent_limit = policy_summary["intake"]["maxRecentSubmissions"]
    recent = requests[-recent_limit:]
    return {
        "schemaVersion": SCHEMA_VERSION,
        "generator": generator,
        "generatedAt": now_iso(),
        "submissionCount": len(requests),
        "byIntentClass": dict(Counter(item.get("normalizedIntentClass", "unclassified") for item in requests)),
        "byFusionDecision": dict(Counter(item.get("fusionDecision", "unclassified") for item in requests)),
        "duplicateCount": sum(1 for item in requests if item.get("normalizedIntentClass") == "duplicate_or_noop"),
        "contextMergeCount": sum(1 for item in requests if item.get("fusionDecision") == "merged_as_context"),
        "inspectionOverlayCount": sum(1 for item in requests if item.get("fusionDecision") == "inspection_overlay"),
        "appendRequiresReplanCount": sum(1 for item in requests if item.get("fusionDecision") == "append_requires_replan"),
        "compoundSplitCount": sum(1 for item in requests if item.get("fusionDecision") == "compound_split_created"),
        "recentSubmissions": [
            {
                "requestId": item.get("requestId"),
                "goal": item.get("goal"),
                "status": item.get("status"),
                "threadKey": thread_key_from_request(item),
                "planEpoch": item.get("targetPlanEpoch") or item.get("planEpoch"),
                "normalizedIntentClass": item.get("normalizedIntentClass"),
                "fusionDecision": item.get("fusionDecision"),
                "impactClassification": item.get("impactClassification"),
                "duplicateOfRequestId": item.get("duplicateOfRequestId"),
                "mergedIntoRequestId": item.get("mergedIntoRequestId"),
                "classificationReason": item.get("classificationReason"),
            }
            for item in recent
        ],
        "contextRotWarnings": bounded((thread_state or {}).get("contextRotWarnings", []), policy_summary["intake"]["maxRecentThreads"]),
    }


def build_change_summary(index: dict, thread_state: dict | None, *, generator: str, policy_summary: dict) -> dict:
    requests = index.get("requests", [])
    changes = [
        item for item in requests
        if item.get("normalizedIntentClass") in {"append_change", "compound_split", "inspection", "context_enrichment"}
    ]
    recent_limit = policy_summary["intake"]["maxRecentChanges"]
    return {
        "schemaVersion": SCHEMA_VERSION,
        "generator": generator,
        "generatedAt": now_iso(),
        "changeCount": len(changes),
        "recentChanges": [
            {
                "requestId": item.get("requestId"),
                "threadKey": thread_key_from_request(item),
                "planEpoch": item.get("targetPlanEpoch") or item.get("planEpoch"),
                "normalizedIntentClass": item.get("normalizedIntentClass"),
                "fusionDecision": item.get("fusionDecision"),
                "impactClassification": item.get("impactClassification"),
                "supersededQueuedTaskIds": item.get("supersededQueuedTaskIds", []),
                "impactedTaskIds": item.get("impactedTaskIds", []),
                "effectRequestIds": item.get("effectRequestIds", []),
            }
            for item in changes[-recent_limit:]
        ],
        "activeThreads": bounded((thread_state or {}).get("activeThreads", []), policy_summary["intake"]["maxRecentThreads"]),
    }


def latest_log_for_task(log_index: dict | None, task_id: str | None):
    if not log_index or not task_id:
        return None
    item = (log_index.get("logsByTaskId") or {}).get(task_id)
    if isinstance(item, list):
        return item[-1] if item else None
    return item


def evaluate_task_guardrails(
    task: dict,
    *,
    request: dict | None,
    thread_state: dict | None,
    feedback_summary: dict | None,
    log_index: dict | None,
    policy_summary: dict,
    phase: str,
) -> dict:
    thread_key = thread_key_from_request(request or {}) or task.get("threadKey")
    thread_entry = ((thread_state or {}).get("threads") or {}).get(thread_key or "", {})
    latest_epoch = int(thread_entry.get("currentPlanEpoch") or request.get("targetPlanEpoch") or task.get("planEpoch") or 1)
    task_epoch = int(task.get("planEpoch") or request.get("targetPlanEpoch") or 1)
    failures = []
    warnings = []
    severe_failures = (
        (feedback_summary or {}).get("taskFeedbackSummary", {})
        .get(task.get("taskId"), {})
        .get("recentFailures", [])
    )
    if policy_summary["driftGuards"]["failOnOlderEpoch"] and task_epoch < latest_epoch:
        failures.append("task is on an older plan epoch than the thread")
    if task.get("status") in TASK_SUPERSEDED_STATUSES or task.get("status") == "superseded":
        failures.append("task has been superseded and should not be dispatched")
    if task.get("roleHint") == "worker" and task.get("kind") != "audit" and not task.get("ownedPaths"):
        failures.append("worker task is missing ownedPaths")
    if task.get("verificationStatus") == "fail":
        warnings.append("latest verification failed")
    if severe_failures:
        warnings.append("task has unresolved recent failures")
    if phase in {"pre_resume", "pre_dispatch"} and thread_entry.get("contextRotScore", 0) >= policy_summary["contextRot"]["freshSessionScore"]:
        failures.append("context rot score exceeds fresh-session threshold")
    if phase in {"pre_resume", "pre_dispatch"} and policy_summary["driftGuards"]["requireCompactLogForResume"]:
        if not latest_log_for_task(log_index, task.get("taskId")):
            warnings.append("compact handoff log is missing for resume-sensitive task")
    if phase == "pre_finalize":
        if task_epoch < latest_epoch:
            failures.append("newer appended requirements exist on this thread")
        if not task.get("diffSummary"):
            warnings.append("diffSummary is missing")
        if task.get("verificationResultPath") is None:
            warnings.append("verification result path is missing")
    return {
        "phase": phase,
        "ok": not failures,
        "threadKey": thread_key,
        "taskPlanEpoch": task_epoch,
        "latestValidPlanEpoch": latest_epoch,
        "contextRotScore": thread_entry.get("contextRotScore", 0),
        "contextRotStatus": thread_entry.get("contextRotStatus"),
        "failures": failures,
        "warnings": warnings,
        "checkedAt": now_iso(),
    }


def normalize_request_record(request: dict, *, index: dict, task_map: dict, thread_state: dict | None = None, generator: str = "klein-harness") -> dict:
    request["submissionId"] = request.get("submissionId") or request.get("requestId")
    request["submittedKindHint"] = request.get("submittedKindHint") or request.get("kind")
    request["kindHint"] = request.get("kindHint") or request.get("kind")
    request["kind"] = request.get("kind") or "implementation"
    request["contextPaths"] = request.get("contextPaths") or []
    classification = classify_submission(request, index=index, task_map=task_map, thread_state=thread_state)
    request.update(classification)
    request["idempotencyKey"] = idempotency_key_for_submission(
        request.get("targetThreadKey"),
        request.get("idempotencyKey"),
        request.get("canonicalGoalHash"),
        request.get("evidenceFingerprint"),
    )
    request["updatedAt"] = request.get("updatedAt") or request.get("createdAt") or now_iso()
    request["generator"] = generator
    return request


def ensure_request_index_shape(index: dict) -> dict:
    index.setdefault("requests", [])
    index.setdefault("threads", {})
    return index


def ensure_thread_ledger(index: dict, thread_key: str | None, *, generator: str, created_at: str | None = None) -> dict | None:
    if not thread_key:
        return None
    ensure_request_index_shape(index)
    created_at = created_at or now_iso()
    threads = index.setdefault("threads", {})
    thread = threads.get(thread_key)
    if thread is None:
        thread = {
            "threadKey": thread_key,
            "createdAt": created_at,
            "updatedAt": created_at,
            "currentPlanEpoch": DEFAULT_POLICY_SUMMARY["threading"]["defaultInitialPlanEpoch"],
            "latestSubmissionId": None,
            "latestRequestId": None,
            "lastFusionDecision": None,
            "lastIntentClass": None,
            "lastImpactClassification": None,
            "activeRequestIds": [],
            "recentRequestIds": [],
            "mergedContextRequestIds": [],
            "mergedContextRefs": [],
            "latestContextPaths": [],
            "contextDigest": None,
            "duplicateRequestIds": [],
            "inspectionRequestIds": [],
            "appendChangeCount": 0,
            "contextEnrichmentCount": 0,
            "inspectionCount": 0,
            "duplicateCount": 0,
            "compoundCount": 0,
            "openRequestCount": 0,
            "supersededTaskIds": [],
            "checkpointTaskIds": [],
        }
        threads[thread_key] = thread
    return thread


def bounded_unique(values, limit: int):
    result = []
    seen = set()
    for value in values or []:
        if not value or value in seen:
            continue
        seen.add(value)
        result.append(value)
        if len(result) >= limit:
            break
    return result


def record_request_thread_state(
    index: dict,
    request: dict,
    *,
    impact_classification: str | None = None,
    impacted_task_ids: list[str] | None = None,
    superseded_task_ids: list[str] | None = None,
    checkpoint_task_ids: list[str] | None = None,
    generator: str,
):
    ensure_request_index_shape(index)
    thread_key = thread_key_from_request(request)
    thread = ensure_thread_ledger(index, thread_key, generator=generator, created_at=request.get("createdAt"))
    if thread is None:
        return None
    request_id = request.get("requestId")
    recent_limit = DEFAULT_POLICY_SUMMARY["threading"]["maxRecentThreadEvents"]
    thread["updatedAt"] = now_iso()
    thread["latestSubmissionId"] = request.get("submissionId") or request_id
    thread["latestRequestId"] = request_id
    thread["lastFusionDecision"] = request.get("fusionDecision")
    thread["lastIntentClass"] = request.get("normalizedIntentClass")
    if impact_classification:
        thread["lastImpactClassification"] = impact_classification
    if request.get("targetPlanEpoch"):
        thread["currentPlanEpoch"] = max(int(thread.get("currentPlanEpoch") or 1), int(request.get("targetPlanEpoch") or 1))
    recent_request_ids = [request_id, *(thread.get("recentRequestIds") or [])]
    thread["recentRequestIds"] = bounded_unique(recent_request_ids, recent_limit)
    if request_effect_terminal(request):
        active_request_ids = [item for item in thread.get("activeRequestIds", []) if item != request_id]
    else:
        active_request_ids = [request_id, *(thread.get("activeRequestIds") or [])]
    thread["activeRequestIds"] = bounded_unique(active_request_ids, recent_limit)
    normalized_intent = request.get("normalizedIntentClass")
    if normalized_intent == "append_change":
        thread["appendChangeCount"] = int(thread.get("appendChangeCount") or 0) + 1
    elif normalized_intent == "context_enrichment":
        thread["contextEnrichmentCount"] = int(thread.get("contextEnrichmentCount") or 0) + 1
        thread["mergedContextRequestIds"] = bounded_unique(
            [request_id, *(thread.get("mergedContextRequestIds") or [])],
            recent_limit,
        )
        merged_refs = [
            {
                "requestId": request_id,
                "goal": request.get("goal"),
                "contextPaths": bounded(request.get("contextPaths", []), 5),
                "mergedIntoRequestId": request.get("mergedIntoRequestId"),
                "updatedAt": request.get("updatedAt") or request.get("createdAt"),
            },
            *(thread.get("mergedContextRefs") or []),
        ]
        thread["mergedContextRefs"] = bounded(merged_refs, recent_limit)
        thread["latestContextPaths"] = bounded_unique(
            [*(request.get("contextPaths") or []), *(thread.get("latestContextPaths") or [])],
            recent_limit,
        )
        thread["contextDigest"] = stable_short_hash(
            thread_key or "",
            canonicalize_goal_text(request.get("goal") or ""),
            ",".join(thread.get("latestContextPaths") or []),
        )
    elif normalized_intent == "inspection":
        thread["inspectionCount"] = int(thread.get("inspectionCount") or 0) + 1
        thread["inspectionRequestIds"] = bounded_unique(
            [request_id, *(thread.get("inspectionRequestIds") or [])],
            recent_limit,
        )
    elif normalized_intent == "duplicate_or_noop":
        thread["duplicateCount"] = int(thread.get("duplicateCount") or 0) + 1
        thread["duplicateRequestIds"] = bounded_unique(
            [request_id, *(thread.get("duplicateRequestIds") or [])],
            recent_limit,
        )
    elif normalized_intent == "compound_split":
        thread["compoundCount"] = int(thread.get("compoundCount") or 0) + 1
    thread["openRequestCount"] = len(thread.get("activeRequestIds") or [])
    if impacted_task_ids:
        thread["impactedTaskIds"] = bounded_unique(impacted_task_ids, recent_limit)
    if superseded_task_ids:
        thread["supersededTaskIds"] = bounded_unique(
            [*superseded_task_ids, *(thread.get("supersededTaskIds") or [])],
            recent_limit,
        )
    if checkpoint_task_ids:
        thread["checkpointTaskIds"] = bounded_unique(
            [*checkpoint_task_ids, *(thread.get("checkpointTaskIds") or [])],
            recent_limit,
        )
    return thread


def latest_request_for_thread(index: dict, thread_key: str | None) -> dict | None:
    if not thread_key:
        return None
    thread_requests = [request for request in index.get("requests", []) if thread_key_from_request(request) == thread_key]
    if not thread_requests:
        return None
    return sorted(thread_requests, key=lambda item: (item.get("seq", 0), item.get("createdAt") or ""))[-1]


def active_tasks_for_thread(tasks: list[dict], thread_key: str | None) -> list[dict]:
    if not thread_key:
        return []
    return [task for task in tasks if task.get("threadKey") == thread_key and task.get("status") in TASK_ACTIVE_STATUSES]


def queued_tasks_for_thread(tasks: list[dict], thread_key: str | None) -> list[dict]:
    if not thread_key:
        return []
    return [task for task in tasks if task.get("threadKey") == thread_key and task.get("status") == "queued"]


def recent_verification_for_thread(tasks: list[dict], thread_key: str | None) -> list[dict]:
    related = [
        {
            "taskId": task.get("taskId"),
            "verificationStatus": task.get("verificationStatus"),
            "verificationResultPath": task.get("verificationResultPath"),
            "updatedAt": task.get("updatedAt") or task.get("completedAt"),
        }
        for task in tasks
        if task.get("threadKey") == thread_key and (task.get("verificationStatus") or task.get("verificationResultPath"))
    ]
    related.sort(key=lambda item: (item.get("updatedAt") or "", item.get("taskId") or ""))
    return related[-5:]


def infer_request_impact_class(request: dict, tasks: list[dict], feedback_summary: dict | None = None) -> tuple[str, list[dict]]:
    thread_key = request.get("targetThreadKey") or request.get("threadKey")
    active_tasks = active_tasks_for_thread(tasks, thread_key)
    queued_tasks = queued_tasks_for_thread(tasks, thread_key)
    impacted = [
        {
            "taskId": task.get("taskId"),
            "status": task.get("status"),
            "roleHint": task.get("roleHint"),
            "worktreePath": task.get("worktreePath"),
            "ownedPaths": bounded(task.get("ownedPaths", []), 8),
        }
        for task in active_tasks + queued_tasks
    ]
    normalized_intent = request.get("normalizedIntentClass")
    if normalized_intent == "inspection":
        return "inspection_only_overlay", impacted
    if normalized_intent == "context_enrichment":
        return "continue_with_note", impacted
    if normalized_intent == "append_change":
        if queued_tasks:
            return "supersede_queued", impacted
        if active_tasks:
            return "checkpoint_then_replan", impacted
        return "continue_with_note", impacted
    if normalized_intent == "compound_split":
        return "checkpoint_then_replan" if active_tasks else "continue_with_note", impacted
    return "continue_safe", impacted


def ensure_task_thread_metadata(task: dict, request: dict):
    target_thread_key = request.get("targetThreadKey") or request.get("threadKey")
    if target_thread_key and (
        not task.get("threadKey")
        or (
            task.get("threadKey") != target_thread_key
            and task.get("status") in {None, "queued", "blocked", "recoverable", "superseded"}
        )
    ):
        task["threadKey"] = target_thread_key
    if request.get("targetPlanEpoch"):
        target_epoch = int(request.get("targetPlanEpoch") or 1)
        current_epoch = int(task.get("planEpoch") or 0)
        if current_epoch < target_epoch and task.get("status") in {None, "queued", "blocked", "recoverable", "superseded"}:
            task["planEpoch"] = target_epoch
    return task


def supersede_queued_thread_tasks(root: Path, request: dict, *, generator: str) -> list[str]:
    files = ensure_runtime_scaffold(root, generator=generator)
    task_pool = read_task_pool(files["harness"])
    if not task_pool:
        return []
    thread_key = request.get("targetThreadKey") or request.get("threadKey")
    if not thread_key:
        return []
    superseded = []
    for task in task_pool.get("tasks", []):
        if task.get("threadKey") != thread_key:
            continue
        if task.get("status") != "queued":
            continue
        task["status"] = "superseded"
        task["supersededAt"] = now_iso()
        task["supersededByRequestId"] = request.get("requestId")
        superseded.append(task.get("taskId"))
    if superseded:
        write_json(files["harness"] / "task-pool.json", task_pool)
        lineage_event(
            root,
            "thread.supersede_queued",
            generator,
            request_id=request.get("requestId"),
            detail="superseded queued tasks",
            context={"taskIds": superseded, "threadKey": thread_key},
        )
    return superseded


def checkpoint_active_thread_tasks(root: Path, request: dict, *, generator: str) -> list[str]:
    files = ensure_runtime_scaffold(root, generator=generator)
    task_pool = read_task_pool(files["harness"])
    if not task_pool:
        return []
    thread_key = request.get("targetThreadKey") or request.get("threadKey")
    if not thread_key:
        return []
    checkpointed = []
    for task in task_pool.get("tasks", []):
        if task.get("threadKey") != thread_key:
            continue
        if task.get("status") not in TASK_ACTIVE_STATUSES:
            continue
        task["checkpointRequired"] = True
        task["checkpointReason"] = "new appended requirement on same thread"
        task["latestPlanEpoch"] = request.get("targetPlanEpoch")
        task["updatedAt"] = now_iso()
        checkpointed.append(task.get("taskId"))
    if checkpointed:
        write_json(files["harness"] / "task-pool.json", task_pool)
        lineage_event(
            root,
            "thread.checkpoint_required",
            generator,
            request_id=request.get("requestId"),
            detail="checkpoint required before replan",
            context={"taskIds": checkpointed, "threadKey": thread_key},
        )
    return checkpointed


def context_rot_score(task: dict, request_summary: dict | None, heartbeats: dict | None, thread_state: dict | None, compact_log: dict | None, policy_summary: dict) -> dict:
    heartbeats = heartbeats or {}
    thread_state = thread_state or {}
    request_summary = request_summary or {}
    score = 0
    reasons = []
    task_id = task.get("taskId")
    heartbeat = heartbeats.get(task_id, {})
    resume_count = int(task.get("resumeCount") or len(task.get("candidateResumeSessionIds") or []))
    if resume_count > policy_summary["contextRot"]["maxResumeCount"]:
        score += 2
        reasons.append("resume count exceeded")
    session_age = age_seconds(task.get("claim", {}).get("boundAt") or task.get("lastDispatchAt"))
    if session_age is not None and session_age > policy_summary["contextRot"]["sessionAgeWarnSeconds"]:
        score += 1
        reasons.append("session age high")
    if session_age is not None and session_age > policy_summary["contextRot"]["sessionAgeFreshSeconds"]:
        score += 2
        reasons.append("session age exceeds fresh threshold")
    thread_key = task.get("threadKey")
    latest_epoch = current_plan_epoch_for_thread(thread_state, thread_key)
    if task.get("planEpoch") and task.get("planEpoch") < latest_epoch:
        score += 3
        reasons.append("task on stale plan epoch")
    append_changes = len([
        item for item in request_summary.get("recentRequests", [])
        if thread_key_from_request(item) == thread_key and item.get("normalizedIntentClass") == "append_change"
    ])
    if append_changes >= 2:
        score += 1
        reasons.append("multiple appended changes on thread")
    failure_count = len((task.get("recentFailures") or []))
    if failure_count >= 2:
        score += 1
        reasons.append("unresolved failures")
    heartbeat_age = age_seconds(heartbeat.get("lastHeartbeatAt"))
    if heartbeat_age is not None and heartbeat_age > policy_summary["contextRot"]["staleSummaryAfterSeconds"]:
        score += 1
        reasons.append("stale heartbeat")
    if not compact_log:
        score += 1
        reasons.append("missing compact checkpoint")
    status = "fresh"
    if score >= policy_summary["contextRot"]["freshSessionScore"]:
        status = "downgraded"
    elif score >= policy_summary["contextRot"]["warnScore"]:
        status = "warning"
    else:
        status = "healthy"
    return {
        "score": score,
        "status": status,
        "reasons": reasons,
        "latestPlanEpoch": latest_epoch,
    }


def evaluate_task_drift_checklist(task: dict, *, latest_plan_epoch: int | None, request_summary: dict | None, compact_log: dict | None) -> dict:
    failures = []
    if latest_plan_epoch is not None and task.get("planEpoch") and task.get("planEpoch") < latest_plan_epoch:
        failures.append("task not on latest valid plan epoch")
    if task.get("status") == "superseded":
        failures.append("task superseded")
    if not task.get("ownedPaths"):
        failures.append("ownedPaths missing")
    if task.get("verificationStatus") == "fail":
        failures.append("verification failed")
    if not compact_log and task.get("status") in {"resumed", "recoverable"}:
        failures.append("compact handoff missing")
    thread_key = task.get("threadKey")
    if request_summary and thread_key:
        newer_appends = [
            request for request in request_summary.get("recentRequests", [])
            if thread_key_from_request(request) == thread_key
            and request.get("normalizedIntentClass") == "append_change"
            and (request.get("targetPlanEpoch") or 0) > (task.get("planEpoch") or 0)
        ]
        if newer_appends:
            failures.append("newer appended requirement exists on thread")
    return {
        "status": "fail" if failures else "pass",
        "failures": failures,
    }


def task_kind_matches_request(request_kind: str, task: dict):
    kind = (request_kind or "").lower()
    task_kind = (task.get("kind") or "").lower()
    role = (task.get("roleHint") or "").lower()
    worker_mode = (task.get("workerMode") or "").lower()
    if kind == "stop":
        return task.get("status") in TASK_ACTIVE_STATUSES
    if kind == "audit":
        return task_kind == "audit" or worker_mode == "audit"
    if kind in {"replan", "bootstrap"}:
        return role == "orchestrator" or task_kind in {"replan", "orchestration"}
    if kind == "status":
        return True
    if kind in {"analysis", "research"}:
        return role == "orchestrator" or task_kind in {"analysis", "research", "audit"}
    if kind in {"implementation", "change"}:
        return role in {"worker", "orchestrator"} and task_kind != "audit"
    return True


def task_matches_intent(request: dict, task: dict) -> bool:
    intents = request.get("internalIntents") or [request.get("normalizedIntentClass")]
    request_kind = (request.get("effectiveKind") or request.get("kind") or "").lower()
    task_kind = (task.get("kind") or "").lower()
    role = (task.get("roleHint") or "").lower()
    worker_mode = (task.get("workerMode") or "").lower()
    if request_kind in {"stop", "audit", "replan", "bootstrap", "status", "analysis", "research"}:
        return task_kind_matches_request(request_kind, task)
    if "inspection" in intents:
        if task_kind == "audit" or worker_mode == "audit":
            return True
        if role == "orchestrator":
            return True
    if "append_change" in intents or "fresh_work" in intents or "compound_split" in intents:
        if role in {"worker", "orchestrator"} and task_kind != "audit":
            return True
    if "context_enrichment" in intents and role == "orchestrator":
        return True
    return task_kind_matches_request(request_kind, task)


def task_status_score(task: dict):
    status = task.get("status")
    if status in TASK_SUPERSEDED_STATUSES:
        return -100
    if status == "queued":
        return 30
    if status in {"claimed", "active", "in_progress", "dispatched", "running", "resumed"}:
        return 20
    if status in {"verified", "completed", "done", "pass"}:
        return -50
    return 0


def task_priority_score(task: dict):
    priority = task.get("priority") or "P9"
    match = re.match(r"P(\d+)", priority)
    if not match:
        return 0
    return max(0, 20 - int(match.group(1)) * 3)


def candidate_tasks_for_request(request: dict, tasks: list[dict]):
    request_kind = (request.get("effectiveKind") or request.get("kind") or "").lower()
    strict_kind_match = request_kind in {"stop", "audit", "replan", "bootstrap", "status", "analysis", "research"}
    goal_tokens = request_goal_tokens(request.get("goal"))
    scored = []
    for task in tasks:
        score = task_status_score(task)
        if score < 0:
            continue
        if strict_kind_match and not task_kind_matches_request(request_kind, task):
            continue
        planning_stage = task.get("planningStage")
        if planning_stage and planning_stage != "execution-ready" and (task.get("kind") or "").lower() not in {
            "audit",
            "analysis",
            "orchestration",
            "replan",
            "rollback",
            "lease-recovery",
        }:
            continue
        if task_matches_intent(request, task):
            score += 50
        if request.get("targetThreadKey") and task.get("threadKey") == request.get("targetThreadKey"):
            score += 40
        if request.get("targetPlanEpoch") and task.get("planEpoch") == request.get("targetPlanEpoch"):
            score += 10
        haystack = " ".join([
            task.get("taskId") or "",
            task.get("title") or "",
            task.get("summary") or "",
            task.get("kind") or "",
            task.get("roleHint") or "",
            task.get("threadKey") or "",
        ]).lower()
        token_hits = sum(1 for token in goal_tokens if token in haystack)
        score += token_hits * 8
        score += task_priority_score(task)
        if task.get("taskId") and task.get("taskId").lower() in (request.get("goal") or "").lower():
            score += 25
        scored.append((score, task))

    scored.sort(key=lambda item: (-item[0], item[1].get("taskId") or ""))
    if (request.get("effectiveKind") or request.get("kind") or "").lower() == "stop":
        return [task for score, task in scored if score > 0][:3]
    if request.get("normalizedIntentClass") == "compound_split":
        return [task for score, task in scored if score > 0][:2]
    return [task for score, task in scored if score > 0][:1]


def lineage_event(
    root: Path,
    kind: str,
    generator: str,
    *,
    request_id: str | None = None,
    task_id: str | None = None,
    session_id: str | None = None,
    worktree_path: str | None = None,
    diff_summary: str | None = None,
    verification: dict | None = None,
    outcome: dict | None = None,
    actor: dict | None = None,
    detail: str | None = None,
    reason: str | None = None,
    context: dict | None = None,
):
    files = ensure_runtime_scaffold(root, generator=generator)
    existing = load_jsonl(files["lineage_path"])
    seq = int(existing[-1].get("seq", 0)) + 1 if existing else 1
    event = {
        "seq": seq,
        "timestamp": now_iso(),
        "generator": generator,
        "kind": kind,
        "requestId": request_id,
        "taskId": task_id,
        "sessionId": session_id,
        "worktreePath": worktree_path,
        "diffSummary": diff_summary,
        "verification": verification,
        "outcome": outcome,
        "actor": actor or {"role": "runtime", "id": generator},
        "detail": detail,
        "reason": reason,
        "context": context or {},
    }
    append_jsonl(files["lineage_path"], event)
    return event


def sync_request_from_bindings(request: dict, bindings: list[dict]):
    request["boundTaskIds"] = [binding.get("taskId") for binding in bindings]
    request["bindingIds"] = [binding.get("bindingId") for binding in bindings]
    request["activeTaskId"] = bindings[-1].get("taskId") if bindings else None
    request["sessionIds"] = [
        binding.get("lineage", {}).get("sessionId")
        for binding in bindings
        if binding.get("lineage", {}).get("sessionId")
    ]
    if bindings:
        latest = bindings[-1]
        request["threadKey"] = latest.get("lineage", {}).get("threadKey") or request.get("threadKey")
        request["targetPlanEpoch"] = latest.get("lineage", {}).get("planEpoch") or request.get("targetPlanEpoch")
        request["worktreePath"] = latest.get("lineage", {}).get("worktreePath")
        request["diffSummary"] = latest.get("lineage", {}).get("diffSummary")
        request["verificationStatus"] = latest.get("lineage", {}).get("verificationStatus")
        request["verificationResultPath"] = latest.get("lineage", {}).get("verificationResultPath")
        request["outcome"] = latest.get("lineage", {}).get("outcome")
        request["lastBindingStatus"] = latest.get("status")
        request["statusReason"] = latest.get("statusReason")
        request["updatedAt"] = latest.get("updatedAt")
    return request


def update_request_status(index: dict, request_id: str, status: str, *, reason: str | None = None, extra: dict | None = None):
    request = find_request(index.get("requests", []), request_id)
    request["status"] = status
    request["updatedAt"] = now_iso()
    if reason:
        request["statusReason"] = reason
    if extra:
        request.update({key: value for key, value in extra.items() if value is not None})
    return request


def ensure_binding(
    root: Path,
    request_id: str,
    task: dict,
    *,
    status: str = "bound",
    reason: str,
    generator: str,
):
    files = ensure_runtime_scaffold(root, generator=generator)
    index = load_json(files["request_index_path"])
    task_map = load_json(files["request_task_map_path"])
    request = find_request(index.get("requests", []), request_id)
    task_pool = read_task_pool(files["harness"])
    if task_pool is not None:
        task = find_task(task_pool.get("tasks", []), task.get("taskId"))
        ensure_task_thread_metadata(task, request)
        write_json(files["harness"] / "task-pool.json", task_pool)
    binding = find_binding(task_map.get("bindings", []), request_id, task.get("taskId"))
    timestamp = now_iso()

    if binding is None:
        binding_id = build_binding_id(int(task_map.get("nextBindingSeq", 1)))
        binding = {
            "bindingId": binding_id,
            "requestId": request_id,
            "taskId": task.get("taskId"),
            "workItemId": task.get("workItemId"),
            "taskTitle": task.get("title"),
            "taskKind": task.get("kind"),
            "status": status,
            "statusReason": reason,
            "bindingReason": reason,
            "createdAt": timestamp,
            "updatedAt": timestamp,
            "lineage": {
                "sessionId": task.get("claim", {}).get("boundSessionId") or task.get("preferredResumeSessionId"),
                "worktreePath": task.get("worktreePath"),
                "threadKey": task.get("threadKey"),
                "planEpoch": task.get("planEpoch"),
                "diffBase": task.get("diffBase") or task.get("dispatch", {}).get("diffBase"),
                "diffSummary": task.get("diffSummary"),
                "verificationStatus": task.get("verificationStatus"),
                "verificationSummary": task.get("verificationSummary"),
                "verificationResultPath": task.get("verificationResultPath"),
                "outcome": task.get("outcome"),
            },
            "route": None,
            "history": [{
                "status": status,
                "reason": reason,
                "timestamp": timestamp,
                "generator": generator,
            }],
        }
        task_map.setdefault("bindings", []).append(binding)
        task_map["nextBindingSeq"] = int(task_map.get("nextBindingSeq", 1)) + 1
    else:
        binding["updatedAt"] = timestamp
        binding["status"] = status
        binding["statusReason"] = reason
        binding.setdefault("history", []).append({
            "status": status,
            "reason": reason,
            "timestamp": timestamp,
            "generator": generator,
        })

    task_map["generatedAt"] = timestamp
    task_map["generator"] = generator
    request = update_request_status(index, request_id, status, reason=reason)
    sync_request_from_bindings(
        request,
        [item for item in task_map.get("bindings", []) if item.get("requestId") == request_id],
    )
    index["generatedAt"] = timestamp
    index["generator"] = generator
    write_json(files["request_task_map_path"], task_map)
    write_json(files["request_index_path"], index)

    lineage_event(
        root,
        f"request.{status}",
        generator,
        request_id=request_id,
        task_id=task.get("taskId"),
        session_id=binding.get("lineage", {}).get("sessionId"),
        worktree_path=binding.get("lineage", {}).get("worktreePath"),
        diff_summary=binding.get("lineage", {}).get("diffSummary"),
        outcome=binding.get("lineage", {}).get("outcome"),
        detail=reason,
    )
    return binding


def update_binding_state(
    root: Path,
    request_id: str,
    task_id: str,
    status: str,
    *,
    reason: str,
    generator: str,
    session_id: str | None = None,
    route: dict | None = None,
    verification: dict | None = None,
    outcome: dict | None = None,
    worktree_path: str | None = None,
    diff_summary: str | None = None,
):
    files = ensure_runtime_scaffold(root, generator=generator)
    index = load_json(files["request_index_path"])
    task_map = load_json(files["request_task_map_path"])
    task_pool = read_task_pool(files["harness"])
    binding = find_binding(task_map.get("bindings", []), request_id, task_id)
    if binding is None:
        if task_pool is None:
            raise KeyError(f"binding not found: {request_id} -> {task_id}")
        task = find_task(task_pool.get("tasks", []), task_id)
        binding = ensure_binding(root, request_id, task, status="bound", reason="late binding recovery", generator=generator)
        task_map = load_json(files["request_task_map_path"])
        binding = find_binding(task_map.get("bindings", []), request_id, task_id)

    timestamp = now_iso()
    binding["status"] = status
    binding["statusReason"] = reason
    binding["updatedAt"] = timestamp
    binding.setdefault("history", []).append({
        "status": status,
        "reason": reason,
        "timestamp": timestamp,
        "generator": generator,
    })
    lineage = binding.setdefault("lineage", {})
    if session_id is not None:
        lineage["sessionId"] = session_id
    if worktree_path is not None:
        lineage["worktreePath"] = worktree_path
    if "threadKey" not in lineage and task_pool is not None:
        task = find_task(task_pool.get("tasks", []), task_id)
        lineage["threadKey"] = task.get("threadKey")
        lineage["planEpoch"] = task.get("planEpoch")
    if diff_summary is not None:
        lineage["diffSummary"] = diff_summary
    if verification:
        lineage["verificationStatus"] = verification.get("overallStatus")
        lineage["verificationSummary"] = verification.get("summary")
        lineage["verificationResultPath"] = verification.get("verificationResultPath")
    if outcome:
        lineage["outcome"] = outcome
    if route:
        binding["route"] = route

    request = update_request_status(index, request_id, status, reason=reason, extra={
        "activeTaskId": task_id,
        "sessionId": lineage.get("sessionId"),
        "worktreePath": lineage.get("worktreePath"),
        "diffSummary": lineage.get("diffSummary"),
        "verificationStatus": lineage.get("verificationStatus"),
        "verificationResultPath": lineage.get("verificationResultPath"),
        "outcome": lineage.get("outcome"),
    })
    sync_request_from_bindings(
        request,
        [item for item in task_map.get("bindings", []) if item.get("requestId") == request_id],
    )
    index["generatedAt"] = timestamp
    index["generator"] = generator
    task_map["generatedAt"] = timestamp
    task_map["generator"] = generator
    write_json(files["request_index_path"], index)
    write_json(files["request_task_map_path"], task_map)

    lineage_event(
        root,
        f"request.{status}",
        generator,
        request_id=request_id,
        task_id=task_id,
        session_id=lineage.get("sessionId"),
        worktree_path=lineage.get("worktreePath"),
        diff_summary=lineage.get("diffSummary"),
        verification=verification,
        outcome=outcome,
        detail=reason,
        context={"route": route} if route else {},
    )
    return binding


def request_bindings_for_task(task_map: dict, task_id: str):
    return [binding for binding in task_map.get("bindings", []) if binding.get("taskId") == task_id]


def request_bindings_for_request(task_map: dict, request_id: str):
    return [binding for binding in task_map.get("bindings", []) if binding.get("requestId") == request_id]


def maybe_complete_request(root: Path, request_id: str, *, generator: str):
    files = ensure_runtime_scaffold(root, generator=generator)
    index = load_json(files["request_index_path"])
    task_map = load_json(files["request_task_map_path"])
    task_pool = load_optional_json(files["harness"] / "task-pool.json", {}) or {}
    bindings = request_bindings_for_request(task_map, request_id)
    if not bindings:
        return False
    statuses = {binding.get("status") for binding in bindings}
    if statuses == {"completed"}:
        return False
    task_by_id = {
        item.get("taskId"): item
        for item in task_pool.get("tasks", [])
        if item.get("taskId")
    }
    merge_pending = False
    for binding in bindings:
        task = task_by_id.get(binding.get("taskId"))
        if not task or not task_merge_required(task):
            continue
        if task.get("mergeStatus") != "merged":
            merge_pending = True
            break
    if merge_pending:
        return False
    if statuses and statuses <= {"verified", "merged", "completed"}:
        latest = bindings[-1]
        update_binding_state(
            root,
            request_id,
            latest.get("taskId"),
            "completed",
            reason="all bound tasks verified; request loop closed",
            generator=generator,
            session_id=latest.get("lineage", {}).get("sessionId"),
            worktree_path=latest.get("lineage", {}).get("worktreePath"),
            diff_summary=latest.get("lineage", {}).get("diffSummary"),
            verification={
                "overallStatus": latest.get("lineage", {}).get("verificationStatus"),
                "summary": latest.get("lineage", {}).get("verificationSummary"),
                "verificationResultPath": latest.get("lineage", {}).get("verificationResultPath"),
            },
            outcome={"status": "completed"},
        )
        return True
    return False


def record_route_decision(root: Path, task_id: str, decision: dict, *, generator: str):
    files = ensure_runtime_scaffold(root, generator=generator)
    session_registry = load_json(files["session_registry_path"])
    decisions = [item for item in session_registry.get("routingDecisions", []) if item.get("taskId") != task_id]
    decisions.append({**decision, "routedAt": now_iso()})
    session_registry["routingDecisions"] = decisions[-50:]
    session_registry["generatedAt"] = now_iso()
    session_registry["generator"] = generator
    write_json(files["session_registry_path"], session_registry)

    task_map = load_json(files["request_task_map_path"])
    for binding in request_bindings_for_task(task_map, task_id):
        update_binding_state(
            root,
            binding.get("requestId"),
            task_id,
            "blocked" if decision.get("gateStatus") == "blocked" else binding.get("status"),
            reason=decision.get("gateReason") or "route decision recorded",
            generator=generator,
            session_id=decision.get("preferredResumeSessionId"),
            route=decision,
        )

    lineage_event(
        root,
        "route.decided",
        generator,
        task_id=task_id,
        session_id=decision.get("preferredResumeSessionId"),
        worktree_path=decision.get("worktreePath"),
        detail=decision.get("gateReason"),
        context={"decision": decision},
    )


def update_session_binding(
    root: Path,
    *,
    task_id: str,
    session_id: str | None,
    node_id: str | None,
    status: str,
    bound_from_task_id: str | None = None,
    error: str | None = None,
    worktree_path: str | None = None,
    branch_name: str | None = None,
    base_ref: str | None = None,
    integration_branch: str | None = None,
    generator: str,
):
    files = ensure_runtime_scaffold(root, generator=generator)
    registry = load_json(files["session_registry_path"])
    active = [
        item for item in registry.get("activeBindings", [])
        if item.get("taskId") != task_id and item.get("sessionId") != session_id
    ]
    recoverable = [item for item in registry.get("recoverableBindings", []) if item.get("taskId") != task_id]
    timestamp = now_iso()

    if status in {"dispatched", "running", "resumed"} and session_id:
        active.append({
            "taskId": task_id,
            "sessionId": session_id,
            "nodeId": node_id,
            "boundFromTaskId": bound_from_task_id,
            "worktreePath": worktree_path,
            "branchName": branch_name,
            "baseRef": base_ref,
            "integrationBranch": integration_branch,
            "updatedAt": timestamp,
        })
    elif status == "recoverable" and session_id:
        recoverable.append({
            "taskId": task_id,
            "sessionId": session_id,
            "lastKnownSessionId": session_id,
            "error": error,
            "worktreePath": worktree_path,
            "branchName": branch_name,
            "baseRef": base_ref,
            "integrationBranch": integration_branch,
            "recordedAt": timestamp,
        })
    elif status == "completed" and session_id:
        registry.setdefault("lastCompletedByTask", {})[task_id] = session_id

    registry["activeBindings"] = active
    registry["recoverableBindings"] = recoverable
    registry["generatedAt"] = timestamp
    registry["generator"] = generator
    write_json(files["session_registry_path"], registry)
    return registry


def emit_follow_up_request(
    root: Path,
    *,
    kind: str,
    goal: str,
    source: str,
    generator: str,
    parent_request_id: str | None = None,
    origin_task_id: str | None = None,
    origin_session_id: str | None = None,
    reason: str | None = None,
    dedupe_key: str | None = None,
    thread_key: str | None = None,
    target_plan_epoch: int | None = None,
    idempotency_key: str | None = None,
):
    files = ensure_runtime_scaffold(root, generator=generator)
    index = load_json(files["request_index_path"])
    ensure_request_index_shape(index)
    task_map = load_json(files["request_task_map_path"])
    thread_state = load_optional_json(files["thread_state_path"], index) or index
    existing = next(
        (
            request for request in index.get("requests", [])
            if request.get("dedupeKey") == dedupe_key and request.get("status") not in REQUEST_TERMINAL_STATUSES
        ),
        None,
    )
    if existing is not None:
        return existing

    seq = int(index.get("nextSeq", 1))
    request_id = build_request_id(seq)
    request = {
        "requestId": request_id,
        "seq": seq,
        "source": source,
        "kind": kind,
        "goal": goal,
        "projectRoot": str(root.resolve()),
        "contextPaths": [],
        "threadKey": thread_key,
        "priority": "P0" if kind in {"replan", "stop"} else "P1",
        "scope": "project",
        "mergePolicy": "append",
        "replyPolicy": "summary",
        "status": "queued",
        "createdAt": now_iso(),
        "parentRequestId": parent_request_id,
        "originTaskId": origin_task_id,
        "originSessionId": origin_session_id,
        "statusReason": reason,
        "dedupeKey": dedupe_key,
        "targetPlanEpoch": target_plan_epoch,
        "idempotencyKey": idempotency_key,
        "submittedKindHint": kind,
    }
    request = normalize_request_record(request, index=index, task_map=task_map, thread_state=thread_state, generator=generator)
    append_jsonl(files["queue_path"], request)
    index["nextSeq"] = seq + 1
    index["generatedAt"] = now_iso()
    index["generator"] = generator
    index.setdefault("requests", []).append(request)
    record_request_thread_state(index, request, generator=generator)
    write_json(files["request_index_path"], index)
    update_request_snapshot(files, request, generator=generator)

    lineage_event(
        root,
        "request.follow_up_emitted",
        generator,
        request_id=request_id,
        task_id=origin_task_id,
        session_id=origin_session_id,
        detail=goal,
        reason=reason,
        context={
            "parentRequestId": parent_request_id,
            "kind": kind,
        },
    )
    return request


def read_verification_report(root: Path, verification_result_path: str | None):
    if not verification_result_path:
        return None
    path = (root / verification_result_path).resolve()
    if not path.exists():
        return None
    return load_json(path)


def recent_feedback_for_task(entries: list[dict], task_id: str | None, *, limit: int = 5):
    if not task_id:
        return []
    related = [entry for entry in entries if entry.get("taskId") == task_id]
    related.sort(key=lambda item: (item.get("timestamp") or "", item.get("id") or ""))
    return related[-limit:]


def binding_for_request(index: dict, task_map: dict, request_id: str | None):
    if not request_id:
        return None
    for binding in reversed(task_map.get("bindings", [])):
        if binding.get("requestId") == request_id:
            return binding
    request = next((item for item in index.get("requests", []) if item.get("requestId") == request_id), None)
    if not request:
        return None
    task_id = request.get("activeTaskId")
    if not task_id:
        return None
    return find_binding(task_map.get("bindings", []), request_id, task_id)


def correlate_root_cause_request(
    root: Path,
    request: dict,
    *,
    index: dict,
    task_map: dict,
    task_pool: dict | None,
    session_registry: dict | None,
    feedback_entries: list[dict],
):
    tasks = (task_pool or {}).get("tasks", [])
    tasks_by_id = {task.get("taskId"): task for task in tasks if task.get("taskId")}
    text = " ".join(filter(None, [request.get("goal"), request.get("statusReason") or ""]))
    explicit_task_ids = explicit_ids_from_text(text, r"\bT-\d+\b")
    explicit_request_ids = explicit_ids_from_text(text, r"\bR-\d+\b")
    explicit_session_ids = explicit_ids_from_text(text, r"\b(?:sess|orch)[A-Za-z0-9:_-]*\b")

    task_id = next((task_id for task_id in explicit_task_ids if task_id in tasks_by_id), None)
    correlated_request_id = None
    confidence = 0.35
    evidence_refs = []

    if task_id:
        confidence = 0.95
        evidence_refs.append(f"task:{task_id}")

    binding = None
    if explicit_request_ids:
        for candidate_request_id in explicit_request_ids:
            binding = binding_for_request(index, task_map, candidate_request_id)
            if binding is not None:
                correlated_request_id = candidate_request_id
                task_id = task_id or binding.get("taskId")
                confidence = max(confidence, 0.9)
                evidence_refs.append(f"request:{candidate_request_id}")
                break

    if binding is None and task_id:
        binding = next(
            (
                item for item in reversed(task_map.get("bindings", []))
                if item.get("taskId") == task_id
            ),
            None,
        )
        if binding is not None:
            correlated_request_id = binding.get("requestId")

    if task_id is None:
        severe_feedback = [
            entry for entry in feedback_entries
            if entry.get("severity") in {"error", "critical"} and entry.get("taskId")
        ]
        unique_task_ids = list(dict.fromkeys(entry.get("taskId") for entry in severe_feedback))
        if len(unique_task_ids) == 1:
            task_id = unique_task_ids[0]
            binding = next(
                (
                    item for item in reversed(task_map.get("bindings", []))
                    if item.get("taskId") == task_id
                ),
                None,
            )
            correlated_request_id = binding.get("requestId") if binding else None
            confidence = max(confidence, 0.62)
            evidence_refs.append(f"feedback-task:{task_id}")

    task = tasks_by_id.get(task_id) if task_id else None
    lineage = (binding or {}).get("lineage", {})
    feedback_refs = recent_feedback_for_task(feedback_entries, task_id)
    symptom_feedback_ids = [entry.get("id") for entry in feedback_refs if entry.get("id")]
    evidence_refs.extend(f"feedback:{item}" for item in symptom_feedback_ids)

    session_id = (
        next((item for item in explicit_session_ids if item), None)
        or lineage.get("sessionId")
        or (task or {}).get("claim", {}).get("boundSessionId")
        or (task or {}).get("preferredResumeSessionId")
    )
    if session_id:
        evidence_refs.append(f"session:{session_id}")

    verification_result_path = (
        lineage.get("verificationResultPath")
        or (task or {}).get("verificationResultPath")
    )
    if verification_result_path:
        evidence_refs.append(f"verification:{verification_result_path}")

    return {
        "correlatedRequestId": correlated_request_id,
        "taskId": task_id,
        "sessionId": session_id,
        "worktreePath": lineage.get("worktreePath") or (task or {}).get("worktreePath"),
        "verificationResultPath": verification_result_path,
        "diffBase": lineage.get("diffBase") or (task or {}).get("diffBase") or (task or {}).get("dispatch", {}).get("diffBase"),
        "bindingId": (binding or {}).get("bindingId"),
        "ownedPaths": (task or {}).get("ownedPaths", []),
        "forbiddenPaths": (task or {}).get("forbiddenPaths", []),
        "taskKind": (task or {}).get("kind"),
        "taskStatus": (task or {}).get("status"),
        "taskTitle": (task or {}).get("title"),
        "auditVerdict": (task or {}).get("auditVerdict"),
        "verificationStatus": lineage.get("verificationStatus") or (task or {}).get("verificationStatus"),
        "symptomFeedbackIds": symptom_feedback_ids,
        "evidenceRefs": evidence_refs,
        "correlationConfidence": round(confidence, 2),
        "correlationStrength": "strong" if confidence >= 0.85 else "moderate" if confidence >= 0.6 else "weak",
    }


def classify_root_cause_dimension(request: dict, correlation: dict, verification_report: dict | None, symptom_entries: list[dict]):
    text = " ".join(
        filter(
            None,
            [
                request.get("goal"),
                request.get("statusReason"),
                " ".join(entry.get("message") or "" for entry in symptom_entries),
            ],
        )
    ).lower()
    explicit_dimension = next((dimension for dimension in RCA_DIMENSIONS if dimension in text), None)
    if explicit_dimension:
        return explicit_dimension
    if correlation.get("correlationStrength") == "weak" or not correlation.get("taskId"):
        return "underdetermined"
    feedback_types = {entry.get("feedbackType") for entry in symptom_entries}
    source_values = {entry.get("source") for entry in symptom_entries}
    steps = {entry.get("step") for entry in symptom_entries}
    audit_verdict = correlation.get("auditVerdict")
    verification_status = (verification_report or {}).get("overallStatus") or correlation.get("verificationStatus")

    if "merge" in text or audit_verdict in {"warn", "fail"}:
        return "merge_handoff"
    if verification_status == "fail" or feedback_types & {"verification_failure", "missing_rule", "test_failure"}:
        return "verification_guardrail"
    if feedback_types & {"path_conflict", "session_conflict", "replan_required"}:
        return "routing_session"
    if (
        "ownedpaths" in text
        or "forbiddenpath" in text
        or "forbidden path" in text
        or "session reuse" in text
        or steps & {"route", "resume"}
    ):
        return "routing_session"
    if "acceptance" in text or "prd" in text or "spec" in text or "expected behavior" in text:
        return "spec_acceptance"
    if "blueprint" in text or "decomposition" in text or "task boundary" in text or "work item" in text:
        return "blueprint_decomposition"
    if (
        "dependency" in text
        or "network" in text
        or "module not found" in text
        or "command not found" in text
        or source_values & {"operator", "environment", "dependency"}
    ):
        return "environment_dependency"
    if (
        "harness" in text
        or "runner" in text
        or "tmux" in text
        or "install" in text
        or source_values & {"runtime", "runner", "session-init"}
    ):
        return "runtime_tooling"
    return "execution_change"


def contributing_dimensions(primary_dimension: str, correlation: dict, verification_report: dict | None, symptom_entries: list[dict]):
    contributing = []
    text = " ".join(entry.get("message") or "" for entry in symptom_entries).lower()
    if primary_dimension != "verification_guardrail" and ((verification_report or {}).get("overallStatus") == "fail"):
        contributing.append("verification_guardrail")
    if primary_dimension != "routing_session" and ("path conflict" in text or "session conflict" in text):
        contributing.append("routing_session")
    if primary_dimension != "execution_change" and correlation.get("taskId"):
        contributing.append("execution_change")
    return list(dict.fromkeys(item for item in contributing if item in RCA_DIMENSIONS and item != primary_dimension))


def allocate_root_cause_record(
    root: Path,
    request: dict,
    *,
    generator: str,
    correlation: dict,
    feedback_entries: list[dict],
):
    files = ensure_runtime_scaffold(root, generator=generator)
    existing = load_jsonl(files["root_cause_log_path"])
    next_seq = max((int(item.get("rcaId", "RCA-0000").split("-")[-1]) for item in existing if item.get("rcaId")), default=0) + 1
    verification_report = read_verification_report(root, correlation.get("verificationResultPath"))
    symptom_entries = [
        entry for entry in feedback_entries
        if entry.get("id") in set(correlation.get("symptomFeedbackIds", []))
    ]
    primary_dimension = classify_root_cause_dimension(request, correlation, verification_report, symptom_entries)
    repair_mode = CAUSE_REPAIR_MODE.get(primary_dimension, "audit/research")
    status = "underdetermined" if primary_dimension == "underdetermined" else "allocated"
    task_id = correlation.get("taskId")
    prevention_target = CAUSE_PREVENTION_TARGET.get(primary_dimension)
    prevention_action = (
        f"write prevention signal to {prevention_target}"
        if prevention_target
        else "record prevention note in root-cause summary"
    )
    return {
        "schemaVersion": SCHEMA_VERSION,
        "generator": generator,
        "generatedAt": now_iso(),
        "rcaId": build_rca_id(next_seq),
        "bugId": request.get("requestId") if request.get("kind") == "bug" else None,
        "sourceRequestId": request.get("requestId"),
        "requestId": request.get("requestId"),
        "repairRequestId": None,
        "taskId": task_id,
        "sessionId": correlation.get("sessionId"),
        "worktreePath": correlation.get("worktreePath"),
        "verificationResultPath": correlation.get("verificationResultPath"),
        "symptomFeedbackIds": correlation.get("symptomFeedbackIds", []),
        "primaryCauseDimension": primary_dimension,
        "contributingCauseDimensions": contributing_dimensions(primary_dimension, correlation, verification_report, symptom_entries),
        "ownerRole": CAUSE_OWNER_ROLE.get(primary_dimension, "architect/orchestrator"),
        "repairMode": repair_mode,
        "confidence": correlation.get("correlationConfidence", 0.0),
        "status": status,
        "evidenceRefs": list(dict.fromkeys(correlation.get("evidenceRefs", []))),
        "allocatedAt": now_iso(),
        "preventionTarget": prevention_target,
        "preventionAction": prevention_action,
        "correlationStrength": correlation.get("correlationStrength"),
        "correlatedRequestId": correlation.get("correlatedRequestId"),
        "bindingId": correlation.get("bindingId"),
        "diffBase": correlation.get("diffBase"),
    }


def append_root_cause_record(root: Path, record: dict, *, generator: str):
    files = ensure_runtime_scaffold(root, generator=generator)
    payload = {**record, "generatedAt": now_iso(), "generator": generator}
    append_jsonl(files["root_cause_log_path"], payload)
    lineage_event(
        root,
        "rca.recorded",
        generator,
        request_id=payload.get("sourceRequestId"),
        task_id=payload.get("taskId"),
        session_id=payload.get("sessionId"),
        worktree_path=payload.get("worktreePath"),
        detail=payload.get("primaryCauseDimension"),
        reason=payload.get("status"),
        context={"rcaId": payload.get("rcaId"), "ownerRole": payload.get("ownerRole"), "repairMode": payload.get("repairMode")},
    )
    return payload


def latest_root_cause_records(entries: list[dict]):
    latest = {}
    for entry in entries:
        rca_id = entry.get("rcaId")
        if rca_id:
            latest[rca_id] = entry
    return latest


def build_root_cause_summary(entries: list[dict]):
    latest = latest_root_cause_records(entries)
    records = sorted(
        latest.values(),
        key=lambda item: (item.get("allocatedAt") or "", item.get("generatedAt") or "", item.get("rcaId") or ""),
    )
    by_dimension = Counter(record.get("primaryCauseDimension", "underdetermined") for record in records)
    by_owner = Counter(record.get("ownerRole", "unknown") for record in records)
    open_records = [record for record in records if record.get("status") not in RCA_TERMINAL_STATUSES]
    recurring = [
        {"primaryCauseDimension": dimension, "count": count}
        for dimension, count in sorted(by_dimension.items(), key=lambda item: (-item[1], item[0]))
        if count > 1
    ]
    missing_correlation = [
        {
            "rcaId": record.get("rcaId"),
            "bugId": record.get("bugId"),
            "sourceRequestId": record.get("sourceRequestId"),
            "taskId": record.get("taskId"),
            "confidence": record.get("confidence"),
            "status": record.get("status"),
        }
        for record in open_records
        if record.get("primaryCauseDimension") == "underdetermined" or record.get("confidence", 0) < 0.6
    ]
    return {
        "schemaVersion": SCHEMA_VERSION,
        "generator": "klein-harness",
        "generatedAt": now_iso(),
        "rootCauseLogPath": ".harness/root-cause-log.jsonl",
        "rcaCount": len(records),
        "openCount": len(open_records),
        "underdeterminedCount": sum(1 for record in records if record.get("primaryCauseDimension") == "underdetermined"),
        "byPrimaryCauseDimension": dict(by_dimension),
        "byOwnerRole": dict(by_owner),
        "openItems": [
            {
                "rcaId": record.get("rcaId"),
                "sourceRequestId": record.get("sourceRequestId"),
                "repairRequestId": record.get("repairRequestId"),
                "taskId": record.get("taskId"),
                "primaryCauseDimension": record.get("primaryCauseDimension"),
                "ownerRole": record.get("ownerRole"),
                "repairMode": record.get("repairMode"),
                "status": record.get("status"),
                "confidence": record.get("confidence"),
                "preventionAction": record.get("preventionAction"),
            }
            for record in open_records[-10:]
        ],
        "recurringRootCauses": recurring,
        "bugsMissingLineageCorrelation": missing_correlation[-10:],
        "recentAllocations": [
            {
                "rcaId": record.get("rcaId"),
                "primaryCauseDimension": record.get("primaryCauseDimension"),
                "ownerRole": record.get("ownerRole"),
                "status": record.get("status"),
                "allocatedAt": record.get("allocatedAt"),
            }
            for record in records[-10:]
        ],
    }


def write_root_cause_summary(root: Path, *, generator: str):
    files = ensure_runtime_scaffold(root, generator=generator)
    summary = build_root_cause_summary(load_jsonl(files["root_cause_log_path"]))
    summary["generator"] = generator
    summary["generatedAt"] = now_iso()
    write_json(files["root_cause_summary_path"], summary)
    return summary


def emit_repair_request_for_rca(root: Path, record: dict, *, generator: str):
    task_label = record.get("taskId") or "unbound lineage"
    kind = "analysis"
    goal = f"Research RCA {record['rcaId']} because lineage correlation is underdetermined"
    reason = f"RCA {record['rcaId']} allocated to {record['primaryCauseDimension']}"
    repair_mode = record.get("repairMode")
    thread_key = None
    if repair_mode == "replan/clarification":
        kind = "replan"
        goal = f"Clarify acceptance and replan {task_label} for RCA {record['rcaId']}"
    elif repair_mode == "replan":
        kind = "replan"
        goal = f"Replan blueprint/task decomposition for {task_label} from RCA {record['rcaId']}"
    elif repair_mode == "stop/route-fix":
        kind = "stop"
        goal = f"Stop conflicting route and apply route fix for {task_label} from RCA {record['rcaId']}"
    elif repair_mode in {"bugfix", "test-fix", "harness-fix", "audit/merge-fix"}:
        kind = "implementation"
        goal = f"Repair {task_label} via {repair_mode} from RCA {record['rcaId']}"
        thread_key = f"thread:{stable_short_hash('rca-repair', record.get('rcaId') or '', task_label)[:12]}"
    elif repair_mode == "ops-fix":
        kind = "research"
        goal = f"Investigate external dependency and ops fix for RCA {record['rcaId']}"
    elif repair_mode == "audit/research":
        kind = "audit"
        goal = f"Audit and gather evidence for RCA {record['rcaId']} before repair"

    return emit_follow_up_request(
        root,
        kind=kind,
        goal=goal,
        source="runtime:rca",
        generator=generator,
        parent_request_id=record.get("sourceRequestId"),
        origin_task_id=record.get("taskId"),
        origin_session_id=record.get("sessionId"),
        reason=reason,
        dedupe_key=f"rca:{record['rcaId']}:{repair_mode}:{kind}",
        thread_key=thread_key,
    )


def process_root_cause_request(root: Path, request: dict, *, generator: str):
    files = ensure_runtime_scaffold(root, generator=generator)
    index = load_json(files["request_index_path"])
    task_map = load_json(files["request_task_map_path"])
    task_pool = read_task_pool(files["harness"])
    session_registry = load_optional_json(files["session_registry_path"], {})
    feedback_entries = load_jsonl(files["feedback_log_path"])

    correlation = correlate_root_cause_request(
        root,
        request,
        index=index,
        task_map=task_map,
        task_pool=task_pool,
        session_registry=session_registry,
        feedback_entries=feedback_entries,
    )
    if correlation.get("taskId") and find_binding(task_map.get("bindings", []), request.get("requestId"), correlation.get("taskId")) is None:
        task = find_task((task_pool or {}).get("tasks", []), correlation.get("taskId"))
        ensure_binding(
            root,
            request.get("requestId"),
            task,
            status="bound",
            reason=f"RCA correlated to {correlation.get('taskId')}",
            generator=generator,
        )

    record = allocate_root_cause_record(root, request, generator=generator, correlation=correlation, feedback_entries=feedback_entries)
    follow_up = emit_repair_request_for_rca(root, record, generator=generator)
    if follow_up:
        record["repairRequestId"] = follow_up.get("requestId")
        if record.get("status") != "underdetermined":
            record["status"] = "repair_emitted"
    saved = append_root_cause_record(root, record, generator=generator)

    request_status = "blocked" if saved.get("status") == "underdetermined" else "completed"
    status_reason = (
        f"RCA {saved['rcaId']} underdetermined; emitted {follow_up.get('kind') if follow_up else 'research'} follow-up"
        if saved.get("status") == "underdetermined"
        else f"RCA {saved['rcaId']} allocated to {saved['primaryCauseDimension']} and emitted repair request"
    )
    if correlation.get("taskId"):
        update_binding_state(
            root,
            request.get("requestId"),
            correlation.get("taskId"),
            request_status,
            reason=status_reason,
            generator=generator,
            session_id=correlation.get("sessionId"),
            worktree_path=correlation.get("worktreePath"),
            diff_summary=saved.get("primaryCauseDimension"),
            outcome={
                "status": "repair_request_emitted" if request_status == "completed" else "underdetermined",
                "repairRequestId": saved.get("repairRequestId"),
                "rcaId": saved.get("rcaId"),
            },
        )
        refreshed_index = load_json(files["request_index_path"])
        update_request_status(
            refreshed_index,
            request.get("requestId"),
            request_status,
            reason=status_reason,
            extra={
                "rcaId": saved.get("rcaId"),
                "repairRequestId": saved.get("repairRequestId"),
                "primaryCauseDimension": saved.get("primaryCauseDimension"),
                "ownerRole": saved.get("ownerRole"),
                "repairMode": saved.get("repairMode"),
            },
        )
        refreshed_index["generatedAt"] = now_iso()
        refreshed_index["generator"] = generator
        write_json(files["request_index_path"], refreshed_index)
    else:
        latest_request = update_request_status(
            index,
            request.get("requestId"),
            request_status,
            reason=status_reason,
            extra={
                "rcaId": saved.get("rcaId"),
                "repairRequestId": saved.get("repairRequestId"),
                "primaryCauseDimension": saved.get("primaryCauseDimension"),
                "ownerRole": saved.get("ownerRole"),
                "repairMode": saved.get("repairMode"),
            },
        )
        index["generatedAt"] = now_iso()
        index["generator"] = generator
        write_json(files["request_index_path"], index)
        update_request_snapshot(files, latest_request, generator=generator)
        write_json(files["request_summary_path"], build_request_summary(index, load_json(files["request_task_map_path"]), task_pool))
        lineage_event(
            root,
            "request.completed" if request_status == "completed" else "request.blocked",
            generator,
            request_id=request.get("requestId"),
            detail=status_reason,
            reason=saved.get("primaryCauseDimension"),
            context={"rcaId": saved.get("rcaId"), "repairRequestId": saved.get("repairRequestId")},
        )
        latest_request["updatedAt"] = now_iso()

    refreshed_index = load_json(files["request_index_path"])
    latest_request = next(
        (item for item in refreshed_index.get("requests", []) if item.get("requestId") == request.get("requestId")),
        None,
    )
    if latest_request is not None:
        update_request_snapshot(files, latest_request, generator=generator)
    summary = write_root_cause_summary(root, generator=generator)
    return {
        "requestId": request.get("requestId"),
        "rcaId": saved.get("rcaId"),
        "taskId": correlation.get("taskId"),
        "primaryCauseDimension": saved.get("primaryCauseDimension"),
        "ownerRole": saved.get("ownerRole"),
        "repairMode": saved.get("repairMode"),
        "repairRequestId": saved.get("repairRequestId"),
        "status": saved.get("status"),
        "correlationStrength": saved.get("correlationStrength"),
        "summaryOpenCount": summary.get("openCount"),
    }


def close_request_without_binding(root: Path, request_id: str, *, status: str, reason: str, generator: str, extra: dict | None = None):
    files = ensure_runtime_scaffold(root, generator=generator)
    index = load_json(files["request_index_path"])
    request = update_request_status(index, request_id, status, reason=reason, extra=extra)
    index["generatedAt"] = now_iso()
    index["generator"] = generator
    write_json(files["request_index_path"], index)
    update_request_snapshot(files, request, generator=generator)
    lineage_event(
        root,
        f"request.{status}",
        generator,
        request_id=request_id,
        detail=reason,
        context={key: value for key, value in (extra or {}).items() if value is not None},
    )
    return request


def close_root_causes_for_repair_request(
    root: Path,
    *,
    request_id: str,
    task: dict,
    verification_result_path: str | None,
    generator: str,
):
    files = ensure_runtime_scaffold(root, generator=generator)
    entries = load_jsonl(files["root_cause_log_path"])
    latest = latest_root_cause_records(entries)
    updated = []
    for record in latest.values():
        if record.get("repairRequestId") != request_id:
            continue
        if record.get("status") in RCA_TERMINAL_STATUSES:
            continue
        next_record = {
            **record,
            "generatedAt": now_iso(),
            "generator": generator,
            "status": "repaired",
            "taskId": record.get("taskId") or task.get("taskId"),
            "worktreePath": record.get("worktreePath") or task.get("worktreePath"),
            "verificationResultPath": verification_result_path or record.get("verificationResultPath"),
            "preventionAction": record.get("preventionAction") or f"record prevention note for {task.get('taskId')}",
        }
        append_jsonl(files["root_cause_log_path"], next_record)
        updated.append(next_record)
        lineage_event(
            root,
            "rca.repaired",
            generator,
            request_id=record.get("sourceRequestId"),
            task_id=task.get("taskId"),
            session_id=record.get("sessionId"),
            worktree_path=task.get("worktreePath"),
            verification={"verificationResultPath": verification_result_path, "overallStatus": "pass"},
            detail=record.get("rcaId"),
            reason=record.get("preventionAction"),
        )
    if updated:
        write_root_cause_summary(root, generator=generator)
    return updated


def reconcile_requests(root: Path, *, generator: str = "harness-reconcile"):
    files = ensure_runtime_scaffold(root, generator=generator)
    index = load_json(files["request_index_path"])
    task_map = load_json(files["request_task_map_path"])
    task_pool = read_task_pool(files["harness"])
    tasks = task_pool.get("tasks", []) if task_pool else []
    thread_state = load_optional_json(files["thread_state_path"], {})
    bound = []
    blocked = []
    internalized = []

    normalized_requests = []
    for request in index.get("requests", []):
        normalized_requests.append(
            normalize_request_record(
                request,
                index=index,
                task_map=task_map,
                thread_state=thread_state,
                generator=generator,
            )
        )
    index["requests"] = normalized_requests
    index["generatedAt"] = now_iso()
    index["generator"] = generator
    write_json(files["request_index_path"], index)

    for request in index.get("requests", []):
        if request.get("status") in REQUEST_TERMINAL_STATUSES:
            continue
        impact_classification, impacted_inflight = infer_request_impact_class(request, tasks)
        request["impactClassification"] = impact_classification
        request["impactedInflightTasks"] = impacted_inflight
        if (request.get("kind") or "").lower() in {"bug", "feedback", "rca"}:
            if request.get("primaryCauseDimension") or request.get("rcaId"):
                continue
            outcome = process_root_cause_request(root, request, generator=generator)
            bound.append({
                "requestId": request.get("requestId"),
                "taskId": outcome.get("taskId"),
                "rcaId": outcome.get("rcaId"),
                "kind": request.get("kind"),
            })
            continue
        if request.get("fusionDecision") in {"duplicate_of_existing", "noop"}:
            closed = close_request_without_binding(
                root,
                request.get("requestId"),
                status="completed",
                reason=f"{request.get('fusionDecision')} via deterministic intake fusion",
                generator=generator,
                extra={
                    "duplicateOfRequestId": request.get("duplicateOfRequestId"),
                    "classificationReason": request.get("classificationReason"),
                    "impactClassification": impact_classification,
                },
            )
            internalized.append({"requestId": closed.get("requestId"), "mode": request.get("fusionDecision")})
            continue
        if request.get("fusionDecision") == "merged_as_context":
            closed = close_request_without_binding(
                root,
                request.get("requestId"),
                status="completed",
                reason="merged as context enrichment into existing thread",
                generator=generator,
                extra={
                    "mergedIntoRequestId": request.get("mergedIntoRequestId"),
                    "classificationReason": request.get("classificationReason"),
                    "impactClassification": impact_classification,
                },
            )
            internalized.append({"requestId": closed.get("requestId"), "mode": "merged_as_context"})
            continue
        if request_bindings_for_request(task_map, request.get("requestId")):
            continue
        if impact_classification == "supersede_queued":
            request["supersededQueuedTaskIds"] = supersede_queued_thread_tasks(root, request, generator=generator)
        elif impact_classification == "checkpoint_then_replan":
            request["checkpointTaskIds"] = checkpoint_active_thread_tasks(root, request, generator=generator)

        if request.get("normalizedIntentClass") == "inspection":
            follow_up = emit_follow_up_request(
                root,
                kind="audit",
                goal=f"Inspection overlay for {request.get('targetThreadKey') or request.get('threadKey')}: {request.get('goal')}",
                source="runtime:intake",
                generator=generator,
                parent_request_id=request.get("requestId"),
                reason=request.get("classificationReason"),
                dedupe_key=f"inspection:{request.get('targetThreadKey') or request.get('threadKey')}:{request.get('canonicalGoalHash')}",
                thread_key=request.get("targetThreadKey") or request.get("threadKey"),
                target_plan_epoch=request.get("targetPlanEpoch"),
                idempotency_key=f"{request.get('idempotencyKey')}:inspection" if request.get("idempotencyKey") else None,
            )
            closed = close_request_without_binding(
                root,
                request.get("requestId"),
                status="completed",
                reason="inspection overlay normalized into internal audit request",
                generator=generator,
                extra={
                    "inspectionRequestId": follow_up.get("requestId"),
                    "impactClassification": impact_classification,
                },
            )
            internalized.append({"requestId": closed.get("requestId"), "mode": "inspection_overlay", "followUpRequestId": follow_up.get("requestId")})
            continue
        if request.get("normalizedIntentClass") == "append_change" and request.get("mergedIntoRequestId"):
            follow_up = emit_follow_up_request(
                root,
                kind="replan",
                goal=f"Selective replan for {request.get('targetThreadKey')}: {request.get('goal')}",
                source="runtime:intake",
                generator=generator,
                parent_request_id=request.get("requestId"),
                reason=request.get("classificationReason"),
                dedupe_key=f"append-replan:{request.get('targetThreadKey')}:{request.get('targetPlanEpoch')}",
                thread_key=request.get("targetThreadKey") or request.get("threadKey"),
                target_plan_epoch=request.get("targetPlanEpoch"),
                idempotency_key=f"{request.get('idempotencyKey')}:replan" if request.get("idempotencyKey") else None,
            )
            closed = close_request_without_binding(
                root,
                request.get("requestId"),
                status="completed",
                reason="append_change normalized into internal selective replan request",
                generator=generator,
                extra={
                    "replanRequestId": follow_up.get("requestId"),
                    "impactClassification": impact_classification,
                    "supersededQueuedTaskIds": request.get("supersededQueuedTaskIds"),
                    "checkpointTaskIds": request.get("checkpointTaskIds"),
                },
            )
            internalized.append({"requestId": closed.get("requestId"), "mode": "append_requires_replan", "followUpRequestId": follow_up.get("requestId")})
            continue
        if request.get("normalizedIntentClass") == "compound_split":
            follow_up_ids = []
            if "inspection" in (request.get("internalIntents") or []):
                follow_up = emit_follow_up_request(
                    root,
                    kind="audit",
                    goal=f"Inspection slice for compound request {request.get('requestId')}: {request.get('goal')}",
                    source="runtime:intake",
                    generator=generator,
                    parent_request_id=request.get("requestId"),
                    reason="compound submission inspection split",
                    dedupe_key=f"compound-inspection:{request.get('requestId')}:{request.get('canonicalGoalHash')}",
                    thread_key=request.get("targetThreadKey") or request.get("threadKey"),
                    target_plan_epoch=request.get("targetPlanEpoch"),
                )
                follow_up_ids.append(follow_up.get("requestId"))
            if "append_change" in (request.get("internalIntents") or []) and request.get("mergedIntoRequestId"):
                follow_up = emit_follow_up_request(
                    root,
                    kind="replan",
                    goal=f"Append-change slice for compound request {request.get('requestId')}: {request.get('goal')}",
                    source="runtime:intake",
                    generator=generator,
                    parent_request_id=request.get("requestId"),
                    reason="compound submission append split",
                    dedupe_key=f"compound-replan:{request.get('targetThreadKey')}:{request.get('targetPlanEpoch')}:{request.get('requestId')}",
                    thread_key=request.get("targetThreadKey") or request.get("threadKey"),
                    target_plan_epoch=request.get("targetPlanEpoch"),
                )
                follow_up_ids.append(follow_up.get("requestId"))
            elif any(intent in {"append_change", "fresh_work"} for intent in (request.get("internalIntents") or [])):
                follow_up = emit_follow_up_request(
                    root,
                    kind=request.get("kind") or "implementation",
                    goal=f"Execution slice for compound request {request.get('requestId')}: {request.get('goal')}",
                    source="runtime:intake",
                    generator=generator,
                    parent_request_id=request.get("requestId"),
                    reason="compound submission execution split",
                    dedupe_key=f"compound-exec:{request.get('canonicalGoalHash')}:{request.get('requestId')}",
                    thread_key=request.get("targetThreadKey") or request.get("threadKey"),
                    target_plan_epoch=request.get("targetPlanEpoch"),
                )
                follow_up_ids.append(follow_up.get("requestId"))
            closed = close_request_without_binding(
                root,
                request.get("requestId"),
                status="completed",
                reason="compound submission normalized into internal follow-up requests",
                generator=generator,
                extra={
                    "internalFollowUpRequestIds": follow_up_ids,
                    "impactClassification": impact_classification,
                },
            )
            internalized.append({"requestId": closed.get("requestId"), "mode": "compound_split_created", "followUpRequestIds": follow_up_ids})
            continue
        candidates = candidate_tasks_for_request(request, tasks)
        if not candidates:
            update_request_status(
                index,
                request.get("requestId"),
                "blocked",
                reason="no compatible task available for current runtime state",
            )
            blocked.append(request.get("requestId"))
            lineage_event(
                root,
                "request.blocked",
                generator,
                request_id=request.get("requestId"),
                detail="no compatible task available for current runtime state",
            )
            continue
        for task in candidates:
            ensure_task_thread_metadata(task, request)
            ensure_binding(
                root,
                request.get("requestId"),
                task,
                status="bound",
                reason=f"bound by reconcile to {task.get('taskId')}",
                generator=generator,
            )
            bound.append({
                "requestId": request.get("requestId"),
                "taskId": task.get("taskId"),
            })

    index = load_json(files["request_index_path"])
    index["generatedAt"] = now_iso()
    index["generator"] = generator
    write_json(files["request_index_path"], index)
    return {
        "bound": bound,
        "blocked": blocked,
        "internalized": internalized,
        "requestCount": len(index.get("requests", [])),
    }


def build_feedback_summary(entries):
    ordered = sorted(entries, key=lambda item: (item.get("timestamp") or "", item.get("id") or ""))
    errors = [entry for entry in ordered if entry.get("severity") in {"error", "critical"}]
    illegal_actions = [entry for entry in ordered if entry.get("feedbackType") == "illegal_action"]
    task_summary = {}

    def trim(entry):
        return {
            "id": entry.get("id"),
            "taskId": entry.get("taskId"),
            "sessionId": entry.get("sessionId"),
            "role": entry.get("role"),
            "workerMode": entry.get("workerMode"),
            "feedbackType": entry.get("feedbackType"),
            "severity": entry.get("severity"),
            "source": entry.get("source"),
            "step": entry.get("step"),
            "triggeringAction": entry.get("triggeringAction"),
            "message": entry.get("message"),
            "timestamp": entry.get("timestamp"),
        }

    for entry in ordered:
        task_id = entry.get("taskId")
        if not task_id:
            continue
        summary = task_summary.setdefault(task_id, {
            "taskId": task_id,
            "feedbackCount": 0,
            "errorCount": 0,
            "criticalCount": 0,
            "latestFeedbackType": None,
            "latestSeverity": None,
            "latestMessage": None,
            "latestTimestamp": None,
            "recentFailures": [],
        })
        summary["feedbackCount"] += 1
        if entry.get("severity") in {"error", "critical"}:
            summary["errorCount"] += 1
        if entry.get("severity") == "critical":
            summary["criticalCount"] += 1
        summary["latestFeedbackType"] = entry.get("feedbackType")
        summary["latestSeverity"] = entry.get("severity")
        summary["latestMessage"] = entry.get("message")
        summary["latestTimestamp"] = entry.get("timestamp")

    for task_id, summary in task_summary.items():
        task_entries = [
            trim(entry)
            for entry in ordered
            if entry.get("taskId") == task_id and entry.get("severity") in {"error", "critical"}
        ]
        summary["recentFailures"] = task_entries[-3:]

    return {
        "schemaVersion": SCHEMA_VERSION,
        "generator": "klein-harness",
        "generatedAt": now_iso(),
        "feedbackLogPath": ".harness/feedback-log.jsonl",
        "feedbackEventCount": len(ordered),
        "errorCount": len(errors),
        "criticalCount": sum(1 for entry in ordered if entry.get("severity") == "critical"),
        "illegalActionCount": len(illegal_actions),
        "tasksWithRecentFailures": sorted(task_id for task_id, summary in task_summary.items() if summary["recentFailures"]),
        "byType": dict(Counter(entry.get("feedbackType", "unknown") for entry in ordered)),
        "bySeverity": dict(Counter(entry.get("severity", "unknown") for entry in ordered)),
        "recentFailures": [trim(entry) for entry in errors[-5:]],
        "taskFeedbackSummary": task_summary,
    }


def build_request_summary(index: dict, task_map: dict, task_pool: dict | None = None):
    tasks_by_id = {
        task.get("taskId"): task
        for task in (task_pool or {}).get("tasks", [])
        if task.get("taskId")
    }
    requests = index.get("requests", [])
    counts = dict(Counter(request.get("status", "unknown") for request in requests))
    active = [request for request in requests if request.get("status") not in REQUEST_TERMINAL_STATUSES]
    active.sort(
        key=lambda request: (
            {
                "running": 0,
                "resumed": 1,
                "recoverable": 2,
                "dispatched": 3,
                "bound": 4,
                "queued": 5,
                "blocked": 6,
                "verified": 7,
            }.get(request.get("status"), 9),
            request.get("seq", 0),
        )
    )
    active_request = active[0] if active else None
    bindings = task_map.get("bindings", [])
    binding_summary = []
    for binding in bindings[-20:]:
        task = tasks_by_id.get(binding.get("taskId"), {})
        binding_summary.append({
            "bindingId": binding.get("bindingId"),
            "requestId": binding.get("requestId"),
            "requestStatus": next(
                (
                    request.get("status")
                    for request in requests
                    if request.get("requestId") == binding.get("requestId")
                ),
                None,
            ),
            "taskId": binding.get("taskId"),
            "taskTitle": binding.get("taskTitle") or task.get("title"),
            "bindingStatus": binding.get("status"),
            "branchName": task.get("branchName"),
            "worktreePath": binding.get("lineage", {}).get("worktreePath") or task.get("worktreePath"),
            "sessionId": binding.get("lineage", {}).get("sessionId"),
            "diffSummary": binding.get("lineage", {}).get("diffSummary") or task.get("diffSummary"),
            "verificationStatus": binding.get("lineage", {}).get("verificationStatus") or task.get("verificationStatus"),
            "verificationResultPath": binding.get("lineage", {}).get("verificationResultPath") or task.get("verificationResultPath"),
            "updatedAt": binding.get("updatedAt"),
        })

    return {
        "schemaVersion": SCHEMA_VERSION,
        "generator": "klein-harness",
        "generatedAt": now_iso(),
        "requestCounts": counts,
        "totalRequests": len(requests),
        "activeRequest": active_request,
        "recentRequests": requests[-5:],
        "recentSubmissionClassifications": bounded(
            [
                {
                    "requestId": request.get("requestId"),
                    "frontDoorClass": request.get("frontDoorClass"),
                    "normalizedIntentClass": request.get("normalizedIntentClass"),
                    "fusionDecision": request.get("fusionDecision"),
                    "threadKey": thread_key_from_request(request),
                    "targetPlanEpoch": request.get("targetPlanEpoch"),
                    "effectStatus": request.get("effectStatus"),
                    "classificationReason": request.get("classificationReason"),
                }
                for request in requests[-10:]
            ],
            10,
        ),
        "bindings": binding_summary,
        "boundRequestCount": counts.get("bound", 0),
        "runningRequestCount": counts.get("running", 0),
        "recoverableRequestCount": counts.get("recoverable", 0),
        "completedRequestCount": counts.get("completed", 0),
        "blockedRequestCount": counts.get("blocked", 0),
        "duplicateRequestCount": sum(1 for request in requests if request.get("fusionDecision") == "duplicate_of_existing"),
        "contextMergeCount": sum(1 for request in requests if request.get("fusionDecision") == "merged_as_context"),
        "inspectionOverlayCount": sum(1 for request in requests if request.get("fusionDecision") == "inspection_overlay"),
        "appendChangeCount": sum(1 for request in requests if request.get("normalizedIntentClass") == "append_change"),
        "byFrontDoorClass": dict(Counter(request.get("frontDoorClass", "unknown") for request in requests)),
    }


def build_intake_summary(index: dict, thread_state: dict | None = None, *, generator: str, policy_summary: dict) -> dict:
    requests = index.get("requests", [])
    recent_limit = policy_summary["intake"]["maxRecentSubmissions"]
    return {
        "schemaVersion": SCHEMA_VERSION,
        "generator": generator,
        "generatedAt": now_iso(),
        "submissionCount": len(requests),
        "byFrontDoorClass": dict(Counter(request.get("frontDoorClass", "unknown") for request in requests)),
        "byIntentClass": dict(Counter(request.get("normalizedIntentClass", "unknown") for request in requests)),
        "byFusionDecision": dict(Counter(request.get("fusionDecision", "unknown") for request in requests)),
        "duplicateCount": sum(1 for request in requests if request.get("fusionDecision") == "duplicate_of_existing"),
        "contextMergeCount": sum(1 for request in requests if request.get("fusionDecision") == "merged_as_context"),
        "inspectionOverlayCount": sum(1 for request in requests if request.get("fusionDecision") == "inspection_overlay"),
        "compoundSplitCount": sum(1 for request in requests if request.get("fusionDecision") == "compound_split_created"),
        "contextRotWarningCount": len((thread_state or {}).get("contextRotWarnings", [])),
        "recentSubmissions": bounded(
            [
                {
                    "requestId": request.get("requestId"),
                    "goal": request.get("goal"),
                    "status": request.get("status"),
                    "frontDoorClass": request.get("frontDoorClass"),
                    "normalizedIntentClass": request.get("normalizedIntentClass"),
                    "fusionDecision": request.get("fusionDecision"),
                    "threadKey": thread_key_from_request(request),
                    "targetPlanEpoch": request.get("targetPlanEpoch"),
                    "classificationReason": request.get("classificationReason"),
                    "duplicateOfRequestId": request.get("duplicateOfRequestId"),
                    "mergedIntoRequestId": request.get("mergedIntoRequestId"),
                    "impactClassification": request.get("impactClassification"),
                }
                for request in requests[-recent_limit:]
            ],
            recent_limit,
        ),
        "contextRotWarnings": bounded((thread_state or {}).get("contextRotWarnings", []), policy_summary["intake"]["maxRecentThreads"]),
    }


def build_thread_state(index: dict, task_pool: dict | None, request_summary: dict, *, generator: str, policy_summary: dict) -> dict:
    tasks = (task_pool or {}).get("tasks", [])
    thread_ledger = index.get("threads", {}) if isinstance(index.get("threads"), dict) else {}
    threads = {}
    thread_requests = defaultdict(list)
    for request in index.get("requests", []):
        key = thread_key_from_request(request)
        if key:
            thread_requests[key].append(request)
    for thread_key, requests in thread_requests.items():
        requests = sorted(requests, key=lambda item: (item.get("seq", 0), item.get("createdAt") or ""))
        current_epoch = max(int(item.get("targetPlanEpoch") or 1) for item in requests)
        active_requests = [item for item in requests if not request_effect_terminal(item)]
        related_tasks = [task for task in tasks if task.get("threadKey") == thread_key]
        context_rot_warnings = []
        if sum(1 for item in requests if item.get("normalizedIntentClass") == "append_change") >= 2:
            context_rot_warnings.append("multiple appended changes")
        if sum(1 for task in related_tasks if task.get("status") in TASK_ACTIVE_STATUSES and (task.get("planEpoch") or current_epoch) < current_epoch):
            context_rot_warnings.append("older-epoch active task exists")
        if any(task.get("checkpointRequired") for task in related_tasks):
            context_rot_warnings.append("checkpoint required task exists")
        rot_score = len(context_rot_warnings)
        rot_status = "warning" if rot_score >= policy_summary["contextRot"]["warnScore"] else "healthy"
        threads[thread_key] = {
            "threadKey": thread_key,
            "currentPlanEpoch": current_epoch,
            "requestIds": [item.get("requestId") for item in requests][-10:],
            "activeRequestIds": [item.get("requestId") for item in active_requests][-10:],
            "activeTaskIds": [task.get("taskId") for task in related_tasks if task.get("status") in TASK_ACTIVE_STATUSES][:10],
            "queuedTaskIds": [task.get("taskId") for task in related_tasks if task.get("status") == "queued"][:10],
            "supersededTaskIds": [task.get("taskId") for task in related_tasks if task.get("status") in TASK_SUPERSEDED_STATUSES][:10],
            "appendChangeCount": sum(1 for item in requests if item.get("normalizedIntentClass") == "append_change"),
            "contextEnrichmentCount": sum(1 for item in requests if item.get("normalizedIntentClass") == "context_enrichment"),
            "inspectionCount": sum(1 for item in requests if item.get("normalizedIntentClass") == "inspection"),
            "duplicateCount": sum(1 for item in requests if item.get("normalizedIntentClass") == "duplicate_or_noop"),
            "latestRequestId": requests[-1].get("requestId"),
            "latestFusionDecision": requests[-1].get("fusionDecision"),
            "latestImpactClassification": requests[-1].get("impactClassification"),
            "lastSubmissionAt": requests[-1].get("createdAt"),
            "recentVerification": recent_verification_for_thread(tasks, thread_key),
            "contextRotScore": rot_score,
            "contextRotStatus": rot_status,
            "contextRotWarnings": context_rot_warnings[:5],
            "mergedContextRequestIds": bounded((thread_ledger.get(thread_key, {}) or {}).get("mergedContextRequestIds", []), 10),
            "mergedContextRefs": bounded((thread_ledger.get(thread_key, {}) or {}).get("mergedContextRefs", []), 5),
            "latestContextPaths": bounded((thread_ledger.get(thread_key, {}) or {}).get("latestContextPaths", []), 10),
            "contextDigest": (thread_ledger.get(thread_key, {}) or {}).get("contextDigest"),
        }
    context_rot_warnings = bounded(
        [
            {
                "threadKey": thread.get("threadKey"),
                "contextRotScore": thread.get("contextRotScore", 0),
                "contextRotStatus": thread.get("contextRotStatus"),
                "warnings": thread.get("contextRotWarnings", []),
            }
            for thread in threads.values()
            if thread.get("contextRotWarnings")
        ],
        policy_summary["intake"]["maxRecentThreads"],
    )
    return {
        "schemaVersion": SCHEMA_VERSION,
        "generator": generator,
        "generatedAt": now_iso(),
        "threadCount": len(threads),
        "activeThreadCount": sum(1 for item in threads.values() if item.get("activeRequestIds")),
        "contextRotWarningCount": len(context_rot_warnings),
        "threads": threads,
        "recentThreads": bounded(list(threads.values())[::-1], policy_summary["threading"]["maxRecentThreadEvents"]),
        "contextRotWarnings": context_rot_warnings,
    }


def build_change_summary(index: dict, task_pool: dict | None, thread_state: dict | None = None, *, generator: str, policy_summary: dict) -> dict:
    tasks = (task_pool or {}).get("tasks", [])
    append_requests = [request for request in index.get("requests", []) if request.get("normalizedIntentClass") == "append_change"]
    inspection_requests = [request for request in index.get("requests", []) if request.get("normalizedIntentClass") == "inspection"]
    superseded_tasks = [task for task in tasks if task.get("status") in TASK_SUPERSEDED_STATUSES]
    return {
        "schemaVersion": SCHEMA_VERSION,
        "generator": generator,
        "generatedAt": now_iso(),
        "appendChangeCount": len(append_requests),
        "inspectionOverlayCount": len(inspection_requests),
        "supersededQueuedTaskCount": len(superseded_tasks),
        "contextRotWarningCount": len((thread_state or {}).get("contextRotWarnings", [])),
        "recentAppendChanges": bounded(
            [
                {
                    "requestId": request.get("requestId"),
                    "threadKey": thread_key_from_request(request),
                    "targetPlanEpoch": request.get("targetPlanEpoch"),
                    "impactClassification": request.get("impactClassification"),
                    "supersededQueuedTaskIds": request.get("supersededQueuedTaskIds", []),
                    "checkpointTaskIds": request.get("checkpointTaskIds", []),
                    "goal": request.get("goal"),
                }
                for request in append_requests[-10:]
            ],
            policy_summary["threading"]["maxRecentChanges"],
        ),
        "recentInspectionOverlays": bounded(
            [
                {
                    "requestId": request.get("requestId"),
                    "threadKey": thread_key_from_request(request),
                    "goal": request.get("goal"),
                    "fusionDecision": request.get("fusionDecision"),
                }
                for request in inspection_requests[-10:]
            ],
            policy_summary["threading"]["maxRecentChanges"],
        ),
        "supersededQueuedTaskIds": bounded([task.get("taskId") for task in superseded_tasks], policy_summary["threading"]["maxRecentChanges"]),
        "contextRotWarnings": bounded((thread_state or {}).get("contextRotWarnings", []), policy_summary["intake"]["maxRecentThreads"]),
    }


def build_lineage_index(lineage_entries: list[dict], task_pool: dict | None, task_map: dict):
    tasks_by_id = {
        task.get("taskId"): task
        for task in (task_pool or {}).get("tasks", [])
        if task.get("taskId")
    }
    by_request = {}
    by_task = {}
    latest_by_request = {}
    latest_by_task = {}
    latest_by_session = {}
    for event in lineage_entries:
        request_id = event.get("requestId")
        task_id = event.get("taskId")
        session_id = event.get("sessionId")
        if request_id:
            summary = by_request.setdefault(request_id, {
                "requestId": request_id,
                "taskIds": [],
                "sessionIds": [],
                "worktreePaths": [],
                "verificationStatuses": [],
                "outcomes": [],
                "lastEventKind": None,
                "lastEventAt": None,
            })
            if task_id and task_id not in summary["taskIds"]:
                summary["taskIds"].append(task_id)
            if event.get("sessionId") and event.get("sessionId") not in summary["sessionIds"]:
                summary["sessionIds"].append(event.get("sessionId"))
            if event.get("worktreePath") and event.get("worktreePath") not in summary["worktreePaths"]:
                summary["worktreePaths"].append(event.get("worktreePath"))
            verification_status = (event.get("verification") or {}).get("overallStatus")
            if verification_status and verification_status not in summary["verificationStatuses"]:
                summary["verificationStatuses"].append(verification_status)
            outcome_status = (event.get("outcome") or {}).get("status")
            if outcome_status and outcome_status not in summary["outcomes"]:
                summary["outcomes"].append(outcome_status)
            summary["lastEventKind"] = event.get("kind")
            summary["lastEventAt"] = event.get("timestamp")
            latest_by_request[request_id] = {
                "kind": event.get("kind"),
                "timestamp": event.get("timestamp"),
                "taskId": task_id,
                "sessionId": session_id,
            }
        if task_id:
            task = tasks_by_id.get(task_id, {})
            summary = by_task.setdefault(task_id, {
                "taskId": task_id,
                "requestIds": [],
                "sessionIds": [],
                "worktreePath": task.get("worktreePath"),
                "diffSummary": task.get("diffSummary"),
                "verificationStatus": task.get("verificationStatus"),
                "outcome": task.get("outcome"),
                "lastEventKind": None,
                "lastEventAt": None,
            })
            if request_id and request_id not in summary["requestIds"]:
                summary["requestIds"].append(request_id)
            if event.get("sessionId") and event.get("sessionId") not in summary["sessionIds"]:
                summary["sessionIds"].append(event.get("sessionId"))
            if event.get("worktreePath"):
                summary["worktreePath"] = event.get("worktreePath")
            if event.get("diffSummary"):
                summary["diffSummary"] = event.get("diffSummary")
            if (event.get("verification") or {}).get("overallStatus"):
                summary["verificationStatus"] = event.get("verification", {}).get("overallStatus")
            if event.get("outcome"):
                summary["outcome"] = event.get("outcome")
            summary["lastEventKind"] = event.get("kind")
            summary["lastEventAt"] = event.get("timestamp")
            latest_by_task[task_id] = {
                "kind": event.get("kind"),
                "timestamp": event.get("timestamp"),
                "requestId": request_id,
                "sessionId": session_id,
            }
        if session_id:
            latest_by_session[session_id] = {
                "kind": event.get("kind"),
                "timestamp": event.get("timestamp"),
                "requestId": request_id,
                "taskId": task_id,
                "worktreePath": event.get("worktreePath"),
            }

    for binding in task_map.get("bindings", []):
        task_id = binding.get("taskId")
        request_id = binding.get("requestId")
        if not task_id or not request_id:
            continue
        request_summary = by_request.setdefault(request_id, {
            "requestId": request_id,
            "taskIds": [],
            "sessionIds": [],
            "worktreePaths": [],
            "verificationStatuses": [],
            "outcomes": [],
            "lastEventKind": None,
            "lastEventAt": None,
        })
        if task_id not in request_summary["taskIds"]:
            request_summary["taskIds"].append(task_id)

    active_bindings = []
    for binding in task_map.get("bindings", []):
        if binding.get("status") in REQUEST_TERMINAL_STATUSES:
            continue
        active_bindings.append(
            {
                "bindingId": binding.get("bindingId"),
                "requestId": binding.get("requestId"),
                "taskId": binding.get("taskId"),
                "status": binding.get("status"),
                "sessionId": binding.get("lineage", {}).get("sessionId"),
                "worktreePath": binding.get("lineage", {}).get("worktreePath"),
                "diffSummary": binding.get("lineage", {}).get("diffSummary"),
                "verificationStatus": binding.get("lineage", {}).get("verificationStatus"),
                "outcome": binding.get("lineage", {}).get("outcome"),
            }
        )

    return {
        "schemaVersion": SCHEMA_VERSION,
        "generator": "klein-harness",
        "generatedAt": now_iso(),
        "eventCount": len(lineage_entries),
        "lastSeq": lineage_entries[-1].get("seq") if lineage_entries else 0,
        "recentEvents": lineage_entries[-10:],
        "activeBindings": active_bindings,
        "latestByRequest": latest_by_request,
        "latestByTask": latest_by_task,
        "latestBySession": latest_by_session,
        "requests": by_request,
        "tasks": by_task,
    }
