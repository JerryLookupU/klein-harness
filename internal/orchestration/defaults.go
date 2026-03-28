package orchestration

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type PlannerAgent struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Focus     string `json:"focus"`
	PromptRef string `json:"promptRef"`
}

type PlannerCandidate struct {
	PlannerID      string   `json:"plannerId"`
	PlannerName    string   `json:"plannerName"`
	Focus          string   `json:"focus"`
	TaskName       string   `json:"taskName"`
	ProposedFlow   string   `json:"proposedFlow,omitempty"`
	ResultSummary  string   `json:"resultSummary,omitempty"`
	KeyMoves       []string `json:"keyMoves,omitempty"`
	Risks          []string `json:"risks,omitempty"`
	Evidence       []string `json:"evidence,omitempty"`
	MaterializedBy string   `json:"materializedBy,omitempty"`
}

type JudgeAgent struct {
	ID               string              `json:"id"`
	Name             string              `json:"name"`
	Focus            string              `json:"focus"`
	PromptRef        string              `json:"promptRef"`
	SkillPath        string              `json:"skillPath,omitempty"`
	ActiveSkills     []string            `json:"activeSkills,omitempty"`
	SkillHints       []string            `json:"skillHints,omitempty"`
	CoreCapabilities []string            `json:"coreCapabilities,omitempty"`
	ToolContracts    []JudgeToolContract `json:"toolContracts,omitempty"`
	Dimensions       []string            `json:"dimensions"`
}

type JudgeToolContract struct {
	ID      string `json:"id"`
	Purpose string `json:"purpose"`
	When    string `json:"when"`
	Output  string `json:"output"`
	Source  string `json:"source"`
}

type PacketSynthesisLoop struct {
	PlannerCount     int            `json:"plannerCount"`
	Planners         []PlannerAgent `json:"planners"`
	Judge            JudgeAgent     `json:"judge"`
	PacketFields     []string       `json:"packetFields"`
	WorkerSpecFields []string       `json:"workerSpecFields"`
}

type MethodologyLens struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Trigger     string `json:"trigger"`
	Effect      string `json:"effect"`
	Stage       string `json:"stage"`
	HookifyHint string `json:"hookifyHint,omitempty"`
}

type MethodologyContract struct {
	Mode         string            `json:"mode"`
	GuidePath    string            `json:"guidePath"`
	CoreRules    []string          `json:"coreRules"`
	ActiveLenses []MethodologyLens `json:"activeLenses"`
	ActiveSkills []string          `json:"activeSkills,omitempty"`
}

type JudgeDecision struct {
	JudgeID            string   `json:"judgeId"`
	JudgeName          string   `json:"judgeName"`
	SelectedFlow       string   `json:"selectedFlow"`
	WinnerStrategy     string   `json:"winnerStrategy"`
	Rationale          []string `json:"rationale"`
	SelectedDimensions []string `json:"selectedDimensions"`
	SelectedLensIDs    []string `json:"selectedLensIds"`
	ReviewRequired     bool     `json:"reviewRequired"`
	VerifyRequired     bool     `json:"verifyRequired"`
}

type ExecutionLoopContract struct {
	Mode            string   `json:"mode"`
	Owner           string   `json:"owner"`
	SkillPath       string   `json:"skillPath"`
	ActiveSkills    []string `json:"activeSkills,omitempty"`
	SkillHints      []string `json:"skillHints,omitempty"`
	Phases          []string `json:"phases"`
	CoreRules       []string `json:"coreRules"`
	RetryTransition string   `json:"retryTransition"`
}

type ConstraintRule struct {
	ID               string `json:"id"`
	Layer            string `json:"layer"`
	Category         string `json:"category"`
	Enforcement      string `json:"enforcement"`
	Level            string `json:"level"`
	TargetSignal     string `json:"targetSignal"`
	VerificationMode string `json:"verificationMode,omitempty"`
	Rule             string `json:"rule"`
	Source           string `json:"source"`
}

type ConstraintSystem struct {
	Mode       string           `json:"mode"`
	Objective  string           `json:"objective"`
	Generation string           `json:"generation"`
	Rules      []ConstraintRule `json:"rules"`
}

const promptDir = "prompts/spec"

func DefaultPromptStages() []string {
	return []string{
		"context_assembly",
		"packet_parallel_planning",
		"packet_judging",
		"worker_spec_synthesis",
		"execute",
		"verify",
		"handoff",
	}
}

func PromptDir(root string) string {
	return filepath.Join(root, promptDir)
}

