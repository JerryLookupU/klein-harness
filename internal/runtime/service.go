package runtime

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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
	"klein-harness/internal/orchestration"
	"klein-harness/internal/projectspace"
	"klein-harness/internal/route"
	"klein-harness/internal/state"
	"klein-harness/internal/tmux"
	"klein-harness/internal/verify"
	"klein-harness/internal/worker"
	"klein-harness/internal/worktree"
)

type SubmitRequest struct {
	Root           string
	Goal           string
	Kind           string
	Contexts       []string
	ProjectID      string
	ProjectSpaceID string
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

type submitClassification struct {
	FrontDoorTriage       string
	NormalizedIntentClass string
	FusionDecision        string
	TaskFamily            string
	SOPID                 string
	TargetThreadKey       string
	TargetPlanEpoch       int
	IdempotencyKey        string
	CanonicalGoalHash     string
	EvidenceFingerprint   string
	ClassificationReason  string
}

type submissionBinding struct {
	Action string
	Task   adapter.Task
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
	requestID := nextRequestID(paths.QueuePath)
	kind := strings.TrimSpace(request.Kind)
	if kind == "" {
		kind = inferKind(request.Goal)
	}
	classification := classifySubmission(request, pool.Tasks)
	projectID, projectSpaceID, err := projectspace.ResolveProjectSpace(paths.Root, request.ProjectID, request.ProjectSpaceID)
	if err != nil {
		return SubmitResult{}, err
	}
	binding, err := resolveSubmissionBinding(paths.Root, requestID, now, kind, request, pool.Tasks, classification)
	if err != nil {
		return SubmitResult{}, err
	}
	binding.Task.ProjectID = projectID
	binding.Task.ProjectSpaceID = projectSpaceID
	if err := adapter.UpsertTask(paths.Root, binding.Task); err != nil {
		return SubmitResult{}, err
	}
	record := RequestRecord{
		RequestID:             requestID,
		ProjectID:             projectID,
		ProjectSpaceID:        projectSpaceID,
		TaskID:                binding.Task.TaskID,
		BindingAction:         binding.Action,
		ThreadKey:             binding.Task.ThreadKey,
		TargetThreadKey:       firstString(classification.TargetThreadKey, binding.Task.ThreadKey),
		TargetPlanEpoch:       classification.TargetPlanEpoch,
		Kind:                  kind,
		TaskFamily:            classification.TaskFamily,
		SOPID:                 classification.SOPID,
		Goal:                  request.Goal,
		Contexts:              uniqueNonEmpty(request.Contexts),
		Status:                firstString(binding.Task.Status, "queued"),
		FrontDoorTriage:       classification.FrontDoorTriage,
		NormalizedIntentClass: classification.NormalizedIntentClass,
		FusionDecision:        classification.FusionDecision,
		IdempotencyKey:        classification.IdempotencyKey,
		CanonicalGoalHash:     classification.CanonicalGoalHash,
		EvidenceFingerprint:   classification.EvidenceFingerprint,
		ClassificationReason:  classification.ClassificationReason,
		CreatedAt:             now,
		UpdatedAt:             now,
	}
	if classification.FusionDecision == "accepted_existing_thread" {
		record.AppendToTaskID = binding.Task.TaskID
		record.AppendToThreadKey = binding.Task.ThreadKey
	}
	if binding.Action == "reused_existing_task" {
		record.ReusedTaskID = binding.Task.TaskID
	}
	if err := appendRequest(paths.QueuePath, record); err != nil {
		return SubmitResult{}, err
	}
	if err := writeRequestHotState(paths, record, binding.Task); err != nil {
		return SubmitResult{}, err
	}
	if err := updateIntakeState(paths, record, binding.Task); err != nil {
		return SubmitResult{}, err
	}
	if err := updateRuntime(paths.RuntimePath, func(current RuntimeState) RuntimeState {
		current.Status = firstString(binding.Task.Status, "queued")
		current = bindRuntimeTask(current, binding.Task, adapter.TaskCWD(paths, binding.Task))
		current = clearRuntimeExecutionRefs(current)
		current.LastRunAt = now
		current.LastVerificationStatus = binding.Task.VerificationStatus
		current.LastFollowUp = ""
		current.LastError = ""
		return current
	}); err != nil {
		return SubmitResult{}, err
	}
	return SubmitResult{
		Initialized: true,
		Request:     record,
		Task:        binding.Task,
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
			current = clearRuntimeTaskRefs(current)
			current.LastRunAt = state.NowUTC()
			current.LastFollowUp = ""
			current.LastVerificationStatus = ""
			current.LastError = ""
			return current
		}); err != nil {
			return RunResult{}, err
		}
		return RunResult{RuntimeStatus: "idle"}, nil
	}
	task, err = ensureTaskClassification(paths.Root, task)
	if err != nil {
		return RunResult{}, err
	}

	workerID := strings.TrimSpace(options.WorkerID)
	if workerID == "" {
		workerID = "harness-daemon"
	}
	now := state.NowUTC()
	if err := updateTask(paths.Root, task.TaskID, func(current *adapter.Task) {
		current.Status = "routing"
		current.StatusReason = "daemon cycle"
		current.UpdatedAt = now
	}); err != nil {
		return RunResult{}, err
	}
	if err := updateRuntime(paths.RuntimePath, func(current RuntimeState) RuntimeState {
		current.Status = "running"
		current = bindRuntimeTask(current, task, adapter.TaskCWD(paths, task))
		current = clearRuntimeExecutionRefs(current)
		current.LastRunAt = now
		current.LastFollowUp = ""
		current.LastVerificationStatus = ""
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
	routeInput, err := BuildRouteInput(paths.Root, task, latestPlanEpoch, checkpointFresh, sessionContested, runtimeSummaryVersion(paths.RuntimePath))
	if err != nil {
		return RunResult{}, err
	}
	decision := route.Evaluate(routeInput)
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
			current = bindRuntimeTask(current, task, adapter.TaskCWD(paths, task))
			current = clearRuntimeExecutionRefs(current)
			current.LastRunAt = state.NowUTC()
			current.LastFollowUp = ""
			current.LastVerificationStatus = task.VerificationStatus
			current.LastError = ""
			return current
		}); err != nil {
			return RunResult{}, err
		}
		updatedTask, loadErr := adapter.LoadTask(paths.Root, task.TaskID)
		if loadErr != nil {
			return RunResult{}, loadErr
		}
		if err := refreshExecutionIndexes(paths, updatedTask, "", ""); err != nil {
			return RunResult{}, err
		}
		return RunResult{
			RuntimeStatus: status,
			Task:          updatedTask,
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
		PromptRef:              "prompts/spec/apply.md",
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
		current.ExecutionMode = ""
		current.VerificationStatus = ""
		current.VerificationSummary = ""
		current.VerificationResultPath = ""
		current.UpdatedAt = state.NowUTC()
	}); err != nil {
		return RunResult{}, err
	}
	if err := updateRuntime(paths.RuntimePath, func(current RuntimeState) RuntimeState {
		current.Status = "running"
		current = bindRuntimeTask(current, task, adapter.TaskCWD(paths, task))
		current = bindRuntimeDispatch(current, task, ticket, bundle)
		current.LastRunAt = state.NowUTC()
		current.LastFollowUp = ""
		current.LastVerificationStatus = ""
		current.LastError = ""
		return current
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
		CommandBanner:  bundle.CommandBanner,
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
		return handleBurstStartupFailure(paths, task, ticket, leaseRecord, decision, fmt.Errorf("worker startup blocked: %w", err))
	}

	closeoutResult, err := verify.EnsureCloseoutArtifacts(paths.Root, task, ticket, bundle.ArtifactDir, burst.LogPath, burst.DiffStats, burst.Status, burst.Summary)
	if err != nil {
		return RunResult{}, err
	}
	if closeoutResult.Generated {
		burst.Artifacts = append(burst.Artifacts, closeoutResult.GeneratedArtifacts...)
		if burst.Status == "succeeded" {
			burst.Status = closeoutResult.Status
		}
		burst.Summary = coalesce(closeoutResult.Summary, burst.Summary)
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
		Summary:       "daemon cycle checkpoint persisted",
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
	sessionID, err := ingestSessionBinding(paths.Root, task, bundle.ArtifactDir, burst.LogPath)
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

	analysisLoop := shouldEnterAnalysisLoop(burst.Status, verifyStatus, followUp, verifyErr)
	runtimeFollowUp := followUp
	if analysisLoop {
		runtimeFollowUp = "analysis.required"
	}

	taskStatus := burst.Status
	switch {
	case verifyStatus == "passed" && verifyErr == nil && followUp == "task.completed":
		taskStatus = "completed"
	case analysisLoop:
		taskStatus = "needs_replan"
	case verifyErr != nil:
		taskStatus = "blocked"
	case verifyStatus == "blocked":
		taskStatus = "blocked"
	}
	if taskStatus != "completed" && verifyErr == nil {
		sessionRef := sessionID
		if sessionRef == "" {
			sessionRef = decision.ResumeSessionID
		}
		_, _ = verify.RecordOuterLoopMemory(paths.Root, verify.OuterLoopMemoryInput{
			Task:                   task,
			Ticket:                 ticket,
			SessionID:              sessionRef,
			BurstStatus:            burst.Status,
			BurstSummary:           burst.Summary,
			VerifyStatus:           verifyStatus,
			VerifySummary:          verifySummary,
			FollowUp:               runtimeFollowUp,
			VerificationResultPath: verifyPath,
			MissingArtifacts:       closeoutResult.MissingArtifacts,
			EvidenceRefs: uniqueNonEmpty([]string{
				verifyPath,
				burst.LogPath,
				filepath.Join(bundle.ArtifactDir, "worker-result.json"),
				filepath.Join(bundle.ArtifactDir, "handoff.md"),
				checkpointPath,
				outcomePath,
			}),
		})
	}
	verifyCompleted := followUp == "task.completed"
	if err := updateTask(paths.Root, task.TaskID, func(current *adapter.Task) {
		current.Status = taskStatus
		current.StatusReason = coalesce(runtimeFollowUp, burst.Summary)
		current.LastDispatchID = ticket.DispatchID
		current.LastLeaseID = ""
		current.ExecutionMode = coalesce(burst.ExecutionMode, current.ExecutionMode)
		current.VerificationStatus = verifyStatus
		current.VerificationSummary = verifySummary
		current.VerificationResultPath = verifyPath
		if sessionID != "" {
			current.PreferredResumeSessionID = sessionID
			current.CandidateResumeSessionIDs = uniqueNonEmpty(append([]string{sessionID}, current.CandidateResumeSessionIDs...))
			current.ResumeStrategy = "resume"
		}
		if analysisLoop {
			current.PlanEpoch++
			current.PromptStages = analysisPromptStages()
			current.ResumeStrategy = "fresh"
			current.PreferredResumeSessionID = ""
			current.CandidateResumeSessionIDs = nil
			current.CompletedAt = ""
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
		FollowUp:   coalesce(runtimeFollowUp, errorString(verifyErr)),
	}); err != nil {
		return RunResult{}, err
	}
	if err := updateRuntime(paths.RuntimePath, func(current RuntimeState) RuntimeState {
		current.Status = taskStatus
		current = bindRuntimeTask(current, task, adapter.TaskCWD(paths, task))
		current = bindRuntimeDispatch(current, task, ticket, bundle)
		current.LastVerificationStatus = verifyStatus
		current.LastFollowUp = coalesce(runtimeFollowUp, errorString(verifyErr))
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
	if err := refreshExecutionIndexes(paths, finalTask, "", ""); err != nil {
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
		FollowUpEvent: runtimeFollowUp,
	}, nil
}

func handleBurstStartupFailure(paths adapter.Paths, task adapter.Task, ticket dispatch.Ticket, leaseRecord lease.Record, decision route.Decision, burstErr error) (RunResult, error) {
	now := state.NowUTC()
	summary := strings.TrimSpace(errorString(burstErr))
	if summary == "" {
		summary = "worker startup blocked"
	}
	sessionName := ""
	logPath := ""
	if _, err := dispatch.UpdateStatus(paths.Root, ticket.DispatchID, "blocked", "harness-runtime"); err != nil {
		return RunResult{}, err
	}
	if _, err := lease.Release(paths.Root, leaseRecord.LeaseID, ticket.CausationID, []string{"daemon_startup_blocked"}); err != nil {
		return RunResult{}, err
	}
	if session, ok, err := tmux.FindTaskSession(paths.Root, task.TaskID, ""); err != nil {
		return RunResult{}, err
	} else if ok {
		sessionName = session.SessionName
		logPath = session.LogPath
		session.Status = "blocked"
		session.FinishedAt = now
		if err := tmux.UpsertSessionState(paths.Root, session); err != nil {
			return RunResult{}, err
		}
	}
	if err := updateTask(paths.Root, task.TaskID, func(current *adapter.Task) {
		current.Status = "blocked"
		current.StatusReason = summary
		current.LastDispatchID = ticket.DispatchID
		current.LastLeaseID = ""
		current.ExecutionMode = "tmux"
		current.VerificationStatus = "blocked"
		current.VerificationSummary = summary
		current.VerificationResultPath = ""
		current.TmuxSession = coalesce(sessionName, current.TmuxSession)
		current.TmuxLogPath = coalesce(logPath, current.TmuxLogPath)
		current.UpdatedAt = now
	}); err != nil {
		return RunResult{}, err
	}
	if err := updateVerification(paths.VerificationSummaryPath, VerificationEntry{
		TaskID:     task.TaskID,
		DispatchID: ticket.DispatchID,
		Status:     "blocked",
		Summary:    summary,
		ResultPath: "",
		UpdatedAt:  now,
		Completed:  false,
		FollowUp:   "task.blocked",
	}); err != nil {
		return RunResult{}, err
	}
	if err := updateRuntime(paths.RuntimePath, func(current RuntimeState) RuntimeState {
		current.Status = "blocked"
		current = bindRuntimeTask(current, task, adapter.TaskCWD(paths, task))
		current.CurrentDispatchID = ticket.DispatchID
		current.CurrentResumeSessionID = ticket.ResumeSessionID
		current.LastVerificationStatus = "blocked"
		current.LastFollowUp = "task.blocked"
		current.LastRunAt = now
		current.LastError = summary
		return current
	}); err != nil {
		return RunResult{}, err
	}
	finalTask, err := adapter.LoadTask(paths.Root, task.TaskID)
	if err != nil {
		return RunResult{}, err
	}
	if err := refreshExecutionIndexes(paths, finalTask, "", ""); err != nil {
		return RunResult{}, err
	}
	return RunResult{
		RuntimeStatus: "blocked",
		Task:          finalTask,
		Route:         decision,
		Dispatch:      ticket,
		LeaseID:       leaseRecord.LeaseID,
		BurstStatus:   "blocked",
		VerifyStatus:  "blocked",
		FollowUpEvent: "task.blocked",
	}, nil
}

func Loop(root string, interval time.Duration, options RunOptions) error {
	return LoopContext(context.Background(), root, interval, options)
}

func LoopContext(ctx context.Context, root string, interval time.Duration, options RunOptions) error {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-timer.C:
		}
		if _, err := RunOnce(root, options); err != nil {
			return err
		}
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(interval)
	}
}

func classifySubmission(request SubmitRequest, tasks []adapter.Task) submitClassification {
	goal := strings.TrimSpace(request.Goal)
	contexts := uniqueNonEmpty(request.Contexts)
	canonicalGoal := normalizeGoal(goal)
	canonicalGoalHash := hashString(canonicalGoal)
	evidenceFingerprint := hashString(strings.Join(contexts, "\n"))
	frontDoorTriage := frontDoorTriage(goal, contexts)
	family, sopID := orchestration.ClassifyTaskFamily(request.Kind, goal, contexts)
	intentClass := "fresh_work"
	fusionDecision := "accepted_new_thread"
	targetThreadKey := ""
	targetPlanEpoch := 1
	classificationReason := "new goal created a new execution thread"

	if match, ok := latestMatchingTask(tasks, canonicalGoalHash); ok {
		targetThreadKey = match.ThreadKey
		if targetThreadKey == "" {
			targetThreadKey = match.TaskID
		}
		targetPlanEpoch = maxInt(match.PlanEpoch, 1)
		// When reusing the same canonical goal thread, keep inspection/advisory triage semantics
		// (e.g. read-only context enrichment) so route decisions can avoid unnecessary replan.
		if frontDoorTriage != "inspection" && frontDoorTriage != "advisory_read_only" {
			frontDoorTriage = "duplicate_or_context"
		}
		if len(contexts) > 0 {
			intentClass = "context_enrichment"
			fusionDecision = "accepted_existing_thread"
			classificationReason = "same canonical goal with new context was attached to an existing thread"
		} else {
			intentClass = "append_change"
			fusionDecision = "accepted_existing_thread"
			classificationReason = "same canonical goal was bound to an existing thread for progressive execution"
		}
	} else {
		targetThreadKey = ""
	}
	if frontDoorTriage == "inspection" {
		intentClass = "inspection"
		if fusionDecision == "accepted_new_thread" {
			classificationReason = "inspection-like request kept a new thread because no matching execution thread existed"
		}
	}
	return submitClassification{
		FrontDoorTriage:       frontDoorTriage,
		NormalizedIntentClass: intentClass,
		FusionDecision:        fusionDecision,
		TaskFamily:            string(family),
		SOPID:                 sopID,
		TargetThreadKey:       targetThreadKey,
		TargetPlanEpoch:       targetPlanEpoch,
		IdempotencyKey:        "submit:" + canonicalGoalHash + ":" + evidenceFingerprint,
		CanonicalGoalHash:     canonicalGoalHash,
		EvidenceFingerprint:   evidenceFingerprint,
		ClassificationReason:  classificationReason,
	}
}

func updateIntakeState(paths adapter.Paths, record RequestRecord, task adapter.Task) error {
	intakeSummaryPath := filepath.Join(paths.StateDir, "intake-summary.json")
	changeSummaryPath := filepath.Join(paths.StateDir, "change-summary.json")
	threadKey := coalesce(record.TargetThreadKey, record.ThreadKey, task.ThreadKey, task.TaskID)
	if err := refreshThreadState(paths, task, record.RequestID, record.CanonicalGoalHash); err != nil {
		return err
	}
	threadState, err := loadThreadState(filepath.Join(paths.StateDir, "thread-state.json"))
	if err != nil {
		return err
	}

	intakeSummary := IntakeSummary{}
	if _, err := state.LoadJSONIfExists(intakeSummaryPath, &intakeSummary); err != nil {
		return err
	}
	intakeSummary.LatestRequestID = record.RequestID
	intakeSummary.LatestTaskID = task.TaskID
	intakeSummary.LatestThreadKey = threadKey
	intakeSummary.FrontDoorTriage = record.FrontDoorTriage
	intakeSummary.NormalizedIntentClass = record.NormalizedIntentClass
	intakeSummary.FusionDecision = record.FusionDecision
	intakeSummary.TaskFamily = record.TaskFamily
	intakeSummary.SOPID = record.SOPID
	intakeSummary.RequestCount++
	intakeSummary.ActiveThreadCount = len(threadState.Threads)
	if _, err := state.WriteSnapshot(intakeSummaryPath, &intakeSummary, "harness-runtime", intakeSummary.Revision); err != nil {
		return err
	}

	changeSummary := ChangeSummary{}
	if _, err := state.LoadJSONIfExists(changeSummaryPath, &changeSummary); err != nil {
		return err
	}
	changeSummary.LatestRequestID = record.RequestID
	changeSummary.LatestTaskID = task.TaskID
	changeSummary.TargetThreadKey = threadKey
	changeSummary.ChangeKind = record.NormalizedIntentClass
	changeSummary.TaskFamily = record.TaskFamily
	changeSummary.SOPID = record.SOPID
	changeSummary.Summary = record.ClassificationReason
	changeSummary.AffectsExecution = record.FrontDoorTriage != "advisory_read_only"
	if _, err := state.WriteSnapshot(changeSummaryPath, &changeSummary, "harness-runtime", changeSummary.Revision); err != nil {
		return err
	}
	if err := refreshTodoSummary(paths, threadKey, record.RequestID); err != nil {
		return err
	}
	return nil
}

func resolveSubmissionBinding(root, requestID, now, kind string, request SubmitRequest, tasks []adapter.Task, classification submitClassification) (submissionBinding, error) {
	threadKey := firstString(classification.TargetThreadKey, requestID)
	if match, ok := latestMatchingTask(tasks, classification.CanonicalGoalHash); ok && canReuseSubmissionTask(match) {
		match.ThreadKey = firstString(match.ThreadKey, threadKey, match.TaskID)
		match.TaskFamily = firstString(match.TaskFamily, classification.TaskFamily)
		match.SOPID = firstString(match.SOPID, classification.SOPID)
		match.Title = firstString(match.Title, shortTitle(request.Goal))
		if strings.TrimSpace(request.Goal) != "" {
			match.Summary = request.Goal
		}
		match.Description = mergeTaskDescription(match.Description, request.Contexts)
		match.StatusReason = "request reused by submit"
		match.PlanEpoch = maxInt(match.PlanEpoch, classification.TargetPlanEpoch, 1)
		match.UpdatedAt = now
		return submissionBinding{
			Action: "reused_existing_task",
			Task:   match,
		}, nil
	}
	return submissionBinding{
		Action: "created_new_task",
		Task: adapter.Task{
			TaskID:                 nextTaskID(tasks),
			ThreadKey:              threadKey,
			Kind:                   kind,
			TaskFamily:             classification.TaskFamily,
			SOPID:                  classification.SOPID,
			RoleHint:               "worker",
			Title:                  shortTitle(request.Goal),
			Summary:                request.Goal,
			Description:            strings.Join(uniqueNonEmpty(request.Contexts), "\n"),
			WorkerMode:             "execution",
			Status:                 "queued",
			StatusReason:           "submitted",
			PlanEpoch:              maxInt(classification.TargetPlanEpoch, 1),
			OwnedPaths:             defaultOwnedPaths(root),
			ForbiddenPaths:         []string{".git/**", ".harness/**"},
			VerificationRuleIDs:    []string{},
			ResumeStrategy:         "fresh",
			RoutingModel:           "gpt-5.4",
			ExecutionModel:         "gpt-5.3-codex",
			OrchestrationSessionID: "runtime",
			PromptStages:           []string{"route", "dispatch", "execute", "verify"},
			Dispatch: adapter.DispatchProfile{
				WorkspaceRoot: root,
				WorktreePath:  ".",
				BranchName:    "main",
				BaseRef:       "HEAD",
				DiffBase:      "HEAD",
			},
			UpdatedAt: now,
		},
	}, nil
}

func canReuseSubmissionTask(task adapter.Task) bool {
	switch task.Status {
	case "", "queued", "needs_replan", "recoverable":
		return task.LastDispatchID == "" && task.LastLeaseID == "" && task.TmuxSession == ""
	default:
		return false
	}
}

func mergeTaskDescription(existing string, contexts []string) string {
	lines := []string{}
	if strings.TrimSpace(existing) != "" {
		lines = append(lines, strings.Split(existing, "\n")...)
	}
	lines = append(lines, contexts...)
	return strings.Join(uniqueNonEmpty(lines), "\n")
}

func writeRequestHotState(paths adapter.Paths, record RequestRecord, task adapter.Task) error {
	threadKey := firstString(task.ThreadKey, record.TargetThreadKey, record.ThreadKey, task.TaskID)
	requestSummary, err := loadRequestSummary(paths.RequestSummaryPath)
	if err != nil {
		return err
	}
	requestSummary.LatestRequestID = record.RequestID
	requestSummary.LatestTaskID = task.TaskID
	requestSummary.LatestThreadKey = threadKey
	requestSummary.FrontDoorTriage = record.FrontDoorTriage
	requestSummary.NormalizedIntentClass = record.NormalizedIntentClass
	requestSummary.FusionDecision = record.FusionDecision
	requestSummary.TaskFamily = record.TaskFamily
	requestSummary.SOPID = record.SOPID
	requestSummary.BindingAction = record.BindingAction
	requestSummary.TargetPlanEpoch = record.TargetPlanEpoch
	requestSummary.RequestCount++
	if record.BindingAction == "reused_existing_task" {
		requestSummary.ReusedTaskCount++
	} else {
		requestSummary.CreatedTaskCount++
	}
	if _, err := state.WriteSnapshot(paths.RequestSummaryPath, &requestSummary, "harness-runtime", requestSummary.Revision); err != nil {
		return err
	}

	requestIndex, err := loadRequestIndex(paths.RequestIndexPath)
	if err != nil {
		return err
	}
	requestIndex.RequestsByID[record.RequestID] = record
	requestIndex.LatestRequestByTaskID[task.TaskID] = record.RequestID
	requestIndex.LatestRequestByThreadKey[threadKey] = record.RequestID
	if record.IdempotencyKey != "" {
		requestIndex.LatestRequestByIdempotencyKey[record.IdempotencyKey] = record.RequestID
	}
	if _, err := state.WriteSnapshot(paths.RequestIndexPath, &requestIndex, "harness-runtime", requestIndex.Revision); err != nil {
		return err
	}

	requestTaskMap, err := loadRequestTaskMap(paths.RequestTaskMapPath)
	if err != nil {
		return err
	}
	requestTaskMap.RequestToTask[record.RequestID] = task.TaskID
	requestTaskMap.RequestToThread[record.RequestID] = threadKey
	requestTaskMap.TaskToRequests[task.TaskID] = appendUnique(requestTaskMap.TaskToRequests[task.TaskID], record.RequestID)
	requestTaskMap.ThreadToRequests[threadKey] = appendUnique(requestTaskMap.ThreadToRequests[threadKey], record.RequestID)
	requestTaskMap.ThreadToTasks[threadKey] = appendUnique(requestTaskMap.ThreadToTasks[threadKey], task.TaskID)
	if _, err := state.WriteSnapshot(paths.RequestTaskMapPath, &requestTaskMap, "harness-runtime", requestTaskMap.Revision); err != nil {
		return err
	}
	return nil
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

func shouldEnterAnalysisLoop(burstStatus, verifyStatus, followUp string, verifyErr error) bool {
	if verifyErr != nil {
		return false
	}
	switch burstStatus {
	case "failed", "timed_out":
		return true
	}
	switch verifyStatus {
	case "failed":
		return true
	}
	switch followUp {
	case "replan.emitted":
		return true
	}
	return false
}

func ShouldEnterAnalysisLoop(burstStatus, verifyStatus, followUp string, verifyErr error) bool {
	return shouldEnterAnalysisLoop(burstStatus, verifyStatus, followUp, verifyErr)
}

func analysisPromptStages() []string {
	return append([]string{"analysis"}, orchestration.DefaultPromptStages()...)
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

func BuildRouteInput(root string, task adapter.Task, latestPlanEpoch int, checkpointFresh, sessionContested bool, requiredSummaryVersion string) (route.Input, error) {
	paths, err := adapter.Resolve(root)
	if err != nil {
		return route.Input{}, err
	}
	requestRecord, _, err := loadLatestRequestForTask(paths, task.TaskID)
	if err != nil {
		return route.Input{}, err
	}
	todoSummary, _, err := loadTodoSummary(filepath.Join(paths.StateDir, "todo-summary.json"))
	if err != nil {
		return route.Input{}, err
	}
	family, sopID := orchestration.ResolveTaskClassification(task)
	return route.Input{
		TaskID:                    task.TaskID,
		RoleHint:                  task.RoleHint,
		Kind:                      task.Kind,
		TaskFamily:                string(family),
		SOPID:                     sopID,
		Title:                     task.Title,
		Summary:                   strings.TrimSpace(strings.Join([]string{task.Summary, task.Description}, "\n")),
		FrontDoorTriage:           requestRecord.FrontDoorTriage,
		NormalizedIntentClass:     requestRecord.NormalizedIntentClass,
		FusionDecision:            requestRecord.FusionDecision,
		ChangeAffectsExecution:    requestRecord.FrontDoorTriage != "advisory_read_only" && requestRecord.NormalizedIntentClass != "inspection",
		PendingTaskCount:          todoSummary.PendingCount,
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
		RequiredSummaryVersion:    requiredSummaryVersion,
	}, nil
}

func ensureTaskClassification(root string, task adapter.Task) (adapter.Task, error) {
	resolved := orchestration.MaterializeTaskClassification(task)
	if resolved.TaskFamily == task.TaskFamily && resolved.SOPID == task.SOPID {
		return resolved, nil
	}
	now := state.NowUTC()
	if err := updateTask(root, task.TaskID, func(current *adapter.Task) {
		current.TaskFamily = resolved.TaskFamily
		current.SOPID = resolved.SOPID
		current.UpdatedAt = now
	}); err != nil {
		return adapter.Task{}, err
	}
	return adapter.LoadTask(root, task.TaskID)
}

func RefreshExecutionIndexesForTask(root string, task adapter.Task) error {
	paths, err := adapter.Resolve(root)
	if err != nil {
		return err
	}
	return refreshExecutionIndexes(paths, task, "", "")
}

func loadLatestRequestForTask(paths adapter.Paths, taskID string) (RequestRecord, bool, error) {
	requestIndex, err := loadRequestIndex(paths.RequestIndexPath)
	if err != nil {
		return RequestRecord{}, false, err
	}
	if requestID := requestIndex.LatestRequestByTaskID[taskID]; requestID != "" {
		if record, ok := requestIndex.RequestsByID[requestID]; ok {
			return record, true, nil
		}
	}
	return loadLatestRequestForTaskFromQueue(paths.QueuePath, taskID)
}

func loadLatestRequestForTaskFromQueue(queuePath, taskID string) (RequestRecord, bool, error) {
	file, err := os.Open(queuePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return RequestRecord{}, false, nil
		}
		return RequestRecord{}, false, err
	}
	defer file.Close()

	found := false
	latest := RequestRecord{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var record RequestRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			continue
		}
		if record.TaskID != taskID {
			continue
		}
		latest = record
		found = true
	}
	if err := scanner.Err(); err != nil {
		return RequestRecord{}, false, err
	}
	return latest, found, nil
}

func loadRequestSummary(path string) (RequestSummary, error) {
	summary := RequestSummary{}
	if _, err := state.LoadJSONIfExists(path, &summary); err != nil {
		return RequestSummary{}, err
	}
	return summary, nil
}

func loadRequestIndex(path string) (RequestIndex, error) {
	index := RequestIndex{
		RequestsByID:                  map[string]RequestRecord{},
		LatestRequestByTaskID:         map[string]string{},
		LatestRequestByThreadKey:      map[string]string{},
		LatestRequestByIdempotencyKey: map[string]string{},
	}
	if _, err := state.LoadJSONIfExists(path, &index); err != nil {
		return RequestIndex{}, err
	}
	if index.RequestsByID == nil {
		index.RequestsByID = map[string]RequestRecord{}
	}
	if index.LatestRequestByTaskID == nil {
		index.LatestRequestByTaskID = map[string]string{}
	}
	if index.LatestRequestByThreadKey == nil {
		index.LatestRequestByThreadKey = map[string]string{}
	}
	if index.LatestRequestByIdempotencyKey == nil {
		index.LatestRequestByIdempotencyKey = map[string]string{}
	}
	return index, nil
}

func loadRequestTaskMap(path string) (RequestTaskMap, error) {
	mapping := RequestTaskMap{
		RequestToTask:    map[string]string{},
		RequestToThread:  map[string]string{},
		TaskToRequests:   map[string][]string{},
		ThreadToRequests: map[string][]string{},
		ThreadToTasks:    map[string][]string{},
	}
	if _, err := state.LoadJSONIfExists(path, &mapping); err != nil {
		return RequestTaskMap{}, err
	}
	if mapping.RequestToTask == nil {
		mapping.RequestToTask = map[string]string{}
	}
	if mapping.RequestToThread == nil {
		mapping.RequestToThread = map[string]string{}
	}
	if mapping.TaskToRequests == nil {
		mapping.TaskToRequests = map[string][]string{}
	}
	if mapping.ThreadToRequests == nil {
		mapping.ThreadToRequests = map[string][]string{}
	}
	if mapping.ThreadToTasks == nil {
		mapping.ThreadToTasks = map[string][]string{}
	}
	return mapping, nil
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

func bindRuntimeTask(current RuntimeState, task adapter.Task, worktreePath string) RuntimeState {
	current.ActiveTaskID = task.TaskID
	current.ActiveTaskFamily = task.TaskFamily
	current.ActiveSOPID = task.SOPID
	current.ActiveThreadKey = task.ThreadKey
	current.CurrentWorktreePath = worktreePath
	current.CurrentOwnedPaths = uniqueNonEmpty(task.OwnedPaths)
	return current
}

func bindRuntimeDispatch(current RuntimeState, task adapter.Task, ticket dispatch.Ticket, bundle worker.DispatchBundle) RuntimeState {
	current.CurrentDispatchID = ticket.DispatchID
	current.CurrentExecutionSliceID = bundle.ExecutionSliceID
	current.CurrentResumeSessionID = firstString(ticket.ResumeSessionID, task.PreferredResumeSessionID)
	current.CurrentTakeoverPath = bundle.TakeoverPath
	current.CurrentContextLayersPath = bundle.ContextLayersPath
	current.CurrentTaskGraphPath = bundle.TaskGraphPath
	current.CurrentVerifySkeletonPath = bundle.VerifySkeletonPath
	current.CurrentCloseoutPath = bundle.CloseoutPath
	current.CurrentHandoffPath = bundle.HandoffPath
	current.CurrentArtifactDir = bundle.ArtifactDir
	return current
}

func clearRuntimeExecutionRefs(current RuntimeState) RuntimeState {
	current.CurrentDispatchID = ""
	current.CurrentExecutionSliceID = ""
	current.CurrentResumeSessionID = ""
	current.CurrentTakeoverPath = ""
	current.CurrentContextLayersPath = ""
	current.CurrentTaskGraphPath = ""
	current.CurrentVerifySkeletonPath = ""
	current.CurrentCloseoutPath = ""
	current.CurrentHandoffPath = ""
	current.CurrentArtifactDir = ""
	return current
}

func clearRuntimeTaskRefs(current RuntimeState) RuntimeState {
	current.ActiveTaskID = ""
	current.ActiveTaskFamily = ""
	current.ActiveSOPID = ""
	current.ActiveThreadKey = ""
	current.CurrentWorktreePath = ""
	current.CurrentOwnedPaths = nil
	return clearRuntimeExecutionRefs(current)
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

func normalizeGoal(goal string) string {
	goal = strings.ToLower(strings.TrimSpace(goal))
	space := regexp.MustCompile(`\s+`)
	return space.ReplaceAllString(goal, " ")
}

func hashString(value string) string {
	sum := sha256.Sum256([]byte(value))
	return fmt.Sprintf("%x", sum[:8])
}

func latestMatchingTask(tasks []adapter.Task, canonicalGoalHash string) (adapter.Task, bool) {
	for index := len(tasks) - 1; index >= 0; index-- {
		task := tasks[index]
		if hashString(normalizeGoal(task.Summary)) != canonicalGoalHash {
			continue
		}
		return task, true
	}
	return adapter.Task{}, false
}

func frontDoorTriage(goal string, contexts []string) string {
	signal := strings.ToLower(strings.TrimSpace(goal))
	switch {
	case matchesSignal(signal, "inspect", "status", "show", "list", "what is", "what's", "read only"):
		return "inspection"
	case matchesSignal(signal, "advice", "recommendation", "compare options", "trade-off", "tradeoff"):
		return "advisory_read_only"
	case len(contexts) > 0:
		// Context can carry read-only / display-only semantics even when the goal itself is a work-order.
		// This helps incremental "append requirements" avoid forcing a full replan when changes only affect operator surface.
		contextSignal := strings.ToLower(strings.Join(contexts, "\n"))
		switch {
		case matchesSignal(contextSignal, "inspect", "status", "show", "list", "what is", "what's", "read only"):
			return "inspection"
		case matchesSignal(contextSignal, "advice", "recommendation", "展示", "显示", "只做读面", "读面", "分析", "不修改", "不改", "无需修改", "不需要修改"):
			return "advisory_read_only"
		default:
			return "duplicate_or_context"
		}
	default:
		return "work_order"
	}
}

func matchesSignal(signal string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(signal, needle) {
			return true
		}
	}
	return false
}

func appendUnique(values []string, value string) []string {
	for _, item := range values {
		if item == value {
			return values
		}
	}
	return append(values, value)
}

func firstString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func maxInt(values ...int) int {
	maxValue := 0
	for _, value := range values {
		if value > maxValue {
			maxValue = value
		}
	}
	return maxValue
}

func refreshExecutionIndexes(paths adapter.Paths, task adapter.Task, latestRequestID, canonicalGoalHash string) error {
	if err := refreshThreadState(paths, task, latestRequestID, canonicalGoalHash); err != nil {
		return err
	}
	if err := refreshTodoSummary(paths, task.ThreadKey, latestRequestID); err != nil {
		return err
	}
	return nil
}

func refreshThreadState(paths adapter.Paths, task adapter.Task, latestRequestID, canonicalGoalHash string) error {
	threadKey := firstString(task.ThreadKey, task.TaskID)
	if threadKey == "" {
		return nil
	}
	threadStatePath := filepath.Join(paths.StateDir, "thread-state.json")
	threadState, err := loadThreadState(threadStatePath)
	if err != nil {
		return err
	}
	threadEntry := threadState.Threads[threadKey]
	threadEntry.ThreadKey = threadKey
	threadEntry.ProjectID = firstString(task.ProjectID, threadEntry.ProjectID)
	threadEntry.ProjectSpaceID = firstString(task.ProjectSpaceID, threadEntry.ProjectSpaceID)
	if canonicalGoalHash != "" {
		threadEntry.CanonicalGoalHash = canonicalGoalHash
	}
	if latestRequestID != "" {
		threadEntry.LatestRequestID = latestRequestID
		threadEntry.RequestIDs = appendUnique(threadEntry.RequestIDs, latestRequestID)
	}
	threadEntry.LatestTaskID = task.TaskID
	threadEntry.CurrentPlanEpoch = maxInt(threadEntry.CurrentPlanEpoch, threadEntry.PlanEpoch, task.PlanEpoch)
	threadEntry.PlanEpoch = threadEntry.CurrentPlanEpoch
	threadEntry.LatestValidPlanEpoch = maxInt(threadEntry.LatestValidPlanEpoch, threadEntry.CurrentPlanEpoch)
	threadEntry.TaskIDs = appendUnique(threadEntry.TaskIDs, task.TaskID)
	threadEntry.Status = task.Status
	threadEntry.UpdatedAt = task.UpdatedAt
	threadState.Threads[threadKey] = threadEntry
	_, err = state.WriteSnapshot(threadStatePath, &threadState, "harness-runtime", threadState.Revision)
	return err
}

func loadThreadState(path string) (ThreadState, error) {
	threadState := ThreadState{Threads: map[string]ThreadEntry{}}
	if _, err := state.LoadJSONIfExists(path, &threadState); err != nil {
		return ThreadState{}, err
	}
	if threadState.Threads == nil {
		threadState.Threads = map[string]ThreadEntry{}
	}
	return threadState, nil
}

func loadTodoSummary(path string) (TodoSummary, bool, error) {
	todoSummary := TodoSummary{}
	ok, err := state.LoadJSONIfExists(path, &todoSummary)
	if err != nil {
		return TodoSummary{}, false, err
	}
	return todoSummary, ok, nil
}

func refreshTodoSummary(paths adapter.Paths, activeThreadKey, latestRequestID string) error {
	todoSummaryPath := filepath.Join(paths.StateDir, "todo-summary.json")
	pool, err := adapter.LoadTaskPool(paths.Root)
	if err != nil {
		return err
	}
	todoIDs := make([]string, 0)
	for _, item := range pool.Tasks {
		switch item.Status {
		case "", "queued", "needs_replan", "recoverable", "routing", "running":
			todoIDs = append(todoIDs, item.TaskID)
		}
	}
	todoSummary, _, err := loadTodoSummary(todoSummaryPath)
	if err != nil {
		return err
	}
	todoSummary.NextTaskID = firstString(todoIDs...)
	todoSummary.TaskIDs = todoIDs
	todoSummary.PendingCount = len(todoIDs)
	if activeThreadKey != "" {
		todoSummary.ActiveThreadKey = activeThreadKey
	}
	if latestRequestID != "" {
		todoSummary.LatestRequestID = latestRequestID
	}
	_, err = state.WriteSnapshot(todoSummaryPath, &todoSummary, "harness-runtime", todoSummary.Revision)
	return err
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
		return "failed", "verification artifact is missing", ""
	}
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return "failed", "verification artifact is invalid JSON", verifyPath
	}
	if !verifyArtifactHasSignal(decoded) {
		return "failed", "verification artifact is empty or lacks usable evidence", verifyPath
	}
	summary := coalesce(stringValue(decoded["overallSummary"]), stringValue(decoded["summary"]), burstSummary, "verification completed")
	status := strings.ToLower(strings.TrimSpace(stringValue(decoded["status"])))
	overall := strings.ToLower(strings.TrimSpace(stringValue(decoded["overallStatus"])))
	result := strings.ToLower(strings.TrimSpace(stringValue(decoded["result"])))
	switch {
	case status == "blocked" || overall == "blocked" || result == "blocked":
		return "blocked", summary, verifyPath
	case status == "failed" || status == "fail" || overall == "failed" || overall == "fail" || result == "failed" || result == "fail":
		return "failed", summary, verifyPath
	default:
		return "passed", summary, verifyPath
	}
}

func DeriveVerification(artifactDir, burstStatus, burstSummary string) (string, string, string) {
	return deriveVerification(artifactDir, burstStatus, burstSummary)
}

func verifyArtifactHasSignal(decoded map[string]any) bool {
	if len(decoded) == 0 {
		return false
	}
	for _, key := range []string{"overallStatus", "status", "result", "overallSummary", "summary"} {
		if strings.TrimSpace(stringValue(decoded[key])) != "" {
			return true
		}
	}
	for _, key := range []string{"scorecard", "evidenceLedger", "findings", "reviewChecklist", "commands"} {
		switch value := decoded[key].(type) {
		case []any:
			if len(value) > 0 {
				return true
			}
		case map[string]any:
			if len(value) > 0 {
				return true
			}
		}
	}
	return false
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
	case nil:
		return ""
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

func ingestSessionBinding(root string, task adapter.Task, artifactDir, tmuxLogPath string) (string, error) {
	sessionID := detectSessionID(
		filepath.Join(artifactDir, "worker-result.json"),
		filepath.Join(artifactDir, "last-message.txt"),
		tmuxLogPath,
	)
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

var sessionIDPatterns = []*regexp.Regexp{
	regexp.MustCompile(`"nativeSessionId"\s*:\s*"([^"]+)"`),
	regexp.MustCompile(`"sessionId"\s*:\s*"([^"]+)"`),
	regexp.MustCompile(`"threadId"\s*:\s*"([^"]+)"`),
	regexp.MustCompile(`"thread_id"\s*:\s*"([^"]+)"`),
}

func detectSessionID(paths ...string) string {
	for _, path := range paths {
		if strings.TrimSpace(path) == "" {
			continue
		}
		payload, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			continue
		}
		for _, pattern := range sessionIDPatterns {
			matches := pattern.FindAllSubmatch(payload, -1)
			if len(matches) == 0 {
				continue
			}
			value := strings.TrimSpace(string(matches[len(matches)-1][1]))
			if value != "" {
				return value
			}
		}
	}
	return ""
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
