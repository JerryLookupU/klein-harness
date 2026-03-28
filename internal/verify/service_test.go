package verify

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"klein-harness/internal/a2a"
	"klein-harness/internal/adapter"
	"klein-harness/internal/dispatch"
	"klein-harness/internal/orchestration"
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
	if gate.Satisfied || gate.Status != "needs_replan" || gate.RecommendedNextAction != "replan" {
		t.Fatalf("expected needs_replan completion gate, got %+v", gate)
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
	if gate.Status != "needs_review" || gate.RecommendedNextAction != "review" {
		t.Fatalf("expected needs_review gate, got %+v", gate)
	}
	if check := gate.Checks["reviewEvidence"]; check.OK {
		t.Fatalf("expected review evidence check to fail: %+v", gate.Checks)
	}
}

func TestIngestPassedWithEvidenceCompletes(t *testing.T) {
	root := t.TempDir()
	ticket := issueTestDispatch(t, root)
	relVerifyPath := writeVerificationArtifacts(t, root, ticket.DispatchID, false)
	writeSharedConstraintSnapshot(t, root, "T-1", ticket.DispatchID, 1)
	writeAcceptedPacketAndContract(t, root, ticket)

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
	if check := gate.Checks["acceptedPacket"]; !check.OK {
		t.Fatalf("expected accepted packet check to pass, got %+v", gate.Checks)
	}
	if check := gate.Checks["taskContract"]; !check.OK {
		t.Fatalf("expected task contract check to pass, got %+v", gate.Checks)
	}
	if check := gate.Checks["verificationScorecard"]; !check.OK {
		t.Fatalf("expected verification scorecard check to pass, got %+v", gate.Checks)
	}
	if check := gate.Checks["hardConstraints"]; !check.OK {
		t.Fatalf("expected hard constraints check to pass, got %+v", gate)
	}
	if gate.RecommendedNextAction != "archive" {
		t.Fatalf("expected archive next action, got %+v", gate)
	}
	if len(gate.HardConstraintChecks) == 0 {
		t.Fatalf("expected itemized hard constraint checks, got %+v", gate)
	}
	guard := loadGuardState(t, root)
	if guard.Status != "retire_ready" || !guard.SafeToArchive {
		t.Fatalf("expected retire-ready guard state, got %+v", guard)
	}
}

func TestIngestWritesTaskScopedGateAndAlias(t *testing.T) {
	root := t.TempDir()
	ticket := issueTestDispatch(t, root)
	relVerifyPath := writeVerificationArtifacts(t, root, ticket.DispatchID, false)
	writeSharedConstraintSnapshot(t, root, "T-1", ticket.DispatchID, 1)
	writeAcceptedPacketAndContract(t, root, ticket)

	if _, err := Ingest(Request{
		Root:                   root,
		TaskID:                 "T-1",
		DispatchID:             ticket.DispatchID,
		PlanEpoch:              1,
		Attempt:                1,
		CausationID:            "outcome-1",
		Status:                 "passed",
		Summary:                "verification passed with task-scoped state",
		VerificationResultPath: relVerifyPath,
	}); err != nil {
		t.Fatalf("ingest verification: %v", err)
	}

	taskGate := loadTaskCompletionGate(t, root, "T-1")
	if !taskGate.Satisfied || taskGate.TaskID != "T-1" {
		t.Fatalf("expected task-scoped completion gate: %+v", taskGate)
	}
	aliasGate := loadCompletionGate(t, root)
	if aliasGate.TaskID != "T-1" || aliasGate.Status != taskGate.Status {
		t.Fatalf("expected singleton alias to mirror task-scoped completion gate: %+v %+v", aliasGate, taskGate)
	}

	taskGuard := loadTaskGuardState(t, root, "T-1")
	if taskGuard.TaskID != "T-1" || !taskGuard.SafeToArchive {
		t.Fatalf("expected task-scoped guard state: %+v", taskGuard)
	}
	aliasGuard := loadGuardState(t, root)
	if aliasGuard.TaskID != "T-1" || aliasGuard.Status != taskGuard.Status {
		t.Fatalf("expected singleton alias to mirror task-scoped guard state: %+v %+v", aliasGuard, taskGuard)
	}
}

