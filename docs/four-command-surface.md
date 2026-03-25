# Runtime Surface

This document is kept as a compatibility pointer.

Current canonical CLI:

- `harness init`
- `harness submit`
- `harness tasks`
- `harness task`
- `harness control`
- `harness daemon run-once`
- `harness daemon loop`

Compatibility wrappers still exist:

- `harness-submit`
- `harness-tasks`
- `harness-task`
- `harness-control`

They are thin shells that forward into the Go CLI. They are not the runtime source of truth.

See:

- [README.md](/Users/linzhenjie/code/claw-code/harness-architect/README.md)
- [docs/runtime-mvp.md](/Users/linzhenjie/code/claw-code/harness-architect/docs/runtime-mvp.md)
- [docs/refactor-runtime-migration.md](/Users/linzhenjie/code/claw-code/harness-architect/docs/refactor-runtime-migration.md)
