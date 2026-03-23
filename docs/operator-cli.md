# Operator CLI

`harness-ops` is the project-local machine-first operator facade behind the public 4-command UX.

Canonical public wrappers:

- `harness-submit`
- `harness-tasks`
- `harness-task`
- `harness-control`

`harness-ops` remains the expert/project-local surface when you want the raw operator subcommands directly.

Path:

- `.harness/bin/harness-ops`

It reads bounded JSON summaries first and avoids model calls.

## Key commands

```bash
.harness/bin/harness-ops . top
.harness/bin/harness-ops . queue
.harness/bin/harness-ops . tasks
.harness/bin/harness-ops . task T-003
.harness/bin/harness-ops . request R-001
.harness/bin/harness-ops . workers
.harness/bin/harness-ops . daemon status
.harness/bin/harness-ops . blockers
.harness/bin/harness-ops . logs
.harness/bin/harness-ops . doctor
.harness/bin/harness-ops . watch --view top --interval 5
```

## What it reads

Primary inputs:

- `state/progress.json`
- `state/queue-summary.json`
- `state/task-summary.json`
- `state/worker-summary.json`
- `state/daemon-summary.json`
- `state/request-summary.json`
- `state/lineage-index.json`
- `state/log-index.json`
- `state/policy-summary.json`

## Health model

`harness-ops` separates:

- runtime health
- worker-node health
- dispatch backend state

This prevents tmux liveness from being confused with scheduler health.

## Daemon controls

`harness-ops daemon` wraps the existing runner daemon surface:

- `status`
- `start`
- `stop`
- `restart`

The daemon remains repo-local. Upstream triggers such as OpenClaw, shell, or cron only ask the runtime to tick; they are not the scheduler.