func PromptFiles() []string {
	return []string{
		"README.md",
		"orchestrator.md",
		"methodology.md",
		"propose.md",
		"packet.md",
		"tasks.md",
		"worker-spec.md",
		"dispatch-ticket.md",
		"worker-result.md",
		"apply.md",
		"verify.md",
		"archive.md",
		"planner-architecture.md",
		"planner-delivery.md",
		"planner-risk.md",
		"judge.md",
		"judge-tools.md",
	}
}

func PromptRefs(root string) map[string]string {
	dir := PromptDir(root)
	return map[string]string{
		"promptDir":                dir,
		"runtimeReadme":            filepath.Join(dir, "README.md"),
		"orchestratorPrompt":       filepath.Join(dir, "orchestrator.md"),
		"methodologyGuide":         filepath.Join(dir, "methodology.md"),
		"packetWorkflow":           filepath.Join(dir, "propose.md"),
		"orchestrationPacketGuide": filepath.Join(dir, "packet.md"),
		"tasksGuide":               filepath.Join(dir, "tasks.md"),
		"workerSpecGuide":          filepath.Join(dir, "worker-spec.md"),
		"dispatchTicketGuide":      filepath.Join(dir, "dispatch-ticket.md"),
		"workerResultGuide":        filepath.Join(dir, "worker-result.md"),
		"applyWorkflow":            filepath.Join(dir, "apply.md"),
		"verifyWorkflow":           filepath.Join(dir, "verify.md"),
		"archiveWorkflow":          filepath.Join(dir, "archive.md"),
		"plannerArchitecture":      filepath.Join(dir, "planner-architecture.md"),
		"plannerDelivery":          filepath.Join(dir, "planner-delivery.md"),
		"plannerRisk":              filepath.Join(dir, "planner-risk.md"),
		"judgePrompt":              filepath.Join(dir, "judge.md"),
		"judgeToolsGuide":          filepath.Join(dir, "judge-tools.md"),
	}
}

func DefaultPacketSynthesisLoop(root string) PacketSynthesisLoop {
	promptsDir := PromptDir(root)
	return PacketSynthesisLoop{
		PlannerCount: 3,
		Planners: []PlannerAgent{
			{
				ID:        "packet-architecture",
				Name:      "Packet Planner A",
				Focus:     "Architecture fit, authority boundaries, and bounded change shape",
				PromptRef: filepath.Join(promptsDir, "planner-architecture.md"),
			},
			{
				ID:        "packet-delivery",
				Name:      "Packet Planner B",
				Focus:     "Incremental delivery, worker-spec slicing, and dependency order",
				PromptRef: filepath.Join(promptsDir, "planner-delivery.md"),
			},
			{
				ID:        "packet-risk",
				Name:      "Packet Planner C",
				Focus:     "Risk, verification, rollback, noop-validation, and phase-1 control-plane fit",
				PromptRef: filepath.Join(promptsDir, "planner-risk.md"),
			},
		},
		Judge: JudgeAgent{
			ID:        "packet-judge",
			Name:      "Packet Judge",
			Focus:     "Compile one execution-ready task graph from planner outputs, shared spec, and dependency lineage",
			PromptRef: filepath.Join(promptsDir, "judge.md"),
			SkillPath: filepath.Join(root, "skills", "judge-task-compiler", "SKILL.md"),
			ActiveSkills: []string{
				"judge-task-compiler",
				"blueprint-architect",
			},
			SkillHints: []string{
				"normalize planner outputs before deciding execution tasks",
				"freeze shared spec and constraints before fanout",
				"split same-type repeated work into one object per worker whenever the object roster is known",
				"keep worker handoff to background, constraints, shared spec, current task body, and closeout requirements",
			},
			CoreCapabilities: []string{
				"blueprint_decomposition",
				"swarm_assembly",
				"lineage_orchestration",
			},
			ToolContracts: []JudgeToolContract{
				{
					ID:      "collect_b3ehive_outputs",
					Purpose: "collect planner and swarm outputs into clean, source-tagged inputs for judging",
					When:    "before blueprint decomposition when multiple agent outputs or partial artifacts exist",
					Output:  "normalized planner outputs with sourceAgent, input summary, and evidence handles",
					Source:  filepath.Join(promptsDir, "judge-tools.md"),
				},
				{
					ID:      "extract_spec_constraints",
					Purpose: "extract roster, file contract, schema fields, source policy, and hard constraints",
					When:    "when the request or planner outputs contain reusable shared requirements",
					Output:  "shared spec object and constraint bundle ready for packet.sharedContext",
					Source:  filepath.Join(promptsDir, "judge-tools.md"),
				},
				{
					ID:      "synthesize_task_graph",
					Purpose: "compile shared spec into execution tasks, dependencies, and fanout groups",
					When:    "after roster or entity scope is known well enough to materialize dispatchable work",
					Output:  "task list, DAG edges, parallel groups, and orchestration expansion decision",
					Source:  filepath.Join(promptsDir, "judge-tools.md"),
				},
				{
					ID:      "validate_dispatch_contracts",
					Purpose: "ensure every execution task has clear inputs, outputs, done criteria, and evidence requirements",
					When:    "before finalizing finalPacket and finalWorkerSpecCandidates",
					Output:  "validated dispatch contracts with missing-field or ambiguity findings",
					Source:  filepath.Join(promptsDir, "judge-tools.md"),
				},
			},
			Dimensions: []string{
				"packet_clarity",
				"repo_fit",
				"execution_feasibility",
				"verification_completeness",
				"rollback_risk",
			},
		},
		PacketFields: []string{
			"objective",
			"constraints",
			"flowSelection",
			"policyTagsApplied",
			"selectedPlan",
			"rejectedAlternatives",
			"sharedContext",
			"executionTasks",
			"verificationPlan",
			"decisionRationale",
			"ownedPaths",
			"taskBudgets",
			"acceptanceMarkers",
			"replanTriggers",
			"rollbackHints",
		},
		WorkerSpecFields: []string{
			"taskId",
			"objective",
			"sharedContextPath",
			"sharedContext",
			"constraints",
			"ownedPaths",
			"blockedPaths",
			"taskBudget",
			"acceptanceMarkers",
			"verificationPlan",
			"replanTriggers",
			"rollbackHints",
		},
	}
}

