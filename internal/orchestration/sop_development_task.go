package orchestration

import (
	"fmt"
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
	return CompiledFlow{
		Family:               TaskFamilyDevelopmentTask,
		SOPID:                SOPDevelopmentTaskV1,
		RequirementSpec:      req,
		ArchitectureContract: arch,
		InterfaceContract:    iface,
		ExecutionTasks:       tasks,
		SharedFlowContext: SharedFlowContext{
			TaskFamily:      TaskFamilyDevelopmentTask,
			SOPID:           SOPDevelopmentTaskV1,
			Summary:         strings.TrimSpace(coalesce(task.Description, task.Summary, task.Title)),
			BoundarySummary: uniqueStrings(append([]string{"程序冻结 requirement / architecture / interface contract", "worker 只执行当前开发 slice"}, task.OwnedPaths...)),
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
	slices := []ExecutionTask{
		{
			ID:           fmt.Sprintf("%s.slice.1", task.TaskID),
			Title:        "建立需求与结构合同",
			Summary:      fmt.Sprintf("冻结 `%s` 的 requirement / architecture / interface contract。", short),
			TaskGroupID:  task.TaskID + ".group",
			BatchLabel:   "contracts",
			DoneCriteria: []string{"需求合同冻结", "架构合同冻结", "接口合同冻结"},
		},
		{
			ID:            fmt.Sprintf("%s.slice.2", task.TaskID),
			Title:         "实现当前开发切片",
			Summary:       fmt.Sprintf("在受控边界内实现 `%s`。", short),
			TaskGroupID:   task.TaskID + ".group",
			BatchLabel:    "implementation",
			OutputTargets: uniqueStrings(arch.AffectedPaths),
			DoneCriteria:  []string{"代码实现完成", "验证命令已记录"},
		},
		{
			ID:           fmt.Sprintf("%s.slice.3", task.TaskID),
			Title:        "完成验证与收口",
			Summary:      "基于 verify skeleton 完成集成验证和 closeout。",
			TaskGroupID:  task.TaskID + ".group",
			BatchLabel:   "closeout",
			DoneCriteria: []string{"verify evidence 完整", "handoff 完整"},
		},
	}
	if len(req.InScope) == 0 && len(iface.APIEndpoints) == 0 {
		slices = slices[:2]
	}
	return slices
}
