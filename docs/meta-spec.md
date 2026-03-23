# Harness Meta Spec

This document defines the operating policy above the runtime implementation.

It answers five control-plane questions:

1. what counts as actionable todo
2. when automation is allowed to change code
3. who is allowed to commit or push
4. how failure, timeout, and dirty state are handled
5. how the system decides work is truly done

## Scope

This is a repo-local execution spec for Klein-Harness.

- `harness-submit` is the only human-originating write path for new work.
- `harness-runner tick` and `daemon` are automation paths.
- `harness-runner run` and `recover` are explicit operator actions.
- `tmux` is only the current worker-node backend. It is not the scheduler.

Tmux convergence policy:

- at most one live daemon session per repo
- exited worker sessions are not retained as operator-visible state
- `tick`, `daemon`, and `daemon-stop` may garbage-collect stale repo-local `hr-*` sessions

## Authoritative State

The meta-spec is enforced through these ledgers and hot summaries:

- request intake: `.harness/requests/queue.jsonl`
- task truth: `.harness/task-pool.json`
- runner truth: `.harness/state/runner-state.json`
- guard truth: `.harness/state/guard-state.json`
- worktree truth: `.harness/state/worktree-registry.json`
- completion truth: `.harness/state/completion-gate.json`

Markdown views are operator surfaces, not machine truth.

## 1. Actionable Todo

Something counts as actionable todo only when all of the following are true:

- it is represented by a task in `.harness/task-pool.json`
- the task is not terminal, superseded, or archived
- the task still belongs to the current plan epoch / request thread
- the completion gate is not already satisfied or retired

Actionable task states are:

- `queued`
- `bound`
- `worktree_prepared`
- `dispatched`
- `running`
- `resumed`
- `recoverable`
- `verified`
- `merge_queued`
- `merge_checked`
- `merge_conflict`
- `merge_resolution_requested`

A task is not actionable when:

- it is in a terminal state such as `completed`, `merged`, `pass`, `verified` with closed loop
- it is superseded by newer appended requirements
- it is blocked by unmet dependencies
- it requires a checkpoint before further execution

## 2. When Automation May Change Code

Non-interactive automation may change code only when the guard is fully green.

That means:

- repo root is a usable git repository
- completion gate is still open
- there is actionable todo
- no conflicting live execution exists
- unknown dirty worktree state is absent
- worktree / merge state is coherent enough for automation
- route decision is `dispatchReady = true`
- task drift / context-rot checks pass

The automation paths are:

- `harness-runner tick`
- `harness-runner daemon`
- any upstream trigger that only wakes the runner

Operator-initiated runs are narrower and may override automation-only dirty blockers:

- `harness-runner run`
- `harness-runner recover`

Current policy:

- explicit `run/recover` may bypass `unknown dirty worktree blocks automation`
- explicit `run/recover` may not bypass task drift, failed verification, missing ownership, satisfied completion gate, or superseded-task blockers
- environment degradation such as `UNBORN_HEAD` or deferred worktree creation is treated as degraded runtime state, not as task failure

## 3. Who Commits Or Pushes

Workers may edit code inside their assigned execution scope, but they do not own final integration.

- worker execution may modify only task-owned paths
- worker execution may produce branch-local commits when the task uses a dedicated worktree
- worker execution must not push directly as part of the default runtime loop
- local integration is serialized through the runtime / orchestrator against the `integrationBranch`
- remote push is an explicit operator or orchestrator action after local merge / audit policy is satisfied

Default ownership:

- worker: implement and verify inside the assigned lane
- runtime / orchestrator: checkpoint, merge queue, local integration, archive
- operator: remote push / publish / out-of-repo side effects unless a future project-specific policy says otherwise

## 4. Failures, Timeouts, Dirty State

### Failure classes

- dispatch failure: runner or backend exits before a valid task result exists
- verification failure: verification rules fail or are missing
- finalize guard failure: finalize sees drift / stale conditions that invalidate closure
- environment degradation: repo or worktree environment is incomplete, but the task itself is not blamed

### Timeout and stale execution

- stale or dead backend session converts the task to `recoverable`
- runner exit with non-zero code converts the task to `recoverable`
- timeout is treated as a recoverable execution failure unless policy marks it otherwise

### Dirty state

- unknown dirty state blocks non-interactive automation
- managed dirty state becomes checkpoint-eligible, not silently absorbed
- repo-root dirty state is tracked once at repo scope; root-bound queued tasks should not create duplicate unknown worktree entries
- degraded worktree preparation caused by repo environment is surfaced as warning/degraded state, not as a hard incoherence blocker by itself

### Worktree and unborn-head handling

- missing dedicated worktree with valid git baseline may still be recoverable by later preparation
- `UNBORN_HEAD` means diff/worktree preparation is degraded until the repo has an initial commit
- diff summary in that state records `degraded` instead of failing the task

## 5. Done Means Closed Loop, Not Just Exit Code

A task is truly done only when:

- execution stopped without invalidating guard checks
- verification passed, or was explicitly skipped by policy
- required local integration / merge queue work is complete
- compact handoff and lineage are written
- request bindings for the task have reached a terminal closed-loop state

Project-level completion is stricter:

- no actionable todo remains
- no open merge conflict remains
- no required audit verdict is missing
- completion gate is satisfied or retired

So:

- runner exit code alone is never enough
- verification alone is not enough when merge is still pending
- a task can be `recoverable` without being a harness bug
- degraded environment state must remain visible, but it must not be confused with task failure
