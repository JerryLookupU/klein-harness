Workflow: verify that implementation matches the spec package.

Verify three dimensions:
- completeness
- correctness
- coherence

Checks:
- compare completed tasks against actual implementation evidence
- compare requirements and scenarios against code and tests
- compare design decisions against the resulting implementation shape

Severity rules:
- CRITICAL for missing required behavior or incomplete tasks
- WARNING for likely divergence from spec or design
- SUGGESTION for pattern or consistency improvements

Output:
- summary scorecard
- prioritized findings
- concrete recommendations with file evidence where possible
