package runtime

import (
	"fmt"
	"strings"

	"klein-harness/internal/adapter"
	"klein-harness/internal/state"
	"klein-harness/internal/tmux"
	"klein-harness/internal/verify"
)

func RestartFromStage(root, taskID, stage, reason string) (adapter.Task, error) {
	if strings.TrimSpace(stage) == "" {
		stage = "queued"
	}
	task, err := adapter.LoadTask(root, taskID)
	if err != nil {
		return adapter.Task{}, err
	}
	if task.TmuxSession != "" {
		_ = tmux.KillSession(task.TmuxSession)
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
	return adapter.LoadTask(root, taskID)
}

func StopTask(root, taskID, reason string) (adapter.Task, error) {
	task, err := adapter.LoadTask(root, taskID)
	if err != nil {
		return adapter.Task{}, err
	}
	if task.TmuxSession != "" {
		_ = tmux.KillSession(task.TmuxSession)
	}
	if err := updateTask(root, taskID, func(current *adapter.Task) {
		current.Status = "blocked"
		current.StatusReason = coalesce(reason, "stopped by operator")
		current.LastLeaseID = ""
		current.UpdatedAt = state.NowUTC()
	}); err != nil {
		return adapter.Task{}, err
	}
	return adapter.LoadTask(root, taskID)
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
	if ok, err := state.LoadJSONIfExists(paths.CompletionGatePath, &gate); err != nil {
		return adapter.Task{}, err
	} else if !ok || gate.TaskID != taskID || !gate.Satisfied || gate.Retired {
		return adapter.Task{}, fmt.Errorf("%w: task=%s", verify.ErrCompletionGateOpen, taskID)
	}
	var guard verify.GuardState
	_, _ = state.LoadJSONIfExists(paths.GuardStatePath, &guard)
	if task.TmuxSession != "" {
		_ = tmux.KillSession(task.TmuxSession)
	}
	gate.Retired = true
	gate.Status = "retired"
	gate.RetireEligible = false
	if _, err := state.WriteSnapshot(paths.CompletionGatePath, &gate, "harness-control", gate.Revision); err != nil {
		return adapter.Task{}, err
	}
	guard.Status = "archived"
	guard.TaskID = taskID
	guard.DispatchID = gate.DispatchID
	guard.SafeToArchive = false
	guard.CompletionGateStatus = gate.Status
	guard.CompletionGateSatisfied = gate.Satisfied
	guard.RetireEligible = false
	if _, err := state.WriteSnapshot(paths.GuardStatePath, &guard, "harness-control", guard.Revision); err != nil {
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
	return adapter.LoadTask(root, taskID)
}
