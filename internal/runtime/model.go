package runtime

import "klein-harness/internal/state"

type RequestRecord struct {
	RequestID             string   `json:"requestId"`
	ProjectID             string   `json:"projectId,omitempty"`
	ProjectSpaceID        string   `json:"projectSpaceId,omitempty"`
	TaskID                string   `json:"taskId,omitempty"`
	BindingAction         string   `json:"bindingAction,omitempty"`
	ReusedTaskID          string   `json:"reusedTaskId,omitempty"`
	AppendToTaskID        string   `json:"appendToTaskId,omitempty"`
	AppendToThreadKey     string   `json:"appendToThreadKey,omitempty"`
	ThreadKey             string   `json:"threadKey,omitempty"`
	TargetThreadKey       string   `json:"targetThreadKey,omitempty"`
	TargetPlanEpoch       int      `json:"targetPlanEpoch,omitempty"`
	Kind                  string   `json:"kind,omitempty"`
	TaskFamily            string   `json:"taskFamily,omitempty"`
	SOPID                 string   `json:"sopId,omitempty"`
	Goal                  string   `json:"goal"`
	Contexts              []string `json:"contexts,omitempty"`
	Status                string   `json:"status"`
	FrontDoorTriage       string   `json:"frontDoorTriage,omitempty"`
	NormalizedIntentClass string   `json:"normalizedIntentClass,omitempty"`
	FusionDecision        string   `json:"fusionDecision,omitempty"`
	IdempotencyKey        string   `json:"idempotencyKey,omitempty"`
	CanonicalGoalHash     string   `json:"canonicalGoalHash,omitempty"`
	EvidenceFingerprint   string   `json:"evidenceFingerprint,omitempty"`
	ClassificationReason  string   `json:"classificationReason,omitempty"`
	CreatedAt             string   `json:"createdAt"`
	UpdatedAt             string   `json:"updatedAt"`
}

type RequestSummary struct {
	state.Metadata
	LatestRequestID       string `json:"latestRequestId,omitempty"`
	LatestTaskID          string `json:"latestTaskId,omitempty"`
	LatestThreadKey       string `json:"latestThreadKey,omitempty"`
	FrontDoorTriage       string `json:"frontDoorTriage,omitempty"`
	NormalizedIntentClass string `json:"normalizedIntentClass,omitempty"`
	FusionDecision        string `json:"fusionDecision,omitempty"`
	BindingAction         string `json:"bindingAction,omitempty"`
	TargetPlanEpoch       int    `json:"targetPlanEpoch,omitempty"`
	TaskFamily            string `json:"taskFamily,omitempty"`
	SOPID                 string `json:"sopId,omitempty"`
	RequestCount          int    `json:"requestCount"`
	ReusedTaskCount       int    `json:"reusedTaskCount"`
	CreatedTaskCount      int    `json:"createdTaskCount"`
}

type RequestIndex struct {
	state.Metadata
	RequestsByID                  map[string]RequestRecord `json:"requestsById,omitempty"`
	LatestRequestByTaskID         map[string]string        `json:"latestRequestByTaskId,omitempty"`
	LatestRequestByThreadKey      map[string]string        `json:"latestRequestByThreadKey,omitempty"`
	LatestRequestByIdempotencyKey map[string]string        `json:"latestRequestByIdempotencyKey,omitempty"`
}

type RequestTaskMap struct {
	state.Metadata
	RequestToTask    map[string]string   `json:"requestToTask,omitempty"`
	RequestToThread  map[string]string   `json:"requestToThread,omitempty"`
	TaskToRequests   map[string][]string `json:"taskToRequests,omitempty"`
	ThreadToRequests map[string][]string `json:"threadToRequests,omitempty"`
	ThreadToTasks    map[string][]string `json:"threadToTasks,omitempty"`
}

type IntakeSummary struct {
	state.Metadata
	LatestRequestID       string `json:"latestRequestId,omitempty"`
	LatestTaskID          string `json:"latestTaskId,omitempty"`
	LatestThreadKey       string `json:"latestThreadKey,omitempty"`
	FrontDoorTriage       string `json:"frontDoorTriage,omitempty"`
	NormalizedIntentClass string `json:"normalizedIntentClass,omitempty"`
	FusionDecision        string `json:"fusionDecision,omitempty"`
	TaskFamily            string `json:"taskFamily,omitempty"`
	SOPID                 string `json:"sopId,omitempty"`
	RequestCount          int    `json:"requestCount"`
	ActiveThreadCount     int    `json:"activeThreadCount"`
}

