---
name: klein-harness
description: "为项目构建 .harness/ agent 协作系统。重点不是把文件铺满，而是让进入项目的 Claude/Codex agent 能持续判断角色、认领任务、回退错误、交接工作，并把代码稳定写出来。"
allowed-tools: ["Bash", "Read", "Write", "Edit", "Glob", "Grep"]
---

# This Skill Is For

这不是“生成一堆产物然后结束”的 skill。

它的目标是给项目装一份运行中的 `.harness/` 协作契约，让后续 agent 不需要重新猜：

- 现在项目在干什么
- 该先编排还是先写代码
- 哪条路线已经被别人占了
- 走错以后怎么回退
- 下一位 agent 应该怎么接力

一句话：

> Harness 是为了持续写代码而存在，不是为了证明文档齐全而存在。

再进一步：

> 好的 harness 应该像 gstack 的流程层一样，让多 agent 并行不是“更多窗口”，而是“更清楚的分工、停点和交接”。

# 边界

## 这个 skill 负责

- bootstrap `.harness/`
- refresh / replan
- audit 现有 harness 是否还能支撑多 agent 推进
- 定义 agent 入场协议、claim 规则、回退规则、交接规则
- 当项目存在 `AGENTS.md` 时，检查并规范其中的 `SOUL` / 人格设定块
- 当用户明确需要 unattended / parallel / forever 模式时，补齐最小可用的 operator UX：
  - 总览入口
  - 当前活跃任务视图
  - 机器可读指标快照
  - 可选的 tmux / cron / codex exec 派发脚本

## 这个 skill 不负责

- 充当永远在线的中心调度器
- 替 OpenClaw 持续轮询
- 绑定某一种宿主实现去保活 worker 节点

日常推进依赖的是 `.harness/` 文件本身，不是整份 `SKILL.md`。
后续 agent 平时主要读这些：

- `.harness/progress.md`
- `.harness/work-items.json`
- `.harness/task-pool.json`
- `.harness/spec.json`
- `.harness/standards.md`
- `.harness/session-registry.json`
- `.harness/session-init.sh`

如果项目已经启用热状态和 compact log layer，优先再读：

- `.harness/state/current.json`
- `.harness/state/runtime.json`
- `.harness/state/request-summary.json`
- `.harness/state/lineage-index.json`
- `.harness/state/log-index.json`
- `.harness/log-<taskId>.md`

不要默认扫全文 `.harness/state/runner-logs/*.log`。
raw logs 只用于 operator debug、RCA 证据、或定向 detail retrieval。

如果项目根存在 `AGENTS.md`，也应在初始化和刷新时检查：

- 是否存在清晰的 `SOUL` / 人格设定段
- 是否已切换到用户要求的人格模板
- 是否与当前 harness 的工作模式冲突

如果用户关心“现在谁在跑、正在干什么、整体做到哪”，优先补这些额外入口：

- `bin/*overview*.sh`
- `bin/*watch*.sh`
- `.harness/*metrics*.json`
- `README.md` 中的 operator 命令段

# 先决定当前模式

只在需要时进入对应模式。不要默认 bootstrap。

## `bootstrap`

满足任一条件时进入：

- `.harness/` 不存在
- 关键文件缺失
- 现有 harness 语义已经坏了，无法安全接手

## `refresh`

满足任一条件时进入：

- `.harness/` 存在，但 spec / task / standards 明显过时
- 需要大规模 replan
- 现有 schema 还能读，但状态需要重整

## `audit`

满足任一条件时进入：

- 你要检查这套 harness 还能不能继续支撑多 agent 干活
- 你怀疑 example、schema、角色协议、调度协议已经漂移

## `agent-entry`

满足以下条件时进入：

- `.harness/` 齐全
- 你只是接力推进现有工作
- 不需要重建底层协作制度

# Progressive Disclosure

不要把所有 example 和参考文件一次性塞进上下文。

## 默认只读

- 当前 `SKILL.md`
- 项目里的 `.harness/*`

## 需要 schema 时再读

看 [`references/schema-contracts.md`](references/schema-contracts.md)。
这里只放字段契约、ID 体系、task readiness gate、example 对照表。

## 需要 OpenClaw 调度集成时再读

看 [`references/openclaw-dispatch.md`](references/openclaw-dispatch.md)。
这里只放 `cron -> tmux -> codex exec -> .harness` 的调度映射、dispatch 契约、注入模板，以及标准写法与本机兼容写法的区别。

## 需要判定角色写权限时再读

看 [`references/role-matrix.md`](references/role-matrix.md)。
这里只放 planner / orchestrator / worker / wait 的写权限边界和最小动作集合。

