Artifact: judge tool contracts

Purpose:
- define the deterministic extraction and validation surface that the packet judge can rely on
- keep the judge focused on orchestration synthesis, not on re-parsing raw multi-agent output every time

Available contracts:

1. `collect_b3ehive_outputs`
- collect planner, swarm, and prior artifact outputs into one normalized input set
- preserve `sourceAgent`, `inputSummary`, `resultSummary`, and evidence handles

2. `extract_spec_constraints`
- extract shared spec, roster, file contract, schema fields, source plan, and hard constraints
- populate the task-group truth that should live in `finalPacket.sharedContext`

3. `synthesize_task_graph`
- turn shared spec into bounded execution tasks, dependency edges, and parallel groups
- for same-type repeated work, prefer one object per worker once the roster is known
- when the roster is not frozen yet, emit one orchestration slice and keep `orchestrationExpansionPending` open

4. `validate_dispatch_contracts`
- verify each task has explicit inputs, output path or object, done criteria, evidence requirements, and a bounded task body
- reject worker handoffs that dump shared JSON blobs instead of a structured plain-text task contract

Judge operating order:
- first collect and normalize facts
- then extract shared spec and constraints
- then synthesize the task graph and lineage
- then validate dispatch contracts before finalizing packet output

Fallback rule:
- when skill-only reasoning would require guessing roster, schema, or dependency facts, use these tool contracts first
- the judge may still synthesize and merge the final packet, but it should not invent missing structure without first attempting extraction
