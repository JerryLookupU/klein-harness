package runtime

import (
	"fmt"
	"strings"

	"klein-harness/internal/adapter"
	"klein-harness/internal/state"
	"klein-harness/internal/tmux"
	"klein-harness/internal/verify"
)

func FinalizeTaskAfterVerification(root, taskID, dispatchID, verifyStatus, verifySummary, verifyPath, followUp string, verifyErr error) (adapter.Task, error) {
	paths, err := adapter.Resolve(root)
	if err != nil {
		return adapter.Task{}, err
	}
	now := state.NowUTC()
	runtimeFollowUp := followUp
	analysisLoop := shouldEnterAnalysisLoop("", verifyStatus, followUp, verifyErr)
	if analysisLoop {
		runtimeFollowUp = "analysis.required"
	}
	taskStatus := ""
	switch {
	case verifyStatus == "passed" && verifyErr == nil && followUp == "task.completed":
		taskStatus = "completed"
	case analysisLoop:
		taskStatus = "needs_replan"
	case verifyErr != nil:
		taskStatus = "blocked"
	case verifyStatus == "blocked":
		taskStatus = "blocked"
	default:
		taskStatus = "queued"
	}
	verifyCompleted := followUp == "task.completed" && verifyErr == nil
	if err := updateTask(root, taskID, func(current *adapter.Task) {
		current.Status = taskStatus
		current.StatusReason = coalesce(runtimeFollowUp, verifySummary, errorString(verifyErr))
		current.LastDispatchID = coalesce(dispatchID, current.LastDispatchID)
		current.LastLeaseID = ""
		current.VerificationStatus = verifyStatus
		current.VerificationSummary = verifySummary
		current.VerificationResultPath = verifyPath
		if analysisLoop {
			current.PlanEpoch++
			current.PromptStages = analysisPromptStages()
			current.ResumeStrategy = "fresh"
			current.PreferredResumeSessionID = ""
			current.CandidateResumeSessionIDs = nil
			current.CompletedAt = ""
		}
		if verifyCompleted {
			current.CompletedAt = now
		}
		current.UpdatedAt = now
	}); err != nil {
		return adapter.Task{}, err
	}
	if err := updateVerification(paths.VerificationSummaryPath, VerificationEntry{
		TaskID:     taskID,
		DispatchID: dispatchID,
		Status:     verifyStatus,
		Summary:    verifySummary,
		ResultPath: verifyPath,
		UpdatedAt:  now,
		Completed:  verifyCompleted,
		FollowUp:   coalesce(runtimeFollowUp, errorString(verifyErr)),
	}); err != nil {
		return adapter.Task{}, err
	}
	task, err := adapter.LoadTask(root, taskID)
	if err != nil {
		return adapter.Task{}, err
	}
	if err := updateRuntime(paths.RuntimePath, func(current RuntimeState) RuntimeState {
		current.Status = taskStatus
		current = bindRuntimeTask(current, task, adapter.TaskCWD(paths, task))
		current.CurrentDispatchID = coalesce(dispatchID, current.CurrentDispatchID)
		current.LastVerificationStatus = verifyStatus
		current.LastFollowUp = coalesce(runtimeFollowUp, errorString(verifyErr))
		current.LastRunAt = now
		current.LastError = errorString(verifyErr)
		return current
	}); err != nil {
		return adapter.Task{}, err
	}
	if err := refreshExecutionIndexes(paths, task, "", ""); err != nil {
		return adapter.Task{}, err
	}
	return task, nil
}

func RestartFromStage(root, taskID, stage, reason string) (adapter.Task, error) {
	if strings.TrimSpace(stage) == "" {
		stage = "queued"
	}
	paths, err := adapter.Resolve(root)
	if err != nil {
		return adapter.Task{}, err
	}
	task, err := adapter.LoadTask(root, taskID)
	if err != nil {
		return adapter.Task{}, err
	}
	if sessionName := taskTmuxSession(root, task); sessionName != "" {
		_ = tmux.KillSession(sessionName)
	}
	if err := updateTask(root, taskID, func(current *adapter.Task) {
		current.Status = "queued"
		current.StatusReason = coalesce(reason, "restarted from "+stage)
		current.LastLeaseID = ""
		current.VerificationStatus = ""
		current.VerificationSummary = ""
		current.VerificationResultPath = ""
		current.CompletedAt = ""
		current.ArchivedAt = ""
		current.UpdatedAt = state.NowUTC()
	}); err != nil {
		return adapter.Task{}, err
	}
	updated, err := adapter.LoadTask(root, taskID)
	if err != nil {
		return adapter.Task{}, err
	}
	if err := updateRuntime(paths.RuntimePath, func(current RuntimeState) RuntimeState {
		current.Status = "queued"
		current = bindRuntimeTask(current, updated, adapter.TaskCWD(paths, updated))
		current = clearRuntimeExecutionRefs(current)
		current.LastVerificationStatus = ""
		current.LastFollowUp = ""
		current.LastRunAt = state.NowUTC()
		current.LastError = ""
		return current
	}); err != nil {
		return adapter.Task{}, err
	}
	return updated, nil
}

