# Git Worktree Playbook

在以下情况读取本文件：

- 你要把 git / diff / worktree 接进 `.harness`
- 你要决定 task 是否需要独立工作区
- 你要设计 merge gate / audit gate

不要在普通 `agent-entry` 时默认加载全文。

## 核心判断

git / diff / worktree 不是额外负担，它们是 harness 的代码隔离层。

目标只有 4 个：

1. 降低并行 worker 互相踩路径
2. 让 audit 能快速看清改动范围
3. 让 merge gate 有稳定基线
4. 让回退和回收更便宜

## 什么时候应该开独立 worktree

默认应该开：

- 当前 task 会改业务源码
- 当前 task `mergeRequired = true`
- 当前 task 命中高冲突路径
- 当前 task 需要长时间运行、反复测试
- 当前 task 需要和别的活跃代码任务并行

默认可以不开：

- 纯 `.harness` 控制面任务
- 纯文档任务且不与其他任务冲突
- 一次性极小补丁，且当前没有并发代码任务

一句话：

- 代码型 task 优先开独立 worktree
- 控制面 task 优先留在主工作区

## 默认映射

推荐一一对应：

- 一个活跃代码型 `task`
- 一个 `branchName`
- 一个 `worktreePath`

推荐命名：

- `branchName = task/<TASK_ID>-<slug>`
- `worktreePath = <repo>/.worktrees/<TASK_ID>-<slug>`

如果任务只是 audit：

- 默认仍可用 worker lane
- 可以新开 audit worktree
- 也可以直接在只读主工作区做 diff 采样
- 是否新开 worktree，取决于 audit 是否需要跑隔离验证

## 谁负责什么

### orchestrator

负责：

- 分配 `branchName`
- 分配 `worktreePath`
- 创建 worktree
- 写入 `dispatch`
- 串行 merge
- 回收 worktree

不负责：

- 直接做业务实现

### worker

负责：

- 只在自己的 `worktreePath` 中改动
- 只提交 `ownedPaths` 内的改动
- 回写 `diffSummary`
- 在退出前确认工作区状态

不负责：

- 自己决定换 branch
- 自己决定合并到 integration branch

## 推荐命令习惯

创建：

```bash
git worktree add .worktrees/T-002-fix-delta -b task/T-002-fix-delta orch/spec-S-003
```

查看：

```bash
git worktree list
git -C .worktrees/T-002-fix-delta status --short
```

看改动范围：

```bash
git -C .worktrees/T-002-fix-delta diff --stat refs/heads/orch/spec-S-003...HEAD
git -C .worktrees/T-002-fix-delta diff --name-only refs/heads/orch/spec-S-003...HEAD
```

审计时看具体差异：

```bash
git -C .worktrees/T-002-fix-delta diff refs/heads/orch/spec-S-003...HEAD -- src/deep-archive/
```

如果你要比较两轮提交的演进，而不是当前工作区脏改动：

```bash
git range-diff <old-base>...<old-head> <new-base>...<new-head>
```

## diff 基线怎么选

默认优先级：

1. `diffBase`
2. `baseRef`
3. `integrationBranch`

不要优先用：

- 模糊的当前 `HEAD`
- 没写进 `.harness` 的临时 commit

推荐：

- worker 回写 `diffSummary`
- audit 基于 `diffBase...branchName` 做复核
- merge 前再做一次 `diffBase...HEAD` 的最终检查

## 和 session 的关系

不要把 session 分支和 git branch 混成一回事。

- git branch / worktree
  - 是代码隔离
- session family / resume
  - 是上下文隔离

推荐：

- 同一个代码 task 可以续用旧 session
- 但如果它改动范围已经和旧 branch 不一致，优先新开 branch / worktree
- audit task 可以参考别的 worker session，但不等于要续写同一代码 branch

## audit 怎么借力 diff

audit 最应该看的不是“这个 worker 说自己改了什么”，而是：

1. `diffBase...branchName` 改了哪些路径
2. 这些路径是否落在 `ownedPaths`
3. handoff 写的结论和 diff 是否一致
4. 验证是否覆盖了 diff 涉及的关键区域

如果 audit 发现：

- diff 超出 `ownedPaths`
- diff 命中共享账本 merge gate
- diff 和 handoff 叙述不一致

优先：

1. 写 `audit-report.md`
2. 写 request
3. 交回 `orchestrator`

## 最佳策略

最稳的默认值：

- `gpt-5.4` 负责 route / plan / branch / worktree 分配
- `gpt-5.3-codex` 在独立 worktree 里执行
- `audit worker` 结合 diff 复核结果
- `orchestrator` 串行 merge 和回收

一句话：

让 session 管上下文，让 worktree 管代码，让 diff 管证据。