## 需要设计 bash-first 管理面时再读

看 [`references/bash-python-toolkit.md`](references/bash-python-toolkit.md)。
这里只放 bash 入口、Python 核心、谱系校验、claim / replan / stop / status / metrics 这类基础组件建议。

## 需要设计模型分工和 session 复用策略时再读

看 [`references/model-routing.md`](references/model-routing.md)。
这里只放：

- 哪些步骤由 `gpt-5.4` 做判断
- 哪些步骤交给 `gpt-5.3-codex`
- resume / fresh 的判定规则
- session 记录结构

如果下游模型较弱，例如 `gpt-5.3-codex`，写 prompt 时默认遵循：

- 先写身份
- 再写禁止项
- 再写固定顺序步骤
- 再写失败时动作
- 再写必须回写的字段

不要把 prompt 写成说明文。优先短句、编号、字段名、硬约束。

## 需要给弱模型做渐进式 prompt 暴露时再读

看 [`references/progressive-prompt-exposure.md`](references/progressive-prompt-exposure.md)。
这里只放：

- prompt 分几层暴露
- 每层该放哪些字段
- 什么时候继续展开下一层
- 怎么用脚本动态渲染最小 prompt

## 需要把 git / diff / worktree 接进执行链时再读

看 [`references/git-worktree-playbook.md`](references/git-worktree-playbook.md)。
这里只放：

- 什么时候应该开独立 worktree
- branch / worktree / claim 怎么对应
- diff 用什么基线
- 谁负责 branch 创建、提交、merge
- audit / replan / merge gate 场景下怎么安全比对改动

## 生成或刷新产物前

只读对应 example，不要无差别全读：

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
- `orchestrator routing prompt (gpt-5.4)` -> `examples/orchestrator-routing-prompt.example.md`
- `worker prompt (gpt-5.3-codex)` -> `examples/worker-prompt-gpt-5.3-codex.example.md`
- `progressive prompt renderer` -> `examples/harness-render-prompt.example.sh`
- `harness-status` -> `examples/harness-status.example.sh`
- `harness-query` -> `examples/harness-query.example.sh`
- `harness-log-search` -> `examples/harness-log-search.example.sh`
- `harness-dashboard` -> `examples/harness-dashboard.example.sh`
- `harness-install-tools` -> `examples/harness-install-tools.example.sh`
- `release smoke` -> `examples/harness-release-smoke.example.sh`
- `harness-route-session` -> `examples/harness-route-session.example.sh`
- `harness-prepare-worktree` -> `examples/harness-prepare-worktree.example.sh`
- `harness-diff-summary` -> `examples/harness-diff-summary.example.sh`
- `.harness/scripts/status.py` -> `examples/status.example.py`
- `.harness/scripts/query.py` -> `examples/query-harness.example.py`
- `.harness/scripts/log-search.py` -> `examples/log-search.example.py`
- `.harness/scripts/refresh-state.py` -> `examples/refresh-state.example.py`
- `.harness/scripts/route-session.py` -> `examples/route-session.example.py`
- `.harness/scripts/render-prompt.py` -> `examples/render-prompt.example.py`
- `.harness/scripts/prepare-worktree.py` -> `examples/prepare-worktree.example.py`
- `.harness/scripts/diff-summary.py` -> `examples/diff-summary.example.py`
- `lineage.jsonl` -> `examples/lineage.example.jsonl`
- `drift-log/*.jsonl` -> `examples/drift-log.example.jsonl`
- `feedback-log.jsonl` -> `examples/feedback-log.example.jsonl`
- `tooling-manifest.json` -> `examples/tooling-manifest.example.json`
- `session-init.sh` -> `examples/session-init.example.sh`
- `audit-report.md` -> `examples/audit-report.example.md`
- `audit worker prompt (gpt-5.3-codex)` -> `examples/audit-worker-prompt-gpt-5.3-codex.example.md`
- `state/feedback-summary.json` -> `examples/feedback-summary.example.json`
- `state/log-index.json` -> `hot compact log index`
- `state/research-index.json` -> `hot research memo index`

# Agent 进场顺序

按顺序执行。不要跳步。

## 1. 先读状态

至少读：

1. `.harness/progress.md`
2. `.harness/work-items.json`
3. `.harness/task-pool.json`
4. `.harness/spec.json`
5. `.harness/standards.md`
6. `.harness/session-registry.json`
7. `.harness/session-init.sh`

如果项目根存在 `AGENTS.md`，在 bootstrap / refresh / audit 时也要读它。

如果当前任务涉及 schema 漂移、OpenClaw 调度、角色写权限，再加载对应 reference。

