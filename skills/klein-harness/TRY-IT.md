# Try It

这份仓库适合用一个小项目先试，不要一上来拿超大仓库做首测。

## 推荐试用范围

- 一个中小型工具项目
- 一个你熟悉的代码仓库
- 任务量在 1 到 3 个 feature
- 能接受用 `.harness/` 跑几轮编排和执行

## 建议试用步骤

1. 选一个测试项目目录
2. 安装最小工具集
3. 手工准备最小 `.harness/` 样例，或用现有模板落盘
4. 先验证 `query` / `dashboard`
5. 再接 routing、worktree、audit
6. 记录真实卡点

安装：

```bash
./examples/harness-install-tools.example.sh <PROJECT_ROOT>
```

进入目标项目后，建议先验证：

```bash
.harness/bin/harness-query overview .
.harness/bin/harness-dashboard .
python3 .harness/scripts/refresh-state.py .
```

## 重点观察什么

- 新人第一次看懂要多久
- 弱模型会不会被 prompt 淹没
- session routing 是否容易理解
- worktree/diff 流程是否顺手
- query/dashboard 是否足够回答“现在在干什么”
- replan/stop 设计是否过重

## 不建议的首测方式

- 一上来多达 6 到 8 个活跃任务
- 一上来就全量并发
- 一上来就把所有脚本都装进去
- 没有任何真实项目，只做静态阅读

建议先用默认稳态：

- `maxActiveTasks = 2`
- 同一 conflict group 只跑 1 条
- 同一 session 只绑定 1 个活跃 task
