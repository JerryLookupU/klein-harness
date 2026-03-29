package worker

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"klein-harness/internal/adapter"
	"klein-harness/internal/dispatch"
	"klein-harness/internal/orchestration"
)

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func containsSubstring(values []string, want string) bool {
	for _, value := range values {
		if strings.Contains(value, want) {
			return true
		}
	}
	return false
}

func TestPrepareWritesDispatchTicketWorkerSpecAndPrompt(t *testing.T) {
	root := t.TempDir()
	harnessDir := filepath.Join(root, ".harness")
	if err := os.MkdirAll(filepath.Join(harnessDir, "verification-rules"), 0o755); err != nil {
		t.Fatalf("mkdir harness dirs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(harnessDir, "task-pool.json"), []byte(`{
  "tasks": [
    {
      "taskId": "T-1",
      "threadKey": "thread-1",
      "kind": "feature",
      "roleHint": "worker",
      "title": "Fix worker manifest plumbing",
      "summary": "Ensure worker reads a dispatch manifest before acting.",
      "workerMode": "execution",
      "planEpoch": 3,
      "ownedPaths": ["internal/worker/**"],
      "forbiddenPaths": [".harness/**"],
      "verificationRuleIds": ["VR-1"],
      "resumeStrategy": "resume",
      "preferredResumeSessionId": "sess-1",
      "routingModel": "gpt-5.4",
      "executionModel": "gpt-5.3-codex",
      "orchestrationSessionId": "orch-1",
      "promptStages": ["context_assembly", "plan", "execute", "verify"],
      "dispatch": {
        "worktreePath": ".worktrees/T-1",
        "branchName": "task/T-1",
        "diffBase": "refs/heads/main"
      }
    }
  ]
}`), 0o644); err != nil {
		t.Fatalf("write task-pool: %v", err)
	}
	if err := os.WriteFile(filepath.Join(harnessDir, "project-meta.json"), []byte(`{
  "repoRole": "body_repo",
  "directTargetEditAllowed": false
}`), 0o644); err != nil {
		t.Fatalf("write project-meta: %v", err)
	}
	if err := os.WriteFile(filepath.Join(harnessDir, "verification-rules", "manifest.json"), []byte(`{
  "rules": [
    {
      "id": "VR-1",
      "title": "Go tests",
      "exec": "go test ./...",
      "timeout": 600,
      "readOnlySafe": true
    }
  ]
}`), 0o644); err != nil {
		t.Fatalf("write verification manifest: %v", err)
	}

	bundle, err := Prepare(root, dispatch.Ticket{
		DispatchID:     "dispatch_T-1_3_1",
		IdempotencyKey: "dispatch:T-1:epoch_3:attempt_1",
		TaskID:         "T-1",
		ThreadKey:      "thread-1",
		PlanEpoch:      3,
		Attempt:        1,
		PromptRef:      "prompts/worker-burst.md",
		ReasonCodes: []string{
			"dispatch_ready",
			"policy_bug_rca_first",
			"policy_resume_state_first",
			"policy_log_compact_first",
			"policy_verify_evidence_required",
			"policy_review_if_multi_file_or_high_risk",
		},
		ResumeSessionID: "sess-1",
	}, "lease-1")
	if err != nil {
		t.Fatalf("prepare bundle: %v", err)
	}

	var ticket struct {
		SchemaVersion           string   `json:"schemaVersion"`
		DispatchID              string   `json:"dispatchId"`
		IdempotencyKey          string   `json:"idempotencyKey"`
		LeaseID                 string   `json:"leaseId"`
		TaskID                  string   `json:"taskId"`
		TaskFamily              string   `json:"taskFamily"`
		SOPID                   string   `json:"sopId"`
		RepoRole                string   `json:"repoRole"`
		DirectTargetEditAllowed bool     `json:"directTargetEditAllowed"`
		ArtifactDir             string   `json:"artifactDir"`
		PlanningTracePath       string   `json:"planningTracePath"`
		AcceptedPacketPath      string   `json:"acceptedPacketPath"`
		TaskContractPath        string   `json:"taskContractPath"`
		ExecutionSliceID        string   `json:"executionSliceId"`
		WorkerSpecPath          string   `json:"workerSpecPath"`
		ReasonCodes             []string `json:"reasonCodes"`
		PolicyTags              []string `json:"policyTags"`
		ActiveSkills            []string `json:"activeSkills"`
		SkillHints              []string `json:"skillHints"`
		PacketSynthesis         struct {
			PlannerCount int `json:"plannerCount"`
			Judge        struct {
				ID string `json:"id"`
			} `json:"judge"`
		} `json:"packetSynthesis"`
		Methodology struct {
			Mode         string `json:"mode"`
			GuidePath    string `json:"guidePath"`
			ActiveLenses []struct {
				ID    string `json:"id"`
				Stage string `json:"stage"`
			} `json:"activeLenses"`
		} `json:"methodology"`
		JudgeDecision struct {
			JudgeID            string   `json:"judgeId"`
			SelectedFlow       string   `json:"selectedFlow"`
			WinnerStrategy     string   `json:"winnerStrategy"`
			SelectedDimensions []string `json:"selectedDimensions"`
			SelectedLensIDs    []string `json:"selectedLensIds"`
			ReviewRequired     bool     `json:"reviewRequired"`
			VerifyRequired     bool     `json:"verifyRequired"`
		} `json:"judgeDecision"`
		ExecutionLoop struct {
			Mode            string   `json:"mode"`
			Owner           string   `json:"owner"`
			SkillPath       string   `json:"skillPath"`
			ActiveSkills    []string `json:"activeSkills"`
			SkillHints      []string `json:"skillHints"`
			Phases          []string `json:"phases"`
			CoreRules       []string `json:"coreRules"`
			RetryTransition string   `json:"retryTransition"`
		} `json:"executionLoop"`
		ConstraintSystem struct {
			Mode       string `json:"mode"`
			Objective  string `json:"objective"`
			Generation string `json:"generation"`
			Rules      []struct {
				ID               string `json:"id"`
				Layer            string `json:"layer"`
				Category         string `json:"category"`
				Enforcement      string `json:"enforcement"`
				Level            string `json:"level"`
				VerificationMode string `json:"verificationMode"`
			} `json:"rules"`
		} `json:"constraintSystem"`
		ConstraintPath string `json:"constraintPath"`
		Verification   struct {
			Commands []map[string]any `json:"commands"`
		} `json:"verification"`
		ValidationHooks []struct {
			Name   string `json:"name"`
			Event  string `json:"event"`
			Action string `json:"action"`
		} `json:"validationHooks"`
		LearningHints []string `json:"learningHints"`
	}
	payload, err := os.ReadFile(bundle.TicketPath)
	if err != nil {
		t.Fatalf("read ticket: %v", err)
	}
	if err := json.Unmarshal(payload, &ticket); err != nil {
		t.Fatalf("unmarshal ticket: %v", err)
	}
	if ticket.SchemaVersion != "kh.dispatch-ticket.v1" {
		t.Fatalf("unexpected ticket schema: %+v", ticket)
	}
	if ticket.DispatchID != "dispatch_T-1_3_1" || ticket.LeaseID != "lease-1" || ticket.TaskID != "T-1" {
		t.Fatalf("unexpected ticket identity: %+v", ticket)
	}
	if ticket.TaskFamily != string(orchestration.TaskFamilyDevelopmentTask) || ticket.SOPID != orchestration.SOPDevelopmentTaskV1 {
		t.Fatalf("expected dispatch ticket to carry compiled family+sop, got %+v", ticket)
	}
	if ticket.IdempotencyKey != "dispatch:T-1:epoch_3:attempt_1" {
		t.Fatalf("ticket missing idempotency key: %+v", ticket)
	}
	if ticket.RepoRole != "body_repo" || ticket.DirectTargetEditAllowed {
		t.Fatalf("project meta not propagated: %+v", ticket)
	}
	if ticket.ArtifactDir != bundle.ArtifactDir {
		t.Fatalf("artifact dir mismatch: ticket=%s bundle=%s", ticket.ArtifactDir, bundle.ArtifactDir)
	}
	if ticket.PlanningTracePath != bundle.PlanningTracePath {
		t.Fatalf("planning trace path mismatch: ticket=%s bundle=%s", ticket.PlanningTracePath, bundle.PlanningTracePath)
	}
	if ticket.AcceptedPacketPath != bundle.AcceptedPacketPath {
		t.Fatalf("accepted packet path mismatch: ticket=%s bundle=%s", ticket.AcceptedPacketPath, bundle.AcceptedPacketPath)
	}
	if ticket.TaskContractPath != bundle.TaskContractPath {
		t.Fatalf("task contract path mismatch: ticket=%s bundle=%s", ticket.TaskContractPath, bundle.TaskContractPath)
	}
	if ticket.ExecutionSliceID == "" {
		t.Fatalf("expected execution slice id in ticket: %+v", ticket)
	}
	if ticket.WorkerSpecPath != bundle.WorkerSpecPath {
		t.Fatalf("worker spec path mismatch: ticket=%s bundle=%s", ticket.WorkerSpecPath, bundle.WorkerSpecPath)
	}
	if len(ticket.Verification.Commands) != 1 {
		t.Fatalf("expected one verification command, got %d", len(ticket.Verification.Commands))
	}
	if len(ticket.ReasonCodes) == 0 || len(ticket.PolicyTags) == 0 {
		t.Fatalf("expected route reason codes and policy tags in ticket: %+v", ticket)
	}
	if len(ticket.ActiveSkills) < 2 {
		t.Fatalf("expected active skills in ticket: %+v", ticket)
	}
	if !containsString(ticket.ActiveSkills, "qiushi-execution") || !containsString(ticket.ActiveSkills, "systematic-debugging") || !containsString(ticket.ActiveSkills, "harness-log-search-cskill") {
		t.Fatalf("ticket missing expected active skills: %+v", ticket.ActiveSkills)
	}
	if len(ticket.SkillHints) == 0 {
		t.Fatalf("expected skill hints in ticket: %+v", ticket)
	}
	if len(ticket.ValidationHooks) < 3 {
		t.Fatalf("expected hookified validation plan in ticket: %+v", ticket.ValidationHooks)
	}
	if ticket.PacketSynthesis.PlannerCount != 3 || ticket.PacketSynthesis.Judge.ID != "packet-judge" {
		t.Fatalf("packet synthesis contract missing: %+v", ticket.PacketSynthesis)
	}
	if ticket.Methodology.Mode == "" || !strings.Contains(ticket.Methodology.GuidePath, filepath.Join("prompts", "spec", "methodology.md")) {
		t.Fatalf("methodology contract missing from ticket: %+v", ticket.Methodology)
	}
	if len(ticket.Methodology.ActiveLenses) < 6 {
		t.Fatalf("expected active methodology lenses in ticket: %+v", ticket.Methodology)
	}
	if ticket.JudgeDecision.JudgeID != "packet-judge" || ticket.JudgeDecision.SelectedFlow != "compact-log-first packet" {
		t.Fatalf("judge decision missing from ticket: %+v", ticket.JudgeDecision)
	}
	if len(ticket.JudgeDecision.SelectedDimensions) == 0 || len(ticket.JudgeDecision.SelectedLensIDs) < 6 || !ticket.JudgeDecision.ReviewRequired || !ticket.JudgeDecision.VerifyRequired {
		t.Fatalf("judge decision incomplete: %+v", ticket.JudgeDecision)
	}
	if ticket.ExecutionLoop.Mode != "qiushi execution / validation loop" || len(ticket.ExecutionLoop.Phases) != 6 || !strings.Contains(ticket.ExecutionLoop.SkillPath, filepath.Join("skills", "qiushi-execution", "SKILL.md")) {
		t.Fatalf("execution loop contract missing from ticket: %+v", ticket.ExecutionLoop)
	}
	if len(ticket.ExecutionLoop.ActiveSkills) < 2 || len(ticket.ExecutionLoop.SkillHints) == 0 {
		t.Fatalf("execution loop skill surface missing from ticket: %+v", ticket.ExecutionLoop)
	}
	if ticket.ConstraintSystem.Mode != "two-level layered constraints" || len(ticket.ConstraintSystem.Rules) < 8 {
		t.Fatalf("constraint system missing from ticket: %+v", ticket.ConstraintSystem)
	}
	if ticket.ConstraintPath == "" || !strings.Contains(ticket.ConstraintPath, filepath.Join(".harness", "state", "constraints-T-1.json")) {
		t.Fatalf("constraint path missing from ticket: %+v", ticket)
	}
	if _, err := os.Stat(ticket.ConstraintPath); err != nil {
		t.Fatalf("expected shared constraint snapshot to exist: %v", err)
	}

	var workerSpec struct {
		SchemaVersion         string   `json:"schemaVersion"`
		DispatchID            string   `json:"dispatchId"`
		TaskID                string   `json:"taskId"`
		TaskFamily            string   `json:"taskFamily"`
		SOPID                 string   `json:"sopId"`
		ThreadKey             string   `json:"threadKey"`
		PlanEpoch             int      `json:"planEpoch"`
		Objective             string   `json:"objective"`
		SelectedPlan          string   `json:"selectedPlan"`
		AcceptanceMarkers     []string `json:"acceptanceMarkers"`
		ConstraintPath        string   `json:"constraintPath"`
		SharedContextPath     string   `json:"sharedContextPath"`
		SharedFlowContextPath string   `json:"sharedFlowContextPath"`
		SliceContextPath      string   `json:"sliceContextPath"`
		TaskContractPath      string   `json:"taskContractPath"`
		TaskGraphPath         string   `json:"taskGraphPath"`
		RequestContextPath    string   `json:"requestContextPath"`
		RuntimeContextPath    string   `json:"runtimeContextPath"`
		ContextLayersPath     string   `json:"contextLayersPath"`
		VerifySkeletonPath    string   `json:"verifySkeletonPath"`
		CloseoutSkeletonPath  string   `json:"closeoutSkeletonPath"`
		HandoffContractPath   string   `json:"handoffContractPath"`
		PhaseArtifacts        []struct {
			PhaseID string `json:"phaseId"`
			Role    string `json:"role"`
			Path    string `json:"path"`
		} `json:"phaseArtifacts"`
	}
	workerSpecPayload, err := os.ReadFile(bundle.WorkerSpecPath)
	if err != nil {
		t.Fatalf("read worker spec: %v", err)
	}
	if err := json.Unmarshal(workerSpecPayload, &workerSpec); err != nil {
		t.Fatalf("unmarshal worker spec: %v", err)
	}
	if workerSpec.SchemaVersion != "kh.worker-spec.v1" || workerSpec.DispatchID != ticket.DispatchID || workerSpec.TaskID != ticket.TaskID {
		t.Fatalf("worker spec identity mismatch: %+v", workerSpec)
	}
	if workerSpec.TaskFamily != ticket.TaskFamily || workerSpec.SOPID != ticket.SOPID {
		t.Fatalf("worker spec missing compiled family+sop: %+v ticket=%+v", workerSpec, ticket)
	}
	if workerSpec.ThreadKey != "thread-1" || workerSpec.PlanEpoch != 3 {
		t.Fatalf("worker spec missing lineage: %+v", workerSpec)
	}
	if workerSpec.Objective == "" || workerSpec.SelectedPlan == "" || len(workerSpec.AcceptanceMarkers) != 1 {
		t.Fatalf("worker spec missing execution contract: %+v", workerSpec)
	}
	if len(workerSpec.PhaseArtifacts) < 4 {
		t.Fatalf("expected worker spec to carry compiled phase artifacts, got %+v", workerSpec.PhaseArtifacts)
	}
	if workerSpec.ConstraintPath != ticket.ConstraintPath {
		t.Fatalf("worker spec missing shared constraint path: %+v ticket=%+v", workerSpec, ticket)
	}
	if workerSpec.SharedContextPath == "" {
		t.Fatalf("worker spec missing shared context path: %+v", workerSpec)
	}
	for _, path := range []string{
		workerSpec.SharedFlowContextPath,
		workerSpec.SliceContextPath,
		workerSpec.TaskContractPath,
		workerSpec.TaskGraphPath,
		workerSpec.RequestContextPath,
		workerSpec.RuntimeContextPath,
		workerSpec.ContextLayersPath,
		workerSpec.VerifySkeletonPath,
		workerSpec.CloseoutSkeletonPath,
		workerSpec.HandoffContractPath,
	} {
		if path == "" {
			t.Fatalf("worker spec missing compiled context path: %+v", workerSpec)
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected compiled context artifact to exist: %s err=%v", path, err)
		}
	}
	if _, err := os.Stat(bundle.AcceptedPacketPath); err != nil {
		t.Fatalf("expected accepted packet to exist: %v", err)
	}
	if _, err := os.Stat(bundle.TaskContractPath); err != nil {
		t.Fatalf("expected task contract to exist: %v", err)
	}
	prompt, err := os.ReadFile(bundle.PromptPath)
	if err != nil {
		t.Fatalf("read prompt: %v", err)
	}
	promptText := string(prompt)
	if !strings.Contains(promptText, "Task background:") {
		t.Fatalf("prompt missing task background section")
	}
	if !strings.Contains(promptText, "activeSkills: qiushi-execution") {
		t.Fatalf("prompt missing active skills guidance: %s", promptText)
	}
	if !strings.Contains(promptText, "runtime-owned files inside .harness remain authoritative, but this prompt is the primary worker handoff.") {
		t.Fatalf("prompt missing skill authority guidance: %s", promptText)
	}
	if !strings.Contains(promptText, bundle.WorkerSpecPath) {
		t.Fatalf("prompt missing worker spec path: %s", promptText)
	}
	if !strings.Contains(promptText, bundle.AcceptedPacketPath) {
		t.Fatalf("prompt missing accepted packet path: %s", promptText)
	}
	if !strings.Contains(promptText, bundle.TaskContractPath) {
		t.Fatalf("prompt missing task contract path: %s", promptText)
	}
	if !strings.Contains(promptText, workerSpec.ContextLayersPath) || !strings.Contains(promptText, workerSpec.HandoffContractPath) || !strings.Contains(promptText, workerSpec.CloseoutSkeletonPath) {
		t.Fatalf("prompt missing compiled context refs: %s", promptText)
	}
	if !strings.Contains(promptText, workerSpec.SharedContextPath) {
		t.Fatalf("prompt missing shared context path: %s", promptText)
	}
	if !strings.Contains(promptText, ticket.ExecutionSliceID) {
		t.Fatalf("prompt missing execution slice id: %s", promptText)
	}
	if !strings.Contains(promptText, "metadata-backed B3Ehive") {
		t.Fatalf("prompt missing visible B3Ehive guidance")
	}
	if !strings.Contains(promptText, "Shared constraints:") {
		t.Fatalf("prompt missing shared constraints section")
	}
	if !strings.Contains(promptText, "Shared spec:") {
		t.Fatalf("prompt missing shared spec section")
	}
	if !strings.Contains(promptText, "Current worker task:") {
		t.Fatalf("prompt missing current worker task section")
	}
	if !strings.Contains(promptText, "Shared task-group context:") {
		t.Fatalf("prompt missing shared task-group context section")
	}
	if !strings.Contains(promptText, "Soft constraints:") {
		t.Fatalf("prompt missing soft constraints section")
	}
	if !strings.Contains(promptText, "Hard constraints checked by runtime / verify:") {
		t.Fatalf("prompt missing hard constraints section")
	}
	if !strings.Contains(promptText, "[execution/process/soft/enforced]") {
		t.Fatalf("prompt missing soft execution-layer constraint preview")
	}
	if !strings.Contains(promptText, "[execution/boundary/hard/enforced]") {
		t.Fatalf("prompt missing hard execution-layer constraint preview")
	}
	if !strings.Contains(promptText, "check=owned_path_audit") {
		t.Fatalf("prompt missing hard verification mode preview")
	}
	if !strings.Contains(promptText, filepath.Join("skills", "qiushi-execution", "SKILL.md")) {
		t.Fatalf("prompt missing qiushi skill path")
	}
	if !strings.Contains(promptText, "Route policy guardrails:") {
		t.Fatalf("prompt missing policy guardrail section")
	}
	if !strings.Contains(promptText, "On-demand runtime refs when blocked:") {
		t.Fatalf("prompt missing on-demand runtime refs section")
	}
	if strings.Contains(promptText, "Read the immutable dispatch ticket") || strings.Contains(promptText, "Read the task-local worker spec") {
		t.Fatalf("prompt should not instruct the worker to front-load raw JSON files: %s", promptText)
	}
	if !strings.Contains(promptText, "Hookified verification flow:") {
		t.Fatalf("prompt missing hookified verification flow section")
	}
	if !strings.Contains(promptText, "Before exit, if any required closeout artifact is missing, stop editing and write the missing artifact first.") {
		t.Fatalf("prompt missing explicit closeout hook guidance")
	}
	if strings.Contains(promptText, filepath.Join("prompts", "spec", "orchestrator.md")) {
		t.Fatalf("prompt should not eagerly include orchestrator path")
	}
	if strings.Contains(promptText, filepath.Join("prompts", "spec", "tasks.md")) {
		t.Fatalf("prompt should not eagerly include tasks guide path")
	}
	planningTrace, err := os.ReadFile(bundle.PlanningTracePath)
	if err != nil {
		t.Fatalf("read planning trace: %v", err)
	}
	planningText := string(planningTrace)
	if !strings.Contains(planningText, "qiushi-inspired fact-first / focus-first / verify-first discipline") {
		t.Fatalf("planning trace missing methodology layer: %s", planningText)
	}
	if !strings.Contains(planningText, "## Active Methodology Lenses") {
		t.Fatalf("planning trace missing active methodology lenses: %s", planningText)
	}
	if !strings.Contains(planningText, "RCA First") || !strings.Contains(planningText, "State First Resume") || !strings.Contains(planningText, "Compact Log First") || !strings.Contains(planningText, "Review Before Done") {
		t.Fatalf("planning trace missing policy-derived methodology lenses: %s", planningText)
	}
	if !strings.Contains(planningText, "## Judge Decision") || !strings.Contains(planningText, "selectedFlow: compact-log-first packet") {
		t.Fatalf("planning trace missing judge decision: %s", planningText)
	}
	if !strings.Contains(planningText, "## Execution Validation Loop") || !strings.Contains(planningText, "mode: qiushi execution / validation loop") {
		t.Fatalf("planning trace missing qiushi execution loop: %s", planningText)
	}
	if !strings.Contains(planningText, "## Layered Constraints") || !strings.Contains(planningText, "[planning/format/hard/enforced] planning-format-json-schema") {
		t.Fatalf("planning trace missing layered constraints: %s", planningText)
	}
	if !strings.Contains(planningText, "verificationMode: schema_validation") {
		t.Fatalf("planning trace missing hard constraint verification mode: %s", planningText)
	}
	if !strings.Contains(planningText, "metadata-backed packet synthesis (3 planners + 1 judge)") {
		t.Fatalf("planning trace missing packet synthesis summary: %s", planningText)
	}
	if !strings.Contains(planningText, "Packet Planner A") || !strings.Contains(planningText, "Packet Judge") {
		t.Fatalf("planning trace missing planner/judge details: %s", planningText)
	}

	var contextLayers struct {
		SchemaVersion string `json:"schemaVersion"`
		Request       struct {
			Goal string `json:"goal"`
			Kind string `json:"kind"`
		} `json:"request"`
		SliceLocal struct {
			ExecutionSliceID string `json:"executionSliceId"`
		} `json:"sliceLocal"`
		RuntimeControl struct {
			ExecutionCWD         string `json:"executionCwd"`
			WorktreePath         string `json:"worktreePath"`
			TaskGraphPath        string `json:"taskGraphPath"`
			CloseoutSkeletonPath string `json:"closeoutSkeletonPath"`
		} `json:"runtimeControl"`
	}
	if payload, err := os.ReadFile(workerSpec.ContextLayersPath); err != nil {
		t.Fatalf("read context layers: %v", err)
	} else if err := json.Unmarshal(payload, &contextLayers); err != nil {
		t.Fatalf("unmarshal context layers: %v", err)
	}
	if contextLayers.SchemaVersion != "kh.context-layers.v1" || contextLayers.Request.Goal == "" || contextLayers.SliceLocal.ExecutionSliceID == "" || contextLayers.RuntimeControl.ExecutionCWD == "" || contextLayers.RuntimeControl.WorktreePath == "" || contextLayers.RuntimeControl.TaskGraphPath == "" || contextLayers.RuntimeControl.CloseoutSkeletonPath == "" {
		t.Fatalf("unexpected context layers contract: %+v", contextLayers)
	}

	var sliceContext struct {
		TaskContractPath    string   `json:"taskContractPath"`
		TaskGraphPath       string   `json:"taskGraphPath"`
		PromptCompileInputs []string `json:"promptCompileInputs"`
		ResumeArtifacts     []string `json:"resumeArtifacts"`
	}
	if payload, err := os.ReadFile(workerSpec.SliceContextPath); err != nil {
		t.Fatalf("read slice context: %v", err)
	} else if err := json.Unmarshal(payload, &sliceContext); err != nil {
		t.Fatalf("unmarshal slice context: %v", err)
	}
	if sliceContext.TaskContractPath == "" || sliceContext.TaskGraphPath == "" || len(sliceContext.PromptCompileInputs) == 0 || len(sliceContext.ResumeArtifacts) == 0 {
		t.Fatalf("unexpected slice context contract: %+v", sliceContext)
	}
	for _, want := range []string{
		workerSpec.ContextLayersPath,
		workerSpec.TaskContractPath,
		workerSpec.VerifySkeletonPath,
		workerSpec.CloseoutSkeletonPath,
		workerSpec.HandoffContractPath,
	} {
		if !containsSubstring(sliceContext.PromptCompileInputs, want) {
			t.Fatalf("expected slice context prompt inputs to include %q, got %+v", want, sliceContext.PromptCompileInputs)
		}
	}

	var verifySkeleton struct {
		RequestContextPath string `json:"requestContextPath"`
		RuntimeContextPath string `json:"runtimeContextPath"`
		TaskContractPath   string `json:"taskContractPath"`
		TaskGraphPath      string `json:"taskGraphPath"`
		PhaseArtifacts     []struct {
			PhaseID string `json:"phaseId"`
			Path    string `json:"path"`
		} `json:"phaseArtifacts"`
	}
	if payload, err := os.ReadFile(workerSpec.VerifySkeletonPath); err != nil {
		t.Fatalf("read verify skeleton: %v", err)
	} else if err := json.Unmarshal(payload, &verifySkeleton); err != nil {
		t.Fatalf("unmarshal verify skeleton: %v", err)
	}
	if verifySkeleton.RequestContextPath == "" || verifySkeleton.RuntimeContextPath == "" || verifySkeleton.TaskContractPath == "" || verifySkeleton.TaskGraphPath == "" || len(verifySkeleton.PhaseArtifacts) == 0 {
		t.Fatalf("unexpected verify skeleton contract: %+v", verifySkeleton)
	}

	var closeout struct {
		SchemaVersion      string   `json:"schemaVersion"`
		TaskContractPath   string   `json:"taskContractPath"`
		TaskGraphPath      string   `json:"taskGraphPath"`
		VerifySkeletonPath string   `json:"verifySkeletonPath"`
		WorkerMustProvide  []string `json:"workerMustProvide"`
		ResumeChecklist    []string `json:"resumeChecklist"`
		PhaseArtifacts     []struct {
			PhaseID string `json:"phaseId"`
			Path    string `json:"path"`
		} `json:"phaseArtifacts"`
	}
	if payload, err := os.ReadFile(workerSpec.CloseoutSkeletonPath); err != nil {
		t.Fatalf("read closeout skeleton: %v", err)
	} else if err := json.Unmarshal(payload, &closeout); err != nil {
		t.Fatalf("unmarshal closeout skeleton: %v", err)
	}
	if closeout.SchemaVersion != "kh.closeout-skeleton.v1" || closeout.TaskContractPath == "" || closeout.TaskGraphPath == "" || closeout.VerifySkeletonPath == "" || len(closeout.WorkerMustProvide) == 0 || len(closeout.PhaseArtifacts) == 0 || !containsSubstring(closeout.ResumeChecklist, "executionCwd=") {
		t.Fatalf("unexpected closeout skeleton contract: %+v", closeout)
	}

	var takeover struct {
		SchemaVersion   string `json:"schemaVersion"`
		ResumeSessionID string `json:"resumeSessionId"`
		TaskStatus      string `json:"taskStatus"`
		ExecutionCWD    string `json:"executionCwd"`
		WorktreePath    string `json:"worktreePath"`
		ArtifactDir     string `json:"artifactDir"`
		PhaseArtifacts  []struct {
			PhaseID string `json:"phaseId"`
			Path    string `json:"path"`
		} `json:"phaseArtifacts"`
		RequestContextPath   string   `json:"requestContextPath"`
		RuntimeContextPath   string   `json:"runtimeContextPath"`
		ContextLayersPath    string   `json:"contextLayersPath"`
		TaskContractPath     string   `json:"taskContractPath"`
		TaskGraphPath        string   `json:"taskGraphPath"`
		CloseoutSkeletonPath string   `json:"closeoutSkeletonPath"`
		HandoffContractPath  string   `json:"handoffContractPath"`
		ReadOrder            []string `json:"readOrder"`
		RequiredArtifacts    []string `json:"requiredArtifacts"`
		OwnedPaths           []string `json:"ownedPaths"`
		SessionRegistryPath  string   `json:"sessionRegistryPath"`
		EntryChecklist       []string `json:"entryChecklist"`
		ControlPlaneGuards   []string `json:"controlPlaneGuards"`
	}
	if payload, err := os.ReadFile(filepath.Join(bundle.ArtifactDir, "takeover-context.json")); err != nil {
		t.Fatalf("read takeover context: %v", err)
	} else if err := json.Unmarshal(payload, &takeover); err != nil {
		t.Fatalf("unmarshal takeover context: %v", err)
	}
	if takeover.SchemaVersion != "kh.multi-session-continuation.v1" || takeover.RequestContextPath == "" || takeover.RuntimeContextPath == "" || takeover.ContextLayersPath == "" || takeover.TaskContractPath == "" || takeover.TaskGraphPath == "" || takeover.CloseoutSkeletonPath == "" || takeover.HandoffContractPath == "" || takeover.SessionRegistryPath == "" || len(takeover.ReadOrder) == 0 || len(takeover.RequiredArtifacts) == 0 {
		t.Fatalf("unexpected continuation protocol: %+v", takeover)
	}
	if takeover.ResumeSessionID != "sess-1" || takeover.TaskStatus == "" || takeover.ExecutionCWD == "" || takeover.WorktreePath == "" || takeover.ArtifactDir != bundle.ArtifactDir || len(takeover.OwnedPaths) == 0 || len(takeover.EntryChecklist) == 0 || len(takeover.ControlPlaneGuards) == 0 {
		t.Fatalf("expected continuation protocol to carry resume/session state, got %+v", takeover)
	}
	if len(takeover.PhaseArtifacts) == 0 {
		t.Fatalf("expected continuation protocol to carry phase artifacts, got %+v", takeover)
	}
	if !containsSubstring(takeover.EntryChecklist, "executionCwd=") {
		t.Fatalf("expected continuation checklist to preserve execution cwd, got %+v", takeover.EntryChecklist)
	}
	for _, want := range []string{
		workerSpec.ContextLayersPath,
		workerSpec.RequestContextPath,
		workerSpec.RuntimeContextPath,
		workerSpec.SharedFlowContextPath,
		workerSpec.TaskGraphPath,
		workerSpec.SliceContextPath,
		workerSpec.TaskContractPath,
		workerSpec.VerifySkeletonPath,
		workerSpec.CloseoutSkeletonPath,
		workerSpec.HandoffContractPath,
		filepath.Join(root, ".harness", "state", "session-registry.json"),
		filepath.Join(bundle.ArtifactDir, "handoff.md"),
	} {
		if !containsSubstring(takeover.ReadOrder, want) {
			t.Fatalf("expected takeover read order to include %q, got %+v", want, takeover.ReadOrder)
		}
		if !containsSubstring(takeover.RequiredArtifacts, want) {
			t.Fatalf("expected takeover required artifacts to include %q, got %+v", want, takeover.RequiredArtifacts)
		}
	}
}

func TestPrepareIncludesOuterLoopMemoryInPromptWhenFeedbackSummaryExists(t *testing.T) {
	root := t.TempDir()
	harnessDir := filepath.Join(root, ".harness")
	if err := os.MkdirAll(filepath.Join(harnessDir, "state"), 0o755); err != nil {
		t.Fatalf("mkdir harness state: %v", err)
	}
	if err := os.WriteFile(filepath.Join(harnessDir, "task-pool.json"), []byte(`{
  "tasks": [
    {
      "taskId": "T-1",
      "threadKey": "thread-1",
      "kind": "bug",
      "roleHint": "worker",
      "title": "Retry after failed verification",
      "summary": "Retry after failed verification",
      "workerMode": "execution",
      "planEpoch": 2,
      "ownedPaths": ["test/**"],
      "forbiddenPaths": [".harness/**"],
      "resumeStrategy": "fresh",
      "routingModel": "gpt-5.4",
      "executionModel": "gpt-5.3-codex",
      "orchestrationSessionId": "orch-1",
      "promptStages": ["analysis", "route", "dispatch", "execute", "verify"]
    }
  ]
}`), 0o644); err != nil {
		t.Fatalf("write task-pool: %v", err)
	}
	if err := os.WriteFile(filepath.Join(harnessDir, "state", "feedback-summary.json"), []byte(`{
  "schemaVersion": "kh.feedback-summary.v1",
  "generator": "kh-runtime",
  "generatedAt": "2026-03-26T10:00:00Z",
  "feedbackLogPath": ".harness/feedback-log.jsonl",
  "feedbackEventCount": 1,
  "errorCount": 1,
  "criticalCount": 0,
  "illegalActionCount": 0,
  "recentFailures": [],
  "taskFeedbackSummary": {
    "T-1": {
      "taskId": "T-1",
      "feedbackCount": 1,
      "errorCount": 1,
      "criticalCount": 0,
      "latestFeedbackType": "verification_failure",
      "latestSeverity": "error",
      "latestMessage": "verify evidence rejected the previous output",
      "latestThinkingSummary": "Identify the exact acceptance mismatch before changing code again.",
      "latestNextAction": "Read verify evidence first, then re-enter execution with one bounded fix.",
      "latestTimestamp": "2026-03-26T10:00:00Z",
      "recentFailures": [
        {
          "id": "FB-00001",
          "taskId": "T-1",
          "feedbackType": "verification_failure",
          "severity": "error",
          "source": "verification",
          "step": "verify",
          "message": "verify evidence rejected the previous output",
          "thinkingSummary": "Identify the exact acceptance mismatch before changing code again.",
          "nextAction": "Read verify evidence first, then re-enter execution with one bounded fix.",
          "timestamp": "2026-03-26T10:00:00Z"
        }
      ]
    }
  }
}`), 0o644); err != nil {
		t.Fatalf("write feedback summary: %v", err)
	}

	bundle, err := Prepare(root, dispatch.Ticket{
		DispatchID:     "dispatch_T-1_2_1",
		IdempotencyKey: "dispatch:T-1:epoch_2:attempt_1",
		TaskID:         "T-1",
		ThreadKey:      "thread-1",
		PlanEpoch:      2,
		Attempt:        1,
		PromptRef:      "prompts/spec/apply.md",
		ReasonCodes:    []string{"dispatch_ready"},
	}, "lease-1")
	if err != nil {
		t.Fatalf("prepare bundle: %v", err)
	}
	prompt, err := os.ReadFile(bundle.PromptPath)
	if err != nil {
		t.Fatalf("read prompt: %v", err)
	}
	promptText := string(prompt)
	if !strings.Contains(promptText, "Recent failure memory:") {
		t.Fatalf("prompt missing outer-loop memory section: %s", promptText)
	}
	if !strings.Contains(promptText, "feedback-summary.json") {
		t.Fatalf("prompt missing feedback summary path: %s", promptText)
	}
	if !strings.Contains(promptText, "Identify the exact acceptance mismatch before changing code again.") {
		t.Fatalf("prompt missing thinking summary: %s", promptText)
	}
}

func TestPrepareDerivesSemanticExecutionSlicesAndSelectsByAttempt(t *testing.T) {
	root := t.TempDir()
	harnessDir := filepath.Join(root, ".harness")
	if err := os.MkdirAll(filepath.Join(harnessDir, "verification-rules"), 0o755); err != nil {
		t.Fatalf("mkdir harness dirs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(harnessDir, "task-pool.json"), []byte(`{
  "tasks": [
    {
      "taskId": "T-2",
      "threadKey": "thread-2",
      "kind": "feature",
      "roleHint": "worker",
      "title": "Visual dashboard delivery",
      "summary": "Build the runtime dashboard.\n1. Show planner lanes.\n2. Show judge merge result.\n3. Show tmux worker chain.",
      "workerMode": "execution",
      "planEpoch": 1,
      "ownedPaths": ["internal/worker/**", "internal/verify/**", "prompts/spec/**"],
      "forbiddenPaths": [".harness/**"],
      "resumeStrategy": "fresh",
      "routingModel": "gpt-5.4",
      "executionModel": "gpt-5.3-codex",
      "orchestrationSessionId": "orch-2",
      "promptStages": ["route", "dispatch", "execute", "verify"]
    }
  ]
}`), 0o644); err != nil {
		t.Fatalf("write task-pool: %v", err)
	}

	bundle, err := Prepare(root, dispatch.Ticket{
		DispatchID:     "dispatch_T-2_1_2",
		IdempotencyKey: "dispatch:T-2:epoch_1:attempt_2",
		TaskID:         "T-2",
		ThreadKey:      "thread-2",
		PlanEpoch:      1,
		Attempt:        2,
		PromptRef:      "prompts/spec/apply.md",
		ReasonCodes:    []string{"dispatch_ready"},
	}, "lease-2")
	if err != nil {
		t.Fatalf("prepare bundle: %v", err)
	}

	var ticket struct {
		ExecutionSliceID string `json:"executionSliceId"`
		AcceptedPacket   struct {
			ExecutionTasks []struct {
				ID      string   `json:"id"`
				InScope []string `json:"inScope"`
			} `json:"executionTasks"`
		} `json:"acceptedPacket"`
		TaskContract struct {
			ExecutionSliceID string   `json:"executionSliceId"`
			InScope          []string `json:"inScope"`
		} `json:"taskContract"`
	}
	payload, err := os.ReadFile(bundle.TicketPath)
	if err != nil {
		t.Fatalf("read ticket: %v", err)
	}
	if err := json.Unmarshal(payload, &ticket); err != nil {
		t.Fatalf("unmarshal ticket: %v", err)
	}
	if len(ticket.AcceptedPacket.ExecutionTasks) != 4 {
		t.Fatalf("expected 4 semantic execution tasks, got %+v", ticket.AcceptedPacket.ExecutionTasks)
	}
	if ticket.ExecutionSliceID != "T-2.slice.2" {
		t.Fatalf("expected attempt 2 to select slice 2, got %+v", ticket)
	}
	if ticket.TaskContract.ExecutionSliceID != "T-2.slice.2" {
		t.Fatalf("expected task contract to bind selected slice, got %+v", ticket.TaskContract)
	}
	if strings.Join(ticket.TaskContract.InScope, ",") != "internal/worker/**,internal/verify/**,prompts/spec/**" {
		t.Fatalf("expected semantic slice to inherit owned path boundary, got %+v", ticket.TaskContract)
	}
}

func TestPrepareSelectsFirstIncompleteSliceFromProgress(t *testing.T) {
	root := t.TempDir()
	harnessDir := filepath.Join(root, ".harness")
	if err := os.MkdirAll(filepath.Join(harnessDir, "state"), 0o755); err != nil {
		t.Fatalf("mkdir harness state: %v", err)
	}
	if err := os.WriteFile(filepath.Join(harnessDir, "task-pool.json"), []byte(`{
  "tasks": [
    {
      "taskId": "T-3",
      "threadKey": "thread-3",
      "kind": "feature",
      "roleHint": "worker",
      "title": "Progress-driven slice selection",
      "summary": "Track dashboard execution.\n1. Show tasklist.\n2. Show checklist.\n3. Show token usage.",
      "workerMode": "execution",
      "planEpoch": 1,
      "ownedPaths": ["internal/worker/**", "internal/verify/**", "prompts/spec/**"],
      "forbiddenPaths": [".harness/**"],
      "resumeStrategy": "fresh",
      "routingModel": "gpt-5.4",
      "executionModel": "gpt-5.3-codex",
      "orchestrationSessionId": "orch-3",
      "promptStages": ["route", "dispatch", "execute", "verify"]
    }
  ]
}`), 0o644); err != nil {
		t.Fatalf("write task-pool: %v", err)
	}
	if err := orchestration.WritePacketProgress(orchestration.PacketProgressPath(root, "T-3"), orchestration.PacketProgress{
		SchemaVersion:     "kh.packet-progress.v1",
		Generator:         "test",
		UpdatedAt:         "2026-03-26T10:00:00Z",
		TaskID:            "T-3",
		ThreadKey:         "thread-3",
		PlanEpoch:         1,
		AcceptedPacketID:  "packet_T-3_1",
		CompletedSliceIDs: []string{"T-3.slice.1"},
		LastDispatchID:    "dispatch_T-3_1_1",
	}); err != nil {
		t.Fatalf("write packet progress: %v", err)
	}

	bundle, err := Prepare(root, dispatch.Ticket{
		DispatchID:     "dispatch_T-3_1_5",
		IdempotencyKey: "dispatch:T-3:epoch_1:attempt_5",
		TaskID:         "T-3",
		ThreadKey:      "thread-3",
		PlanEpoch:      1,
		Attempt:        5,
		PromptRef:      "prompts/spec/apply.md",
		ReasonCodes:    []string{"dispatch_ready"},
	}, "lease-3")
	if err != nil {
		t.Fatalf("prepare bundle: %v", err)
	}

	var ticket struct {
		ExecutionSliceID string `json:"executionSliceId"`
		TaskContract     struct {
			ExecutionSliceID string   `json:"executionSliceId"`
			InScope          []string `json:"inScope"`
		} `json:"taskContract"`
	}
	payload, err := os.ReadFile(bundle.TicketPath)
	if err != nil {
		t.Fatalf("read ticket: %v", err)
	}
	if err := json.Unmarshal(payload, &ticket); err != nil {
		t.Fatalf("unmarshal ticket: %v", err)
	}
	if ticket.ExecutionSliceID != "T-3.slice.2" {
		t.Fatalf("expected first incomplete slice to be selected, got %+v", ticket)
	}
	if ticket.TaskContract.ExecutionSliceID != "T-3.slice.2" {
		t.Fatalf("expected task contract to bind first incomplete slice, got %+v", ticket.TaskContract)
	}
	if strings.Join(ticket.TaskContract.InScope, ",") != "internal/worker/**,internal/verify/**,prompts/spec/**" {
		t.Fatalf("expected first incomplete slice to keep owned path boundary, got %+v", ticket.TaskContract)
	}
}

func TestPrepareResetsStalePacketProgressAcrossPlanEpoch(t *testing.T) {
	root := t.TempDir()
	harnessDir := filepath.Join(root, ".harness")
	if err := os.MkdirAll(filepath.Join(harnessDir, "state"), 0o755); err != nil {
		t.Fatalf("mkdir harness state: %v", err)
	}
	if err := os.WriteFile(filepath.Join(harnessDir, "task-pool.json"), []byte(`{
  "tasks": [
    {
      "taskId": "T-3b",
      "threadKey": "thread-3b",
      "kind": "feature",
      "roleHint": "worker",
      "title": "Progress reset on replan",
      "summary": "Track dashboard execution.\n1. Show tasklist.\n2. Show checklist.\n3. Show token usage.",
      "workerMode": "execution",
      "planEpoch": 2,
      "ownedPaths": ["internal/worker/**", "internal/verify/**", "prompts/spec/**"],
      "forbiddenPaths": [".harness/**"],
      "resumeStrategy": "fresh",
      "routingModel": "gpt-5.4",
      "executionModel": "gpt-5.3-codex",
      "orchestrationSessionId": "orch-3b",
      "promptStages": ["route", "dispatch", "execute", "verify"]
    }
  ]
}`), 0o644); err != nil {
		t.Fatalf("write task-pool: %v", err)
	}
	if err := orchestration.WritePacketProgress(orchestration.PacketProgressPath(root, "T-3b"), orchestration.PacketProgress{
		SchemaVersion:     "kh.packet-progress.v1",
		Generator:         "test",
		UpdatedAt:         "2026-03-26T10:00:00Z",
		TaskID:            "T-3b",
		ThreadKey:         "thread-3b",
		PlanEpoch:         1,
		AcceptedPacketID:  "packet_T-3b_1",
		CompletedSliceIDs: []string{"T-3b.slice.1"},
		LastDispatchID:    "dispatch_T-3b_1_1",
	}); err != nil {
		t.Fatalf("write stale packet progress: %v", err)
	}

	bundle, err := Prepare(root, dispatch.Ticket{
		DispatchID:     "dispatch_T-3b_2_1",
		IdempotencyKey: "dispatch:T-3b:epoch_2:attempt_1",
		TaskID:         "T-3b",
		ThreadKey:      "thread-3b",
		PlanEpoch:      2,
		Attempt:        1,
		PromptRef:      "prompts/spec/apply.md",
		ReasonCodes:    []string{"dispatch_ready"},
	}, "lease-3b")
	if err != nil {
		t.Fatalf("prepare bundle: %v", err)
	}

	var ticket struct {
		ExecutionSliceID string `json:"executionSliceId"`
		TaskContract     struct {
			ExecutionSliceID string `json:"executionSliceId"`
		} `json:"taskContract"`
	}
	payload, err := os.ReadFile(bundle.TicketPath)
	if err != nil {
		t.Fatalf("read ticket: %v", err)
	}
	if err := json.Unmarshal(payload, &ticket); err != nil {
		t.Fatalf("unmarshal ticket: %v", err)
	}
	if ticket.ExecutionSliceID != "T-3b.slice.1" || ticket.TaskContract.ExecutionSliceID != "T-3b.slice.1" {
		t.Fatalf("expected stale progress to be ignored after replan, got %+v", ticket)
	}

	progress, err := orchestration.LoadPacketProgress(orchestration.PacketProgressPath(root, "T-3b"))
	if err != nil {
		t.Fatalf("load packet progress: %v", err)
	}
	if progress.PlanEpoch != 2 || progress.AcceptedPacketID != "packet_T-3b_2" || len(progress.CompletedSliceIDs) != 0 {
		t.Fatalf("expected stale packet progress to be reset, got %+v", progress)
	}
}

func TestPrepareDoesNotSplitExecutionTasksByOwnedPathsWithoutExplicitRequirements(t *testing.T) {
	root := t.TempDir()
	harnessDir := filepath.Join(root, ".harness")
	if err := os.MkdirAll(filepath.Join(harnessDir, "verification-rules"), 0o755); err != nil {
		t.Fatalf("mkdir harness dirs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(harnessDir, "task-pool.json"), []byte(`{
  "tasks": [
    {
      "taskId": "T-4",
      "threadKey": "thread-4",
      "kind": "feature",
      "roleHint": "worker",
      "title": "Inspect doc lineage",
      "summary": "Traverse the owned document tree and prepare a bounded dashboard update.",
      "workerMode": "execution",
      "planEpoch": 1,
      "ownedPaths": ["docs/**", "internal/dashboard/**", "internal/query/**"],
      "forbiddenPaths": [".harness/**"],
      "resumeStrategy": "fresh",
      "routingModel": "gpt-5.4",
      "executionModel": "gpt-5.3-codex",
      "orchestrationSessionId": "orch-4",
      "promptStages": ["route", "dispatch", "execute", "verify"]
    }
  ]
}`), 0o644); err != nil {
		t.Fatalf("write task-pool: %v", err)
	}

	bundle, err := Prepare(root, dispatch.Ticket{
		DispatchID:     "dispatch_T-4_1_1",
		IdempotencyKey: "dispatch:T-4:epoch_1:attempt_1",
		TaskID:         "T-4",
		ThreadKey:      "thread-4",
		PlanEpoch:      1,
		Attempt:        1,
		PromptRef:      "prompts/spec/apply.md",
		ReasonCodes:    []string{"dispatch_ready"},
	}, "lease-4")
	if err != nil {
		t.Fatalf("prepare bundle: %v", err)
	}

	var ticket struct {
		AcceptedPacket struct {
			ExecutionTasks []struct {
				ID      string   `json:"id"`
				InScope []string `json:"inScope"`
			} `json:"executionTasks"`
		} `json:"acceptedPacket"`
		TaskContract struct {
			ExecutionSliceID string   `json:"executionSliceId"`
			InScope          []string `json:"inScope"`
		} `json:"taskContract"`
	}
	payload, err := os.ReadFile(bundle.TicketPath)
	if err != nil {
		t.Fatalf("read ticket: %v", err)
	}
	if err := json.Unmarshal(payload, &ticket); err != nil {
		t.Fatalf("unmarshal ticket: %v", err)
	}
	if len(ticket.AcceptedPacket.ExecutionTasks) != 1 {
		t.Fatalf("expected ownedPaths to stay as boundary only, got %+v", ticket.AcceptedPacket.ExecutionTasks)
	}
	if ticket.TaskContract.ExecutionSliceID != "T-4.slice.1" {
		t.Fatalf("expected single semantic slice to stay selected, got %+v", ticket.TaskContract)
	}
	if strings.Join(ticket.TaskContract.InScope, ",") != "docs/**,internal/dashboard/**,internal/query/**" {
		t.Fatalf("expected task contract to keep full owned path boundary, got %+v", ticket.TaskContract)
	}
}

func TestPrepareDerivesInlineDisplayRequirementsFromSingleLineGoal(t *testing.T) {
	root := t.TempDir()
	harnessDir := filepath.Join(root, ".harness")
	if err := os.MkdirAll(filepath.Join(harnessDir, "verification-rules"), 0o755); err != nil {
		t.Fatalf("mkdir harness dirs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(harnessDir, "task-pool.json"), []byte(`{
  "tasks": [
    {
      "taskId": "T-5",
      "threadKey": "thread-5",
      "kind": "feature",
      "roleHint": "worker",
      "title": "Harness dashboard frontend",
      "summary": "针对本地 harness-architect 做前端可视化开发任务：展示 planner/judge、tasklist/checklist、tmux worker 链路与 token 花销；本次先只做读面分析，不修改业务代码",
      "description": "补充：需要把追加需求的落点和 token 热区变化也显式展示在 dashboard 里",
      "workerMode": "execution",
      "planEpoch": 1,
      "ownedPaths": ["docs/**", "internal/dashboard/**", "internal/query/**"],
      "forbiddenPaths": [".harness/**"],
      "resumeStrategy": "fresh",
      "routingModel": "gpt-5.4",
      "executionModel": "gpt-5.3-codex",
      "orchestrationSessionId": "orch-5",
      "promptStages": ["route", "dispatch", "execute", "verify"]
    }
  ]
}`), 0o644); err != nil {
		t.Fatalf("write task-pool: %v", err)
	}

	bundle, err := Prepare(root, dispatch.Ticket{
		DispatchID:     "dispatch_T-5_1_1",
		IdempotencyKey: "dispatch:T-5:epoch_1:attempt_1",
		TaskID:         "T-5",
		ThreadKey:      "thread-5",
		PlanEpoch:      1,
		Attempt:        1,
		PromptRef:      "prompts/spec/apply.md",
		ReasonCodes:    []string{"dispatch_ready"},
	}, "lease-5")
	if err != nil {
		t.Fatalf("prepare bundle: %v", err)
	}

	var ticket struct {
		AcceptedPacket struct {
			ExecutionTasks []struct {
				Title   string `json:"title"`
				Summary string `json:"summary"`
			} `json:"executionTasks"`
		} `json:"acceptedPacket"`
	}
	payload, err := os.ReadFile(bundle.TicketPath)
	if err != nil {
		t.Fatalf("read ticket: %v", err)
	}
	if err := json.Unmarshal(payload, &ticket); err != nil {
		t.Fatalf("unmarshal ticket: %v", err)
	}
	if len(ticket.AcceptedPacket.ExecutionTasks) != 5 {
		t.Fatalf("expected title + 4 inline semantic tasks, got %+v", ticket.AcceptedPacket.ExecutionTasks)
	}
	if !strings.Contains(ticket.AcceptedPacket.ExecutionTasks[1].Summary, "展示 planner/judge") {
		t.Fatalf("expected planner/judge inline task, got %+v", ticket.AcceptedPacket.ExecutionTasks)
	}
	if !strings.Contains(ticket.AcceptedPacket.ExecutionTasks[4].Summary, "追加需求的落点和 token 热区变化") {
		t.Fatalf("expected appended requirement task, got %+v", ticket.AcceptedPacket.ExecutionTasks)
	}
}

func TestInferCorpusPlanningHonorsExplicitSingleFileOutput(t *testing.T) {
	task := adapter.Task{
		TaskID:      "T-linguists",
		Title:       "语言学家资料单文档交付",
		Summary:     "根据 docs/prd.md 产出 20 位世界顶级语言学家资料，写入 output/linguists.md，总正文不少于 2000 字。",
		Description: "每位学者包含 基本信息、代表成果、核心贡献、历史影响。",
	}

	info := inferCorpusPlanning(task)
	if !info.SingleDocument {
		t.Fatalf("expected explicit output file to select single-document mode, got %+v", info)
	}
	if info.OutputFile != "output/linguists.md" {
		t.Fatalf("expected explicit output file, got %+v", info)
	}
	if info.OutputDir != "output" {
		t.Fatalf("expected output dir to follow output file, got %+v", info)
	}
	if info.IndexFile != "" {
		t.Fatalf("expected single-document plan to avoid default index file, got %+v", info)
	}
	if info.SubjectCount != 20 || info.SubjectLabel != "世界顶级语言学家资料" {
		t.Fatalf("expected subject parsing to survive single-document mode, got %+v", info)
	}
	if info.MinChars != 2000 {
		t.Fatalf("expected doc-level min chars, got %+v", info)
	}
	if !containsString(info.RequiredSections, "基本信息") || !containsString(info.RequiredSections, "历史影响") {
		t.Fatalf("expected required sections to be preserved, got %+v", info)
	}
}

func TestInferCorpusPlanningRecognizesRepeatedObjectCountsBeyondWei(t *testing.T) {
	task := adapter.Task{
		TaskID:  "T-cards",
		Title:   "角色卡片批量生成",
		Summary: "生成 5 个角色卡片，分别是 战士、法师、盗贼、牧师、游侠。",
	}

	info := inferCorpusPlanning(task)
	if info.SubjectCount != 5 || info.SubjectLabel != "角色卡片" {
		t.Fatalf("expected generic repeated-object parsing, got %+v", info)
	}
	if len(info.EntityRoster) != 5 {
		t.Fatalf("expected explicit entity roster, got %+v", info)
	}
}

func TestInferCorpusPlanningRecognizesChineseCountWords(t *testing.T) {
	task := adapter.Task{
		TaskID:  "T-cn-count",
		Title:   "语言学家资料",
		Summary: "需要二十名语言学家资料，最终汇总到 output/linguists.md。",
	}

	info := inferCorpusPlanning(task)
	if info.SubjectCount != 20 || info.SubjectUnit != "名" || info.SubjectLabel != "语言学家资料" {
		t.Fatalf("expected Chinese count words to be parsed, got %+v", info)
	}
}

func TestInferCorpusPlanningRecognizesHuiZongOutputFile(t *testing.T) {
	task := adapter.Task{
		TaskID:  "T-huizong",
		Title:   "语言学家资料汇总",
		Summary: "需要 20 位语言学家资料，最终汇总到 output/linguists.md。",
	}

	info := inferCorpusPlanning(task)
	if !info.SingleDocument || info.OutputFile != "output/linguists.md" {
		t.Fatalf("expected hui-zong wording to preserve single-document output, got %+v", info)
	}
}

func TestSingleDocumentCorpusContextAvoidsMultiFileDefaults(t *testing.T) {
	task := adapter.Task{
		TaskID:      "T-linguists",
		Title:       "语言学家资料单文档交付",
		Summary:     "根据 docs/prd.md 产出 20 位世界顶级语言学家资料，写入 output/linguists.md，总正文不少于 2000 字。",
		Description: "每位学者包含 基本信息、代表成果、核心贡献、历史影响。",
	}

	executionTasks := deriveExecutionTasks(task, nil)
	if len(executionTasks) != 1 {
		t.Fatalf("expected roster-freeze-only planning before orchestration expansion, got %+v", executionTasks)
	}
	if executionTasks[0].Title != "冻结名单与分片规格" {
		t.Fatalf("expected roster-freeze slice first, got %+v", executionTasks)
	}
	if len(executionTasks[0].OutputTargets) != 1 || executionTasks[0].OutputTargets[0] != "output/linguists.roster.md" {
		t.Fatalf("expected roster-freeze slice to target the roster artifact, got %+v", executionTasks[0])
	}

	sharedContext := buildSharedTaskGroupContext(task, executionTasks)
	if sharedContext == nil {
		t.Fatalf("expected shared context for single-document corpus task")
	}
	if sharedContext.ContentContract.OutputFile != "output/linguists.md" {
		t.Fatalf("expected shared context to preserve explicit output file, got %+v", sharedContext.ContentContract)
	}
	if sharedContext.ContentContract.IndexFile != "" || sharedContext.ContentContract.FileNamingRule != "" {
		t.Fatalf("expected single-document context to avoid multi-file defaults, got %+v", sharedContext.ContentContract)
	}
	if containsString(sharedContext.ContentContract.FormatConstraints, "每位对象单独一个文件") ||
		containsString(sharedContext.ContentContract.FormatConstraints, "总索引与正文文件分离") {
		t.Fatalf("expected single-document context to avoid multi-file format constraints, got %+v", sharedContext.ContentContract)
	}
	if !containsString(sharedContext.ContentContract.FormatConstraints, "固定单文件交付") {
		t.Fatalf("expected single-document constraint marker, got %+v", sharedContext.ContentContract)
	}
	if !containsString(sharedContext.ContentContract.FormatConstraints, "同类对象默认先拆成逐对象片段，再汇总成单文档") {
		t.Fatalf("expected single-document context to advertise atomic split rule, got %+v", sharedContext.ContentContract)
	}
	if !containsString(sharedContext.OperatorTaskList, "冻结名单与分片规格") {
		t.Fatalf("expected execution task list to include the roster-freeze slice, got %+v", sharedContext.OperatorTaskList)
	}
	if !containsSubstring(sharedContext.SharedPrompt, "总正文不少于 2000 字") {
		t.Fatalf("expected shared prompt to describe doc-level min chars, got %+v", sharedContext.SharedPrompt)
	}
	if containsSubstring(sharedContext.SharedPrompt, "每个文件不少于 2000 字") {
		t.Fatalf("expected shared prompt to avoid per-file min chars wording, got %+v", sharedContext.SharedPrompt)
	}
	if !containsSubstring(sharedContext.SharedPrompt, "默认拆成 20 个逐对象 worker slice") {
		t.Fatalf("expected shared prompt to describe atomic split rule, got %+v", sharedContext.SharedPrompt)
	}
}

func TestSingleDocumentCorpusExpandsAfterFrozenRosterExists(t *testing.T) {
	root := t.TempDir()
	task := adapter.Task{
		TaskID:      "T-linguists",
		Title:       "语言学家资料单文档交付",
		Summary:     "根据 docs/prd.md 产出 20 位世界顶级语言学家资料，写入 output/linguists.md，总正文不少于 2000 字。",
		Description: "每位学者包含 基本信息、代表成果、核心贡献、历史影响。",
	}
	if err := os.MkdirAll(filepath.Join(root, "output"), 0o755); err != nil {
		t.Fatalf("mkdir output: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "output", "linguists.roster.md"), []byte("- Ferdinand de Saussure\n- Noam Chomsky\n"), 0o644); err != nil {
		t.Fatalf("write roster: %v", err)
	}

	executionTasks := deriveExecutionTasksWithRoot(root, task, nil)
	if len(executionTasks) != 4 {
		t.Fatalf("expected roster-driven fanout after orchestration expansion, got %+v", executionTasks)
	}
	if executionTasks[1].EntityBatch[0] != "Ferdinand de Saussure" {
		t.Fatalf("expected first entity to come from frozen roster, got %+v", executionTasks[1])
	}
	if executionTasks[2].EntityBatch[0] != "Noam Chomsky" {
		t.Fatalf("expected second entity to come from frozen roster, got %+v", executionTasks[2])
	}
	if executionTasks[3].OutputTargets[0] != "output/linguists.md" {
		t.Fatalf("expected final closeout to target the final document, got %+v", executionTasks[3])
	}
}

func TestCompiledPhaseArtifactsForRepeatedEntityCorpus(t *testing.T) {
	refs := compiledPhaseArtifacts(
		orchestration.CompiledFlow{SOPID: orchestration.SOPRepeatedEntityCorpusV1},
		"/repo/.harness/artifacts/T-201/dispatch/shared-spec.json",
		"/repo/.harness/artifacts/T-201/dispatch/variable-inputs.json",
		"",
		"",
		"",
		"/repo/.harness/artifacts/T-201/dispatch/task-graph.json",
		"/repo/.harness/artifacts/T-201/dispatch/context-layers.json",
		"/repo/.harness/artifacts/T-201/dispatch/slice-context.json",
		"/repo/.harness/artifacts/T-201/dispatch/task-contract.json",
		"/repo/.harness/artifacts/T-201/dispatch/verify-skeleton.json",
		"/repo/.harness/artifacts/T-201/dispatch/closeout-skeleton.json",
		"/repo/.harness/artifacts/T-201/dispatch/handoff-contract.json",
		"/repo/.harness/artifacts/T-201/dispatch/takeover-context.json",
		"/repo/.harness/state/runner-prompt-T-201.md",
	)
	for _, want := range []struct {
		phase string
		layer string
		role  string
	}{
		{phase: "extract_shared_spec", layer: "shared_flow", role: "shared_contract"},
		{phase: "extract_variable_inputs", layer: "shared_flow", role: "variable_inputs"},
		{phase: "compile_task_graph", layer: "shared_flow", role: "task_graph"},
		{phase: "compile_worker_prompt", layer: "slice_local", role: "slice_context"},
		{phase: "compile_worker_prompt", layer: "runtime_control", role: "context_layers"},
		{phase: "compile_worker_prompt", layer: "runtime_control", role: "task_contract"},
		{phase: "compile_worker_prompt", layer: "runtime_control", role: "takeover_contract"},
		{phase: "compile_worker_prompt", layer: "runtime_control", role: "worker_prompt"},
		{phase: "programmatic_verify", layer: "runtime_control", role: "verify_skeleton"},
		{phase: "closeout", layer: "runtime_control", role: "closeout_skeleton"},
		{phase: "closeout", layer: "runtime_control", role: "handoff_contract"},
	} {
		if !hasPhaseArtifact(refs, want.phase, want.layer, want.role) {
			t.Fatalf("expected repeated corpus phase artifacts to include %+v, got %+v", want, refs)
		}
	}
}

func hasPhaseArtifact(refs []orchestration.PhaseArtifactRef, phase, layer, role string) bool {
	for _, ref := range refs {
		if ref.PhaseID == phase && ref.Layer == layer && ref.Role == role && ref.Path != "" {
			return true
		}
	}
	return false
}