func TestIngestReviewRequiredWithEmbeddedReviewEvidenceCompletes(t *testing.T) {
	root := t.TempDir()
	ticket := issueTestDispatch(t, root)
	relVerifyPath := writeVerificationArtifacts(t, root, ticket.DispatchID, true)
	writeAcceptedPacketAndContract(t, root, ticket)
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

func TestIngestPassedWithoutTaskContractDoesNotComplete(t *testing.T) {
	root := t.TempDir()
	ticket := issueTestDispatch(t, root)
	relVerifyPath := writeVerificationArtifacts(t, root, ticket.DispatchID, false)
	writeSharedConstraintSnapshot(t, root, "T-1", ticket.DispatchID, 1)
	writeAcceptedPacketOnly(t, root, ticket)

	_, err := Ingest(Request{
		Root:                   root,
		TaskID:                 "T-1",
		DispatchID:             ticket.DispatchID,
		PlanEpoch:              1,
		Attempt:                1,
		CausationID:            "outcome-1",
		Status:                 "passed",
		Summary:                "verification passed with evidence but no task contract",
		VerificationResultPath: relVerifyPath,
	})
	if !errors.Is(err, ErrCompletionGateOpen) || !errors.Is(err, ErrTaskContractRequired) {
		t.Fatalf("expected missing task contract gate error, got %v", err)
	}

	gate := loadCompletionGate(t, root)
	if check := gate.Checks["taskContract"]; check.OK {
		t.Fatalf("expected task contract check to fail: %+v", gate.Checks)
	}
}

func TestIngestPassedWithRemainingExecutionSlicesEmitsReplan(t *testing.T) {
	root := t.TempDir()
	ticket := issueTestDispatch(t, root)
	relVerifyPath := writeVerificationArtifacts(t, root, ticket.DispatchID, false)
	writeSharedConstraintSnapshot(t, root, "T-1", ticket.DispatchID, 1)
	writeAcceptedPacketWithMultipleSlices(t, root)
	writeTaskContractForSlice(t, root, ticket, "T-1.slice.1")

	result, err := Ingest(Request{
		Root:                   root,
		TaskID:                 "T-1",
		DispatchID:             ticket.DispatchID,
		PlanEpoch:              1,
		Attempt:                1,
		CausationID:            "outcome-1",
		Status:                 "passed",
		Summary:                "first execution slice verified",
		VerificationResultPath: relVerifyPath,
	})
	if err != nil {
		t.Fatalf("expected remaining slices to emit follow-up instead of error, got %v", err)
	}
	if result.FollowUpEvent != "replan.emitted" {
		t.Fatalf("expected replan follow-up, got %+v", result)
	}
	gate := loadCompletionGate(t, root)
	if gate.Status != "needs_replan" || gate.RecommendedNextAction != "replan" {
		t.Fatalf("expected needs_replan gate, got %+v", gate)
	}
	if check := gate.Checks["executionTasks"]; check.OK {
		t.Fatalf("expected execution tasks check to remain open: %+v", gate.Checks)
	}
}

func TestIngestPassedWithPendingOrchestrationExpansionEmitsReplan(t *testing.T) {
	root := t.TempDir()
	ticket := issueTestDispatch(t, root)
	relVerifyPath := writeVerificationArtifacts(t, root, ticket.DispatchID, false)
	writeSharedConstraintSnapshot(t, root, "T-1", ticket.DispatchID, 1)
	writeAcceptedPacketOnly(t, root, ticket)
	writeTaskContractForSlice(t, root, ticket, "T-1.slice.1")

	packet, err := orchestration.LoadAcceptedPacket(orchestration.AcceptedPacketPath(root, "T-1"))
	if err != nil {
		t.Fatalf("load accepted packet: %v", err)
	}
	packet.OrchestrationExpansionPending = true
	packet.OrchestrationExpansionReason = "roster_freeze_required_before_atomic_fanout"
	packet.OrchestrationExpansionSource = "output/linguists.roster.md"
	if err := orchestration.WriteAcceptedPacket(orchestration.AcceptedPacketPath(root, "T-1"), packet); err != nil {
		t.Fatalf("rewrite accepted packet: %v", err)
	}

	result, err := Ingest(Request{
		Root:                   root,
		TaskID:                 "T-1",
		DispatchID:             ticket.DispatchID,
		PlanEpoch:              1,
		Attempt:                1,
		CausationID:            "outcome-1",
		Status:                 "passed",
		Summary:                "roster freeze verified",
		VerificationResultPath: relVerifyPath,
	})
	if err != nil {
		t.Fatalf("expected orchestration expansion to emit replan instead of error, got %v", err)
	}
	if result.FollowUpEvent != "replan.emitted" {
		t.Fatalf("expected orchestration expansion replan follow-up, got %+v", result)
	}
	gate := loadCompletionGate(t, root)
	if gate.Status != "needs_replan" || gate.RecommendedNextAction != "replan" {
		t.Fatalf("expected needs_replan gate, got %+v", gate)
	}
	if check := gate.Checks["orchestrationExpansion"]; check.OK {
		t.Fatalf("expected orchestration expansion check to remain open: %+v", gate.Checks)
	}
}

func TestIngestPassedWithBlockingFindingsDoesNotComplete(t *testing.T) {
	root := t.TempDir()
	ticket := issueTestDispatch(t, root)
	relVerifyPath := writeVerificationPayload(t, root, ticket.DispatchID, `{
  "overallStatus": "pass",
  "results": [{"ruleId":"VR-1","status":"pass"}],
  "findings": [{"severity":"critical","summary":"required contract evidence is inconsistent"}],
  "evidenceRefs": [".harness/artifacts/T-1/`+ticket.DispatchID+`/worker-result.json"]
}`)
	writeSharedConstraintSnapshot(t, root, "T-1", ticket.DispatchID, 1)
	writeAcceptedPacketAndContract(t, root, ticket)

	_, err := Ingest(Request{
		Root:                   root,
		TaskID:                 "T-1",
		DispatchID:             ticket.DispatchID,
		PlanEpoch:              1,
		Attempt:                1,
		CausationID:            "outcome-1",
		Status:                 "passed",
		Summary:                "verification passed but critical findings remain",
		VerificationResultPath: relVerifyPath,
	})
	if !errors.Is(err, ErrCompletionGateOpen) || !errors.Is(err, ErrBlockingVerificationFindings) {
		t.Fatalf("expected blocking findings gate error, got %v", err)
	}

	gate := loadCompletionGate(t, root)
	if gate.Status != "needs_replan" || gate.RecommendedNextAction != "repair" {
		t.Fatalf("expected repair-oriented needs_replan gate, got %+v", gate)
	}
	if check := gate.Checks["blockingFindings"]; check.OK {
		t.Fatalf("expected blocking findings check to fail: %+v", gate.Checks)
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
	verifyPayload := `{"overallStatus":"pass","results":[{"ruleId":"VR-1","status":"pass"}]}`
	if includeReview {
		verifyPayload = `{"overallStatus":"pass","results":[{"ruleId":"VR-1","status":"pass"}],"reviewEvidence":[{"kind":"checklist","summary":"reviewed"}]}`
	}
	return writeVerificationPayload(t, root, dispatchID, verifyPayload)
}

func writeVerificationPayload(t *testing.T, root, dispatchID, verifyPayload string) string {
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

func writeSharedConstraintSnapshot(t *testing.T, root, taskID, dispatchID string, planEpoch int) {
	t.Helper()
	system := orchestration.DefaultConstraintSystem(root, []string{"dispatch_ready"})
	softRules, hardRules := orchestration.SplitConstraintRules(system)
	if err := orchestration.WriteConstraintSnapshot(orchestration.ConstraintSnapshotPath(root, taskID), orchestration.ConstraintSnapshot{
		SchemaVersion:    "kh.constraint-snapshot.v1",
		Generator:        "test",
		GeneratedAt:      "2026-03-26T10:00:00Z",
		TaskID:           taskID,
		DispatchID:       dispatchID,
		PlanEpoch:        planEpoch,
		ConstraintSystem: system,
		SoftRules:        softRules,
		HardRules:        hardRules,
	}); err != nil {
		t.Fatalf("write shared constraint snapshot: %v", err)
	}
}

func writeAcceptedPacketAndContract(t *testing.T, root string, ticket dispatch.Ticket) {
	t.Helper()
	writeAcceptedPacketOnly(t, root, ticket)
	writeTaskContractForSlice(t, root, ticket, "T-1.slice.1")
}

func writeTaskContractForSlice(t *testing.T, root string, ticket dispatch.Ticket, sliceID string) {
	t.Helper()
	artifactDir := filepath.Join(root, ".harness", "artifacts", "T-1", ticket.DispatchID)
	path := orchestration.TaskContractPath(artifactDir)
	if err := orchestration.WriteTaskContract(path, orchestration.TaskContract{
		SchemaVersion:     "kh.task-contract.v1",
		Generator:         "test",
		GeneratedAt:       "2026-03-26T10:00:00Z",
		ContractID:        "contract_T-1_1_1",
		TaskID:            "T-1",
		DispatchID:        ticket.DispatchID,
		ThreadKey:         "thread-1",
		PlanEpoch:         1,
		ExecutionSliceID:  sliceID,
		Objective:         "verify completion path",
		InScope:           []string{"internal/verify/**"},
		OutOfScope:        []string{".harness/**"},
		DoneCriteria:      []string{"verify evidence recorded", "closeout artifacts written"},
		AcceptanceMarkers: []string{"VR-1"},
		VerificationChecklist: []orchestration.VerificationChecklistItem{
			{ID: "vr-1", Title: "Go verify rule", Required: true, Status: "required", Detail: "run VR-1 evidence"},
			{ID: "closeout_artifacts", Title: "closeout artifacts exist", Required: true, Status: "required", Detail: "verify.json, worker-result.json, handoff.md"},
		},
		RequiredEvidence:   []string{"verify.json", "worker-result.json", "handoff.md"},
		ReviewRequired:     false,
		ContractStatus:     "accepted",
		ProposedBy:         "test",
		AcceptedBy:         "test",
		AcceptedAt:         "2026-03-26T10:00:00Z",
		AcceptedPacketPath: orchestration.AcceptedPacketPath(root, "T-1"),
		SharedFlowContextPath: filepath.Join(artifactDir, "shared-flow-context.json"),
		TaskGraphPath:         filepath.Join(artifactDir, "task-graph.json"),
		SliceContextPath:      filepath.Join(artifactDir, "slice-context.json"),
		ContextLayersPath:     filepath.Join(artifactDir, "context-layers.json"),
		VerifySkeletonPath:    filepath.Join(artifactDir, "verify-skeleton.json"),
		CloseoutSkeletonPath:  filepath.Join(artifactDir, "closeout-skeleton.json"),
		HandoffContractPath:   filepath.Join(artifactDir, "handoff-contract.json"),
		TakeoverPath:          filepath.Join(artifactDir, "takeover-context.json"),
	}); err != nil {
		t.Fatalf("write task contract: %v", err)
	}
}

func writeAcceptedPacketOnly(t *testing.T, root string, ticket dispatch.Ticket) {
	t.Helper()
	if err := orchestration.WriteAcceptedPacket(orchestration.AcceptedPacketPath(root, "T-1"), orchestration.AcceptedPacket{
		SchemaVersion:     "kh.accepted-packet.v1",
		Generator:         "test",
		GeneratedAt:       "2026-03-26T10:00:00Z",
		TaskID:            "T-1",
		ThreadKey:         "thread-1",
		PlanEpoch:         1,
		PacketID:          "packet_T-1_1",
		Objective:         "verify completion path",
		Constraints:       []string{"stay within task-local scope"},
		FlowSelection:     "standard bounded delivery",
		SelectedPlan:      "execute one bounded slice and verify",
		ExecutionTasks:    []orchestration.ExecutionTask{{ID: "T-1.slice.1", Title: "slice", Summary: "summary"}},
		VerificationPlan:  map[string]any{"ruleIds": []string{"VR-1"}},
		DecisionRationale: "test accepted packet",
		OwnedPaths:        []string{"internal/verify/**"},
		TaskBudgets:       map[string]any{"dispatchId": ticket.DispatchID},
		AcceptanceMarkers: []string{"VR-1"},
		ReplanTriggers:    []string{"verification_failed"},
		RollbackHints:     []string{"preserve checkpoint"},
		AcceptedAt:        "2026-03-26T10:00:00Z",
		AcceptedBy:        "test",
	}); err != nil {
		t.Fatalf("write accepted packet: %v", err)
	}
}

func writeAcceptedPacketWithMultipleSlices(t *testing.T, root string) {
	t.Helper()
	if err := orchestration.WriteAcceptedPacket(orchestration.AcceptedPacketPath(root, "T-1"), orchestration.AcceptedPacket{
		SchemaVersion: "kh.accepted-packet.v1",
		Generator:     "test",
		GeneratedAt:   "2026-03-26T10:00:00Z",
		TaskID:        "T-1",
		ThreadKey:     "thread-1",
		PlanEpoch:     1,
		PacketID:      "packet_T-1_1",
		Objective:     "verify multi-slice progress",
		Constraints:   []string{"stay within task-local scope"},
		FlowSelection: "standard bounded delivery",
		SelectedPlan:  "execute three narrow slices",
		ExecutionTasks: []orchestration.ExecutionTask{
			{ID: "T-1.slice.1", Title: "slice 1", Summary: "one"},
			{ID: "T-1.slice.2", Title: "slice 2", Summary: "two"},
			{ID: "T-1.slice.3", Title: "slice 3", Summary: "three"},
		},
		VerificationPlan:  map[string]any{"ruleIds": []string{"VR-1"}},
		DecisionRationale: "test accepted packet with multiple slices",
		OwnedPaths:        []string{"internal/verify/**"},
		TaskBudgets:       map[string]any{},
		AcceptanceMarkers: []string{"VR-1"},
		ReplanTriggers:    []string{"verification_failed"},
		RollbackHints:     []string{"preserve checkpoint"},
		AcceptedAt:        "2026-03-26T10:00:00Z",
		AcceptedBy:        "test",
	}); err != nil {
		t.Fatalf("write accepted packet with multiple slices: %v", err)
	}
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

func loadTaskCompletionGate(t *testing.T, root, taskID string) CompletionGate {
	t.Helper()
	paths, err := adapter.Resolve(root)
	if err != nil {
		t.Fatalf("resolve paths: %v", err)
	}
	var gate CompletionGate
	if err := state.LoadJSON(paths.CompletionGateTaskPath(taskID), &gate); err != nil {
		t.Fatalf("load task-scoped completion gate: %v", err)
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

func loadTaskGuardState(t *testing.T, root, taskID string) GuardState {
	t.Helper()
	paths, err := adapter.Resolve(root)
	if err != nil {
		t.Fatalf("resolve paths: %v", err)
	}
	var guard GuardState
	if err := state.LoadJSON(paths.GuardStateTaskPath(taskID), &guard); err != nil {
		t.Fatalf("load task-scoped guard state: %v", err)
	}
	return guard
}
