package orchestration

import (
	"testing"

	"klein-harness/internal/adapter"
)

func TestCompileDevelopmentTaskBuildsPhaseSkeletonAndSemanticSlices(t *testing.T) {
	flow := CompileDevelopmentTask(adapter.Task{
		TaskID:      "T-201",
		Kind:        "feature",
		TaskFamily:  string(TaskFamilyFeatureModule),
		SOPID:       SOPDevelopmentTaskV1,
		Title:       "扩展 dashboard 任务视图",
		Summary:     "实现 dashboard 扩展",
		Description: "要求：\n- 展示 planner/judge 结果\n- 展示 tasklist/checklist\n- 展示 tmux worker 链路",
		OwnedPaths:  []string{"internal/dashboard/**", "internal/query/**"},
	})
	if flow.Family != TaskFamilyFeatureModule || flow.SOPID != SOPDevelopmentTaskV1 {
		t.Fatalf("expected development flow to preserve feature family and SOP, got %+v", flow)
	}
	if flow.RequirementSpec == nil || flow.ArchitectureContract == nil || flow.InterfaceContract == nil {
		t.Fatalf("expected development contracts to be compiled, got %+v", flow)
	}
	if flow.SharedFlowContext.DirectPass {
		t.Fatalf("expected semantic development task graph, got %+v", flow.SharedFlowContext)
	}
	if len(flow.SharedFlowContext.CompiledPhases) != 7 {
		t.Fatalf("expected full development phases, got %+v", flow.SharedFlowContext.CompiledPhases)
	}
	if !contains(flow.SharedFlowContext.BoundarySummary, "runtime 保留原始 task family=feature_module") {
		t.Fatalf("expected boundary summary to preserve concrete family, got %+v", flow.SharedFlowContext.BoundarySummary)
	}
	if len(flow.ExecutionTasks) < 4 {
		t.Fatalf("expected anchor + semantic slices, got %+v", flow.ExecutionTasks)
	}
	if flow.ExecutionTasks[0].BatchLabel != "anchor" {
		t.Fatalf("expected first slice to anchor the task graph, got %+v", flow.ExecutionTasks[0])
	}
	if flow.ExecutionTasks[1].BatchLabel != "semantic" {
		t.Fatalf("expected semantic slice after anchor, got %+v", flow.ExecutionTasks[1])
	}
}

func TestCompileDevelopmentTaskBugfixUsesDirectPass(t *testing.T) {
	flow := CompileDevelopmentTask(adapter.Task{
		TaskID:      "T-202",
		Kind:        "bugfix",
		TaskFamily:  string(TaskFamilyBugfixSmall),
		SOPID:       SOPDevelopmentTaskV1,
		Title:       "修复 verify contract 漏字段",
		Summary:     "修复 verify contract 漏字段",
		Description: "仅修改 internal/worker/manifest.go",
		OwnedPaths:  []string{"internal/worker/manifest.go"},
	})
	if flow.Family != TaskFamilyBugfixSmall || !flow.SharedFlowContext.DirectPass {
		t.Fatalf("expected bugfix flow to use direct pass, got %+v", flow)
	}
	if len(flow.ExecutionTasks) != 1 || flow.ExecutionTasks[0].BatchLabel != "direct-pass" {
		t.Fatalf("expected one direct-pass execution slice, got %+v", flow.ExecutionTasks)
	}
	if !contains(flow.SharedFlowContext.BoundarySummary, "当前任务启用 single slice direct pass") {
		t.Fatalf("expected direct pass boundary note, got %+v", flow.SharedFlowContext.BoundarySummary)
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
