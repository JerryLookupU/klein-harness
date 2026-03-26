package worker

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"klein-harness/internal/adapter"
	"klein-harness/internal/dispatch"
	"klein-harness/internal/orchestration"
	"klein-harness/internal/verify"
)

type verificationManifest struct {
	Rules []verificationRule `json:"rules"`
}

type verificationRule struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Exec         string `json:"exec"`
	Timeout      int    `json:"timeout"`
	ReadOnlySafe bool   `json:"readOnlySafe"`
}

type DispatchBundle struct {
	TicketPath        string
	WorkerSpecPath    string
	PromptPath        string
	PlanningTracePath string
	ArtifactDir       string
}

func Prepare(root string, ticket dispatch.Ticket, leaseID string) (DispatchBundle, error) {
	paths, err := adapter.Resolve(root)
	if err != nil {
		return DispatchBundle{}, err
	}
	task, err := adapter.LoadTask(root, ticket.TaskID)
	if err != nil {
		return DispatchBundle{}, err
	}
	projectMeta, err := adapter.LoadProjectMeta(root)
	if err != nil {
		return DispatchBundle{}, err
	}
	verifyCommands, err := verificationCommands(paths.VerificationRulesPath, task.VerificationRuleIDs)
	if err != nil {
		return DispatchBundle{}, err
	}
	hookPlan := verify.BuildHookPlan(root, task, ticket, verifyCommands)
	feedbackSummary, _ := verify.LoadFeedbackSummary(root)
	taskFeedback, hasTaskFeedback := verify.CurrentTaskFeedback(feedbackSummary, task.TaskID)
	executionCwd := adapter.TaskCWD(paths, task)
	artifactDir := filepath.Join(paths.ArtifactsDir, task.TaskID, ticket.DispatchID)
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		return DispatchBundle{}, err
	}
	ticketPath := filepath.Join(paths.StateDir, fmt.Sprintf("dispatch-ticket-%s.json", task.TaskID))
	workerSpecPath := filepath.Join(artifactDir, "worker-spec.json")
	promptPath := filepath.Join(paths.StateDir, fmt.Sprintf("runner-prompt-%s.md", task.TaskID))
	planningTracePath := filepath.Join(paths.StateDir, fmt.Sprintf("planning-trace-%s.md", task.TaskID))
	repoRole := projectMeta.RepoRole
	if repoRole == "" {
		repoRole = "target_repo"
	}
	directTargetEditAllowed := true
	if projectMeta.DirectTargetEditAllowed != nil {
		directTargetEditAllowed = *projectMeta.DirectTargetEditAllowed
	}
	intentFingerprint := stableFingerprint(
		task.TaskID,
		fmt.Sprintf("%d", task.PlanEpoch),
		task.Title,
		task.Summary,
		strings.Join(task.OwnedPaths, "|"),
		strings.Join(task.VerificationRuleIDs, "|"),
	)
	workerSpec := map[string]any{
		"schemaVersion":     "kh.worker-spec.v1",
		"generator":         "kh-worker-supervisor",
		"generatedAt":       nowUTC(),
		"dispatchId":        ticket.DispatchID,
		"taskId":            task.TaskID,
		"threadKey":         task.ThreadKey,
		"planEpoch":         task.PlanEpoch,
		"attempt":           ticket.Attempt,
		"reasonCodes":       unique(ticket.ReasonCodes),
		"policyTags":        policyTags(ticket.ReasonCodes),
		"objective":         coalesce(task.Summary, task.Title),
		"selectedPlan":      coalesce(task.Description, task.Summary, task.Title),
		"constraints":       taskConstraints(task),
		"ownedPaths":        unique(task.OwnedPaths),
		"blockedPaths":      unique(task.ForbiddenPaths),
		"taskBudget":        ticket.Budget,
		"acceptanceMarkers": hookPlan.AcceptanceMarkers,
		"verificationPlan": map[string]any{
			"ruleIds":  unique(task.VerificationRuleIDs),
			"commands": verifyCommands,
		},
		"validationHooks":   hookPlan.Hooks,
		"learningHints":     hookPlan.LearningHints,
		"outerLoopMemory":   taskFeedback,
		"decisionRationale": coalesce(task.Description, task.Summary),
		"replanTriggers": []string{
			"verification_failed",
			"acceptance_markers_missing",
			"owned_paths_conflict",
			"authority_boundary_conflict",
		},
		"rollbackHints": []string{
			"leave_task_local_artifacts_intact",
			"preserve_checkpoint_for_supervisor",
			"handoff_before_exit_when_blocked",
		},
	}
	if err := writeJSON(workerSpecPath, workerSpec); err != nil {
		return DispatchBundle{}, err
	}
	packetSynthesis := orchestration.DefaultPacketSynthesisLoop(paths.Root)
	methodology := orchestration.DefaultMethodologyContract(paths.Root, unique(ticket.ReasonCodes))
	judgeDecision := orchestration.DefaultJudgeDecision(packetSynthesis, methodology, unique(ticket.ReasonCodes))
	executionLoop := orchestration.DefaultExecutionLoopContract(paths.Root, unique(ticket.ReasonCodes))
	constraintSystem := orchestration.DefaultConstraintSystem(paths.Root, unique(ticket.ReasonCodes))
	planningTrace := orchestration.RenderPlanningTrace(
		task.TaskID,
		task.ThreadKey,
		task.PlanEpoch,
		task.ResumeStrategy,
		task.RoutingModel,
		task.ExecutionModel,
		unique(task.PromptStages),
		unique(ticket.ReasonCodes),
		packetSynthesis,
	)
	if err := os.WriteFile(planningTracePath, []byte(planningTrace), 0o644); err != nil {
		return DispatchBundle{}, err
	}
	dispatchTicket := map[string]any{
		"schemaVersion":           "kh.dispatch-ticket.v1",
		"generator":               "kh-worker-supervisor",
		"generatedAt":             nowUTC(),
		"dispatchId":              ticket.DispatchID,
		"idempotencyKey":          ticket.IdempotencyKey,
		"leaseId":                 leaseID,
		"taskId":                  task.TaskID,
		"threadKey":               task.ThreadKey,
		"planEpoch":               task.PlanEpoch,
		"attempt":                 ticket.Attempt,
		"intentFingerprint":       intentFingerprint,
		"taskKind":                task.Kind,
		"workerMode":              task.WorkerMode,
		"roleHint":                task.RoleHint,
		"repoRole":                repoRole,
		"directTargetEditAllowed": directTargetEditAllowed,
		"projectRoot":             paths.Root,
		"executionCwd":            executionCwd,
		"worktreePath":            coalesce(task.Dispatch.WorktreePath, task.WorktreePath),
		"branchName":              coalesce(task.Dispatch.BranchName, task.BranchName),
		"diffBase":                coalesce(task.Dispatch.DiffBase, task.DiffBase, task.BaseRef),
		"resumeStrategy":          task.ResumeStrategy,
		"sessionId":               ticket.ResumeSessionID,
		"routingModel":            task.RoutingModel,
		"executionModel":          task.ExecutionModel,
		"orchestrationSessionId":  task.OrchestrationSessionID,
		"promptStages":            unique(task.PromptStages),
		"reasonCodes":             unique(ticket.ReasonCodes),
		"policyTags":              policyTags(ticket.ReasonCodes),
		"allowedWriteGlobs":       unique(task.OwnedPaths),
		"blockedWriteGlobs":       unique(task.ForbiddenPaths),
		"artifactDir":             artifactDir,
		"planningTracePath":       planningTracePath,
		"workerSpecPath":          workerSpecPath,
		"workerSpec":              workerSpec,
		"artifacts": map[string]string{
			"workerSpec":   workerSpecPath,
			"workerResult": filepath.Join(artifactDir, "worker-result.json"),
			"verify":       filepath.Join(artifactDir, "verify.json"),
			"handoff":      filepath.Join(artifactDir, "handoff.md"),
		},
		"authorityBoundary": map[string]any{
			"routeFirstDispatchSecond":  true,
			"workerMayWriteGlobalState": false,
			"workerMayMergeOrArchive":   false,
			"completionOwnedByRuntime":  true,
			"completionGatePath":        filepath.Join(paths.StateDir, "completion-gate.json"),
		},
		"verification": map[string]any{
			"ruleIds":  unique(task.VerificationRuleIDs),
			"commands": verifyCommands,
		},
		"validationHooks":  hookPlan.Hooks,
		"learningHints":    hookPlan.LearningHints,
		"outerLoopMemory":  taskFeedback,
		"methodology":      methodology,
		"judgeDecision":    judgeDecision,
		"executionLoop":    executionLoop,
		"constraintSystem": constraintSystem,
		"packetSynthesis":  packetSynthesis,
		"runtimeRefs": mergeStringMaps(
			map[string]string{
				"promptRef":       ticket.PromptRef,
				"promptPath":      promptPath,
				"workerSpec":      workerSpecPath,
				"planningTrace":   planningTracePath,
				"feedbackSummary": filepath.Join(paths.StateDir, "feedback-summary.json"),
			},
			orchestration.PromptRefs(paths.Root),
		),
	}
	if err := writeJSON(ticketPath, dispatchTicket); err != nil {
		return DispatchBundle{}, err
	}
	var taskFeedbackPtr *verify.TaskFeedbackSummary
	if hasTaskFeedback {
		copy := taskFeedback
		taskFeedbackPtr = &copy
	}
	prompt := buildPrompt(ticketPath, workerSpecPath, planningTracePath, artifactDir, filepath.Join(paths.StateDir, "feedback-summary.json"), task, ticket, packetSynthesis, hookPlan, taskFeedbackPtr, constraintSystem)
	if err := os.WriteFile(promptPath, []byte(prompt), 0o644); err != nil {
		return DispatchBundle{}, err
	}
	return DispatchBundle{
		TicketPath:        ticketPath,
		WorkerSpecPath:    workerSpecPath,
		PromptPath:        promptPath,
		PlanningTracePath: planningTracePath,
		ArtifactDir:       artifactDir,
	}, nil
}

