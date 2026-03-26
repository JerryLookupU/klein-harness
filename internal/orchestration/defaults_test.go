package orchestration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultPacketSynthesisLoop(t *testing.T) {
	root := "/repo"
	loop := DefaultPacketSynthesisLoop(root)
	if loop.PlannerCount != 3 || len(loop.Planners) != 3 {
		t.Fatalf("unexpected planner count: %+v", loop)
	}
	if loop.Judge.ID != "packet-judge" {
		t.Fatalf("unexpected judge: %+v", loop.Judge)
	}
	if got := loop.Planners[0].PromptRef; got != filepath.Join(root, "prompts", "spec", "planner-architecture.md") {
		t.Fatalf("unexpected planner prompt ref: %s", got)
	}
}

func TestDefaultConstraintSystem(t *testing.T) {
	root := "/repo"
	system := DefaultConstraintSystem(root, []string{"policy_bug_rca_first", "policy_resume_state_first"})
	if system.Mode != "two-level layered constraints" {
		t.Fatalf("unexpected constraint mode: %+v", system)
	}
	if len(system.Rules) < 10 {
		t.Fatalf("expected layered rules, got %+v", system)
	}
	if !strings.Contains(system.Rules[0].Source, filepath.Join(root, "prompts", "spec")) {
		t.Fatalf("expected prompt-backed source, got %+v", system.Rules[0])
	}
	var hasSoft, hasHard, hasVerificationMode bool
	for _, rule := range system.Rules {
		if rule.Enforcement == "soft" {
			hasSoft = true
		}
		if rule.Enforcement == "hard" {
			hasHard = true
		}
		if rule.VerificationMode != "" {
			hasVerificationMode = true
		}
	}
	if !hasSoft || !hasHard {
		t.Fatalf("expected both soft and hard constraints, got %+v", system.Rules)
	}
	if !hasVerificationMode {
		t.Fatalf("expected at least one hard verification mode, got %+v", system.Rules)
	}
}

func TestDefaultTopLevelPromptLoadsPromptDirectory(t *testing.T) {
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
		"prompts/spec/packet.md",
		"prompts/spec/tasks.md",
		"prompts/spec/worker-spec.md",
		"prompts/spec/judge.md",
		"executionTasks",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q: %s", want, prompt)
		}
	}
}
