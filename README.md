# Klein-Harness

## 中文

`Klein-Harness` 是一套用于建立和维护 `.harness/` 协作系统的 skill 与模板仓库。
当前仓库路径和技能目录仍保留 `harness-architect` 命名，以维持现有 CLI、安装路径和引用兼容。

当前版本主要面向 `Codex` 工作流，默认采用以下模型与命令约定：

- `codex exec`
- `codex exec resume`
- `gpt-5.4` 用于 orchestration fallback、prompt refinement、replan
- `gpt-5.3-codex` 用于 worker execution

仓库同时保留了对 Claude 与其他 agent 工作流的兼容思路，但命令组织、session 路由、prompt 模板与 operator CLI 以 `Codex-first` 为默认设计中心。

### 项目概述

本仓库用于将 PRD、代码仓库与 agent 执行流程组织为一套可持续推进、可并发、可恢复、可审计的 `.harness` 执行链。

适用场景：

- 需要将一份 PRD 稳定落成项目
- 需要多个 agent 接力或并行推进
- 需要降低换模型、换会话、换执行者后的上下文恢复成本
- 需要让试用者快速上手并提供结构化反馈

### 仓库内容

本仓库主要包含三类内容：

- `skills/harness-architect/SKILL.md`
  主说明书，定义模式、gate、角色、默认执行链与产物约束
- `skills/harness-architect/references/`
  协议、路由、worktree、prompt、query、schema 等参考文档
- `skills/harness-architect/examples/`
  可直接复用的模板、脚本、JSON、Markdown 与 prompt 示例

### 解决的问题

本仓库主要处理以下问题：

1. 长时间运行任务的状态恢复
2. 多 worker 并发时的路径冲突与编排漂移
3. `gpt-5.4` 与 `gpt-5.3-codex` 的模型分工
4. `session / worktree / diff / audit` 的闭环
5. 面向人类与工具的 operator / query 界面

### Klein 闭环

当前实现已经从“phase-1 request intake toolkit”升级为 Klein 风格闭环运行时，同时保留原有 CLI 名称：

- `harness-submit` 继续只做 append-only request intake
- runtime 会把 request 绑定到 task，并把状态推进到 `bound / dispatched / running / verified / completed`
- `lineage.jsonl` 与 `state/lineage-index.json` 会把 `request -> task -> session -> worktree -> verification -> outcome` 串起来
- `state/current.json`、`state/runtime.json`、`state/request-summary.json`、`state/lineage-index.json` 是人类 / operator / agent / runtime 共用的热路径
- runtime 发现失败、阻塞、审计需求后，会把 `replan / stop / audit` follow-up request 重新写回 repo-local request queue，并同步维护 `{kind}-requests.json` snapshot

这也是当前 harness 的明确特性，而不是文档概念：

- 请求是可重入的，report / failure / audit 结果都能成为下一轮 request
- session、worktree、verification、outcome 都有 repo-local 谱系，不依赖“记住上次 prompt”
- apparent self-intersection 通过 request lineage、session lineage、worktree isolation 和 verification state 显式拆维

### 快速部署

先安装 skill 到 Codex：

```bash
./install.sh
```

安装后会写入这些全局 helper：

```bash
~/.codex/bin/harness-init
~/.codex/bin/harness-bootstrap
~/.codex/bin/harness-submit
~/.codex/bin/harness-report
~/.codex/bin/harness-kick
```

安装脚本会默认尝试把对应的 `bin` 目录写入当前 shell 的 rc 文件。
如果你传了 `--no-shell-rc`，或者想手动处理 `PATH`，再执行：

```bash
echo 'export PATH="$HOME/.codex/bin:$PATH"' >> ~/.zshrc
source ~/.zshrc
```

如果你只想尽快把项目跑起来，按下面四步走就够了。

#### 1. 安装

```bash
git clone <this-repo>
cd harness-architect
chmod +x install.sh
./install.sh
```

#### 2. 初始化项目

```bash
harness-init /path/to/project
```

这一步只会安装 `.harness/` 骨架，不会调用模型。

#### 3. 做第一次 bootstrap

```bash
harness-bootstrap /path/to/project "根据 PRD 生成代码" "React + Vite" --context docs/prd.md --daemon
```

