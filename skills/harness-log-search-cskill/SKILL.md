---
name: harness-log-search-cskill
description: "搜索 Klein-Harness 的 compact handoff logs，并在需要时回落到 raw runner logs 的局部证据窗口。适用于跨 worker handoff、operator 调试、定向日志检索。"
allowed-tools: ["Bash", "Read", "Grep"]
---

# This Skill Is For

这个 skill 用于在不广播完整 worker transcript 的前提下，搜索 `.harness` 日志面。

默认目标：

- 先读热路径状态和 compact logs
- 只在 `--detail` 或证据不足时回落到 raw runner logs

# Retrieval Order

默认读取顺序：

1. `.harness/state/current.json`
2. `.harness/state/runtime.json`
3. `.harness/state/request-summary.json`
4. `.harness/state/lineage-index.json`
5. `.harness/state/log-index.json`
6. `.harness/log-<taskId>.md`
7. `.harness/state/runner-logs/<taskId>.log` 仅用于定向细节

不要默认把所有 raw logs 都扫一遍。

# Preferred Command Surface

优先使用：

```bash
.harness/bin/harness-log-search . --task-id T-003
.harness/bin/harness-log-search . --keyword verify --detail
.harness/bin/harness-query logs . --text
.harness/bin/harness-query log . T-003 --detail --text
```

# What To Return

优先返回：

- one-screen summary
- cross-worker relevant facts
- blockers / risks
- verification notes
- evidence refs

如果需要 raw evidence，只返回相关窗口，不要贴完整 transcript。
