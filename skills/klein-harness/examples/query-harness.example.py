#!/usr/bin/env python3
from __future__ import annotations
import argparse
import json
import sys
from collections import Counter
from pathlib import Path

from runtime_common import (
    build_log_index,
    collect_compact_log_entries,
    ensure_runtime_scaffold,
    extract_raw_log_windows,
    read_progress_state,
)


def load_json(path: Path):
    return json.loads(path.read_text())

def load_optional_json(path: Path, default=None):
    if path.exists():
        return load_json(path)
    return default


def summarize_tasks(tasks):
    return dict(Counter(task.get("status", "unknown") for task in tasks))


def summarize_features(features):
    return dict(Counter(feature.get("verificationStatus", feature.get("status", "unknown")) for feature in features))


def active_tasks(tasks):
    return [t for t in tasks if t.get("status") in {"active", "claimed", "in_progress"}]


def find_task(tasks, task_id: str):
    for task in tasks:
        if task.get("taskId") == task_id:
            return task
    raise KeyError(f"task not found: {task_id}")


def recent_task_failures(feedback_summary, task_id: str):
    if not feedback_summary or not task_id:
        return []
    by_task = feedback_summary.get("taskFeedbackSummary", {})
    return by_task.get(task_id, {}).get("recentFailures", [])


def make_overview(progress, tasks, work_items, features, session_registry, runtime_state=None, feedback_summary=None):
    active = active_tasks(tasks)
    return {
        "mode": progress.get("mode"),
        "planningStage": progress.get("planningStage"),
        "currentFocus": progress.get("currentFocus"),
        "currentRole": progress.get("currentRole"),
        "currentTaskId": progress.get("currentTaskId"),
        "currentTaskTitle": progress.get("currentTaskTitle"),
        "currentTaskSummary": progress.get("currentTaskSummary"),
        "blockerCount": len(progress.get("blockers", [])),
        "nextActions": progress.get("nextActions", []),
        "taskStatusCounts": summarize_tasks(tasks),
        "workItemStatusCounts": dict(Counter(item.get("status", "unknown") for item in work_items)),
        "featureStatusCounts": summarize_features(features),
        "activeTaskCount": (runtime_state or {}).get("activeTaskCount", len(active)),
        "activeRunnerCount": (runtime_state or {}).get("activeRunnerCount", 0),
        "recoverableTaskCount": (runtime_state or {}).get("recoverableTaskCount", 0),
        "staleRunnerCount": (runtime_state or {}).get("staleRunnerCount", 0),
        "blockedRouteCount": (runtime_state or {}).get("blockedRouteCount", 0),
        "orchestrationSessionId": (runtime_state or {}).get("orchestrationSessionId", session_registry.get("orchestrationSessionId")),
        "feedbackEventCount": (runtime_state or {}).get("feedbackEventCount", (feedback_summary or {}).get("feedbackEventCount", 0)),
        "feedbackErrorCount": (runtime_state or {}).get("feedbackErrorCount", (feedback_summary or {}).get("errorCount", 0)),
        "illegalActionCount": (runtime_state or {}).get("illegalActionCount", (feedback_summary or {}).get("illegalActionCount", 0)),
        "tasksWithRecentFailures": (runtime_state or {}).get("tasksWithRecentFailures", (feedback_summary or {}).get("tasksWithRecentFailures", [])),
        "requestCounts": (runtime_state or {}).get("requestCounts", {}),
        "activeRequest": (runtime_state or {}).get("activeRequest"),
    }


def make_progress(progress, tasks, work_items, features, feedback_summary=None):
    return {
        "mode": progress.get("mode"),
        "planningStage": progress.get("planningStage"),
        "currentFocus": progress.get("currentFocus"),
        "lastAuditStatus": progress.get("lastAuditStatus"),
        "blockers": progress.get("blockers", []),
        "taskStatusCounts": summarize_tasks(tasks),
        "workItemStatusCounts": dict(Counter(item.get("status", "unknown") for item in work_items)),
        "featureStatusCounts": summarize_features(features),
        "claimSummary": progress.get("claimSummary", {}),
        "recentFailures": (feedback_summary or {}).get("recentFailures", []),
    }