这一步会完成首次 bootstrap，并在你传 `--daemon` 时启动后台续跑。

#### 4. 看当前状态

```bash
harness-report /path/to/project
```

如果你只是想快速完成“安装 + bootstrap prompt + 可选自动执行”这一套，也可以继续用兼容入口：

```bash
harness-kick "我要创建一个番茄时钟项目" "React + Vite"
```

`harness-kick` 现在更适合被理解成：

- `harness-init` + `bootstrap prompt` 的快捷封装
- 可选自动执行 `codex exec --yolo`
- 可选顺手拉起 `runner daemon`

如果你只想安装 `.harness` 并生成 prompt，不自动 bootstrap：

```bash
harness-kick --manual "我要创建一个番茄时钟项目" "React + Vite"
```

如果 bootstrap 需要先读附加上下文，`--prd` 只是 `--context` 的别名：

```bash
harness-bootstrap /path/to/project "帮我简单分析一下我的代码，给一个 markdown 分析报告" --context docs/prd.md
```

或者：

```bash
harness-kick --prd docs/prd.md "帮我简单分析一下我的代码，给一个 markdown 分析报告"
```

这里的 `docs/prd.md` 只是普通上下文文件，不代表特殊 PRD 模式；它和其他 `docs/*.md`、`notes/*.md`、设计说明文件的处理方式相同。

### 快速使用指南

当前推荐主路径分成四个入口：

1. `harness-init`
   只初始化项目内 `.harness/` 骨架，不调用模型
2. `harness-bootstrap`
   首次 bootstrap，产出 `spec / work-items / task-pool / verification-rules`
3. `harness-submit`
   日常增量需求入口，`OpenClaw / shell / cron` 都走这里
4. `harness-report`
   读取 request + runtime 热状态，回报当前情况

最常见的使用方式：

```bash
harness-submit /path/to/project --kind analysis --goal "分析这个代码库的结构细节"
harness-submit /path/to/project --kind research --goal "找十篇相关报告"
harness-submit /path/to/project --kind implementation --goal "根据 PRD 生成代码" --context docs/prd.md
harness-submit /path/to/project --kind status --goal "查看当前进度并汇报"
python3 /path/to/project/.harness/scripts/request.py reconcile --root /path/to/project
harness-report /path/to/project
```

如果你用 `OpenClaw / shell / cron`，统一原则只有一条：

- 上游只提交 request
- 项目运行时负责编排、派发、恢复、验证、汇报

### 运行时模型

当前推荐的统一心智是：

```text
OpenClaw / shell / cron / future callers
  -> harness-submit
  -> .harness/requests/queue.jsonl
  -> request-index / request-task-map / lineage
  -> project runtime / orchestration
  -> task-pool / runner / tmux / codex
  -> verification / refresh-state
  -> harness-report
```

也就是说：

- 上游调用方只负责提交 request
- 项目运行时负责编排、绑定、派发、恢复、验证、汇报
- `runner` 是执行器，不是总入口

更细的运行时约束见：

- [docs/runtime-request-spec.md](./docs/runtime-request-spec.md)
- [docs/klein-architecture.md](./docs/klein-architecture.md)

推荐试用流程：

1. 运行 `./install.sh`
2. 阅读 [SKILL.md](./skills/harness-architect/SKILL.md)
3. 选择一个测试项目目录
4. 使用安装脚本将最小 CLI 与脚本写入该项目的 `.harness/`
5. 运行 `query` 与 `dashboard`
6. 根据项目需要接入 routing、worktree、audit

安装最小工具集：

```bash
./skills/harness-architect/examples/harness-install-tools.example.sh <PROJECT_ROOT>
```

进入新项目并打印推荐的 Codex bootstrap 指令：

```bash
./skills/harness-architect/examples/harness-kickoff.example.sh <PROJECT_ROOT> "建立一个简单的番茄闹钟 app" "React + Vite"
```

安装完整 operator/tooling 面：

```bash
./skills/harness-architect/examples/harness-install-full.example.sh <PROJECT_ROOT>
```

发布前 smoke test：

```bash
bash ./skills/harness-architect/examples/harness-release-smoke.example.sh
```

进入新项目并打印完整 bootstrap 指令：