func DefaultMethodologyContract(root string, reasonCodes []string) MethodologyContract {
	return MethodologyContract{
		Mode:      "qiushi-inspired fact-first / focus-first / verify-first discipline",
		GuidePath: filepath.Join(PromptDir(root), "methodology.md"),
		CoreRules: []string{
			"investigate before deciding when evidence is weak",
			"concentrate on one bounded main slice instead of a blended oversized plan",
			"verify with concrete evidence before claiming success",
			"close out honestly: name what changed, what was verified, and what remains risky",
		},
		ActiveLenses: methodologyLenses(reasonCodes),
		ActiveSkills: activeSkills(reasonCodes),
	}
}

func DefaultJudgeDecision(loop PacketSynthesisLoop, methodology MethodologyContract, reasonCodes []string) JudgeDecision {
	flow := selectedFlow(reasonCodes)
	rationale := []string{
		"prefer one bounded packet that fits the repo and can be verified cleanly",
		"favor rollback safety and verification completeness over broad blended execution",
	}
	winnerStrategy := "bounded winner chosen by repo fit, execution feasibility, verification completeness, and rollback safety"
	codes := uniqueStrings(reasonCodes)
	if containsString(codes, "policy_bug_rca_first") {
		rationale = append(rationale, "require failure evidence and one active hypothesis before implementation")
	}
	if containsString(codes, "policy_harness_state_first") {
		rationale = append(rationale, "inspect control plane, execution plane, and operator plane before changing harness-oriented surfaces")
	}
	if containsString(codes, "policy_log_compact_first") {
		rationale = append(rationale, "prefer compact log and hot-state evidence before expanding into raw runner logs")
	}
	if containsString(codes, "policy_options_before_plan") {
		rationale = append(rationale, "compare options before narrowing to one execution packet")
		winnerStrategy = "options-first winner narrowed to one bounded packet after trade-off comparison"
	}
	if containsString(codes, "policy_resume_state_first") {
		rationale = append(rationale, "require state and session inspection before resuming execution")
	}
	if containsString(codes, "policy_review_if_multi_file_or_high_risk") {
		rationale = append(rationale, "require review evidence before done when scope or risk rises")
	}
	return JudgeDecision{
		JudgeID:            loop.Judge.ID,
		JudgeName:          loop.Judge.Name,
		SelectedFlow:       flow,
		WinnerStrategy:     winnerStrategy,
		Rationale:          rationale,
		SelectedDimensions: append([]string(nil), loop.Judge.Dimensions...),
		SelectedLensIDs:    methodologyLensIDs(methodology.ActiveLenses),
		ReviewRequired:     containsString(codes, "policy_review_if_multi_file_or_high_risk"),
		VerifyRequired:     true,
	}
}

