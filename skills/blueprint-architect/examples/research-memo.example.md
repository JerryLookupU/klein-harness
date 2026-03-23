---
schemaVersion: "1.0"
generator: "blueprint-architect"
generatedAt: "2026-03-23T00:00:00+08:00"
slug: "react-router-data-loading"
researchMode: "targeted"
question: "React Router data APIs should be fetched at route boundary or component boundary for this migration?"
sources:
  - "https://reactrouter.com/"
  - "repo:src/routes"
---

## Summary

- Route-boundary loading keeps cache ownership aligned with navigation lifecycle.
- Component-local fetching remains useful for low-stakes secondary widgets.

## Findings

- Official guidance prefers route loaders for primary navigation data.
- Current repo already centralizes route definitions, so migration cost is moderate.

## Options Compared

- Route loaders improve consistency and simplify retry semantics.
- Component fetches reduce initial migration scope but keep duplicate loading states.

## Risks

- Deep nested routes may need outlet-level fallback handling.

## Recommendation

- Use route-boundary loaders for primary page data and keep component fetches for isolated secondary panels.
