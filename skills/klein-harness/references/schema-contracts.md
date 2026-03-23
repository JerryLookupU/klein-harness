# Schema Contracts

在以下情况读取本文件：

- 你要生成或刷新 `.harness/*` 文件
- 你要检查 example 是否和 SKILL.md 漂移
- 你要判断某项工作该留在 `work-items.json` 还是进入 `task-pool.json`

不要在普通 `agent-entry` 时默认加载全文。

## 统一产物约束

- 快照型 JSON 产物使用 `schemaVersion`
- 快照型结构化产物带 `generator` 和 `generatedAt`
- `lineage.jsonl`、`feedback-log.jsonl`、`root-cause-log.jsonl` 与 `drift-log/*.jsonl` 属于 append-only 事件日志；它们按事件记录 `timestamp` 和 `generator`，不要求文件级 `schemaVersion` / `generatedAt`
- `lineage.jsonl`、`feedback-log.jsonl`、`root-cause-log.jsonl` 与 `drift-log/*.jsonl` 只追加，不覆盖
- ID 一经分配不要重写

如果项目根存在 `AGENTS.md`，把它视作 harness 的相邻控制面文件，而不是可忽略的额外文档。

推荐最少包含：

- `## SOUL`
- `## Working Rules`

规范化时：

- 优先只改 `SOUL` / `Persona` / `人格` 段
- 如果没有该段，则新增 `## SOUL`
- 不要覆盖其他 repo 级工程规则

如果 harness 采用“两层初始化编排”，推荐额外表达当前编排阶段：

- `draft`
- `refinement`
- `execution-ready`

这个阶段值可以放在 `spec.json`、`progress.md` 或等效状态文件里。

如果用户关心性能、CLI 响应速度或 query 频率较高，推荐额外维护一组热路径状态文件：

- `state/current.json`
- `state/runtime.json`
- `state/blueprint-index.json`
- `state/feedback-summary.json`
- `state/root-cause-summary.json`

推荐规则：

- `progress.md` 继续服务人类阅读
- `state/*.json` 优先服务机器读取
- query / dashboard / daemon / operator 脚本默认先读 `state/*.json`
- state 缺失时再回退到 `progress.md` / `task-pool.json` / `spec.json`
- 下游 worker 默认读取顺序应为：`current/runtime/request-summary/lineage-index -> log-index / compact log -> raw runner log`
- 不要默认让 worker 扫全文 `.harness/state/runner-logs/*.log`

## `lint` 复审节奏

`lint.lastReview` / `lint.nextReview` 是 drift 扫描的主字段。

推荐规则：

- `lastReview` 和 `nextReview` 优先使用带时区的 ISO 时间戳，不要只写日期
- 刚改过、仍在快速变化的面，优先用小时级复审，不要默认 30 天
- 推荐用 Fibonacci 小时节奏递增：`1h -> 3h -> 5h -> 8h -> 13h -> 21h -> 34h -> 55h -> 89h -> 144h`
- 稳定后再放宽到天级复审
- 兼容旧字段：已有 `cycleDays` 可以保留，但新产物优先增加 `cycleHours` / `cadenceKind` / `cadenceStep`

推荐最小结构：

- `lastReview`
- `nextReview`
- `cycleHours`
- `cadenceKind`
- `cadenceStep`

如果需要兼容旧仓库，可额外保留：

- `cycleDays`

## ID 约定

- `STD-xxx`：标准
- `VR-xxx`：验证规则
- `F-xxx`：feature
- `WI-xxx`：待办项
- `S-xxx`：蓝图修订号
- `TB-xxx`：蓝图片段
- `T-xxx`：可领取任务

## 产物与 example 对照