func DefaultExecutionLoopContract(root string, reasonCodes []string) ExecutionLoopContract {
	rules := []string{
		"investigate with real repo facts before editing when evidence is weak",
		"execute one bounded slice at a time instead of blending multiple goals",
		"verify with command/file evidence before claiming success",
		"if verify or closeout fails, move to analysis and then re-enter execution",
	}
	hints := []string{
		"skill docs are entry guidance; runtime contracts and worker prompt remain authoritative",
	}
	codes := uniqueStrings(reasonCodes)
	if containsString(codes, "policy_bug_rca_first") {
		rules = append(rules, "for bug flows, confirm evidence and one active hypothesis before changing code")
	}
	if containsString(codes, "policy_resume_state_first") {
		rules = append(rules, "for resume flows, inspect state, sessions, and prior artifacts before continuing")
		hints = append(hints, "prefer hot state, compact logs, and prior artifacts before resuming execution")
	}
	if containsString(codes, "policy_log_compact_first") {
		hints = append(hints, "prefer compact log and index surfaces before falling back to raw runner logs")
	}
	if containsString(codes, "policy_harness_state_first") {
		hints = append(hints, "inspect control plane first, then execution plane, then operator plane for harness-oriented tasks")
	}
	if containsString(codes, "policy_operator_surface_required") {
		hints = append(hints, "close out with operator-visible overview, watch, or metrics surfaces when requested")
	}
	if containsString(codes, "policy_review_if_multi_file_or_high_risk") {
		rules = append(rules, "for high-risk or multi-file work, include a short review pass before closeout")
	}
	return ExecutionLoopContract{
		Mode:            "qiushi execution / validation loop",
		Owner:           "worker + verify + runtime closeout",
		SkillPath:       filepath.Join(root, "skills", "qiushi-execution", "SKILL.md"),
		ActiveSkills:    activeSkills(reasonCodes),
		SkillHints:      uniqueStrings(hints),
		Phases:          []string{"investigate", "execute", "verify", "closeout", "analysis", "re-execute"},
		CoreRules:       rules,
		RetryTransition: "verify_not_passed -> analysis.required -> needs_replan -> next dispatch",
	}
}