## 1.5 先按三层结构理解 harness

重新阅读现有 harness 时，不要把所有文件当成平铺清单。

优先按三层去理解：

- `control plane`
  - `.harness/spec.json`
  - `.harness/work-items.json`
  - `.harness/task-pool.json`
  - `.harness/features.json`
- `execution plane`
  - `claim`
  - `dispatch`
  - `handoff`
  - `lineage.jsonl`
  - `runtime/heartbeat/log`
- `operator plane`
  - `README.md` 中的命令入口
  - `bin/*status*`
  - `bin/*watch*`
  - `bin/*metrics*`
  - `bin/*forever*`

这样做的目的：

- 先判断蓝图是否稳
- 再判断执行是否健康
- 最后判断人类操作者是否看得懂、接得住

## 2. 决定自己是什么角色

不要等别人分配。自己判断。

- `planner`
  只在 `spec.json` / `context-map.json` 需要明显重写时引入；适用于蓝图缺失、路径边界不清、feature/work-item/task 映射断裂
- `orchestrator`
  默认承担可执行控制面的编排工作；适用于 claim 状态混乱、过期 lease 需要回收、task 待展开、replan、提交待串行交接或合并
- `worker`
  适用于存在可执行 `T-*`、依赖满足、`ownedPaths` 清晰、当前没有更高优先级编排任务压在前面；普通实现和 `audit` 复核都走 worker lane
- `wait`
  适用于依赖没解开、路径冲突未处理、蓝图要求先别动、当前没有安全任务可领

等待不是失败。乱写才是。

默认优先收敛成两类活跃角色：

- `orchestrator`
- `worker`

如果项目进入结果复核、交接验收、schema 漂移检查，优先补 `kind = "audit"` 的 worker task，而不是长出新角色。

只有当你真的要动 `spec.json` / `context-map.json` 的蓝图层，才额外拉出 `planner`。
如果你拿不准，而当前任务要改 `work-items.json` / `task-pool.json` / claim / dispatch，默认用 `orchestrator`，不要混成 `planner`。

## 3. claim 再改代码

任何代码变更前先 claim。没有 claim，不要动。

claim 至少写清：

- `agentId`
- `role`
- `nodeId`
- `leasedAt`
- `leaseExpiresAt`

如果发现别人已经占了这条路线：

- 不抢
- 不绕
- 不偷偷跨 `ownedPaths`
- 需要的话新增 orchestration / replan work item

## 4. 执行

- `worker` 只动当前 task / work-item 的 `ownedPaths`
- `planner` / `orchestrator` 优先修蓝图、任务池、claim、依赖、冲突
- 代码型 `worker` 默认优先在独立 git worktree 中工作；小型 `.harness` 纯控制面任务可留在主工作区
- 走错了只回退自己引入的错误
- lineage / drift 必须记录真实状态，不要假装没事

## 5. 提交或交接

完成后至少更新：

- `task-pool.json`
- `progress.md`
- `lineage.jsonl`

需要交给下一位 agent 时，明确：

- 下一步是 `worker` 继续写，还是 `planner` 先 replan
- 审计是否需要先补一个 `audit` worker task
- 下一个 work item / task 是什么
- 是否存在 blocker / conflict / merge gate

# 借鉴 gstack 的三条核心理念

把下面三条当成 harness 设计时的默认偏好。

## 1. Process Over Tooling

gstack 的核心不是“很多命令”，而是 `Think -> Plan -> Build -> Review -> Test -> Ship -> Reflect` 这条流程链。

对应到 harness：

- 不要只造任务池，要把任务之间的前后关系写清
- 不要只给 worker 一个 todo，要给它停点、交付物、下一跳
- 如果并行很多实例，优先补流程 gate，而不是补更多脚本

## 2. Persistent State Over Clever Prompting

gstack 之所以快，不是因为 prompt 更花，而是因为浏览器状态能持续复用。

对应到 harness：

- 下一位 agent 应该在 30-60 秒内恢复上下文，而不是重新读半个仓库
- `progress.md`、`work-items.json`、`task-pool.json`、`lineage.jsonl` 要优先服务“快速恢复现场”
- 如果用户明确需要 unattended 模式，应该为运行实例拆分独立日志、handoff、heartbeat，而不是把所有输出糊到一个文件

## 3. Parallelism Requires Structure

gstack 强调 10-15 parallel sprints，但前提是每个 sprint 都知道自己在哪个阶段、何时停止、成果如何交给下一位。

对应到 harness：

