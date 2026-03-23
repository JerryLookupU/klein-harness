# Blueprint Schema

在这些场景读取本文件：

- 你要输出蓝图草稿
- 你要把草稿收敛成定稿
- 你需要判断 blueprint 和 plan / spec / task list 的边界

## 最小蓝图结构

如果 `researchMode != none`，在进入 blueprint 之前，先产出一份 repo-local research memo。

推荐顺序：

1. `Design Question`
2. `Repo-local Scan`
3. `Research Gate`
4. `Research Memo`
5. `Draft Blueprint`
6. `Conflict Review`
7. `Final Blueprint`

一份合格 blueprint 至少包含：

1. `Background`
2. `Goal`
3. `Non-Goals`
4. `Current State`
5. `Constraints`
6. `Design`
7. `Conflict Analysis`
8. `Verification`
9. `Rollout / Migration`
10. `Open Questions`

## Draft vs Final

## Research Gate

推荐固定字段：

- `researchMode: none | targeted | deep`
- `researchQuestion`
- `researchMemoPath`

规则：

- `none`：直接从 repo-local scan 进入 draft blueprint
- `targeted`：允许少量外部确认，先写 memo 再出 draft
- `deep`：外部行为/选型是主约束，必须先把 findings 收敛进 memo

### Draft blueprint

允许：

- 保留 2 到 3 个候选方向
- 有待确认问题
- 有未完全定案的迁移顺序

但不能缺：

- 问题定义
- 约束
- 初步冲突分析
- 初步验证面

### Final blueprint

必须：

- 只保留主方案
- 说明为什么选它
- 把主要冲突明确归类并给出处理方式
- 给出验证和迁移顺序

## Blueprint 和 Plan 的边界

Blueprint 负责：

- 设计方向
- 边界
- 结构
- 契约
- 风险
- 验证策略

Plan / task list 负责：

- 谁先做什么
- 哪个任务依赖哪个任务
- ownedPaths
- session / worktree / verification assignment

不要把 implementation task list 伪装成 blueprint。

## 推荐的机器可读摘要

如果需要机器可读摘要，建议至少包含：

- `mode`
- `goal`
- `nonGoals`
- `constraints`
- `touchedAreas`
- `candidateDirections`
- `chosenDirection`
- `hardConflicts`
- `softConflicts`
- `verificationPlan`

## 当仓库已有 `.harness/`

推荐额外补一段：

- `Blueprint -> Harness Mapping`

至少回答：

- 哪些点会进入 `features.json`
- 哪些点只是 `work-items`
- 哪些点暂时不进 `task-pool`