def make_current(progress, tasks, session_registry, feedback_summary=None):
    active = active_tasks(tasks)
    current = []
    for task in active:
        current.append(
            {
                "taskId": task.get("taskId"),
                "kind": task.get("kind"),
                "roleHint": task.get("roleHint"),
                "workerMode": task.get("workerMode"),
                "title": task.get("title"),
                "summary": task.get("summary"),
                "nodeId": task.get("claim", {}).get("nodeId"),
                "boundSessionId": task.get("claim", {}).get("boundSessionId"),
                "worktreePath": task.get("worktreePath"),
                "branchName": task.get("branchName"),
                "recentFailures": recent_task_failures(feedback_summary, task.get("taskId")),
            }
        )
    return {
        "currentTaskId": progress.get("currentTaskId"),
        "currentTaskTitle": progress.get("currentTaskTitle"),
        "currentTaskSummary": progress.get("currentTaskSummary"),
        "currentRole": progress.get("currentRole"),
        "activeTasks": current,
        "orchestrationSessionId": session_registry.get("orchestrationSessionId"),
    }


def make_blueprint(spec, task_pool, work_items, tasks):
    by_block = {}
    for block in spec.get("blocks", []):
        block_id = block.get("id")
        block_tasks = [t for t in tasks if t.get("blockId") == block_id]
        block_items = [w for w in work_items if set(w.get("featureIds", [])) & set(block.get("featureIds", []))]
        by_block[block_id] = {
            "title": block.get("title"),
            "goal": block.get("goal"),
            "status": block.get("status"),
            "featureIds": block.get("featureIds", []),
            "ownedPaths": block.get("ownedPaths", []),
            "verificationRuleIds": block.get("verificationRuleIds", []),
            "workItemIds": [w.get("id") for w in block_items],
            "taskIds": [t.get("taskId") for t in block_tasks],
        }
    return {
        "specRevision": spec.get("specRevision"),
        "planningStage": spec.get("planningStage"),
        "objective": spec.get("objective"),
        "integrationBranch": task_pool.get("integrationBranch"),
        "blocks": by_block,
    }


def make_task_view(task, feedback_summary=None):
    return {
        "taskId": task.get("taskId"),
        "threadKey": task.get("threadKey"),
        "planEpoch": task.get("planEpoch"),
        "kind": task.get("kind"),
        "roleHint": task.get("roleHint"),
        "workerMode": task.get("workerMode"),
        "title": task.get("title"),
        "summary": task.get("summary"),
        "description": task.get("description"),
        "status": task.get("status"),
        "planningStage": task.get("planningStage"),
        "dependsOn": task.get("dependsOn", []),
        "ownedPaths": task.get("ownedPaths", []),
        "verificationRuleIds": task.get("verificationRuleIds", []),
        "branchName": task.get("branchName"),
        "baseRef": task.get("baseRef") or task.get("dispatch", {}).get("baseRef"),
        "worktreePath": task.get("worktreePath"),
        "worktreeStatus": task.get("worktreeStatus"),
        "worktreePreparedAt": task.get("worktreePreparedAt"),
        "integrationBranch": task.get("integrationBranch") or task.get("dispatch", {}).get("integrationBranch"),
        "mergeRequired": task.get("mergeRequired", task.get("handoff", {}).get("mergeRequired")),
        "mergeStatus": task.get("mergeStatus"),
        "mergeCheckedAt": task.get("mergeCheckedAt"),
        "mergedCommit": task.get("mergedCommit"),
        "cleanupStatus": task.get("cleanupStatus"),
        "conflictPaths": task.get("conflictPaths", []),
        "diffBase": task.get("diffBase"),
        "diffSummary": task.get("diffSummary"),
        "resumeStrategy": task.get("resumeStrategy"),
        "preferredResumeSessionId": task.get("preferredResumeSessionId"),
        "verificationStatus": task.get("verificationStatus"),
        "verificationSummary": task.get("verificationSummary"),
        "verificationResultPath": task.get("verificationResultPath"),
        "claim": task.get("claim", {}),
        "handoff": task.get("handoff", {}),
        "checkpointRequired": task.get("checkpointRequired"),
        "checkpointReason": task.get("checkpointReason"),
        "supersededByRequestId": task.get("supersededByRequestId"),
        "recentFailures": recent_task_failures(feedback_summary, task.get("taskId")),
    }


