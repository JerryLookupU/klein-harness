---
name: judge-task-compiler
description: "供中枢 packet judge 使用的编排技能。适用于把 planner 或 swarm 输出整理成共享 spec、同类子任务、依赖 DAG 与 dispatch contract；当纯推理不稳定时，优先借助配套 tool contracts 做结构化抽取与校验。"
---

# This Skill Is For

这个 skill 服务于中枢编排节点，不服务于普通 worker。

它负责把上游 planner、swarm agent、用户初始输入和已有产物，收敛成一份可执行的 task graph。

默认目标：

- 先定共享 `spec`
- 再定 `task list`
- 再定 `dependency lineage`
- 最后才交给 worker 执行

# Core Capabilities

## 1. 蓝图拆解

先把需求压成共享蓝图，不要急着出 worker task：

- 明确目标、非目标、交付物
- 冻结对象集合或名单
- 冻结文件合同、字段模板、长度要求、来源策略
- 把共享信息放进 `packet.sharedContext`

## 2. Swarm 组装

识别哪些任务是“同类、同模板、不同对象”的 fanout 任务。

默认规则：

- 同类型重复任务，优先一对象一 worker
- worker 任务主体只保留当前对象的差异
- 共用背景、约束、SPEC 不要重复散落到每个 worker 自己推断

例子：

- `20 位语言学家资料` -> `20 个对象 worker + 1 个 assemble`
- `5 个角色卡片生成` -> `5 个对象 worker + 1 个 assemble`

## 3. 谱系编排

把任务整理成 DAG，而不是平铺清单。

至少回答：

- 哪些任务要先跑
- 哪些任务可以并行
- 哪些任务依赖冻结名单或共享 spec
- 哪些任务负责 assemble / verify / closeout

# Working Rules

- 先读用户初始输入与 planner outputs，再决定 fanout
- 当 roster 未冻结时，不要提前生成实名 worker slices
- 对 repeated-object 请求，先生成 `冻结名单与分片规格` 之类的 orchestration slice
- roster 一旦冻结，重新物化 `executionTasks`
- worker handoff 必须是纯文字分层，不要把整包 JSON 原样塞给 worker

# Tool-First Fallback

当 skill 判断不稳定、对象识别含糊、或需要结构化校验时，不要硬猜。

改用 [`references/tool-contracts.md`](references/tool-contracts.md) 里的 tool contracts：

- `collect_b3ehive_outputs`
- `extract_spec_constraints`
- `synthesize_task_graph`
- `validate_dispatch_contracts`

优先顺序：

1. 用 tool 收集和抽取结构化事实
2. 用 skill 做蓝图收敛、fanout 和谱系判断
3. 用 tool 校验 dispatch contract 是否足够原子、清晰、可执行

# Output Standard

这个 skill 的直接产物应该能映射到：

- `shared_spec`
- `task_list`
- `dependency_graph`
- `dispatch_contracts`

如果 roster 还没冻结，允许只输出第一阶段 orchestrator slice，并明确 `orchestration expansion pending`。
