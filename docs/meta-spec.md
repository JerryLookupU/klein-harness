# Harness Meta Spec

This document captures the MVP runtime policy above the implementation.

## Scope

- `harness submit` is the only human-originating write path
- `harness daemon run-once` and `harness daemon loop` are automation paths
- `harness control` is the operator control path
- native `tmux` is an execution backend, not the scheduler

## Authoritative State

- request intake: `.harness/requests/queue.jsonl`
- task truth: `.harness/task-pool.json`
- dispatch truth: `.harness/state/dispatch-summary.json`
- lease truth: `.harness/state/lease-summary.json`
- session truth: `.harness/state/session-registry.json`
- runtime truth: `.harness/state/runtime.json`
- verification truth: `.harness/state/verification-summary.json`
- tmux truth: `.harness/state/tmux-summary.json`
- completion truth: `.harness/state/completion-gate.json`
- guard truth: `.harness/state/guard-state.json`

## Automation Policy

Automation may change code only when:

- the task is actionable
- the completion gate is open
- route is dispatch-ready
- no conflicting live lease exists
- owned path constraints are coherent

Automation paths:

- `harness daemon run-once`
- `harness daemon loop`

Operator actions:

- `harness control ... restart-from-stage`
- `harness control ... stop`
- `harness control ... archive`

## Done Means Closed Loop

A task is truly done only when:

- execution finished
- verification passed
- completion evidence exists
- review evidence exists when required
- the runtime completion gate is satisfied

Archive is allowed only after that gate is satisfied.

See [docs/runtime-mvp.md](/Users/linzhenjie/code/claw-code/harness-architect/docs/runtime-mvp.md) for the concrete implementation shape.
