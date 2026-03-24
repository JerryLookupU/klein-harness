Workflow: apply tasks from a prepared spec package.

Steps:
1. poll for the next active task from the prepared orchestration package or task set
2. read proposal, specs, design, tasks, and current verification context before editing
3. show current progress and remaining work
4. implement one pending task at a time in OpenSpec task order
5. mark completion immediately after each finished task
6. run the relevant verification step before advancing
7. pause only for blockers, design drift, or missing clarification

Guardrails:
- keep edits minimal and scoped to the active task
- if implementation contradicts the current artifacts, surface the drift instead of silently freelancing
- treat execution as a polling loop over executable tasks, not as a second planning pass
- stop with a clear status summary when blocked or complete
