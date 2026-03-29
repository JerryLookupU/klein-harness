package orchestration

import (
	"fmt"
	"regexp"
	"strings"

	"klein-harness/internal/adapter"
)

func developmentTaskSOP() SOPDefinition {
	return SOPDefinition{
		ID:                 SOPDevelopmentTaskV1,
		Family:             TaskFamilyDevelopmentTask,
		Description:        "Program-owned requirement, architecture, interface, task-graph, verify, and closeout scaffolding for development work.",
		DirectPassEligible: true,
		Phases: []SOPPhase{
			{ID: "requirement_spec", Description: "Extract requirement spec", ModelOutput: []string{"goal", "in_scope", "out_of_scope", "success_criteria", "user_flows", "constraints", "risks"}, ProgramOwns: []string{"requirement-spec.json"}},
			{ID: "architecture_contract", Description: "Extract architecture contract", ModelOutput: []string{"target_modules", "new_paths", "affected_paths", "reuse_points", "boundary_rules", "core_types", "core_services", "data_flow", "state_flow", "error_flow"}, ProgramOwns: []string{"architecture-contract.json"}},
			{ID: "interface_contract", Description: "Extract interface contract", ModelOutput: []string{"api_endpoints", "request_shapes", "response_shapes", "domain_models", "validation_rules"}, ProgramOwns: []string{"interface-contract.json"}},
			{ID: "task_graph_compile", Description: "Compile scaffold, implementation, integration, tests, closeout slices", ProgramOwns: []string{"task-graph.json"}},
			{ID: "worker_execute", Description: "Worker executes only current slice"},
			{ID: "integration_verify", Description: "Program-led verify skeleton", ProgramOwns: []string{"verify-skeleton.json"}},
			{ID: "closeout", Description: "Program-led closeout summary", ProgramOwns: []string{"closeout skeleton"}},
		},
		VerifyRules: []VerifyRuleSpec{
			{ID: "scoped_changes", Description: "Changes remain inside allowed paths and declared slice scope."},
			{ID: "tests_recorded", Description: "Verification must record commands and results."},
		},
		CloseoutRules: []CloseoutRuleSpec{
			{ID: "program_closeout", Description: "Program aggregates touched files, tests, and risks before model summary."},
		},
		ContinuationArtifacts: []string{"requirement-spec.json", "architecture-contract.json", "interface-contract.json", "shared-flow-context.json", "slice-context.json", "verify-skeleton.json", "handoff.md"},
	}
}

func CompileDevelopmentTask(task adapter.Task) CompiledFlow {
	req := ExtractDevelopmentRequirementSpec(task)
	arch := ExtractDevelopmentArchitectureContract(task)
	iface := ExtractDevelopmentInterfaceContract(task)
	tasks := CompileDevelopmentTaskGraph(task, req, arch, iface)
	family := developmentFlowFamily(task)
	directPass := len(tasks) == 1
	return CompiledFlow{
		Family:               family,
		SOPID:                SOPDevelopmentTaskV1,
		RequirementSpec:      req,
		ArchitectureContract: arch,
		InterfaceContract:    iface,
		TaskGraphCompile:     developmentTaskGraphCompileSpec(family, tasks),
		ExecutionTasks:       tasks,
		SharedFlowContext: SharedFlowContext{
			TaskFamily:      family,
			SOPID:           SOPDevelopmentTaskV1,
			Summary:         strings.TrimSpace(coalesce(task.Description, task.Summary, task.Title)),
			CompiledPhases:  []string{"requirement_spec", "architecture_contract", "interface_contract", "task_graph_compile", "worker_execute", "integration_verify", "closeout"},
			DirectPass:      directPass,
			BoundarySummary: uniqueStrings(append(append([]string{"程序冻结 requirement / architecture / interface contract", "worker 只执行当前开发 slice"}, developmentTaskBoundaryNotes(family, directPass)...), task.OwnedPaths...)),
		},
		SharedTaskGroupContext: &SharedTaskGroupContext{
			GroupID: task.TaskID + ".group",
			Summary: strings.TrimSpace(coalesce(task.Description, task.Summary, task.Title)),
			SharedPrompt: []string{
				"先读 requirement / architecture / interface contract，再处理当前 slice。",
				"不要自行重排整条开发 flow。",
			},
			VerificationFocus: []string{
				"程序先生成 verify skeleton；模型只补风险与说明。",
				"下一 session 通过固定文件合同接手，而不是重扫控制面。",
			},
		},
	}
}

