package tmux

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"klein-harness/internal/dispatch"
)

func TestRunBoundedBurstWritesOutcome(t *testing.T) {
	root := t.TempDir()
	checkpointPath := filepath.Join(root, "checkpoints", "task.json")
	outcomePath := filepath.Join(root, "checkpoints", "outcome.json")
	promptPath := filepath.Join(root, "runner-prompt.md")
	if err := os.WriteFile(promptPath, []byte("manifest-path\n"), 0o644); err != nil {
		t.Fatalf("write prompt: %v", err)
	}
	capturedPath := filepath.Join(root, "captured.txt")
	result, err := RunBoundedBurst(BurstRequest{
		TaskID:         "T-1",
		DispatchID:     "dispatch-1",
		WorkerID:       "worker-1",
		Cwd:            root,
		Command:        "python3 -c 'import pathlib,sys; pathlib.Path(\"captured.txt\").write_text(sys.stdin.read())'",
		PromptPath:     promptPath,
		CheckpointPath: checkpointPath,
		OutcomePath:    outcomePath,
		Artifacts:      []string{"artifact-a"},
		Budget: dispatch.Budget{
			MaxMinutes: 1,
		},
	})
	if err != nil {
		t.Fatalf("run bounded burst: %v", err)
	}
	if result.Status != "succeeded" {
		t.Fatalf("expected succeeded status, got %s", result.Status)
	}
	captured, err := os.ReadFile(capturedPath)
	if err != nil {
		t.Fatalf("read captured stdin: %v", err)
	}
	if string(captured) != "manifest-path\n" {
		t.Fatalf("unexpected captured stdin: %q", string(captured))
	}
	if !slicesContain(result.Artifacts, "artifact-a") {
		t.Fatalf("expected artifact list to include manifest artifacts: %#v", result.Artifacts)
	}
	if !slicesContain(result.Artifacts, checkpointPath) || !slicesContain(result.Artifacts, outcomePath) {
		t.Fatalf("expected checkpoint/outcome artifacts: %#v", result.Artifacts)
	}
}

func slicesContain(values []string, want string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == want {
			return true
		}
	}
	return false
}
