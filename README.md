# Klein-Harness

A repo-local closed-loop `.harness` runtime for Codex-first agent work.

Klein-Harness turns a repository into a re-entrant control surface:

- requests are append-only and machine-readable
- runtime binds requests to tasks explicitly
- session / worktree / verification lineage stays repo-local
- reports, failures, audits, and replans can re-enter as the next request

This repo ships two Codex skills:

- `klein-harness` for repo-local closed-loop runtime and operator control
- `blueprint-architect` for blueprint decomposition, research, draft/review/final design

## What It Is

Klein-Harness is for projects that need more than a one-shot prompt.

It is designed for:

- long-running implementation work
- multi-agent or multi-session handoff
- safe session resume instead of prompt guesswork
- repo-local recovery after failure or interruption
- operator-friendly status and reporting without re-opening model context

The default split is:

- `gpt-5.4` for orchestration fallback, routing judgment, prompt refinement, and replan
- `gpt-5.3-codex` for worker execution
- `codex exec` / `codex exec resume` for the actual run surface

## Why Klein

The key idea is simple:

There is no stable “inside the agent” vs “outside the agent”.
Everything important must be able to leave the current run, land in the repo, and become the next run.

Klein-Harness does that with explicit dimensions instead of fuzzy prompts:

- `request lineage`
- `task lineage`
- `session lineage`
- `worktree isolation`
- `verification state`

That is how it avoids self-intersection, unsafe resume, and lost context.

## Quick Start

Install the skills and helper commands:

```bash
./install.sh
```

This installs:

- `klein-harness`
- `blueprint-architect`

- `harness-init`
- `harness-bootstrap`
- `harness-submit`
- `harness-report`
- `harness-kick`

Initialize a target project:

```bash
harness-init /path/to/project
```

Bootstrap the first orchestration round:

```bash
harness-bootstrap /path/to/project "根据 PRD 生成代码" "React + Vite" --context docs/prd.md
```

By default, `harness-bootstrap` auto-starts the runner daemon after bootstrap completes. Use `--no-daemon` to opt out.

Submit incremental work:

```bash
harness-submit /path/to/project --kind implementation --goal "根据 PRD 落一个增量改动" --context docs/prd.md
```

Read the current runtime state:

```bash
harness-report /path/to/project
```

## Runtime Loop

The default closed loop is:

```text
submit
  -> .harness/requests/queue.jsonl
  -> request reconcile
  -> request-task binding
  -> route-session
  -> runner dispatch / recover / resume
  -> verify-task
  -> refresh-state
  -> report
  -> runtime follow-up request (audit / replan / stop)
```

Core lifecycle states:

- `queued -> bound -> dispatched -> running -> verified -> completed`
- `queued -> blocked`
- `queued -> cancelled`
- `running -> recoverable -> resumed`

## Shared Repo Surface

Primary hot state:

- `.harness/state/current.json`
- `.harness/state/runtime.json`
- `.harness/state/blueprint-index.json`
- `.harness/state/feedback-summary.json`
- `.harness/state/request-summary.json`
- `.harness/state/lineage-index.json`

Primary append-only logs:

- `.harness/requests/queue.jsonl`
- `.harness/lineage.jsonl`
- `.harness/feedback-log.jsonl`

Primary mutable ledgers:

- `.harness/state/request-index.json`
- `.harness/state/request-task-map.json`
- `.harness/session-registry.json`

## Commands

Global entry points:

```bash
harness-init /path/to/project
harness-bootstrap /path/to/project "<GOAL>" [STACK_HINT]
harness-submit /path/to/project --kind implementation --goal "<GOAL>"
harness-report /path/to/project
harness-kick "<PROJECT_GOAL>" [STACK_HINT] [PROJECT_ROOT]
```

Project-local operator commands:

```bash
.harness/bin/harness-status .
.harness/bin/harness-report .
.harness/bin/harness-query overview . --text
.harness/bin/harness-dashboard .
.harness/bin/harness-watch . 2
```

Runner and verification surface:

```bash
.harness/bin/harness-runner tick .
.harness/bin/harness-runner tick . --dispatch-mode print
.harness/bin/harness-runner daemon . --interval 60
.harness/bin/harness-runner daemon-status .
.harness/bin/harness-runner daemon-stop .
.harness/bin/harness-runner recover <TASK_ID> .
.harness/bin/harness-verify-task <TASK_ID> . --write-back
python3 .harness/scripts/refresh-state.py .
```

Notes:

- `--dispatch-mode tmux` is the default real dispatch mode
- `--dispatch-mode print` writes route and dispatch evidence without starting `tmux`
- `harness-runner daemon` keeps ticking and refreshing hot state on a fixed interval
- `harness-bootstrap` / `harness-kick` start the runner daemon by default after bootstrap success
- use `--no-daemon` when you want a manual or fully operator-driven session

## One-Command Demo

Minimal demo path:

```bash
harness-init /path/to/project
harness-submit /path/to/project --kind implementation --goal "实现一个最小 smoke 任务"
/path/to/project/.harness/bin/harness-runner tick /path/to/project --dispatch-mode print
python3 /path/to/project/.harness/scripts/refresh-state.py /path/to/project
harness-report /path/to/project
```

Release smoke:

```bash
bash ./skills/klein-harness/examples/harness-release-smoke.example.sh
```

## Repository Layout

Skills:

- `skills/klein-harness/SKILL.md`
- `skills/blueprint-architect/SKILL.md`

References:

- `skills/klein-harness/references/schema-contracts.md`
- `skills/klein-harness/references/openclaw-dispatch.md`
- `skills/klein-harness/references/model-routing.md`
- `skills/klein-harness/references/git-worktree-playbook.md`
- `skills/klein-harness/references/bash-python-toolkit.md`
- `skills/blueprint-architect/references/blueprint-schema.md`
- `skills/blueprint-architect/references/conflict-checklist.md`

Examples:

- `skills/klein-harness/examples/`

Architecture docs:

- `docs/runtime-request-spec.md`
- `docs/klein-architecture.md`

## Recommended Reading

Read in this order:

1. `skills/klein-harness/SKILL.md`
2. `skills/blueprint-architect/SKILL.md`
3. `docs/runtime-request-spec.md`
4. `docs/klein-architecture.md`
4. `skills/klein-harness/references/schema-contracts.md`
5. `skills/klein-harness/references/openclaw-dispatch.md`
6. `skills/klein-harness/references/model-routing.md`

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

## English Summary

Klein-Harness is a Codex-first closed-loop `.harness` runtime that keeps request intake, task binding, session routing, worktree isolation, verification, and follow-up requests inside the repo as machine-readable state.

If you want the deeper protocol, read:

- `docs/runtime-request-spec.md`
- `docs/klein-architecture.md`
- `skills/klein-harness/SKILL.md`

## License

[MIT](./LICENSE)