```bash
./skills/harness-architect/examples/harness-full-kickoff.example.sh <PROJECT_ROOT> "建立一个简单的番茄闹钟 app" "React + Vite"
```

刷新热状态：

```bash
python3 .harness/scripts/refresh-state.py .
```

查看总览：

```bash
.harness/bin/harness-dashboard .
```

查看结构化查询：

```bash
.harness/bin/harness-query overview .
.harness/bin/harness-query feedback .
```

### 核心模型

- `gpt-5.4`
  负责 orchestration fallback、prompt refinement、replan
- `gpt-5.3-codex`
  负责 worker execution
- `orchestrationSessionId`
  单写主线 session
- `worktree`
  代码隔离层
- `diff`
  审计与 merge 的证据层
- `state/*.json`
  面向机器热路径的状态层

可以将这套结构理解为：

- `session` 管理上下文
- `worktree` 管理代码隔离
- `diff` 管理证据
- `.harness` 管理状态

### 最小安装集

推荐的最小安装集如下：

- `.harness/bin/harness-query`
- `.harness/bin/harness-dashboard`
- `.harness/scripts/query.py`
- `.harness/scripts/refresh-state.py`
- `.harness/tooling-manifest.json`

### 最小热路径

建议工具优先读取：

- `.harness/state/current.json`
- `.harness/state/runtime.json`
- `.harness/state/blueprint-index.json`
- `.harness/state/feedback-summary.json`
- `.harness/state/request-summary.json`
- `.harness/state/lineage-index.json`

建议人工优先阅读：

- `.harness/progress.md`
- `.harness/work-items.json`
- `.harness/task-pool.json`
- `.harness/spec.json`
- `.harness/feedback-log.jsonl`

每轮 orchestration / daemon / session 结束后，建议刷新热状态：

```bash
python3 .harness/scripts/refresh-state.py .
```

### 常用 CLI

推荐优先使用的全局入口：

```bash
harness-init /path/to/project
harness-bootstrap /path/to/project "根据 PRD 生成代码" --context docs/prd.md --daemon
harness-submit /path/to/project --kind implementation --goal "根据 PRD 生成代码" --context docs/prd.md
harness-submit /path/to/project --kind status --goal "查看当前进度并汇报"
python3 /path/to/project/.harness/scripts/request.py reconcile --root /path/to/project
harness-report /path/to/project
harness-report /path/to/project --request-id R-0003 --format json
```

项目内 operator/query 入口：

```bash
.harness/bin/harness-status .
.harness/bin/harness-report .
.harness/bin/harness-query overview . --text
.harness/bin/harness-query current . --text
.harness/bin/harness-query feedback . --text
.harness/bin/harness-dashboard .
.harness/bin/harness-watch . 2
```

runner / verification 入口：

```bash
.harness/bin/harness-runner tick .
.harness/bin/harness-runner tick . --dispatch-mode print
.harness/bin/harness-runner list .
.harness/bin/harness-runner attach T-004 .
.harness/bin/harness-runner recover T-004 .
.harness/bin/harness-verify-task T-004 . --write-back
python3 .harness/scripts/refresh-state.py .
```

说明：

- `--dispatch-mode tmux` 是默认真实派发模式
- `--dispatch-mode print` 会保留 route / dispatch evidence 和 request lifecycle 回写，但不启动 `tmux`，适合 smoke test 和发布前检查

### 典型执行链

```text
session-init
-> program pre-worker gate
-> if ambiguous: gpt-5.4 orchestration fallback
-> gpt-5.3-codex worker
-> audit worker
-> merge / replan / stop
-> refresh-state
```

### 流程图

```text
+------------------------------------------------------+
| Input Layer                                          |
| PRD / Repo Input  -->  session-init                  |
+------------------------------------------------------+
                           |
                           v
+======================================================+
|| Execution Core                                      ||
||----------------------------------------------------||
|| program pre-worker gate                            ||
|| claimable / blocked / orchestrator_review          ||
|| fresh / resume / promptStages                      ||
|| if ambiguous -> gpt-5.4 orchestration fallback     ||
|| gpt-5.3-codex worker                               ||
|| worktree execution                                 ||
|| diff + verification                                ||
+======================================================+
            |                               |
            v                               v
+-------------------------+     +-------------------------+
| audit worker            |     | merge or continue       |
+-------------------------+     +-------------------------+
            \                               /
             \                             /
              v                           v
            +----------------------------------+
            | replan needed?                   |
            +----------------------------------+
                  | yes               | no
                  v                   v
        +--------------------+   +----------------------+
        | orchestration loop |   | refresh-state        |
        +--------------------+   +----------------------+
                  |                   |
                  +---------+---------+
                            v
                 +----------------------+
                 | query / dashboard    |
                 +----------------------+
```

