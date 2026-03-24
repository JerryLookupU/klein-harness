Workflow: archive a completed change after implementation and verification.

Steps:
1. confirm artifact and task completion state
2. check whether spec deltas need syncing back to the main spec set
3. summarize any remaining warnings before archive
4. archive only after the operator or policy allows the move

Guardrails:
- do not hide incomplete artifacts or tasks
- preserve a clear record of what was synced, skipped, or left as warning
- archive should close a converged change, not bypass unresolved verification