def make_feedback_view(feedback_summary, task_id=None):
    if not feedback_summary:
        return {
            "feedbackEventCount": 0,
            "errorCount": 0,
            "criticalCount": 0,
            "illegalActionCount": 0,
            "recentFailures": [],
            "taskFeedbackSummary": {},
        }

    if task_id:
        task_summary = feedback_summary.get("taskFeedbackSummary", {}).get(task_id)
        if not task_summary:
            raise KeyError(f"feedback not found for task: {task_id}")
        return task_summary

    return {
        "feedbackEventCount": feedback_summary.get("feedbackEventCount", 0),
        "errorCount": feedback_summary.get("errorCount", 0),
        "criticalCount": feedback_summary.get("criticalCount", 0),
        "illegalActionCount": feedback_summary.get("illegalActionCount", 0),
        "tasksWithRecentFailures": feedback_summary.get("tasksWithRecentFailures", []),
        "recentFailures": feedback_summary.get("recentFailures", []),
        "byType": feedback_summary.get("byType", {}),
        "bySeverity": feedback_summary.get("bySeverity", {}),
    }


def make_requests_view(request_summary, intake_summary=None, thread_state=None, change_summary=None):
    return {
        "requestCounts": request_summary.get("requestCounts", {}),
        "byFrontDoorClass": request_summary.get("byFrontDoorClass", (intake_summary or {}).get("byFrontDoorClass", {})),
        "duplicateRequestCount": request_summary.get("duplicateRequestCount", 0),
        "contextMergeCount": request_summary.get("contextMergeCount", 0),
        "inspectionOverlayCount": request_summary.get("inspectionOverlayCount", 0),
        "threadCount": (thread_state or {}).get("threadCount", 0),
        "activeThreadCount": (thread_state or {}).get("activeThreadCount", 0),
        "compoundSplitCount": (intake_summary or {}).get("compoundSplitCount", 0),
        "appendChangeCount": (change_summary or {}).get("appendChangeCount", 0),
        "activeRequest": request_summary.get("activeRequest"),
        "recentRequests": request_summary.get("recentRequests", []),
        "recentSubmissionClassifications": request_summary.get("recentSubmissionClassifications", []),
        "bindings": request_summary.get("bindings", []),
    }


def make_workers_view(worker_summary):
    return worker_summary or {
        "workerCount": 0,
        "healthyWorkerCount": 0,
        "staleWorkerCount": 0,
        "recoverableWorkerCount": 0,
        "dispatchBackendCounts": {},
        "workerNodes": [],
    }


def make_daemon_view(daemon_summary):
    return daemon_summary or {
        "status": "stopped",
        "runtimeHealth": "unknown",
        "dispatchBackendDefault": None,
        "workerBackendHealth": {},
    }


def make_worktrees_view(worktree_registry):
    return worktree_registry or {
        "schemaVersion": "1.0",
        "worktrees": [],
    }


def make_merge_queue_view(merge_queue, merge_summary):
    return {
        "integrationBranch": merge_queue.get("integrationBranch") if merge_queue else None,
        "queueDepth": merge_summary.get("queueDepth", 0) if merge_summary else 0,
        "readyToMergeCount": merge_summary.get("readyToMergeCount", 0) if merge_summary else 0,
        "conflictCount": merge_summary.get("conflictCount", 0) if merge_summary else 0,
        "items": (merge_queue or {}).get("items", []),
        "readyToMerge": (merge_summary or {}).get("readyToMerge", []),
        "openConflicts": (merge_summary or {}).get("openConflicts", []),
        "recentMerged": (merge_summary or {}).get("recentMerged", []),
        "supersededCandidates": (merge_summary or {}).get("supersededCandidates", []),
    }


def make_blockers_view(queue_summary, task_summary, log_index):
    return {
        "queueBlocked": queue_summary.get("recentBlockedRequests", []) if queue_summary else [],
        "routeBlocked": task_summary.get("blockedRoutes", []) if task_summary else [],
        "logBlocked": log_index.get("openBlockers", []) if log_index else [],
    }


def matches_log(entry, *, task_id=None, request_id=None, session_id=None, tag=None, path_filter=None, severity=None, status=None, keyword=None):
    meta = entry.get("frontMatter", {})
    haystack = " ".join(
        [
            meta.get("taskId") or "",
            meta.get("requestId") or "",
            meta.get("sessionId") or "",
            " ".join(meta.get("tags", []) or []),
            entry.get("body") or "",
        ]
    ).lower()
    if task_id and meta.get("taskId") != task_id:
        return False
    if request_id and meta.get("requestId") != request_id:
        return False
    if session_id and meta.get("sessionId") != session_id:
        return False
    if tag and tag not in (meta.get("tags") or []):
        return False
    if path_filter:
        paths = (meta.get("ownedPaths") or []) + [meta.get("rawLogPath") or ""]
        if not any(path_filter in item for item in paths if item):
            return False
    if severity and meta.get("severity") != severity:
        return False
    if status and meta.get("status") != status:
        return False
    if keyword and keyword.lower() not in haystack:
        return False
    return True


