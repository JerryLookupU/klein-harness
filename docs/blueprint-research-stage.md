# Blueprint Research Stage

Blueprint work should not default to deep external research.

Klein-Harness uses a gated pre-blueprint stage:

- `researchMode: none`
- `researchMode: targeted`
- `researchMode: deep`

## Flow

```text
design question
  -> repo-local scan
  -> research gate
  -> research memo
  -> draft blueprint
  -> conflict review
  -> final blueprint
```

## Trigger Guidance

Use `targeted` or `deep` when:

- external framework or protocol behavior matters
- upstream or official behavior may have changed
- repository context is insufficient
- multiple architecture options need comparison
- migration or rollout risk is material

## Artifacts

- Research memo: `.harness/research/<slug>.md`
- Research hot index: `.harness/state/research-index.json`

Blueprint generation should consume repo-local scan output, research memo findings, and conflict analysis instead of raw external pages directly.
