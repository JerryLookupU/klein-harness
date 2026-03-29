# SOP Runtime Blueprint

## 目标

把当前 harness 从“planner / judge 临场自由编排”推进到“程序主导 SOP，agent 主导单 slice 执行”的运行时。目标不是削弱 `codex --yolo`，而是把它放进一个更干净、更稳定、更可恢复的执行管道里。

最终形态应支持：

- 用户持续提交需求
- runtime 自动完成 `classify -> sop select -> context compile -> slice compile -> execute -> verify -> replan -> resume -> closeout`
- flow 级上下文干净
- 多 session 可以通过文件合同接续
- 断电或中断后可以恢复

## 为什么从自由 Packet 转向 SOP

现有 packet synthesis 对复杂任务依然有价值，但对稳定重复型任务有几个问题：

- shared spec、slice、verify、closeout 仍有较大自由度
- worker 需要阅读过多 control-plane 内容
- 内容型任务和开发型任务缺少稳定 SOP
- verify / handoff 过度依赖模型自觉，容易出现空 `verify.json`

因此第一版改造方向是：

- 模型只输出关键变化量
- 程序负责稳定执行结构
- shared flow context 与 slice-local context 明确分层
- worker prompt 基于编译结果生成

## Task Family

当前第一版 family classifier 至少支持：

- `repeated_entity_corpus`
- `single_artifact_generation`
- `bugfix_small`
- `feature_module`
- `feature_system`
- `development_task`
- `integration_external`
- `review_or_audit`
- `repair_or_resume`

第一版已接入 runtime submit 分类，并把 `taskFamily` / `sopId` 写入 request record、task、request summary、intake summary、change summary。
同时 route gate 已显式读到 `taskFamily` / `sopId`：

- `repeated_entity_corpus` 会挂上 shared-spec-frozen / programmatic-verify-first 策略信号
- `development_task` 及其子 family 会挂上 compiled-contract-first 策略信号
- family 不再只是 submit metadata，而是 dispatch 前就进入 route policy
- 对历史遗留或丢失 `taskFamily` / `sopId` 的 task，`RunOnce` 会先做一次 programmatic backfill，再进入 route

## 上下文四层模型

### 1. Request Context

- 用户原始需求
- submit kind
- 附加上下文

### 2. Shared Flow Context

- 整支 flow 共用的稳定上下文
- shared spec / requirement spec / architecture contract / interface contract 引用
- 边界摘要

### 3. Slice-Local Context

- 当前 execution slice id
- 当前标题、摘要
- 允许修改路径
- 禁止修改路径
- 当前输出目标
- 当前 done criteria

### 4. Runtime Control Context

- dispatch id
- accepted packet path
- task contract path
- session registry path
- artifact dir

原则：

- shared flow context 由程序冻结
- slice-local context 由程序编译
- runtime control context 主要给程序，不要求 worker 大量读取
- 四层会同时落成 `request-context.json`、`shared-flow-context.json`、`slice-context.json`、`runtime-control-context.json`
- `runtime-control-context.json` 会显式带上 `executionCwd`、`worktreePath`、`ownedPaths`，避免续跑或 verify 时误回到 git 根目录
- runtime 另外会聚合生成 `context-layers.json`，让下一 session 按固定入口接棒
- control/query 面会显式暴露这些 compiled context refs，便于 operator 追踪当前 slice 绑定的是哪一套合同

同时 runtime 真相账本 `runtime.json` 现在会显式跟踪：

- `activeTaskId` / `activeTaskFamily` / `activeSopId`
- `currentDispatchId` / `currentExecutionSliceId`
- `currentResumeSessionId`
- `currentTakeoverPath` / `currentContextLayersPath`
- `currentTaskGraphPath` / `currentVerifySkeletonPath` / `currentCloseoutPath`
- `currentHandoffPath` / `currentArtifactDir`
- `lastVerificationStatus` / `lastFollowUp`

这样 operator 不必反向猜测“当前 runtime 正卡在哪个 slice、哪一套 takeover 合同、哪一轮 verify”。

## SOP Registry

第一版 registry 在 `internal/orchestration` 中显式落下，当前至少注册：

- `sop.repeated_entity_corpus.v1`
- `sop.development_task.v1`

## repeated_entity_corpus.v1

固定阶段：

1. `extract_shared_spec`
2. `extract_variable_inputs`
3. `compile_task_graph`
4. `compile_worker_prompt`
5. `programmatic_verify`
6. `closeout`

程序负责：

- `shared-spec.json`
- `variable-inputs.json`
- `task-graph.json`
- `shared-flow-context.json`
- `slice-context.json`
- `context-layers.json`
- `verify-skeleton.json`
- `closeout-skeleton.json`
- `handoff-contract.json`
- `takeover-context.json`

