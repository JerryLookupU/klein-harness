---
name: blueprint-architect
description: "为项目做蓝图构建与设计收敛。适用于功能设计、架构扩展、改造规划、冲突分析、草稿蓝图、定稿蓝图。重点是先拆解问题、查资料、定位扩展点与约束，再形成可执行蓝图，而不是直接跳到写代码。"
allowed-tools: ["Bash", "Read", "Write", "Edit", "Glob", "Grep", "WebSearch"]
---

# This Skill Is For

这个 skill 负责把“我要做什么”收敛成“接下来该怎么做”的蓝图面。

它适合这些场景：

- 新能力设计
- 现有系统扩展
- 大改造前的设计收敛
- 需求模糊、边界不清、需要先定位约束
- 多方案对比和冲突分析
- 需要先出 draft blueprint，再 review，再定稿
- 需要先决定是否要做 targeted / deep research，再进入 blueprint

一句话：

> 先把蓝图做对，再让实现层接手。

它不是纯文档美化 skill。
它要产出的是可执行、可检查、可交接的 blueprint。

# 这个 Skill 负责什么

- 拆解目标、边界、成功标准
- 在仓库内定位相关代码、接口、配置、测试、扩展点
- 需要时查外部资料、官方文档、上游实现
- 提炼约束、风险、依赖、兼容性要求
- 在需要时先形成 research memo，再把 memo 消化进 blueprint
- 生成草稿蓝图
- 做蓝图检查
- 做蓝图冲突分析
- 收敛为定稿蓝图

# 这个 Skill 不负责什么

- 直接跳过设计进入大规模实现
- 用一句“以后再说”掩盖关键冲突
- 只写口号式架构说明，不给接口、边界、迁移和验证面
- 把 blueprint 和 task plan 混成一份模糊清单

# 先决定当前模式

只选一个主模式，不要混着做。

## `greenfield`

适合：

- 新功能、新系统、新子模块
- 仓库里还没有明确实现

重点：

- 先定义边界、接口、数据流、状态流、验证面

## `extension`

适合：

- 现有系统上继续加能力
- 需要明确挂载点、兼容性、迁移成本

重点：

- 先定位扩展点、冲突点、复用面、不可破坏面

## `refactor`

适合：

- 想重构、重分层、换抽象
- 需要避免“看起来更整洁但打坏现有行为”

重点：

- 先列行为约束、迁移顺序、验证门槛、回退路径

## `audit`

适合：

- 已有蓝图，但怀疑它和代码/需求/测试漂移

重点：

- 先找缺口、冲突、假设失效点，再决定是修订还是重写

# Progressive Disclosure

不要一次性把所有仓库文件和外部资料都塞进上下文。

## 默认先读

- 用户目标
- 项目根 `README.md`
- 相关 package / build / test 配置
- 与目标最相关的源码目录
- 与目标最相关的测试目录

## 需要蓝图结构时再读

看 [`references/blueprint-schema.md`](references/blueprint-schema.md)。

这里只放：

- blueprint 应该最少包含哪些段
- draft 和 final 的差别
- blueprint 与 task plan 的边界

## 需要做冲突分析时再读

看 [`references/conflict-checklist.md`](references/conflict-checklist.md)。

这里只放：

- 常见冲突维度
- 如何判断是假冲突还是硬冲突
- 冲突写法模板

## 需要示例时再读

- 草稿蓝图示例：[`examples/blueprint-draft.example.md`](examples/blueprint-draft.example.md)
- 定稿蓝图示例：[`examples/blueprint-final.example.md`](examples/blueprint-final.example.md)

# 工作顺序

按这个顺序做。不要上来就写“最终方案”。

## 1. 先把目标改写成设计问题

至少明确：

- 要解决什么问题
- 哪些不是这次要解决的问题
- 成功标准是什么
- 哪些约束不能碰

如果用户目标太大，先拆成 1 个主设计问题和 2 到 5 个子问题。

## 2. 做 repo-local 定位

先在仓库里找：

- 入口文件
- 关键模块
- 配置面
- 数据结构
- 测试覆盖
- 历史约束

优先用 `rg`、`rg --files`、`sed`。

不要只根据 README 猜结构。

## 3. 先决定 `researchMode`

只选一个：

- `none`
- `targeted`
- `deep`

默认不要直接进 `deep`。

判定规则：

- `none`
  - repo-local scan 已足够回答设计问题
  - 外部框架行为不是关键变量
- `targeted`
  - 需要确认少量官方行为、上游接口、迁移约束
  - 需要比较 1 到 2 个明确选项
- `deep`
  - 上游 / 协议 / framework 行为是主约束
  - repository context 明显不足
  - 多个架构方向都可行，但迁移/回滚/兼容风险很高

