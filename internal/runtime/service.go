package runtime

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"klein-harness/internal/adapter"
	"klein-harness/internal/bootstrap"
	"klein-harness/internal/checkpoint"
	"klein-harness/internal/codexconfig"
	"klein-harness/internal/dispatch"
	executorcodex "klein-harness/internal/executor/codex"
	"klein-harness/internal/lease"
	"klein-harness/internal/route"
	"klein-harness/internal/state"
	"klein-harness/internal/tmux"
	"klein-harness/internal/verify"
	"klein-harness/internal/worker"
	"klein-harness/internal/worktree"
)

type SubmitRequest struct {
	Root     string
	Goal     string
	Kind     string
	Contexts []string
}

type SubmitResult struct {
	Initialized bool          `json:"initialized"`
	Request     RequestRecord `json:"request"`
	Task        adapter.Task  `json:"task"`
}

type RunOptions struct {
	WorkerID         string
	Model            string
	ApprovalPolicy   string
	SandboxMode      string
	SkipGitRepoCheck bool
}

type RunResult struct {
	RuntimeStatus string          `json:"runtimeStatus"`
	Task          adapter.Task    `json:"task,omitempty"`
	Route         route.Decision  `json:"route,omitempty"`
	Dispatch      dispatch.Ticket `json:"dispatch,omitempty"`
	LeaseID       string          `json:"leaseId,omitempty"`
	BurstStatus   string          `json:"burstStatus,omitempty"`
	VerifyStatus  string          `json:"verifyStatus,omitempty"`
	FollowUpEvent string          `json:"followUpEvent,omitempty"`
}

