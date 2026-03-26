Qiushi-inspired methodology mapping for Klein-Harness

Purpose:
- keep runtime behavior fact-first, bounded, verifiable, and reviewable
- absorb the useful parts of qiushi-skill into Klein without creating a second control plane

Division of labor:
- B3Ehive owns orchestration: route shaping, planner convergence, judge selection, task slicing, and verification design
- qiushi owns execution loop discipline: investigate -> execute -> verify -> closeout -> analysis -> re-execute

Method lenses:
- fact-first investigation:
  - when context is thin, unknowns are material, or failure evidence is missing, route and planning should gather concrete repo/runtime facts before choosing a flow
- concentrated execution:
  - when several possible tasks compete, judge should prefer one bounded, highest-leverage slice instead of a broad blended task
- practice-before-claim:
  - execution plans must lead to file, command, or state evidence before the worker may claim success
- self-critique before closeout:
  - verify and handoff must name what changed, what was checked, what remains risky, and why the run should be accepted or blocked
- balanced trade-off handling:
  - route and judge should prefer the plan that best balances repo fit, verification completeness, rollback safety, and delivery scope

Stage mapping:
- route:
  - investigate before deciding when evidence is weak
  - choose one main direction when multiple paths exist
  - keep trade-offs explicit instead of implicit
- b3ehive packet synthesis:
  - planner A emphasizes boundaries and fit
  - planner B emphasizes slicing and execution order
  - planner C emphasizes risk, verify, and rollback
  - judge selects one winner; do not average conflicting candidates into ambiguity
  - this layer stops at task orchestration output; it must not execute repo work directly
- worker execution:
  - this is the qiushi loop owner
  - read the bound runtime artifacts first
  - move from context to execution quickly once the required inputs are clear
  - avoid running a second free-form planning phase inside the worker
- verify / handoff:
  - require concrete evidence
  - write explicit review notes when the run is partial, risky, or blocked

Rules:
- methodology guides route, planning, execution, and verify; it does not create new runtime-owned ids or stages
- planning-stage methodology must not be used as justification for doing execution-stage work early
- planning-stage investigation may read requirements, code, state, logs, artifacts, and external references, but must stop at packet / worker-spec / task-slice / verification design
- when methodology and repo facts conflict, repo facts win
- when evidence is incomplete, prefer a blocked or inspection outcome over a guessed success

Layered constraint model:
- first classify constraints by layer:
  - planning
  - execution
  - verification
  - runtime
  - learning
- then classify each rule by category:
  - objective
  - boundary
  - process
  - evidence
  - format
  - escalation
- not every discovered rule should become a hard block immediately:
  - observed -> suggested -> enforced
- the purpose of constraints is to calibrate distilled key signals, not to blindly enforce every raw condition discovered during trial and error
