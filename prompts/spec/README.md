This directory contains the default spec-planning prompts for Klein-Harness.

Source shape:
- OpenSpec-style artifact flow for proposal -> specs -> design -> tasks
- b3ehive-style convergence for parallel planning plus judged selection

Runtime role split:
- outer orchestrator: Claude Code style mini-agent-loop expressed as GPT prompts
- outer output: OpenSpec-like orchestration package
- inner executor: polls executable tasks and runs them through apply -> verify -> handoff flow

Default load order:
1. orchestrator.md
2. propose.md
3. proposal.md
4. specs.md
5. design.md
6. tasks.md
7. apply.md
8. verify.md
9. archive.md
10. planner-architecture.md
11. planner-delivery.md
12. planner-risk.md
13. judge.md

Usage rules:
- When a request arrives as a requirement or a spec, start from this directory.
- Use artifact prompts to shape output before code execution starts.
- Use planner and judge prompts only after the artifact-guided shape is clear.
- Prefer bounded, verifiable task sets over broad architectural narration.
