# Checkpoint Provenance

Klein-Harness distinguishes:

- managed dirty state
- unknown dirty state

## Managed Dirty

Managed dirty state is change detected inside a runtime-owned task worktree with task provenance.

This becomes:

- pending checkpoint signal
- visible in `guard-state.json`
- visible in `worktree-registry.json`

## Unknown Dirty

Unknown dirty state blocks non-interactive automation.

Typical causes:

- manual edits in repo root
- dirty worktree with no active managed task provenance

## Why

The runtime may only auto-checkpoint work it can prove belongs to a managed run.
Unknown dirty state must be surfaced to the operator instead of silently absorbed.

## Operational Meaning

- managed dirty state becomes a pending-checkpoint signal, not an automatic claim of success
- unknown dirty state blocks non-interactive execution until the operator resolves it
- this is why checkpoint provenance is separate from completion and separate from archive
