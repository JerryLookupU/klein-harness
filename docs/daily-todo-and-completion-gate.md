# Daily Todo And Completion Gate

Klein-Harness separates three layers:

1. blueprint / checklist / PRD
2. daily todo
3. completion gate

## Blueprint

Blueprint and source docs define intended work.
They are not the same thing as today's execution window.

## Daily Todo

`todo-summary.json` is a derived execution panel.
It is regenerated from:

- task state
- request state
- merge state
- checkpoint state

Humans should not maintain it by hand.

## Completion Gate

`completion-gate.json` decides whether work can retire.

It checks:

- requests settled
- actionable todo drained
- merge state coherent
- verification state coherent
- completion evidence present for passed-like verification results
- review evidence present when a task is explicitly marked `reviewRequired`

Completion is not decided by optimistic checkboxes or exit code alone.
`verification.completed` may exist while completion-gate is still open.

Runtime now treats this as a hard gate:

- `passed` / `succeeded` / `verified` do not auto-complete without evidence
- `reviewRequired` tasks do not complete without review evidence
- the same gate is what archive / retire paths must respect

## Archive / Retire

When the completion gate is truly satisfied:

- the loop becomes retire-eligible
- the operator can archive it through `harness control ... task <TASK_ID> archive`
- runtime state then reports the loop as archived / retired instead of indefinitely spinning

If the gate is still open, archive must refuse instead of silently retiring the loop.
