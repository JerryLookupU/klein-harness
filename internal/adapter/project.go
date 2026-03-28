package adapter

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type Paths struct {
	Root                      string
	HarnessDir                string
	StateDir                  string
	EventsDir                 string
	LogsDir                   string
	RequestsDir               string
	CheckpointsDir            string
	ArtifactsDir              string
	TmuxLogsDir               string
	EventLogPath              string
	QueuePath                 string
	LeaseSummaryPath          string
	DispatchSummaryPath       string
	CheckpointSummaryPath     string
	VerificationSummaryPath   string
	ReleaseSnapshotPath       string
	TmuxSummaryPath           string
	RequestSummaryPath        string
	RequestIndexPath          string
	RequestTaskMapPath        string
	CompletionGatePath        string
	GuardStatePath            string
	TaskPoolPath              string
	SessionRegistryPath       string
	LegacySessionRegistryPath string
	RuntimePath               string
	ThreadStatePath           string
	ProjectMetaPath           string
	VerificationRulesPath     string
}

type CommandProfile struct {
	Standard    string `json:"standard"`
	LocalCompat string `json:"localCompat"`
}

type DispatchProfile struct {
	WorkspaceRoot  string         `json:"workspaceRoot"`
	WorktreePath   string         `json:"worktreePath"`
	BranchName     string         `json:"branchName"`
	BaseRef        string         `json:"baseRef"`
	DiffBase       string         `json:"diffBase"`
	CommandProfile CommandProfile `json:"commandProfile"`
}

type Task struct {
	TaskID                    string          `json:"taskId"`
	ProjectID                 string          `json:"projectId,omitempty"`
	ProjectSpaceID            string          `json:"projectSpaceId,omitempty"`
	ThreadKey                 string          `json:"threadKey"`
	Kind                      string          `json:"kind"`
	TaskFamily                string          `json:"taskFamily,omitempty"`
	SOPID                     string          `json:"sopId,omitempty"`
	RoleHint                  string          `json:"roleHint"`
	Title                     string          `json:"title"`
	Summary                   string          `json:"summary"`
	Description               string          `json:"description"`
	WorkerMode                string          `json:"workerMode"`
	Status                    string          `json:"status"`
	PlanEpoch                 int             `json:"planEpoch"`
	WorktreePath              string          `json:"worktreePath"`
	BranchName                string          `json:"branchName"`
	BaseRef                   string          `json:"baseRef"`
	DiffBase                  string          `json:"diffBase"`
	OwnedPaths                []string        `json:"ownedPaths"`
	ForbiddenPaths            []string        `json:"forbiddenPaths"`
	VerificationRuleIDs       []string        `json:"verificationRuleIds"`
	ReviewRequired            bool            `json:"reviewRequired,omitempty"`
	ReviewEvidencePath        string          `json:"reviewEvidencePath,omitempty"`
	VerificationStatus        string          `json:"verificationStatus,omitempty"`
	VerificationSummary       string          `json:"verificationSummary,omitempty"`
	VerificationResultPath    string          `json:"verificationResultPath,omitempty"`
	LastDispatchID            string          `json:"lastDispatchId,omitempty"`
	LastLeaseID               string          `json:"lastLeaseId,omitempty"`
	ExecutionMode             string          `json:"executionMode,omitempty"`
	TmuxSession               string          `json:"tmuxSession,omitempty"`
	TmuxLogPath               string          `json:"tmuxLogPath,omitempty"`
	StatusReason              string          `json:"statusReason,omitempty"`
	CompletedAt               string          `json:"completedAt,omitempty"`
	ArchivedAt                string          `json:"archivedAt,omitempty"`
	UpdatedAt                 string          `json:"updatedAt,omitempty"`
	ResumeStrategy            string          `json:"resumeStrategy"`
	PreferredResumeSessionID  string          `json:"preferredResumeSessionId"`
	CandidateResumeSessionIDs []string        `json:"candidateResumeSessionIds"`
	CheckpointRequired        bool            `json:"checkpointRequired"`
	CheckpointReason          string          `json:"checkpointReason"`
	RoutingModel              string          `json:"routingModel"`
	ExecutionModel            string          `json:"executionModel"`
	OrchestrationSessionID    string          `json:"orchestrationSessionId"`
	PromptStages              []string        `json:"promptStages"`
	Dispatch                  DispatchProfile `json:"dispatch"`
}