func DefaultConstraintSystem(root string, reasonCodes []string) ConstraintSystem {
	rules := []ConstraintRule{
		{
			ID:           "planning-objective-bounded-packet",
			Layer:        "planning",
			Category:     "objective",
			Enforcement:  "soft",
			Level:        "enforced",
			TargetSignal: "packet.selectedPlan",
			Rule:         "planning must converge to one bounded packet and task slice instead of blended broad execution",
			Source:       filepath.Join(PromptDir(root), "orchestrator.md"),
		},
		{
			ID:           "planning-boundary-research-only",
			Layer:        "planning",
			Category:     "boundary",
			Enforcement:  "soft",
			Level:        "enforced",
			TargetSignal: "planner/judge output",
			Rule:         "planning may inspect requirements, code, state, logs, artifacts, and external references, but must stop at orchestration output",
			Source:       filepath.Join(PromptDir(root), "methodology.md"),
		},
		{
			ID:               "planning-format-json-schema",
			Layer:            "planning",
			Category:         "format",
			Enforcement:      "hard",
			Level:            "enforced",
			TargetSignal:     "planner/judge payloads",
			VerificationMode: "schema_validation",
			Rule:             "planner and judge outputs must follow the shared exact JSON schema without markdown fences",
			Source:           filepath.Join(PromptDir(root), "README.md"),
		},
		{
			ID:               "execution-boundary-owned-paths",
			Layer:            "execution",
			Category:         "boundary",
			Enforcement:      "hard",
			Level:            "enforced",
			TargetSignal:     "changedPaths",
			VerificationMode: "owned_path_audit",
			Rule:             "execution must stay inside ownedPaths and must not mutate global control-plane ledgers",
			Source:           filepath.Join(PromptDir(root), "apply.md"),
		},
		{
			ID:           "execution-process-required-reads",
			Layer:        "execution",
			Category:     "process",
			Enforcement:  "soft",
			Level:        "enforced",
			TargetSignal: "worker prompt read order",
			Rule:         "worker must read dispatch ticket, worker spec, and planning trace before editing; feedback summary is read only when recent failures exist",
			Source:       filepath.Join(PromptDir(root), "apply.md"),
		},
		{
			ID:               "execution-format-closeout-artifacts",
			Layer:            "execution",
			Category:         "format",
			Enforcement:      "hard",
			Level:            "enforced",
			TargetSignal:     "artifactDir contents",
			VerificationMode: "closeout_artifact_gate",
			Rule:             "execution closeout must write worker-result.json, verify.json, and handoff.md",
			Source:           filepath.Join(PromptDir(root), "worker-result.md"),
		},
		{
			ID:               "verification-evidence-required",
			Layer:            "verification",
			Category:         "evidence",
			Enforcement:      "hard",
			Level:            "enforced",
			TargetSignal:     "verify summary and completion gate",
			VerificationMode: "verify_evidence_gate",
			Rule:             "verification requires command, file, diff, or runtime evidence for every success claim",
			Source:           filepath.Join(PromptDir(root), "verify.md"),
		},
		{
			ID:               "verification-escalation-analysis-loop",
			Layer:            "verification",
			Category:         "escalation",
			Enforcement:      "hard",
			Level:            "enforced",
			TargetSignal:     "followUpEvent",
			VerificationMode: "runtime_followup_gate",
			Rule:             "verify or closeout failure must re-enter analysis.required instead of claiming completion",
			Source:           filepath.Join(PromptDir(root), "verify.md"),
		},
		{
			ID:               "runtime-process-route-first",
			Layer:            "runtime",
			Category:         "process",
			Enforcement:      "hard",
			Level:            "enforced",
			TargetSignal:     "dispatch readiness",
			VerificationMode: "route_gate",
			Rule:             "runtime remains route-first-dispatch-second; workers may not recreate the outer planner loop",
			Source:           filepath.Join(PromptDir(root), "README.md"),
		},
		{
			ID:           "learning-process-progressive-promotion",
			Layer:        "learning",
			Category:     "process",
			Enforcement:  "soft",
			Level:        "suggested",
			TargetSignal: "feedback-summary and learning state",
			Rule:         "repeated failures should first be observed, then suggested, and only later promoted into enforced constraints",
			Source:       filepath.Join(PromptDir(root), "methodology.md"),
		},
		{
			ID:               "evolution-owned-paths-nonempty-after-path-conflict",
			Layer:            "evolution",
			Category:         "failure_tightening",
			Enforcement:      "soft",
			Level:            "enforced",
			TargetSignal:     "feedback-summary recentFailures[path_conflict] x>=2",
			VerificationMode: "owned_path_nonempty_gate",
			Rule:             "when repeated path_conflict is observed, require worker-result.json.changedPaths to be non-empty and remain inside ownedPaths",
			Source:           filepath.Join(PromptDir(root), "methodology.md"),
		},
	}
	codes := uniqueStrings(reasonCodes)
	if containsString(codes, "policy_bug_rca_first") {
		rules = append(rules, ConstraintRule{
			ID:           "planning-process-rca-first",
			Layer:        "planning",
			Category:     "process",
			Enforcement:  "soft",
			Level:        "enforced",
			TargetSignal: "bug flow packet",
			Rule:         "bug flows must preserve one active hypothesis and failure evidence before implementation",
			Source:       filepath.Join(PromptDir(root), "methodology.md"),
		})
	}
	if containsString(codes, "policy_resume_state_first") {
		rules = append(rules, ConstraintRule{
			ID:           "execution-process-resume-state-first",
			Layer:        "execution",
			Category:     "process",
			Enforcement:  "soft",
			Level:        "enforced",
			TargetSignal: "resume flow",
			Rule:         "resume-sensitive execution must inspect hot state, session bindings, and recent failure memory before continuing",
			Source:       filepath.Join(PromptDir(root), "methodology.md"),
		})
	}
	if containsString(codes, "policy_review_if_multi_file_or_high_risk") {
		rules = append(rules, ConstraintRule{
			ID:               "verification-evidence-review-required",
			Layer:            "verification",
			Category:         "evidence",
			Enforcement:      "hard",
			Level:            "enforced",
			TargetSignal:     "multi-file or high-risk changes",
			VerificationMode: "review_evidence_gate",
			Rule:             "multi-file or high-risk work requires a short review pass before done",
			Source:           filepath.Join(PromptDir(root), "verify.md"),
		})
	}
	return ConstraintSystem{
		Mode:       "two-level layered constraints",
		Objective:  "calibrate distilled key signals instead of blindly enforcing every discovered condition",
		Generation: "progressive generation from execution and verification loops",
		Rules:      rules,
	}
}

