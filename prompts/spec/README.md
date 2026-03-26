This directory contains the runtime-internal orchestration prompts for Klein-Harness.

Runtime shape:
- the outer runtime is route-first-dispatch-second and remains repo-owned
- orchestration meaning is carried by a runtime-owned packet, not a visible outer `proposal/specs/design/tasks` stage
- b3e-style 3+1 convergence exists only inside packet synthesis subunits

Runtime role split:
- orchestrator runtime: submit -> classify -> fuse -> bind -> route -> issue dispatch ticket -> ingest outcome -> verify -> refresh summaries
- packet synthesis subunit: 3 planners + 1 judge produce one orchestration packet and task-local worker-spec candidates
- worker execution: read dispatch ticket + worker-spec -> execute -> verify -> handoff
- methodology layer: qiushi-inspired fact-first / focus-first / verify-first discipline shapes route, planning, execution, and closeout without creating a second runtime

Role boundary rule:
- orchestration prompts produce task orchestration artifacts only
- orchestration prompts may read repo facts, state, logs, artifacts, and external references when needed for planning
- orchestration prompts must not execute tasks, write repo code, or perform execution-layer validation
- execution belongs to dispatch + worker after orchestration output is accepted

Canonical output contract:
- planner outputs are exact JSON objects with a shared top-level schema
- judge output is an exact JSON object with a shared top-level schema
- the final packet follows `packet.md`
- final worker-spec candidates follow `worker-spec.md`
- packet and worker-spec candidates must not be mixed into one ambiguous top-level blob

Default load order:
1. orchestrator.md
2. methodology.md
3. propose.md
4. packet.md
5. tasks.md
6. worker-spec.md
7. dispatch-ticket.md
8. worker-result.md
9. apply.md
10. verify.md
11. archive.md
12. planner-architecture.md
13. planner-delivery.md
14. planner-risk.md
15. judge.md

Compatibility note:
- `proposal.md`, `specs.md`, `design.md`, and `tasks.md` remain only as mapping shims for older mental models
- do not treat those shim files as first-class runtime stages
- behavior guardrails from dotfiles-style workflows should map into this prompt layer, route reason codes, and task-local verify/review requirements rather than a separate Hookify runtime

Usage rules:
- when a request arrives as a requirement, start from this directory
- synthesize or refresh a packet only when runtime state does not already hold an accepted packet for the active epoch
- planner and judge prompts may shape packet and worker-spec candidates, but completion still belongs to `completion-gate.json`
- prefer bounded, verifiable task slices over broad architectural narration
- treat `methodology.md` as a discipline lens, not as permission to invent extra runtime stages
- when route or dispatch supplies `reasonCodes` with `policy_*` tags, treat those tags as hard guardrails that must be reflected in packet flow selection, execution tasks, verification, and review

Constraint taxonomy:
- classify constraints first by layer:
  - planning
  - execution
  - verification
  - runtime
  - learning
- classify each rule second by category:
  - objective
  - boundary
  - process
  - evidence
  - format
  - escalation
- prefer progressive promotion:
  - observed -> suggested -> enforced
