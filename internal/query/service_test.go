package query

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"klein-harness/internal/adapter"
	"klein-harness/internal/orchestration"
	"klein-harness/internal/runtime"
	"klein-harness/internal/state"
	"klein-harness/internal/verify"
)

func TestTaskIncludesPlanningActiveSkillsAndHints(t *testing.T) {
	root := t.TempDir()
	if err := adapter.UpsertTask(root, adapter.Task{
		TaskID:         "T-skill",
		ThreadKey:      "thread-skill",
		Title:          "Resume with compact log hints",
		Summary:        "Resume execution and inspect logs first",
		PlanEpoch:      1,
		Status:         "running",
		LastDispatchID: "dispatch-skill",
		OwnedPaths:     []string{"internal/query/**"},
	}); err != nil {
		t.Fatalf("upsert task: %v", err)
	}
	paths, err := adapter.Resolve(root)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	payload := `{
		"resumeStrategy":"resume",
		"sessionId":"sess-1",
		"promptStages":["context_assembly","execute","verify"],
		"planningTracePath":"` + filepath.Join(paths.StateDir, "planning-trace-T-skill.md") + `",
		"constraintPath":"` + filepath.Join(paths.StateDir, "constraints-T-skill.json") + `",
		"acceptedPacketPath":"` + filepath.Join(paths.StateDir, "accepted-packet-T-skill.json") + `",
		"taskContractPath":"` + filepath.Join(paths.ArtifactsDir, "T-skill", "dispatch-skill", "task-contract.json") + `",
		"executionSliceId":"T-skill.slice.1",
		"runtimeRefs":{"promptPath":"` + filepath.Join(paths.StateDir, "runner-prompt-T-skill.md") + `"},
		"methodology":{"mode":"qiushi-inspired fact-first / focus-first / verify-first discipline","guidePath":"prompts/spec/methodology.md","coreRules":[],"activeLenses":[],"activeSkills":["qiushi-execution","harness-log-search-cskill"]},
		"judgeDecision":{"judgeId":"packet-judge","judgeName":"Packet Judge","selectedFlow":"state-first resume packet","winnerStrategy":"bounded winner","rationale":[],"selectedDimensions":[],"selectedLensIds":[],"reviewRequired":false,"verifyRequired":true},
		"executionLoop":{"mode":"qiushi execution / validation loop","owner":"worker + verify + runtime closeout","skillPath":"skills/qiushi-execution/SKILL.md","activeSkills":["qiushi-execution","harness-log-search-cskill"],"skillHints":["prefer hot state, compact logs, and prior artifacts before resuming execution"],"phases":["investigate","execute","verify","closeout","analysis","re-execute"],"coreRules":[],"retryTransition":"retry"},
		"constraintSystem":{"mode":"two-level layered constraints","objective":"obj","generation":"gen","rules":[]},
		"packetSynthesis":{"plannerCount":3,"planners":[],"judge":{"id":"packet-judge","name":"Packet Judge","focus":"focus","promptRef":"judge.md","dimensions":[]},"packetFields":[],"workerSpecFields":[]}
	}`
	if err := os.WriteFile(filepath.Join(paths.StateDir, "dispatch-ticket-T-skill.json"), []byte(payload), 0o644); err != nil {
		t.Fatalf("write dispatch ticket: %v", err)
	}
	if err := os.WriteFile(filepath.Join(paths.StateDir, "planning-trace-T-skill.md"), []byte("trace"), 0o644); err != nil {
		t.Fatalf("write planning trace: %v", err)
	}
	view, err := Task(root, "T-skill")
	if err != nil {
		t.Fatalf("task view: %v", err)
	}
	if len(view.ActiveSkills) != 2 || view.ActiveSkills[0] != "qiushi-execution" {
		t.Fatalf("expected active skills from planning view: %+v", view.ActiveSkills)
	}
	if len(view.SkillHints) == 0 {
		t.Fatalf("expected skill hints from planning view: %+v", view.SkillHints)
	}
	if view.Planning == nil || len(view.Planning.ActiveSkills) != 2 || len(view.Planning.SkillHints) == 0 {
		t.Fatalf("expected planning active skills and hints: %+v", view.Planning)
	}
}

