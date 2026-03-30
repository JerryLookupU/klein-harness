# Klein-Harness

![Klein-Harness Surface](docs/klein-surface-hero.png)

Klein-Harness 是一个 repo-local 的 agent runtime。它的目标不是“再包一层脚本”，而是把任务提交、路由、派发、执行、验证、控制这整条链路收进一个可审计、可恢复、可追踪的运行时里。

一句人话总结：

- 你把任务交给 `harness`
- `harness` 先定边界和规则
- worker 再去执行
- 运行时自己验收结果
- 整个过程都落在仓库本地的 `.harness/` 里

## 它解决什么问题

适合这些场景：

- 你希望在一个真实仓库里持续跑 Codex worker，而不是每次手工复制上下文
- 你需要任务队列、tmux 会话、checkpoint、verify、handoff 这些运行时能力
- 你希望“执行的人不能自己宣布完成”，而是由 runtime 做验收和状态推进
- 你想把规划、执行、验证、控制面拆开，避免 agent 越界

它的核心边界是：

- `tmux` 只负责承载执行会话
- `codex exec` / `codex exec resume` 只负责模型执行
- Go runtime 才是控制面真相来源
- shell 脚本只是兼容入口，不是 runtime authority

## 仓库里有什么

Canonical implementation：

- CLI: `cmd/harness`
- Runtime: `internal/runtime`
- Bootstrap: `internal/bootstrap`
- Routing: `internal/route`
- Dispatch: `internal/dispatch`
- Lease: `internal/lease`
- Verify: `internal/verify`
- Query: `internal/query`
- Executor: `internal/executor/codex` 和 `internal/tmux`

兼容层还保留着，但只是转发：

- `scripts/harness-*.sh`
- `cmd/kh-codex`
- `cmd/kh-orchestrator`
- `cmd/kh-worker-supervisor`

## 运行前需要什么

按仓库当前实现，建议把下面三样准备好：

- `codex`
  - worker 实际是通过原生 `codex exec` / `codex exec resume` 跑起来的
- `tmux`
  - `daemon` / `dashboard` / bounded burst 执行会用到它
  - `install.sh` 会检测它；没有也能安装，但 bootstrap / daemon 能力会受限
- `go`
  - 用来编译 canonical `harness` 二进制
  - 如果没有 Go，安装脚本会跳过二进制构建，兼容 wrapper 仍可回退到仓库里的 `go run`

## 安装

最直接的方式：

```bash
./install.sh --force
```

安装脚本会做这些事：

- 把 skills 安装到 `$CODEX_HOME/skills`，默认是 `~/.codex/skills`
- 把 `harness` 编译到 `$CODEX_HOME/bin`
- 安装兼容 wrapper，例如 `harness-submit`、`harness-control`、`harness-status`
- 更新 `$CODEX_HOME/AGENTS.md` 里的 managed block，但不覆盖非托管内容
- 更新 `$CODEX_HOME/config.toml` 里的 managed profiles，但不覆盖你自己的 profile

当前安装进去的关键 skills 包括：

- `klein-harness`
- `blueprint-architect`
- `qiushi-execution`
- `systematic-debugging`
- `harness-log-search-cskill`
- `markdown-fetch`
- `generate-contributor-guide`

## 5 分钟上手

下面这套命令，是现在这套库最推荐的最小路径。

### 1. 初始化一个仓库

```bash
harness init /path/to/repo
```

这一步会在目标仓库下创建并初始化 `.harness/` 运行时目录。

### 2. 提交一个任务

```bash
harness submit /path/to/repo \
  --goal "Fix failing verify regression" \
  --context docs/prd.md
```

目前 `submit` 已确认支持这些 flag：

- `--goal`
- `--context`
- `--kind`

### 3. 启动调度循环

```bash
harness daemon loop /path/to/repo --interval 30s
```

常用可选项：

- `--model`
- `--approval-policy`
- `--sandbox-mode`
- `--worker-id`
- `--skip-git-repo-check`

### 4. 查看任务状态

```bash
harness tasks /path/to/repo
harness task /path/to/repo T-001
harness control /path/to/repo task T-001 status
```

### 5. 需要看执行现场时 attach

```bash
harness control /path/to/repo task T-001 attach
```

在非交互环境里，这个命令会直接告诉你应该用哪条 `tmux attach-session` 命令去接入现场。

## 日常怎么用

如果你是第一次接手一个 repo，可以直接按这个顺序：

```bash
harness init /path/to/repo
harness submit /path/to/repo --goal "实现某个需求"
harness daemon loop /path/to/repo --interval 30s
harness tasks /path/to/repo
harness control /path/to/repo task T-001 status
```

如果你想开一个可视化面板，而不是只看命令行：