推荐触发器：

- 外部 framework 或 protocol 行为会影响设计
- 上游 / 官方行为可能已变化
- 当前仓库上下文不足
- 多个架构选项需要对比
- migration / rollout 风险较大

## 4. 需要时做外部资料查询

当 `researchMode != none` 且以下任一成立时，查外部资料：

- 用户明确要求参考上游项目
- 目标依赖外部框架/协议/官方行为
- 当前仓库缺少足够上下文
- 相关事实可能已经变化

查资料时优先：

- 官方文档
- 上游仓库
- 主实现或 primary source

不要把“社区二手总结”当成主依据。

## 5. 写 research memo

如果 `researchMode != none`，先把外部资料消化成 repo-local memo，再进入 blueprint。

推荐产物：

- `.harness/research/<slug>.md`
- `.harness/state/research-index.json`

research memo 推荐至少包含：

- front matter:
  - `schemaVersion`
  - `generator`
  - `generatedAt`
  - `slug`
  - `researchMode`
  - `question`
  - `sources`
- body:
  - `## Summary`
  - `## Findings`
  - `## Options Compared`
  - `## Risks`
  - `## Recommendation`

规则：

- blueprint 读取 memo，而不是直接把外部网页长文本塞进主蓝图
- memo 只保留设计决策需要的事实和比较
- 外部原始页面仍然是证据，不是共享热面

## 6. 定位扩展点和不可破坏面

至少写清楚：

- 可以在哪扩展
- 哪些地方必须复用而不是重写
- 哪些行为必须保持兼容
- 哪些测试和接口是硬边界

如果是扩展/重构模式，这一步不能跳。

## 7. 生成草稿蓝图

草稿蓝图输入面优先级：

- repo-local scan
- research memo
- conflict analysis

不要默认直接引用外部页面原文。

草稿蓝图至少要有：

- 背景
- 目标
- 非目标
- 当前现状
- 约束
- 方案草图
- 风险
- 待确认问题

草稿可以保留多个备选方案，但不要只写成散文。

## 8. 做蓝图检查

检查这些问题：

- 有没有把目标和实现细节混掉
- 有没有缺接口/状态/数据流说明
- 有没有缺迁移策略
- 有没有缺验证策略
- 有没有关键假设没落地
- 有没有把“以后再处理”留在主路径上

## 9. 做蓝图冲突分析

冲突至少按这些维度扫一遍：

- 与现有架构冲突
- 与接口契约冲突
- 与状态模型冲突
- 与并发/会话模型冲突
- 与验证/测试冲突
- 与迁移顺序冲突
- 与运维/发布/回滚冲突

冲突要分三类：

- `hard conflict`
- `soft conflict`
- `false conflict`

不要只写“存在风险”。
要写清楚冲突发生在哪里，为什么发生，怎么解。

## 10. 定稿蓝图

定稿蓝图必须能回答：

- 为什么选这个方案
- 它改哪些面
- 它不改哪些面
- 先做什么，后做什么
- 如何验证
- 如果失败怎么回退

如果仓库已经有 `.harness/`，还要补一句：

- 这个 blueprint 应该如何映射到 `features / work-items / task-pool`

# 输出要求

默认输出两层：

## 机器可读摘要

优先给一个简短结构化摘要，至少包括：

- mode
- goal
- constraints
- touched areas
- conflicts
- chosen direction
- verification

## 人类可读蓝图

正文优先按这个顺序：

1. 背景
2. 目标 / 非目标
3. 现状
4. 约束
5. 设计方案
6. 冲突分析
7. 迁移 / rollout
8. 验证
9. open questions

# 和 Klein-Harness 的关系

如果仓库已经有 `.harness/`：

- blueprint 不直接替代 `.harness/spec.json`
- 先产出设计蓝图，再决定是否把它映射进 `features.json / work-items.json / task-pool.json`
- 需要闭环时，可把蓝图定稿作为下一次 `harness-submit` 或 `replan` 的输入

如果仓库还没有 `.harness/`：

- 这个 skill 也可以独立工作
- 蓝图默认落在仓库文档或用户指定路径

# 质量门槛

以下任一存在时，不要宣称“蓝图完成”：

- 关键接口没定义
- 迁移顺序没定义
- 验证策略没定义
- 关键冲突没归类
- 方案选择理由不足
- blueprint 和 task list 混在一起无法执行

# 最后提醒

好的 blueprint 应该降低后续实现的不确定性。

如果看完蓝图，下一位 agent 仍然不知道：

- 该先改哪里
- 不能改哪里
- 怎么验证
- 失败怎么退

那它还不是合格蓝图。