func TestTaskLoadsCompiledContextAndContinuationContracts(t *testing.T) {
	root := t.TempDir()
	if err := adapter.UpsertTask(root, adapter.Task{
		TaskID:         "T-contract",
		ThreadKey:      "thread-contract",
		Title:          "Resume compiled contracts",
		Summary:        "Resume compiled contracts",
		PlanEpoch:      1,
		Status:         "running",
		LastDispatchID: "dispatch-contract",
		OwnedPaths:     []string{"internal/query/**"},
	}); err != nil {
		t.Fatalf("upsert task: %v", err)
	}
	paths, err := adapter.Resolve(root)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	artifactDir := filepath.Join(paths.ArtifactsDir, "T-contract", "dispatch-contract")
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		t.Fatalf("mkdir artifact dir: %v", err)
	}
	contextLayersPath := filepath.Join(artifactDir, "context-layers.json")
	sharedFlowPath := filepath.Join(artifactDir, "shared-flow-context.json")
	sliceContextPath := filepath.Join(artifactDir, "slice-context.json")
	verifySkeletonPath := filepath.Join(artifactDir, "verify-skeleton.json")
	closeoutPath := filepath.Join(artifactDir, "closeout-skeleton.json")
	handoffPath := filepath.Join(artifactDir, "handoff-contract.json")
	takeoverPath := filepath.Join(artifactDir, "takeover-context.json")

	if err := writeJSONFile(contextLayersPath, map[string]any{
		"schemaVersion":  "kh.context-layers.v1",
		"request":        map[string]any{"goal": "Resume compiled contracts"},
		"sharedFlow":     map[string]any{"taskFamily": "feature_system", "sopId": "sop.development_task.v1", "summary": "shared summary"},
		"sliceLocal":     map[string]any{"executionSliceId": "T-contract.slice.1", "sliceMode": "direct_pass"},
		"runtimeControl": map[string]any{"taskId": "T-contract", "dispatchId": "dispatch-contract", "executionCwd": "/repo/.worktrees/T-contract", "worktreePath": ".worktrees/T-contract", "ownedPaths": []string{"internal/query/**"}},
	}); err != nil {
		t.Fatalf("write context layers: %v", err)
	}
	if err := writeJSONFile(sharedFlowPath, map[string]any{
		"taskFamily":     "feature_system",
		"sopId":          "sop.development_task.v1",
		"summary":        "shared summary",
		"compiledPhases": []string{"requirement_spec", "task_graph_compile"},
		"taskGraphRef":   filepath.Join(artifactDir, "task-graph.json"),
	}); err != nil {
		t.Fatalf("write shared flow: %v", err)
	}
	if err := writeJSONFile(sliceContextPath, map[string]any{
		"executionSliceId":  "T-contract.slice.1",
		"sliceMode":         "direct_pass",
		"allowedWriteGlobs": []string{"internal/query/**"},
	}); err != nil {
		t.Fatalf("write slice context: %v", err)
	}
	if err := writeJSONFile(verifySkeletonPath, map[string]any{
		"schemaVersion":    "kh.verify-skeleton.v1",
		"taskId":           "T-contract",
		"executionSliceId": "T-contract.slice.1",
		"checks":           []map[string]any{{"id": "required_artifacts", "kind": "artifact_presence", "description": "required artifacts"}},
	}); err != nil {
		t.Fatalf("write verify skeleton: %v", err)
	}
	if err := writeJSONFile(closeoutPath, map[string]any{
		"schemaVersion":      "kh.closeout-skeleton.v1",
		"taskId":             "T-contract",
		"verifySkeletonPath": verifySkeletonPath,
		"workerMustProvide":  []string{"verify.json", "handoff.md"},
	}); err != nil {
		t.Fatalf("write closeout skeleton: %v", err)
	}
	if err := writeJSONFile(handoffPath, map[string]any{
		"schemaVersion":      "kh.handoff-contract.v1",
		"taskId":             "T-contract",
		"executionSliceId":   "T-contract.slice.1",
		"requiredArtifacts":  []string{"verify.json", "handoff.md"},
		"resumeInstructions": []string{"resume from context-layers.json"},
	}); err != nil {
		t.Fatalf("write handoff contract: %v", err)
	}
	if err := writeJSONFile(takeoverPath, map[string]any{
		"schemaVersion":         "kh.multi-session-continuation.v1",
		"taskId":                "T-contract",
		"dispatchId":            "dispatch-contract",
		"executionSliceId":      "T-contract.slice.1",
		"executionCwd":          "/repo/.worktrees/T-contract",
		"worktreePath":          ".worktrees/T-contract",
		"contextLayersPath":     contextLayersPath,
		"sharedFlowContextPath": sharedFlowPath,
		"sliceContextPath":      sliceContextPath,
		"verifySkeletonPath":    verifySkeletonPath,
		"closeoutSkeletonPath":  closeoutPath,
		"handoffContractPath":   handoffPath,
		"ownedPaths":            []string{"internal/query/**"},
		"readOrder":             []string{contextLayersPath, sharedFlowPath, sliceContextPath},
	}); err != nil {
		t.Fatalf("write takeover contract: %v", err)
	}
	if err := orchestration.WriteTaskContract(filepath.Join(artifactDir, "task-contract.json"), orchestration.TaskContract{
		SchemaVersion:         "kh.task-contract.v1",
		Generator:             "test",
		GeneratedAt:           "2026-03-29T10:00:00Z",
		ContractID:            "contract_T-contract_1_1",
		TaskID:                "T-contract",
		TaskFamily:            orchestration.TaskFamilyFeatureSystem,
		SOPID:                 orchestration.SOPDevelopmentTaskV1,
		DispatchID:            "dispatch-contract",
		ThreadKey:             "thread-contract",
		PlanEpoch:             1,
		ExecutionSliceID:      "T-contract.slice.1",
		ExecutionCWD:          "/repo/.worktrees/T-contract",
		WorktreePath:          ".worktrees/T-contract",
		Objective:             "Resume compiled contracts",
		InScope:               []string{"internal/query/**"},
		DoneCriteria:          []string{"contracts remain traceable"},
		VerificationChecklist: []orchestration.VerificationChecklistItem{{ID: "required_artifacts", Title: "required artifacts", Required: true}},
		RequiredEvidence:      []string{"verify.json"},
		ReviewRequired:        false,
		ContractStatus:        "accepted",
		ProposedBy:            "test",
		AcceptedBy:            "test",
		AcceptedAt:            "2026-03-29T10:00:00Z",
		AcceptedPacketPath:    orchestration.AcceptedPacketPath(root, "T-contract"),
		SharedFlowContextPath: sharedFlowPath,
		TaskGraphPath:         filepath.Join(artifactDir, "task-graph.json"),
		SliceContextPath:      sliceContextPath,
		ContextLayersPath:     contextLayersPath,
		VerifySkeletonPath:    verifySkeletonPath,
		CloseoutSkeletonPath:  closeoutPath,
		HandoffContractPath:   handoffPath,
		TakeoverPath:          takeoverPath,
	}); err != nil {
		t.Fatalf("write task contract: %v", err)
	}

	view, err := Task(root, "T-contract")
	if err != nil {
		t.Fatalf("task view: %v", err)
	}
	if view.ContextLayers == nil || view.ContextLayers.SliceLocal.ExecutionSliceID != "T-contract.slice.1" {
		t.Fatalf("expected context layers in task view: %+v", view.ContextLayers)
	}
	if view.SharedFlow == nil || view.SharedFlow.SOPID != orchestration.SOPDevelopmentTaskV1 {
		t.Fatalf("expected shared flow context in task view: %+v", view.SharedFlow)
	}
	if view.VerifySkeleton == nil || len(view.VerifySkeleton.Checks) == 0 {
		t.Fatalf("expected verify skeleton in task view: %+v", view.VerifySkeleton)
	}
	if view.Closeout == nil || len(view.Closeout.WorkerMustProvide) == 0 {
		t.Fatalf("expected closeout skeleton in task view: %+v", view.Closeout)
	}
	if view.Handoff == nil || len(view.Handoff.ResumeInstructions) == 0 {
		t.Fatalf("expected handoff contract in task view: %+v", view.Handoff)
	}
	if view.Continuation == nil || view.Continuation.ContextLayersPath != contextLayersPath || view.Continuation.HandoffContractPath != handoffPath {
		t.Fatalf("expected continuation protocol in task view: %+v", view.Continuation)
	}
	if view.ContextLayers.RuntimeControl.ExecutionCWD == "" || view.ContextLayers.RuntimeControl.WorktreePath == "" || view.Continuation.ExecutionCWD == "" || view.Continuation.WorktreePath == "" || len(view.Continuation.OwnedPaths) == 0 {
		t.Fatalf("expected execution cwd/worktree continuity in task view: context=%+v continuation=%+v", view.ContextLayers, view.Continuation)
	}
}

