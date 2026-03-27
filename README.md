# Klein-Harness

![Klein-Harness Surface](docs/klein-surface-hero.png)

Klein is a repo-local agent runtime built for predictable, auditable execution.

- Native `tmux` is the execution shell.
- Native `codex exec` / `codex exec resume` is the worker runner.
- Go owns bootstrap, routing, dispatch, lease, checkpoint, outcome, verify, query, and control.
- Shell scripts are compatibility wrappers, not the runtime source of truth.

## Quick links

- Architecture docs: [docs/klein-architecture.md](docs/klein-architecture.md)
- Runtime details: [docs/runtime-mvp.md](docs/runtime-mvp.md)
- CLI surface: [docs/four-command-surface.md](docs/four-command-surface.md)
- Guardrails: [docs/guard-loop.md](docs/guard-loop.md)
- Migration notes: [docs/refactor-runtime-migration.md](docs/refactor-runtime-migration.md)

## What is in this repo

Canonical implementation:

- CLI: `cmd/harness`
- Runtime: `internal/runtime`
- Bootstrap: `internal/bootstrap`
- Routing: `internal/route`
- Dispatch: `internal/dispatch`
- Lease: `internal/lease`
- Verify: `internal/verify`
- Query: `internal/query`
- Executor: `internal/executor/codex` and `internal/tmux`

Compatibility paths still exist and delegate into the canonical runtime:

- `scripts/harness-*.sh`
- `cmd/kh-codex`
- `cmd/kh-orchestrator`
- `cmd/kh-worker-supervisor`

## Install

```bash
./install.sh --force
```

Install effects:

- Installs skills into `$CODEX_HOME/skills`.
- Installs the canonical `harness` CLI into `$CODEX_HOME/bin` when Go is available.
- Installs compatibility wrappers such as `harness-submit` and `harness-control`.
- Updates managed block in `$CODEX_HOME/AGENTS.md` without changing non-managed user content.
- Updates managed profiles in `$CODEX_HOME/config.toml` without overriding custom user profiles.

Installed skills include:

- `klein-harness`
- `blueprint-architect`
- `qiushi-execution`
- `systematic-debugging`
- `harness-log-search-cskill`
- `markdown-fetch`
- `generate-contributor-guide`

## Canonical CLI

Use `harness` directly:

```bash
harness init /path/to/repo
harness submit /path/to/repo --goal "Fix failing verify regression" --context docs/prd.md
harness tasks /path/to/repo
harness task /path/to/repo T-001
harness control /path/to/repo task T-001 status
harness daemon loop /path/to/repo --interval 30s --skip-git-repo-check
harness dashboard /path/to/repo --addr 127.0.0.1:7420 --skip-git-repo-check
```

Compatibility wrappers are still available:

```bash
harness-submit /path/to/repo --goal "Fix failing verify regression"
harness-tasks /path/to/repo
harness-task /path/to/repo T-001
harness-control /path/to/repo task T-001 status
```

## Runtime model

### Fresh worker burst

```text
codex exec --json --output-last-message <path> ...
```

### Resume burst

```text
codex exec resume <SESSION_ID> --json --output-last-message <path> ...
```

### Execution flow

1. `harness submit`
2. `harness daemon loop` or `harness dashboard`
3. route task
4. issue dispatch
5. acquire and claim lease
6. create real tmux session
7. run native codex inside tmux
8. persist checkpoint and outcome
9. ingest verify
10. expose query/control state

`harness dashboard` starts operator page and repo-local daemon loop together by default.
Use `--no-daemon` if you only need read surface.

The operator page runs on a `go-zero` `rest.Server`.
When daemon mode is enabled, it is started in the same `ServiceGroup` as the scheduler loop so HTTP and runtime share one lifecycle.

`harness control /repo task <TASK_ID> attach` prints the exact tmux attach command in non-interactive contexts.

## Planning and dispatch

Klein treats planning, dispatch, and execution as three layers:

- planner/judge establish shared task context
- judge emits dispatch-ready task list
- tmux workers execute a bounded execution slice with shared context

For corpus/batch tasks, planning should determine shared decisions before any worker starts:

- in-scope target set
- output schema and required fields
- format and length constraints
- output path and naming rules
- source and research policy

These decisions live in packet-level `.harness/artifacts/<TASK>/<DISPATCH>/shared-context.json`.

Workers should:

- read `dispatch-ticket`
- read `worker-spec`
- read `shared-context.json`
- read `task-contract`
- execute only the assigned slice

Workers should avoid reopening full planning scope, rewriting shared prompts, or re-running upstream planner/judge decisions unless artifacts conflict.

`ownedPaths` remains a boundary and audit surface, not the primary human task plan.

## Worker prompt contract

- planning metadata remains in `.harness`
- shared group context remains in `.harness`
- active tmux node label format: `[harness:<task-id>] <node-task-description>`
- worker writes `worker-result.json`, `verify.json`, and `handoff.md` for each slice

This keeps worker behavior bounded for large-volume groups.

## Runtime state

Authoritative files:

- `.harness/requests/queue.jsonl`
- `.harness/task-pool.json`
- `.harness/state/dispatch-summary.json`
- `.harness/state/lease-summary.json`
- `.harness/state/session-registry.json`
- `.harness/state/runtime.json`
- `.harness/state/verification-summary.json`
- `.harness/state/tmux-summary.json`
- `.harness/checkpoints/*`
- `.harness/artifacts/*`

Derived/view-only:

- `.harness/state/completion-gate.json`
- `.harness/state/guard-state.json`

Runtime state ownership is documented in [docs/runtime-mvp.md](docs/runtime-mvp.md).

## Guardrails

Klein does not implement a Hookify runtime.
The legacy dotfiles intent maps to runtime surfaces as follows:

| dotfiles intent | Klein mapping |
| --- | --- |
| bug / failure / regression | `policy_bug_rca_first` + debugging-first worker guidance + `systematic-debugging` |
| recommendation / compare / choose | `policy_options_before_plan` + `blueprint-architect` |
| continue / resume | `policy_resume_state_first` + state/log/skill-first resume flow |
| verify-before-stop | prompt evidence rules + runtime completion gate |
| review-before-done | review-required metadata + runtime review evidence gate |
| methodology discipline | fact-first / focus-first / verify-first mapping in prompts, planning trace, and managed AGENTS guidance |

Qiushi-inspired design mapping is documented in [docs/qiushi-integration.md](docs/qiushi-integration.md).

Current role split:

- `b3e` / `B3Ehive` = orchestration and packet convergence
- `qiushi-execution` = execution / validation loop

Details and old/new mapping are in [docs/refactor-runtime-migration.md](docs/refactor-runtime-migration.md).

## Testing

Unit tests:

```bash
go test ./...
```

If macOS linker fails with `libtapi.dylib` signature errors:

```bash
CGO_ENABLED=0 go test ./...
CGO_ENABLED=0 go build ./cmd/harness
```

Coverage-oriented integration tests:

```bash
go test -tags=integration ./...
```

These tests are for regression coverage, not the only runtime truth source.
