package orchestration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultSpecLoop(t *testing.T) {
	root := "/repo"
	loop := DefaultSpecLoop(root)
	if loop.PlannerCount != 3 || len(loop.Planners) != 3 {
		t.Fatalf("unexpected planner count: %+v", loop)
	}
	if loop.Judge.ID != "spec-judge" {
		t.Fatalf("unexpected judge: %+v", loop.Judge)
	}
	if got := loop.Planners[0].PromptRef; got != filepath.Join(root, "prompts", "spec", "planner-architecture.md") {
		t.Fatalf("unexpected planner prompt ref: %s", got)
	}
}

func TestDefaultTopLevelPromptLoadsSpecPromptDirectory(t *testing.T) {
	root := t.TempDir()
	specDir := filepath.Join(root, "prompts", "spec")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatalf("mkdir spec dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "orchestrator.md"), []byte("Spec orchestrator base prompt."), 0o644); err != nil {
		t.Fatalf("write orchestrator prompt: %v", err)
	}
	prompt := DefaultTopLevelPrompt(root, "Implement a bounded orchestrator.")
	for _, want := range []string{
		"Spec orchestrator base prompt.",
		"prompts/spec/proposal.md",
		"prompts/spec/specs.md",
		"prompts/spec/judge.md",
		"execution_tasks",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q: %s", want, prompt)
		}
	}
}