- `features.json` -> `examples/features.example.json`
- `standards.md` -> `examples/standards.example.md`
- `verification-rules/manifest.json` -> `examples/manifest.example.json`
- `work-items.json` -> `examples/work-items.example.json`
- `spec.json` -> `examples/spec.example.json`
- `task-pool.json` -> `examples/task-pool.example.json`
- `context-map.json` -> `examples/context-map.example.json`
- `progress.md` -> `examples/progress.example.md`
- `AGENTS.md template` -> `examples/AGENTS.example.md`
- `operator status output` -> `examples/operator-status.example.txt`
- `replan requests` -> `examples/replan-requests.example.json`
- `stop requests` -> `examples/stop-requests.example.json`
- `session registry` -> `examples/session-registry.example.json`
- `lineage.jsonl` -> `examples/lineage.example.jsonl`
- `drift-log/*.jsonl` -> `examples/drift-log.example.jsonl`
- `feedback-log.jsonl` -> `examples/feedback-log.example.jsonl`
- `root-cause-log.jsonl` -> `append-only RCA log`
- `tooling-manifest.json` -> `examples/tooling-manifest.example.json`
- `session-init.sh` -> `examples/session-init.example.sh`
- `audit-report.md` -> `examples/audit-report.example.md`
- `audit worker prompt (gpt-5.3-codex)` -> `examples/audit-worker-prompt-gpt-5.3-codex.example.md`
- `state/current.json` -> `examples/current-state.example.json`
- `state/runtime.json` -> `examples/runtime-state.example.json`
- `state/blueprint-index.json` -> `examples/blueprint-index.example.json`
- `state/feedback-summary.json` -> `examples/feedback-summary.example.json`
- `state/root-cause-summary.json` -> `hot-state RCA summary`
- `state/log-index.json` -> `compact log hot index`
- `state/research-index.json` -> `research memo hot index`

如果项目准备把 CLI 模板复制到 `.harness/bin` / `.harness/scripts`，推荐再维护：

- `tooling-manifest.json`

推荐语义：

- 记录已安装的 CLI / script
- 记录来源模板
- 记录版本或刷新时间
- refresh 时据此判断是否需要增量更新

## `.harness/log-<taskId>.md`

如果项目需要把 worker transcript 压成跨 worker 可共享的 handoff，建议额外维护：

- `.harness/log-<taskId>.md`

推荐 front matter 至少包含：

- `schemaVersion`
- `generator`
- `generatedAt`
- `taskId`
- `requestId`
- `bindingId`
- `sessionId`
- `tmuxSession`
- `roleHint`
- `kind`
- `status`
- `shareability`
- `rawLogPath`
- `promptPath`
- `verificationResultPath`
- `diffSummaryPath`
- `ownedPaths`
- `tags`
- `severity`

推荐 body 至少包含：

- `# One-screen summary`
- `## Cross-worker relevant facts`
- `## Decisions and assumptions`
- `## Touched contracts / paths`
- `## Blockers / risks`
- `## Verification`
- `## Evidence refs`

推荐规则：

- 不要转储完整 transcript
- 不要写 hidden reasoning
- 默认只保留下游 worker 真需要的事实和引用
- 优先引用 raw log / prompt / verification 证据路径，而不是复制大段内容

## `state/log-index.json`

如果 query / dashboard / operator 需要热路径读取 compact logs，建议额外维护：

- `.harness/state/log-index.json`

推荐字段：

- `schemaVersion`
- `generator`
- `generatedAt`
- `compactLogCount`
- `logsByTaskId`
- `logsByRequestId`
- `logsBySessionId`
- `recentHighSignalLogs`
- `openBlockers`
- `recurringTags`

## `.harness/research/<slug>.md`

如果 blueprint 需要先做 gated research，建议把外部资料先收敛成 repo-local memo：

- `.harness/research/<slug>.md`

推荐 front matter：

- `schemaVersion`
- `generator`
- `generatedAt`
- `slug`
- `researchMode`
- `question`
- `sources`

推荐规则：

- blueprint 优先消费 memo，而不是直接消费外部网页原文
- `researchMode` 先固定为 `none | targeted | deep`
- 研究 memo 是热共享结论，外部原始页面仍然是冷证据

## `state/research-index.json`

如果 blueprint flow 需要热路径读取 research memo 概览，建议额外维护：

- `.harness/state/research-index.json`

推荐字段：

- `schemaVersion`
- `generator`
- `generatedAt`
- `memoCount`
- `researchModes`
- `recentMemos`
- `bySlug`

## `feedback-log.jsonl`

如果项目需要稳定恢复失败上下文，建议追加维护：

- `.harness/feedback-log.jsonl`

这是 append-only 失败事件流，不要覆盖旧事件。

每条记录推荐至少包含：

- `id`
- `taskId`
- `sessionId`
- `role`
- `workerMode`
- `feedbackType`
- `severity`
- `source`
- `step`
- `triggeringAction`
- `message`
- `timestamp`

推荐 `feedbackType` 先固定一小组：

- `illegal_action`
- `verification_failure`
- `dependency_missing`
- `path_conflict`
- `session_conflict`
- `execution_error`
- `timeout`
- `replan_required`

推荐规则：

