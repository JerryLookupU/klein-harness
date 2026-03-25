package bootstrap

import (
	"os"
	"testing"

	"klein-harness/internal/state"
)

func TestInitCreatesMinimalRuntimeState(t *testing.T) {
	root := t.TempDir()
	paths, err := Init(root)
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	for _, path := range []string{
		paths.TaskPoolPath,
		paths.QueuePath,
		paths.RuntimePath,
		paths.DispatchSummaryPath,
		paths.LeaseSummaryPath,
		paths.CheckpointSummaryPath,
		paths.VerificationSummaryPath,
		paths.TmuxSummaryPath,
		paths.SessionRegistryPath,
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s to exist: %v", path, err)
		}
	}
	var runtime struct {
		Status string `json:"status"`
	}
	if err := state.LoadJSON(paths.RuntimePath, &runtime); err != nil {
		t.Fatalf("load runtime: %v", err)
	}
	if runtime.Status != "idle" {
		t.Fatalf("expected idle runtime, got %q", runtime.Status)
	}
}