def make_logs_view(root: Path, log_index, **filters):
    entries = collect_compact_log_entries(root)
    matches = [entry for entry in entries if matches_log(entry, **filters)]
    return {
        "compactLogCount": log_index.get("compactLogCount", len(entries)),
        "matchCount": len(matches),
        "recentHighSignalLogs": log_index.get("recentHighSignalLogs", []),
        "openBlockers": log_index.get("openBlockers", []),
        "recurringTags": log_index.get("recurringTags", {}),
        "matches": [
            {
                "taskId": entry.get("frontMatter", {}).get("taskId"),
                "requestId": entry.get("frontMatter", {}).get("requestId"),
                "sessionId": entry.get("frontMatter", {}).get("sessionId"),
                "severity": entry.get("frontMatter", {}).get("severity"),
                "status": entry.get("frontMatter", {}).get("status"),
                "tags": entry.get("frontMatter", {}).get("tags", []),
                "path": entry.get("path"),
                "summary": entry.get("oneScreenSummary", [])[:3],
            }
            for entry in matches[:20]
        ],
    }


def make_log_view(root: Path, *, task_id=None, request_id=None, keyword=None, detail=False):
    entries = collect_compact_log_entries(root)
    selected = None
    for entry in entries:
        if task_id and entry.get("frontMatter", {}).get("taskId") == task_id:
            selected = entry
            break
        if request_id and entry.get("frontMatter", {}).get("requestId") == request_id:
            selected = entry
            break
    if selected is None:
        raise KeyError("compact log not found")
    payload = {
        "path": selected.get("path"),
        "frontMatter": selected.get("frontMatter"),
        "oneScreenSummary": selected.get("oneScreenSummary", []),
        "facts": selected.get("facts", [])[:6],
        "blockers": selected.get("blockers", [])[:6],
        "evidenceRefs": selected.get("evidenceRefs", [])[:8],
    }
    if detail:
        raw_log_rel = selected.get("frontMatter", {}).get("rawLogPath")
        if raw_log_rel:
            payload["detailWindows"] = extract_raw_log_windows(root / raw_log_rel, keywords=[keyword] if keyword else None, task_id=task_id)
    return payload