- 非法越权动作不要混成普通 execution failure
- worker / audit 失败时先写结构化 feedback，再写长文本总结
- `feedback-log.jsonl` 保留完整历史，供 audit 和回放使用
- 不要把 RCA 结论写回 `feedback-log.jsonl`

## `root-cause-log.jsonl`

如果项目需要让 bug / feedback 进入显式根因分配闭环，建议额外维护：

- `.harness/root-cause-log.jsonl`

这是 append-only RCA 决策流，不要覆盖旧结论。

每条记录推荐至少包含：

- `rcaId`
- `bugId` 或 `sourceRequestId`
- `requestId`
- `taskId`
- `sessionId`
- `worktreePath`
- `symptomFeedbackIds`
- `primaryCauseDimension`
- `contributingCauseDimensions`
- `ownerRole`
- `repairMode`
- `confidence`
- `status`
- `evidenceRefs`
- `allocatedAt`
- `preventionTarget`
- `preventionAction`

固定 taxonomy：

- `spec_acceptance`
- `blueprint_decomposition`
- `routing_session`
- `execution_change`
- `verification_guardrail`
- `runtime_tooling`
- `environment_dependency`
- `merge_handoff`
- `underdetermined`

推荐规则：

- `feedback-log.jsonl` 记录症状；`root-cause-log.jsonl` 记录结论，不要混写
- RCA 更新也走 append-only，按 `rcaId` 追加新状态，再由 summary 归并 latest
- 如果 lineage 证据不足，先记 `underdetermined`，再发 `audit` / `research`

## `state/feedback-summary.json`

如果 query / dashboard / routing 需要热路径读取最近失败窗口，建议额外维护：

- `.harness/state/feedback-summary.json`

推荐字段：

- `schemaVersion`
- `generator`
- `generatedAt`
- `feedbackLogPath`
- `feedbackEventCount`
- `errorCount`
- `criticalCount`
- `illegalActionCount`
- `tasksWithRecentFailures`
- `byType`
- `bySeverity`
- `recentFailures`
- `taskFeedbackSummary`

推荐规则：

- `recentFailures` 默认只保留最近 5 条 `severity >= error`
- `taskFeedbackSummary[taskId].recentFailures` 默认只保留最近 3 条
- orchestrator / worker prompt 默认优先读 `feedback-summary.json`，不要先扫全文 `feedback-log.jsonl`
- 如果最近失败窗口命中 `illegal_action` / `path_conflict` / `session_conflict`，默认优先停下而不是盲目续跑

## `state/root-cause-summary.json`

如果 report / dashboard / operator 需要热路径读取 RCA 概览，建议额外维护：

- `.harness/state/root-cause-summary.json`

推荐字段：

- `schemaVersion`
- `generator`
- `generatedAt`
- `rootCauseLogPath`
- `rcaCount`
- `openCount`
- `underdeterminedCount`
- `byPrimaryCauseDimension`
- `byOwnerRole`
- `openItems`
- `recurringRootCauses`
- `bugsMissingLineageCorrelation`
- `recentAllocations`

推荐规则：

- summary 只读 latest RCA state，不改写 append-only log
- `openItems` 默认展示当前未关闭 RCA
- `bugsMissingLineageCorrelation` 专门暴露 correlation 弱的 case
- operator / report 优先读 `root-cause-summary.json`，不要全文扫 `root-cause-log.jsonl`

## `work-items.json`

每个 item 至少有：

- `id`
- `kind`
- `title`
- `summary`
- `status`
- `priority`
- `roleHint`
- `featureIds`
- `dependsOn`
- `ownedPaths`
- `acceptance`
- `claim`
- `dispatch`
- `handoff`

推荐增加：

- `description`
- `parentWorkItemId`
- `childWorkItemIds`
- `lineagePath`
- `planningStage`
- `replanScope`
- `candidateResumeSessionIds`
- `lastKnownSessionId`
- `sessionFamilyId`
- `cacheAffinityKey`
- `operatorNotes`
- `reviewOfTaskIds`
- `auditScope`
- `auditVerdict`
- `workerMode`
- `branchName`
- `worktreePath`
- `diffBase`
- `diffSummary`

建议 `claim` 至少有：

- `agentId`
- `role`
- `nodeId`
- `leasedAt`
- `leaseExpiresAt`

如果项目要稳定追踪 worker 实际接入的 session，建议 `claim` 再增加：

- `boundSessionId`
- `boundResumeStrategy`
- `boundFromTaskId`
- `boundAt`

