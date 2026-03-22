# Changelog

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