func verificationCommands(path string, ruleIDs []string) ([]map[string]any, error) {
	if len(ruleIDs) == 0 {
		return []map[string]any{}, nil
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []map[string]any{}, nil
		}
		return nil, err
	}
	var manifest verificationManifest
	if err := json.Unmarshal(payload, &manifest); err != nil {
		return nil, err
	}
	index := map[string]verificationRule{}
	for _, rule := range manifest.Rules {
		if rule.ID != "" {
			index[rule.ID] = rule
		}
	}
	commands := make([]map[string]any, 0, len(ruleIDs))
	for _, ruleID := range ruleIDs {
		rule, ok := index[ruleID]
		if !ok {
			continue
		}
		commands = append(commands, map[string]any{
			"ruleId":       rule.ID,
			"title":        rule.Title,
			"exec":         rule.Exec,
			"timeout":      rule.Timeout,
			"readOnlySafe": rule.ReadOnlySafe,
		})
	}
	return commands, nil
}

func buildPrompt(ticketPath, workerSpecPath, planningTracePath, artifactDir, feedbackSummaryPath string, task adapter.Task, ticket dispatch.Ticket, packetSynthesis orchestration.PacketSynthesisLoop, hookPlan verify.HookPlan, taskFeedback *verify.TaskFeedbackSummary, constraintSystem orchestration.ConstraintSystem) string {
	routePolicyTags := policyTags(ticket.ReasonCodes)
	lines := []string{
		"You are the Klein worker for exactly one bound task inside a repo-local closed-loop runtime.",
		"",
		"Required reads before execution:",
		fmt.Sprintf("1. Read the immutable dispatch ticket first: %s", ticketPath),
		fmt.Sprintf("2. Read the task-local worker spec: %s", workerSpecPath),
		fmt.Sprintf("3. Read the planning trace for the visible B3Ehive packet-synthesis contract: %s", planningTracePath),
		"4. If task-local artifacts already exist, read worker-result.json, verify.json, handoff.md, and referenced compact handoff logs.",
		"5. If feedback-summary exists and this task has recent failures, read only the current task's recent 3 high-severity failures before re-execution.",
		"6. After those reads, move to execution in owned paths. Do not keep expanding into prompt docs unless blocked on artifact format or verification wording.",
		"",
		"Hard authority rules:",
		"- Never create or mutate thread keys, request ids, task ids, plan epochs, leases, or global `.harness/state/*` ledgers.",
		"- Never edit files outside the bound worktree.",
		"- Never edit paths outside `allowedWriteGlobs`.",
		"- Never edit `blockedWriteGlobs`.",
		"- Never write task-local outputs outside `artifactDir`.",
		"- Never merge, rebase, push, archive, delete branches, or delete worktrees.",
		"- Never decide that the loop is complete. You may only decide the terminal outcome of this worker run.",
		"",
		"Execution defaults:",
		"- Fix root causes, not symptoms.",
		"- Keep changes minimal, focused, and consistent with the existing codebase.",
		"- Read a file before editing it.",
		"- Before each meaningful tool/action group, briefly state your immediate intent.",
		"- Treat the three required reads above as enough context for the normal case.",
		"- Use the planning trace as context, not as a request to run a second planning pass.",
		"- Do not skip directly from the request text to edits before reading the required inputs.",
		"- The outer runtime already owns submit -> route -> dispatch. Do not recreate planning or orchestration inside this task unless the ticket is internally inconsistent.",
		"",
		"Visible orchestration layer for this dispatch:",
		fmt.Sprintf("- packetSynthesisMode: metadata-backed B3Ehive (%d planners + 1 judge)", packetSynthesis.PlannerCount),
		"- observableRuntimeBehavior: one dispatch ticket + one worker execution; planner/judge roles are persisted as planning metadata for now",
	}
	for _, planner := range packetSynthesis.Planners {
		lines = append(lines, fmt.Sprintf("- planner: %s | %s | %s", planner.ID, planner.Name, planner.Focus))
	}
	lines = append(lines,
		fmt.Sprintf("- judge: %s | %s | %s", packetSynthesis.Judge.ID, packetSynthesis.Judge.Name, packetSynthesis.Judge.Focus),
		fmt.Sprintf("- planningTracePath: %s", planningTracePath),
		"",
		"Visible execution / validation loop for this dispatch:",
		"- executionLoopMode: qiushi execution / validation loop",
		fmt.Sprintf("- executionLoopSkill: %s", filepath.Join("skills", "qiushi-execution", "SKILL.md")),
		"- executionLoopPhases: investigate -> execute -> verify -> closeout -> analysis -> re-execute",
		"- executionLoopRule: if verify or closeout fails, return to analysis instead of faking completion",
		"",
	)
	if taskFeedback != nil && len(taskFeedback.RecentFailures) > 0 {
		lines = append(lines,
			"Outer-loop memory from verify/error sidecar:",
			fmt.Sprintf("- feedbackSummaryPath: %s", feedbackSummaryPath),
			fmt.Sprintf("- latestFailureType: %s", taskFeedback.LatestFeedbackType),
			fmt.Sprintf("- latestFailureMessage: %s", taskFeedback.LatestMessage),
		)
		if taskFeedback.LatestThinkingSummary != "" {
			lines = append(lines, fmt.Sprintf("- latestThinkingSummary: %s", taskFeedback.LatestThinkingSummary))
		}
		if taskFeedback.LatestNextAction != "" {
			lines = append(lines, fmt.Sprintf("- latestNextAction: %s", taskFeedback.LatestNextAction))
		}
		lines = append(lines, "- recentFailures: read these reminders instead of scanning the full feedback log")
		for _, failure := range taskFeedback.RecentFailures {
			lines = append(lines, fmt.Sprintf("  - %s | %s | %s | %s", failure.ID, failure.Step, failure.FeedbackType, failure.Message))
			if failure.ThinkingSummary != "" {
				lines = append(lines, fmt.Sprintf("    thought: %s", failure.ThinkingSummary))
			}
			if failure.NextAction != "" {
				lines = append(lines, fmt.Sprintf("    next: %s", failure.NextAction))
			}
		}
		lines = append(lines, "")
	}
	lines = append(lines, "Soft constraints appended after the base prompt:")
	for _, rule := range constraintSystem.Rules {
		if rule.Enforcement != "soft" {
			continue
		}
		if rule.Layer != "execution" && rule.Layer != "verification" && rule.Layer != "learning" {
			continue
		}
		lines = append(lines, fmt.Sprintf("- [%s/%s/%s/%s] %s", rule.Layer, rule.Category, rule.Enforcement, rule.Level, rule.Rule))
	}
	lines = append(lines, "", "Hard constraints verified item-by-item by runtime / verify:")
	for _, rule := range constraintSystem.Rules {
		if rule.Enforcement != "hard" {
			continue
		}
		if rule.Layer != "execution" && rule.Layer != "verification" && rule.Layer != "runtime" {
			continue
		}
		mode := rule.VerificationMode
		if mode == "" {
			mode = "runtime_gate"
		}
		lines = append(lines, fmt.Sprintf("- [%s/%s/%s/%s] %s | check=%s", rule.Layer, rule.Category, rule.Enforcement, rule.Level, rule.Rule, mode))
	}
	lines = append(lines, "")
	if len(routePolicyTags) > 0 {
		lines = append(lines,
			"Policy guardrails from route reasonCodes:",
			fmt.Sprintf("- reasonCodes: %s", strings.Join(unique(ticket.ReasonCodes), ", ")),
			fmt.Sprintf("- policyTags: %s", strings.Join(routePolicyTags, ", ")),
		)
		if contains(routePolicyTags, "policy_bug_rca_first") {
			lines = append(lines,
				"- Bug / failure flow: reproduce or capture concrete failure evidence before editing.",
				"- Keep one active hypothesis at a time and test it with the smallest discriminating step.",
				"- Do not apply or suggest quick fixes before the evidence supports a root cause.",
				"- After confirmation, prefer one minimal change tied to the confirmed cause.",
			)
		}
		if contains(routePolicyTags, "policy_options_before_plan") {
			lines = append(lines,
				"- Recommendation / compare flow: write 2 to 3 viable options with trade-offs first.",
				"- Make one recommendation before expanding into blueprint or implementation work.",
			)
		}
		if contains(routePolicyTags, "policy_resume_state_first") {
			lines = append(lines,
				"- Resume flow: read `AGENTS.md`, `.harness/state/current.json`, `.harness/state/runtime.json`, `.harness/state/request-summary.json`, `.harness/task-pool.json`, `.harness/session-registry.json`, and the relevant compact log before coding.",
				"- If active task state or prior handoff is unclear, stop and record that ambiguity instead of guessing.",
			)
		}
		lines = append(lines, "")
	}
	lines = append(lines,
		"On-demand references only when blocked on format:",
		fmt.Sprintf("- methodologyGuide: %s", filepath.Join("prompts", "spec", "methodology.md")),
		fmt.Sprintf("- applyWorkflow: %s", filepath.Join("prompts", "spec", "apply.md")),
		fmt.Sprintf("- verifyWorkflow: %s", filepath.Join("prompts", "spec", "verify.md")),
		fmt.Sprintf("- workerResultGuide: %s", filepath.Join("prompts", "spec", "worker-result.md")),
		"",
		"Hookified verification flow:",
	)
	for _, hook := range hookPlan.Hooks {
		lines = append(lines, fmt.Sprintf("- %s | event=%s | action=%s | status=%s", hook.Name, hook.Event, hook.Action, hook.Status))
		for _, item := range hook.Checklist {
			lines = append(lines, fmt.Sprintf("  - %s: %s (%s)", item.ID, item.Title, item.Status))
		}
	}
	if len(hookPlan.LearningHints) > 0 {
		lines = append(lines, "", "Learned reminders:")
		for _, hint := range hookPlan.LearningHints {
			lines = append(lines, "- "+hint)
		}
	}
	lines = append(lines,
		"",
		"Verification:",
		"- Run verify commands from the dispatch ticket in order.",
		"- Start with the narrowest relevant validation, then broader checks when required.",
		"- Record each command, exit code, and output path in verify.json.",
		"- A noop completion is valid only when acceptance is already satisfied and verify.json records concrete evidence for that claim.",
		"- Do not claim completion without command or file evidence that supports the claim.",
		"- When the run changes multiple files or touches high-risk control-plane surfaces, perform a short review pass and record the findings in verify.json or handoff.md.",
		"- Before exit, if any required closeout artifact is missing, stop editing and write the missing artifact first.",
		"",
		"Required artifacts before exit:",
		fmt.Sprintf("- %s", workerSpecPath),
		fmt.Sprintf("- %s", filepath.Join(artifactDir, "worker-result.json")),
		fmt.Sprintf("- %s", filepath.Join(artifactDir, "verify.json")),
		fmt.Sprintf("- %s", filepath.Join(artifactDir, "handoff.md")),
		"",
		"Task focus:",
		fmt.Sprintf("- taskId: %s", task.TaskID),
		fmt.Sprintf("- planEpoch: %d", task.PlanEpoch),
		fmt.Sprintf("- roleHint: %s", task.RoleHint),
		fmt.Sprintf("- taskKind: %s", task.Kind),
		fmt.Sprintf("- workerMode: %s", task.WorkerMode),
		fmt.Sprintf("- routingModel: %s", task.RoutingModel),
		fmt.Sprintf("- executionModel: %s", task.ExecutionModel),
		fmt.Sprintf("- orchestrationSessionId: %s", task.OrchestrationSessionID),
		fmt.Sprintf("- promptStages: %s", strings.Join(task.PromptStages, ", ")),
		fmt.Sprintf("- title: %s", task.Title),
		fmt.Sprintf("- summary: %s", task.Summary),
		fmt.Sprintf("- description: %s", task.Description),
		fmt.Sprintf("- ownedPaths: %s", strings.Join(task.OwnedPaths, ", ")),
		fmt.Sprintf("- verificationRuleIds: %s", strings.Join(task.VerificationRuleIDs, ", ")),
		fmt.Sprintf("- reasonCodes: %s", strings.Join(unique(ticket.ReasonCodes), ", ")),
		fmt.Sprintf("- policyTags: %s", strings.Join(routePolicyTags, ", ")),
		fmt.Sprintf("- promptRef: %s", ticket.PromptRef),
		"",
		"Final response:",
		"- Be brief.",
		"- Report only the terminal worker outcome and the key artifact path(s).",
		"- Do not claim global completion.",
	)
	return strings.Join(lines, "\n") + "\n"
}

