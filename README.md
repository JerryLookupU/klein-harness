# Klein-Harness

A repo-local closed-loop `.harness` runtime for Codex-first agent work.

> 闭环系统 / 自指结构 / AI 执行网络 / 工程化控制
>
> Closed-loop system / Self-referential structure / AI execution network / Engineered control

![Klein surface visualization](docs/klein-surface-hero.png)

Klein-Harness turns a repository into a re-entrant control surface:

- requests stay append-only and machine-readable
- runtime binds requests to tasks explicitly
- session / worktree / verification lineage stays repo-local
- symptom evidence stays separate from RCA allocation
- code workers default to isolated task worktrees with local merge integration
- reports, failures, audits, and replans can re-enter as the next request

This repo ships three Codex skills:

- `klein-harness` for runtime, routing, dispatch, verification, and operator control
- `blueprint-architect` for decomposition, research, draft blueprinting, conflict review, and final blueprint handoff
- `harness-log-search-cskill` for compact handoff log retrieval and targeted raw evidence windows

## At A Glance

Klein-Harness is for repositories that need more than a one-shot prompt.

Use it when you need:

- long-running implementation work
- multi-agent or multi-session handoff
- safe session resume instead of prompt guesswork
- repo-local recovery after failure or interruption
- operator-friendly status and reporting without re-opening model context

Default split:

- `gpt-5.4` for orchestration fallback, routing judgment, prompt refinement, and replan
- `gpt-5.3-codex` for worker execution
- `codex exec` / `codex exec resume` for the actual run surface

## Why Klein

There is no stable “inside the agent” vs “outside the agent”.
Everything important must be able to leave the current run, land in the repo, and become the next run.

Klein-Harness does that with explicit dimensions instead of fuzzy prompts:

- `request lineage`
- `task lineage`
- `session lineage`
- `worktree isolation`
- `verification state`

That is how it avoids self-intersection, unsafe resume, and lost context.

## What Makes It Different

- request intake is append-only, but runtime state is still hot and queryable
- worker evidence, compact handoff logs, verification, and RCA all stay repo-local
- routing is explicit before dispatch; resume is not left to model guesswork
- code workers execute in task worktrees and merge locally through runtime-controlled integration
- blueprint work can stop at repo-local scan, or escalate through `researchMode`
- downstream workers default to hot state -> compact logs -> raw logs, not transcript flooding

## Quick Start

Install the skills and helper commands:

```bash
./install.sh
```

This installs:

- skills: `klein-harness`, `blueprint-architect`, `harness-log-search-cskill`
- primary commands: `harness-submit`, `harness-tasks`, `harness-task`, `harness-control`
- compatibility helpers: `harness-init`, `harness-bootstrap`, `harness-report`, `harness-kick`

Canonical 4-command surface:

```bash
harness-submit /path/to/project --goal "根据 PRD 生成代码" --context docs/prd.md
harness-tasks /path/to/project
harness-task /path/to/project T-001
harness-control /path/to/project daemon status
```

`harness-submit` auto-runs `harness-init` when the project has not been initialized yet.
The runtime handles first setup, appended requirements, duplicate submissions, extra context, and inspection intent through the same write path.

Common submit shapes:

```bash
harness-submit /path/to/project --goal "根据 PRD 落一个增量改动" --context docs/prd.md
harness-submit /path/to/project --goal "检查当前 verify 状态和 compact log"
harness-submit /path/to/project --goal "补充这次失败的额外日志证据" --context .harness/state/runner-logs/T-003.log
harness-submit /path/to/project --goal "分析现有仓库并给出第一轮蓝图" --context docs/prd.md --context src/
```

`harness-submit` is the only human write path. `--kind` is optional and treated as a hint.
The runtime classifies duplicate / context enrichment / inspection / append change / fresh work internally.

Bug / feedback intake still uses the same request surface:

```bash
harness-submit /path/to/project --kind bug --goal "T-042 在 verify 后回归"
harness-submit /path/to/project --kind feedback --goal "当前 session handoff 存在歧义"
```

Read / inspect with the canonical surfaces:

```bash
harness-tasks /path/to/project
harness-task /path/to/project T-001
harness-task /path/to/project T-001 logs --detail
harness-control /path/to/project daemon restart
```

## Runtime Loop

Default loop:

```text
submit
  -> .harness/requests/queue.jsonl
  -> intake classification + request fusion
  -> thread correlation + inflight impact analysis
  -> selective replan / inspection overlay when needed
  -> request reconcile
  -> request-task binding
  -> worktree prepare
  -> route-session
  -> runner dispatch / recover / resume
  -> verify-task
  -> local merge queue / merge preview / conflict follow-up
  -> root-cause allocation / repair emission
  -> refresh-state
  -> report
  -> runtime follow-up request (audit / replan / stop / repair)
```

Core lifecycle states:

- `queued -> bound -> dispatched -> running -> verified -> completed`
- `queued -> blocked`
- `queued -> cancelled`
- `running -> recoverable -> resumed`
- code-task merge stages: `worktree_prepared -> merge_queued -> merge_checked -> merged`
- conflict path: `merge_conflict -> merge_resolution_requested`

## Shared Surface

Primary hot state:

- `.harness/state/current.json`
- `.harness/state/runtime.json`
- `.harness/state/progress.json`
- `.harness/state/queue-summary.json`
- `.harness/state/task-summary.json`
- `.harness/state/worker-summary.json`
- `.harness/state/daemon-summary.json`
- `.harness/state/worktree-registry.json`
- `.harness/state/merge-queue.json`
- `.harness/state/merge-summary.json`
- `.harness/state/blueprint-index.json`
- `.harness/state/feedback-summary.json`
- `.harness/state/root-cause-summary.json`
- `.harness/state/request-summary.json`
- `.harness/state/intake-summary.json`
- `.harness/state/thread-state.json`
- `.harness/state/change-summary.json`
- `.harness/state/lineage-index.json`
- `.harness/state/log-index.json`
- `.harness/state/research-index.json`

Primary append-only logs:

- `.harness/requests/queue.jsonl`
- `.harness/lineage.jsonl`
- `.harness/feedback-log.jsonl`
- `.harness/root-cause-log.jsonl`

Primary mutable ledgers:

- `.harness/state/request-index.json`
- `.harness/state/request-task-map.json`
- `.harness/task-pool.json`
- `.harness/session-registry.json`
- `.harness/state/worktree-registry.json`
- `.harness/state/merge-queue.json`

The control plane stays explicit in three layers:

- cold evidence: append-only logs and raw runner output
- runtime ledgers: mutable request/task/session truth
- hot summaries: bounded JSON snapshots for operator and worker reads

Progressive execution is thread-aware:

- each submission gets classified and fused into a thread
- `planEpoch` only bumps when appended change really affects current work
- unaffected inflight tasks continue
- superseded queued tasks stay visible in ledgers but do not dispatch
- context rot and drift checks prefer checkpoint + fresh session over blind resume

Evidence and RCA are intentionally split:

- `feedback-log.jsonl` stores symptom evidence and runtime events
- `root-cause-log.jsonl` stores RCA decisions, owner allocation, repair mode, and prevention write-back

## Command Surface

Canonical public surface:

```bash
harness-submit /path/to/project --goal "<GOAL>" [--kind <HINT>] [--thread-key <KEY>] [--idempotency-key <KEY>]
harness-tasks /path/to/project [summary|queue|tasks|requests|workers|daemon|blockers|logs]
harness-task /path/to/project <TASK_ID|REQUEST_ID> [detail|logs]
harness-control /path/to/project <daemon|task|request> [args...]
```

Examples:

```bash
harness-submit /path/to/project --goal "为当前仓库建立第一轮闭环" --context docs/prd.md
harness-tasks /path/to/project queue
harness-task /path/to/project T-003
harness-task /path/to/project T-003 logs --detail
harness-control /path/to/project task T-003 checkpoint --reason "operator wants a safe pause"
harness-control /path/to/project daemon status
```

Advanced and compatibility helpers still exist, but they are no longer the primary UX:

```bash
.harness/bin/harness-ops . top
.harness/bin/harness-ops . doctor
.harness/bin/harness-query overview . --text
.harness/bin/harness-log-search . --task-id T-003
.harness/bin/harness-runner tick .
.harness/bin/harness-runner daemon . --interval 60
.harness/bin/harness-verify-task <TASK_ID> . --write-back
harness-init /path/to/project
harness-bootstrap /path/to/project "<GOAL>" [STACK_HINT]
harness-report /path/to/project
harness-kick "<PROJECT_GOAL>" [STACK_HINT] [PROJECT_ROOT]
```

Notes:

