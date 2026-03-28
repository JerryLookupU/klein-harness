package worker

import (
	"fmt"
	"path/filepath"
	"strings"

	"klein-harness/internal/adapter"
	"klein-harness/internal/dispatch"
	"klein-harness/internal/orchestration"
	"klein-harness/internal/verify"
)

type PromptCompileInput struct {
	TicketPath          string
	WorkerSpecPath      string
	AcceptedPacketPath  string
	TaskContractPath    string
	TaskGraphPath       string
	ContextLayersPath   string
	RequestContextPath  string
	RuntimeContextPath  string
	PlanningTracePath   string
	ConstraintPath      string
	SharedContextPath   string
	SharedFlowPath      string
	SliceContextPath    string
	VerifySkeletonPath  string
	HandoffContractPath string
	TakeoverPath        string
	ArtifactDir         string
	FeedbackSummaryPath string
	Task                adapter.Task
	Ticket              dispatch.Ticket
	ExecutionLoop       orchestration.ExecutionLoopContract
	HookPlan            verify.HookPlan
	TaskFeedback        *verify.TaskFeedbackSummary
	ConstraintSystem    orchestration.ConstraintSystem
	SharedFlowContext   orchestration.SharedFlowContext
	SliceContext        orchestration.SliceLocalContext
}

