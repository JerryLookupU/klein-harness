# OpenAI Codex Parity Blueprint

Date: 2026-03-24
Mode: extension
Research mode: targeted
Research memo: `docs/dev/openai-codex-research-2026-03-24.md`

## Background

The request is to make Klein-Harness "like `openai/codex`".

This cannot mean "copy the repo shape" only. Public upstream behavior spans CLI UX, auth, approvals, sandboxing, AGENTS discovery, non-interactive automation, review workflows, and app-server surfaces.

Klein-Harness is currently a repo-local closed-loop runtime kernel. It is not yet a Codex-like end-user product shell.

## Goal

Build a Codex-like product layer on top of Klein-Harness so the body repo can offer:

- a single `codex`-style entrypoint
- config-driven auth/policy/session behavior
- Codex-like `exec` and `resume` automation
- deterministic AGENTS discovery
- explicit approvals and sandbox modes
- direct review workflows
- compatibility with Klein's existing request/task/worker kernel

## Non-Goals

- exact OpenAI cloud parity
- ChatGPT-backed subscription auth parity
- exact Desktop app parity
- exact TUI visual parity with upstream Rust app
- reproducing OpenAI-managed web search cache or proxy internals
- replacing Klein's request/task ledgers with opaque chat memory

## Current State

Klein-Harness already has:

- repo-local request/task runtime
- orchestrator and worker-supervisor split
- dispatch / lease / checkpoint summaries
- daemon-oriented automation concepts
- worktree-first execution
- four-command public harness surface

Klein-Harness does not yet have:

- a single Codex-like binary entrypoint
- user-facing auth commands and session status
- config.toml-driven profiles for approval/sandbox policy
- public AGENTS discovery rules modeled after Codex
- a generic `exec` / `exec resume` / `resume` surface
- codex-style review mode over branch / uncommitted / commit scopes
- app-server or SDK product layer

## Constraints

- Phase 1 remains body-repo-first: changes happen in Klein-Harness, then get validated in a target repo.
- The new layer must not break the current four-command harness surface.
- The kernel remains repo-local and explicit; no hidden mutable state in prompt transcripts.
- API-key auth is implementable; ChatGPT auth parity is not a safe assumption.
- Current implementation is Go plus shell wrappers, so parity work should prefer extending the existing Go kernel instead of replacing it with a Rust/TUI rewrite.

## Design

### Chosen direction

Add a new Codex-like product shell above the existing Klein kernel.

Working name:

- `cmd/kh-codex`
- installed aliases: `codex`-like compatibility binary or `kh-codex`

Core principle:

- Codex-like UX on top
- Klein ledgers underneath

### Product layers

#### 1. Product shell

Add a single front door with subcommands shaped after Codex:

- `codex`
- `codex exec`
- `codex exec resume`
- `codex resume`
- `codex login`
- `codex sandbox`
- `codex review`
- `codex features`

This shell translates user intent into Klein runtime actions instead of bypassing them.

#### 2. Authentication module

Introduce an auth subsystem with explicit modes:

- `api_key`
- `none`
- future placeholder: `chatgpt_oauth`

Phase-1 implementation:

- `codex login --with-api-key`
- `codex login status`
- config/env-backed credential discovery
- session metadata persisted under a Codex-compatible home directory

Do not fake ChatGPT sign-in. Mark it unsupported or stubbed.

#### 3. Policy/config module

Add a config file layer compatible with Codex concepts:

- config home: `~/.codex/`
- `config.toml`
- profiles
- approval policy
- sandbox mode
- model defaults
- review model
- fallback instruction filenames

Map policy to Klein runtime semantics rather than duplicating logic.

#### 4. AGENTS discovery module

Implement deterministic instruction discovery:

- global `~/.codex/AGENTS.override.md` then `~/.codex/AGENTS.md`
- repo root down to current working directory
- one instruction file per directory
- configurable fallback filenames
- byte-budget cap

This should feed both interactive and non-interactive runs.

#### 5. Exec/resume session layer

Add a Codex-like non-interactive execution API:

- prompt from argv or stdin
- JSONL streaming output
- final-message file output
- optional output-schema validation
- session persistence and resume by ID
- current-directory-scoped `--last`

