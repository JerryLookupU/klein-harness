# OpenAI Codex Research Memo

Date: 2026-03-24
Mode: targeted
Research question: if Klein-Harness wants to become "like openai/codex", which public product surfaces are actually visible in the upstream repo/docs, and which parts are clearly proprietary or OpenAI-hosted?

## Sources

- OpenAI Codex repo README: <https://github.com/openai/codex>
- Codex CLI docs root: <https://developers.openai.com/codex/cli>
- Codex CLI reference: <https://developers.openai.com/codex/cli/reference>
- Codex auth docs: <https://developers.openai.com/codex/auth>
- Codex agent approvals and security: <https://developers.openai.com/codex/agent-approvals-security>
- Codex AGENTS.md guide: <https://developers.openai.com/codex/guides/agents-md>
- Codex GitHub Action: <https://github.com/openai/codex-action>

## Findings

### 1. Upstream is a user-facing coding product, not only a hidden runtime

The public repo positions Codex as a local coding agent with multiple faces:

- CLI entrypoint (`codex`)
- non-interactive execution (`codex exec`)
- session resume (`codex exec resume`, `codex resume`)
- desktop launcher (`codex app`)
- local app server (`codex app-server`)
- GitHub Action (`openai/codex-action`)

That means "做一个一样的" is not equivalent to only adding worker-orchestrator internals. It needs a product shell above the runtime.

### 2. Authentication is a first-class surface

Public docs show two sign-in modes:

- ChatGPT sign-in for subscription access
- API key sign-in for usage-based access

CLI defaults to ChatGPT sign-in when no valid session exists, but docs explicitly recommend API key auth for programmatic CLI workflows such as CI/CD.

Practical implication for Klein:

- API-key auth is reproducible.
- ChatGPT auth parity is not realistically reproducible unless we are intentionally building against OpenAI's own OAuth/session ecosystem.

### 3. Approval policy and sandboxing are core product behavior

Codex exposes explicit policy knobs:

- `approval_policy`
- `sandbox_mode`
- `--ask-for-approval`
- `--sandbox`
- `--full-auto`
- `--yolo`
- OS-specific sandbox helpers via `codex sandbox`

This is much closer to a codified local execution policy engine than Klein's current mix of repo-local guard logic plus shell wrappers.

### 4. AGENTS.md discovery is a formalized instruction chain

Codex documents deterministic AGENTS discovery:

- global `~/.codex/AGENTS.md` or `AGENTS.override.md`
- repo root down to current directory
- one file per directory
- merge order from root to leaf
- configurable fallback filenames and byte limits

Klein uses AGENTS in the repo, but it does not currently expose this as a productized instruction-discovery subsystem.

### 5. Non-interactive automation is a core design axis

The CLI reference and GitHub Action both emphasize:

- `codex exec` for CI/scripted runs
- JSONL output
- output-file support
- output schema validation
- session resume
- sandbox/approval presets

Klein already has a closed-loop runtime and worker bursts, but it does not yet expose a general-purpose Codex-style non-interactive command surface.

### 6. Review is treated as a dedicated user workflow

Codex CLI features include local review modes over:

- branch diff
- uncommitted changes
- specific commits

Klein has audit/review concepts in the runtime, but not yet as a direct "developer asks for review now" product mode.

### 7. Subagents are explicit opt-in, not ambient magic

Codex docs say subagents only run when explicitly requested.

This aligns well with Klein's current boundary that delegation should be explicit and bounded. No design conflict here.

### 8. OpenAI-hosted features are out of scope for strict clone parity

The following are visible in docs but should be treated as non-reproducible or not phase-1 targets:

- ChatGPT subscription-backed auth/session behavior
- Codex Desktop parity
- Codex cloud / web parity
- exact proprietary model routing and credit semantics
- exact OpenAI-managed search cache and proxy behaviors

## Repo-local implications for Klein-Harness

Klein today is strongest in:

- request/task/plan epoch control
- dispatch / lease / checkpoint ledgers
- repo-local daemon / worktree / verification loop

Klein is weak relative to upstream Codex in:

- single-binary product shell
- auth UX
- config-driven policy surface
- AGENTS discovery as a public feature
- general interactive CLI/TUI UX
- generic `exec` JSONL output contract
- codex-style review entrypoints

## Conclusion

If the goal is "做一个像 openai/codex 一样的", the right interpretation is:

1. keep Klein's closed-loop runtime as the execution kernel
2. add a Codex-like product shell above it
3. implement reproducible parity first:
   - API-key auth
   - config/profiles
   - approval and sandbox policy surface
   - AGENTS discovery
   - `exec` / `resume` / JSONL automation
   - review mode
4. explicitly defer non-reproducible OpenAI-hosted parity:
   - ChatGPT sign-in
   - Codex cloud/web
   - Desktop parity

This should be treated as a body-repo architecture extension, not a prompt tweak.
