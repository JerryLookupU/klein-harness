# Log Search Architecture

Klein-Harness keeps raw runner logs as cold evidence and adds a compact shareable log layer for cross-worker handoff.

## Retrieval Order

Downstream workers should prefer:

1. `.harness/state/current.json`
2. `.harness/state/runtime.json`
3. `.harness/state/request-summary.json`
4. `.harness/state/lineage-index.json`
5. `.harness/log-<taskId>.md`
6. `.harness/state/runner-logs/<taskId>.log` only when targeted detail is required

Default search should stay summary-first.

## Surfaces

- Raw evidence: `.harness/state/runner-logs/<taskId>.log`
- Compact handoff: `.harness/log-<taskId>.md`
- Hot index: `.harness/state/log-index.json`
- Operator command: `.harness/bin/harness-log-search`
- Additive query views: `harness-query logs` and `harness-query log`

## Compact Log Rules

- Keep the log small enough to fit in a downstream worker prompt
- Reference evidence paths instead of copying long transcript sections
- Expose only cross-worker relevant facts, touched contracts, blockers, verification notes, and open questions
- Never dump hidden reasoning

## Search Behavior

Default search scans compact markdown logs and `log-index.json`.

Supported filters:

- `taskId`
- `requestId`
- `sessionId`
- `tag`
- `path`
- `severity`
- `status`
- `keyword`

`--detail` retrieves targeted windows from raw logs instead of opening the full transcript by default.
