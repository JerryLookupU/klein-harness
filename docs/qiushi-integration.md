# Qiushi Integration

This document records how Klein-Harness absorbs the useful methodology from [`qiushi-skill`](https://github.com/HughYau/qiushi-skill) without copying its full plugin runtime into this repo.

## What We Took

Klein now adopts three design ideas inspired by `qiushi-skill`:

1. a visible methodology entrance
2. a small set of reusable execution disciplines
3. session / prompt level guidance instead of a second orchestration runtime

## What We Did Not Take

Klein does not import `qiushi-skill` as a second control plane.

It does not add:

- a separate hook daemon
- a second scheduler
- new runtime ledger entities
- a plugin-only execution path outside the existing `harness` runtime

## Klein Mapping

### Division of labor with B3Ehive

- B3Ehive is the orchestration layer:
  - choose flow
  - run 3 planners + 1 judge
  - produce packet, worker-spec slices, and verification design
- qiushi is the execution loop layer:
  - investigate before action
  - execute one bounded slice
  - verify with evidence
  - close out honestly
  - if not passed, return to analysis and re-enter execution

Klein should not use qiushi to replace B3Ehive planning, and should not use B3Ehive as the main execution worker.

### Fact-first investigation

Mapped into:

- route flow selection
- packet synthesis constraints
- prompt guidance for evidence-first debugging and resume flows

### Concentrated effort

Mapped into:

- judge preference for one bounded winning packet
- worker guidance to avoid broad blended tasks

### Practice and verification

Mapped into:

- worker execution rules
- hookified verification flow
- completion gate

### Self-critique and honest closeout

Mapped into:

- verify evidence requirements
- handoff requirements
- runtime closeout blocking when artifacts are incomplete

### Balanced trade-offs

Mapped into:

- route and judge dimensions such as repo fit, feasibility, rollback risk, and verification completeness

## Concrete Repo Changes

- added [`skills/qiushi-execution/SKILL.md`](/Users/linzhenjie/code/claw-code/harness-architect/skills/qiushi-execution/SKILL.md) as a repo-local methodology skill
- added [`prompts/spec/methodology.md`](/Users/linzhenjie/code/claw-code/harness-architect/prompts/spec/methodology.md) to the prompt contract
- updated orchestration defaults so planning traces and prompt refs make the methodology layer visible
- updated install-managed AGENTS content so global guidance also reflects the new discipline

## Working Rule

The short version is:

`investigate first -> focus the main slice -> execute in bounds -> verify with evidence -> close out honestly`
