# Guard Loop

The public UX is not the scheduler.

The repo-local runtime is the guard loop:

```text
submit
-> classify
-> fuse
-> derive/update todo
-> safety checks
-> route
-> dispatch / recover / resume
-> verify
-> checkpoint / merge / archive when safe
-> refresh summaries
-> next tick
```

## Rules

- triggers only wake the guard
- the guard owns safety boundaries, not `tmux`
- the guard may converge stale repo-local tmux sessions before dispatch
- execution is blocked when the control plane is unsure
- summary state is the default operator surface
- semantic success requires gate-aligned evidence, not only exit code

## Safety Boundary

Before non-interactive execution the guard checks:

- repo and git state are usable
- conflicting live execution is not already in progress
- unknown dirty worktree state is absent
- actionable todo still exists
- completion gate is not already satisfied
- worktree / merge state is coherent

Operator-initiated `harness-runner run` / `recover` may override automation-only dirty blockers,
but they do not override task drift, failed verification, satisfied completion gate, or superseded-task blockers.

Current implementation note:

- `lockHealthy` is currently derived from daemon/runtime health plus conflicting-execution checks
- it is not yet a dedicated on-disk lease ledger

## Key State

- `.harness/state/guard-state.json`
- `.harness/state/todo-summary.json`
- `.harness/state/completion-gate.json`
- `.harness/state/runtime.json`
