# Operator CLI

This document now serves as a legacy/compatibility note.

Canonical operator surface:

- `harness tasks`
- `harness task`
- `harness control`
- `harness daemon run-once`
- `harness daemon loop`

Examples:

```bash
harness tasks /repo
harness task /repo T-001
harness control /repo task T-001 status
harness control /repo task T-001 attach
harness control /repo task T-001 restart-from-stage queued
harness daemon run-once /repo --skip-git-repo-check
```

Historical `.harness/bin/harness-ops` references are no longer canonical. If an older repo still has them, treat them as legacy project-local helpers, not as public runtime truth.

See [docs/runtime-mvp.md](/Users/linzhenjie/code/claw-code/harness-architect/docs/runtime-mvp.md) for the active CLI and state model.