```bash
harness dashboard /path/to/repo --addr 127.0.0.1:7420
```

当前 `dashboard` 默认会把调度循环一起带起来；如果你只想看页面，不想顺手启动 daemon：

```bash
harness dashboard /path/to/repo --no-daemon
```

## 运行时到底怎么推进

当前 canonical flow 是：

1. `harness init`
2. `harness submit`
3. `harness daemon loop` 或 `harness dashboard`
4. route task
5. issue dispatch
6. acquire and claim lease
7. create real tmux session
8. run native codex inside tmux
9. persist checkpoint and outcome
10. ingest verify
11. expose query / control state

如果任务适合直接收口，runtime 会把它编成单个 slice 直接执行。
如果任务复杂，它会先冻结 shared context、task contract、verify skeleton，再把受控 slice 交给 worker。

重点不是“AI 去执行”，而是“runtime 先把执行边界定好”。

## 你会经常用到的命令

Canonical CLI：

```bash
harness init /path/to/repo
harness submit /path/to/repo --goal "Fix failing verify regression"
harness tasks /path/to/repo
harness task /path/to/repo T-001
harness control /path/to/repo task T-001 status
harness control /path/to/repo task T-001 attach
harness control /path/to/repo task T-001 restart-from-stage queued
harness control /path/to/repo task T-001 stop
harness control /path/to/repo task T-001 archive
harness daemon loop /path/to/repo --interval 30s
harness dashboard /path/to/repo --addr 127.0.0.1:7420
```

兼容 wrapper 也还在：

```bash
harness-submit /path/to/repo --goal "Fix failing verify regression"
harness-tasks /path/to/repo
harness-task /path/to/repo T-001
harness-control /path/to/repo task T-001 status
harness-status /path/to/repo
```

但它们只是兼容入口。真正的 runtime source of truth 还是 `harness`。

## `.harness/` 里会落什么

权威状态文件主要在这里：

- `.harness/requests/queue.jsonl`
- `.harness/task-pool.json`
- `.harness/state/dispatch-summary.json`
- `.harness/state/lease-summary.json`
- `.harness/state/session-registry.json`
- `.harness/state/runtime.json`
- `.harness/state/verification-summary.json`
- `.harness/state/tmux-summary.json`
- `.harness/checkpoints/*`
- `.harness/artifacts/*`

其中最值得你先看的通常是：

- `task-pool.json`
  - 当前任务池和状态总览
- `state/runtime.json`
  - 当前 daemon / 调度运行时状态
- `state/tmux-summary.json`
  - tmux 会话摘要
- `artifacts/<TASK>/<DISPATCH>/`
  - 某个任务某次派发的 worker 产物、verify、handoff

派生视图包括：

- `.harness/state/completion-gate.json`
- `.harness/state/guard-state.json`

## 控制面的原则

这套库有几个很关键的设计选择：

- worker 只拥有 task-local execution 权限
- completion / archive / merge 不是 worker 说了算
- verify 是硬门，不是“最好做一下”
- runtime 会把 route、dispatch、lease、checkpoint、verify 统一收在 Go 控制面里

所以你会看到：

- `attach` 是控制动作
- `restart-from-stage` 是控制动作
- `archive` 也要服从 completion gate

## 测试

单元测试：

```bash
go test ./...
```

如果 macOS linker 报 `libtapi.dylib` 签名问题：

```bash
CGO_ENABLED=0 go test ./...
CGO_ENABLED=0 go build ./cmd/harness
```

覆盖导向的 integration tests：

```bash
go test -tags=integration ./...
```

## 深入阅读

如果你已经能跑起来，下一步建议按这个顺序读：

- 架构总览：[docs/klein-architecture.md](docs/klein-architecture.md)
- Runtime 事实模型：[docs/runtime-mvp.md](docs/runtime-mvp.md)
- 命令面：[docs/four-command-surface.md](docs/four-command-surface.md)
- Operator 控制面：[docs/operator-cli.md](docs/operator-cli.md)
- Guardrails：[docs/guard-loop.md](docs/guard-loop.md)
- 迁移说明：[docs/refactor-runtime-migration.md](docs/refactor-runtime-migration.md)
- Qiushi 设计映射：[docs/qiushi-integration.md](docs/qiushi-integration.md)

如果你更想看中文、并且想一次性看完整设计，推荐直接读：

- [docs/project-architecture-complete.zh.md](docs/project-architecture-complete.zh.md)

## 一句话记忆

如果你只记住一件事，那就是：

Klein-Harness 不是“帮 AI 跑任务的脚本集合”，而是“把任务的提交、执行、验证、控制和恢复统一收进 repo-local runtime 的控制面”。
