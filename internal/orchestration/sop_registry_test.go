package orchestration

import (
	"testing"

	"klein-harness/internal/adapter"
)

func TestDefaultSOPRegistryLookup(t *testing.T) {
	registry := DefaultSOPRegistry()
	if _, ok := registry.Lookup(SOPRepeatedEntityCorpusV1); !ok {
		t.Fatalf("expected repeated entity corpus sop to be registered")
	}
	if _, ok := registry.Lookup(SOPDevelopmentTaskV1); !ok {
		t.Fatalf("expected development task sop to be registered")
	}
	if got := registry.LookupByFamily(TaskFamilyRepeatedEntityCorpus); len(got) == 0 {
		t.Fatalf("expected family lookup to return at least one sop")
	}
}

func TestClassifyTaskFamily(t *testing.T) {
	cases := []struct {
		name   string
		kind   string
		goal   string
		family TaskFamily
		sopID  string
	}{
		{name: "repeated entity corpus", goal: "生成 10 个世界上最伟大的程序员 markdown 文档，每个人不少于 2000 字", family: TaskFamilyRepeatedEntityCorpus, sopID: SOPRepeatedEntityCorpusV1},
		{name: "single artifact", goal: "生成单文档架构总结报告", family: TaskFamilySingleArtifact, sopID: SOPDevelopmentTaskV1},
		{name: "bugfix small", kind: "bug", goal: "修复 runtime verify 报错", family: TaskFamilyBugfixSmall, sopID: SOPDevelopmentTaskV1},
		{name: "feature module", kind: "feature", goal: "扩展 dashboard 模块页面", family: TaskFamilyFeatureModule, sopID: SOPDevelopmentTaskV1},
		{name: "feature system", kind: "feature", goal: "重构 harness runtime orchestration pipeline", family: TaskFamilyFeatureSystem, sopID: SOPDevelopmentTaskV1},
		{name: "development task", kind: "feature", goal: "实现需求分析、接口设计、模块开发和联调测试", family: TaskFamilyDevelopmentTask, sopID: SOPDevelopmentTaskV1},
		{name: "integration external", goal: "接入 OAuth 第三方登录", family: TaskFamilyIntegrationExternal, sopID: SOPDevelopmentTaskV1},
		{name: "review audit", goal: "审计当前 runtime route 与 verify 合同", family: TaskFamilyReviewOrAudit, sopID: SOPDevelopmentTaskV1},
		{name: "repair resume", goal: "恢复上次中断的 session 并继续执行", family: TaskFamilyRepairOrResume, sopID: SOPDevelopmentTaskV1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			family, sopID := ClassifyTaskFamily(tc.kind, tc.goal, nil)
			if family != tc.family || sopID != tc.sopID {
				t.Fatalf("expected family=%s sop=%s, got family=%s sop=%s", tc.family, tc.sopID, family, sopID)
			}
		})
	}
}

func TestNormalizeTaskClassificationUsesRegistryFallback(t *testing.T) {
	family, sopID := NormalizeTaskClassification(TaskFamilyRepairOrResume, "")
	if family != TaskFamilyRepairOrResume || sopID != SOPDevelopmentTaskV1 {
		t.Fatalf("expected registry fallback for repair/resume, got family=%s sop=%s", family, sopID)
	}
}

func TestCompileFlowForTaskUsesRegistryAndPreservesDevelopmentFamily(t *testing.T) {
	flow := CompileFlowForTask("/repo", adapter.Task{
		TaskID:      "T-bug",
		Kind:        "bugfix",
		TaskFamily:  string(TaskFamilyBugfixSmall),
		SOPID:       SOPDevelopmentTaskV1,
		Title:       "Fix runtime drift",
		Summary:     "Fix runtime drift",
		OwnedPaths:  []string{"internal/runtime/**"},
		Description: "Only touch runtime submit plumbing",
	})
	if flow.SOPID != SOPDevelopmentTaskV1 || flow.Family != TaskFamilyBugfixSmall {
		t.Fatalf("expected registry-driven development flow preserving family, got %+v", flow)
	}
	if !flow.SharedFlowContext.DirectPass || len(flow.ExecutionTasks) != 1 {
		t.Fatalf("expected bugfix family to use direct pass, got %+v", flow)
	}
}

func TestMaterializeTaskClassificationPreservesDevelopmentSubFamily(t *testing.T) {
	task := MaterializeTaskClassification(adapter.Task{
		TaskID:      "T-legacy",
		Kind:        "bug",
		Title:       "Fix resume verify drift",
		Summary:     "Fix resume verify drift",
		Description: "恢复上次中断的 session 并继续执行",
	})
	if task.TaskFamily != string(TaskFamilyRepairOrResume) {
		t.Fatalf("expected repair/resume family, got %+v", task)
	}
	if task.SOPID != SOPDevelopmentTaskV1 {
		t.Fatalf("expected development task sop for repair/resume family, got %+v", task)
	}
}

func TestCompileFlowForTaskMapsSingleArtifactAndReviewFamiliesToDevelopmentSOP(t *testing.T) {
	for _, tc := range []struct {
		name   string
		task   adapter.Task
		family TaskFamily
	}{
		{
			name: "single artifact",
			task: adapter.Task{
				TaskID:      "T-single",
				Kind:        "generate",
				TaskFamily:  string(TaskFamilySingleArtifact),
				Title:       "生成单文档架构总结报告",
				Summary:     "生成单文档架构总结报告",
				OwnedPaths:  []string{"docs/runtime-report.md"},
				Description: "只输出一个文件并记录验证证据",
			},
			family: TaskFamilySingleArtifact,
		},
		{
			name: "review audit",
			task: adapter.Task{
				TaskID:      "T-review",
				Kind:        "review",
				TaskFamily:  string(TaskFamilyReviewOrAudit),
				Title:       "审计 runtime route 与 verify 合同",
				Summary:     "审计 runtime route 与 verify 合同",
				OwnedPaths:  []string{"internal/route/**", "internal/verify/**"},
				Description: "输出审查结论并保留 handoff/verify 收口",
			},
			family: TaskFamilyReviewOrAudit,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			task := MaterializeTaskClassification(tc.task)
			if task.SOPID != SOPDevelopmentTaskV1 {
				t.Fatalf("expected family to materialize onto development SOP, got %+v", task)
			}
			flow := CompileFlowForTask("/repo", task)
			if flow.SOPID != SOPDevelopmentTaskV1 || flow.Family != tc.family {
				t.Fatalf("expected compiled flow to preserve family on development SOP, got %+v", flow)
			}
			if len(flow.SharedFlowContext.CompiledPhases) == 0 {
				t.Fatalf("expected compiled phases for %s, got %+v", tc.name, flow.SharedFlowContext)
			}
		})
	}
}