- 并行不是简单多开 worker
- 默认模式是“区分 orchestrator 责任和 worker 责任”，不是强制常驻一个总控进程
- 当共享账本需要串行维护时，由 orchestrator 负责合并
- worker 只写自己的 `ownedPaths` 或 shard 产物
- 如果多个实例会碰共享文件，先引入 shard / merge gate / handoff，再允许并行
- 如果多个代码型 worker 并行，优先给每个活跃代码任务分独立 branch + worktree，而不是共用一个工作区

进一步约束：

- orchestrator 是角色，不等于常驻进程
- 有些项目适合 `1 orchestrator + N workers`
- 有些项目更适合“按需激活 orchestrator + 持续 worker”
- 只要 claim、merge gate、handoff、shared-state 更新仍然清楚，就不要求 orchestrator 常驻

# 默认编排模型

如果用户没有另给一套编排理论，优先按下面这套来建 harness。

## 0. 初始化编排默认分两层

初始化编排不要一开始就把所有任务写成最终版。

默认分两步：

### Phase A: draft orchestration

目标：

- 先快速产出初始谱系草稿
- 先确定 feature / parent / leaf 的大致边界
- 先把冲突域和大块 ownedPaths 划出来

这一阶段允许信息还不够细，但至少要让系统知道：

- 哪些任务大体相关
- 哪些任务大体独立
- 哪些区域不适合并行

### Phase B: refinement orchestration

在草稿完成后，再进入细化：

- 补齐 `summary`
- 补齐 `description`
- 补齐更准确的 `ownedPaths`
- 补齐 `acceptance`
- 补齐验证方式
- 必要时查源码、文档、测试、运行路径来修正任务边界

这一阶段允许使用工具继续查信息，不要求闭门空想。

只有当 refinement 完成到足够安全时，才释放 worker。

一句话：

- 先有草图
- 再补细节
- 最后才执行

# AGENTS.md / SOUL 规则

如果项目根存在 `AGENTS.md`，不要忽略它。

默认检查以下内容：

- 是否存在专门的 `SOUL` / 人格设定块
- 是否存在与当前用户要求冲突的人格
- 是否会影响 agent 的执行风格、沟通方式和默认行为

如果用户明确要求统一人格模板，bootstrap / refresh 时默认进行规范化。

## 示例 SOUL 模板

这个 skill 不会对所有项目强行设置统一人格。

只有在用户明确指定某个人格模板时，才按要求规范化。
例如用户明确要求本轮这类模板时，可以使用：

> 16岁超级天才编程少女

这不是要求写花哨文案，而是要求 `AGENTS.md` 有一个稳定、可读、可复用的人格锚点。

## 规范化规则

如果存在 `AGENTS.md`：

- 优先保留项目里其他非人格规则
- 只替换或重写 `SOUL` / persona 段
- 如果缺少 `SOUL` / persona 段，则新增一个 `## SOUL` 段并写入模板
- 如果存在多个冲突人格，统一收敛到当前要求的人格，不保留并列人格
- 不要无端覆盖整个 `AGENTS.md`

如果不存在 `AGENTS.md`，但用户要求模板：

- 可按 example 生成基础模板
- 至少包含 `SOUL` 段和最小行为说明

## 规范化动作顺序

当 bootstrap / refresh 遇到项目根 `AGENTS.md` 时，默认按这个顺序处理：

1. 读取现有 `AGENTS.md`
2. 定位 `SOUL` / `Persona` / `人格` 段
3. 如果找到，优先只重写该段内容
4. 如果找不到，在文件前部新增 `## SOUL`
5. 将人格统一成当前要求的模板
6. 保留其他 repo 级工程规则、命令规则、边界规则

不要因为人格切换而清空原有的 repo 约束。

## 1. 谱系是树，不是平面队列

默认把执行单元组织成树状谱系：

- `feature`
- `parent task`
- `subtask`
- `leaf task`

约束：

- 真正执行代码/文档变更的，默认只能是 `leaf task`
- `feature` 用来表达能力和目标，不直接执行
- `parent task` 负责收纳一组可局部重排的子任务
- 后续重编排默认只在当前节点的 `subtree` 内发生

如果没有强理由，不要做“全量扁平任务池”。

## 2. orchestrator 也是 task

不要把 orchestrator 想成悬在系统外部的特殊存在。

更稳的建模是：

- orchestrator 是一种角色
- orchestrator work 也是一种 task
- 它的 `kind` 通常是：
  - `orchestration`
  - `replan`
  - `rollback`
  - `merge`
  - `lease-recovery`

这样守护进程、worker、operator 都可以用同一套任务协议理解它。

## 3. 默认做局部重编排，不做全局重编排

如果执行中发现前提变化、路径冲突、需要回退、或子树切分不合理：

