# Runtime Request Spec

## Goal

Unify all upstream entry points into one repo-local runtime:

- `OpenClaw`
- `shell`
- `cron`
- future external callers

All of them submit requests.
Only the project runtime decides binding, routing, dispatch, recovery, verification, RCA allocation, follow-up requests, and reporting.

## Entry Model

Canonical public commands:

- `harness-submit <ROOT> --goal <TEXT> [options...]`
- `harness-tasks <ROOT> [summary|queue|tasks|requests|workers|daemon|blockers|logs]`
- `harness-task <ROOT> <TASK_ID|REQUEST_ID> [detail|logs]`
- `harness-control <ROOT> <daemon|task|request|project> [args...]`

Compatibility shims still exist, but they are not the primary UX:

- `harness-init`
- `harness-bootstrap`
- `harness-report`
- `harness-kick`

Project-local expert surfaces under `.harness/bin`:

- `harness-submit`
- `harness-tasks`
- `harness-task`
- `harness-control`
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

`harness-submit` stays the single human-originating write path.
`--kind` remains supported, but it is only a hint.
The runtime decides deterministic-first classification, fusion, thread correlation, selective replan, and dispatch impact.

Important boundary:

- `harness-submit` can auto-initialize the `.harness` scaffold for an uninitialized repo
- it does not pretend to be a one-shot full bootstrap wizard
- planning / routing / execution still happen through the running control loop after submit

The front door stays soft:

- humans do not choose between append / check / replan / duplicate commands
- cheap deterministic front-door triage distinguishes conversational, advisory, inspection, work-order, and duplicate/context interactions
- heavy runtime classification and fusion still happen inside the control loop

The guard stays hard:

- todo is derived from facts, not hand-maintained
- completion gate is separate from todo and separate from blueprint source docs
- unknown dirty worktrees block non-interactive execution
- only managed dirty worktrees can become checkpoint-eligible state

## Single-Entry Intake

Every submission remains append-only in `.harness/requests/queue.jsonl`, but its effect is normalized in mutable ledgers and hot summaries.

Deterministic-first internal intent classes:

- `duplicate_or_noop`
- `context_enrichment`
- `inspection`
- `append_change`
- `fresh_work`
- `compound_split`
- `ambiguous_needs_orchestrator`

Deterministic-first fusion decisions:

- `accepted_new_thread`
- `accepted_existing_thread`
- `duplicate_of_existing`
- `merged_as_context`
- `inspection_overlay`
- `append_requires_replan`
- `compound_split_created`
- `noop`

Important distinction:

- requests are append-only events
- effects are expected to be idempotent
- same thread + same idempotency key should not create duplicate effective work
- same thread + new evidence usually merges as context instead of spawning a full new implementation plan

## Request Lifecycle

Append-only intake lives in `.harness/requests/queue.jsonl`.
The queue entry is never rewritten.
Lifecycle is tracked in `.harness/state/request-index.json`, `.harness/state/request-task-map.json`, and `.harness/lineage.jsonl`.

Supported request states:

- `queued -> bound -> dispatched -> running -> verified -> completed`
- `queued -> blocked`
- `queued -> cancelled`
- `running -> recoverable -> resumed`

For code-bearing tasks the runtime may also materialize task/merge lifecycle stages through task status and merge ledgers:

- `queued -> worktree_prepared -> dispatched -> running -> verified -> merge_queued -> merge_checked -> merged -> completed`
- conflict path: `verified -> merge_queued -> merge_conflict -> merge_resolution_requested`

Notes:

- `bound` means runtime chose at least one task and wrote an explicit binding artifact.
- `dispatched` means routing passed and runner wrote dispatch evidence, even in `--dispatch-mode print`.
- `running` is driven by runner heartbeat, not by model self-report.
- `verified` is written by `harness-verify-task`.
- `worktree_prepared` means the runtime prepared the task branch/worktree binding before execution.
- `merge_queued` / `merge_checked` / `merged` are runtime-controlled local integration stages; workers do not choose these transitions themselves.
- `completed` closes the request loop once all bound work is verified.
- `recoverable` means the runtime has enough lineage/session state to resume or replan.

