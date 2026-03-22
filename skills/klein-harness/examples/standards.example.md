---
generator: klein-harness
generatedAt: "2026-03-19T14:30:00+08:00"
project: openclaw-brain-plugin
---

# Engineering Standards

## STD-001: Unit Test Coverage

所有导出的公共模块必须有对应的单元测试文件。

**理由**: 确保核心逻辑在重构和迭代中保持正确性。

**验证方式**: 检查每个 `src/` 下的 `.ts` 模块在 `tests/unit/` 下有对应 `.test.mjs` 文件。

<!-- @harness-lint: kind=standard id=STD-001 status=active reviewCycle=30d lastReview=2026-03-19 nextReview=2026-04-18 -->

---

## STD-002: Error Recovery

涉及外部 I/O（网络、文件系统、数据库）的操作必须有明确的错误恢复路径，不允许静默吞错。

**理由**: 长时 agent 任务中，静默错误会导致不可追踪的漂移。

**验证方式**: 静态检查 — 所有 `await` 调用外部服务的代码路径必须有 `try/catch` 或 `.catch()` 且包含日志或重抛。

<!-- @harness-lint: kind=standard id=STD-002 status=active reviewCycle=30d lastReview=2026-03-19 nextReview=2026-04-18 -->

---

## STD-003: Incremental Processing

批量数据处理操作必须支持增量模式，避免全量重跑。

**理由**: 全量重跑在大数据集上不可接受，且浪费 token/算力。

**验证方式**: 对应测试用例验证：给定已处理过的数据集，再次运行只处理新增/变更项。

<!-- @harness-lint: kind=standard id=STD-003 status=active reviewCycle=60d lastReview=2026-03-19 nextReview=2026-05-18 -->

---

## STD-004: Graceful Degradation

当可选依赖（QMD、pgvector、OpenViking）不可用时，系统必须降级到备选方案而非崩溃。

**理由**: 不同部署环境的依赖可用性不同，插件必须在最小配置下可用。

**验证方式**: 集成测试 — 在不配置可选依赖的情况下启动插件，验证核心功能正常。

<!-- @harness-lint: kind=standard id=STD-004 status=active reviewCycle=60d lastReview=2026-03-19 nextReview=2026-05-18 -->

---

## STD-005: TypeScript Strict Mode

所有源码必须在 `strict: true` 下编译通过，不允许 `@ts-ignore` 或 `any` 类型逃逸。

**理由**: 类型安全是长期可维护性的基础。

**验证方式**: `tsc --noEmit` 零错误。

<!-- @harness-lint: kind=standard id=STD-005 status=active reviewCycle=120d lastReview=2026-03-19 nextReview=2026-07-17 -->
