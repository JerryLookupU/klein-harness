# Four-Command Surface

Klein-Harness teaches exactly 4 primary commands:

- `harness-submit`
- `harness-tasks`
- `harness-task`
- `harness-control`

Everything else is compatibility or expert surface.

## Intent

- keep one public write path
- keep read paths compact and summary-first
- keep control explicit without expanding into many top-level verbs

## Canonical Usage

```bash
harness-submit /repo --goal "根据 PRD 落增量改动" --context docs/prd.md --context src/
harness-tasks /repo
harness-task /repo T-003
harness-control /repo daemon status
```

## Operator Quickstart

```bash
harness-submit /repo --goal "根据 PRD 落一个增量改动" --context docs/prd.md
harness-tasks /repo
harness-task /repo T-003
harness-control /repo task T-003 checkpoint --reason "safe pause"
harness-control /repo task T-003 restart-from-stage queued --reason "retry cleanly"
harness-control /repo project archive --reason "loop retired"
```

`harness-submit` can auto-create the `.harness` scaffold for an uninitialized repo.
That is front-door setup, not a claim that the whole project is immediately fully planned.

## Compatibility

Older helpers remain available:

- `harness-init`
- `harness-bootstrap`
- `harness-report`
- `harness-kick`

They are not the primary UX.

## Migration Notes

- old helper names remain callable for compatibility
- new docs and install output teach only the 4 commands above
- operator reads should start with `harness-tasks` and `harness-task`, not raw expert tools

## Operational Checklist

Before release or hand-off, verify:

1. `harness-submit` accepts the requirement and any context paths
2. `harness-tasks` shows the work in queue/task summaries
3. `harness-task` shows detail, lineage, and compact log summary
4. `harness-control` can stop, checkpoint, archive, and restart-from-stage
5. unknown dirty state blocks automation
6. managed dirty state becomes checkpoint provenance
7. completion gate reports why work is or is not done
8. archive/retire moves the loop to archived state explicitly
9. compatibility shims still function
10. docs/help/examples still teach only the 4-command surface
