# Runtime Request Spec

## Goal

Unify all upstream entry points into one repo-local runtime:

- `OpenClaw`
- `shell`
- `cron`
- future external callers

All of them submit requests.
Only the project runtime decides binding, routing, dispatch, recovery, verification, follow-up requests, and reporting.

## Entry Model

Global entry commands:

- `harness-init <ROOT>`
- `harness-bootstrap <ROOT> <GOAL> [STACK_HINT] [kick options...]`
- `harness-submit <ROOT> --kind <KIND> --goal <TEXT> [options...]`
- `harness-report <ROOT> [--request-id <ID>] [--format text|json]`

Project-local entry commands under `.harness/bin`:

- `harness-submit`
- `harness-report`
- `harness-runner`
- `harness-status`
- `harness-query`
- `harness-dashboard`
- `harness-watch`
- `harness-route-session`
- `harness-verify-task`

Repo-local Python control surface:

- `.harness/scripts/request.py reconcile --root <ROOT>`
- `.harness/scripts/request.py cancel --root <ROOT> --request-id <ID>`
- `.harness/scripts/refresh-state.py <ROOT>`

## Request Lifecycle

Append-only intake lives in `.harness/requests/queue.jsonl`.
The queue entry is never rewritten.
Lifecycle is tracked in `.harness/state/request-index.json`, `.harness/state/request-task-map.json`, and `.harness/lineage.jsonl`.

Supported request states:

- `queued -> bound -> dispatched -> running -> verified -> completed`
- `queued -> blocked`
- `queued -> cancelled`
- `running -> recoverable -> resumed`

Notes:

- `bound` means runtime chose at least one task and wrote an explicit binding artifact.
- `dispatched` means routing passed and runner wrote dispatch evidence, even in `--dispatch-mode print`.
- `running` is driven by runner heartbeat, not by model self-report.
- `verified` is written by `harness-verify-task`.
- `completed` closes the request loop once all bound work is verified.
- `recoverable` means the runtime has enough lineage/session state to resume or replan.

## Init

`harness-init` creates the minimal operator/runtime skeleton without invoking a model.

Creates at least:

- `.harness/bin/*`
- `.harness/scripts/*`
- `.harness/state/*`
- `.harness/requests/queue.jsonl`
- `.harness/requests/archive/`
- `.harness/state/request-index.json`
- `.harness/state/request-task-map.json`
- `.harness/state/request-summary.json`
- `.harness/state/lineage-index.json`
- `.harness/lineage.jsonl`
- `.harness/session-registry.json`
- `.harness/project-meta.json`

All structured snapshot files include:

- `schemaVersion`
- `generator`
- `generatedAt`

All append-only event logs follow append-only semantics:

- `.harness/requests/queue.jsonl`
- `.harness/lineage.jsonl`
- `.harness/feedback-log.jsonl`

## Binding

`request.py reconcile` and `harness-runner` both use the same repo-local binding model.

Binding rules:

- do not mutate the append-only intake line
- write machine-readable bindings into `.harness/state/request-task-map.json`
- mirror request status in `.harness/state/request-index.json`
- append a lineage event for each meaningful transition
- prefer explicit reasons over implicit prompt interpretation

Each binding records at least:

- `bindingId`
- `requestId`
- `taskId`
- `status`
- `bindingReason`
- `createdAt`
- `updatedAt`
- `lineage.sessionId`
- `lineage.worktreePath`
- `lineage.diffBase`
- `lineage.diffSummary`
- `lineage.verificationStatus`
- `lineage.verificationResultPath`
- `history[]`

## Route First, Dispatch Second

`harness-route-session` is the pre-worker gate.

Runtime contract:

1. reconcile queued requests to tasks
2. run route gate and persist the decision
3. only dispatch when route output says `dispatchReady=true`
4. write claim/session binding explicitly
5. then launch or preview execution

The execution model never decides `fresh` vs `resume` on its own.

## Anti-Self-Intersection Rules

- sibling concurrent workers must not resume the same active session
- `session-registry.activeBindings` is the shared truth for active session ownership
- `ownedPaths`, `worktreePath`, `diffBase`, `claim.boundSessionId`, and request binding stay explicit
- blocked routes emit auditable evidence instead of vague prompt text

## Hot State

Primary machine-readable hot path:

- `.harness/state/current.json`
- `.harness/state/runtime.json`
- `.harness/state/blueprint-index.json`
- `.harness/state/feedback-summary.json`
- `.harness/state/request-summary.json`
- `.harness/state/lineage-index.json`

`refresh-state.py` refreshes these from:

- progress / spec / work-items / task-pool
- request index + request-task map
- session registry
- runner state + heartbeat
- feedback log
- lineage log

Operator tools should prefer hot state first and degrade gracefully to the source ledgers.

## Re-entrant Follow-Ups

Runtime findings can emit repo-local follow-up requests back into the request queue.

Current minimal mechanism:

- verification failure emits `replan`
- blocked session/path conflicts emit `replan` or `stop`
- verified merge-required work can emit `audit`

These follow-up requests:

- are appended to `.harness/requests/queue.jsonl`
- appear in `request-index.json`
- update `{kind}-requests.json` snapshots such as `audit-requests.json`, `replan-requests.json`, and `stop-requests.json`
- get lineage events in `.harness/lineage.jsonl`
- remain scriptable and repo-local

## Report Surface

`harness-report` reads hot state first and summarizes:

- request counts
- selected / active request
- bound task
- bound session
- worktree path
- verification summary
- current runtime focus
- active / recoverable / stale runners
- lineage event count

## Smoke Path

The release smoke proves:

1. install
2. init
3. submit
4. reconcile / bind
5. route
6. runner print dispatch evidence
7. recover / resume
8. verify
9. refresh-state
10. report

The smoke also checks that follow-up requests and lineage summaries are written.