- 先尝试 `self` 范围内调整
- 不够再升到 `subtree`
- 再不够才升到 `parent`
- 默认不要直接重排整个 feature，更不要重排整棵树

理由很简单：

- 全局 replan 最贵
- 最容易误伤正在执行的任务
- 最容易把稳定 worker 的上下文全部打散

## 4. worker 发现问题时，提 request，不直接接管编排

如果普通 worker 在执行中发现：

- 需要 rollback
- 需要 replan
- 路径冲突
- 前提失效
- 共享账本 merge gate 被击中

推荐动作不是“自己顺手改编排”，而是：

1. 写一个 orchestration request
2. 标清 anchor 节点、影响范围、建议停止的任务
3. 进入 `pause_requested` 或 `finishing_then_pause`
4. 由守护进程在空槽出现后拉起 orchestrator task

也就是说：

- `worker detects`
- `daemon schedules`
- `orchestrator mutates plan`

除非当前 task 本身就是 orchestration 类任务，否则不要让 worker 越权改全局编排。

# 局部重编排协议

如果要支持 unattended / parallel 模式，建议把 replan 做成显式协议，而不是临时备注。

## request 最少字段

一次 replan / rollback request 至少应包含：

- `requestId`
- `kind`: `replan | rollback | merge | lease-recovery`
- `anchorTaskId`
- `scope`: `self | subtree | parent`
- `reason`
- `suggestedStopTaskIds`
- `stopMode`: `graceful | immediate`
- `requestedBy`
- `requestedAt`

## stop 不是只有一种

至少区分：

- `graceful`
  - 允许 worker 完成当前原子步骤
  - 写 checkpoint
  - 再退出
- `immediate`
  - 用于明确冲突、错误扩散、危险写入
  - 守护进程可以直接停任务

不要只有“继续跑”或“直接杀”两档。

## worker 快结束时怎么办

默认不要机械地马上停。

如果同时满足：

- 当前步骤是原子的
- 没碰到即将被回退的共享路径
- 剩余时间很短
- 没有高优先 `immediate` stop

则允许 worker 进入 `finishing_then_pause`，收尾后退出。

这比一律抢停更稳。

## 如果待编排范围里有正在执行的任务

不要默认把它们都杀掉。

建议先分三类：

- `non-conflicting`: 不停，继续
- `safe-to-finish`: 允许收尾后停
- `unsafe-conflict`: 立即停

orchestrator request 里最好显式写出建议停止的任务号，而不是让守护进程自己猜。

# 守护进程和基础组件

如果项目准备接 unattended / forever / cron / tmux 执行，不要只给 prompt，优先沉成可复用基础脚本。

最有价值的不是大而全，而是这几类基础组件：

- `claim-next`
- `check-conflict`
- `request-replan`
- `request-stop`
- `checkpoint`
- `resolve-orchestrator`

它们的目标是把下面这些动作标准化：

- 认领下一个可执行 leaf task
- 判断当前任务和其他 branch 是否路径冲突
- 生成局部 replan request
- 对指定 task 发 graceful / immediate stop
- 写可恢复现场的 checkpoint
- 在需要时产生 orchestrator task，而不是假设 orchestrator 常驻

如果只做一件事，优先把“replan request + stop request”封成基础脚本。

# 隐式编排优先

如果用户明确说“不希望有显式的编排”，默认解释为：

- 用户愿意提供全局约束
- 不希望人工频繁下 orchestrator 指令
- 系统应根据约束和事件自动产生最小必要的 orchestrator work

此时优先做的是“约束驱动”，不是“命令驱动”。

至少把这些全局约束写清：

- 树结构规则
- task 粒度上限
- `ownedPaths` 冲突规则
- 允许的 replan scope：`self/subtree/parent`
- 哪些事件会自动触发 orchestrator task
- stop policy：`graceful/immediate`
- checkpoint policy：何时必须写
- merge gate：哪些文件只能 orchestrator 改

这样平时不需要显式 orchestrate，系统也能收敛到正确行为。

# Bash-First 管理优先

如果用户明确说“希望用 bash 就能管理”，默认不要把 harness 设计成必须依赖 Web UI、数据库或常驻服务才能操作。

优先给出一套 bash 可管理的最小控制面：

- `claim-next`
- `status`
- `watch`
- `metrics`
- `request-replan`
- `request-stop`
- `checkpoint`
- `resolve-orchestrator`

目标不是脚本名字统一，而是下面这些动作都能在 bash 里完成：

- 看当前进度
- 看当前正在做什么
- 认领下一个安全 task
- 发起局部 replan
- 请求停止冲突任务
- 恢复或接管 orchestrator work

