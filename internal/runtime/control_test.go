package runtime

import (
	"testing"

	"klein-harness/internal/adapter"
	"klein-harness/internal/bootstrap"
	"klein-harness/internal/state"
	"klein-harness/internal/verify"
)

func TestRestartFromStageResetsTaskToQueued(t *testing.T) {
	root := t.TempDir()
	if _, err := bootstrap.Init(root); err != nil {
		t.Fatalf("init: %v", err)
	}
	task := adapter.Task{
		TaskID:                 "T-001",
		ThreadKey:              "R-001",
		Kind:                   "feature",
		Title:                  "test",
		Summary:                "test",
		Status:                 "completed",
		VerificationStatus:     "passed",
		VerificationSummary:    "ok",
		VerificationResultPath: "artifact/verify.json",
		CompletedAt:            state.NowUTC(),
	}
	if err := adapter.UpsertTask(root, task); err != nil {
		t.Fatalf("upsert task: %v", err)
	}
	updated, err := RestartFromStage(root, task.TaskID, "queued", "")
	if err != nil {
		t.Fatalf("restart: %v", err)
	}
	if updated.Status != "queued" {
		t.Fatalf("expected queued status, got %q", updated.Status)
	}
	if updated.VerificationStatus != "" || updated.CompletedAt != "" {
		t.Fatalf("expected verification fields to clear: %#v", updated)
	}
}

func TestArchiveTaskRequiresSatisfiedCompletionGate(t *testing.T) {
	root := t.TempDir()
	paths, err := bootstrap.Init(root)
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	task := adapter.Task{
		TaskID:    "T-001",
		ThreadKey: "R-001",
		Kind:      "feature",
		Title:     "test",
		Summary:   "test",
		Status:    "completed",
	}
	if err := adapter.UpsertTask(root, task); err != nil {
		t.Fatalf("upsert task: %v", err)
	}
	if _, err := ArchiveTask(root, task.TaskID, ""); err == nil {
		t.Fatalf("expected archive to fail without gate")
	}
	gate := verify.CompletionGate{
		Status:     "satisfied",
		Satisfied:  true,
		TaskID:     task.TaskID,
		DispatchID: "dispatch_T_001_1_1",
	}
	gateRevision, err := state.CurrentRevision(paths.CompletionGatePath)
	if err != nil {
		t.Fatalf("gate revision: %v", err)
	}
	if _, err := state.WriteSnapshot(paths.CompletionGatePath, &gate, "test", gateRevision); err != nil {
		t.Fatalf("write gate: %v", err)
	}
	guard := verify.GuardState{
		Status:                  "retire_ready",
		TaskID:                  task.TaskID,
		CompletionGateStatus:    "satisfied",
		CompletionGateSatisfied: true,
		RetireEligible:          true,
		SafeToArchive:           true,
	}
	guardRevision, err := state.CurrentRevision(paths.GuardStatePath)
	if err != nil {
		t.Fatalf("guard revision: %v", err)
	}
	if _, err := state.WriteSnapshot(paths.GuardStatePath, &guard, "test", guardRevision); err != nil {
		t.Fatalf("write guard: %v", err)
	}
	updated, err := ArchiveTask(root, task.TaskID, "")
	if err != nil {
		t.Fatalf("archive: %v", err)
	}
	if updated.Status != "archived" {
		t.Fatalf("expected archived status, got %q", updated.Status)
	}
	var archivedGate verify.CompletionGate
	if err := state.LoadJSON(paths.CompletionGatePath, &archivedGate); err != nil {
		t.Fatalf("load gate: %v", err)
	}
	if !archivedGate.Retired {
		t.Fatalf("expected retired gate: %#v", archivedGate)
	}
}
