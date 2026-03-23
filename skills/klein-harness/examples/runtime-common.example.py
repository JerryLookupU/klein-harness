#!/usr/bin/env python3
import json
import re
from collections import Counter
from datetime import datetime, timezone
from pathlib import Path


SCHEMA_VERSION = "1.0"
REQUEST_TERMINAL_STATUSES = {"completed", "cancelled"}
TASK_ACTIVE_STATUSES = {"active", "claimed", "in_progress", "dispatched", "running", "resumed"}
TASK_COMPLETED_STATUSES = {"completed", "validated", "done", "pass", "verified"}
SEVERE_ROUTE_FAILURES = {"illegal_action", "path_conflict", "session_conflict", "replan_required"}
REQUEST_SNAPSHOT_KINDS = {"audit", "replan", "stop", "bug", "feedback", "rca"}
RCA_TERMINAL_STATUSES = {"repaired", "closed"}
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


def now_iso():
    return datetime.now(timezone.utc).astimezone().isoformat(timespec="seconds")


def load_json(path: Path):
    return json.loads(path.read_text())


def load_optional_json(path: Path, default=None):
    if path.exists():
        return load_json(path)
    return default


def write_json(path: Path, data):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(data, ensure_ascii=False, indent=2) + "\n")


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


def ensure_runtime_scaffold(root: Path, generator: str = "harness-runtime"):
    root = root.resolve()
    harness = root / ".harness"
    state_dir = harness / "state"
    requests_dir = harness / "requests"
    archive_dir = requests_dir / "archive"
    research_dir = harness / "research"
    verification_dir = state_dir / "verification"
    runner_logs_dir = state_dir / "runner-logs"

    for directory in (
        harness,
        state_dir,
        requests_dir,
        archive_dir,
        research_dir,
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

    for path_name in ("current.json", "runtime.json", "blueprint-index.json", "request-summary.json", "lineage-index.json"):
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
        "lineage_path": lineage_path,
        "lineage_index_path": state_dir / "lineage-index.json",
        "log_index_path": log_index_path,
        "research_index_path": research_index_path,
        "request_summary_path": state_dir / "request-summary.json",
        "session_registry_path": session_registry_path,
        "runner_state_path": runner_state_path,
        "runner_heartbeats_path": runner_heartbeats_path,
        "verification_dir": verification_dir,
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


def request_goal_tokens(text: str):
    return {
        token
        for token in re.findall(r"[a-zA-Z0-9_-]+", (text or "").lower())
        if len(token) >= 3
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


def task_status_score(task: dict):
    status = task.get("status")
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
    goal_tokens = request_goal_tokens(request.get("goal"))
    scored = []
    for task in tasks:
        score = task_status_score(task)
        if score < 0:
            continue
        if task_kind_matches_request(request.get("kind"), task):
            score += 50
        haystack = " ".join([
            task.get("taskId") or "",
            task.get("title") or "",
            task.get("summary") or "",
            task.get("kind") or "",
            task.get("roleHint") or "",
        ]).lower()
        token_hits = sum(1 for token in goal_tokens if token in haystack)
        score += token_hits * 8
        score += task_priority_score(task)
        if task.get("taskId") and task.get("taskId").lower() in (request.get("goal") or "").lower():
            score += 25
        scored.append((score, task))

    scored.sort(key=lambda item: (-item[0], item[1].get("taskId") or ""))
    if (request.get("kind") or "").lower() == "stop":
        return [task for score, task in scored if score > 0][:3]
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
    bindings = request_bindings_for_request(task_map, request_id)
    if not bindings:
        return False
    statuses = {binding.get("status") for binding in bindings}
    if statuses == {"completed"}:
        return False
    if statuses and statuses <= {"verified", "completed"}:
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
            "updatedAt": timestamp,
        })
    elif status == "recoverable" and session_id:
        recoverable.append({
            "taskId": task_id,
            "sessionId": session_id,
            "lastKnownSessionId": session_id,
            "error": error,
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
):
    files = ensure_runtime_scaffold(root, generator=generator)
    index = load_json(files["request_index_path"])
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
        "threadKey": None,
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
    }
    append_jsonl(files["queue_path"], request)
    index["nextSeq"] = seq + 1
    index["generatedAt"] = now_iso()
    index["generator"] = generator
    index.setdefault("requests", []).append(request)
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


def explicit_ids_from_text(text: str, pattern: str) -> list[str]:
    if not text:
        return []
    return list(dict.fromkeys(re.findall(pattern, text)))


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
    bound = []
    blocked = []

    for request in index.get("requests", []):
        if request.get("status") in REQUEST_TERMINAL_STATUSES:
            continue
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
        if request_bindings_for_request(task_map, request.get("requestId")):
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
        "bindings": binding_summary,
        "boundRequestCount": counts.get("bound", 0),
        "runningRequestCount": counts.get("running", 0),
        "recoverableRequestCount": counts.get("recoverable", 0),
        "completedRequestCount": counts.get("completed", 0),
        "blockedRequestCount": counts.get("blocked", 0),
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
