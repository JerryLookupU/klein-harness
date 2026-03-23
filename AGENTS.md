## Working Rules

- 当前总目标分三阶段：
- 第一阶段：通过完善外部项目来暴露并修复 `harness` 自身缺口，重点是让外部项目在尽量少人工干预下跑通。
- 第二阶段：在外部项目持续跑通的同时，让 `harness` 也能驱动并改进自己的仓库，进入自进化。
- 第三阶段：在内核稳定后，把外部项目常见能力下沉成动态层和自动化策略，实现高自治迭代。
- 当前默认锁定第一阶段；除非用户明确切换阶段，否则优先级始终是“补齐 harness 本体能力，让目标项目闭环跑通”。
- 终极目标是让 `harness` 本体在面对用户需求时，尽量不依赖 operator 的人工干预，也能自己完成闭环；目标仓库主要是验证场，不是主要修改对象。
- 发现问题时，先把现象抽象为 `harness` 的能力缺口、提示词缺口、控制面缺口、状态机缺口或安装链缺口，再决定如何补齐；不要把一次性的人工补救当成最终方案。
- 像整理 worktree、收敛多余 tmux session、区分 managed dirty / unknown dirty、处理 topic drift、触发 replan / blueprint 等，都默认视为 `harness` 应内建的系统能力，而不是 operator 长期手工代劳的步骤。
- 对目标仓库的真实运行，默认是给 `harness` 做验收和采样；采到的问题需要回灌到 `harness-architect`，沉淀成可复用机制，而不是只修当前仓库这一次。
- 当目标是通过 harness 驱动业务仓库时，默认角色是 operator / harness maintainer，不直接修改业务仓库代码。
- 对 `/Users/linzhenjie/code/claw-code/openclaw-brain-plugin` 这类目标项目，优先通过 `harness-submit <root> --goal "<需求>"` 提交需求，再观察 `.harness` 执行链。
- 发现任务执行异常时，先判断是提示词未遵循规范，还是 harness 系统能力缺口；不要先入为主地改业务代码。
- 只有在确认是 harness 问题后，才允许修改 `/Users/linzhenjie/code/claw-code/harness-architect` 内的模板、脚本、安装链，然后重装到目标项目验证。
- 除非用户明确授权，否则不直接提交或推送目标业务仓库的代码；harness 自动产出的业务改动也应按 spec 走 verify / finalize / submit 责任链。

## Operator Loop

- 当前阶段约束：
- 先让 `harness` 自己具备识别、拆解、执行、恢复、清理和收敛目标仓库任务的能力，再谈更高层自进化。
- 目标仓库上的临时人工操作只允许用于取证、安装、重试、停止或验收；不允许把人工补救当成长期运行方案。
- 每次真实运行失败后，都要先回答“这暴露的是哪个 `harness` 缺口”，再决定修哪里。
- 对当前阶段的 `klein-harness` 本体，遵循“入无必要，勿增实体”：优先复用现有 command surface、账本和状态文件，不为局部问题额外发明新层。
- 什么算待做：仍在当前 thread / plan epoch 内，未 terminal、未 supersede、未被 completion gate 关闭，且没有被 spec 明确判定为 inspection-only 的 task / request。
- 什么情况下允许自动改代码：只有 harness guard 明确放行、任务已 dispatch、工作区满足 spec 要求、且当前路径属于 worker 而非 operator 时。
- 自动改完后谁来提交/推送：先由 harness worker 在受管 lane 内完成修改和验证；提交/推送责任以项目 spec 为准，默认不由 operator 直接代做远端 push。
- 出错、超时、脏工作区怎么处理：先看 spec 定义的 recoverable / degraded / blocked 分类；环境缺口不记成任务失败，unknown dirty 阻断自动化，超时与 verify fail 进入 recoverable 并保留证据。
- 怎么知道真的完成：不能只看进程退出；必须同时满足 verify、lineage / compact handoff、completion gate、以及 spec 定义的 finalize 条件。

## Escalation

- 如果提示词没有遵循上述要点，先检查 prompt 渲染、task routing、worker prompt 分层是否缺字段。
- 如果 prompt 已完整但行为仍偏离，再检查 harness runtime、guard、dispatch、finalize、worktree、日志链是否有系统性缺口。
- 如果执行过程中发现 topic drift，不要直接把新话题混入当前 task；先判断应写 `audit`、`replan`、`blueprint` 还是 `stop` 类 follow-up，再按 thread / plan epoch 收敛。
- `blueprint` / `replan` / `append_change` 只在确有范围漂移、依赖变化、或当前计划失效时触发；不要因为轻微实现细节就反复升级控制面。
- 只有在 `codex-gpt` 额度不足时才回退到 `gpt-5.3-codex`；如果两者都不可用，停止执行并等待用户下一步操作。
