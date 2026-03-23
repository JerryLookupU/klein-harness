# Final Blueprint Example

## Background

OpenClaw Brain already stores foldable memory records, but provenance is inconsistent across message events and tool-result artifacts.

## Goal

Introduce deterministic provenance envelopes at ingest time so later retrieval, audit, and feedback flows can explain record origin without re-inferring history.

## Non-Goals

- no retrieval ranking redesign
- no memory slot protocol redesign
- no migration of old records in this lane

## Current State

- hooks provide multiple ingest entry points
- projection layer owns normalized record writes
- recall layer depends on stable record shape

## Constraints

- plugin API compatibility must hold
- recall planner behavior must not regress
- provenance must survive artifact-based ingest and async fallback paths

## Research Gate

- `researchMode`: `targeted`
- `researchMemoPath`: `.harness/research/react-router-data-loading.md`
- external framework behavior was first normalized into a repo-local memo before finalizing this blueprint

## Design

- create a write-time provenance envelope
- attach it in hook lanes before projection writes
- keep projection responsible for normalized persistence
- keep recall layer read-only over provenance fields

## Conflict Analysis

- `hard conflict`
  where: retrieval-time provenance inference
  why: non-deterministic and not auditable
  resolution: move provenance ownership to ingest-time envelope

- `soft conflict`
  where: hook payload size
  why: more metadata at write time
  resolution: keep envelope minimal and normalize large payloads into artifact pointers

## Rollout

1. implement observation/provenance lane
2. verify hook + projection tests
3. audit interaction with recall/deep-archive lanes
4. only then map into broader memory evolution work

## Verification

- unit tests for message/tool-result provenance capture
- regression tests for overlay ingest hooks
- no failing plugin registration or install-flow tests

## Open Questions

- whether old records need backfill in a later migration lane