建议 `dispatch` 至少有：

- `runner`
- `targetKind`
- `targetSelector`
- `entryRole`
- `taskContextId`
- `logPath`
- `heartbeatPath`
- `maxParallelism`
- `cooldownSeconds`

如果项目采用 git / worktree 隔离，建议 `dispatch` 再增加：

- `workspaceRoot`
- `worktreePath`
- `branchName`
- `baseRef`
- `diffBase`

建议 `handoff` 至少有：

- `nextSuggestedWorkItemIds`
- `nextSuggestedTaskIds`
- `replanOnFail`
- `mergeRequired`
- `returnToRole`

推荐语义：

- `branchName`
  - 当前 task 绑定的工作 branch
- `worktreePath`
  - 当前 task 默认执行目录
- `diffBase`
  - 提交前和审计时对比改动的基线
- `diffSummary`
  - 可选的人类可读改动摘要

## `task-pool.json`

每个 task 至少有：

- `taskId`
- `workItemId`
- `kind`
- `roleHint`
- `title`
- `summary`
- `status`
- `dependsOn`
- `ownedPaths`
- `forbiddenPaths`
- `verificationRuleIds`
- `claim`
- `dispatch`
- `handoff`

推荐增加：

- `description`
- `parentTaskId`
- `childTaskIds`
- `lineagePath`
- `planningStage`
- `replanScope`
- `stopPolicy`
- `checkpointPolicy`
- `resumeStrategy`
- `preferredResumeSessionId`
- `candidateResumeSessionIds`
- `lastKnownSessionId`
- `sessionFamilyId`
- `cacheAffinityKey`
- `routingModel`
- `executionModel`
- `routingReason`
- `operatorNotes`
- `reviewOfTaskIds`
- `auditScope`
- `auditVerdict`
- `workerMode`

`work-item` 是待办池。
`task` 是执行单元。
`task-pool` 里可以同时存在：

- 当前可 claim 的 task
- 已 claim / active 的 task
- 依赖已声明但尚未解开的 queued / blocked task

不要把 `task` 和 `当前可 claim` 完全等同；是否当前可领，还要结合 `status` 与 `dependsOn` 判断。
也不要把 `task` 和 `work-item` 混掉。

如果项目要把审计纳入常规闭环，推荐增加一种显式 task：

- `kind = "audit"`
- `roleHint = "worker"`
- `workerMode = "audit"`

推荐语义：

- `reviewOfTaskIds`
  - 当前 audit task 复核哪些实现 task / orchestration task
- `auditScope`
  - 例如 `task-result` / `subtree` / `handoff` / `harness-health`
- `auditVerdict`
  - `pending | pass | warn | fail`

默认建议：

- audit task 也是 leaf task
- audit task 默认在 `execution-ready` 时可 claim
- audit task 默认由 `gpt-5.4` 先做 routing / prompt 精化，再由 `gpt-5.3-codex` 执行
- audit task 不直接改业务源码，只写审计产物与最小状态回写
- audit task 发现问题时，优先写 `replan-requests.json` / `stop-requests.json`，再把结论交回 `orchestrator`
- 如果 audit task 需要看精确改动范围，优先对 `diffBase...branchName` 做比对

## `harness-route-session` 输出契约

如果项目采用“程序 gate + LLM fallback”派发，建议 `harness-route-session` 至少输出：

- `taskId`
- `routingMode`
- `needsOrchestrator`
- `dispatchReady`
- `gateStatus`
- `gateReason`
- `resumeStrategy`
- `preferredResumeSessionId`
- `candidateResumeSessionIds`
- `promptStages`
- `recentFailures`

推荐语义：

- `routingMode = "programmatic" | "llm-fallback"`
- `needsOrchestrator = true` 表示程序 gate 不足以安全放行，必须回退给 `gpt-5.4`
- `dispatchReady = true` 表示 runner 可以直接投递 worker
- `gateStatus = "claimable" | "blocked" | "orchestrator_review"`
- `promptStages` 由程序 gate 选择，如 `["start", "execute", "recover"]` 或 `["audit"]`
- `recentFailures` 默认只带当前 task 最近 3 条高严重度失败

## git / worktree 推荐结构

如果项目本身是 git 仓库，建议把 branch / worktree 也纳入 task 契约。

默认建议：

- 代码型 worker task：
  - `branchName` 必填
  - `worktreePath` 优先填写
  - `diffBase` 必填
