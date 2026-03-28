package route

import (
	"strings"

	"klein-harness/internal/worktree"
)

type Input struct {
	TaskID                    string
	RoleHint                  string
	Kind                      string
	TaskFamily                string
	SOPID                     string
	Title                     string
	Summary                   string
	FrontDoorTriage           string
	NormalizedIntentClass     string
	FusionDecision            string
	ChangeAffectsExecution    bool
	PendingTaskCount          int
	WorkerMode                string
	PlanEpoch                 int
	LatestPlanEpoch           int
	ResumeStrategy            string
	PreferredResumeSessionID  string
	CandidateResumeSessionIDs []string
	SessionContested          bool
	CheckpointRequired        bool
	CheckpointFresh           bool
	WorktreePath              string
	OwnedPaths                []string
	RequiredSummaryVersion    string
}

type Decision struct {
	Route                  string   `json:"route"`
	DispatchReady          bool     `json:"dispatchReady"`
	ReasonCodes            []string `json:"reasonCodes"`
	RequiredSummaryVersion string   `json:"requiredSummaryVersion"`
	ResumeSessionID        string   `json:"resumeSessionId,omitempty"`
	WorktreePath           string   `json:"worktreePath,omitempty"`
	OwnedPaths             []string `json:"ownedPaths,omitempty"`
}

func Evaluate(input Input) Decision {
	policyTags := policyReasonCodes(input)
	reasons := make([]string, 0)
	if input.FusionDecision == "accepted_existing_thread" &&
		input.NormalizedIntentClass == "context_enrichment" &&
		input.ChangeAffectsExecution {
		return Decision{
			Route:                  "replan",
			DispatchReady:          false,
			ReasonCodes:            append([]string{"context_enrichment_requires_replan", "existing_thread_replan"}, policyTags...),
			RequiredSummaryVersion: input.RequiredSummaryVersion,
			WorktreePath:           input.WorktreePath,
			OwnedPaths:             input.OwnedPaths,
		}
	}
	if input.LatestPlanEpoch > 0 && input.PlanEpoch > 0 && input.PlanEpoch < input.LatestPlanEpoch {
		return Decision{
			Route:                  "replan",
			DispatchReady:          false,
			ReasonCodes:            append([]string{"plan_epoch_stale"}, policyTags...),
			RequiredSummaryVersion: input.RequiredSummaryVersion,
			WorktreePath:           input.WorktreePath,
			OwnedPaths:             input.OwnedPaths,
		}
	}
	if input.CheckpointRequired {
		return Decision{
			Route:                  "block",
			DispatchReady:          false,
			ReasonCodes:            append([]string{"checkpoint_required"}, policyTags...),
			RequiredSummaryVersion: input.RequiredSummaryVersion,
			WorktreePath:           input.WorktreePath,
			OwnedPaths:             input.OwnedPaths,
		}
	}
	if worktree.RequiresIsolatedWorktree(input.RoleHint, input.Kind, input.WorkerMode) {
		if input.WorktreePath == "" {
			return Decision{
				Route:                  "block",
				DispatchReady:          false,
				ReasonCodes:            append([]string{"worktree_missing"}, policyTags...),
				RequiredSummaryVersion: input.RequiredSummaryVersion,
			}
		}
		if len(input.OwnedPaths) == 0 {
			return Decision{
				Route:                  "block",
				DispatchReady:          false,
				ReasonCodes:            append([]string{"owned_paths_missing"}, policyTags...),
				RequiredSummaryVersion: input.RequiredSummaryVersion,
				WorktreePath:           input.WorktreePath,
			}
		}
	}

	if input.ResumeStrategy == "resume" || input.PreferredResumeSessionID != "" || len(input.CandidateResumeSessionIDs) > 0 {
		if input.SessionContested {
			return Decision{
				Route:                  "block",
				DispatchReady:          false,
				ReasonCodes:            append([]string{"resume_session_contested"}, policyTags...),
				RequiredSummaryVersion: input.RequiredSummaryVersion,
				WorktreePath:           input.WorktreePath,
				OwnedPaths:             input.OwnedPaths,
			}
		}
		if input.CheckpointFresh && input.PreferredResumeSessionID != "" {
			return Decision{
				Route:                  "resume",
				DispatchReady:          true,
				ReasonCodes:            append([]string{"checkpoint_fresh", "owned_paths_valid"}, policyTags...),
				RequiredSummaryVersion: input.RequiredSummaryVersion,
				ResumeSessionID:        input.PreferredResumeSessionID,
				WorktreePath:           input.WorktreePath,
				OwnedPaths:             input.OwnedPaths,
			}
		}
		reasons = append(reasons, "checkpoint_stale_fresh_start")
	}

	if len(reasons) == 0 {
		reasons = append(reasons, "dispatch_ready")
	}
	reasons = append(reasons, policyTags...)
	return Decision{
		Route:                  "dispatch",
		DispatchReady:          true,
		ReasonCodes:            uniqueReasonCodes(reasons),
		RequiredSummaryVersion: input.RequiredSummaryVersion,
		WorktreePath:           input.WorktreePath,
		OwnedPaths:             input.OwnedPaths,
	}
}