type TaskPool struct {
	Tasks []Task `json:"tasks"`
}

type ActiveBinding struct {
	TaskID          string `json:"taskId"`
	SessionID       string `json:"sessionId"`
	NodeID          string `json:"nodeId"`
	BoundFromTaskID string `json:"boundFromTaskId,omitempty"`
}

type SessionRecord struct {
	SessionID           string `json:"sessionId"`
	RootSessionID       string `json:"rootSessionId,omitempty"`
	ParentSessionID     string `json:"parentSessionId,omitempty"`
	BranchRootSessionID string `json:"branchRootSessionId,omitempty"`
	BranchOfSessionID   string `json:"branchOfSessionId,omitempty"`
	SessionFamilyID     string `json:"sessionFamilyId,omitempty"`
	SourceTaskID        string `json:"sourceTaskId,omitempty"`
	Model               string `json:"model,omitempty"`
	Role                string `json:"role,omitempty"`
	Status              string `json:"status,omitempty"`
	Purpose             string `json:"purpose,omitempty"`
	LastUsedAt          string `json:"lastUsedAt,omitempty"`
}

type RoutingDecisionRecord struct {
	TaskID                    string   `json:"taskId"`
	OrchestrationSessionID    string   `json:"orchestrationSessionId,omitempty"`
	RoutingMode               string   `json:"routingMode,omitempty"`
	NeedsOrchestrator         bool     `json:"needsOrchestrator"`
	DispatchReady             bool     `json:"dispatchReady"`
	GateStatus                string   `json:"gateStatus,omitempty"`
	GateReason                string   `json:"gateReason,omitempty"`
	RoutingModel              string   `json:"routingModel,omitempty"`
	ExecutionModel            string   `json:"executionModel,omitempty"`
	ResumeStrategy            string   `json:"resumeStrategy,omitempty"`
	PreferredResumeSessionID  string   `json:"preferredResumeSessionId,omitempty"`
	CandidateResumeSessionIDs []string `json:"candidateResumeSessionIds,omitempty"`
	SessionFamilyID           string   `json:"sessionFamilyId,omitempty"`
	CacheAffinityKey          string   `json:"cacheAffinityKey,omitempty"`
	RoutingReason             string   `json:"routingReason,omitempty"`
	PromptStages              []string `json:"promptStages,omitempty"`
	BranchOfSessionID         string   `json:"branchOfSessionId,omitempty"`
	RoutedAt                  string   `json:"routedAt,omitempty"`
}

type SessionRegistry struct {
	OrchestrationSessionID string                  `json:"orchestrationSessionId"`
	OrchestrationSessions  []SessionRecord         `json:"orchestrationSessions,omitempty"`
	Sessions               []SessionRecord         `json:"sessions,omitempty"`
	RoutingDecisions       []RoutingDecisionRecord `json:"routingDecisions,omitempty"`
	ActiveBindings         []ActiveBinding         `json:"activeBindings"`
	LastCompletedByTask    map[string]string       `json:"lastCompletedByTask,omitempty"`
}

type ProjectMeta struct {
	RepoRole                string `json:"repoRole"`
	DirectTargetEditAllowed *bool  `json:"directTargetEditAllowed"`
}