func Submit(request SubmitRequest) (SubmitResult, error) {
	if strings.TrimSpace(request.Goal) == "" {
		return SubmitResult{}, errors.New("goal is required")
	}
	paths, err := bootstrap.Init(request.Root)
	if err != nil {
		return SubmitResult{}, err
	}
	pool, err := adapter.LoadTaskPool(paths.Root)
	if err != nil {
		return SubmitResult{}, err
	}
	now := state.NowUTC()
	taskID := nextTaskID(pool.Tasks)
	requestID := nextRequestID(paths.QueuePath)
	kind := strings.TrimSpace(request.Kind)
	if kind == "" {
		kind = inferKind(request.Goal)
	}
	task := adapter.Task{
		TaskID:                 taskID,
		ThreadKey:              requestID,
		Kind:                   kind,
		RoleHint:               "worker",
		Title:                  shortTitle(request.Goal),
		Summary:                request.Goal,
		Description:            strings.Join(uniqueNonEmpty(request.Contexts), "\n"),
		WorkerMode:             "execution",
		Status:                 "queued",
		StatusReason:           "submitted",
		PlanEpoch:              1,
		OwnedPaths:             defaultOwnedPaths(paths.Root),
		ForbiddenPaths:         []string{".git/**", ".harness/**"},
		VerificationRuleIDs:    []string{},
		ResumeStrategy:         "fresh",
		RoutingModel:           "gpt-5.4",
		ExecutionModel:         "gpt-5.3-codex",
		OrchestrationSessionID: "runtime",
		PromptStages:           []string{"route", "dispatch", "execute", "verify"},
		Dispatch: adapter.DispatchProfile{
			WorkspaceRoot: paths.Root,
			WorktreePath:  ".",
			BranchName:    "main",
			BaseRef:       "HEAD",
			DiffBase:      "HEAD",
		},
		UpdatedAt: now,
	}
	if err := adapter.UpsertTask(paths.Root, task); err != nil {
		return SubmitResult{}, err
	}
	record := RequestRecord{
		RequestID: requestID,
		TaskID:    taskID,
		Kind:      kind,
		Goal:      request.Goal,
		Contexts:  uniqueNonEmpty(request.Contexts),
		Status:    "queued",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := appendRequest(paths.QueuePath, record); err != nil {
		return SubmitResult{}, err
	}
	if err := updateRuntime(paths.RuntimePath, func(current RuntimeState) RuntimeState {
		current.Status = "queued"
		current.ActiveTaskID = taskID
		current.LastRunAt = now
		return current
	}); err != nil {
		return SubmitResult{}, err
	}
	return SubmitResult{
		Initialized: true,
		Request:     record,
		Task:        task,
	}, nil
}

func RunOnce(root string, options RunOptions) (RunResult, error) {
	paths, err := bootstrap.Init(root)
	if err != nil {
		return RunResult{}, err
	}
	task, ok, err := nextRunnableTask(paths.Root)
	if err != nil {
		return RunResult{}, err
	}
	if !ok {
		if err := updateRuntime(paths.RuntimePath, func(current RuntimeState) RuntimeState {
			current.Status = "idle"
			current.ActiveTaskID = ""
			current.LastRunAt = state.NowUTC()
			current.LastError = ""
			return current
		}); err != nil {
			return RunResult{}, err
		}
		return RunResult{RuntimeStatus: "idle"}, nil
	}

	workerID := strings.TrimSpace(options.WorkerID)
	if workerID == "" {
		workerID = "harness-daemon"
	}
	now := state.NowUTC()
	if err := updateTask(paths.Root, task.TaskID, func(current *adapter.Task) {
		current.Status = "routing"
		current.StatusReason = "daemon run-once"
		current.UpdatedAt = now
	}); err != nil {
		return RunResult{}, err
	}
	if err := updateRuntime(paths.RuntimePath, func(current RuntimeState) RuntimeState {
		current.Status = "running"
		current.ActiveTaskID = task.TaskID
		current.LastRunAt = now
		current.LastError = ""
		return current
	}); err != nil {
		return RunResult{}, err
	}

	latestPlanEpoch, err := adapter.LoadLatestPlanEpoch(paths.Root, task)
	if err != nil {
		return RunResult{}, err
	}
	checkpointFresh, err := adapter.LoadCheckpointFresh(paths.Root, task.TaskID)
	if err != nil {
		return RunResult{}, err
	}
	registry, err := adapter.LoadSessionRegistry(paths.Root)
	if err != nil {
		return RunResult{}, err
	}
	sessionContested := false
	if task.PreferredResumeSessionID != "" {
		for _, binding := range registry.ActiveBindings {
			if binding.TaskID != task.TaskID && binding.SessionID == task.PreferredResumeSessionID {
				sessionContested = true
				break
			}
		}
	}
	decision := route.Evaluate(route.Input{
		TaskID:                    task.TaskID,
		RoleHint:                  task.RoleHint,
		Kind:                      task.Kind,
		Title:                     task.Title,
		Summary:                   strings.TrimSpace(strings.Join([]string{task.Summary, task.Description}, "\n")),
		WorkerMode:                task.WorkerMode,
		PlanEpoch:                 task.PlanEpoch,
		LatestPlanEpoch:           latestPlanEpoch,
		ResumeStrategy:            task.ResumeStrategy,
		PreferredResumeSessionID:  task.PreferredResumeSessionID,
		CandidateResumeSessionIDs: task.CandidateResumeSessionIDs,
		SessionContested:          sessionContested,
		CheckpointRequired:        task.CheckpointRequired,
		CheckpointFresh:           checkpointFresh,
		WorktreePath:              adapter.TaskCWD(paths, task),
		OwnedPaths:                task.OwnedPaths,
		RequiredSummaryVersion:    runtimeSummaryVersion(paths.RuntimePath),
	})
	if !decision.DispatchReady {
		status := "blocked"
		if decision.Route == "replan" {
			status = "needs_replan"
		}
		if err := updateTask(paths.Root, task.TaskID, func(current *adapter.Task) {
			current.Status = status
			current.StatusReason = strings.Join(decision.ReasonCodes, ", ")
			current.UpdatedAt = state.NowUTC()
		}); err != nil {
			return RunResult{}, err
		}
		if err := updateRuntime(paths.RuntimePath, func(current RuntimeState) RuntimeState {
			current.Status = status
			current.ActiveTaskID = task.TaskID
			current.LastRunAt = state.NowUTC()
			return current
		}); err != nil {
			return RunResult{}, err
		}
		return RunResult{
			RuntimeStatus: status,
			Task:          task,
			Route:         decision,
		}, nil
	}

	profile := codexconfig.Effective(codexconfig.Config{}, "", codexconfig.Profile{
		Model:          coalesce(options.Model, task.ExecutionModel, "gpt-5.3-codex"),
		ApprovalPolicy: coalesce(options.ApprovalPolicy, "never"),
		SandboxMode:    coalesce(options.SandboxMode, "workspace-write"),
	})
	command := executorcodex.BuildFresh(profile, executorcodex.BuildOptions{
		SkipGitRepoCheck:      options.SkipGitRepoCheck,
		OutputLastMessagePath: "<LAST_MESSAGE_PATH>",
	})
	if decision.ResumeSessionID != "" {
		command = executorcodex.BuildResume(profile, executorcodex.BuildOptions{
			SessionID:             decision.ResumeSessionID,
			SkipGitRepoCheck:      options.SkipGitRepoCheck,
			OutputLastMessagePath: "<LAST_MESSAGE_PATH>",
		})
	}

	attempt, err := adapter.CountDispatchAttempts(paths.Root, task.TaskID)
	if err != nil {
		return RunResult{}, err
	}
	dispatchCausationID := fmt.Sprintf("route:%s:%d:%d", task.TaskID, task.PlanEpoch, attempt+1)
	ticket, _, err := dispatch.Issue(dispatch.IssueRequest{
		Root:                   paths.Root,
		RequestID:              task.ThreadKey,
		TaskID:                 task.TaskID,
		ThreadKey:              task.ThreadKey,
		PlanEpoch:              task.PlanEpoch,
		Attempt:                attempt + 1,
		IdempotencyKey:         fmt.Sprintf("dispatch:%s:epoch_%d:attempt_%d", task.TaskID, task.PlanEpoch, attempt+1),
		CausationID:            dispatchCausationID,
		ReasonCodes:            decision.ReasonCodes,
		WorkerClass:            profile.Model,
		Cwd:                    adapter.TaskCWD(paths, task),
		Command:                command,
		PromptRef:              "prompts/worker-burst.md",
		Budget:                 dispatch.Budget{MaxTurns: 8, MaxMinutes: 20, MaxToolCalls: 30},
		LeaseTTLSec:            1800,
		RequiredSummaryVersion: decision.RequiredSummaryVersion,
		ResumeSessionID:        decision.ResumeSessionID,
		WorktreePath:           decision.WorktreePath,
		OwnedPaths:             decision.OwnedPaths,
	})
	if err != nil {
		return RunResult{}, err
	}
	leaseRecord, err := lease.Acquire(lease.AcquireRequest{
		Root:        paths.Root,
		TaskID:      ticket.TaskID,
		DispatchID:  ticket.DispatchID,
		WorkerID:    workerID,
		TTLSeconds:  ticket.LeaseTTLSec,
		CausationID: dispatchCausationID,
		ReasonCodes: []string{"daemon_run_once"},
	})
	if err != nil {
		return RunResult{}, err
	}
	ticket, err = dispatch.Claim(dispatch.ClaimRequest{
		Root:        paths.Root,
		DispatchID:  ticket.DispatchID,
		WorkerID:    workerID,
		LeaseID:     leaseRecord.LeaseID,
		CausationID: dispatchCausationID,
		ReasonCodes: []string{"daemon_run_once"},
	})
	if err != nil {
		return RunResult{}, err
	}
	bundle, err := worker.Prepare(paths.Root, ticket, leaseRecord.LeaseID)
	if err != nil {
		return RunResult{}, err
	}
	if err := updateTask(paths.Root, task.TaskID, func(current *adapter.Task) {
		current.Status = "running"
		current.LastDispatchID = ticket.DispatchID
		current.LastLeaseID = leaseRecord.LeaseID
		current.VerificationStatus = ""
		current.VerificationSummary = ""
		current.VerificationResultPath = ""
		current.UpdatedAt = state.NowUTC()
	}); err != nil {
		return RunResult{}, err
	}

	checkpointPath := dispatch.DefaultCheckpointPath(paths.Root, ticket.TaskID, ticket.Attempt)
	outcomePath := filepath.Join(filepath.Dir(checkpointPath), "outcome.json")
	burst, err := tmux.RunBoundedBurst(tmux.BurstRequest{
		Root:           paths.Root,
		TaskID:         ticket.TaskID,
		DispatchID:     ticket.DispatchID,
		WorkerID:       workerID,
		Cwd:            ticket.Cwd,
		Command:        resolveCommand(ticket.Command, map[string]string{"LAST_MESSAGE_PATH": filepath.Join(bundle.ArtifactDir, "last-message.txt")}),
		PromptPath:     bundle.PromptPath,
		Budget:         ticket.Budget,
		CheckpointPath: checkpointPath,
		OutcomePath:    outcomePath,
		Artifacts: []string{
			bundle.TicketPath,
			bundle.WorkerSpecPath,
			bundle.PromptPath,
			filepath.Join(bundle.ArtifactDir, "worker-result.json"),
			filepath.Join(bundle.ArtifactDir, "verify.json"),
			filepath.Join(bundle.ArtifactDir, "handoff.md"),
		},
	})
	if err != nil {
		return RunResult{}, err
	}

	if violation, ok := ownedPathViolation(bundle.ArtifactDir, task.OwnedPaths); ok {
		burst.Status = "failed"
		burst.Summary = violation
	}

	if _, err := checkpoint.IngestCheckpoint(checkpoint.IngestCheckpointRequest{
		Root:          paths.Root,
		RequestID:     task.ThreadKey,
		TaskID:        ticket.TaskID,
		DispatchID:    ticket.DispatchID,
		PlanEpoch:     ticket.PlanEpoch,
		Attempt:       ticket.Attempt,
		CausationID:   dispatchCausationID,
		ReasonCodes:   []string{"daemon_checkpoint"},
		ThreadKey:     ticket.ThreadKey,
		LeaseID:       leaseRecord.LeaseID,
		CheckpointRef: checkpointPath,
		Status:        "checkpointed",
		Summary:       "daemon run-once checkpoint persisted",
	}); err != nil {
		return RunResult{}, err
	}
	nextKind := ""
	if burst.Status == "failed" || burst.Status == "timed_out" {
		nextKind = "replan"
	}
	if _, err := checkpoint.IngestOutcome(checkpoint.IngestOutcomeRequest{
		Root:              paths.Root,
		RequestID:         task.ThreadKey,
		TaskID:            ticket.TaskID,
		DispatchID:        ticket.DispatchID,
		PlanEpoch:         ticket.PlanEpoch,
		Attempt:           ticket.Attempt,
		CausationID:       dispatchCausationID,
		WorkerID:          workerID,
		LeaseID:           leaseRecord.LeaseID,
		ReasonCodes:       []string{"daemon_outcome"},
		ThreadKey:         ticket.ThreadKey,
		Status:            burst.Status,
		Summary:           burst.Summary,
		CheckpointRef:     checkpointPath,
		DiffStats:         checkpoint.DiffStats{FilesChanged: burst.DiffStats["filesChanged"], Insertions: burst.DiffStats["insertions"], Deletions: burst.DiffStats["deletions"]},
		Artifacts:         burst.Artifacts,
		NextSuggestedKind: nextKind,
	}); err != nil {
		return RunResult{}, err
	}
	if _, err := dispatch.UpdateStatus(paths.Root, ticket.DispatchID, burst.Status, "harness-runtime"); err != nil {
		return RunResult{}, err
	}
	if _, err := lease.Release(paths.Root, leaseRecord.LeaseID, dispatchCausationID, []string{"daemon_finished"}); err != nil {
		return RunResult{}, err
	}

	verifyStatus, verifySummary, verifyPath := deriveVerification(bundle.ArtifactDir, burst.Status, burst.Summary)
	sessionID, err := ingestSessionBinding(paths.Root, task, bundle.ArtifactDir)
	if err != nil {
		return RunResult{}, err
	}
	followUp := ""
	verifyErr := error(nil)
	if verifyStatus != "" {
		verifyResult, err := verify.Ingest(verify.Request{
			Root:                   paths.Root,
			RequestID:              task.ThreadKey,
			TaskID:                 ticket.TaskID,
			DispatchID:             ticket.DispatchID,
			PlanEpoch:              ticket.PlanEpoch,
			Attempt:                ticket.Attempt,
			CausationID:            dispatchCausationID,
			ReasonCodes:            decision.ReasonCodes,
			Status:                 verifyStatus,
			Summary:                verifySummary,
			VerificationResultPath: verifyPath,
			FollowUp:               nextKind,
		})
		if err != nil {
			verifyErr = err
		} else {
			followUp = verifyResult.FollowUpEvent
		}
	}

	taskStatus := burst.Status
	switch {
	case verifyStatus == "passed" && verifyErr == nil && followUp == "task.completed":
		taskStatus = "completed"
	case verifyStatus == "blocked":
		taskStatus = "blocked"
	case followUp == "replan.emitted" || burst.Status == "failed" || burst.Status == "timed_out":
		taskStatus = "needs_replan"
	case verifyErr != nil:
		taskStatus = "verified"
	}
	verifyCompleted := followUp == "task.completed"
	if err := updateTask(paths.Root, task.TaskID, func(current *adapter.Task) {
		current.Status = taskStatus
		current.StatusReason = coalesce(followUp, burst.Summary)
		current.LastDispatchID = ticket.DispatchID
		current.LastLeaseID = ""
		current.VerificationStatus = verifyStatus
		current.VerificationSummary = verifySummary
		current.VerificationResultPath = verifyPath
		if sessionID != "" {
			current.PreferredResumeSessionID = sessionID
			current.CandidateResumeSessionIDs = uniqueNonEmpty(append([]string{sessionID}, current.CandidateResumeSessionIDs...))
			current.ResumeStrategy = "resume"
		}
		current.TmuxSession = burst.SessionName
		current.TmuxLogPath = burst.LogPath
		current.UpdatedAt = state.NowUTC()
		if verifyCompleted {
			current.CompletedAt = state.NowUTC()
		}
	}); err != nil {
		return RunResult{}, err
	}
	if err := updateVerification(paths.VerificationSummaryPath, VerificationEntry{
		TaskID:     task.TaskID,
		DispatchID: ticket.DispatchID,
		Status:     verifyStatus,
		Summary:    verifySummary,
		ResultPath: verifyPath,
		UpdatedAt:  state.NowUTC(),
		Completed:  verifyCompleted,
		FollowUp:   coalesce(followUp, errorString(verifyErr)),
	}); err != nil {
		return RunResult{}, err
	}
	if err := updateRuntime(paths.RuntimePath, func(current RuntimeState) RuntimeState {
		current.Status = taskStatus
		current.ActiveTaskID = task.TaskID
		current.LastRunAt = state.NowUTC()
		current.LastError = errorString(verifyErr)
		return current
	}); err != nil {
		return RunResult{}, err
	}

	finalTask, err := adapter.LoadTask(paths.Root, task.TaskID)
	if err != nil {
		return RunResult{}, err
	}
	return RunResult{
		RuntimeStatus: taskStatus,
		Task:          finalTask,
		Route:         decision,
		Dispatch:      ticket,
		LeaseID:       leaseRecord.LeaseID,
		BurstStatus:   burst.Status,
		VerifyStatus:  verifyStatus,
		FollowUpEvent: followUp,
	}, nil
}

func Loop(root string, interval time.Duration, options RunOptions) error {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	for {
		if _, err := RunOnce(root, options); err != nil {
			return err
		}
		time.Sleep(interval)
	}
}

func nextRunnableTask(root string) (adapter.Task, bool, error) {
	pool, err := adapter.LoadTaskPool(root)
	if err != nil {
		return adapter.Task{}, false, err
	}
	sort.SliceStable(pool.Tasks, func(i, j int) bool {
		return pool.Tasks[i].TaskID < pool.Tasks[j].TaskID
	})
	for _, task := range pool.Tasks {
		switch task.Status {
		case "", "queued", "needs_replan", "recoverable":
			return task, true, nil
		}
	}
	return adapter.Task{}, false, nil
}

func nextTaskID(tasks []adapter.Task) string {
	maxValue := 0
	for _, task := range tasks {
		if !strings.HasPrefix(task.TaskID, "T-") {
			continue
		}
		value, err := strconv.Atoi(strings.TrimPrefix(task.TaskID, "T-"))
		if err == nil && value > maxValue {
			maxValue = value
		}
	}
	return fmt.Sprintf("T-%03d", maxValue+1)
}

func nextRequestID(queuePath string) string {
	file, err := os.Open(queuePath)
	if err != nil {
		return "R-001"
	}
	defer file.Close()
	maxValue := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var record RequestRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			continue
		}
		if !strings.HasPrefix(record.RequestID, "R-") {
			continue
		}
		value, err := strconv.Atoi(strings.TrimPrefix(record.RequestID, "R-"))
		if err == nil && value > maxValue {
			maxValue = value
		}
	}
	return fmt.Sprintf("R-%03d", maxValue+1)
}

