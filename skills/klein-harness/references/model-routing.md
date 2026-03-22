# Model Routing

在以下情况读取本文件：

- 用户要求不同模型承担不同阶段
- 你要决定某个 task 用 `fresh` 还是 `resume`
- 你要设计 session 记录结构
- 你要让高阶模型为低阶 worker 生成更稳的 prompt

## 默认分工

如果用户明确要求“高阶模型做编排，低阶模型做执行”，默认按下面这套：

- 程序 gate
  - claimable / blocked 判断
  - dependsOn / ownedPaths / active session 冲突判断
  - `fresh / resume` 的规则内决策
  - prompt 层级选择
- `gpt-5.4`
  - draft orchestration
  - refinement orchestration
  - task routing fallback
  - 语义性 `fresh / resume` 判断
  - 给 `gpt-5.3-codex` 生成 worker prompt
- `gpt-5.3-codex`
  - 按已声明 task 执行
  - 不自行重做编排
  - 不自行重做 session routing
  - 用最明确的 prompt 和边界干活

这是默认模型约束，不要写成“高阶模型”“低阶模型”这种模糊代称后就结束。
落到 `.harness/*` 或调度模板时，优先显式写出：

- `routingModel = "gpt-5.4"`
- `executionModel = "gpt-5.3-codex"`
- `orchestrationSessionId = "<gpt-5.4-session-id>"`

## 推荐一条持续的 orchestration session

`gpt-5.4` 不只是单次判断器，更适合有一条持续追加的编排主线 session。
但这条 session 不应该成为每个 worker 派发前的默认路径。

这条 session 默认负责：

- 初始 draft orchestration
- refinement orchestration
- pre-worker routing fallback
- replan / rollback
- session 连接判断

推荐原则：

- 整个 harness 默认只有一条活跃的 `orchestrationSessionId`
- 这条 session 给 `gpt-5.4`
- worker 不直接共用这条 session
- worker 只消费程序 gate 或这条 session 产出的 routing 决策
- worker 可以因为依赖关系接入别的 worker session，但必须先经过程序 gate；若存在歧义，再由 `gpt-5.4` 明确选中后再 claim 绑定

## 为什么不要让 `gpt-5.3-codex` 自己判断 resume

- 它更容易把“有依赖”误当成“应复用 session”
- 它更容易低估旧 session 的污染
- 它更容易在角色切换或 replan 之后继续沿用不该续用的上下文

所以默认建议改成：

- 规则内 `resume` 判断交给程序 gate
- 语义性或冲突性 `resume` 判断交给 `gpt-5.4`
- 执行交给 `gpt-5.3-codex`

进一步约束：

- 这不是 worker 运行中的附带动作
- 这是 pre-worker gate
- 先跑程序 gate
- 只有 gate 判定 `needsOrchestrator=true` 时，才调用 `gpt-5.4`

## `fresh` vs `resume` 判定

优先 `resume` 的条件：

- 同一 `parent` / 同一冲突域
- 同一角色
- `ownedPaths` 高度重合
- 当前任务是上一个任务的直接后续
- 没有经历 `replan / rollback / superseded`

优先 `fresh` 的条件：

- 跨 parent / 跨冲突域
- 虽有依赖，但任务面已经明显变化
- 角色切换明显
- 上一个 session 历史已经很长、污染较重
- 同一 session 可能被多个 worker 并行争用

## 最重要的并发规则

不要让多个 worker 同时 `resume` 同一个 session。

推荐约束：

- 一个 session 同一时刻只绑定一个 active task
- sibling 并行任务默认各自 `fresh`
- 真要复用，也只允许“直接后继 task”线性接续
- 如果一个 task 有多个上游 session 线索，先列入 `candidateResumeSessionIds`，再由 `gpt-5.4` 选出唯一 `preferredResumeSessionId`

## session 会不会持续追加内容

会。

`codex exec resume <SESSION_ID>` 会在原 session 上继续追加消息和状态，不会生成天然隔离的新分支。

这意味着：

- 旧 session 会越来越长
- 后续 resume 会继承新增内容
- 如果不做记录和限流，很容易把不该共享的历史带给新任务

## 所以推荐 session tree，但它是逻辑结构，不是 CLI 原生 fork

当前这套 CLI 里，自动化上稳定可用的是：

- `codex exec`
- `codex exec resume`

不要假设有稳定的非交互 `fork`。

因此推荐做“逻辑 session tree”：

