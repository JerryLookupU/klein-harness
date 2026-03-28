package worker

import (
	"strings"
	"testing"

	"klein-harness/internal/adapter"
	"klein-harness/internal/dispatch"
	"klein-harness/internal/orchestration"
)

func TestCompileWorkerPromptIncludesCompiledContracts(t *testing.T) {
	prompt := CompileWorkerPrompt(PromptCompileInput{
		TicketPath:          "/repo/.harness/state/dispatch-ticket-T-1.json",
		WorkerSpecPath:      "/repo/.harness/artifacts/T-1/dispatch/worker-spec.json",
		AcceptedPacketPath:  "/repo/.harness/state/accepted-packet-T-1.json",
		TaskContractPath:    "/repo/.harness/artifacts/T-1/dispatch/task-contract.json",
		TaskGraphPath:       "/repo/.harness/artifacts/T-1/dispatch/task-graph.json",
		ContextLayersPath:   "/repo/.harness/artifacts/T-1/dispatch/context-layers.json",
		RequestContextPath:  "/repo/.harness/artifacts/T-1/dispatch/request-context.json",
		RuntimeContextPath:  "/repo/.harness/artifacts/T-1/dispatch/runtime-control-context.json",
		PlanningTracePath:   "/repo/.harness/state/planning-trace-T-1.md",
		ConstraintPath:      "/repo/.harness/state/constraints-T-1.json",
		SharedContextPath:   "/repo/.harness/artifacts/T-1/dispatch/shared-context.json",
		SharedFlowPath:      "/repo/.harness/artifacts/T-1/dispatch/shared-flow-context.json",
		SliceContextPath:    "/repo/.harness/artifacts/T-1/dispatch/slice-context.json",
		VerifySkeletonPath:  "/repo/.harness/artifacts/T-1/dispatch/verify-skeleton.json",
		HandoffContractPath: "/repo/.harness/artifacts/T-1/dispatch/handoff-contract.json",
		TakeoverPath:        "/repo/.harness/artifacts/T-1/dispatch/takeover-context.json",
		ArtifactDir:         "/repo/.harness/artifacts/T-1/dispatch",
		Task: adapter.Task{
			TaskID:     "T-1",
			TaskFamily: string(orchestration.TaskFamilyDevelopmentTask),
			SOPID:      orchestration.SOPDevelopmentTaskV1,
			Title:      "实现 development sop",
			Summary:    "实现 development sop",
		},
		Ticket: dispatch.Ticket{
			TaskID:      "T-1",
			DispatchID:  "dispatch-T-1",
			ReasonCodes: []string{"dispatch_ready", "policy_verify_evidence_required"},
		},
		ExecutionLoop: orchestration.ExecutionLoopContract{
			ActiveSkills: []string{"qiushi-execution"},
		},
		SharedFlowContext: orchestration.SharedFlowContext{
			TaskFamily:     orchestration.TaskFamilyDevelopmentTask,
			SOPID:          orchestration.SOPDevelopmentTaskV1,
			Summary:        "shared development context",
			ScopeRef:       "/repo/.harness/artifacts/T-1/dispatch/requirement-spec.json",
			ModulePlanRef:  "/repo/.harness/artifacts/T-1/dispatch/architecture-contract.json",
			TaskGraphRef:   "/repo/.harness/artifacts/T-1/dispatch/task-graph.json",
			CompiledPhases: []string{"requirement_spec", "task_graph_compile", "worker_execute"},
		},
		SliceContext: orchestration.SliceLocalContext{
			ExecutionSliceID:    "T-1.slice.2",
			SliceMode:           "staged",
			Sequence:            2,
			TotalSlices:         3,
			Title:               "实现当前开发切片",
			Summary:             "只修改当前模块",
			AllowedWriteGlobs:   []string{"internal/**"},
			ForbiddenWriteGlobs: []string{".harness/**"},
			OutputTargets:       []string{"internal/orchestration/sop_registry.go"},
		},
	})
	for _, want := range []string{
		"taskFamily: development_task",
		"sopId: sop.development_task.v1",
		"context-layers.json",
		"request-context.json",
		"handoff-contract.json",
		"shared-flow-context.json",
		"taskGraphRef: /repo/.harness/artifacts/T-1/dispatch/task-graph.json",
		"slicePosition: 2/3",
		"verify-skeleton.json",
		"Route policy guardrails:",
		"Hookified verification flow:",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("expected prompt to contain %q, got:\n%s", want, prompt)
		}
	}
}