func TestTaskIncludesPacketProgressAndRemainingSlices(t *testing.T) {
	root := t.TempDir()
	if err := adapter.UpsertTask(root, adapter.Task{
		TaskID:     "T-1",
		ThreadKey:  "thread-1",
		Title:      "Query task progress",
		Summary:    "Expose remaining slices",
		PlanEpoch:  1,
		Status:     "queued",
		OwnedPaths: []string{"internal/query/**"},
	}); err != nil {
		t.Fatalf("upsert task: %v", err)
	}
	if err := orchestration.WriteAcceptedPacket(orchestration.AcceptedPacketPath(root, "T-1"), orchestration.AcceptedPacket{
		SchemaVersion: "kh.accepted-packet.v1",
		Generator:     "test",
		GeneratedAt:   "2026-03-26T10:00:00Z",
		TaskID:        "T-1",
		ThreadKey:     "thread-1",
		PlanEpoch:     1,
		PacketID:      "packet_T-1_1",
		Objective:     "Expose remaining slices",
		SelectedPlan:  "Run slices in order",
		ExecutionTasks: []orchestration.ExecutionTask{
			{ID: "T-1.slice.1", Title: "slice 1", Summary: "one"},
			{ID: "T-1.slice.2", Title: "slice 2", Summary: "two"},
			{ID: "T-1.slice.3", Title: "slice 3", Summary: "three"},
		},
		VerificationPlan:  map[string]any{},
		DecisionRationale: "test packet",
		AcceptedAt:        "2026-03-26T10:00:00Z",
		AcceptedBy:        "test",
	}); err != nil {
		t.Fatalf("write accepted packet: %v", err)
	}
	if err := orchestration.WritePacketProgress(orchestration.PacketProgressPath(root, "T-1"), orchestration.PacketProgress{
		SchemaVersion:     "kh.packet-progress.v1",
		Generator:         "test",
		UpdatedAt:         "2026-03-26T10:00:00Z",
		TaskID:            "T-1",
		ThreadKey:         "thread-1",
		PlanEpoch:         1,
		AcceptedPacketID:  "packet_T-1_1",
		CompletedSliceIDs: []string{"T-1.slice.1"},
	}); err != nil {
		t.Fatalf("write packet progress: %v", err)
	}

	view, err := Task(root, "T-1")
	if err != nil {
		t.Fatalf("load task view: %v", err)
	}
	if view.PacketProgress == nil {
		t.Fatalf("expected packet progress in task view")
	}
	if len(view.RemainingSlices) != 2 || view.RemainingSlices[0] != "T-1.slice.2" || view.RemainingSlices[1] != "T-1.slice.3" {
		t.Fatalf("unexpected remaining slices: %+v", view.RemainingSlices)
	}
	if view.NextSliceID != "T-1.slice.2" {
		t.Fatalf("expected next slice id, got %q", view.NextSliceID)
	}
	if view.Release.Status != "more_slices_remaining" || view.Release.NextAction != "replan" {
		t.Fatalf("expected release readiness to surface remaining slices: %+v", view.Release)
	}
}

