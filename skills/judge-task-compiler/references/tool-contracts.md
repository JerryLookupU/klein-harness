# Judge Tool Contracts

这些 contracts 不是给 worker 的执行脚本，而是给中枢 `judge` 用来做结构化抽取、编译和校验的工具面。

## `collect_b3ehive_outputs`

用途：

- 汇总 b3ehive / swarm / planner 的干净输出
- 保留每个输出的来源与初始输入线索

最低输入：

- `initial_input`
- `planner_outputs[]`
- `artifact_refs[]`

最低输出：

- `normalized_outputs[]`

每条 normalized output 至少含：

- `sourceAgent`
- `sourceTask`
- `inputSummary`
- `resultSummary`
- `evidenceRefs[]`

## `extract_spec_constraints`

用途：

- 从 normalized outputs 中抽共享 SPEC 和约束

重点抽取：

- roster / entity selection
- file contract
- schema / field template
- source policy
- hard constraints
- acceptance markers

最低输出：

- `shared_spec`
- `constraint_bundle`

## `synthesize_task_graph`

用途：

- 把 `shared_spec + constraint_bundle` 编译成 dispatchable task graph

重点能力：

- fanout 同类任务
- 一对象一 worker
- 冻结依赖关系
- 标记 `orchestrationExpansionPending`

最低输出：

- `execution_tasks[]`
- `dependency_edges[]`
- `parallel_groups[]`
- `expansion_decision`

## `validate_dispatch_contracts`

用途：

- 检查 task graph 中每个任务是不是足够原子、清晰、可执行

每个任务至少检查：

- 输入是否明确
- 输出路径是否明确
- done criteria 是否明确
- evidence 要求是否明确
- 是否错误混入共享 JSON 大包

最低输出：

- `validated_contracts[]`
- `findings[]`

如果发现 contract 依然要求 worker 自己重新拆任务，应该判定为未通过。
