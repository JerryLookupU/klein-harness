# harness-architect

`harness-architect` 是一套给项目建立和维护 `.harness/` 协作系统的 skill 与模板仓库。

它当前主要面向 `Codex` 工作流设计，默认优先考虑：

- `codex exec`
- `codex exec resume`
- `gpt-5.4` 编排
- `gpt-5.3-codex` 执行

同时它也保留了对 Claude / 其他 agent 工作流的兼容思路，但仓库里的命令、session 路由、prompt 模板和 operator CLI，默认都是按 `Codex-first` 来组织的。

它面向这类场景：

- 你手里有一份 PRD，想把它稳定落成项目
- 你要让多个 Codex/Claude agent 接力或并行干活
- 你不想每次换模型、换会话、换人后都重新猜上下文
- 你希望别人能直接试用这套方法，并给你真实反馈

它的目标不是把文件铺满，而是让后续进入项目的 agent 能稳定回答这几个问题：

- 现在项目在做什么
- 当前该先编排还是先执行
- 哪个 task 可以认领
- 哪个 session 应该续用
- 哪个 worktree 应该执行
- 出错后该怎么停、怎么回退、怎么交接

## 这是什么仓库

这个仓库现在是一个公开试用包，包含三类东西：

- `SKILL.md`
  skill 主说明书
- `references/`
  协议、路由、worktree、prompt、query 等参考文档
- `examples/`
  可以直接抄到项目里的模板、脚本、JSON 和 prompt

如果你要让别人试用，直接把这个目录发到 Git 即可。

## 快速开始

1. 先通读 [SKILL.md](./SKILL.md)
2. 选一个测试项目目录
3. 用安装脚本把最小 CLI 和脚本落到该项目的 `.harness/`
4. 先跑 `query` / `dashboard`
5. 再按你的项目需要接入 routing、worktree、audit

安装最小工具集：

```bash
./examples/harness-install-tools.example.sh <PROJECT_ROOT>
```

刷新热状态：

```bash
python3 .harness/scripts/refresh-state.py .
```

看总览：

```bash
.harness/bin/harness-dashboard .
```

看结构化查询：

```bash
.harness/bin/harness-query overview .
```

## 它解决什么问题

这套 skill 重点解决 5 类问题：

1. 长时间运行任务的状态恢复
2. 多 worker 并发时的路径冲突和编排漂移
3. `gpt-5.4` 和 `gpt-5.3-codex` 的模型分工
4. `session / worktree / diff / audit` 的闭环
5. 给人和工具都能读的 operator / query 面

## 核心模型

- `gpt-5.4`
  - 负责 orchestration、pre-worker routing、prompt 精化、replan
- `gpt-5.3-codex`
  - 负责 worker 执行
- `orchestrationSessionId`
  - 单写主线 session
- `worktree`
  - 代码隔离层
- `diff`
  - 审计和 merge 的证据层
- `state/*.json`
  - 机器热路径

一句话：

`session` 管上下文，`worktree` 管代码，`diff` 管证据，`.harness` 管状态。

## 最小安装面

封版后的最小推荐安装集是：

- `.harness/bin/harness-query`
- `.harness/bin/harness-dashboard`
- `.harness/scripts/query.py`
- `.harness/scripts/refresh-state.py`
- `.harness/tooling-manifest.json`

安装模板：

```bash
./examples/harness-install-tools.example.sh <PROJECT_ROOT>
```

## 最小热路径

推荐让机器优先读这些：

- `.harness/state/current.json`
- `.harness/state/runtime.json`
- `.harness/state/blueprint-index.json`

推荐让人优先读这些：

- `.harness/progress.md`
- `.harness/work-items.json`
- `.harness/task-pool.json`
- `.harness/spec.json`

每轮 orchestration / daemon / session 结束前，刷新热状态：

```bash
python3 .harness/scripts/refresh-state.py .
```

## 常用 CLI

机器可读查询：

```bash
.harness/bin/harness-query overview .
.harness/bin/harness-query progress .
.harness/bin/harness-query current .
.harness/bin/harness-query blueprint .
.harness/bin/harness-query task . T-004
```

人类操作面板：

```bash
.harness/bin/harness-dashboard .
.harness/bin/harness-dashboard . T-004
.harness/bin/harness-dashboard . T-004 --watch 2
```

## 典型执行链

```text
session-init
-> gpt-5.4 orchestration
-> pre-worker routing
-> gpt-5.3-codex worker
-> audit worker
-> merge / replan / stop
-> refresh-state
```

## 目录说明

`SKILL.md`
- 主说明书，定义模式、gate、角色和默认执行链

`references/`
- 细分参考：
  - `schema-contracts.md`
  - `openclaw-dispatch.md`
  - `model-routing.md`
  - `role-matrix.md`
  - `bash-python-toolkit.md`
  - `git-worktree-playbook.md`
  - `progressive-prompt-exposure.md`

`examples/`
- 可直接抄用的模板
- 包括：
  - `.harness` 结构文件
  - prompt 模板
  - query / dashboard / route / worktree / diff / install / refresh-state CLI

## 推荐使用顺序

1. 先看 `SKILL.md`
2. 需要字段契约时看 `references/schema-contracts.md`
3. 需要调度链时看 `references/openclaw-dispatch.md`
4. 需要模型/session 规则时看 `references/model-routing.md`
5. 需要 worktree/diff 时看 `references/git-worktree-playbook.md`
6. 需要 CLI 时直接抄 `examples/`

## 试用和反馈

如果你准备让别人试用，建议先看：

- [TRY-IT.md](./TRY-IT.md)
- [FEEDBACK.md](./FEEDBACK.md)

推荐收集这几类反馈：

- 哪个环节最难理解
- 哪些文档太长
- 哪些字段命名不够直观
- 哪个脚本最先坏
- 弱模型最容易在哪一步跑偏
- 并发、session、worktree 的心智成本高不高

## 当前定位

这份 skill 已经不是“生成几份 `.harness` 文件”的脚手架。

它更接近一个小型 agent coordination runtime：

- 可安装
- 可查询
- 可并发
- 可审计
- 可恢复
- 可封版

## License

This repository is licensed under the MIT License. See [LICENSE](./LICENSE).