func developmentTaskGraphCompileSpec(family TaskFamily, tasks []ExecutionTask) TaskGraphCompileSpec {
	spec := TaskGraphCompileSpec{
		CompileMode:    "bounded_stage_compile",
		ResumeProtocol: "kh.multi-session-continuation.v1",
		ReplanTriggers: []string{
			"verification_failed",
			"compiled_contract_missing",
			"scope_violation_detected",
			"integration_verify_blocked",
		},
		ProgramOwnedNotes: []string{
			"program freezes requirement, architecture, interface, and task graph contracts before execution",
			"worker executes the selected development slice only",
		},
	}
	switch {
	case len(tasks) <= 1:
		spec.CompileMode = "single_slice_direct_pass"
		spec.DirectPassReason = "family_or_scope_allows_direct_pass"
	case family == TaskFamilyFeatureModule || family == TaskFamilyFeatureSystem:
		spec.CompileMode = "semantic_stage_compile"
		spec.DirectPassReason = "semantic_multi_slice_required"
	default:
		spec.DirectPassReason = "bounded_stage_required"
	}
	return spec
}

func developmentFlowFamily(task adapter.Task) TaskFamily {
	family := TaskFamily(task.TaskFamily)
	switch family {
	case TaskFamilySingleArtifact, TaskFamilyBugfixSmall, TaskFamilyFeatureModule, TaskFamilyFeatureSystem, TaskFamilyDevelopmentTask, TaskFamilyIntegrationExternal, TaskFamilyReviewOrAudit, TaskFamilyRepairOrResume:
		return family
	default:
		return TaskFamilyDevelopmentTask
	}
}

func developmentTaskBoundaryNotes(family TaskFamily, directPass bool) []string {
	notes := []string{}
	if family != TaskFamilyDevelopmentTask && family != "" {
		notes = append(notes, "runtime 保留原始 task family="+string(family))
	}
	if directPass {
		notes = append(notes, "当前任务启用 single slice direct pass")
	}
	return notes
}

func ExtractDevelopmentRequirementSpec(task adapter.Task) *DevelopmentRequirementSpec {
	summary := strings.TrimSpace(coalesce(task.Summary, task.Title))
	desc := strings.TrimSpace(task.Description)
	inScope := []string{}
	if len(task.OwnedPaths) > 0 {
		inScope = append(inScope, task.OwnedPaths...)
	}
	success := []string{"实现当前目标", "验证结果有证据", "closeout artifacts 完整"}
	constraints := []string{"程序主导编排", "worker 只处理当前 slice"}
	if desc != "" {
		constraints = append(constraints, desc)
	}
	return &DevelopmentRequirementSpec{
		Goal:            summary,
		InScope:         uniqueStrings(inScope),
		OutOfScope:      uniqueStrings(append([]string{"全局 .harness 真相账本", "全局完成判定"}, task.ForbiddenPaths...)),
		SuccessCriteria: success,
		UserFlows:       []string{"submit -> classify -> compile -> execute -> verify -> closeout"},
		Constraints:     uniqueStrings(constraints),
		Risks:           []string{"scope drift", "context pollution", "verify/handoff 不完整"},
	}
}

func ExtractDevelopmentArchitectureContract(task adapter.Task) *DevelopmentArchitectureContract {
	targetModules := uniqueStrings(task.OwnedPaths)
	if len(targetModules) == 0 {
		targetModules = []string{"repo-local bounded module"}
	}
	return &DevelopmentArchitectureContract{
		TargetModules: targetModules,
		NewPaths:      uniqueStrings(task.OwnedPaths),
		AffectedPaths: uniqueStrings(task.OwnedPaths),
		ReusePoints:   []string{"existing runtime orchestration", "existing worker manifest", "existing verify pipeline"},
		BoundaryRules: []string{"程序负责控制面", "worker 不改全局状态账本"},
		CoreTypes:     []string{"task family", "sop registry", "context layers", "slice contract"},
		CoreServices:  []string{"submit classifier", "worker prompt compiler", "verify skeleton compiler"},
		DataFlow:      []string{"request -> family -> sop -> contracts -> slices"},
		StateFlow:     []string{"queued -> routing -> running -> verify -> follow-up"},
		ErrorFlow:     []string{"verify failure -> replan", "artifact missing -> blocked"},
	}
}