func TestTaskIgnoresStalePacketProgressAcrossPlanEpoch(t *testing.T) {
	root := t.TempDir()
	if err := adapter.UpsertTask(root, adapter.Task{
		TaskID:     "T-stale",
		ThreadKey:  "thread-stale",
		Title:      "Ignore stale progress",
		Summary:    "Expose remaining slices",
		PlanEpoch:  2,
		Status:     "queued",
		OwnedPaths: []string{"internal/query/**"},
	}); err != nil {
		t.Fatalf("upsert task: %v", err)
	}
	if err := orchestration.WriteAcceptedPacket(orchestration.AcceptedPacketPath(root, "T-stale"), orchestration.AcceptedPacket{
		SchemaVersion: "kh.accepted-packet.v1",
		Generator:     "test",
		GeneratedAt:   "2026-03-26T10:00:00Z",
		TaskID:        "T-stale",
		ThreadKey:     "thread-stale",
		PlanEpoch:     2,
		PacketID:      "packet_T-stale_2",
		Objective:     "Expose remaining slices",
		SelectedPlan:  "Run slices in order",
		ExecutionTasks: []orchestration.ExecutionTask{
			{ID: "T-stale.slice.1", Title: "slice 1", Summary: "one"},
			{ID: "T-stale.slice.2", Title: "slice 2", Summary: "two"},
		},
		VerificationPlan:  map[string]any{},
		DecisionRationale: "test packet",
		AcceptedAt:        "2026-03-26T10:00:00Z",
		AcceptedBy:        "test",
	}); err != nil {
		t.Fatalf("write accepted packet: %v", err)
	}
	if err := orchestration.WritePacketProgress(orchestration.PacketProgressPath(root, "T-stale"), orchestration.PacketProgress{
		SchemaVersion:     "kh.packet-progress.v1",
		Generator:         "test",
		UpdatedAt:         "2026-03-26T10:00:00Z",
		TaskID:            "T-stale",
		ThreadKey:         "thread-stale",
		PlanEpoch:         1,
		AcceptedPacketID:  "packet_T-stale_1",
		CompletedSliceIDs: []string{"T-stale.slice.1"},
	}); err != nil {
		t.Fatalf("write stale packet progress: %v", err)
	}

	view, err := Task(root, "T-stale")
	if err != nil {
		t.Fatalf("load task view: %v", err)
	}
	if len(view.RemainingSlices) != 2 || view.RemainingSlices[0] != "T-stale.slice.1" {
		t.Fatalf("expected stale progress to be ignored, got %+v", view.RemainingSlices)
	}
}