- `orchestrationSessionId`
- `rootSessionId`
- `parentSessionId`
- `sessionId`
- `sessionFamilyId`
- `sourceTaskId`
- `resumeStrategy`
- `model`
- `status`
- `routingReason`

这个 tree 用来：

- 判断某个 task 是否适合接续旧 session
- 防止多个 worker 抢同一 session
- 在 replan 后把旧 session 标记为降级或 superseded

## 推荐文件

建议显式记录到：

- `.harness/session-registry.json`

至少应包含：

- `orchestrationSessionId`
- `orchestrationSessions`
- `sessions`
- `families`
- `activeBindings`
- `lastCompletedByTask`
- `routingDecisions`
- `recoverableBindings`

## cache hit 的正确思路

不要把 cache hit 简化成“尽量 resume”。

更稳的做法是：

1. 先跑程序 gate，筛掉依赖未满足、路径冲突、活跃 session 冲突
2. 先读取 `.harness/state/feedback-summary.json` 中当前 task 最近失败窗口
3. 只有 gate 判定 `needsOrchestrator=true` 时，才 `resume orchestrationSessionId` 让 `gpt-5.4` 进入完整编排上下文
4. 由程序 gate 或 `gpt-5.4` 产出 `fresh / resume`
5. 保持稳定 prefix
6. 把高频变化信息留在 task suffix
7. 只在线性后续 task 上续 session
8. 并行 sibling 默认新开 session

## 建议输出 machine-readable routing decision

`gpt-5.4` 做 routing 时，不要只给口头判断。

推荐至少产出：

```json
{
  "orchestrationSessionId": "019d0b19-aaaa-bbbb-cccc-111111111111",
  "taskId": "T-002",
  "resumeStrategy": "resume",
  "preferredResumeSessionId": "019d0b19-e0c1-7613-a18e-33380b167c90",
  "candidateResumeSessionIds": [
    "019d0b19-e0c1-7613-a18e-33380b167c90",
    "019d0b19-ffff-eeee-dddd-222222222222"
  ],
  "sessionFamilyId": "SF-F003-WI001",
  "cacheAffinityKey": "feature:F-003|parent:WI-001|role:worker",
  "routingReason": "同 parent、同角色、直接后续且 ownedPaths 高重合，适合线性续用旧 session。"
}
```

这样做的目的：

- 让 orchestrator 写回 task 字段时更稳定
- 让 daemon / bash 工具可以直接读取
- 让后续 audit 能知道当时为什么选择 `fresh` 或 `resume`
- 让失败或报错后还能回到最近一次实际绑定的 session

## 给弱模型写 prompt 的规则

如果下游模型较弱，例如 `gpt-5.3-codex`，prompt 不要写得像说明文。

优先使用这些写法：

- 先写模型身份
- 再写“你不负责什么”
- 再写固定顺序步骤
- 再写失败时该做什么
- 再写必须回写哪些字段

推荐：

- 一句只表达一件事
- 优先写字段名，不要只写概念
- 优先写“如果 X，则做 Y”，不要写长段分析
- 优先写禁止项，避免模型自行发挥
- 能列编号就列编号

不推荐：

- “请综合判断”
- “必要时灵活处理”
- “根据上下文自行决定”
- “尽量”
- “通常”

对于 `gpt-5.3-codex`，更好的方式是：

1. 先给固定输入文件清单
2. 再给固定执行顺序
3. 再给固定回写字段
4. 再给固定停止条件
5. 最近失败窗口默认只给当前 task 最近 3 条高严重度反馈

如果 task 很重，或者下游模型阅读能力偏弱，推荐再做一层“渐进式 prompt 暴露”：

1. 第一层只给：
   - 身份
   - 禁止项
   - 当前 task 的 `title` / `summary`
   - 最小执行顺序
2. 第二层再给：
   - `ownedPaths`
   - `worktreePath`
   - `diffBase`
   - `verificationRuleIds`
3. 第三层再给：
   - 失败动作
   - request 写法
   - 必须回写字段
4. `description` / `handoff` / 长解释默认不要第一屏就全部暴露

也就是：

- 先让弱模型知道“你是谁、不要做什么、先做哪三步”
- 再让它知道“具体边界在哪里”
- 最后才给“出错以后怎么收尾”

这样比“讲清理念”更有效。

一句话：

> cache hit 靠“稳定前缀 + 受控 resume”，不是靠“所有有关联的任务都续同一个 session”。