func ExtractDevelopmentInterfaceContract(task adapter.Task) *DevelopmentInterfaceContract {
	return &DevelopmentInterfaceContract{
		APIEndpoints:    []string{"Submit", "RunOnce", "Prepare"},
		RequestShapes:   []string{"SubmitRequest", "Task submission metadata"},
		ResponseShapes:  []string{"SubmitResult", "DispatchBundle"},
		DomainModels:    []string{"Task", "AcceptedPacket", "TaskContract", "ContextLayers"},
		ValidationRules: []string{"verify skeleton must not be empty", "session handoff must be file-backed"},
	}
}

func CompileDevelopmentTaskGraph(task adapter.Task, req *DevelopmentRequirementSpec, arch *DevelopmentArchitectureContract, iface *DevelopmentInterfaceContract) []ExecutionTask {
	short := strings.TrimSpace(coalesce(task.Title, task.TaskID))
	if shouldUseSingleSliceDirectPass(task, arch, iface) {
		return []ExecutionTask{
			{
				ID:            fmt.Sprintf("%s.slice.1", task.TaskID),
				Title:         "单 slice 直通",
				Summary:       fmt.Sprintf("在受控边界内直接完成 `%s` 的实现、验证与收口。", short),
				TaskGroupID:   task.TaskID + ".group",
				BatchLabel:    "direct-pass",
				OutputTargets: uniqueStrings(arch.AffectedPaths),
				DoneCriteria:  []string{"代码实现完成", "验证命令已记录", "closeout artifacts 完整"},
			},
		}
	}
	requirements := developmentRequirementLines(task)
	if len(requirements) > 0 {
		slices := make([]ExecutionTask, 0, len(requirements)+1)
		if title := strings.TrimSpace(task.Title); title != "" {
			slices = append(slices, ExecutionTask{
				ID:           fmt.Sprintf("%s.slice.1", task.TaskID),
				Title:        title,
				Summary:      fmt.Sprintf("建立主任务骨架并稳定任务命名，确保后续规划、追加需求和执行链都挂在同一主线上。 | %s", title),
				TaskGroupID:  task.TaskID + ".group",
				BatchLabel:   "anchor",
				DoneCriteria: []string{"主任务骨架稳定", "验证证据已记录"},
			})
		}
		for index, requirement := range requirements {
			slices = append(slices, ExecutionTask{
				ID:            fmt.Sprintf("%s.slice.%d", task.TaskID, len(slices)+1),
				Title:         fmt.Sprintf("%s [%d]", semanticDevelopmentTaskTitle(task, requirement), index+1),
				Summary:       requirement,
				TaskGroupID:   task.TaskID + ".group",
				BatchLabel:    "semantic",
				OutputTargets: uniqueStrings(arch.AffectedPaths),
				DoneCriteria:  []string{"requirement intent is reflected in runtime artifacts", "verification evidence recorded", "closeout artifacts written"},
			})
		}
		return slices
	}
	return []ExecutionTask{
		{
			ID:            fmt.Sprintf("%s.slice.1", task.TaskID),
			Title:         coalesce(task.Title, task.TaskID),
			Summary:       fmt.Sprintf("在受控边界内推进 `%s`，并把验证与 closeout 收口在同一 slice。", short),
			TaskGroupID:   task.TaskID + ".group",
			BatchLabel:    "bounded",
			OutputTargets: uniqueStrings(arch.AffectedPaths),
			DoneCriteria:  uniqueStrings(append([]string{"bounded change applied", "verification evidence recorded", "closeout artifacts written"}, req.SuccessCriteria...)),
		},
	}
}

func shouldUseSingleSliceDirectPass(task adapter.Task, arch *DevelopmentArchitectureContract, iface *DevelopmentInterfaceContract) bool {
	family := developmentFlowFamily(task)
	if family == TaskFamilyBugfixSmall || family == TaskFamilyRepairOrResume {
		return true
	}
	if len(arch.NewPaths) == 0 && len(arch.AffectedPaths) <= 1 && len(iface.APIEndpoints) == 0 {
		return true
	}
	return false
}