- 纯 `.harness` 控制面 task：
  - 可以留在主工作区
  - 但如果会碰 merge gate，也可以单独分配 control-plane worktree

推荐规则：

- 一个活跃代码型 task，默认绑定一个 branch
- 一个活跃代码型 branch，默认绑定一个 worktree
- 同一个 worktree 默认不要并行承载多个活跃代码 task
- orchestrator 负责 branch / worktree 分配与回收
- worker 负责在自己绑定的 worktree 中做改动、跑 diff、回写摘要
- audit 优先基于 `diffBase...branchName` 复核，不要只看工作区脏文件

## 树状谱系推荐

如果项目支持局部重编排，优先把任务结构建成树：

- feature
- parent work item / parent task
- child / subtask
- leaf task

推荐约束：

- 默认持久层级使用 3 层：`feature -> parent -> leaf`
- 最多 4 层；第 4 层只在确实需要独立 stop / replan / claim 时启用
- 超过 4 层的细分，优先放进 task 自身的 checklist / description，而不是继续加持久谱系层级
- 真正执行代码/文档变更的，默认只落在 `leaf task`
- `parent` 用来承载局部 replan 边界
- `lineagePath` 默认表达“从 feature 到当前节点的完整逻辑祖先链”
- 如果当前节点有 parent，则 `lineagePath` 必须显式包含 parent
- 默认只允许在 `self / subtree / parent` 范围内 replan

推荐把 `parent` 设计成“冲突域”而不是“文档目录树”：

- 共享关键路径多的任务，放在同一个 `parent`
- 往往需要一起回退的任务，放在同一个 `parent`
- 可以独立验证、独立停止、独立回退的任务，尽量拆到不同 `parent`

这样 worker 在执行前，可以直接判断自己是否和兄弟任务冲突，而不是扫描整池任务。

推荐示意：

- parent work item: `["F-003", "WI-001"]`
- child work item: `["F-003", "WI-001", "WI-002"]`
- parent task: `["F-003", "WI-001", "T-001"]`
- child task: `["F-003", "WI-001", "T-001", "WI-002", "T-002"]`

## 任务文本推荐

默认把任务文本拆成三层：

- `title`
  - 短标题，用于表格或列表
- `summary`
  - 一句话进度说明，用于 status/watch/CLI
- `description`
  - 详细目标、边界、停点、风险

如果用户说“我要更友好的进度查看”，优先补 `summary` 字段。

## task readiness gate

如果项目采用“两层初始化编排”，默认只有 `execution-ready` 阶段的 leaf task 才应被 worker claim。

如果同时存在全局 `planningStage` 和 task 自身的 `planningStage`：

- task-level `planningStage` 优先
- 全局 `planningStage` 只作为 coarse-grained 状态提示
- 守护进程和 `claim-next` 应按 task-level 字段判断是否可放行

只有同时满足以下条件，才把工作放进 `task-pool.json`：

- 依赖清楚
- `ownedPaths` 清楚
- `forbiddenPaths` 或等效边界清楚
- `verificationRuleIds` 清楚，或已明确说明该任务是纯编排任务、没有可运行验证规则
- 单个 agent 能安全推进

如果依赖未解开、路径边界不清、验证不清、或本质上仍是编排问题，就留在 `work-items.json`。

### draft / refinement 推荐差异

`draft` 阶段至少应有：

- 初始谱系
- 初始冲突域
- 大致 `ownedPaths`
- 祖先 / 子树边界

`refinement` 阶段应继续补齐：

- `summary`
- `description`
- 更精确的 `ownedPaths`
- `acceptance`
- 验证方式

如果 task 还停留在 `draft`，默认不要释放给 worker。

推荐：

- orchestration task 可以在 `refinement` 阶段执行
- audit task 可以在 `execution-ready` 阶段执行
- 普通 worker task 默认只有 `execution-ready` 才可 claim

## session / cache 推荐结构

如果项目希望通过 `codex exec resume` 提高 cache hit，建议显式记录 session 元数据，而不是靠人记忆。

推荐字段：

- `orchestrationSessionId`
  - `gpt-5.4` 编排主线 session
- `sessionFamilyId`
  - 一般对应 feature 或 parent conflict group
- `cacheAffinityKey`
  - 表示稳定上下文前缀归属
- `resumeStrategy`
  - `fresh | resume`
- `preferredResumeSessionId`
  - 当前最推荐续用的 session
- `branchOfSessionId`
  - 当前 session 的逻辑父 session；只用于 registry 追踪，不代表原生 CLI 提供真正 fork

