package verify

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"klein-harness/internal/a2a"
	"klein-harness/internal/adapter"
	"klein-harness/internal/dispatch"
	"klein-harness/internal/state"
)

func TestIngestPassedWithoutEvidenceDoesNotComplete(t *testing.T) {
	root := t.TempDir()
	ticket := issueTestDispatch(t, root)

	_, err := Ingest(Request{
		Root:        root,
		TaskID:      "T-1",
		DispatchID:  ticket.DispatchID,
		PlanEpoch:   1,
		Attempt:     1,
		CausationID: "outcome-1",
		Status:      "passed",
		Summary:     "verification says pass",
	})
	if !errors.Is(err, ErrCompletionGateOpen) || !errors.Is(err, ErrVerifiedWithoutEvidence) {
		t.Fatalf("expected completion gate evidence error, got %v", err)
	}

	assertNoEventKind(t, root, "task.completed")
	gate := loadCompletionGate(t, root)
	if gate.Satisfied || gate.Status != "open" {
		t.Fatalf("expected open completion gate, got %+v", gate)
	}
	if check := gate.Checks["verificationEvidence"]; check.OK {
		t.Fatalf("expected verification evidence check to fail: %+v", gate.Checks)
	}
}

func TestIngestReviewRequiredWithoutReviewEvidenceDoesNotComplete(t *testing.T) {
	root := t.TempDir()
	ticket := issueTestDispatch(t, root)
	relVerifyPath := writeVerificationArtifacts(t, root, ticket.DispatchID, false)
	upsertTask(t, root, adapter.Task{
		TaskID:              "T-1",
		ThreadKey:           "thread-1",
		PlanEpoch:           1,
		ReviewRequired:      true,
		VerificationRuleIDs: []string{"VR-1"},
	})

	_, err := Ingest(Request{
		Root:                   root,
		TaskID:                 "T-1",
		DispatchID:             ticket.DispatchID,
		PlanEpoch:              1,
		Attempt:                1,
		CausationID:            "outcome-1",
		Status:                 "passed",
		Summary:                "verification passed but review still required",
		VerificationResultPath: relVerifyPath,
	})
	if !errors.Is(err, ErrCompletionGateOpen) || !errors.Is(err, ErrReviewEvidenceRequired) {
		t.Fatalf("expected review evidence gate error, got %v", err)
	}

	assertNoEventKind(t, root, "task.completed")
	gate := loadCompletionGate(t, root)
	if check := gate.Checks["reviewEvidence"]; check.OK {
		t.Fatalf("expected review evidence check to fail: %+v", gate.Checks)
	}
}

func TestIngestPassedWithEvidenceCompletes(t *testing.T) {
	root := t.TempDir()
	ticket := issueTestDispatch(t, root)
	relVerifyPath := writeVerificationArtifacts(t, root, ticket.DispatchID, false)

	result, err := Ingest(Request{
		Root:                   root,
		TaskID:                 "T-1",
		DispatchID:             ticket.DispatchID,
		PlanEpoch:              1,
		Attempt:                1,
		CausationID:            "outcome-1",
		Status:                 "passed",
		Summary:                "verification passed with concrete evidence",
		VerificationResultPath: relVerifyPath,
	})
	if err != nil {
		t.Fatalf("ingest verification: %v", err)
	}
	if result.FollowUpEvent != "task.completed" {
		t.Fatalf("expected completion follow up, got %+v", result)
	}

	events := loadEvents(t, root)
	if !hasEventKind(events, "task.completed") {
		t.Fatalf("expected task.completed event, got %+v", events)
	}
	gate := loadCompletionGate(t, root)
	if !gate.Satisfied || gate.Status != "satisfied" {
		t.Fatalf("expected satisfied completion gate, got %+v", gate)
	}
	guard := loadGuardState(t, root)
	if guard.Status != "retire_ready" || !guard.SafeToArchive {
		t.Fatalf("expected retire-ready guard state, got %+v", guard)
	}
}

func TestIngestReviewRequiredWithEmbeddedReviewEvidenceCompletes(t *testing.T) {
	root := t.TempDir()
	ticket := issueTestDispatch(t, root)
	relVerifyPath := writeVerificationArtifacts(t, root, ticket.DispatchID, true)
	upsertTask(t, root, adapter.Task{
		TaskID:              "T-1",
		ThreadKey:           "thread-1",
		PlanEpoch:           1,
		ReviewRequired:      true,
		VerificationRuleIDs: []string{"VR-1"},
	})

	result, err := Ingest(Request{
		Root:                   root,
		TaskID:                 "T-1",
		DispatchID:             ticket.DispatchID,
		PlanEpoch:              1,
		Attempt:                1,
		CausationID:            "outcome-1",
		Status:                 "passed",
		Summary:                "verification and review evidence are both present",
		VerificationResultPath: relVerifyPath,
	})
	if err != nil {
		t.Fatalf("ingest verification with review evidence: %v", err)
	}
	if result.FollowUpEvent != "task.completed" {
		t.Fatalf("expected completion follow up, got %+v", result)
	}

	gate := loadCompletionGate(t, root)
	if check := gate.Checks["reviewEvidence"]; !check.OK {
		t.Fatalf("expected review evidence check to pass: %+v", gate.Checks)
	}
}

func TestIngestBlockedStillEmitsBlockedFollowUp(t *testing.T) {
	root := t.TempDir()
	ticket := issueTestDispatch(t, root)

	result, err := Ingest(Request{
		Root:        root,
		TaskID:      "T-1",
		DispatchID:  ticket.DispatchID,
		PlanEpoch:   1,
		Attempt:     1,
		CausationID: "outcome-1",
		Status:      "blocked",
		Summary:     "waiting on missing external credential",
	})
	if err != nil {
		t.Fatalf("ingest blocked verification: %v", err)
	}
	if result.FollowUpEvent != "task.blocked" {
		t.Fatalf("expected blocked follow up, got %+v", result)
	}
	assertNoEventKind(t, root, "task.completed")
}

