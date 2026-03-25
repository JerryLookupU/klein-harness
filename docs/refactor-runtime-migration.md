# Refactor Runtime Migration

This document records the MVP runtime cutover.

## What Changed

Old shape:

- shell commands were presented as canonical
- project-local `.harness/bin/*` often acted as the real runtime
- `control.py` still owned parts of control flow
- `internal/tmux` used `/bin/sh -lc` instead of a real tmux session lifecycle

New shape:

- `cmd/harness` is the canonical CLI
- shell scripts are thin compatibility wrappers only
- control actions live in Go
- burst execution uses real `tmux` commands and native `codex`

## Old To New

- `harness-submit` -> `harness submit`
- `harness-tasks` -> `harness tasks`
- `harness-task` -> `harness task`
- `harness-control` -> `harness control`
- ad-hoc runner tick scripts -> `harness daemon run-once`
- ad-hoc runner loop scripts -> `harness daemon loop`

## Removed From Canonical Path

- `.harness/scripts/control.py`
- `.harness/bin/harness-submit`
- `.harness/bin/harness-control`
- `.harness/bin/harness-runner`
- `.harness/bin/harness-ops`
- skill example shell install scripts as runtime truth

These may still exist in older repos or historical docs, but they are legacy compatibility references only.

## Compatibility Behavior

- installed `scripts/harness-*.sh` remain callable
- wrappers now exec the Go CLI
- `HARNESS_BIN` can override the delegated binary for testing or local pinning

## Operator Migration

Recommended migration:

1. run `./install.sh --force`
2. use `harness ...` as the primary CLI
3. keep wrappers only for muscle-memory compatibility
4. stop depending on `.harness/bin/*` or `control.py`

## Runtime Migration

- resume state now comes from `.harness/state/session-registry.json`
- tmux state now comes from `.harness/state/tmux-summary.json`
- completion and archive safety come from runtime verify/completion gate, not prompt-only reminders
