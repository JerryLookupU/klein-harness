# Changelog

## v0.2.1 - Unreleased

Post-release cleanup and UX compression on top of `v0.2`.

### Highlights

- Compressed the canonical public command surface to:
  - `harness-submit`
  - `harness-tasks`
  - `harness-task`
  - `harness-control`
- Kept older helpers as compatibility shims instead of removing them.
- Aligned wrapper help, install output, README, and examples so `harness-submit` is clearly the single canonical write path and `--kind` is only an optional hint.
- Added lightweight front-door triage metadata for submission ergonomics:
  - `conversational_help`
  - `advisory_read_only`
  - `inspection`
  - `work_order`
  - `duplicate_or_context`
- Added canonical local wrappers under `.harness/bin` for:
  - `harness-tasks`
  - `harness-task`
  - `harness-control`
- Added task/request control helpers for checkpoint, archive, stop, restart staging, and request cancel flows.
- Updated release smoke to cover the compressed 4-command UX while preserving runner/worktree/merge/RCA compatibility checks.

## v0.2 - 2026-03-23

Published the machine-first, progressive, merge-aware control-plane release tagged at `f92a08c`.

### Highlights

- Added machine-first hot summaries and deterministic operator surfaces:
  - `progress.json`
  - `queue-summary.json`
  - `task-summary.json`
  - `worker-summary.json`
  - `daemon-summary.json`
- Added `harness-ops` as a machine-first operator facade.
- Added compact log summaries plus targeted raw log retrieval.
- Added single-entry intake classification, request fusion, thread-aware epochs, context-rot guards, and drift checklists.
- Added worktree-first execution with local merge queue, merge preview, and merge-conflict follow-up handling.
- Preserved append-only requests and repo-local lineage / RCA surfaces.

## v0.1.0 - 2026-03-22

First publishable release of the `klein-harness` Codex-first `.harness/` collaboration toolkit.

### Highlights

- Added structured failure memory with:
  - `.harness/feedback-log.jsonl`
  - `.harness/state/feedback-summary.json`
- Added program-first pre-worker routing:
  - program gate decides `claimable / blocked / orchestrator_review`
  - `gpt-5.4` is now fallback for ambiguous routing and replanning
- Added route decision outputs such as:
  - `routingMode`
  - `needsOrchestrator`
  - `dispatchReady`
  - `promptStages`
- Updated worker and orchestrator prompt examples to consume recent high-severity failures instead of scanning full history
- Added runner support for `--dispatch-mode print` so release validation can run without `tmux`
- Added release smoke script:
  - `skills/klein-harness/examples/harness-release-smoke.example.sh`
- Updated install flow to initialize:
  - `feedback-log.jsonl`
  - `state/feedback-summary.json`
  - `state/runner-state.json`
  - `state/runner-heartbeats.json`

### Validation

Validated with:

```bash
python3 -m py_compile skills/klein-harness/examples/*.py
bash ./skills/klein-harness/examples/harness-release-smoke.example.sh
```