func StopTask(root, taskID, reason string) (adapter.Task, error) {
	paths, err := adapter.Resolve(root)
	if err != nil {
		return adapter.Task{}, err
	}
	task, err := adapter.LoadTask(root, taskID)
	if err != nil {
		return adapter.Task{}, err
	}
	if sessionName := taskTmuxSession(root, task); sessionName != "" {
		_ = tmux.KillSession(sessionName)
	}
	if err := updateTask(root, taskID, func(current *adapter.Task) {
		current.Status = "blocked"
		current.StatusReason = coalesce(reason, "stopped by operator")
		current.LastLeaseID = ""
		current.UpdatedAt = state.NowUTC()
	}); err != nil {
		return adapter.Task{}, err
	}
	updated, err := adapter.LoadTask(root, taskID)
	if err != nil {
		return adapter.Task{}, err
	}
	if err := updateRuntime(paths.RuntimePath, func(current RuntimeState) RuntimeState {
		current.Status = "blocked"
		current = bindRuntimeTask(current, updated, adapter.TaskCWD(paths, updated))
		current.LastFollowUp = "task.blocked"
		current.LastRunAt = state.NowUTC()
		current.LastError = coalesce(reason, "stopped by operator")
		return current
	}); err != nil {
		return adapter.Task{}, err
	}
	return updated, nil
}

func ArchiveTask(root, taskID, reason string) (adapter.Task, error) {
	paths, err := adapter.Resolve(root)
	if err != nil {
		return adapter.Task{}, err
	}
	task, err := adapter.LoadTask(root, taskID)
	if err != nil {
		return adapter.Task{}, err
	}
	var gate verify.CompletionGate
	if ok, err := loadTaskCompletionGate(paths, taskID, &gate); err != nil {
		return adapter.Task{}, err
	} else if !ok || !gate.Satisfied || gate.Retired {
		return adapter.Task{}, fmt.Errorf("%w: task=%s", verify.ErrCompletionGateOpen, taskID)
	}
	var guard verify.GuardState
	_, _ = loadTaskGuardState(paths, taskID, &guard)
	if sessionName := taskTmuxSession(root, task); sessionName != "" {
		_ = tmux.KillSession(sessionName)
	}
	gate.Retired = true
	gate.Status = "retired"
	gate.RetireEligible = false
	taskGateRevision, err := state.CurrentRevision(paths.CompletionGateTaskPath(taskID))
	if err != nil {
		return adapter.Task{}, err
	}
	if _, err := state.WriteSnapshot(paths.CompletionGateTaskPath(taskID), &gate, "harness-control", taskGateRevision); err != nil {
		return adapter.Task{}, err
	}
	aliasGateRevision, err := state.CurrentRevision(paths.CompletionGatePath)
	if err != nil {
		return adapter.Task{}, err
	}
	if _, err := state.WriteSnapshot(paths.CompletionGatePath, &gate, "harness-control", aliasGateRevision); err != nil {
		return adapter.Task{}, err
	}
	guard.Status = "archived"
	guard.TaskID = taskID
	guard.DispatchID = gate.DispatchID
	guard.SafeToArchive = false
	guard.CompletionGateStatus = gate.Status
	guard.CompletionGateSatisfied = gate.Satisfied
	guard.RetireEligible = false
	taskGuardRevision, err := state.CurrentRevision(paths.GuardStateTaskPath(taskID))
	if err != nil {
		return adapter.Task{}, err
	}
	if _, err := state.WriteSnapshot(paths.GuardStateTaskPath(taskID), &guard, "harness-control", taskGuardRevision); err != nil {
		return adapter.Task{}, err
	}
	aliasGuardRevision, err := state.CurrentRevision(paths.GuardStatePath)
	if err != nil {
		return adapter.Task{}, err
	}
	if _, err := state.WriteSnapshot(paths.GuardStatePath, &guard, "harness-control", aliasGuardRevision); err != nil {
		return adapter.Task{}, err
	}
	if err := updateTask(root, taskID, func(current *adapter.Task) {
		current.Status = "archived"
		current.StatusReason = coalesce(reason, "archived")
		current.ArchivedAt = state.NowUTC()
		current.UpdatedAt = state.NowUTC()
	}); err != nil {
		return adapter.Task{}, err
	}
	updated, err := adapter.LoadTask(root, taskID)
	if err != nil {
		return adapter.Task{}, err
	}
	if err := updateRuntime(paths.RuntimePath, func(current RuntimeState) RuntimeState {
		current.Status = "archived"
		current = bindRuntimeTask(current, updated, adapter.TaskCWD(paths, updated))
		current.LastFollowUp = "task.archived"
		current.LastRunAt = state.NowUTC()
		current.LastError = ""
		return current
	}); err != nil {
		return adapter.Task{}, err
	}
	return updated, nil
}

func taskTmuxSession(root string, task adapter.Task) string {
	if task.TmuxSession != "" {
		return task.TmuxSession
	}
	session, ok, err := tmux.FindTaskSession(root, task.TaskID, "")
	if err != nil || !ok {
		return ""
	}
	return session.SessionName
}

func loadTaskCompletionGate(paths adapter.Paths, taskID string, gate *verify.CompletionGate) (bool, error) {
	for _, path := range []string{paths.CompletionGateTaskPath(taskID), paths.CompletionGatePath} {
		ok, err := state.LoadJSONIfExists(path, gate)
		if err != nil {
			return false, err
		}
		if ok && gate.TaskID == taskID {
			return true, nil
		}
	}
	return false, nil
}

func loadTaskGuardState(paths adapter.Paths, taskID string, guard *verify.GuardState) (bool, error) {
	for _, path := range []string{paths.GuardStateTaskPath(taskID), paths.GuardStatePath} {
		ok, err := state.LoadJSONIfExists(path, guard)
		if err != nil {
			return false, err
		}
		if ok && guard.TaskID == taskID {
			return true, nil
		}
	}
	return false, nil
}
