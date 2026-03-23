# Context Rot And Drift Guards

Long-running sessions accumulate history, appended changes, failures, and stale summaries.
Klein-Harness checks that deterministically before dispatch, resume, and finalize.

## Context Rot

Context rot is scored from bounded signals such as:

- resume count
- session age
- appended change count
- divergence from latest plan epoch
- unresolved failure count
- stale summary age
- missing compression checkpoints

When rot exceeds policy thresholds, the runtime should prefer:

- checkpoint current state
- refresh compact summaries
- fresh execution session over blind resume

## Drift Checklists

Pre-dispatch / pre-resume:

- task still on latest valid epoch
- task not superseded
- checkpoint not required
- owned paths still valid
- no appended requirement invalidates the task
- blockers are not unresolved beyond policy
- resume is safe for the task/session relationship

Pre-finalize:

- execution still matches current acceptance
- task stayed inside owned paths
- no newer appended requirement invalidates the output
- verification aligns with latest epoch
- diff summary and task summary still agree

## Machine Surfaces

Warnings are surfaced in bounded hot state:

- `.harness/state/thread-state.json`
- `.harness/state/change-summary.json`
- `.harness/state/runtime.json`
- `.harness/state/task-summary.json`
- `.harness/state/worker-summary.json`

These warnings are operator-facing and model-sparing.
Raw logs stay available as cold evidence, but they are not the default decision surface.