如果项目要 unattended / tmux / cron / codex exec，默认优先做 bash 入口，再考虑更复杂的调度层。

# 任务可读性规则

很多 harness 只给一个 `title`，这对机器勉强够用，对人类操作者不够。

默认把任务文本拆成三层：

- `title`
  - 短标题
  - 适合列表、表格、状态面板
- `summary`
  - 一句话说明“当前到底在干什么”
  - 适合总览、watch、CLI 状态行
- `description`
  - 说明目标、边界、停点、为什么做
  - 适合交接和 replan 时读取

推荐做法：

- `title` 控制在短句
- `summary` 控制在一行，用户一眼能看懂
- `description` 才放细节

如果用户说“我要看进度时更友好”，默认优先补 `summary`，而不是把长描述直接塞进状态面板。

# 显式 Gate

以下 gate 不可跳过。

## Gate 1: 没必要时不要 bootstrap

如果 `.harness/` 已经齐全且语义可用，DO NOT 重建整套系统。
直接进入 `agent-entry` 或 `audit`。

## Gate 2: 没标准时不要派 worker

DO NOT 在缺少下列任一项时批量展开 worker 路线：

- `standards.md`
- `verification-rules/manifest.json`
- 对应 feature / work-item / task 的验收标准
- 初始化编排仍停留在 `draft`

## Gate 3: 不安全的工作不要进 `task-pool`

只有同时满足以下条件，才可以把 work item 展开成 execution task：

- 依赖清楚
- 路径边界清楚
- 验证方式清楚
- 单个 agent 能安全推进

否则留在 `work-items.json`，不要硬塞进 `task-pool.json`。
进入 `task-pool.json` 之后，仍要结合 `status`、`dependsOn` 和 claim 状态判断“当前是否可领”。

## Gate 4: 调度信息先落 `.harness/`，再派发

如果你要把任务投递给 OpenClaw / tmux worker node / `codex exec`：

DO NOT 先发命令再补状态。

必须先写：

- `claim`
- `dispatch`
- 必要的 `handoff`
- `lineage.jsonl` 中的调度事件

然后再投递执行命令。

## Gate 4.5: pre-worker session 连接先过程序 gate，再决定是否回退给 `gpt-5.4`

任何普通 worker 真正启动前，必须先经过一次 pre-worker routing。

这一步默认由程序 gate 完成，至少要决定：

- 当前 task 是否可 claim
- 当前 task 用 `fresh` 还是 `resume`
- 如果 `resume`，连接哪个 `sessionId`
- 当前 task 的 `sessionFamilyId`
- 当前 task 的 `cacheAffinityKey`
- 当前 task 的 `routingReason`

如果这些字段还没写进 `.harness/task-pool.json` 或等效状态文件：

- 不要直接启动 `gpt-5.3-codex`
- 先补一次 routing / orchestration 判断

一句话：

- pre-worker 先连 session
- session 连接先过程序 gate
- gate 判定语义不清或冲突高时再回退给 `gpt-5.4`
- 然后才允许 `gpt-5.3-codex` 开工

## Gate 5: `worker` 不得擅自改编排文件

除非当前任务本身就是 `orchestration` / `replan` 且 `ownedPaths` 明确包含相关 `.harness/` 文件，
否则 `worker` DO NOT 修改：

- `.harness/spec.json`
- `.harness/work-items.json`
- `.harness/context-map.json`
- `.harness/features.json`
- `.harness/standards.md`
- `.harness/verification-rules/manifest.json`

`kind = "audit"` 的 worker 默认也不得改上面这些编排文件。审计发现问题时，先写 `audit-report.md` 和 request，再交回 `orchestrator`。

# OpenClaw 调度映射

这个 skill 支持你当前的 `cron -> tmux -> codex exec` 流程，但不要把实现和概念混掉。

- `cron` = 调度触发层
- `tmux` session = worker node 层
- 一次 `codex exec` / `codex exec resume` = 单次执行层
- `.harness/*` = 状态层
- `git branch + git worktree` = 代码隔离层

模型约束默认写死：

- 编排模型：`gpt-5.4`
- worker 执行模型：`gpt-5.3-codex`
- 编排主线 session：`orchestrationSessionId`

默认让 `gpt-5.4` 持有一条持续的编排 session，用于：

- 初始两轮编排
- pre-worker session 连接
- replan / rollback
- worker 路由判断

普通 worker 不直接共用这条 session，只消费它写出的 routing 决策。
`kind = "audit"` 的 worker 默认先由 `gpt-5.4` 做 prompt 精化，再交给 `gpt-5.3-codex` 执行。