func policyReasonCodes(input Input) []string {
	tags := []string{
		"policy_verify_evidence_required",
		"policy_review_if_multi_file_or_high_risk",
	}
	signal := strings.ToLower(strings.TrimSpace(strings.Join([]string{
		input.Kind,
		input.TaskFamily,
		input.SOPID,
		input.Title,
		input.Summary,
	}, " ")))
	if matchesSignal(signal,
		"bug", "failure", "failing", "error", "regression", "broken", "not working",
		"unexpected", "crash", "traceback", "exception", "debug", "investigate", "wrong",
	) {
		tags = append(tags, "policy_bug_rca_first")
	}
	if matchesSignal(signal,
		"recommend", "comparison", "compare", "trade-off", "tradeoff", "choose",
		"best way", "best approach", "which one", "which option", "pros and cons",
		"how should", "help me choose", "help me decide",
	) {
		tags = append(tags, "policy_options_before_plan")
	}
	if matchesSignal(signal,
		"harness", "bootstrap", "refresh", "audit", "agent-entry", "agent entry",
		"claim", "handoff", "coordination", "session registry", "task pool",
		"control plane", "execution plane", "operator plane",
	) {
		tags = append(tags, "policy_harness_state_first")
	}
	if matchesSignal(signal,
		"log search", "compact log", "handoff log", "runner log", "raw log",
		"transcript", "evidence window", "log window", "runtime log", "logs",
	) {
		tags = append(tags, "policy_log_compact_first")
	}
	if matchesSignal(signal,
		"dashboard", "overview", "watch", "metrics", "forever", "unattended",
		"parallel", "who is running", "status board", "operator surface",
	) {
		tags = append(tags, "policy_operator_surface_required")
	}
	if input.FrontDoorTriage == "inspection" || input.FrontDoorTriage == "advisory_read_only" {
		tags = append(tags, "policy_read_only_intake")
	}
	if input.FusionDecision == "accepted_existing_thread" {
		tags = append(tags, "policy_thread_reuse")
	}
	if input.PendingTaskCount > 1 {
		tags = append(tags, "policy_smallest_pending_slice_first")
	}
	if input.ResumeStrategy == "resume" || input.PreferredResumeSessionID != "" || len(input.CandidateResumeSessionIDs) > 0 ||
		matchesSignal(signal, "continue", "resume", "pick up", "keep going", "continue from", "continued from") {
		tags = append(tags, "policy_resume_state_first")
	}
	switch input.TaskFamily {
	case "repeated_entity_corpus":
		tags = append(tags,
			"policy_shared_spec_frozen",
			"policy_programmatic_verify_first",
		)
	case "development_task", "bugfix_small", "feature_module", "feature_system", "integration_external", "repair_or_resume":
		tags = append(tags, "policy_compiled_contract_first")
	}
	return uniqueReasonCodes(tags)
}

func matchesSignal(signal string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(signal, needle) {
			return true
		}
	}
	return false
}

func uniqueReasonCodes(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
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