Internally this should bind to Klein's dispatch / lease / checkpoint / artifact chain instead of inventing a second execution engine.

#### 6. Review mode

Add first-class review workflows:

- review base branch diff
- review uncommitted changes
- review one commit

Implementation strategy:

- translate review requests into read-only review tasks
- route them through Klein audit/review lane
- emit prioritized findings without mutating the worktree

#### 7. Sandbox surface

Add a user-facing sandbox command surface and policy mapping:

- `read-only`
- `workspace-write`
- `danger-full-access`

Phase-1 implementation may wrap current guard and OS execution constraints without claiming full Seatbelt/Landlock parity.

### Kernel mapping

Codex-style product shell maps onto Klein components as follows:

- auth/config -> new product layer
- AGENTS discovery -> new product layer
- `exec` / `resume` -> Klein dispatch + worker-supervisor
- review -> Klein audit/review lane
- sandbox policy -> Klein guard + runtime execution adapters
- session persistence -> Klein session/lease/checkpoint ledgers

## Conflict Analysis

### Hard conflict: exact upstream parity vs reproducible parity

Exact parity is not realistic because upstream includes OpenAI-hosted auth and product surfaces.

Resolution:

- target behavioral parity where reproducible
- document unsupported hosted-only features explicitly

### Hard conflict: Codex single-binary UX vs Klein four-command UX

Klein teaches `harness-*` commands today.

Resolution:

- keep `harness-*` as kernel/operator surface
- add `codex`-style shell as a higher-level façade

### Soft conflict: product shell vs closed-loop runtime

A Codex clone could drift toward one-shot direct prompting.

Resolution:

- all code-changing flows still route through Klein ledgers and guards
- product shell is UX, not a second runtime

### Soft conflict: auth expectations

Users may expect ChatGPT sign-in because upstream offers it.

Resolution:

- ship API-key auth first
- reserve a provider abstraction for future OAuth work
- fail clearly instead of faking unsupported auth

## Verification

The parity track is acceptable only when all of the following pass:

1. `codex login --with-api-key` stores credentials and `codex login status` reports mode correctly.
2. `codex exec` accepts prompt text or stdin and emits machine-readable progress plus a final message artifact.
3. `codex exec resume` and `codex resume --last` resume the correct persisted session.
4. AGENTS discovery loads global + repo + nested instructions in the documented order.
5. `codex review` runs read-only and reports findings without changing the worktree.
6. sandbox/approval profiles map deterministically to runtime guard behavior.
7. the existing `harness-submit` closed loop still works after the new product shell lands.

## Rollout / Migration

### Phase 0: product shell skeleton

- add `cmd/kh-codex`
- add config loading
- add auth status command
- add AGENTS discovery library

### Phase 1: non-interactive parity

- add `exec`
- add `exec resume`
- add JSONL output
- add output schema validation
- route all runs through Klein dispatch artifacts

### Phase 2: review parity

- add branch / uncommitted / commit review entrypoints
- map them onto audit/review lane

### Phase 3: sandbox/policy parity

- add profile presets
- add sandbox helper surface
- tighten policy-to-runtime mapping

### Phase 4: interactive parity

- add interactive CLI session manager
- add session picker / resume UX
- only then consider richer TUI or app-server parity

## Blueprint -> Harness Mapping

Should enter code now:

- new `cmd/kh-codex`
- new `internal/auth`
- new `internal/config`
- new `internal/instructions`
- new `internal/session`
- new `internal/review`

Should remain kernel-owned:

- dispatch
- lease
- checkpoint
- verify
- worktree guard
- daemon/runtime ledgers

Should stay deferred:

- ChatGPT auth parity
- desktop parity
- cloud/web parity
- full OS sandbox clone parity

## Open Questions

- Should the public binary actually be named `codex`, or should we keep `kh-codex` and only offer a compatibility alias?
- Do we want config compatibility with upstream field names, or only conceptual compatibility?
- Should review results land purely in stdout/output files, or also be materialized as Klein audit tasks?
- How much interactive CLI UX do we want before target-repo validation starts?