func CompileWorkerPrompt(input PromptCompileInput) string {
	lines := []string{
		"You are the execution agent for one compiled slice.",
		"",
		"Task background:",
		fmt.Sprintf("- objective: %s", coalesce(input.Task.Summary, input.Task.Title, input.Task.TaskID)),
		fmt.Sprintf("- orchestrationMode: %s", coalesce(input.Task.SOPID, "legacy-runtime-contract")),
		"- metadata-backed B3Ehive remains the orchestration backdrop; this prompt is the compiled execution handoff.",
		fmt.Sprintf("- activeSkills: %s", strings.Join(input.ExecutionLoop.ActiveSkills, ", ")),
		"- runtime-owned files inside .harness remain authoritative, but this prompt is the primary worker handoff.",
		"",
		"Shared constraints:",
		"Execution boundary:",
		fmt.Sprintf("- taskId: %s", input.Task.TaskID),
		fmt.Sprintf("- taskFamily: %s", input.Task.TaskFamily),
		fmt.Sprintf("- sopId: %s", input.Task.SOPID),
		fmt.Sprintf("- executionSliceId: %s", input.SliceContext.ExecutionSliceID),
		fmt.Sprintf("- title: %s", coalesce(input.SliceContext.Title, input.Task.Title, input.Task.TaskID)),
		fmt.Sprintf("- summary: %s", coalesce(input.SliceContext.Summary, input.Task.Summary, input.Task.Title)),
		fmt.Sprintf("- allowedWriteGlobs: %s", strings.Join(unique(input.SliceContext.AllowedWriteGlobs), ", ")),
		fmt.Sprintf("- forbiddenWriteGlobs: %s", strings.Join(unique(input.SliceContext.ForbiddenWriteGlobs), ", ")),
		"- Do not re-plan the full flow.",
		"- Do not mutate global .harness truth ledgers.",
		"- If shared flow context is incomplete, report planning drift instead of freelancing.",
		"",
		"Read order:",
		fmt.Sprintf("- %s", input.ContextLayersPath),
		fmt.Sprintf("- %s", input.SharedContextPath),
		fmt.Sprintf("- %s", input.SharedFlowPath),
		fmt.Sprintf("- %s", input.SliceContextPath),
		fmt.Sprintf("- %s", input.VerifySkeletonPath),
		fmt.Sprintf("- %s", input.HandoffContractPath),
		fmt.Sprintf("- %s", input.TaskContractPath),
		"",
		"Compiled context layers:",
		fmt.Sprintf("- requestContextPath: %s", input.RequestContextPath),
		fmt.Sprintf("- runtimeControlContextPath: %s", input.RuntimeContextPath),
		fmt.Sprintf("- contextLayersPath: %s", input.ContextLayersPath),
		"",
		"Shared spec:",
		fmt.Sprintf("- summary: %s", input.SharedFlowContext.Summary),
	}
	if input.SharedFlowContext.SharedSpecRef != "" {
		lines = append(lines, fmt.Sprintf("- sharedSpecRef: %s", input.SharedFlowContext.SharedSpecRef))
	}
	if input.SharedFlowContext.VariableRef != "" {
		lines = append(lines, fmt.Sprintf("- variableRef: %s", input.SharedFlowContext.VariableRef))
	}
	if input.SharedFlowContext.ScopeRef != "" {
		lines = append(lines, fmt.Sprintf("- scopeRef: %s", input.SharedFlowContext.ScopeRef))
	}
	if input.SharedFlowContext.ModulePlanRef != "" {
		lines = append(lines, fmt.Sprintf("- modulePlanRef: %s", input.SharedFlowContext.ModulePlanRef))
	}
	if input.SharedFlowContext.InterfaceRef != "" {
		lines = append(lines, fmt.Sprintf("- interfaceRef: %s", input.SharedFlowContext.InterfaceRef))
	}
	if input.SharedFlowContext.TaskGraphRef != "" {
		lines = append(lines, fmt.Sprintf("- taskGraphRef: %s", input.SharedFlowContext.TaskGraphRef))
	}
	if len(input.SharedFlowContext.CompiledPhases) > 0 {
		lines = append(lines, fmt.Sprintf("- compiledPhases: %s", strings.Join(input.SharedFlowContext.CompiledPhases, ", ")))
	}
	if input.SharedFlowContext.DirectPass {
		lines = append(lines, "- directPass: true")
	}
	for _, item := range input.SharedFlowContext.BoundarySummary {
		lines = append(lines, "- "+item)
	}
	lines = append(lines, "", "Shared task-group context:")
	lines = append(lines, fmt.Sprintf("- sharedFlowContextPath: %s", input.SharedFlowPath))
	lines = append(lines, "", "Current worker task:")
	if len(input.SliceContext.OutputTargets) > 0 {
		lines = append(lines, fmt.Sprintf("- outputTargets: %s", strings.Join(input.SliceContext.OutputTargets, ", ")))
	}
	if len(input.SliceContext.DoneCriteria) > 0 {
		lines = append(lines, fmt.Sprintf("- doneCriteria: %s", strings.Join(input.SliceContext.DoneCriteria, " | ")))
	}
	if len(input.SliceContext.Inputs) > 0 {
		lines = append(lines, fmt.Sprintf("- inputs: %s", strings.Join(input.SliceContext.Inputs, ", ")))
	}
	if input.SliceContext.SliceMode != "" {
		lines = append(lines, fmt.Sprintf("- sliceMode: %s", input.SliceContext.SliceMode))
	}
	if input.SliceContext.Sequence > 0 && input.SliceContext.TotalSlices > 0 {
		lines = append(lines, fmt.Sprintf("- slicePosition: %d/%d", input.SliceContext.Sequence, input.SliceContext.TotalSlices))
	}
	if input.TaskFeedback != nil && len(input.TaskFeedback.RecentFailures) > 0 {
		lines = append(lines, "", "Recent failure memory:")
		lines = append(lines, fmt.Sprintf("- feedbackSummaryPath: %s", input.FeedbackSummaryPath))
		lines = append(lines, fmt.Sprintf("- latestFailure: %s", input.TaskFeedback.LatestMessage))
		if input.TaskFeedback.LatestThinkingSummary != "" {
			lines = append(lines, fmt.Sprintf("- latestThinkingSummary: %s", input.TaskFeedback.LatestThinkingSummary))
		}
		if input.TaskFeedback.LatestNextAction != "" {
			lines = append(lines, fmt.Sprintf("- latestNextAction: %s", input.TaskFeedback.LatestNextAction))
		}
	}
	lines = append(lines,
		"",
		"Soft constraints:",
	)
	for _, rule := range input.ConstraintSystem.Rules {
		if rule.Enforcement != "soft" {
			continue
		}
		if rule.Layer != "execution" && rule.Layer != "verification" && rule.Layer != "learning" {
			continue
		}
		lines = append(lines, fmt.Sprintf("- [%s/%s/%s/%s] %s", rule.Layer, rule.Category, rule.Enforcement, rule.Level, rule.Rule))
	}
	lines = append(lines,
		"",
		"Hard constraints checked by runtime / verify:",
	)
	for _, rule := range input.ConstraintSystem.Rules {
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
	lines = append(lines,
		"",
		"Route policy guardrails:",
		fmt.Sprintf("- reasonCodes: %s", strings.Join(unique(input.Ticket.ReasonCodes), ", ")),
		fmt.Sprintf("- executionLoopSkill: %s", filepath.Join("skills", "qiushi-execution", "SKILL.md")),
		"",
		"Verification and closeout:",
		fmt.Sprintf("- Read verify skeleton: %s", input.VerifySkeletonPath),
		"- Record command evidence and output evidence in verify.json.",
		fmt.Sprintf("- Before exit write: %s, %s, %s", filepath.Join(input.ArtifactDir, "worker-result.json"), filepath.Join(input.ArtifactDir, "verify.json"), filepath.Join(input.ArtifactDir, "handoff.md")),
		"- Before exit, if any required closeout artifact is missing, stop editing and write the missing artifact first.",
		"- If evidence is incomplete, exit blocked instead of claiming success.",
		"",
		"On-demand runtime refs when blocked:",
		fmt.Sprintf("- dispatch ticket: %s", input.TicketPath),
		fmt.Sprintf("- worker spec: %s", input.WorkerSpecPath),
		fmt.Sprintf("- accepted packet: %s", input.AcceptedPacketPath),
		fmt.Sprintf("- planning trace: %s", input.PlanningTracePath),
		fmt.Sprintf("- constraints: %s", input.ConstraintPath),
		fmt.Sprintf("- task graph: %s", input.TaskGraphPath),
		fmt.Sprintf("- request context: %s", input.RequestContextPath),
		fmt.Sprintf("- runtime control context: %s", input.RuntimeContextPath),
		fmt.Sprintf("- handoff contract: %s", input.HandoffContractPath),
		fmt.Sprintf("- takeover contract: %s", input.TakeoverPath),
		"",
		"Hookified verification flow:",
	)
	for _, hook := range input.HookPlan.Hooks {
		lines = append(lines, fmt.Sprintf("- %s | event=%s | action=%s | status=%s", hook.Name, hook.Event, hook.Action, hook.Status))
	}
	return strings.Join(lines, "\n") + "\n"
}