func TestIngestFailedStillSupportsRCAAndReplan(t *testing.T) {
	root := t.TempDir()
	ticket := issueTestDispatch(t, root)

	result, err := Ingest(Request{
		Root:        root,
		TaskID:      "T-1",
		DispatchID:  ticket.DispatchID,
		PlanEpoch:   1,
		Attempt:     1,
		CausationID: "outcome-1",
		Status:      "failed",
		Summary:     "verification failed after route resume",
		FollowUp:    "rca",
		ReasonCodes: []string{"resume_route_regression"},
	})
	if err != nil {
		t.Fatalf("ingest failed verification with rca: %v", err)
	}
	if result.FollowUpEvent != "rca.allocated" {
		t.Fatalf("expected RCA follow up, got %+v", result)
	}

	root2 := t.TempDir()
	ticket2 := issueTestDispatch(t, root2)
	result2, err := Ingest(Request{
		Root:        root2,
		TaskID:      "T-1",
		DispatchID:  ticket2.DispatchID,
		PlanEpoch:   1,
		Attempt:     1,
		CausationID: "outcome-2",
		Status:      "failed",
		Summary:     "verification failed and needs replan",
	})
	if err != nil {
		t.Fatalf("ingest failed verification with replan: %v", err)
	}
	if result2.FollowUpEvent != "replan.emitted" {
		t.Fatalf("expected replan follow up, got %+v", result2)
	}
	assertNoEventKind(t, root2, "task.completed")
}

func issueTestDispatch(t *testing.T, root string) dispatch.Ticket {
	t.Helper()
	ticket, _, err := dispatch.Issue(dispatch.IssueRequest{
		Root:           root,
		TaskID:         "T-1",
		ThreadKey:      "thread-1",
		PlanEpoch:      1,
		Attempt:        1,
		IdempotencyKey: "dispatch:T-1:1:1",
		CausationID:    "route-1",
		WorkerClass:    "codex-go",
		Cwd:            root,
		Command:        "printf ok",
		PromptRef:      "prompts/worker-burst.md",
	})
	if err != nil {
		t.Fatalf("issue dispatch: %v", err)
	}
	return ticket
}

func writeVerificationArtifacts(t *testing.T, root, dispatchID string, includeReview bool) string {
	t.Helper()
	paths, err := adapter.Resolve(root)
	if err != nil {
		t.Fatalf("resolve paths: %v", err)
	}
	artifactDir := filepath.Join(paths.ArtifactsDir, "T-1", dispatchID)
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		t.Fatalf("mkdir artifact dir: %v", err)
	}
	verifyPath := filepath.Join(artifactDir, "verify.json")
	verifyPayload := `{"overallStatus":"pass","results":[{"ruleId":"VR-1","status":"pass"}]}`
	if includeReview {
		verifyPayload = `{"overallStatus":"pass","results":[{"ruleId":"VR-1","status":"pass"}],"reviewEvidence":[{"kind":"checklist","summary":"reviewed"}]}`
	}
	if err := os.WriteFile(verifyPath, []byte(verifyPayload), 0o644); err != nil {
		t.Fatalf("write verify artifact: %v", err)
	}
	if err := os.WriteFile(filepath.Join(artifactDir, "worker-result.json"), []byte(`{"status":"succeeded","changedPaths":["internal/verify/service.go"]}`), 0o644); err != nil {
		t.Fatalf("write worker-result artifact: %v", err)
	}
	if err := os.WriteFile(filepath.Join(artifactDir, "handoff.md"), []byte("reviewed handoff"), 0o644); err != nil {
		t.Fatalf("write handoff artifact: %v", err)
	}
	return filepath.ToSlash(filepath.Join(".harness", "artifacts", "T-1", dispatchID, "verify.json"))
}

func upsertTask(t *testing.T, root string, task adapter.Task) {
	t.Helper()
	if err := adapter.UpsertTask(root, task); err != nil {
		t.Fatalf("upsert task: %v", err)
	}
}

func loadEvents(t *testing.T, root string) []a2a.Envelope {
	t.Helper()
	paths, err := adapter.Resolve(root)
	if err != nil {
		t.Fatalf("resolve paths: %v", err)
	}
	events, err := a2a.LoadEvents(paths.EventLogPath)
	if err != nil {
		t.Fatalf("load events: %v", err)
	}
	return events
}

func hasEventKind(events []a2a.Envelope, kind string) bool {
	for _, event := range events {
		if event.Kind == kind {
			return true
		}
	}
	return false
}

func assertNoEventKind(t *testing.T, root, kind string) {
	t.Helper()
	if hasEventKind(loadEvents(t, root), kind) {
		t.Fatalf("did not expect %s event", kind)
	}
}

func loadCompletionGate(t *testing.T, root string) CompletionGate {
	t.Helper()
	paths, err := adapter.Resolve(root)
	if err != nil {
		t.Fatalf("resolve paths: %v", err)
	}
	var gate CompletionGate
	if err := state.LoadJSON(paths.CompletionGatePath, &gate); err != nil {
		t.Fatalf("load completion gate: %v", err)
	}
	return gate
}

func loadGuardState(t *testing.T, root string) GuardState {
	t.Helper()
	paths, err := adapter.Resolve(root)
	if err != nil {
		t.Fatalf("resolve paths: %v", err)
	}
	var guard GuardState
	if err := state.LoadJSON(paths.GuardStatePath, &guard); err != nil {
		t.Fatalf("load guard state: %v", err)
	}
	return guard
}
