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
	if len(flow.ExecutionTasks) != 4 {
		t.Fatalf("expected three entity slices plus closeout, got %+v", flow.ExecutionTasks)
	}
	if got := flow.ExecutionTasks[0].OutputTargets[0]; got != filepath.Join("rundata/programmers", "01-艾伦图灵.md") {
		t.Fatalf("unexpected first output target: %s", got)
	}
}