func Resolve(root string) (Paths, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return Paths{}, err
	}
	harnessDir := filepath.Join(absRoot, ".harness")
	stateDir := filepath.Join(harnessDir, "state")
	eventsDir := filepath.Join(harnessDir, "events")
	logsDir := filepath.Join(harnessDir, "logs")
	requestsDir := filepath.Join(harnessDir, "requests")
	checkpointsDir := filepath.Join(harnessDir, "checkpoints")
	artifactsDir := filepath.Join(harnessDir, "artifacts")
	tmuxLogsDir := filepath.Join(logsDir, "tmux")
	for _, dir := range []string{harnessDir, stateDir, eventsDir, logsDir, requestsDir, checkpointsDir, artifactsDir, tmuxLogsDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return Paths{}, err
		}
	}
	return Paths{
		Root:                      absRoot,
		HarnessDir:                harnessDir,
		StateDir:                  stateDir,
		EventsDir:                 eventsDir,
		LogsDir:                   logsDir,
		RequestsDir:               requestsDir,
		CheckpointsDir:            checkpointsDir,
		ArtifactsDir:              artifactsDir,
		TmuxLogsDir:               tmuxLogsDir,
		EventLogPath:              filepath.Join(eventsDir, "a2a.jsonl"),
		QueuePath:                 filepath.Join(requestsDir, "queue.jsonl"),
		LeaseSummaryPath:          filepath.Join(stateDir, "lease-summary.json"),
		DispatchSummaryPath:       filepath.Join(stateDir, "dispatch-summary.json"),
		CheckpointSummaryPath:     filepath.Join(stateDir, "checkpoint-summary.json"),
		VerificationSummaryPath:   filepath.Join(stateDir, "verification-summary.json"),
		ReleaseSnapshotPath:       filepath.Join(stateDir, "release-snapshot.json"),
		TmuxSummaryPath:           filepath.Join(stateDir, "tmux-summary.json"),
		RequestSummaryPath:        filepath.Join(stateDir, "request-summary.json"),
		RequestIndexPath:          filepath.Join(stateDir, "request-index.json"),
		RequestTaskMapPath:        filepath.Join(stateDir, "request-task-map.json"),
		CompletionGatePath:        filepath.Join(stateDir, "completion-gate.json"),
		GuardStatePath:            filepath.Join(stateDir, "guard-state.json"),
		TaskPoolPath:              filepath.Join(harnessDir, "task-pool.json"),
		SessionRegistryPath:       filepath.Join(stateDir, "session-registry.json"),
		LegacySessionRegistryPath: filepath.Join(harnessDir, "session-registry.json"),
		RuntimePath:               filepath.Join(stateDir, "runtime.json"),
		ThreadStatePath:           filepath.Join(stateDir, "thread-state.json"),
		ProjectMetaPath:           filepath.Join(harnessDir, "project-meta.json"),
		VerificationRulesPath:     filepath.Join(harnessDir, "verification-rules", "manifest.json"),
	}, nil
}

func LoadTask(root, taskID string) (Task, error) {
	pool, err := LoadTaskPool(root)
	if err != nil {
		return Task{}, err
	}
	for _, task := range pool.Tasks {
		if task.TaskID == taskID {
			return task, nil
		}
	}
	return Task{}, errors.New("task not found")
}

func LoadTaskPool(root string) (TaskPool, error) {
	paths, err := Resolve(root)
	if err != nil {
		return TaskPool{}, err
	}
	var pool TaskPool
	if err := loadJSON(paths.TaskPoolPath, &pool); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return TaskPool{}, nil
		}
		return TaskPool{}, err
	}
	return pool, nil
}

func LoadSessionRegistry(root string) (SessionRegistry, error) {
	paths, err := Resolve(root)
	if err != nil {
		return SessionRegistry{}, err
	}
	var registry SessionRegistry
	if err := loadJSON(paths.SessionRegistryPath, &registry); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if err := loadJSON(paths.LegacySessionRegistryPath, &registry); err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return SessionRegistry{}, nil
				}
				return SessionRegistry{}, err
			}
			return registry, nil
		}
		if errors.Is(err, os.ErrNotExist) {
			return SessionRegistry{}, nil
		}
		return SessionRegistry{}, err
	}
	return registry, nil
}

func SaveTaskPool(root string, pool TaskPool) error {
	paths, err := Resolve(root)
	if err != nil {
		return err
	}
	return writeJSON(paths.TaskPoolPath, pool)
}

func UpsertTask(root string, task Task) error {
	pool, err := LoadTaskPool(root)
	if err != nil {
		return err
	}
	for index, existing := range pool.Tasks {
		if existing.TaskID == task.TaskID {
			pool.Tasks[index] = task
			return SaveTaskPool(root, pool)
		}
	}
	pool.Tasks = append(pool.Tasks, task)
	return SaveTaskPool(root, pool)
}

