package orchestration

import "testing"

func TestBuildContextLayersAndContinuationProtocol(t *testing.T) {
	contextLayers := BuildContextLayers(
		RequestContext{Goal: "Fix runtime", Kind: "bugfix", Contexts: []string{"logs/run.log"}},
		SharedFlowContext{TaskFamily: TaskFamilyBugfixSmall, SOPID: SOPDevelopmentTaskV1, Summary: "shared"},
		SliceLocalContext{ExecutionSliceID: "T-1.slice.1", SliceMode: "direct_pass", Sequence: 1, TotalSlices: 1},
		RuntimeControlContext{TaskID: "T-1", DispatchID: "dispatch-1", ExecutionCWD: "/repo/.worktrees/T-1", WorktreePath: ".worktrees/T-1", OwnedPaths: []string{"internal/runtime/**"}, ContextLayersPath: "/repo/.harness/artifacts/T-1/context-layers.json"},
	)
	if contextLayers.SchemaVersion != "kh.context-layers.v1" || contextLayers.SliceLocal.SliceMode != "direct_pass" || contextLayers.RuntimeControl.ExecutionCWD == "" || contextLayers.RuntimeControl.WorktreePath == "" {
		t.Fatalf("unexpected context layers: %+v", contextLayers)
	}

	protocol := BuildContinuationProtocol(ContinuationProtocolInput{
		TaskID:                "T-1",
		DispatchID:            "dispatch-1",
		TaskFamily:            TaskFamilyBugfixSmall,
		SOPID:                 SOPDevelopmentTaskV1,
		ExecutionSliceID:      "T-1.slice.1",
		ResumeStrategy:        "resume",
		ResumeSessionID:       "sess-1",
		TaskStatus:            "running",
		ExecutionCWD:          "/repo/.worktrees/T-1",
		WorktreePath:          ".worktrees/T-1",
		ContextLayersPath:     "/repo/.harness/artifacts/T-1/context-layers.json",
		RequestContextPath:    "/repo/.harness/artifacts/T-1/request-context.json",
		RuntimeContextPath:    "/repo/.harness/artifacts/T-1/runtime-control-context.json",
		SharedFlowContextPath: "/repo/.harness/artifacts/T-1/shared-flow-context.json",
		SliceContextPath:      "/repo/.harness/artifacts/T-1/slice-context.json",
		TaskContractPath:      "/repo/.harness/artifacts/T-1/task-contract.json",
		TaskGraphPath:         "/repo/.harness/artifacts/T-1/task-graph.json",
		AcceptedPacketPath:    "/repo/.harness/state/accepted-packet-T-1.json",
		VerifySkeletonPath:    "/repo/.harness/artifacts/T-1/verify-skeleton.json",
		CloseoutSkeletonPath:  "/repo/.harness/artifacts/T-1/closeout-skeleton.json",
		HandoffContractPath:   "/repo/.harness/artifacts/T-1/handoff-contract.json",
		HandoffPath:           "/repo/.harness/artifacts/T-1/handoff.md",
		SessionRegistryPath:   "/repo/.harness/state/session-registry.json",
		ArtifactDir:           "/repo/.harness/artifacts/T-1/dispatch-1",
		ReadOrder:             []string{"context-layers.json", "shared-flow-context.json", "slice-context.json"},
		RequiredArtifacts:     []string{"context-layers.json", "verify-skeleton.json", "handoff.md"},
		OwnedPaths:            []string{"internal/runtime/**"},
		AllowedWriteGlobs:     []string{"internal/runtime/**"},
		ForbiddenWriteGlobs:   []string{".harness/**"},
		EntryChecklist:        []string{"resume from context-layers.json"},
		ControlPlaneGuards:    []string{"do not mutate global .harness/state truth ledgers"},
	})
	if protocol.SchemaVersion != "kh.multi-session-continuation.v1" || protocol.ProtocolID == "" {
		t.Fatalf("unexpected continuation protocol: %+v", protocol)
	}
	if protocol.ContextLayersPath == "" || len(protocol.ReadOrder) == 0 || len(protocol.RequiredArtifacts) == 0 {
		t.Fatalf("expected continuation protocol to carry explicit file contract, got %+v", protocol)
	}
	if protocol.CloseoutSkeletonPath == "" || protocol.ResumeSessionID != "sess-1" || protocol.TaskStatus != "running" || protocol.ArtifactDir == "" || protocol.ExecutionCWD == "" || protocol.WorktreePath == "" || len(protocol.OwnedPaths) == 0 {
		t.Fatalf("expected continuation protocol to carry status summary, got %+v", protocol)
	}
	if len(protocol.EntryChecklist) == 0 || len(protocol.ControlPlaneGuards) == 0 {
		t.Fatalf("expected continuation protocol to carry resume checklist and guardrails, got %+v", protocol)
	}
}
