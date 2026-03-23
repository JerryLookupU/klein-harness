# Request Fusion And Progressive Execution

The runtime treats submissions as thread-aware events, not isolated one-shot prompts.

## Thread Key And Plan Epoch

- `threadKey` identifies the ongoing work stream
- `planEpoch` identifies the currently valid plan revision for that thread

New context does not automatically bump epoch.
Epoch only bumps when appended change actually affects execution scope or acceptance.

## Progressive Execution Loop

```text
submit
  -> intake classification
  -> request fusion
  -> thread correlation
  -> inflight impact analysis
  -> selective replan when needed
  -> route
  -> dispatch / recover / resume
  -> verify
  -> refresh summaries
  -> next tick
```

The runtime stays live while new submissions arrive.
Bootstrap is not required again just because requirements changed.

## Impact Classes

- `continue_safe`
- `continue_with_note`
- `checkpoint_then_replan`
- `supersede_queued`
- `inspection_only_overlay`

Rules:

- unaffected active tasks continue
- queued tasks on an older invalid epoch should not dispatch
- affected active tasks may checkpoint before replan
- inspection overlays should not stop unrelated work by default

## Compound Split

One external submission may contain mixed intent.
The runtime may split it into minimal internal work items, but it remains one external submission record.

## Context Enrichment

Merged context should not vanish into raw transcripts.
Thread state carries bounded merged context references such as:

- merged context request ids
- latest context paths
- context digest

This gives downstream workers a compact deterministic surface instead of forcing raw log scans.