func appendRequest(path string, record RequestRecord) error {
	payload, err := json.Marshal(record)
	if err != nil {
		return err
	}
	handle, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer handle.Close()
	_, err = handle.Write(append(payload, '\n'))
	return err
}

func updateTask(root, taskID string, update func(*adapter.Task)) error {
	pool, err := adapter.LoadTaskPool(root)
	if err != nil {
		return err
	}
	for index := range pool.Tasks {
		if pool.Tasks[index].TaskID != taskID {
			continue
		}
		update(&pool.Tasks[index])
		return adapter.SaveTaskPool(root, pool)
	}
	return fmt.Errorf("task not found: %s", taskID)
}

func updateRuntime(path string, update func(RuntimeState) RuntimeState) error {
	current := RuntimeState{}
	if _, err := state.LoadJSONIfExists(path, &current); err != nil {
		return err
	}
	next := update(current)
	_, err := state.WriteSnapshot(path, &next, "harness-runtime", current.Revision)
	return err
}

func updateVerification(path string, entry VerificationEntry) error {
	current := VerificationSummary{Tasks: map[string]VerificationEntry{}}
	if _, err := state.LoadJSONIfExists(path, &current); err != nil {
		return err
	}
	if current.Tasks == nil {
		current.Tasks = map[string]VerificationEntry{}
	}
	current.Tasks[entry.TaskID] = entry
	_, err := state.WriteSnapshot(path, &current, "harness-runtime", current.Revision)
	return err
}

