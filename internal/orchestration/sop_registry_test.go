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
	family, sopID := ClassifyTaskFamily("", "生成 10 个世界上最伟大的程序员 markdown 文档，每个人不少于 2000 字", nil)
	if family != TaskFamilyRepeatedEntityCorpus || sopID != SOPRepeatedEntityCorpusV1 {
		t.Fatalf("expected repeated entity corpus classification, got family=%s sop=%s", family, sopID)
	}
	family, sopID = ClassifyTaskFamily("feature", "实现需求分析、接口设计、模块开发和联调测试", nil)
	if family != TaskFamilyDevelopmentTask || sopID != SOPDevelopmentTaskV1 {
		t.Fatalf("expected development task classification, got family=%s sop=%s", family, sopID)
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
