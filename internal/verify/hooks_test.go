package verify

import (
	"os"
	"path/filepath"
	"testing"

	"klein-harness/internal/adapter"
	"klein-harness/internal/dispatch"
)

func TestBuildHookPlanPromotesLearnedCloseoutPattern(t *testing.T) {
	root := t.TempDir()
	if err := recordLearning(root, patternMissingCloseoutArtifacts); err != nil {
		t.Fatalf("record learning once: %v", err)
	}
	if err := recordLearning(root, patternMissingCloseoutArtifacts); err != nil {
		t.Fatalf("record learning twice: %v", err)
	}

	plan := BuildHookPlan(root, adapter.Task{
		TaskID:       "T-1",
		OwnedPaths:   []string{"test/**"},
		PromptStages: []string{"route", "dispatch", "execute", "verify"},
	}, dispatch.Ticket{
		DispatchID:  "dispatch_T-1_1_1",
		ReasonCodes: []string{"dispatch_ready"},
	}, nil)

	if len(plan.Hooks) == 0 || plan.Hooks[0].Action != "block" {
		t.Fatalf("expected promoted preflight hook to block, got %+v", plan.Hooks)
	}
	if len(plan.LearningHints) == 0 {
		t.Fatalf("expected learning hints, got %+v", plan)
	}
}

func TestEnsureCloseoutArtifactsWritesBlockingFallbacks(t *testing.T) {
	root := t.TempDir()
	paths, err := adapter.Resolve(root)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	artifactDir := filepath.Join(paths.ArtifactsDir, "T-1", "dispatch_T-1_1_1")
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		t.Fatalf("mkdir artifact dir: %v", err)
	}
	result, err := EnsureCloseoutArtifacts(root, adapter.Task{
		TaskID:    "T-1",
		ThreadKey: "R-1",
		PlanEpoch: 1,
	}, dispatch.Ticket{
		DispatchID: "dispatch_T-1_1_1",
	}, artifactDir, filepath.Join(paths.LogsDir, "tmux.log"), map[string]int{"filesChanged": 1}, "succeeded", "bounded burst completed")
	if err != nil {
		t.Fatalf("ensure closeout artifacts: %v", err)
	}
	if !result.Generated || result.Status != "blocked" {
		t.Fatalf("expected generated blocking closeout result, got %+v", result)
	}
	for _, path := range []string{
		filepath.Join(artifactDir, "verify.json"),
		filepath.Join(artifactDir, "worker-result.json"),
		filepath.Join(artifactDir, "handoff.md"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s to exist: %v", path, err)
		}
	}

	learning, err := loadLearningState(root)
	if err != nil {
		t.Fatalf("load learning state: %v", err)
	}
	if learning.Patterns[patternMissingCloseoutArtifacts].Count == 0 {
		t.Fatalf("expected closeout miss to be learned: %+v", learning)
	}
}