func TestTaskIncludesIntakeThreadChangeAndTodoSummaries(t *testing.T) {
	root := t.TempDir()
	submitResult, err := runtime.Submit(runtime.SubmitRequest{
		Root:     root,
		Goal:     "Refine intake query visibility",
		Contexts: []string{"docs/notes.md"},
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}

	view, err := Task(root, submitResult.Task.TaskID)
	if err != nil {
		t.Fatalf("load task view: %v", err)
	}
	if view.IntakeSummary == nil || view.IntakeSummary.LatestTaskID != submitResult.Task.TaskID {
		t.Fatalf("expected intake summary in task view: %+v", view.IntakeSummary)
	}
	if view.ThreadEntry == nil || view.ThreadEntry.ThreadKey != submitResult.Task.ThreadKey {
		t.Fatalf("expected thread entry in task view: %+v", view.ThreadEntry)
	}
	if view.ChangeSummary == nil || view.ChangeSummary.LatestTaskID != submitResult.Task.TaskID {
		t.Fatalf("expected change summary in task view: %+v", view.ChangeSummary)
	}
	if view.TodoSummary == nil || view.TodoSummary.NextTaskID != submitResult.Task.TaskID {
		t.Fatalf("expected todo summary in task view: %+v", view.TodoSummary)
	}
	if view.Request == nil || view.Request.TaskID != submitResult.Task.TaskID || view.Request.NormalizedIntentClass == "" {
		t.Fatalf("expected latest request record in task view: %+v", view.Request)
	}
}

func TestTaskIncludesReleaseReadyStateFromGateAndGuard(t *testing.T) {
	root := t.TempDir()
	if err := adapter.UpsertTask(root, adapter.Task{
		TaskID:              "T-2",
		ThreadKey:           "thread-2",
		Title:               "Ready for archive",
		Summary:             "Ready for archive",
		Status:              "completed",
		VerificationStatus:  "passed",
		VerificationSummary: "verified",
	}); err != nil {
		t.Fatalf("upsert task: %v", err)
	}
	paths, err := adapter.Resolve(root)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if _, err := state.WriteSnapshot(paths.CompletionGatePath, &verify.CompletionGate{
		Status:     "satisfied",
		Satisfied:  true,
		TaskID:     "T-2",
		DispatchID: "dispatch-2",
	}, "test", 0); err != nil {
		t.Fatalf("write completion gate: %v", err)
	}
	if _, err := state.WriteSnapshot(paths.GuardStatePath, &verify.GuardState{
		Status:                  "retire_ready",
		TaskID:                  "T-2",
		DispatchID:              "dispatch-2",
		SafeToArchive:           true,
		CompletionGateStatus:    "satisfied",
		CompletionGateSatisfied: true,
		RetireEligible:          true,
	}, "test", 0); err != nil {
		t.Fatalf("write guard state: %v", err)
	}

	view, err := Task(root, "T-2")
	if err != nil {
		t.Fatalf("load task view: %v", err)
	}
	if view.Release.Status != "release_ready" || !view.Release.Ready || !view.Release.SafeToArchive || view.Release.NextAction != "archive" {
		t.Fatalf("expected release ready view: %+v", view.Release)
	}
}

func TestTaskPrefersTaskScopedCompletionAndGuard(t *testing.T) {
	root := t.TempDir()
	if err := adapter.UpsertTask(root, adapter.Task{
		TaskID:             "T-3",
		ThreadKey:          "thread-3",
		Title:              "task scoped gate",
		Summary:            "task scoped gate",
		Status:             "completed",
		VerificationStatus: "passed",
	}); err != nil {
		t.Fatalf("upsert task: %v", err)
	}
	paths, err := adapter.Resolve(root)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if _, err := state.WriteSnapshot(paths.CompletionGatePath, &verify.CompletionGate{
		Status:     "blocked",
		Satisfied:  false,
		TaskID:     "other-task",
		DispatchID: "dispatch-other",
	}, "test", 0); err != nil {
		t.Fatalf("write alias completion gate: %v", err)
	}
	if _, err := state.WriteSnapshot(paths.GuardStatePath, &verify.GuardState{
		Status:                  "blocked",
		TaskID:                  "other-task",
		DispatchID:              "dispatch-other",
		SafeToArchive:           false,
		CompletionGateStatus:    "blocked",
		CompletionGateSatisfied: false,
	}, "test", 0); err != nil {
		t.Fatalf("write alias guard state: %v", err)
	}
	if _, err := state.WriteSnapshot(paths.CompletionGateTaskPath("T-3"), &verify.CompletionGate{
		Status:     "satisfied",
		Satisfied:  true,
		TaskID:     "T-3",
		DispatchID: "dispatch-3",
	}, "test", 0); err != nil {
		t.Fatalf("write task-scoped completion gate: %v", err)
	}
	if _, err := state.WriteSnapshot(paths.GuardStateTaskPath("T-3"), &verify.GuardState{
		Status:                  "retire_ready",
		TaskID:                  "T-3",
		DispatchID:              "dispatch-3",
		SafeToArchive:           true,
		CompletionGateStatus:    "satisfied",
		CompletionGateSatisfied: true,
		RetireEligible:          true,
	}, "test", 0); err != nil {
		t.Fatalf("write task-scoped guard state: %v", err)
	}

	view, err := Task(root, "T-3")
	if err != nil {
		t.Fatalf("load task view: %v", err)
	}
	if view.Completion == nil || view.Completion.TaskID != "T-3" || !view.Completion.Satisfied {
		t.Fatalf("expected task-scoped completion gate to win: %+v", view.Completion)
	}
	if view.Guard == nil || view.Guard.TaskID != "T-3" || !view.Guard.SafeToArchive {
		t.Fatalf("expected task-scoped guard state to win: %+v", view.Guard)
	}
}

func TestTaskLoadsLatestRequestFromRequestIndex(t *testing.T) {
	root := t.TempDir()
	if err := adapter.UpsertTask(root, adapter.Task{
		TaskID:    "T-4",
		ThreadKey: "thread-4",
		Title:     "request index",
		Summary:   "request index",
		Status:    "queued",
	}); err != nil {
		t.Fatalf("upsert task: %v", err)
	}
	paths, err := adapter.Resolve(root)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	index := runtime.RequestIndex{
		RequestsByID: map[string]runtime.RequestRecord{
			"R-4": {
				RequestID:             "R-4",
				TaskID:                "T-4",
				ThreadKey:             "thread-4",
				BindingAction:         "created_new_task",
				Goal:                  "hydrate request index",
				Status:                "queued",
				NormalizedIntentClass: "fresh_work",
			},
		},
		LatestRequestByTaskID: map[string]string{
			"T-4": "R-4",
		},
	}
	if _, err := state.WriteSnapshot(paths.RequestIndexPath, &index, "test", 0); err != nil {
		t.Fatalf("write request index: %v", err)
	}

	view, err := Task(root, "T-4")
	if err != nil {
		t.Fatalf("load task view: %v", err)
	}
	if view.Request == nil || view.Request.RequestID != "R-4" || view.Request.TaskID != "T-4" {
		t.Fatalf("expected latest request from request index: %+v", view.Request)
	}
}

func TestReleaseStatusAggregatesReleaseBoard(t *testing.T) {
	root := t.TempDir()
	if err := adapter.UpsertTask(root, adapter.Task{
		TaskID:             "T-10",
		ThreadKey:          "thread-10",
		Title:              "ready",
		Summary:            "ready",
		Status:             "completed",
		VerificationStatus: "passed",
	}); err != nil {
		t.Fatalf("upsert ready task: %v", err)
	}
	if err := adapter.UpsertTask(root, adapter.Task{
		TaskID:             "T-11",
		ThreadKey:          "thread-11",
		Title:              "awaiting gate",
		Summary:            "awaiting gate",
		Status:             "running",
		VerificationStatus: "passed",
	}); err != nil {
		t.Fatalf("upsert awaiting gate task: %v", err)
	}
	if err := adapter.UpsertTask(root, adapter.Task{
		TaskID:     "T-12",
		ThreadKey:  "thread-12",
		Title:      "needs slices",
		Summary:    "needs slices",
		Status:     "queued",
		OwnedPaths: []string{"internal/query/**"},
	}); err != nil {
		t.Fatalf("upsert remaining slices task: %v", err)
	}

	paths, err := adapter.Resolve(root)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if _, err := state.WriteSnapshot(paths.CompletionGatePath, &verify.CompletionGate{
		Status:     "satisfied",
		Satisfied:  true,
		TaskID:     "T-10",
		DispatchID: "dispatch-10",
	}, "test", 0); err != nil {
		t.Fatalf("write completion gate: %v", err)
	}
	if _, err := state.WriteSnapshot(paths.GuardStatePath, &verify.GuardState{
		Status:                  "retire_ready",
		TaskID:                  "T-10",
		DispatchID:              "dispatch-10",
		SafeToArchive:           true,
		CompletionGateStatus:    "satisfied",
		CompletionGateSatisfied: true,
		RetireEligible:          true,
	}, "test", 0); err != nil {
		t.Fatalf("write guard state: %v", err)
	}
	if err := orchestration.WriteAcceptedPacket(orchestration.AcceptedPacketPath(root, "T-12"), orchestration.AcceptedPacket{
		SchemaVersion: "kh.accepted-packet.v1",
		Generator:     "test",
		GeneratedAt:   "2026-03-26T10:00:00Z",
		TaskID:        "T-12",
		ThreadKey:     "thread-12",
		PlanEpoch:     1,
		PacketID:      "packet_T-12_1",
		Objective:     "Expose release board",
		SelectedPlan:  "Run slices in order",
		ExecutionTasks: []orchestration.ExecutionTask{
			{ID: "T-12.slice.1", Title: "slice 1", Summary: "one"},
			{ID: "T-12.slice.2", Title: "slice 2", Summary: "two"},
		},
		VerificationPlan:  map[string]any{},
		DecisionRationale: "test packet",
		AcceptedAt:        "2026-03-26T10:00:00Z",
		AcceptedBy:        "test",
	}); err != nil {
		t.Fatalf("write accepted packet: %v", err)
	}

	board, err := ReleaseStatus(root)
	if err != nil {
		t.Fatalf("release status: %v", err)
	}
	if board.ReadyCount != 1 || board.AwaitingGateCount != 1 || board.RemainingSliceCount != 1 {
		t.Fatalf("unexpected release board counters: %+v", board)
	}
	if len(board.Items) != 3 {
		t.Fatalf("expected three board items, got %+v", board.Items)
	}
}

func TestBuildReleaseSnapshotMarksDirtyBoardAsNotReady(t *testing.T) {
	board := ReleaseBoard{
		ReadyCount:        1,
		AwaitingGateCount: 1,
	}
	snapshot := buildReleaseSnapshot("/repo", "v0.2-24-g04a8e73-dirty", "v0.2.3", true, board)
	if snapshot.Ready {
		t.Fatalf("expected dirty snapshot with awaiting gate to remain not ready: %+v", snapshot)
	}
	if len(snapshot.BlockingReasons) < 2 {
		t.Fatalf("expected blocking reasons in release snapshot: %+v", snapshot)
	}
}

func writeJSONFile(path string, value any) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return os.WriteFile(path, payload, 0o644)
}
