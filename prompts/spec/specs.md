Artifact: specs

Purpose:
- define what the system must do in a testable, requirement-shaped form

Structure rules:
- one spec file per capability
- each requirement uses `### Requirement: <name>`
- each requirement has at least one scenario
- each scenario uses `#### Scenario: <name>`
- use SHALL or MUST for normative behavior

Delta rules:
- ADDED for new requirements
- MODIFIED for changed behavior and include the full updated requirement
- REMOVED only with reason and migration guidance
- RENAMED only for name changes

Quality bar:
- requirements should be independently testable
- scenarios should be concrete enough to become verification cases