func developmentRequirementLines(task adapter.Task) []string {
	lines := strings.Split(strings.ReplaceAll(task.Summary+"\n"+task.Description, "\r\n", "\n"), "\n")
	requirements := explicitDevelopmentRequirementLines(lines)
	if len(requirements) > 0 {
		return requirements
	}
	return inlineDevelopmentRequirementLines(lines)
}

func explicitDevelopmentRequirementLines(lines []string) []string {
	requirementRE := regexp.MustCompile(`^(?:\d+[\.\)、:：-]*|[-*•]+)\s*`)
	requirements := make([]string, 0, len(lines))
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		if lower == "要求" || lower == "requirements" || lower == "requirement" {
			continue
		}
		if !requirementRE.MatchString(line) {
			continue
		}
		line = strings.TrimSpace(requirementRE.ReplaceAllString(line, ""))
		line = strings.Trim(line, "：:;；")
		if line == "" {
			continue
		}
		requirements = append(requirements, line)
	}
	return uniqueStrings(requirements)
}

func inlineDevelopmentRequirementLines(lines []string) []string {
	requirements := make([]string, 0, len(lines))
	for _, raw := range lines {
		line := normalizeDevelopmentRequirementLine(raw)
		if line == "" {
			continue
		}
		if part := splitDevelopmentNeedToDisplayRequirement(line); part != "" {
			requirements = append(requirements, part)
			continue
		}
		if parts := splitDevelopmentDisplayRequirements(line); len(parts) > 0 {
			requirements = append(requirements, parts...)
			continue
		}
	}
	return uniqueStrings(requirements)
}

func normalizeDevelopmentRequirementLine(raw string) string {
	line := strings.TrimSpace(raw)
	for _, prefix := range []string{"补充：", "补充:", "说明：", "说明:", "要求：", "要求:"} {
		line = strings.TrimPrefix(line, prefix)
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return ""
	}
	lower := strings.ToLower(line)
	if lower == "要求" || lower == "requirements" || lower == "requirement" {
		return ""
	}
	return line
}

func splitDevelopmentDisplayRequirements(line string) []string {
	patterns := []string{"展示", "显示", "包括", "包含"}
	for _, marker := range patterns {
		index := strings.Index(line, marker)
		if index < 0 {
			continue
		}
		remainder := strings.TrimSpace(strings.TrimLeft(line[index+len(marker):], "：:，, "))
		if remainder == "" {
			return nil
		}
		parts := strings.FieldsFunc(remainder, func(r rune) bool {
			return strings.ContainsRune("、；;|", r)
		})
		out := make([]string, 0, len(parts))
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" || strings.HasPrefix(part, "本次先") || strings.Contains(part, "不修改业务代码") {
				continue
			}
			out = append(out, marker+" "+part)
		}
		if len(out) > 0 {
			return out
		}
	}
	return nil
}

func splitDevelopmentNeedToDisplayRequirement(line string) string {
	for _, marker := range []string{"需要把", "需要将"} {
		index := strings.Index(line, marker)
		if index < 0 {
			continue
		}
		part := strings.TrimSpace(line[index+len(marker):])
		part = strings.TrimSuffix(part, "也显式展示在 dashboard 里")
		part = strings.TrimSuffix(part, "显式展示在 dashboard 里")
		part = strings.TrimSuffix(part, "展示在 dashboard 里")
		part = strings.TrimSuffix(part, "也展示在 dashboard 里")
		part = strings.Trim(part, "。")
		if part == "" {
			return ""
		}
		return marker + part
	}
	return ""
}

func semanticDevelopmentTaskTitle(task adapter.Task, requirement string) string {
	switch {
	case strings.Contains(requirement, "planner/judge"):
		return coalesce(task.Title, "展示 planner/judge")
	case strings.Contains(requirement, "tasklist"), strings.Contains(requirement, "checklist"):
		return coalesce(task.Title, "展示 tasklist/checklist")
	case strings.Contains(requirement, "tmux"), strings.Contains(requirement, "worker"):
		return coalesce(task.Title, "展示 tmux worker 链路")
	case strings.Contains(requirement, "token"):
		return coalesce(task.Title, "展示 token 热区")
	default:
		return coalesce(task.Title, task.TaskID)
	}
}
