package runtime

import "klein-harness/internal/state"

type RequestRecord struct {
	RequestID string   `json:"requestId"`
	TaskID    string   `json:"taskId,omitempty"`
	Kind      string   `json:"kind,omitempty"`
	Goal      string   `json:"goal"`
	Contexts  []string `json:"contexts,omitempty"`
	Status    string   `json:"status"`
	CreatedAt string   `json:"createdAt"`
	UpdatedAt string   `json:"updatedAt"`
}

type RuntimeState struct {
	state.Metadata
	Status       string `json:"status"`
	ActiveTaskID string `json:"activeTaskId,omitempty"`
	LastRunAt    string `json:"lastRunAt,omitempty"`
	LastError    string `json:"lastError,omitempty"`
}

type VerificationEntry struct {
	TaskID     string `json:"taskId"`
	DispatchID string `json:"dispatchId,omitempty"`
	Status     string `json:"status"`
	Summary    string `json:"summary,omitempty"`
	ResultPath string `json:"resultPath,omitempty"`
	UpdatedAt  string `json:"updatedAt"`
	Completed  bool   `json:"completed"`
	FollowUp   string `json:"followUp,omitempty"`
}

type VerificationSummary struct {
	state.Metadata
	Tasks map[string]VerificationEntry `json:"tasks"`
}

type TmuxSession struct {
	SessionName   string `json:"sessionName"`
	TaskID        string `json:"taskId"`
	DispatchID    string `json:"dispatchId"`
	WorkerID      string `json:"workerId,omitempty"`
	Status        string `json:"status"`
	LogPath       string `json:"logPath,omitempty"`
	Cwd           string `json:"cwd,omitempty"`
	Command       string `json:"command,omitempty"`
	StartedAt     string `json:"startedAt,omitempty"`
	FinishedAt    string `json:"finishedAt,omitempty"`
	ExitCode      int    `json:"exitCode,omitempty"`
	AttachCommand string `json:"attachCommand,omitempty"`
}

type TmuxSummary struct {
	state.Metadata
	Sessions     map[string]TmuxSession `json:"sessions"`
	LatestByTask map[string]string      `json:"latestByTask"`
}