func methodologyLenses(reasonCodes []string) []MethodologyLens {
	lenses := []MethodologyLens{
		{
			ID:          "investigation-first",
			Name:        "Investigation First",
			Trigger:     "default discipline whenever evidence is thin or repo state is unclear",
			Effect:      "collect repo/runtime facts before deciding route, judge outcome, or worker action",
			Stage:       "route",
			HookifyHint: "warn before speculative planning",
		},
		{
			ID:          "concentrate-forces",
			Name:        "Concentrate Forces",
			Trigger:     "default discipline for packet judging and task slicing",
			Effect:      "prefer one bounded, highest-leverage slice instead of a blended oversized task",
			Stage:       "judge",
			HookifyHint: "block diffuse task expansion",
		},
		{
			ID:          "practice-cognition",
			Name:        "Practice Before Claim",
			Trigger:     "default discipline for execution and verify",
			Effect:      "move from plan to action quickly, then validate with concrete command/file evidence",
			Stage:       "worker",
			HookifyHint: "block completion without evidence",
		},
		{
			ID:          "criticism-self-criticism",
			Name:        "Honest Closeout",
			Trigger:     "default discipline for verify and handoff",
			Effect:      "record what changed, what was checked, and what remains risky instead of hiding gaps",
			Stage:       "closeout",
			HookifyHint: "stop checklist before terminal outcome",
		},
	}
	codes := uniqueStrings(reasonCodes)
	if containsString(codes, "policy_bug_rca_first") {
		lenses = append(lenses, MethodologyLens{
			ID:          "rca-first",
			Name:        "RCA First",
			Trigger:     "reasonCodes include policy_bug_rca_first",
			Effect:      "capture failure evidence and keep one active hypothesis before editing",
			Stage:       "route",
			HookifyHint: "warn or block quick fixes without evidence",
		})
	}
	if containsString(codes, "policy_options_before_plan") {
		lenses = append(lenses, MethodologyLens{
			ID:          "options-before-plan",
			Name:        "Options Before Plan",
			Trigger:     "reasonCodes include policy_options_before_plan",
			Effect:      "compare 2 to 3 viable approaches before turning one winner into a packet",
			Stage:       "judge",
			HookifyHint: "checklist before narrowing to one packet",
		})
	}
	if containsString(codes, "policy_resume_state_first") {
		lenses = append(lenses, MethodologyLens{
			ID:          "state-first-resume",
			Name:        "State First Resume",
			Trigger:     "reasonCodes include policy_resume_state_first",
			Effect:      "read hot state, session bindings, and compact logs before assuming the next action",
			Stage:       "route",
			HookifyHint: "warn before resume without state inspection",
		})
	}
	if containsString(codes, "policy_harness_state_first") {
		lenses = append(lenses, MethodologyLens{
			ID:          "harness-state-first",
			Name:        "Harness State First",
			Trigger:     "reasonCodes include policy_harness_state_first",
			Effect:      "inspect control plane first, then execution plane, then operator plane before editing harness-oriented tasks",
			Stage:       "route",
			HookifyHint: "warn before editing harness tasks without state inspection",
		})
	}
	if containsString(codes, "policy_log_compact_first") {
		lenses = append(lenses, MethodologyLens{
			ID:          "compact-log-first",
			Name:        "Compact Log First",
			Trigger:     "reasonCodes include policy_log_compact_first",
			Effect:      "prefer compact log and hot state surfaces before raw runner logs",
			Stage:       "worker",
			HookifyHint: "warn before broad raw log scans",
		})
	}
	if containsString(codes, "policy_operator_surface_required") {
		lenses = append(lenses, MethodologyLens{
			ID:          "operator-surface-required",
			Name:        "Operator Surface Required",
			Trigger:     "reasonCodes include policy_operator_surface_required",
			Effect:      "ensure overview, watch, or metrics-facing surfaces are part of closeout when explicitly requested",
			Stage:       "closeout",
			HookifyHint: "warn before closing without operator-visible outputs",
		})
	}
	if containsString(codes, "policy_review_if_multi_file_or_high_risk") {
		lenses = append(lenses, MethodologyLens{
			ID:          "review-before-done",
			Name:        "Review Before Done",
			Trigger:     "reasonCodes include policy_review_if_multi_file_or_high_risk",
			Effect:      "require a short review pass when change scope or risk rises",
			Stage:       "closeout",
			HookifyHint: "final checklist before completion",
		})
	}
	return lenses
}

