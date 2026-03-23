# Control Plane State

Klein-Harness keeps the repo-local runtime explicit in three layers.

## 1. Cold evidence

Cold evidence is append-only or raw execution output. It is retained for audit, RCA, and targeted retrieval, not for default operator reads.

- `.harness/requests/queue.jsonl`
- `.harness/lineage.jsonl`
- `.harness/feedback-log.jsonl`
- `.harness/root-cause-log.jsonl`
- `.harness/state/runner-logs/*.log`

## 2. Runtime ledgers

Runtime ledgers are mutable source-of-truth files for scheduling and execution state.

- `.harness/state/request-index.json`
- `.harness/state/request-task-map.json`
- `.harness/task-pool.json`
- `.harness/session-registry.json`

These files drive request binding, task routing, claim state, session state, and lineage correlation.

## 3. Hot summaries

Hot summaries are bounded JSON projections for fast operator reads and compact worker context.
They are the default machine-facing surface. Markdown projections are secondary.

- `.harness/state/current.json`
- `.harness/state/runtime.json`
- `.harness/state/progress.json`
- `.harness/state/request-summary.json`
- `.harness/state/lineage-index.json`
- `.harness/state/feedback-summary.json`
- `.harness/state/root-cause-summary.json`
- `.harness/state/queue-summary.json`
- `.harness/state/task-summary.json`
- `.harness/state/worker-summary.json`
- `.harness/state/daemon-summary.json`
- `.harness/state/todo-summary.json`
- `.harness/state/completion-gate.json`
- `.harness/state/guard-state.json`
- `.harness/state/log-index.json`
- `.harness/state/policy-summary.json`
- `.harness/state/research-summary.json`

Every summary snapshot includes:

- `schemaVersion`
- `generator`
- `generatedAt`

## Progress surfaces

Machine-readable progress lives in `.harness/state/progress.json`.

Human-readable progress lives in `.harness/progress.md`.

`progress.md` is rendered from `state/progress.json`. Operator tools and prompts should prefer the JSON surface. Markdown is a human projection, not a machine dependency.

## Guard-owned execution

The runtime executes only when the guard can prove it is safe.

- `todo-summary.json` is the derived execution panel
- `completion-gate.json` is the completion decision surface
- `guard-state.json` is the safety boundary

Unknown dirty worktrees block automation.
Managed dirty worktrees become checkpoint-eligible provenance instead of being silently absorbed.

## Health separation

Klein-Harness keeps runtime health separate from worker-backend health.

- runtime health answers whether the repo-local control loop and daemon are healthy
- worker health answers whether claimed workers or sessions are active, stale, recoverable, or blocked
- dispatch backend answers which execution backend is being used for a run (`tmux` today, `print` for non-executing compatibility/debug)

tmux is the current default backend. It is not the scheduler and not the source of truth.
