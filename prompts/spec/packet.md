Artifact: orchestration packet

Purpose:
- hold the runtime-owned meaning that older stacks spread across `proposal/specs/design/tasks`

Required fields:
- objective
- constraints
- flowSelection
- policyTagsApplied
- selectedPlan
- rejectedAlternatives
- executionTasks
- verificationPlan
- decisionRationale
- ownedPaths
- taskBudgets
- acceptanceMarkers
- replanTriggers
- rollbackHints

Serialization:
- the packet is exactly one JSON object
- do not wrap it in markdown fences
- do not add prose before or after the JSON
- use the exact required field names above
- do not add `workerSpecCandidates` to the packet; worker-spec candidates are judge-side siblings, not packet fields

Field conventions:
- `objective`, `flowSelection`, `selectedPlan`, and `decisionRationale` are strings
- `constraints`, `policyTagsApplied`, `ownedPaths`, `acceptanceMarkers`, `replanTriggers`, and `rollbackHints` are arrays of strings
- `rejectedAlternatives` is an array of objects with `candidateId` and `reason`
- `executionTasks`, `verificationPlan`, and `taskBudgets` stay machine-readable JSON objects or arrays, not prose paragraphs

Rules:
- the packet belongs to one accepted epoch
- same accepted epoch must not produce conflicting packet truth
- executionTasks must stay bounded and dispatchable
- merge, archive, and final completion remain runtime-owned
