package orchestration

import (
	"path/filepath"
	"testing"

	"klein-harness/internal/adapter"
)

func TestCompileRepeatedEntityCorpus(t *testing.T) {
	task := adapter.Task{
		TaskID:      "T-101",
		ThreadKey:   "thread-101",
		TaskFamily:  string(TaskFamilyRepeatedEntityCorpus),
		SOPID:       SOPRepeatedEntityCorpusV1,
		Title:       "生成世界上最伟大的程序员文档",
		Summary:     "在 rundata/programmers 中生成 10 位程序员 markdown 文档，每个人不少于 2000 字",
		Description: "名单：艾伦·图灵、约翰·冯·诺依曼、格蕾丝·霍珀",
	}
	flow := CompileRepeatedEntityCorpus("/repo", task)
	spec, ok := flow.SharedSpec.(RepeatedEntitySharedSpec)
	if !ok {
		t.Fatalf("expected shared spec type, got %#v", flow.SharedSpec)
	}
	if spec.OutputRoot != "rundata/programmers" {
		t.Fatalf("unexpected output root: %+v", spec)
	}
	if spec.MinChars != 2000 {
		t.Fatalf("expected min chars 2000, got %+v", spec)
	}
	vars, ok := flow.VariableInputs.(RepeatedEntityVariableInputs)
	if !ok || len(vars.Entities) != 3 {
		t.Fatalf("expected three roster entries, got %#v", flow.VariableInputs)
	}
	if flow.Family != TaskFamilyRepeatedEntityCorpus || flow.SOPID != SOPRepeatedEntityCorpusV1 {
		t.Fatalf("expected repeated corpus family+sop, got %+v", flow)
	}
	if flow.SharedTaskGroupContext == nil {
		t.Fatalf("expected shared task-group context to be compiled")
	}
	if flow.SharedTaskGroupContext.EntitySelection.TargetCount != 3 || len(flow.SharedTaskGroupContext.EntitySelection.Entities) != 3 {
		t.Fatalf("expected shared task-group entity selection to be frozen, got %+v", flow.SharedTaskGroupContext)
	}
	if len(flow.SharedFlowContext.CompiledPhases) != 6 {
		t.Fatalf("expected repeated corpus phases to be compiled, got %+v", flow.SharedFlowContext.CompiledPhases)
	}
	if flow.TaskGraphCompile.CompileMode != "entity_fanout_closeout" || flow.TaskGraphCompile.ResumeProtocol != "kh.multi-session-continuation.v1" || len(flow.TaskGraphCompile.ReplanTriggers) == 0 {
		t.Fatalf("expected repeated corpus task graph skeleton, got %+v", flow.TaskGraphCompile)
	}
	if flow.SharedFlowContext.DirectPass {
		t.Fatalf("expected multi-entity corpus to stay staged, got %+v", flow.SharedFlowContext)
	}
	if !containsRepeatedCorpusValue(flow.SharedFlowContext.BoundarySummary, "程序负责冻结 shared spec 和 variable inputs") {
		t.Fatalf("expected shared-flow boundary summary to preserve program-owned shared spec, got %+v", flow.SharedFlowContext.BoundarySummary)
	}
	if len(flow.ExecutionTasks) != 4 {
		t.Fatalf("expected three entity slices plus closeout, got %+v", flow.ExecutionTasks)
	}
	if got := flow.ExecutionTasks[0].OutputTargets[0]; got != filepath.Join("rundata/programmers", "01-艾伦图灵.md") {
		t.Fatalf("unexpected first output target: %s", got)
	}
}

func TestCompileRepeatedEntityCorpusSingleEntityUsesDirectPass(t *testing.T) {
	task := adapter.Task{
		TaskID:      "T-102",
		ThreadKey:   "thread-102",
		TaskFamily:  string(TaskFamilyRepeatedEntityCorpus),
		SOPID:       SOPRepeatedEntityCorpusV1,
		Title:       "生成单对象资料",
		Summary:     "在 rundata/programmers 中生成 1 位程序员 markdown 文档，每个人不少于 2000 字",
		Description: "名单：艾伦·图灵",
	}
	flow := CompileRepeatedEntityCorpus("/repo", task)
	if !flow.SharedFlowContext.DirectPass || len(flow.ExecutionTasks) != 1 {
		t.Fatalf("expected single-entity corpus to use direct pass, got %+v", flow)
	}
	if flow.TaskGraphCompile.CompileMode != "single_slice_direct_pass" || flow.TaskGraphCompile.DirectPassReason != "single_entity_roster" {
		t.Fatalf("expected direct-pass repeated corpus compile skeleton, got %+v", flow.TaskGraphCompile)
	}
	if flow.ExecutionTasks[0].BatchLabel != "direct-pass" {
		t.Fatalf("expected direct-pass batch label, got %+v", flow.ExecutionTasks[0])
	}
}

func containsRepeatedCorpusValue(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