def format_text(view, payload):
    if view == "overview":
        lines = [
            f"mode: {payload.get('mode')}",
            f"planningStage: {payload.get('planningStage')}",
            f"focus: {payload.get('currentFocus')}",
            f"role: {payload.get('currentRole')}",
            f"current: {payload.get('currentTaskId')} {payload.get('currentTaskTitle')}",
            f"summary: {payload.get('currentTaskSummary')}",
            f"activeTaskCount: {payload.get('activeTaskCount')}",
            f"activeRunners: {payload.get('activeRunnerCount', 0)}",
            f"recoverableTasks: {payload.get('recoverableTaskCount', 0)}",
            f"blockedRoutes: {payload.get('blockedRouteCount', 0)}",
            f"feedbackErrors: {payload.get('feedbackErrorCount', 0)}",
            f"illegalActions: {payload.get('illegalActionCount', 0)}",
            f"requestCounts: {payload.get('requestCounts', {})}",
        ]
        if payload.get("activeRequest"):
            lines.append(
                f"activeRequest: {payload['activeRequest'].get('requestId')} [{payload['activeRequest'].get('status')}]"
            )
        return "\n".join(lines)
    if view == "progress":
        return "\n".join(
            [
                f"mode: {payload.get('mode')}",
                f"planningStage: {payload.get('planningStage')}",
                f"lastAuditStatus: {payload.get('lastAuditStatus')}",
                f"taskStatusCounts: {payload.get('taskStatusCounts')}",
                f"workItemStatusCounts: {payload.get('workItemStatusCounts')}",
                f"featureStatusCounts: {payload.get('featureStatusCounts')}",
                f"recentFailures: {len(payload.get('recentFailures', []))}",
            ]
        )
    if view == "current":
        lines = [
            f"currentRole: {payload.get('currentRole')}",
            f"current: {payload.get('currentTaskId')} {payload.get('currentTaskTitle')}",
            f"summary: {payload.get('currentTaskSummary')}",
            "activeTasks:",
        ]
        for item in payload.get("activeTasks", []):
            lines.append(f"- {item['taskId']} [{item['kind']}] {item['title']} @ {item.get('nodeId')}")
            for failure in item.get("recentFailures", []):
                lines.append(f"  recentFailure: {failure.get('feedbackType')} {failure.get('message')}")
        return "\n".join(lines)
    if view == "blueprint":
        lines = [
            f"specRevision: {payload.get('specRevision')}",
            f"planningStage: {payload.get('planningStage')}",
            f"objective: {payload.get('objective')}",
            "blocks:",
        ]
        for block_id, block in payload.get("blocks", {}).items():
            lines.append(f"- {block_id} {block['title']} status={block['status']} tasks={block['taskIds']}")
        return "\n".join(lines)
    if view == "task":
        lines = [
            f"taskId: {payload.get('taskId')}",
            f"threadKey: {payload.get('threadKey')}",
            f"planEpoch: {payload.get('planEpoch')}",
            f"kind: {payload.get('kind')}",
            f"roleHint: {payload.get('roleHint')}",
            f"workerMode: {payload.get('workerMode')}",
            f"title: {payload.get('title')}",
            f"summary: {payload.get('summary')}",
            f"status: {payload.get('status')}",
            f"planningStage: {payload.get('planningStage')}",
            f"nodeId: {payload.get('claim', {}).get('nodeId')}",
            f"boundSessionId: {payload.get('claim', {}).get('boundSessionId')}",
            f"branchName: {payload.get('branchName')}",
            f"baseRef: {payload.get('baseRef')}",
            f"worktreePath: {payload.get('worktreePath')}",
            f"worktreeStatus: {payload.get('worktreeStatus')}",
            f"worktreePreparedAt: {payload.get('worktreePreparedAt')}",
            f"integrationBranch: {payload.get('integrationBranch')}",
            f"mergeRequired: {payload.get('mergeRequired')}",
            f"mergeStatus: {payload.get('mergeStatus')}",
            f"mergeCheckedAt: {payload.get('mergeCheckedAt')}",
            f"mergedCommit: {payload.get('mergedCommit')}",
            f"cleanupStatus: {payload.get('cleanupStatus')}",
            f"conflictPaths: {payload.get('conflictPaths')}",
            f"diffBase: {payload.get('diffBase')}",
            f"diffSummary: {payload.get('diffSummary')}",
            f"verificationStatus: {payload.get('verificationStatus')}",
            f"verificationSummary: {payload.get('verificationSummary')}",
            f"verificationResultPath: {payload.get('verificationResultPath')}",
            f"checkpointRequired: {payload.get('checkpointRequired')}",
            f"supersededByRequestId: {payload.get('supersededByRequestId')}",
        ]
        for failure in payload.get("recentFailures", []):
            lines.append(f"recentFailure: {failure.get('feedbackType')} [{failure.get('severity')}] {failure.get('message')}")
        return "\n".join(lines)
    if view == "feedback":
        if "taskId" in payload:
            lines = [
                f"taskId: {payload.get('taskId')}",
                f"feedbackCount: {payload.get('feedbackCount', 0)}",
                f"errorCount: {payload.get('errorCount', 0)}",
                f"criticalCount: {payload.get('criticalCount', 0)}",
                f"latestFeedbackType: {payload.get('latestFeedbackType')}",
                f"latestSeverity: {payload.get('latestSeverity')}",
                f"latestMessage: {payload.get('latestMessage')}",
            ]
            for failure in payload.get("recentFailures", []):
                lines.append(f"- {failure.get('feedbackType')} [{failure.get('severity')}] {failure.get('message')}")
            return "\n".join(lines)
        lines = [
            f"feedbackEventCount: {payload.get('feedbackEventCount', 0)}",
            f"errorCount: {payload.get('errorCount', 0)}",
            f"criticalCount: {payload.get('criticalCount', 0)}",
            f"illegalActionCount: {payload.get('illegalActionCount', 0)}",
            f"tasksWithRecentFailures: {payload.get('tasksWithRecentFailures', [])}",
        ]
        for failure in payload.get("recentFailures", []):
            lines.append(
                f"- {failure.get('taskId')} {failure.get('feedbackType')} [{failure.get('severity')}] {failure.get('message')}"
            )
        return "\n".join(lines)
    if view == "logs":
        lines = [
            f"compactLogCount: {payload.get('compactLogCount', 0)}",
            f"matchCount: {payload.get('matchCount', 0)}",
            f"openBlockers: {len(payload.get('openBlockers', []))}",
            f"recurringTags: {payload.get('recurringTags', {})}",
        ]
        for item in payload.get("matches", []):
            lines.append(
                f"- {item.get('taskId')} [{item.get('severity')}/{item.get('status')}] {item.get('path')} :: {' | '.join(item.get('summary', []))}"
            )
        return "\n".join(lines)
    if view == "log":
        lines = [
            f"path: {payload.get('path')}",
            f"taskId: {payload.get('frontMatter', {}).get('taskId')}",
            f"requestId: {payload.get('frontMatter', {}).get('requestId')}",
            f"sessionId: {payload.get('frontMatter', {}).get('sessionId')}",
            f"severity: {payload.get('frontMatter', {}).get('severity')}",
            f"status: {payload.get('frontMatter', {}).get('status')}",
            f"tags: {payload.get('frontMatter', {}).get('tags', [])}",
        ]
        lines.extend(payload.get("oneScreenSummary", []))
        for blocker in payload.get("blockers", []):
            lines.append(f"blocker: {blocker}")
        if payload.get("detailWindows"):
            lines.append("detailWindows:")
            for window in payload["detailWindows"]:
                lines.append(f"- lines {window.get('lineStart')}-{window.get('lineEnd')}")
                lines.append(window.get("snippet"))
        return "\n".join(lines)
    if view == "requests":
        lines = [
            f"requestCounts: {payload.get('requestCounts', {})}",
            f"byFrontDoorClass: {payload.get('byFrontDoorClass', {})}",
            f"duplicateRequestCount: {payload.get('duplicateRequestCount', 0)}",
            f"contextMergeCount: {payload.get('contextMergeCount', 0)}",
            f"inspectionOverlayCount: {payload.get('inspectionOverlayCount', 0)}",
            f"threadCount: {payload.get('threadCount', 0)}",
            f"activeThreadCount: {payload.get('activeThreadCount', 0)}",
        ]
        active = payload.get("activeRequest")
        if active:
            lines.append(f"activeRequest: {active.get('requestId')} [{active.get('status')}] {active.get('kind')} frontDoor={active.get('frontDoorClass')} intent={active.get('normalizedIntentClass')} fusion={active.get('fusionDecision')} thread={active.get('threadKey')} epoch={active.get('targetPlanEpoch')}")
        for item in payload.get("recentSubmissionClassifications", []):
            lines.append(f"- classification {item.get('requestId')} frontDoor={item.get('frontDoorClass')} intent={item.get('normalizedIntentClass')} fusion={item.get('fusionDecision')} thread={item.get('threadKey')} epoch={item.get('targetPlanEpoch')}")
        for item in payload.get("recentRequests", []):
            lines.append(f"- {item.get('requestId')} [{item.get('status')}] {item.get('kind')} thread={item.get('threadKey')} epoch={item.get('targetPlanEpoch')} {item.get('goal')}")
        return "\n".join(lines)
    if view == "workers":
        lines = [
            f"workerCount: {payload.get('workerCount', 0)}",
            f"healthyWorkerCount: {payload.get('healthyWorkerCount', 0)}",
            f"staleWorkerCount: {payload.get('staleWorkerCount', 0)}",
            f"recoverableWorkerCount: {payload.get('recoverableWorkerCount', 0)}",
            f"dispatchBackendCounts: {payload.get('dispatchBackendCounts', {})}",
        ]
        for item in payload.get("workerNodes", []):
            lines.append(
                f"- {item.get('taskId')} thread={item.get('threadKey')} epoch={item.get('planEpoch')} node={item.get('nodeId')} backend={item.get('dispatchBackend')} nodeHealth={item.get('nodeHealth')} backendHealth={item.get('backendHealth')} worktree={item.get('worktreePath')}"
            )
        return "\n".join(lines)
    if view == "daemon":
        return "\n".join(
            [
                f"status: {payload.get('status')}",
                f"runtimeHealth: {payload.get('runtimeHealth')}",
                f"dispatchBackendDefault: {payload.get('dispatchBackendDefault')}",
                f"sessionName: {payload.get('sessionName')}",
                f"sessionAlive: {payload.get('sessionAlive')}",
                f"activeRunnerCount: {payload.get('activeRunnerCount')}",
                f"staleRunnerCount: {payload.get('staleRunnerCount')}",
                f"workerBackendHealth: {payload.get('workerBackendHealth', {})}",
            ]
        )
    if view == "worktrees":
        lines = [
            f"worktreeCount: {len(payload.get('worktrees', []))}",
        ]
        for item in payload.get("worktrees", [])[:10]:
            lines.append(
                f"- {item.get('taskId')} [{item.get('status')}] branch={item.get('branchName')} worktree={item.get('worktreePath')} merge={item.get('mergeRequired')} cleanup={item.get('cleanupStatus')}"
            )
        return "\n".join(lines)
    if view == "merge-queue":
        lines = [
            f"integrationBranch: {payload.get('integrationBranch')}",
            f"queueDepth: {payload.get('queueDepth')}",
            f"readyToMergeCount: {payload.get('readyToMergeCount')}",
            f"conflictCount: {payload.get('conflictCount')}",
        ]
        for item in payload.get("items", [])[:10]:
            lines.append(
                f"- {item.get('taskId')} [{item.get('mergeStatus')}] branch={item.get('branchName')} epoch={item.get('planEpoch')} supersededByEpoch={item.get('supersededByEpoch')}"
            )
        return "\n".join(lines)
    if view == "integration":
        lines = [
            f"integrationBranch: {payload.get('integrationBranch')}",
            f"readyToMergeCount: {payload.get('readyToMergeCount')}",
            f"conflictCount: {payload.get('conflictCount')}",
        ]
        for item in payload.get("recentMerged", [])[:10]:
            lines.append(
                f"- merged {item.get('taskId')} commit={item.get('mergedCommit')} branch={item.get('branchName')}"
            )
        return "\n".join(lines)
    if view == "conflicts":
        lines = [
            f"conflictCount: {payload.get('conflictCount')}",
        ]
        for item in payload.get("openConflicts", [])[:10]:
            lines.append(
                f"- {item.get('taskId')} branch={item.get('branchName')} conflictPaths={item.get('conflictPaths')}"
            )
        return "\n".join(lines)
    if view == "blockers":
        lines = ["queueBlocked:"]
        if payload.get("queueBlocked"):
            lines.extend(f"- {item.get('requestId')} {item.get('statusReason')}" for item in payload["queueBlocked"])
        else:
            lines.append("- none")
        lines.append("routeBlocked:")
        if payload.get("routeBlocked"):
            lines.extend(f"- {item.get('taskId')} {item.get('gateReason')}" for item in payload["routeBlocked"])
        else:
            lines.append("- none")
        lines.append("logBlocked:")
        if payload.get("logBlocked"):
            lines.extend(f"- {item.get('taskId')} {item.get('blockers')}" for item in payload["logBlocked"])
        else:
            lines.append("- none")
        return "\n".join(lines)
    return json.dumps(payload, ensure_ascii=False, indent=2)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--root", required=True)
    parser.add_argument("--view", required=True, choices=["overview", "progress", "current", "blueprint", "task", "feedback", "requests", "workers", "daemon", "worktrees", "merge-queue", "integration", "conflicts", "blockers", "logs", "log"])
    parser.add_argument("--task-id")
    parser.add_argument("--request-id")
    parser.add_argument("--session-id")
    parser.add_argument("--tag")
    parser.add_argument("--path-filter")
    parser.add_argument("--severity")
    parser.add_argument("--status")
    parser.add_argument("--keyword")
    parser.add_argument("--detail", action="store_true")
    parser.add_argument("--format", default="json", choices=["json", "text"])
    args = parser.parse_args()

    root = Path(args.root).resolve()
    files = ensure_runtime_scaffold(root, generator="harness-query")
    harness = files["harness"]
    state_dir = files["state_dir"]
    current_state = load_optional_json(state_dir / "current.json")
    runtime_state = load_optional_json(state_dir / "runtime.json")
    blueprint_state = load_optional_json(state_dir / "blueprint-index.json")
    feedback_summary = load_optional_json(state_dir / "feedback-summary.json")
    queue_summary = load_optional_json(files["queue_summary_path"], {})
    intake_summary = load_optional_json(files["intake_summary_path"], {})
    thread_state = load_optional_json(files["thread_state_path"], {})
    change_summary = load_optional_json(files["change_summary_path"], {})
    task_summary = load_optional_json(files["task_summary_path"], {})
    worker_summary = load_optional_json(files["worker_summary_path"], {})
    daemon_summary = load_optional_json(files["daemon_summary_path"], {})
    request_summary = load_optional_json(files["request_summary_path"], {})
    worktree_registry = load_optional_json(files["worktree_registry_path"], {})
    merge_queue = load_optional_json(files["merge_queue_path"], {})
    merge_summary = load_optional_json(files["merge_summary_path"], {})
    log_index = load_optional_json(state_dir / "log-index.json") or build_log_index(root)

    progress = current_state or read_progress_state(files, generator="harness-query")
    task_pool = load_json(harness / "task-pool.json")
    work_items = load_json(harness / "work-items.json")
    features = load_json(harness / "features.json")
    spec = load_json(harness / "spec.json")
    session_registry_path = harness / "session-registry.json"
    session_registry = load_json(session_registry_path) if session_registry_path.exists() else {}

    tasks = task_pool.get("tasks", [])
    items = work_items.get("items", [])
    feature_items = features.get("features", [])

    if args.view == "overview":
        if current_state and runtime_state:
            payload = make_overview(progress, tasks, items, feature_items, session_registry, runtime_state, feedback_summary)
        else:
            payload = make_overview(progress, tasks, items, feature_items, session_registry, None, feedback_summary)
    elif args.view == "progress":
        payload = make_progress(progress, tasks, items, feature_items, feedback_summary)
    elif args.view == "current":
        if current_state and runtime_state:
            payload = {
                "currentTaskId": current_state.get("currentTaskId"),
                "currentTaskTitle": current_state.get("currentTaskTitle"),
                "currentTaskSummary": current_state.get("currentTaskSummary"),
                "currentRole": current_state.get("currentRole"),
                "activeTasks": runtime_state.get("activeTasks", []),
                "activeRuns": runtime_state.get("activeRuns", []),
                "orchestrationSessionId": runtime_state.get("orchestrationSessionId"),
                "activeRequest": runtime_state.get("activeRequest"),
                "activeBinding": runtime_state.get("activeBinding"),
            }
        else:
            payload = make_current(progress, tasks, session_registry, feedback_summary)
    elif args.view == "task":
        if not args.task_id:
            raise ValueError("--task-id is required for view=task")
        payload = make_task_view(find_task(tasks, args.task_id), feedback_summary)
    elif args.view == "feedback":
        payload = make_feedback_view(feedback_summary, args.task_id)
    elif args.view == "requests":
        payload = make_requests_view(request_summary, intake_summary, thread_state, change_summary)
    elif args.view == "workers":
        payload = make_workers_view(worker_summary)
    elif args.view == "daemon":
        payload = make_daemon_view(daemon_summary)
    elif args.view == "worktrees":
        payload = make_worktrees_view(worktree_registry)
    elif args.view == "merge-queue":
        payload = make_merge_queue_view(merge_queue, merge_summary)
    elif args.view == "integration":
        payload = make_merge_queue_view(merge_queue, merge_summary)
    elif args.view == "conflicts":
        payload = {
            "conflictCount": merge_summary.get("conflictCount", 0),
            "openConflicts": merge_summary.get("openConflicts", []),
        }
    elif args.view == "blockers":
        payload = make_blockers_view(queue_summary, task_summary, log_index)
    elif args.view == "logs":
        payload = make_logs_view(
            root,
            log_index,
            task_id=args.task_id,
            request_id=args.request_id,
            session_id=args.session_id,
            tag=args.tag,
            path_filter=args.path_filter,
            severity=args.severity,
            status=args.status,
            keyword=args.keyword,
        )
    elif args.view == "log":
        payload = make_log_view(
            root,
            task_id=args.task_id,
            request_id=args.request_id,
            keyword=args.keyword,
            detail=args.detail,
        )
    else:
        payload = blueprint_state or make_blueprint(spec, task_pool, items, tasks)

    if args.format == "text":
        print(format_text(args.view, payload))
    else:
        print(json.dumps(payload, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    try:
        main()
    except Exception as exc:
        print(f"query-harness example failed: {exc}", file=sys.stderr)
        sys.exit(1)
