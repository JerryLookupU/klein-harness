# Progressive Prompt Exposure

在以下情况读取本文件：

- 你要给弱模型下发任务
- 你发现 prompt 已经太长
- 你要减少一次性注入的阅读量

不要在普通强模型执行时默认加载全文。

## 核心原则

不要把所有信息一次性塞给弱模型。

先给它：

1. 我是谁
2. 我不能干什么
3. 我现在先干哪几步

再按需展开。

一句话：

> 弱模型先看骨架，再看边界，最后看异常处理。

## 推荐分层

### Layer 1: Start

只放：

- 身份
- 禁止项
- `title`
- `summary`
- 最小执行顺序

目标：

- 让模型先进入正确角色
- 不要一上来就被长 `description` 淹没

### Layer 2: Execute

只放：

- `ownedPaths`
- `worktreePath`
- `diffBase`
- `verificationRuleIds`
- `resumeStrategy`
- `preferredResumeSessionId`
- 当前 task 最近 3 条高严重度失败反馈

目标：

- 让模型知道改哪里
- 让模型知道在哪个工作区执行
- 让模型知道验证和 session 约束

### Layer 3: Recover

只放：

- 停止条件
- request 写法
- 回写字段
- `lastKnownSessionId`

目标：

- 避免模型一出错就乱补
- 让失败路径也有硬约束

### Layer 4: Extra

只在确实需要时放：

- `description`
- `handoff`
- 长说明
- 背景分析

目标：

- 保留补充信息
- 但不让它污染第一屏

## 谁负责决定展开

默认由 `gpt-5.4` 决定是否展开下一层。

推荐规则：

- 普通小 task：`Layer 1 + Layer 2`
- 高风险 task：`Layer 1 + Layer 2 + Layer 3`
- audit task：`Layer 1 + Layer 2`，必要时再加 `Layer 3`
- 长背景信息：默认留在 `Layer 4`

## 推荐做法

不要只维护一份“大 prompt”。

更好的方式：

- 保留完整 prompt 模板
- 再做一个 `render-prompt` 脚本
- 按 `role` / `workerMode` / `stage` 动态输出

例如：

```bash
./bin/harness-render-prompt T-002 . worker start
./bin/harness-render-prompt T-002 . worker execute
./bin/harness-render-prompt T-004 . worker audit
```

## 最佳策略

最稳的方式不是“让 prompt 更会写”，而是：

- 让 `gpt-5.4` 先做 prompt 裁剪
- 让 `gpt-5.3-codex` 只看到当前层需要的内容

这样做的收益：

- 更少阅读负担
- 更少误读长说明
- 更稳定的 cache 前缀
- 更低的胡乱发挥概率