### 推荐阅读

优先阅读：

- [SKILL.md](./skills/harness-architect/SKILL.md)
- [TRY-IT.md](./skills/harness-architect/TRY-IT.md)
- [FEEDBACK.md](./skills/harness-architect/FEEDBACK.md)
- [references/schema-contracts.md](./skills/harness-architect/references/schema-contracts.md)
- [references/openclaw-dispatch.md](./skills/harness-architect/references/openclaw-dispatch.md)
- [references/model-routing.md](./skills/harness-architect/references/model-routing.md)

阅读顺序建议：

1. `skills/harness-architect/SKILL.md`
2. `skills/harness-architect/references/schema-contracts.md`
3. `skills/harness-architect/references/openclaw-dispatch.md`
4. `skills/harness-architect/references/model-routing.md`
5. `skills/harness-architect/references/git-worktree-playbook.md`
6. `skills/harness-architect/examples/`

### 试用与反馈

建议在试用前先阅读：

- [TRY-IT.md](./skills/harness-architect/TRY-IT.md)
- [FEEDBACK.md](./skills/harness-architect/FEEDBACK.md)

建议重点反馈：

- 哪个环节最难理解
- 哪些文档过长
- 哪些字段命名不够直观
- 哪个脚本最先失效
- 弱模型最容易在哪一步偏离
- 并发、session、worktree 的心智成本是否偏高

### 仓库定位

本仓库提供的是一套可安装的 `.harness` 协作骨架，以及 Codex-first 的任务编排、执行、审计、状态管理与查询工具组合。

### 许可证

本仓库采用 [MIT License](./LICENSE)。

---

## English

`Klein-Harness` is a skill and template repository for building and maintaining a `.harness/` coordination system.
The repository path and installed skill directory still keep the `harness-architect` name for compatibility with existing commands and references.

The current version is primarily designed for `Codex` workflows, with the following default model and command assumptions:

- `codex exec`
- `codex exec resume`
- `gpt-5.4` for orchestration fallback, prompt refinement, and replanning
- `gpt-5.3-codex` for worker execution

The repository also keeps a compatibility path for Claude and other agent workflows, but its command layout, session routing, prompt templates, and operator CLI are organized around a `Codex-first` model.

### Overview

This repository packages PRD-driven planning, repository state, and agent execution into a `.harness` workflow that supports long-running execution, concurrency, recovery, and auditability.

Typical use cases:

- turning a PRD into a real project plan and execution flow
- coordinating multiple agents in sequence or in parallel
- reducing context recovery cost across model, session, or operator changes
- enabling external testers to try the system and submit structured feedback

### Repository Contents

This repository is organized around three main parts:

- `skills/harness-architect/SKILL.md`
  the main specification for modes, gates, roles, execution flow, and output contracts
- `skills/harness-architect/references/`
  protocol, routing, worktree, prompt, query, and schema references
- `skills/harness-architect/examples/`
  reusable templates, scripts, JSON files, Markdown files, and prompt examples

### Problems It Addresses

The repository focuses on these areas:

1. state recovery for long-running tasks
2. path conflicts and orchestration drift under multi-worker concurrency
3. model division between `gpt-5.4` and `gpt-5.3-codex`
4. end-to-end closure across `session / worktree / diff / audit`
5. operator and query surfaces for both humans and tools

### Quick Deploy

Install skills into Codex first:

```bash
./install.sh
```

The installer adds these global helpers:

```bash
~/.codex/bin/harness-init
~/.codex/bin/harness-bootstrap
~/.codex/bin/harness-submit
~/.codex/bin/harness-report
~/.codex/bin/harness-kick
```