推荐规则：

- `gpt-5.4` 应有一条持续的 orchestration session，专门承载 draft / refinement / pre-worker routing fallback / replan
- `gpt-5.4` 的 orchestration session 默认视为单写主线，不要让多个并发任务同时 `resume` 同一个 orchestration session
- 如果需要“分支干活”，优先让 `gpt-5.4` 基于 orchestration session 产出 routing 结论，再新开或续用别的 worker session；不要把 orchestration session 当成可并发 fork 的共享工作树
- 只有“同 parent / 同冲突域 / 同角色 / 高上下文重合”的直接后续任务才优先 `resume`
- 如果任务同时依赖多个上游 worker session，允许由 `gpt-5.4` 从 `candidateResumeSessionIds` 中选择一个真正绑定的 session
- sibling 并行任务默认不要同时 resume 同一个 session
- 遇到 `replan / rollback / superseded` 后，旧 worker session 默认降级，不直接复用
- task 一旦被 claim，应把实际绑定的 `boundSessionId` 写入 claim；失败或报错时也不要清空这个值
- 失败或报错后，应把 `lastKnownSessionId` 与失败原因回写到 session registry，供后续回调或继续执行
- 如果系统要表达“逻辑分支”，建议在 `session-registry.json` 中记录 `branchOfSessionId` / `branchRootSessionId`，不要假设 `codex exec resume` 自带原生分叉语义

### orchestration / replan 任务的验证例外

对于 `orchestration` / `replan` task：

- `verificationRuleIds` 允许为空数组
- 但必须由 `acceptance`、`ownedPaths`、`handoff`、`lineage` 更新要求或其他结构性约束补足可验收性
- 如果连结构性验收都说不清，就不要进 `task-pool.json`

## request 驱动重编排推荐

如果项目支持隐式 orchestrator，建议给 request 独立结构，而不是散落在备注里。

一次 replan / rollback request 至少应表达：

- `requestId`
- `kind`
- `anchorTaskId`
- `scope`
- `reason`
- `suggestedStopTaskIds`
- `stopMode`
- `requestedBy`
- `requestedAt`

推荐 `scope` 只允许：

- `self`
- `subtree`
- `parent`

推荐 `stopMode` 只允许：

- `graceful`
- `immediate`

建议把 request 稳定存放到：

- `.harness/replan-requests.json`
- `.harness/stop-requests.json`
- `.harness/session-registry.json`

这样 daemon、operator、worker 可以共用同一份状态。

### stop request 推荐契约

`stop request` 不要另起一套完全无关的字段。

一次 stop request 至少应表达：

- `requestId`
- `kind`
- `targetTaskId`
- `scope`
- `reason`
- `stopMode`
- `requestedBy`
- `requestedAt`
- `deadlineSeconds`
- `status`

推荐：

- `kind = "stop"`
- `scope = "self"`，除非明确要按子树停
- `stopMode` 仍使用 `graceful | immediate`

如果 stop request 是由 replan request 派生，建议再加：

- `originRequestId`

## `context-map.json`

如果用户关心 Codex / Claude 的 cache hit，`context-map.json` 不应该只是片段索引，还应服务“稳定上下文前缀”。

推荐把上下文拆成：

- `globalPrefix`
  - 很少变化
  - 例如 standards、spec 摘要、全局规则
- `rolePrefix`
  - 按 planner / orchestrator / worker 切分
  - 尽量稳定
- `taskSuffix`
  - 当前 task 的动态细节
  - 允许频繁变化

推荐原则：

- 高频变化的信息尽量放到 suffix
- prefix 段落顺序稳定
- 只对真正变化的 segment 重算 hash
- operator 输出和日志不要混进 prefix

这样更容易让 Codex 命中前缀缓存，而不是每轮都重算整段上下文。

## task status 推荐扩展

除了常见的 `queued / active / completed / blocked`，如果项目有并行和 replan，推荐支持：

- `pause_requested`
- `finishing_then_pause`
- `stop_requested`
- `superseded`

这样守护进程、worker、operator 都能区分：

- 任务是被请求暂停
- 任务正在安全收尾
- 任务应立即退出
- 任务已经被新蓝图取代

## `session-init.sh`

- 只读校准
- 唯一允许写入的是 `drift-log/` 追加
- 退出码约定：
  - `0` = healthy
  - `1` = drift found
  - `2` = harness missing
  - `3` = parse error
