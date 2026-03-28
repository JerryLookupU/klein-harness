package route

import "testing"

func TestEvaluateResumeDecision(t *testing.T) {
	decision := Evaluate(Input{
		TaskID:                   "T-1",
		RoleHint:                 "worker",
		Kind:                     "feature",
		Title:                    "Continue the previous implementation",
		PlanEpoch:                3,
		LatestPlanEpoch:          3,
		ResumeStrategy:           "resume",
		PreferredResumeSessionID: "sess-1",
		CheckpointFresh:          true,
		WorktreePath:             ".worktrees/T-1",
		OwnedPaths:               []string{"internal/worker/**"},
		RequiredSummaryVersion:   "state.v1",
	})
	if decision.Route != "resume" || !decision.DispatchReady {
		t.Fatalf("expected resumable decision, got %+v", decision)
	}
	for _, want := range []string{
		"checkpoint_fresh",
		"owned_paths_valid",
		"policy_resume_state_first",
		"policy_verify_evidence_required",
		"policy_review_if_multi_file_or_high_risk",
	} {
		if !containsReason(decision.ReasonCodes, want) {
			t.Fatalf("resume decision missing %q: %+v", want, decision.ReasonCodes)
		}
	}
}

func TestEvaluateBlocksMissingWorktree(t *testing.T) {
	decision := Evaluate(Input{
		TaskID:                 "T-1",
		RoleHint:               "worker",
		Kind:                   "feature",
		Title:                  "Implement the feature",
		PlanEpoch:              1,
		LatestPlanEpoch:        1,
		RequiredSummaryVersion: "state.v1",
	})
	if decision.Route != "block" {
		t.Fatalf("expected blocked route, got %+v", decision)
	}
}

func TestEvaluateBugRequestAddsDebuggingPolicy(t *testing.T) {
	decision := Evaluate(Input{
		TaskID:                 "T-bug",
		RoleHint:               "worker",
		Kind:                   "bug",
		TaskFamily:             "bugfix_small",
		SOPID:                  "sop.development_task.v1",
		Title:                  "Fix regression after verify failure",
		Summary:                "Unexpected error in route dispatch",
		PlanEpoch:              1,
		LatestPlanEpoch:        1,
		WorktreePath:           ".worktrees/T-bug",
		OwnedPaths:             []string{"internal/route/**"},
		RequiredSummaryVersion: "state.v1",
	})
	for _, want := range []string{
		"policy_bug_rca_first",
		"policy_compiled_contract_first",
		"policy_verify_evidence_required",
		"policy_review_if_multi_file_or_high_risk",
	} {
		if !containsReason(decision.ReasonCodes, want) {
			t.Fatalf("bug decision missing %q: %+v", want, decision.ReasonCodes)
		}
	}
}

func TestEvaluateRepeatedEntityCorpusAddsProgrammaticPolicies(t *testing.T) {
	decision := Evaluate(Input{
		TaskID:                 "T-corpus",
		RoleHint:               "worker",
		Kind:                   "generate",
		TaskFamily:             "repeated_entity_corpus",
		SOPID:                  "sop.repeated_entity_corpus.v1",
		Title:                  "Generate the frozen entity corpus",
		Summary:                "Compile repeated entity corpus slices",
		PlanEpoch:              1,
		LatestPlanEpoch:        1,
		WorktreePath:           ".worktrees/T-corpus",
		OwnedPaths:             []string{"rundata/generated-corpus/**"},
		RequiredSummaryVersion: "state.v1",
	})
	for _, want := range []string{
		"policy_shared_spec_frozen",
		"policy_programmatic_verify_first",
	} {
		if !containsReason(decision.ReasonCodes, want) {
			t.Fatalf("repeated-entity decision missing %q: %+v", want, decision.ReasonCodes)
		}
	}
}

func TestEvaluateRecommendationAddsOptionsPolicy(t *testing.T) {
	decision := Evaluate(Input{
		TaskID:                 "T-design",
		RoleHint:               "worker",
		Kind:                   "design",
		Title:                  "Recommend the best way to route review tasks",
		Summary:                "Compare options and tradeoffs",
		PlanEpoch:              1,
		LatestPlanEpoch:        1,
		WorktreePath:           ".worktrees/T-design",
		OwnedPaths:             []string{"prompts/spec/**"},
		RequiredSummaryVersion: "state.v1",
	})
	if !containsReason(decision.ReasonCodes, "policy_options_before_plan") {
		t.Fatalf("recommendation decision missing options policy: %+v", decision.ReasonCodes)
	}
}

func TestEvaluateContextEnrichmentReplansExistingThread(t *testing.T) {
	decision := Evaluate(Input{
		TaskID:                 "T-ctx",
		RoleHint:               "worker",
		Kind:                   "feature",
		Title:                  "Refine runtime intake",
		Summary:                "Attach new context to an existing execution thread",
		FrontDoorTriage:        "duplicate_or_context",
		NormalizedIntentClass:  "context_enrichment",
		FusionDecision:         "accepted_existing_thread",
		ChangeAffectsExecution: true,
		PendingTaskCount:       3,
		PlanEpoch:              1,
		LatestPlanEpoch:        1,
		WorktreePath:           ".worktrees/T-ctx",
		OwnedPaths:             []string{"internal/runtime/**"},
		RequiredSummaryVersion: "state.v1",
	})
	if decision.Route != "replan" || decision.DispatchReady {
		t.Fatalf("expected context enrichment to force replan, got %+v", decision)
	}
	for _, want := range []string{
		"context_enrichment_requires_replan",
		"existing_thread_replan",
		"policy_thread_reuse",
		"policy_smallest_pending_slice_first",
	} {
		if !containsReason(decision.ReasonCodes, want) {
			t.Fatalf("context enrichment decision missing %q: %+v", want, decision.ReasonCodes)
		}
	}
}

func TestEvaluateInspectionAddsReadOnlyPolicy(t *testing.T) {
	decision := Evaluate(Input{
		TaskID:                 "T-inspect",
		RoleHint:               "worker",
		Kind:                   "feature",
		Title:                  "Show current runtime status",
		Summary:                "Inspect queue and thread state",
		FrontDoorTriage:        "inspection",
		PlanEpoch:              1,
		LatestPlanEpoch:        1,
		WorktreePath:           ".worktrees/T-inspect",
		OwnedPaths:             []string{"internal/runtime/**"},
		RequiredSummaryVersion: "state.v1",
	})
	if !containsReason(decision.ReasonCodes, "policy_read_only_intake") {
		t.Fatalf("inspection decision missing read-only intake policy: %+v", decision.ReasonCodes)
	}
}

func containsReason(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