The installer will try to add the matching `bin` directory to your current shell rc file by default.
If you pass `--no-shell-rc`, or prefer to manage `PATH` yourself, add it manually:

```bash
echo 'export PATH="$HOME/.codex/bin:$PATH"' >> ~/.zshrc
source ~/.zshrc
```

If you mainly want the fastest deployment path, use the four steps below.

#### 1. Install

```bash
git clone <this-repo>
cd harness-architect
chmod +x install.sh
./install.sh
```

#### 2. Initialize a project

```bash
harness-init /path/to/project
```

This only creates the `.harness/` skeleton. It does not invoke a model yet.

#### 3. Run the first bootstrap

```bash
harness-bootstrap /path/to/project "Build from the PRD" "React + Vite" --context docs/prd.md --daemon
```

#### 4. Check the runtime state

```bash
harness-report /path/to/project
```

The compatibility shortcut is still available:

```bash
harness-kick "Build a pomodoro timer app" "React + Vite"
```

Use `harness-kick` when you want one command that installs `.harness`, writes the bootstrap prompt, and optionally runs `codex exec --yolo`.

To keep install + prompt generation only:

```bash
harness-kick --manual "Build a pomodoro timer app" "React + Vite"
```

To feed extra context into bootstrap:

```bash
harness-bootstrap /path/to/project "Give me a markdown analysis report for this codebase" --context docs/prd.md
```

or:

```bash
harness-kick --prd docs/prd.md "Give me a markdown analysis report for this codebase"
```

### Quick Usage Guide

Recommended lifecycle:

1. `harness-init`
   Create the minimal `.harness/` runtime skeleton without invoking a model.
2. `harness-bootstrap`
   Run the first model-backed bootstrap and optionally launch daemon mode.
3. `harness-submit`
   Send incremental requests from OpenClaw, shell, cron, or other callers.
4. `harness-report`
   Read request/runtime state and summarize progress.

Most common usage:

```bash
harness-submit /path/to/project --kind analysis --goal "Analyze the codebase in detail"
harness-submit /path/to/project --kind research --goal "Find ten relevant reports"
harness-submit /path/to/project --kind implementation --goal "Generate code from the PRD" --context docs/prd.md
harness-submit /path/to/project --kind status --goal "Report current progress"
harness-report /path/to/project
```

If you use OpenClaw, shell, or cron, keep one rule:

- upstream callers submit requests
- project runtime handles orchestration, dispatch, recovery, verification, and reporting

Recommended runtime model:

```text
OpenClaw / shell / cron / future callers
  -> harness-submit
  -> request queue
  -> project runtime / orchestration
  -> task-pool / runner / tmux / codex
  -> verification / refresh-state
  -> harness-report
```

This means upstream callers should submit requests instead of mutating `task-pool` directly.

For the detailed runtime contract, see:

- [docs/runtime-request-spec.md](./docs/runtime-request-spec.md)

Recommended trial flow:

1. run `./install.sh`
2. read [SKILL.md](./skills/harness-architect/SKILL.md)
3. choose a test project directory
4. install the minimal CLI and scripts into that project's `.harness/`
5. run `query` and `dashboard`
6. add routing, worktree, and audit components as needed

Install the minimal toolset:

```bash
./skills/harness-architect/examples/harness-install-tools.example.sh <PROJECT_ROOT>
```

Refresh hot state:

```bash
python3 .harness/scripts/refresh-state.py .
```

Open the dashboard:

```bash
.harness/bin/harness-dashboard .
```

Run a structured query:

```bash
.harness/bin/harness-query overview .
```

### Core Model

- `gpt-5.4`
  handles orchestration fallback, prompt refinement, and replanning
- `gpt-5.3-codex`
  handles worker execution
- `orchestrationSessionId`
  the single-writer orchestration session
- `worktree`
  the code isolation layer
- `diff`
  the evidence layer for audit and merge
- `state/*.json`
  the machine-oriented hot-path state layer

This structure can be read as:

- `session` manages context
- `worktree` manages code isolation
- `diff` manages evidence
- `.harness` manages state

### Minimal Install Set

The recommended minimal install set includes:

- `.harness/bin/harness-query`
- `.harness/bin/harness-dashboard`
- `.harness/scripts/query.py`
- `.harness/scripts/refresh-state.py`
- `.harness/tooling-manifest.json`

