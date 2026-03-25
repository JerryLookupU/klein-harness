package bootstrap

import (
	"encoding/json"
	"os"
	"path/filepath"

	"klein-harness/internal/adapter"
	"klein-harness/internal/state"
	"klein-harness/internal/tmux"
)

type taskPoolFile struct {
	Tasks []adapter.Task `json:"tasks"`
}

type verificationManifest struct {
	Rules []map[string]any `json:"rules"`
}

type emptySummary struct {
	state.Metadata
}

type runtimeState struct {
	state.Metadata
	Status       string `json:"status"`
	ActiveTaskID string `json:"activeTaskId,omitempty"`
	LastRunAt    string `json:"lastRunAt,omitempty"`
	LastError    string `json:"lastError,omitempty"`
}

type verificationEntry struct {
	TaskID     string `json:"taskId"`
	DispatchID string `json:"dispatchId,omitempty"`
	Status     string `json:"status,omitempty"`
	Summary    string `json:"summary,omitempty"`
	ResultPath string `json:"resultPath,omitempty"`
	UpdatedAt  string `json:"updatedAt,omitempty"`
	Completed  bool   `json:"completed,omitempty"`
	FollowUp   string `json:"followUp,omitempty"`
}

type verificationSummary struct {
	state.Metadata
	Tasks map[string]verificationEntry `json:"tasks"`
}

func Init(root string) (adapter.Paths, error) {
	paths, err := adapter.Resolve(root)
	if err != nil {
		return adapter.Paths{}, err
	}

	if err := writeJSONIfMissing(paths.TaskPoolPath, taskPoolFile{Tasks: []adapter.Task{}}); err != nil {
		return adapter.Paths{}, err
	}
	if err := writeJSONIfMissing(paths.ProjectMetaPath, map[string]any{
		"repoRole":                "target_repo",
		"directTargetEditAllowed": true,
	}); err != nil {
		return adapter.Paths{}, err
	}
	if err := writeJSONIfMissing(paths.VerificationRulesPath, verificationManifest{Rules: []map[string]any{}}); err != nil {
		return adapter.Paths{}, err
	}
	if err := writeSnapshotIfMissing(paths.RuntimePath, &runtimeState{
		Status: "idle",
	}); err != nil {
		return adapter.Paths{}, err
	}
	if err := writeSnapshotIfMissing(paths.DispatchSummaryPath, &emptySummary{}); err != nil {
		return adapter.Paths{}, err
	}
	if err := writeSnapshotIfMissing(paths.LeaseSummaryPath, &emptySummary{}); err != nil {
		return adapter.Paths{}, err
	}
	if err := writeSnapshotIfMissing(paths.CheckpointSummaryPath, &emptySummary{}); err != nil {
		return adapter.Paths{}, err
	}
	if err := writeSnapshotIfMissing(paths.VerificationSummaryPath, &verificationSummary{
		Tasks: map[string]verificationEntry{},
	}); err != nil {
		return adapter.Paths{}, err
	}
	if err := writeSnapshotIfMissing(paths.TmuxSummaryPath, &tmux.Summary{
		Sessions:     map[string]tmux.SessionState{},
		LatestByTask: map[string]string{},
	}); err != nil {
		return adapter.Paths{}, err
	}
	if err := writeSnapshotIfMissing(paths.CompletionGatePath, &emptySummary{}); err != nil {
		return adapter.Paths{}, err
	}
	if err := writeSnapshotIfMissing(paths.GuardStatePath, &emptySummary{}); err != nil {
		return adapter.Paths{}, err
	}
	if err := writeJSONIfMissing(paths.SessionRegistryPath, adapter.SessionRegistry{}); err != nil {
		return adapter.Paths{}, err
	}
	if err := ensureEmptyFile(paths.QueuePath); err != nil {
		return adapter.Paths{}, err
	}
	return paths, nil
}

func ensureEmptyFile(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	return os.WriteFile(path, nil, 0o644)
}

func writeJSONIfMissing(path string, value any) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(payload, '\n'), 0o644)
}

func writeSnapshotIfMissing(path string, value any) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	_, err := state.WriteSnapshot(path, value, "harness-bootstrap", 0)
	return err
}