func RenderPlanningTrace(taskID, threadKey string, planEpoch int, resumeStrategy, routingModel, executionModel string, promptStages, reasonCodes []string, loop PacketSynthesisLoop) string {
	methodology := DefaultMethodologyContract(".", reasonCodes)
	judgeDecision := DefaultJudgeDecision(loop, methodology, reasonCodes)
	executionLoop := DefaultExecutionLoopContract(".", reasonCodes)
	constraints := DefaultConstraintSystem(".", reasonCodes)
	lines := []string{
		fmt.Sprintf("# Planning Trace for %s", taskID),
		"",
		"This file makes the planning layer visible for one concrete dispatch.",
		"The current runtime persists B3Ehive as packet-synthesis metadata, not as four separate runtime dispatches.",
		"",
		"## Binding",
		fmt.Sprintf("- taskId: %s", taskID),
		fmt.Sprintf("- threadKey: %s", threadKey),
		fmt.Sprintf("- planEpoch: %d", planEpoch),
		fmt.Sprintf("- resumeStrategy: %s", resumeStrategy),
		fmt.Sprintf("- routingModel: %s", routingModel),
		fmt.Sprintf("- executionModel: %s", executionModel),
	}
	if len(promptStages) > 0 {
		lines = append(lines, fmt.Sprintf("- promptStages: %s", strings.Join(promptStages, " -> ")))
	}
	if len(reasonCodes) > 0 {
		lines = append(lines, fmt.Sprintf("- reasonCodes: %s", strings.Join(reasonCodes, ", ")))
	}
	lines = append(lines,
		"",
		"## Methodology Layer",
		fmt.Sprintf("- mode: %s", methodology.Mode),
		fmt.Sprintf("- guidePath: %s", methodology.GuidePath),
	)
	for _, rule := range methodology.CoreRules {
		lines = append(lines, fmt.Sprintf("- rule: %s", rule))
	}
	if len(methodology.ActiveLenses) > 0 {
		lines = append(lines, "", "## Active Methodology Lenses")
		for _, lens := range methodology.ActiveLenses {
			lines = append(lines,
				fmt.Sprintf("- %s (%s)", lens.Name, lens.ID),
				fmt.Sprintf("  stage: %s", lens.Stage),
				fmt.Sprintf("  trigger: %s", lens.Trigger),
				fmt.Sprintf("  effect: %s", lens.Effect),
			)
		}
	}
	lines = append(lines,
		"",
		"## B3Ehive Packet Synthesis",
		fmt.Sprintf("- mode: metadata-backed packet synthesis (%d planners + 1 judge)", loop.PlannerCount),
		"- runtime fan-out: not materialized as separate dispatch tickets in the current runtime",
		"- observable result: chosen packet fields and worker-spec are persisted in the dispatch ticket",
		"",
		"## Planner Roles",
	)
	for index, planner := range loop.Planners {
		lines = append(lines,
			fmt.Sprintf("%d. %s (%s)", index+1, planner.Name, planner.ID),
			fmt.Sprintf("   focus: %s", planner.Focus),
			fmt.Sprintf("   promptRef: %s", planner.PromptRef),
		)
	}
	lines = append(lines,
		"",
		"## Judge Role",
		fmt.Sprintf("- %s (%s)", loop.Judge.Name, loop.Judge.ID),
		fmt.Sprintf("- focus: %s", loop.Judge.Focus),
		fmt.Sprintf("- promptRef: %s", loop.Judge.PromptRef),
	)
	if loop.Judge.SkillPath != "" {
		lines = append(lines, fmt.Sprintf("- skillPath: %s", loop.Judge.SkillPath))
	}
	if len(loop.Judge.ActiveSkills) > 0 {
		lines = append(lines, fmt.Sprintf("- activeSkills: %s", strings.Join(loop.Judge.ActiveSkills, ", ")))
	}
	if len(loop.Judge.CoreCapabilities) > 0 {
		lines = append(lines, fmt.Sprintf("- coreCapabilities: %s", strings.Join(loop.Judge.CoreCapabilities, ", ")))
	}
	for _, hint := range loop.Judge.SkillHints {
		lines = append(lines, fmt.Sprintf("- skillHint: %s", hint))
	}
	for _, contract := range loop.Judge.ToolContracts {
		lines = append(lines,
			fmt.Sprintf("- toolContract: %s", contract.ID),
			fmt.Sprintf("  purpose: %s", contract.Purpose),
			fmt.Sprintf("  when: %s", contract.When),
			fmt.Sprintf("  output: %s", contract.Output),
			fmt.Sprintf("  source: %s", contract.Source),
		)
	}
	if len(loop.Judge.Dimensions) > 0 {
		lines = append(lines, fmt.Sprintf("- dimensions: %s", strings.Join(loop.Judge.Dimensions, ", ")))
	}
	lines = append(lines,
		"",
		"## Judge Decision",
		fmt.Sprintf("- selectedFlow: %s", judgeDecision.SelectedFlow),
		fmt.Sprintf("- winnerStrategy: %s", judgeDecision.WinnerStrategy),
		fmt.Sprintf("- selectedDimensions: %s", strings.Join(judgeDecision.SelectedDimensions, ", ")),
		fmt.Sprintf("- selectedLensIds: %s", strings.Join(judgeDecision.SelectedLensIDs, ", ")),
		fmt.Sprintf("- reviewRequired: %t", judgeDecision.ReviewRequired),
		fmt.Sprintf("- verifyRequired: %t", judgeDecision.VerifyRequired),
	)
	for _, item := range judgeDecision.Rationale {
		lines = append(lines, fmt.Sprintf("- rationale: %s", item))
	}
	lines = append(lines,
		"",
		"## Execution Validation Loop",
		fmt.Sprintf("- mode: %s", executionLoop.Mode),
		fmt.Sprintf("- owner: %s", executionLoop.Owner),
		fmt.Sprintf("- skillPath: %s", executionLoop.SkillPath),
		fmt.Sprintf("- phases: %s", strings.Join(executionLoop.Phases, " -> ")),
		fmt.Sprintf("- retryTransition: %s", executionLoop.RetryTransition),
	)
	for _, rule := range executionLoop.CoreRules {
		lines = append(lines, fmt.Sprintf("- rule: %s", rule))
	}
	lines = append(lines,
		"",
		"## Layered Constraints",
		fmt.Sprintf("- mode: %s", constraints.Mode),
		fmt.Sprintf("- objective: %s", constraints.Objective),
		fmt.Sprintf("- generation: %s", constraints.Generation),
	)
	for _, rule := range constraints.Rules {
		mode := rule.VerificationMode
		if mode == "" {
			mode = "prompt_guidance"
		}
		lines = append(lines,
			fmt.Sprintf("- [%s/%s/%s/%s] %s", rule.Layer, rule.Category, rule.Enforcement, rule.Level, rule.ID),
			fmt.Sprintf("  signal: %s", rule.TargetSignal),
			fmt.Sprintf("  verificationMode: %s", mode),
			fmt.Sprintf("  rule: %s", rule.Rule),
		)
	}
	lines = append(lines,
		"",
		"## Execution Handoff",
		"- context_assembly -> packet_parallel_planning -> packet_judging -> worker_spec_synthesis -> execute -> verify -> handoff",
		"- worker receives the selected packet shape through dispatch-ticket + worker-spec",
	)
	return strings.Join(lines, "\n") + "\n"
}