### Minimal Hot Path

Tools should prefer:

- `.harness/state/current.json`
- `.harness/state/runtime.json`
- `.harness/state/blueprint-index.json`
- `.harness/state/feedback-summary.json`

Humans should usually start with:

- `.harness/progress.md`
- `.harness/work-items.json`
- `.harness/task-pool.json`
- `.harness/spec.json`
- `.harness/feedback-log.jsonl`

After each orchestration / daemon / session round, refresh hot state:

```bash
python3 .harness/scripts/refresh-state.py .
```

Release smoke:

```bash
bash ./skills/harness-architect/examples/harness-release-smoke.example.sh
```

### Common CLI

Machine-readable queries:

```bash
.harness/bin/harness-query overview .
.harness/bin/harness-query progress .
.harness/bin/harness-query current .
.harness/bin/harness-query feedback .
.harness/bin/harness-query blueprint .
.harness/bin/harness-query task . T-004
```

Human-readable dashboard:

```bash
.harness/bin/harness-dashboard .
.harness/bin/harness-dashboard . T-004
.harness/bin/harness-dashboard . T-004 --watch 2
```

### Typical Execution Chain

```text
session-init
-> program pre-worker gate
-> if ambiguous: gpt-5.4 orchestration fallback
-> gpt-5.3-codex worker
-> audit worker
-> merge / replan / stop
-> refresh-state
```

### Flow Diagram

```text
+------------------------------------------------------+
| Input Layer                                          |
| PRD / Repo Input  -->  session-init                  |
+------------------------------------------------------+
                           |
                           v
+======================================================+
|| Execution Core                                      ||
||----------------------------------------------------||
|| program pre-worker gate                            ||
|| claimable / blocked / orchestrator_review          ||
|| fresh / resume / promptStages                      ||
|| if ambiguous -> gpt-5.4 orchestration fallback     ||
|| gpt-5.3-codex worker                               ||
|| worktree execution                                 ||
|| diff + verification                                ||
+======================================================+
            |                               |
            v                               v
+-------------------------+     +-------------------------+
| audit worker            |     | merge or continue       |
+-------------------------+     +-------------------------+
            \                               /
             \                             /
              v                           v
            +----------------------------------+
            | replan needed?                   |
            +----------------------------------+
                  | yes               | no
                  v                   v
        +--------------------+   +----------------------+
        | orchestration loop |   | refresh-state        |
        +--------------------+   +----------------------+
                  |                   |
                  +---------+---------+
                            v
                 +----------------------+
                 | query / dashboard    |
                 +----------------------+
```

### Recommended Reading

Suggested starting points:

- [SKILL.md](./skills/harness-architect/SKILL.md)
- [TRY-IT.md](./skills/harness-architect/TRY-IT.md)
- [FEEDBACK.md](./skills/harness-architect/FEEDBACK.md)
- [references/schema-contracts.md](./skills/harness-architect/references/schema-contracts.md)
- [references/openclaw-dispatch.md](./skills/harness-architect/references/openclaw-dispatch.md)
- [references/model-routing.md](./skills/harness-architect/references/model-routing.md)

Suggested reading order:

1. `skills/harness-architect/SKILL.md`
2. `skills/harness-architect/references/schema-contracts.md`
3. `skills/harness-architect/references/openclaw-dispatch.md`
4. `skills/harness-architect/references/model-routing.md`
5. `skills/harness-architect/references/git-worktree-playbook.md`
6. `skills/harness-architect/examples/`

### Trial and Feedback

Before running a trial, review:

- [TRY-IT.md](./skills/harness-architect/TRY-IT.md)
- [FEEDBACK.md](./skills/harness-architect/FEEDBACK.md)

Useful feedback areas include:

- which step is hardest to understand
- which documents are too long
- which field names are unclear
- which script fails first
- where weaker models drift most often
- whether concurrency, session, and worktree management feel too heavy

### Repository Positioning

This repository provides an installable `.harness` coordination skeleton, together with a Codex-first set of patterns for planning, execution, audit, state management, and operator-facing tooling.

### License

This repository is licensed under the [MIT License](./LICENSE).
