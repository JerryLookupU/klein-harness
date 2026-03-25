package tmux

import (
	"klein-harness/internal/adapter"
	"klein-harness/internal/state"
)

type SessionState struct {
	SessionName   string `json:"sessionName"`
	TaskID        string `json:"taskId,omitempty"`
	DispatchID    string `json:"dispatchId,omitempty"`
	WorkerID      string `json:"workerId,omitempty"`
	Cwd           string `json:"cwd,omitempty"`
	LogPath       string `json:"logPath,omitempty"`
	CheckpointRef string `json:"checkpointRef,omitempty"`
	OutcomeRef    string `json:"outcomeRef,omitempty"`
	Status        string `json:"status,omitempty"`
	StartedAt     string `json:"startedAt,omitempty"`
	FinishedAt    string `json:"finishedAt,omitempty"`
	ExitCode      int    `json:"exitCode,omitempty"`
	AttachCommand string `json:"attachCommand,omitempty"`
}

type Summary struct {
	state.Metadata
	Sessions     map[string]SessionState `json:"sessions"`
	LatestByTask map[string]string       `json:"latestByTask"`
}

func LoadSummary(root string) (Summary, error) {
	paths, err := adapter.Resolve(root)
	if err != nil {
		return Summary{}, err
	}
	summary := Summary{
		Sessions:     map[string]SessionState{},
		LatestByTask: map[string]string{},
	}
	if _, err := state.LoadJSONIfExists(paths.TmuxSummaryPath, &summary); err != nil {
		return Summary{}, err
	}
	if summary.Sessions == nil {
		summary.Sessions = map[string]SessionState{}
	}
	if summary.LatestByTask == nil {
		summary.LatestByTask = map[string]string{}
	}
	return summary, nil
}