func defaultOwnedPaths(root string) []string {
	entries, err := os.ReadDir(root)
	if err != nil {
		return []string{"README.md"}
	}
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if name == ".git" || name == ".harness" {
			continue
		}
		if entry.IsDir() {
			paths = append(paths, name+"/**")
		} else {
			paths = append(paths, name)
		}
	}
	if len(paths) == 0 {
		return []string{"README.md"}
	}
	sort.Strings(paths)
	return paths
}

func shortTitle(goal string) string {
	line := strings.TrimSpace(strings.Split(goal, "\n")[0])
	if len(line) > 80 {
		return line[:80]
	}
	return line
}

func inferKind(goal string) string {
	lower := strings.ToLower(goal)
	switch {
	case strings.Contains(lower, "bug"), strings.Contains(lower, "fix"), strings.Contains(lower, "regression"), strings.Contains(lower, "error"), strings.Contains(lower, "failure"):
		return "bug"
	case strings.Contains(lower, "recommend"), strings.Contains(lower, "compare"), strings.Contains(lower, "choose"), strings.Contains(lower, "best way"):
		return "design"
	default:
		return "feature"
	}
}

func uniqueNonEmpty(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func runtimeSummaryVersion(path string) string {
	var payload struct {
		GeneratedAt string `json:"generatedAt"`
		Revision    int64  `json:"revision"`
	}
	if err := state.LoadJSON(path, &payload); err != nil {
		return "runtime:unknown"
	}
	if payload.Revision > 0 {
		return fmt.Sprintf("state.v%d", payload.Revision)
	}
	return "runtime:" + payload.GeneratedAt
}

func resolveCommand(command string, replacements map[string]string) string {
	resolved := command
	for key, value := range replacements {
		resolved = strings.ReplaceAll(resolved, "<"+key+">", value)
	}
	return resolved
}

func deriveVerification(artifactDir, burstStatus, burstSummary string) (string, string, string) {
	verifyPath := filepath.Join(artifactDir, "verify.json")
	switch burstStatus {
	case "failed", "timed_out":
		return "failed", burstSummary, ""
	}
	payload, err := os.ReadFile(verifyPath)
	if err != nil {
		return "passed", "verification succeeded but evidence is missing", ""
	}
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return "passed", "verification completed", verifyPath
	}
	summary := coalesce(stringValue(decoded["summary"]), burstSummary, "verification completed")
	status := strings.ToLower(strings.TrimSpace(stringValue(decoded["status"])))
	overall := strings.ToLower(strings.TrimSpace(stringValue(decoded["overallStatus"])))
	switch {
	case status == "blocked" || overall == "blocked":
		return "blocked", summary, verifyPath
	case status == "failed" || status == "fail" || overall == "failed" || overall == "fail":
		return "failed", summary, verifyPath
	default:
		return "passed", summary, verifyPath
	}
}