type ThreadEntry struct {
	ThreadKey            string   `json:"threadKey"`
	ProjectID            string   `json:"projectId,omitempty"`
	ProjectSpaceID       string   `json:"projectSpaceId,omitempty"`
	CanonicalGoalHash    string   `json:"canonicalGoalHash,omitempty"`
	LatestRequestID      string   `json:"latestRequestId,omitempty"`
	LatestTaskID         string   `json:"latestTaskId,omitempty"`
	PlanEpoch            int      `json:"planEpoch,omitempty"`
	CurrentPlanEpoch     int      `json:"currentPlanEpoch,omitempty"`
	LatestValidPlanEpoch int      `json:"latestValidPlanEpoch,omitempty"`
	RequestIDs           []string `json:"requestIds,omitempty"`
	TaskIDs              []string `json:"taskIds,omitempty"`
	Status               string   `json:"status,omitempty"`
	UpdatedAt            string   `json:"updatedAt,omitempty"`
}

type ThreadState struct {
	state.Metadata
	Threads map[string]ThreadEntry `json:"threads"`
}

type ChangeSummary struct {
	state.Metadata
	LatestRequestID  string `json:"latestRequestId,omitempty"`
	LatestTaskID     string `json:"latestTaskId,omitempty"`
	TargetThreadKey  string `json:"targetThreadKey,omitempty"`
	ChangeKind       string `json:"changeKind,omitempty"`
	TaskFamily       string `json:"taskFamily,omitempty"`
	SOPID            string `json:"sopId,omitempty"`
	Summary          string `json:"summary,omitempty"`
	AffectsExecution bool   `json:"affectsExecution"`
}

type TodoSummary struct {
	state.Metadata
	NextTaskID      string   `json:"nextTaskId,omitempty"`
	TaskIDs         []string `json:"taskIds,omitempty"`
	PendingCount    int      `json:"pendingCount"`
	ActiveThreadKey string   `json:"activeThreadKey,omitempty"`
	LatestRequestID string   `json:"latestRequestId,omitempty"`
}

type RuntimeState struct {
	state.Metadata
	Status                    string   `json:"status"`
	ActiveTaskID              string   `json:"activeTaskId,omitempty"`
	ActiveTaskFamily          string   `json:"activeTaskFamily,omitempty"`
	ActiveSOPID               string   `json:"activeSopId,omitempty"`
	ActiveThreadKey           string   `json:"activeThreadKey,omitempty"`
	CurrentDispatchID         string   `json:"currentDispatchId,omitempty"`
	CurrentExecutionSliceID   string   `json:"currentExecutionSliceId,omitempty"`
	CurrentResumeSessionID    string   `json:"currentResumeSessionId,omitempty"`
	CurrentTakeoverPath       string   `json:"currentTakeoverPath,omitempty"`
	CurrentContextLayersPath  string   `json:"currentContextLayersPath,omitempty"`
	CurrentTaskGraphPath      string   `json:"currentTaskGraphPath,omitempty"`
	CurrentVerifySkeletonPath string   `json:"currentVerifySkeletonPath,omitempty"`
	CurrentCloseoutPath       string   `json:"currentCloseoutPath,omitempty"`
	CurrentHandoffPath        string   `json:"currentHandoffPath,omitempty"`
	CurrentArtifactDir        string   `json:"currentArtifactDir,omitempty"`
	CurrentWorktreePath       string   `json:"currentWorktreePath,omitempty"`
	CurrentOwnedPaths         []string `json:"currentOwnedPaths,omitempty"`
	LastVerificationStatus    string   `json:"lastVerificationStatus,omitempty"`
	LastFollowUp              string   `json:"lastFollowUp,omitempty"`
	LastRunAt                 string   `json:"lastRunAt,omitempty"`
	LastError                 string   `json:"lastError,omitempty"`
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
