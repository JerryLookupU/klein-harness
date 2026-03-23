# Single-Entry Intake

Klein-Harness keeps one human write path:

```bash
harness-submit <ROOT> --goal "<TEXT>" [options...]
```

`--kind` may still be passed, but it is only a hint.
Operators do not need to choose between append, check, replan, duplicate, or extra-info commands.

## External Simplicity, Internal Normalization

Every submission is appended to `.harness/requests/queue.jsonl`.
The append-only line is not rewritten.

Runtime then normalizes the submission into mutable ledgers and hot summaries with deterministic-first fields such as:

- `normalizedIntentClass`
- `fusionDecision`
- `threadKey`
- `targetThreadKey`
- `targetPlanEpoch`
- `idempotencyKey`
- `canonicalGoalHash`
- `evidenceFingerprint`
- `duplicateOfRequestId`
- `mergedIntoRequestId`
- `compoundGroupId`
- `classificationReason`

## Intent Classes

- `duplicate_or_noop`
- `context_enrichment`
- `inspection`
- `append_change`
- `fresh_work`
- `compound_split`
- `ambiguous_needs_orchestrator`

These are runtime-internal classes.
They are not separate public commands.

## Front-Door Triage

Before the heavier control loop work starts, Klein-Harness also keeps a cheap deterministic front-door classification for CLI ergonomics:

- `conversational_help`
- `advisory_read_only`
- `inspection`
- `work_order`
- `duplicate_or_context`

This triage does not add more public write commands.
It only helps the runtime stay soft at the front door and hard in the control loop.

## Fusion Decisions

- `accepted_new_thread`
- `accepted_existing_thread`
- `duplicate_of_existing`
- `merged_as_context`
- `inspection_overlay`
- `append_requires_replan`
- `compound_split_created`
- `noop`

## Idempotent Effects

Requests stay append-only.
Effects should still be idempotent.

Deterministic-first rules:

- same thread + same idempotency key => duplicate or noop in effect
- same canonical goal + same evidence fingerprint => duplicate in effect
- same canonical goal + new context/evidence => merge as context when safe
- same thread + changed behavior/acceptance => append change with selective replan

## State Surfaces

Primary hot summaries for intake:

- `.harness/state/intake-summary.json`
- `.harness/state/thread-state.json`
- `.harness/state/change-summary.json`
- `.harness/state/request-summary.json`
- `.harness/state/progress.json`

These are the machine-first read surface for operator tools.