func DefaultTopLevelPrompt(root, userPrompt string) string {
	base := loadPromptOrFallback(
		filepath.Join(PromptDir(root), "orchestrator.md"),
		`You are the Klein orchestration agent.

The repo-local runtime owns the outer loop:
- submit -> classify -> fuse -> bind -> route -> issue dispatch ticket -> ingest outcome -> verify -> refresh summaries

When orchestration packet synthesis is needed, use the default b3e convergence subunit:
- run 3 isolated planners that each produce one orchestration packet candidate plus task-local worker-spec candidates
- have 1 judge select and format the final runtime-owned packet`,
	)
	lines := []string{
		strings.TrimSpace(base),
		"",
		"Load supporting runtime prompts from prompts/spec in this order when relevant:",
	}
	for _, file := range PromptFiles() {
		if file == "README.md" || file == "orchestrator.md" {
			continue
		}
		lines = append(lines, fmt.Sprintf("- prompts/spec/%s", file))
	}
	lines = append(lines,
		"",
		"User requirement:",
		strings.TrimSpace(userPrompt),
		"",
		"Final orchestration packet must include:",
		"- objective",
		"- constraints",
		"- flowSelection",
		"- policyTagsApplied",
		"- selectedPlan",
		"- rejectedAlternatives",
		"- executionTasks",
		"- verificationPlan",
		"- decisionRationale",
		"- ownedPaths",
		"- taskBudgets",
		"- acceptanceMarkers",
		"- replanTriggers",
		"- rollbackHints",
	)
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func loadPromptOrFallback(path, fallback string) string {
	payload, err := os.ReadFile(path)
	if err != nil {
		return strings.TrimSpace(fallback)
	}
	text := strings.TrimSpace(string(payload))
	if text == "" {
		return strings.TrimSpace(fallback)
	}
	return text
}

func activeSkills(reasonCodes []string) []string {
	codes := uniqueStrings(reasonCodes)
	skills := []string{"qiushi-execution"}
	if containsString(codes, "policy_bug_rca_first") {
		skills = append(skills, "systematic-debugging")
	}
	if containsString(codes, "policy_options_before_plan") {
		skills = append(skills, "blueprint-architect")
	}
	if containsString(codes, "policy_resume_state_first") || containsString(codes, "policy_log_compact_first") {
		skills = append(skills, "harness-log-search-cskill")
	}
	return uniqueStrings(skills)
}

func uniqueStrings(values []string) []string {
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

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func selectedFlow(reasonCodes []string) string {
	codes := uniqueStrings(reasonCodes)
	switch {
	case containsString(codes, "policy_harness_state_first"):
		return "harness-state-first packet"
	case containsString(codes, "policy_log_compact_first"):
		return "compact-log-first packet"
	case containsString(codes, "policy_bug_rca_first"):
		return "debugging-first bounded packet"
	case containsString(codes, "policy_options_before_plan"):
		return "options-first bounded packet"
	case containsString(codes, "policy_resume_state_first"):
		return "state-first resume packet"
	default:
		return "standard bounded delivery packet"
	}
}

func methodologyLensIDs(lenses []MethodologyLens) []string {
	ids := make([]string, 0, len(lenses))
	for _, lens := range lenses {
		if lens.ID == "" {
			continue
		}
		ids = append(ids, lens.ID)
	}
	return ids
}
