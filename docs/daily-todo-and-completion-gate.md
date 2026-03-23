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

Completion is not decided by optimistic checkboxes or exit code alone.

## Archive / Retire

When the completion gate is truly satisfied:

- the loop becomes retire-eligible
- the operator can archive it through `harness-control ... project archive`
- runtime state then reports the loop as archived / retired instead of indefinitely spinning