命令口径默认双写：

- 标准写法：`codex exec --yolo`
- 本机兼容写法：`codex --yolo exec`

如果要生成长期复用的模板、README、operator 脚本，优先写标准写法；
如果是在当前机器的本地派发脚本里，也可以备注本机兼容写法。

最重要的约束只有两条：

1. `tmux` 不是任务本身，而是可复用 worker node
2. orchestrator 必须先写 `.harness/` 状态，再把任务派进 node

补充一条：

3. 代码型 worker 默认不要直接在共享仓库根目录改代码；优先在 task 绑定的 worktree 中工作，并用 diff 证明自己的改动边界

补充：

- orchestrator 可以是一次性派发者，而不是长期驻留 supervisor
- 如果 worker 的 shard 边界、回写路径、merge gate 已经稳定，允许 orchestrator 只在 replan / merge / conflict / lease 回收时出现

需要具体 dispatch 字段、主会话派发顺序、worker/orchestrator prompt 模板时，读
[`references/openclaw-dispatch.md`](references/openclaw-dispatch.md)。
需要具体 git / diff / worktree 玩法时，读
[`references/git-worktree-playbook.md`](references/git-worktree-playbook.md)。

# Operator UX 规则

当用户在真实工作流里表现出下面任一需求时，不要只停在 `.harness/*.json`：

- “我要离开了，你继续跑”
- “我要看整体进度”
- “我要知道现在正在干什么”
- “我要开多个 Codex 实例并行”
- “我要看到命令结果 / 日志”

此时应额外产出最小 operator UX。

## 至少补齐这些能力

- 一个总览命令：显示当前模式、focus、active work/task、blocker、最近消息
- 一个 watch 命令：自动刷新总览
- 一个 metrics 命令：输出机器可读和人类可读两种进度快照
- 如果是并行模式：显示当前运行实例数、每个实例的角色、heartbeat、日志路径

并且要区分：

- orchestrator 角色当前无人占用
- orchestrator 角色按需激活
- orchestrator 进程意外退出

不要把“当前没有常驻 orchestrator”误判成系统异常。

## 命名可以因项目而异，但目标不变

例如：

- `bin/*overview*.sh`
- `bin/*watch*.sh`
- `bin/*metrics*.sh`
- `bin/*forever*.sh`

重点不是名字，而是：

- 用户不用读 JSON 才知道状态
- agent 不用猜“当前谁在跑”
- unattended 模式下有统一入口可恢复现场

# 科学进度规则

默认不要把“固定 N 条 checklist”当作完成度本身。

例如：

- `200 条 review`
- `50 个 work items`
- `12 个 tasks`

这些更像样本容量，不是质量指标。

至少要同时考虑：

- 覆盖率：是不是都分类/处理过
- 证据覆盖率：是不是都有源码/文档依据
- 闭环质量：`validated/watch/risk/gap` 不能一视同仁
- 残余风险压力：当前还有多少需要持续盯防
- 高优先问题比例：`risk + gap` 占比多少

如果用户持续维护同一份 tracker，优先生成：

- `.harness/*metrics*.json`
- 对应的 `bin/*metrics*.sh`

这样 orchestrator 和人类操作者都能用同一套口径看进度，而不是只盯条目数。

# 产物清单

一套可运行的 `.harness/` 至少包含：

- `standards.md`
- `verification-rules/manifest.json`
- `features.json`
- `work-items.json`
- `spec.json`
- `task-pool.json`
- `context-map.json`
- `progress.md`
- `lineage.jsonl`
- `drift-log/*.jsonl`
- `session-registry.json`
- `session-init.sh`

审计时额外生成：

- `audit-report.md`

如果项目要把审计接进常规执行链，推荐额外在 `task-pool.json` / `work-items.json` 中显式加入：

- `kind = "audit"`
- `roleHint = "worker"`
- `workerMode = "audit"`
- `reviewOfTaskIds`
- `auditScope`
- `auditVerdict`

字段契约、ID 约定、task readiness 规则统一见
[`references/schema-contracts.md`](references/schema-contracts.md)。

# bootstrap / refresh 顺序

只在确实需要时做整套重建或刷新。

## 1. 探边界

- 找项目根
- 找 monorepo 边界
- 找测试、lint、CI、包管理器
- 找高冲突区域

## 2. 先写标准，再谈派工

至少先生成：

- `.harness/standards.md`
- `.harness/verification-rules/manifest.json`

## 2.5 检查 AGENTS.md

如果项目根存在 `AGENTS.md`：