func stableFingerprint(parts ...string) string {
	hash := sha256.Sum256([]byte(strings.Join(parts, "\x1f")))
	return hex.EncodeToString(hash[:16])
}

func unique(values []string) []string {
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

func policyTags(reasonCodes []string) []string {
	tags := make([]string, 0, len(reasonCodes))
	for _, code := range reasonCodes {
		if strings.HasPrefix(code, "policy_") {
			tags = append(tags, code)
		}
	}
	return unique(tags)
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func taskConstraints(task adapter.Task) []string {
	constraints := []string{
		"stay within task-local scope",
		"do not mutate global control-plane ledgers",
		"obey allowedWriteGlobs and blockedWriteGlobs",
		"leave merge, archive, and completion decisions to runtime",
	}
	if task.WorkerMode != "" {
		constraints = append(constraints, "workerMode="+task.WorkerMode)
	}
	if task.ResumeStrategy != "" {
		constraints = append(constraints, "resumeStrategy="+task.ResumeStrategy)
	}
	return constraints
}

func coalesce(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func nowUTC() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func writeJSON(path string, value any) error {
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(payload, '\n'), 0o644)
}

func mergeStringMaps(parts ...map[string]string) map[string]string {
	merged := map[string]string{}
	for _, part := range parts {
		for key, value := range part {
			merged[key] = value
		}
	}
	return merged
}
