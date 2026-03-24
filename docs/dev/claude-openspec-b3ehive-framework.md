# Claude-Style Orchestrator with OpenSpec Task Execution

Date: 2026-03-24
Mode: extension
Research mode: none

## Goal

Freeze the intended framework shape for Klein-Harness:

- the orchestrator is a Claude Code style mini-agent-loop
- the outer orchestration block uses OpenSpec plus b3ehive to generate the final task package
- the executor polls and executes those tasks using an OpenSpec-like apply flow

## Architecture

### 1. Outer orchestrator

The orchestrator is not a generic chat loop.
It is a bounded planning loop with GPT-facing prompts and repo-local state.

Loop:

```text
requirement or spec
-> context assembly
-> determine whether artifact shaping is needed
-> shape proposal/specs/design/tasks when needed
-> run 3 parallel planners
-> run 1 judge
-> emit final orchestration package
-> persist package into task/request state
```

This is the Claude Code influence:

- explicit context assembly before action
- planner-like subagent decomposition
- judge-style convergence instead of naive single-pass planning
- planning separated from execution

### 2. Outer planning block

The outer planning block combines two ideas:

- OpenSpec gives the artifact shape
  - proposal
  - specs
  - design
  - tasks
- b3ehive gives the convergence pattern
  - 3 isolated candidate planners
  - 1 judge/formatter

The orchestrator output should look like an OpenSpec execution package, not a free-form essay.

Required output fields:

- objective
- constraints
- selected_plan
- rejected_alternatives
- execution_tasks
- verification_plan
- decision_rationale

## 3. Inner executor

The executor is not a replanner.
Its job is to poll actionable tasks and execute them.

Loop:

```text
poll next executable task
-> read current proposal/specs/design/tasks context
-> apply one task
-> verify the result
-> write outcome / handoff / checkpoint
-> poll next task
```

The executor may report drift or blockers, but it should not replace the orchestrator's role.

## Current mapping in Klein

Already present:

- prompt-level orchestrator contract in `prompts/spec/`
- `3 + 1` spec loop in worker manifest and orchestration defaults
- Codex-style entrypoint via `kh-codex`
- task/session/dispatch/lease/checkpoint ledgers
- worker-side bounded burst execution

Not fully implemented yet:

- deterministic runtime fan-out/fan-in for the 3 planners plus 1 judge as first-class control-plane tasks
- a dedicated executor poller that consumes only `execution_tasks` as a separate runtime role
- full OpenSpec artifact persistence as first-class runtime outputs instead of prompt-shaped text only

## Design rule

When evolving the framework:

- do not let the executor become a second planner
- do not let the orchestrator skip artifact shaping for vague requirements
- do not merge planner outputs by averaging; judge them and select deliberately
- keep the output package structured enough that execution can proceed without reinterpreting the original request