- 检查是否有 `SOUL` / persona 段
- 只有用户明确指定人格模板时，才检查是否与用户要求一致
- 若用户明确指定人格模板且不存在 `SOUL` / persona 段，则新增该段并写入模板
- 若用户明确指定人格模板且人格不一致，优先只更新人格段，不覆盖其他规则
- 若用户明确指定人格模板且存在多个冲突人格，统一切换成当前要求的人格模板

如果项目根不存在 `AGENTS.md`，但用户明确要求模板：

- 参考 `examples/AGENTS.example.md`
- 生成最小可用模板

## 3. 再抽能力面

生成：

- `.harness/features.json`

feature 记录的是“系统能力”。
bugfix / replan / chore 不要硬塞进 feature。

## 4. 把活列出来

生成：

- `.harness/work-items.json`
- `.harness/spec.json`

这里默认先产出 draft 版谱系，不要求一次把所有描述补满。

至少先明确：

- feature 边界
- parent / leaf 基本关系
- 冲突域
- 大致 ownedPaths

如果此时信息还不够细，先不要急着放 worker。

## 4.5 细化编排再放行

在 worker 进场前，默认再做一轮 refinement：

- 查源码 / 文档 / 测试 / 运行路径
- 补齐 `summary`
- 补齐 `description`
- 修正 `ownedPaths`
- 修正 `acceptance`
- 修正验证规则或明确编排任务的结构性验收

只有 refinement 后，task readiness gate 仍成立，才进入 worker 阶段。

## 5. 只把安全任务放进池子

生成：

- `.harness/task-pool.json`
- `.harness/context-map.json`
- `.harness/progress.md`

## 6. 给新 agent 一个固定入场脚本

生成：

- `.harness/session-init.sh`

它必须只读，唯一允许写的是 `drift-log/` 追加事件。

## 7. 最后审计

生成：

- `.harness/audit-report.md`

审计重点不是“文件在不在”，而是这套 harness 还能不能支撑下一位 agent 安全接手。
如果项目存在 `AGENTS.md`，也要确认其 `SOUL` 设定与当前要求一致。

如果要把审计做成可派发任务，而不是一次性人工检查，推荐插入这条闭环：

1. `worker` 完成实现与验证
2. `orchestrator` 视 `mergeRequired` / 高风险变更 / repeated fail / handoff 复杂度 生成 `audit` task
3. `gpt-5.4` 先为 audit task 做 routing 和 prompt 精化
4. `gpt-5.3-codex` 以 `workerMode = audit` 执行 audit task
5. audit worker 产出 `audit-report.md`，并写 `auditVerdict = pass | warn | fail`
6. `orchestrator` 根据 verdict 决定 merge、replan、stop 或重开 worker

默认触发审计的场景：

- 当前 task `mergeRequired = true`
- 当前 task 触发过 replan / rollback / stop request
- 当前 task 修改高冲突路径或共享账本
- 当前 task 的验证通过，但 handoff 风险仍高
- 当前处于 `audit` 模式，需要检查 harness 健康而非单个业务实现

# 硬规则

1. 不要为了把文件写满而造 harness
2. 没标准时不要批量派 worker
3. 没 `ownedPaths` 的任务先做编排，不做执行
4. 阻塞状态必须落在机器可读文件里
5. `session-init.sh` 只读，除了 `drift-log/` 追加
6. `drift-log/` 和 `lineage.jsonl` 只追加，不能覆盖
7. 只回退自己引入的错误
8. 忽略噪声目录：`node_modules/`, `dist/`, `build/`, `.git/`, `vendor/`, `__pycache__`
9. 并行模式下，默认不要让多个 worker 直接改同一份共享账本；先拆 shard，再由 orchestrator 合并
10. 如果用户明确需要 unattended / operator 可见性，至少要给一个人类可读入口，不要让状态只存在 JSON
11. 不要把 orchestrator 误建模成必须 24/7 常驻；只有当共享状态持续冲突、worker 数量高、或 merge gate 高频触发时，才建议常驻 orchestrator
12. 如果项目存在 `AGENTS.md`，bootstrap / refresh 时不要跳过人格检查；用户指定了 SOUL 模板时，至少要规范到指定人格

# 最后给用户的汇报

完成后只需要说清：

- 当前是 `bootstrap` / `refresh` / `audit` / `agent-entry`
- 更新了哪些 `.harness/` 文件
- 当前最高优先级 work item / task 是什么
- 现在有没有编排任务压在前面
- 有没有 blocker / drift / 路径冲突
- 当前能直接给 OpenClaw / `codex exec` 分发多少个任务
- 如果做了 unattended / parallel 集成：现在有几个实例在跑、各自角色是什么、用户该用哪条命令看总览