- the repo-local runtime / daemon is the scheduler and source of truth
- `--dispatch-mode tmux` is the current default execution backend
- `--dispatch-mode print` is a non-executing compatibility / debug backend
- workers should not merge or push directly; runtime serializes local integration through `integrationBranch`
- git conflict is treated as a structured harness event, not as a late remote-push surprise
- `harness-runner daemon` keeps ticking and refreshing hot state on a fixed interval
- `harness-bootstrap` and `harness-kick` remain available as compatibility / expert helpers
- use `--no-daemon` when you want a manual or fully operator-driven session
- downstream workers should prefer hot state -> compact log md -> raw log
- worker/backend health and runtime health are intentionally surfaced separately

## Compact Logs

Klein-Harness keeps raw runner logs as cold evidence and adds a compact cross-worker handoff layer:

- raw evidence: `.harness/state/runner-logs/<taskId>.log`
- compact handoff: `.harness/log-<taskId>.md`
- hot log summary: `.harness/state/log-index.json`
- targeted retrieval: `.harness/bin/harness-log-search`

Default search stays summary-first.
Use `--detail` only when you need raw evidence windows.

## Blueprint Research Gate

Blueprint work includes a gated research stage instead of forcing deep research on every design:

- `researchMode: none | targeted | deep`
- research memos live in `.harness/research/<slug>.md`
- hot memo summary lives in `.harness/state/research-index.json`
- bounded machine summary lives in `.harness/state/research-summary.json`
- blueprint generation should consume repo-local scan + research memo + conflict review

Recommended triggers for `targeted` or `deep` research:

- upstream behavior may have changed
- repository context is insufficient
- external framework or protocol behavior matters
- architecture options need explicit comparison
- migration or rollout risk is material

## Demo Flow

Minimal end-to-end demo:

```bash
harness-submit /path/to/project --goal "根据当前仓库建立第一轮闭环" --context docs/prd.md
harness-tasks /path/to/project
harness-task /path/to/project T-001
harness-control /path/to/project daemon status
```

Release smoke:

```bash
bash ./skills/klein-harness/examples/harness-release-smoke.example.sh
bash ./skills/klein-harness/examples/worktree-merge-smoke.example.sh
```

## Layout

Primary skills:

- `skills/klein-harness/SKILL.md`
- `skills/blueprint-architect/SKILL.md`
- `skills/harness-log-search-cskill/SKILL.md`

Primary docs:

- `docs/control-plane-state.md`
- `docs/operator-cli.md`
- `docs/blueprint-research-gate.md`
- `docs/worktree-first-execution.md`
- `docs/local-merge-queue.md`
- `docs/merge-conflict-as-runtime-signal.md`
- `docs/single-entry-intake.md`
- `docs/request-fusion-and-progressive-execution.md`
- `docs/context-rot-and-drift-guards.md`
- `docs/runtime-request-spec.md`
- `docs/klein-architecture.md`
- `docs/log-search-architecture.md`

Primary references:

- `skills/klein-harness/references/schema-contracts.md`
- `skills/klein-harness/references/openclaw-dispatch.md`
- `skills/klein-harness/references/model-routing.md`
- `skills/blueprint-architect/references/blueprint-schema.md`
- `skills/blueprint-architect/references/conflict-checklist.md`

## More References

Additional references and examples:

- `skills/klein-harness/references/git-worktree-playbook.md`
- `skills/klein-harness/references/bash-python-toolkit.md`
- `skills/klein-harness/examples/`

## Recommended Reading

Read in this order:

1. `skills/klein-harness/SKILL.md`
2. `skills/blueprint-architect/SKILL.md`
3. `docs/runtime-request-spec.md`
4. `docs/klein-architecture.md`
5. `docs/control-plane-state.md`
6. `docs/operator-cli.md`
7. `docs/worktree-first-execution.md`
8. `docs/local-merge-queue.md`
9. `docs/merge-conflict-as-runtime-signal.md`
10. `docs/log-search-architecture.md`
11. `docs/blueprint-research-gate.md`
12. `skills/klein-harness/references/schema-contracts.md`
13. `skills/klein-harness/references/openclaw-dispatch.md`
14. `skills/klein-harness/references/model-routing.md`

## Trial and Feedback

If you are evaluating the repo, start here:

- `skills/klein-harness/TRY-IT.md`
- `skills/klein-harness/FEEDBACK.md`

Good feedback topics:

- where the runtime model is still hard to understand
- where docs are too long or too implicit
- where field names feel unclear
- which script fails first in real use
- where weaker worker models drift most often

## License

[MIT](./LICENSE)