func ownedPathViolation(artifactDir string, ownedPaths []string) (string, bool) {
	payload, err := os.ReadFile(filepath.Join(artifactDir, "worker-result.json"))
	if err != nil {
		return "", false
	}
	var decoded struct {
		ChangedPaths []string `json:"changedPaths"`
	}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return "", false
	}
	for _, changedPath := range decoded.ChangedPaths {
		allowed := false
		for _, ownedPath := range ownedPaths {
			if worktree.PathOverlap(ownedPath, changedPath) || worktree.PathOverlap(changedPath, ownedPath) {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Sprintf("changed path outside ownedPaths: %s", changedPath), true
		}
	}
	return "", false
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return fmt.Sprint(typed)
	}
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func coalesce(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func ingestSessionBinding(root string, task adapter.Task, artifactDir string) (string, error) {
	payload, err := os.ReadFile(filepath.Join(artifactDir, "worker-result.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	var decoded struct {
		SessionID       string `json:"sessionId"`
		NativeSessionID string `json:"nativeSessionId"`
	}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return "", nil
	}
	sessionID := coalesce(decoded.NativeSessionID, decoded.SessionID)
	if sessionID == "" {
		return "", nil
	}
	registry, err := adapter.LoadSessionRegistry(root)
	if err != nil {
		return "", err
	}
	now := state.NowUTC()
	registry.Sessions = upsertSession(registry.Sessions, adapter.SessionRecord{
		SessionID:    sessionID,
		SourceTaskID: task.TaskID,
		Model:        task.ExecutionModel,
		Role:         "worker",
		Status:       "active",
		LastUsedAt:   now,
	})
	registry.ActiveBindings = upsertBinding(registry.ActiveBindings, adapter.ActiveBinding{
		TaskID:    task.TaskID,
		SessionID: sessionID,
		NodeID:    "worker-supervisor-node",
	})
	if err := adapter.SaveSessionRegistry(root, registry); err != nil {
		return "", err
	}
	return sessionID, nil
}

func upsertSession(records []adapter.SessionRecord, record adapter.SessionRecord) []adapter.SessionRecord {
	for index := range records {
		if records[index].SessionID == record.SessionID {
			records[index] = record
			return records
		}
	}
	return append(records, record)
}

func upsertBinding(records []adapter.ActiveBinding, record adapter.ActiveBinding) []adapter.ActiveBinding {
	for index := range records {
		if records[index].TaskID == record.TaskID {
			records[index] = record
			return records
		}
	}
	return append(records, record)
}
