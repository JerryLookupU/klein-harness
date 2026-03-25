# Runtime MVP

Klein MVP is a repo-local runtime with one canonical control plane:

- `cmd/harness` is the only canonical CLI
- Go owns bootstrap, route, dispatch, lease, burst, verify, query, and control
- native `tmux` owns session execution only
- native `codex` owns model execution only
- shell is compatibility-only

## Canonical Command Map

- `harness init`
- `harness submit`
- `harness tasks`
- `harness task`
- `harness control`
- `harness daemon run-once`
- `harness daemon loop`

## State Files

Authoritative:

- `.harness/requests/queue.jsonl`
  - writer: `internal/runtime.Submit`
  - reader: runtime, operator query, audits
  - rule: append-only
- `.harness/task-pool.json`
  - writer: `internal/runtime`, control actions
  - reader: route, query, control
  - rule: upsert by `taskId`
- `.harness/state/dispatch-summary.json`
  - writer: `internal/dispatch`
  - reader: runtime, worker supervisor, query
  - rule: CAS snapshot, idempotent by `idempotencyKey`
- `.harness/state/lease-summary.json`
  - writer: `internal/lease`
  - reader: runtime, checkpoint, query
  - rule: one active lease per task/dispatch
- `.harness/state/session-registry.json`
  - writer: runtime session ingest
  - reader: route resume selection, query
  - rule: upsert by `sessionId` and active binding by `taskId`
- `.harness/state/runtime.json`
  - writer: `internal/runtime`
  - reader: daemon and operator status
  - rule: latest snapshot wins via CAS
- `.harness/state/verification-summary.json`
  - writer: runtime verify ingest
  - reader: query and control gating
  - rule: one latest entry per task
- `.harness/state/tmux-summary.json`
  - writer: `internal/tmux`
  - reader: query, control attach/status
  - rule: one latest session record per task
- `.harness/checkpoints/*`
  - writer: burst/checkpoint ingest
  - reader: resume and audits
  - rule: immutable per attempt plus latest outcome
- `.harness/artifacts/*`
  - writer: worker burst
  - reader: verify, review, audits, resume
  - rule: task-local per dispatch

Derived:

- `.harness/state/completion-gate.json`
  - writer: `internal/verify`
  - reader: archive/control
- `.harness/state/guard-state.json`
  - writer: `internal/verify`
  - reader: archive/control/operator status

## Execution Stages

1. bootstrap repo-local `.harness`
2. submit request into queue and task pool
3. route task and attach policy tags
4. issue dispatch ticket
5. claim lease
6. prepare worker bundle
7. create tmux session and run native codex
8. ingest checkpoint and outcome
9. ingest verify
10. expose query/control state

## Control Actions

Current Go control actions:

- `task status`
- `task attach`
- `task restart-from-stage`
- `task stop`
- `task archive`

Archive respects the runtime completion gate. A passed status alone is not enough.
