# Draft Blueprint Example

## Background

The current memory layer can ingest artifacts, but provenance is partial and cannot explain why a record exists.

## Goal

Add stable provenance capture for message events and tool-result artifacts.

## Non-Goals

- Do not redesign retrieval ranking in this round
- Do not change memory-adapter slot behavior yet

## Current State

- hooks already capture part of the event stream
- projections write normalized records
- tests cover folding and overlay ingest, but provenance semantics are incomplete

## Constraints

- keep OpenClaw plugin contract compatible
- avoid rewriting recall planner in the first lane
- preserve existing test surfaces where possible

## Research Gate

- `researchMode`: `targeted`
- `researchMemoPath`: `.harness/research/react-router-data-loading.md`
- rationale: upstream routing/data-loading behavior affects migration shape

## Candidate Directions

### Direction A

Add provenance envelope at hook time, persist through projection layer.

### Direction B

Infer provenance lazily during retrieval.

## Early Conflict Analysis

- `hard conflict`: lazy inference makes auditability weak and breaks deterministic verification
- `soft conflict`: hook-time envelope increases write-time payload size

## Verification

- add unit coverage around hook -> projection provenance flow
- verify artifact pointer retention

## Open Questions

- should provenance IDs be globally stable or workspace-scoped?
