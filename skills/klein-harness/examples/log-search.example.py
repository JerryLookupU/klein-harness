#!/usr/bin/env python3
import argparse
import json
import sys
from pathlib import Path

from runtime_common import build_log_index, collect_compact_log_entries, extract_raw_log_windows


def matches_filters(entry: dict, args) -> bool:
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
    if args.task_id and meta.get("taskId") != args.task_id:
        return False
    if args.request_id and meta.get("requestId") != args.request_id:
        return False
    if args.session_id and meta.get("sessionId") != args.session_id:
        return False
    if args.tag and args.tag not in (meta.get("tags") or []):
        return False
    if args.path_filter:
        search_paths = (meta.get("ownedPaths") or []) + [meta.get("rawLogPath") or ""]
        if not any(args.path_filter in item for item in search_paths if item):
            return False
    if args.severity and meta.get("severity") != args.severity:
        return False
    if args.status and meta.get("status") != args.status:
        return False
    if args.keyword and args.keyword.lower() not in haystack:
        return False
    return True


def render_text(payload: dict) -> str:
    lines = [
        f"compactLogCount: {payload.get('compactLogCount', 0)}",
        f"matchCount: {payload.get('matchCount', 0)}",
    ]
    for match in payload.get("matches", []):
        lines.append(
            f"- {match.get('taskId')} [{match.get('severity')}/{match.get('status')}] {match.get('path')}"
        )
        for item in match.get("summary", [])[:3]:
            lines.append(f"  {item}")
        if match.get("detailWindows"):
            for window in match["detailWindows"]:
                lines.append(f"  lines {window.get('lineStart')}-{window.get('lineEnd')}:")
                lines.append(window.get("snippet"))
    return "\n".join(lines)


def main():
    parser = argparse.ArgumentParser(description="search compact harness logs and retrieve raw evidence windows on demand")
    parser.add_argument("--root", required=True)
    parser.add_argument("--task-id")
    parser.add_argument("--request-id")
    parser.add_argument("--session-id")
    parser.add_argument("--tag")
    parser.add_argument("--path-filter")
    parser.add_argument("--severity")
    parser.add_argument("--status")
    parser.add_argument("--keyword")
    parser.add_argument("--detail", action="store_true")
    parser.add_argument("--format", default="text", choices=["text", "json"])
    args = parser.parse_args()

    root = Path(args.root).resolve()
    entries = collect_compact_log_entries(root)
    results = []
    for entry in entries:
        if not matches_filters(entry, args):
            continue
        record = {
            "taskId": entry.get("frontMatter", {}).get("taskId"),
            "requestId": entry.get("frontMatter", {}).get("requestId"),
            "sessionId": entry.get("frontMatter", {}).get("sessionId"),
            "severity": entry.get("frontMatter", {}).get("severity"),
            "status": entry.get("frontMatter", {}).get("status"),
            "tags": entry.get("frontMatter", {}).get("tags", []),
            "path": entry.get("path"),
            "summary": entry.get("oneScreenSummary", [])[:3],
        }
        if args.detail:
            raw_log_rel = entry.get("frontMatter", {}).get("rawLogPath")
            if raw_log_rel:
                record["detailWindows"] = extract_raw_log_windows(
                    root / raw_log_rel,
                    keywords=[args.keyword] if args.keyword else None,
                    task_id=args.task_id,
                )
        results.append(record)

    payload = {
        "compactLogCount": build_log_index(root).get("compactLogCount", 0),
        "matchCount": len(results),
        "matches": results[:20],
    }
    if args.format == "json":
        print(json.dumps(payload, ensure_ascii=False, indent=2))
    else:
        print(render_text(payload))


if __name__ == "__main__":
    try:
        main()
    except Exception as exc:
        print(f"log-search example failed: {exc}", file=sys.stderr)
        sys.exit(1)
