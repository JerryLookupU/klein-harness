# Klein-Harness

Klein is a repo-local agent runtime:

- native `tmux` is the execution shell
- native `codex exec` / `codex exec resume` is the worker runner
- Go owns bootstrap, routing, dispatch, lease, burst, checkpoint, outcome, verify, query, and control
- shell scripts are compatibility wrappers, not runtime source of truth

## Runtime MVP

Canonical implementation:

- CLI: `cmd/harness`
- runtime: `internal/runtime`
- bootstrap: `internal/bootstrap`
- routing: `internal/route`
- dispatch: `internal/dispatch`
- lease: `internal/lease`
- verify: `internal/verify`
- query: `internal/query`
- executor: `internal/executor/codex` and real `internal/tmux`

Compatibility surfaces that still exist:

- `scripts/harness-*.sh`
- `cmd/kh-codex`
- `cmd/kh-orchestrator`
- `cmd/kh-worker-supervisor`

These compatibility paths delegate into the Go runtime. They are no longer the canonical path.

## Install

```bash
./install.sh --force
```

Install effects:

- installs skills into `$CODEX_HOME/skills`
- installs the canonical `harness` CLI into `$CODEX_HOME/bin` when Go is available
- installs compatibility wrappers such as `harness-submit` and `harness-control`
- updates the managed block in `$CODEX_HOME/AGENTS.md` without touching user content outside the block
- updates managed profiles in `$CODEX_HOME/config.toml` without overwriting existing user profiles

Installed skills:

- `klein-harness`
- `blueprint-architect`
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
harness daemon run-once /path/to/repo --skip-git-repo-check
harness daemon loop /path/to/repo --interval 30s --skip-git-repo-check
```

Compatibility wrappers still work:

```bash
harness-submit /path/to/repo --goal "Fix failing verify regression"
harness-tasks /path/to/repo
harness-task /path/to/repo T-001
harness-control /path/to/repo task T-001 status
```

Those wrappers are thin shells that exec the Go CLI. They do not own runtime logic anymore.

## Execution Model

Fresh worker burst:

```text
codex exec --json --output-last-message <path> ...
```

Resume burst:

```text
codex exec resume <SESSION_ID> --json --output-last-message <path> ...
```

Real execution path:

1. `harness submit`
2. `harness daemon run-once`
3. route task
4. issue dispatch
5. acquire and claim lease
6. create real tmux session
7. run native codex inside tmux
8. persist checkpoint and outcome
9. ingest verify
10. expose query/control state

`harness control /repo task <TASK_ID> attach` uses the real tmux session name. In non-interactive contexts it prints the exact attach command.

## Runtime State

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

Derived or view-oriented state:

- `.harness/state/completion-gate.json`
- `.harness/state/guard-state.json`

Writers and readers are documented in [docs/runtime-mvp.md](/Users/linzhenjie/code/claw-code/harness-architect/docs/runtime-mvp.md).

## Guardrail Mapping

Klein does not implement a Hookify runtime. The old dotfiles intent is mapped into route/prompt/policy/runtime surfaces:

| dotfiles intent | Klein mapping |
| --- | --- |
| bug / failure / regression | `policy_bug_rca_first` + debugging-first worker guidance + `systematic-debugging` skill |
| recommendation / compare / choose | `policy_options_before_plan` + `blueprint-architect` |
| continue / resume | `policy_resume_state_first` + state/log/skill-first resume flow |
| verify-before-stop | prompt evidence rules + runtime completion gate |
| review-before-done | review-required metadata + runtime review evidence gate |

## Tests

Unit tests:

```bash
go test ./...
```

Integration tests with fake `tmux` and fake `codex`:

```bash
go test -tags=integration ./...
```

Real smoke tests only when the environment is ready:

```bash
KLEIN_REAL_SMOKE=1 bash scripts/smoke/runtime-smoke.sh
KLEIN_REAL_SMOKE=1 bash scripts/smoke/tmux-codex-smoke.sh
```

## Migration Notes

- `control.py` is no longer on the canonical path
- `.harness/bin/*` is no longer the system source of truth
- `scripts/harness-*.sh` now forward into Go
- the runtime no longer pretends `/bin/sh -lc` is `tmux`

Details and old-to-new mapping are in [docs/refactor-runtime-migration.md](/Users/linzhenjie/code/claw-code/harness-architect/docs/refactor-runtime-migration.md).