worker 负责：

- 只处理当前 entity slice
- 或只处理 closeout slice
- 当 roster 只有 1 个对象时，允许 single slice direct pass

## development_task.v1

固定阶段：

1. `requirement_spec`
2. `architecture_contract`
3. `interface_contract`
4. `task_graph_compile`
5. `worker_execute`
6. `integration_verify`
7. `closeout`

程序负责：

- `requirement-spec.json`
- `architecture-contract.json`
- `interface-contract.json`
- `task-graph.json`
- `shared-flow-context.json`
- `slice-context.json`
- `context-layers.json`
- `verify-skeleton.json`
- `closeout-skeleton.json`
- `handoff-contract.json`
- `takeover-context.json`

worker 负责：

- 只执行当前开发 slice
- 不重排整条开发 flow
- 对 `bugfix_small` / `repair_or_resume` 以及明确单切片场景，允许 single slice direct pass

## Worker Prompt Compile 原则

prompt 从 `internal/worker/prompt_compiler.go` 生成，目标是：

- 保留必要的 shared constraints
- 显式暴露 compiled context 文件
- 缩短 prompt 中控制面叙事
- 保持 verify / handoff / takeover 为第一等输入

prompt 默认引导 worker 先看：

- `context-layers.json`
- `shared-context.json`
- `shared-flow-context.json`
- `slice-context.json`
- `verify-skeleton.json`
- `handoff-contract.json`
- `task-contract.json`

而不是先扫大量 `.harness/state/*`。

## Verify / Closeout 机制

第一版增加程序化 `verify-skeleton.json`，避免 worker 输出空对象。
同时增加程序化 `closeout-skeleton.json` 与 `handoff-contract.json`，把 closeout / handoff 必填段落和 resume read order 固定下来。
`task-contract.json` 也会反向指向 `shared-flow-context.json`、`task-graph.json`、`slice-context.json`、`context-layers.json`、`verify-skeleton.json`、`closeout-skeleton.json`、`handoff-contract.json`、`takeover-context.json`，让 verify gate 能检查 continuation contract 是否完整。

runtime 侧额外增加一条硬闸门：

- `verify.json` 如果是空对象 `{}`，会被视为失败
- `verify.json` 至少要提供 status / summary / scorecard / evidenceLedger 等可用信号之一
- 这样 “写了个 JSON 文件但没有验证内容” 不会再误判为通过

对 repeated corpus，程序应优先检查：

- 文件存在
- 文件数量
- section 完整性
- 最低字数
- support files

对 development task，程序应优先检查：

- 变更范围是否落在 allowed write globs
- verify 命令是否有证据
- handoff 是否说明完成项、风险和下一步

## Multi-Session Continuation Protocol v1

目标是让多个 session 通过文件接力，而不是靠长 prompt 继承上下文。

第一版固定文件合同包括：

- `request-context.json`
- `runtime-control-context.json`
- `context-layers.json`
- `shared-flow-context.json`
- `slice-context.json`
- `task-graph.json`
- `verify-skeleton.json`
- `closeout-skeleton.json`
- `handoff-contract.json`
- `takeover-context.json`
- `handoff.md`
- `session-registry.json`

其中 `takeover-context.json` 现在不只是 path list，还会额外携带：

- `resumeSessionId`
- `taskStatus`
- `executionCwd`
- `worktreePath`
- `artifactDir`
- `ownedPaths`
- `entryChecklist`
- `controlPlaneGuards`

也就是说，下一次 session 拿到 takeover 合同后，不仅知道读哪些文件，也知道当前任务状态、恢复入口、验证应在哪个 cwd 执行，以及必须遵守的控制面边界。

下一个 session 应只需读取这些固定文件，就能知道：

- 本轮 request / runtime control 是什么
- 这一棒的 shared flow context 是什么
- 当前 slice 能改什么、不能改什么
- 编译后的 task graph 和 slice 位次是什么
- verify 需要补什么
- handoff 必须补哪些段落
- handoff 应该如何接棒

推荐 read order：

1. `context-layers.json`
2. `request-context.json`
3. `runtime-control-context.json`
4. `shared-flow-context.json`
5. `task-graph.json`
6. `slice-context.json`
7. `task-contract.json`
8. `verify-skeleton.json`
9. `closeout-skeleton.json`
10. `handoff-contract.json`
11. `session-registry.json`
12. `handoff.md`

## 渐进迁移

第一版采用兼容策略：

- 显式带 `taskFamily` / `sopId` 的新任务走新编排
- 历史未标注 family 的普通任务保留 legacy execution task 推导
- repeated corpus 因为高度模式化，即使旧任务未显式标注 family，也允许优先匹配新 SOP

这样可以先把机制落地，再逐步扩大覆盖面，而不是一次性推翻现有 runtime。
