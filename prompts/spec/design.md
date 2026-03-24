Artifact: design

Purpose:
- explain how the change should be implemented when architecture or risk justifies a design layer

Include design when:
- the change crosses modules or services
- a new dependency or data model shift appears
- security, performance, migration, or rollback risk is non-trivial

Suggested sections:
- Context
- Goals / Non-Goals
- Decisions
- Risks / Trade-offs
- Migration Plan
- Open Questions

Writing rules:
- document why a technical choice was made, not just what was chosen
- keep detailed line-by-line implementation out of this artifact
