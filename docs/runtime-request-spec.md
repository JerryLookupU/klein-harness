# Runtime Request Spec

## Goal

All upstream callers submit into one repo-local runtime.
Go owns binding, routing, dispatch, lease, execution, verify, and control.

## Canonical Entry Model

Canonical commands:

- `harness init <ROOT>`
- `harness submit <ROOT> --goal <TEXT> [--kind <HINT>] [--context <PATH> ...]`
- `harness tasks <ROOT>`
- `harness task <ROOT> <TASK_ID>`
- `harness control <ROOT> task <TASK_ID> <status|attach|restart-from-stage|stop|archive>`
- `harness daemon run-once <ROOT>`
- `harness daemon loop <ROOT>`

Compatibility wrappers:

- `harness-init`
- `harness-submit`
- `harness-tasks`
- `harness-task`
- `harness-control`

Wrappers delegate into the Go CLI. They are not authoritative runtime implementations.

## Request Model

`harness submit` is the single human-originating write path.

- requests append to `.harness/requests/queue.jsonl`
- tasks are materialized in `.harness/task-pool.json`
- the runtime decides route / dispatch / resume behavior
- policy tags are attached through route `reasonCodes`

## Scaffold Init

`harness init` creates the minimal runtime scaffold.
`harness submit` can auto-bootstrap the same scaffold when needed.

Minimum runtime state includes:

- `.harness/requests/queue.jsonl`
- `.harness/task-pool.json`
- `.harness/state/runtime.json`
- `.harness/state/dispatch-summary.json`
- `.harness/state/lease-summary.json`
- `.harness/state/session-registry.json`
- `.harness/state/verification-summary.json`
- `.harness/state/tmux-summary.json`
- `.harness/checkpoints/*`
- `.harness/artifacts/*`

## Execution Contract

The runtime loop is:

1. submit
2. route
3. dispatch
4. lease claim
5. burst in real tmux
6. outcome/checkpoint ingest
7. verify ingest
8. query/control exposure

Execution is native:

- fresh: `codex exec`
- resume: `codex exec resume <SESSION_ID>`
- shell is not a parallel runtime anymore
- Python control paths are not canonical

## Completion Contract

- `verification.completed` does not imply completion by itself
- completion requires evidence-backed completion gate satisfaction
- review-required tasks require review evidence before completion
- archive respects the same gate

For the current state model, see [docs/runtime-mvp.md](/Users/linzhenjie/code/claw-code/harness-architect/docs/runtime-mvp.md).
