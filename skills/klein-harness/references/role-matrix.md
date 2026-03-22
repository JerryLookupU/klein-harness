# Role Matrix

在以下情况读取本文件：

- 你要判定 planner / orchestrator / worker 的写权限
- 你要检查 role blur 或路径踩踏
- 你要审计 `context-map.json`、prompt 模板、实际角色边界是否一致

## 角色选择

- `planner`：仅用于 `spec.json` / `context-map.json` 这类蓝图层改写
- `orchestrator`：默认执行控制面角色；负责 claim 回收、task 展开、replan、交接、串行合并
- `worker`：存在可执行 `T-*`，依赖满足，路径清楚；普通实现和 `audit` 复核都落在 worker lane
- `wait`：当前没有安全任务可领

如果任务要改 `work-items.json` / `task-pool.json` / claim / dispatch / lineage，默认应落到 `orchestrator`，不要混成 `planner`。
如果任务要复核某个实现结果、验证链、交接状态或 harness 漂移，优先建成 `kind = "audit"` 的 worker task，而不是额外长出一个独立角色。

## 写权限矩阵

### planner

默认可写：

- `.harness/spec.json`
- `.harness/context-map.json`
- `.harness/progress.md`
- `.harness/lineage.jsonl`

默认禁写：

- 项目业务源码
- `.harness/task-pool.json`
- `.harness/work-items.json`，除非只是为 spec 变更补最小镜像字段
- 别人的 `ownedPaths`

### orchestrator

默认可写：

- `.harness/work-items.json`
- `.harness/task-pool.json`
- `.harness/progress.md`
- `.harness/lineage.jsonl`
- `.harness/drift-log/*.jsonl`
- `.harness/session-registry.json`

默认禁写：

- 项目业务源码
- `.harness/spec.json`，除非当前 orchestration work 明确声明要串行落 spec 修订

### worker

默认可写：

- 当前 task / work item 的 `ownedPaths`
- 当前 task 的 claim / status / handoff 所需的最小回写
- `.harness/progress.md`
- `.harness/lineage.jsonl`

默认禁写：

- `.harness/spec.json`
- `.harness/work-items.json`
- `.harness/context-map.json`
- `.harness/features.json`
- `.harness/standards.md`
- `.harness/verification-rules/manifest.json`
- 其他 task 的 `ownedPaths`

例外：

只有当前任务本身是 `orchestration` / `replan`，且 `ownedPaths` 明确包含这些 `.harness/` 文件时，worker 才能改对应编排文件。

如果当前 task `kind = "audit"`：

- 默认只允许写 `.harness/audit-report.md`、`.harness/progress.md`、`.harness/lineage.jsonl` 与最小 claim/status 回写
- 默认不直接改业务源码
- 发现问题时先写 request，再交回 `orchestrator`

### wait

- 不改代码
- 不抢 claim
- 只记录 blocker / drift / 依赖未满足状态 / 待审计状态

## 回退原则

- 只回退自己引入的错误
- 先记录 lineage / drift
- 再释放任务或触发 replan