func SaveSessionRegistry(root string, registry SessionRegistry) error {
	paths, err := Resolve(root)
	if err != nil {
		return err
	}
	if err := writeJSON(paths.SessionRegistryPath, registry); err != nil {
		return err
	}
	if _, err := os.Stat(paths.LegacySessionRegistryPath); err == nil {
		if err := writeJSON(paths.LegacySessionRegistryPath, registry); err != nil {
			return err
		}
	}
	return nil
}

func LoadProjectMeta(root string) (ProjectMeta, error) {
	paths, err := Resolve(root)
	if err != nil {
		return ProjectMeta{}, err
	}
	var meta ProjectMeta
	if err := loadJSON(paths.ProjectMetaPath, &meta); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ProjectMeta{}, nil
		}
		return ProjectMeta{}, err
	}
	return meta, nil
}

func LoadLatestPlanEpoch(root string, task Task) (int, error) {
	if task.ThreadKey == "" {
		return task.PlanEpoch, nil
	}
	paths, err := Resolve(root)
	if err != nil {
		return 0, err
	}
	var payload struct {
		Threads map[string]struct {
			LatestValidPlanEpoch int `json:"latestValidPlanEpoch"`
			CurrentPlanEpoch     int `json:"currentPlanEpoch"`
			PlanEpoch            int `json:"planEpoch"`
		} `json:"threads"`
	}
	if err := loadJSON(paths.ThreadStatePath, &payload); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return task.PlanEpoch, nil
		}
		return 0, err
	}
	thread := payload.Threads[task.ThreadKey]
	if thread.LatestValidPlanEpoch > 0 {
		return thread.LatestValidPlanEpoch, nil
	}
	if thread.CurrentPlanEpoch > 0 {
		return thread.CurrentPlanEpoch, nil
	}
	if thread.PlanEpoch > 0 {
		return thread.PlanEpoch, nil
	}
	return task.PlanEpoch, nil
}

func (paths Paths) CompletionGateTaskPath(taskID string) string {
	return filepath.Join(paths.StateDir, "completion-gate-"+taskID+".json")
}

func (paths Paths) GuardStateTaskPath(taskID string) string {
	return filepath.Join(paths.StateDir, "guard-state-"+taskID+".json")
}

func LoadCheckpointFresh(root, taskID string) (bool, error) {
	paths, err := Resolve(root)
	if err != nil {
		return false, err
	}
	var payload struct {
		Tasks map[string]struct {
			LatestCheckpoint struct {
				Status string `json:"status"`
			} `json:"latestCheckpoint"`
		} `json:"tasks"`
	}
	if err := loadJSON(paths.CheckpointSummaryPath, &payload); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	checkpoint := payload.Tasks[taskID]
	switch checkpoint.LatestCheckpoint.Status {
	case "checkpointed", "ready", "succeeded":
		return true, nil
	default:
		return false, nil
	}
}

func CountDispatchAttempts(root, taskID string) (int, error) {
	paths, err := Resolve(root)
	if err != nil {
		return 0, err
	}
	var payload struct {
		TaskIndex map[string][]string `json:"taskIndex"`
	}
	if err := loadJSON(paths.DispatchSummaryPath, &payload); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}
	return len(payload.TaskIndex[taskID]), nil
}

func TaskCWD(paths Paths, task Task) string {
	if task.Dispatch.WorktreePath != "" {
		return joinRoot(paths.Root, task.Dispatch.WorktreePath)
	}
	if task.WorktreePath != "" {
		return joinRoot(paths.Root, task.WorktreePath)
	}
	return paths.Root
}

func DispatchCommand(task Task) string {
	if task.Dispatch.CommandProfile.Standard != "" {
		return task.Dispatch.CommandProfile.Standard
	}
	return task.Dispatch.CommandProfile.LocalCompat
}

func joinRoot(root, path string) string {
	if path == "" {
		return root
	}
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(root, path)
}

func loadJSON(path string, target any) error {
	payload, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(payload, target)
}

func writeJSON(path string, value any) error {
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(payload, '\n'), 0o644)
}
