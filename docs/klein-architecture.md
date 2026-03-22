# Klein Architecture

## What “Klein” Means Here

This repo treats the harness as a re-entrant control surface.
There is no durable “outside” that submits requests and no separate “inside” that holds the real state in a prompt.

Everything that matters is repo-local, machine-readable, recoverable, and able to become the next request.

## Closed-Loop State Machine

Request intake is append-only:

- `.harness/requests/queue.jsonl`

Lifecycle is closed over repo-local snapshots and event logs:

- `.harness/state/request-index.json`
- `.harness/state/request-task-map.json`
- `.harness/lineage.jsonl`

Current request runtime states:

- `queued`
- `bound`
- `dispatched`
- `running`
- `verified`
- `completed`
- `blocked`
- `cancelled`
- `recoverable`
- `resumed`

Typical loop:

```text
submit
-> reconcile
-> bind request to task
-> route session
-> dispatch
-> running heartbeat
-> verify
-> completed
-> optional follow-up request re-enters queue
```

## Lineage Dimensions

The harness avoids self-intersection by adding explicit dimensions instead of letting prompts blur them together.

Primary lineage chain:

```text
requestId
-> taskId
-> sessionId
-> worktreePath
-> diffSummary
-> verification
-> outcome
```

Repo-local artifacts:

- `.harness/state/request-task-map.json`
- `.harness/session-registry.json`
- `.harness/lineage.jsonl`
- `.harness/state/lineage-index.json`

Why the extra dimensions matter:

- `request lineage` says why the runtime is doing work
- `task lineage` says what concrete unit is being executed
- `session lineage` says which context can safely resume
- `worktree lineage` says where code isolation lives
- `verification lineage` says whether the outcome was checked

## Anti-Self-Intersection Rules

The repo resolves apparent conflicts by dimensional separation:

- route first, dispatch second
- sibling concurrent workers must not resume the same active session
- `session-registry.activeBindings` is the shared gate for session ownership
- `ownedPaths` and `worktreePath` stay explicit on the task, not implicit in prompts
- `diffBase` stays explicit so audit and merge can compare against a stable line
- blocked or ambiguous routes become machine-readable binding or follow-up evidence, not hand-wavy prompt text

This is why a sibling worker does not “just continue the same session” even if the code area looks related.
The runtime must prove the reuse is safe in the current dimensions.

## Hot State

The single shared surface for human, operator, agent, and runtime is the hot-state layer:

- `.harness/state/current.json`
- `.harness/state/runtime.json`
- `.harness/state/blueprint-index.json`
- `.harness/state/feedback-summary.json`
- `.harness/state/request-summary.json`
- `.harness/state/lineage-index.json`

Rules:

- humans can still read `progress.md`
- scripts should prefer hot state first
- if hot state is missing, tools degrade to the source ledgers
- hot state is derived, not authoritative by itself

## Re-entrant Outputs

A report or failure becomes the next request by being written back into the same repo-local queue.

Current minimal re-entry paths:

- verification failure emits a `replan` request
- blocked route with session/path conflict emits `replan` or `stop`
- verified merge-required work can emit an `audit` request

Those follow-ups are:

- appended to `.harness/requests/queue.jsonl`
- indexed in `.harness/state/request-index.json`
- mirrored into `{kind}-requests.json` snapshots such as `audit-requests.json`, `replan-requests.json`, and `stop-requests.json`
- visible in `.harness/state/request-summary.json`
- connected through `.harness/lineage.jsonl`

That makes the output of one runtime pass a valid input to the next pass without leaving the repo.

## Compatibility

The CLI surface stays stable:

- `harness-init`
- `harness-bootstrap`
- `harness-submit`
- `harness-report`
- `harness-kick`

The upgrade is architectural, not a rename exercise.
Existing entry names stay intact while the runtime underneath becomes request-aware, lineage-aware, and re-entrant.