The runtime remains live while new submissions arrive.
Fresh bootstrap is not required for appended requirements.

## Scaffold Init

The compatibility helper `harness-init` creates the minimal operator/runtime skeleton without invoking a model.
In the canonical flow, `harness-submit` can trigger the same scaffold creation automatically.

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
- `.harness/state/root-cause-summary.json`
- `.harness/state/worktree-registry.json`
- `.harness/state/merge-queue.json`
- `.harness/state/merge-summary.json`
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
- `.harness/root-cause-log.jsonl`

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

Thread-aware execution metadata should remain explicit when available:

- `threadKey`
- `targetPlanEpoch`
- `impactClassification`
- `duplicateOfRequestId`
- `mergedIntoRequestId`

## RCA Allocation

Bug and feedback handling keeps symptom evidence and RCA decisions separate:

- `.harness/feedback-log.jsonl` stores symptoms / events
- `.harness/root-cause-log.jsonl` stores RCA decisions

Supported request kinds now include:

- `implementation`
- `analysis`
- `research`
- `status`
- `audit`
- `replan`
- `stop`
- `bug`
- `feedback`
- `rca`

RCA uses a fixed taxonomy:

- `spec_acceptance`
- `blueprint_decomposition`
- `routing_session`
- `execution_change`
- `verification_guardrail`
- `runtime_tooling`
- `environment_dependency`
- `merge_handoff`
- `underdetermined`

Before emitting repair, runtime correlates bug / feedback to:

```text
requestId -> taskId -> sessionId -> worktreePath -> verificationResultPath
```

If correlation is weak, RCA stays `underdetermined` and runtime emits `audit` / `research` instead of blind repair.

## Route First, Dispatch Second

`harness-route-session` is the pre-worker gate.

Runtime contract:

1. reconcile queued requests to tasks
2. run route gate and persist the decision
3. only dispatch when route output says `dispatchReady=true`
4. write claim/session binding explicitly
5. then launch or preview execution

The execution model never decides `fresh` vs `resume` on its own.

Pre-dispatch / pre-resume checks are deterministic-first:

- task still on latest valid plan epoch
- queued task not superseded
- checkpoint not required
- owned paths still valid
- new appended requirements have not invalidated the task
- resume is safe for the task/session relationship
- compact handoff and verification state are not stale beyond policy thresholds

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
- `.harness/state/root-cause-summary.json`
- `.harness/state/request-summary.json`
- `.harness/state/intake-summary.json`
- `.harness/state/thread-state.json`
- `.harness/state/change-summary.json`
- `.harness/state/lineage-index.json`

`refresh-state.py` refreshes these from:

- progress / spec / work-items / task-pool
- request index + request-task map
- session registry
- runner state + heartbeat
- feedback log
- root-cause log
- lineage log

Operator tools should prefer hot state first and degrade gracefully to the source ledgers.

`progress.md` is a human projection rendered from `.harness/state/progress.json`.
Machine-facing tools should prefer JSON summaries over Markdown parsing.

## Re-entrant Follow-Ups

Runtime findings can emit repo-local follow-up requests back into the request queue.

Current minimal mechanism:

- verification failure emits `replan`
- blocked session/path conflicts emit `replan` or `stop`
- verified merge-required work can emit `audit`
- RCA allocation can emit repair work such as `implementation`, `replan`, `stop`, `audit`, or `research`

These follow-up requests:

- are appended to `.harness/requests/queue.jsonl`
- appear in `request-index.json`
- update `{kind}-requests.json` snapshots such as `audit-requests.json`, `replan-requests.json`, and `stop-requests.json`
- get lineage events in `.harness/lineage.jsonl`
- remain scriptable and repo-local

## Report Surface

The compatibility helper `harness-report` reads hot state first and summarizes:

- request counts
- selected / active request
- bound task
- bound session
- worktree path
- verification summary
- root-cause summary
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
9. bug / feedback intake
10. RCA allocate -> repair emit
11. verify repair
12. refresh-state
13. report

The smoke also checks that follow-up requests, RCA summaries, and lineage summaries are written.
