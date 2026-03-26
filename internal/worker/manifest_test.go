package worker

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"klein-harness/internal/dispatch"
)

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
		RepoRole                string   `json:"repoRole"`
		DirectTargetEditAllowed bool     `json:"directTargetEditAllowed"`
		ArtifactDir             string   `json:"artifactDir"`
		PlanningTracePath       string   `json:"planningTracePath"`
		WorkerSpecPath          string   `json:"workerSpecPath"`
		ReasonCodes             []string `json:"reasonCodes"`
		PolicyTags              []string `json:"policyTags"`
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
		Verification struct {
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
	if ticket.WorkerSpecPath != bundle.WorkerSpecPath {
		t.Fatalf("worker spec path mismatch: ticket=%s bundle=%s", ticket.WorkerSpecPath, bundle.WorkerSpecPath)
	}
	if len(ticket.Verification.Commands) != 1 {
		t.Fatalf("expected one verification command, got %d", len(ticket.Verification.Commands))
	}
	if len(ticket.ReasonCodes) == 0 || len(ticket.PolicyTags) == 0 {
		t.Fatalf("expected route reason codes and policy tags in ticket: %+v", ticket)
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
	if ticket.JudgeDecision.JudgeID != "packet-judge" || ticket.JudgeDecision.SelectedFlow != "debugging-first bounded packet" {
		t.Fatalf("judge decision missing from ticket: %+v", ticket.JudgeDecision)
	}
	if len(ticket.JudgeDecision.SelectedDimensions) == 0 || len(ticket.JudgeDecision.SelectedLensIDs) < 6 || !ticket.JudgeDecision.ReviewRequired || !ticket.JudgeDecision.VerifyRequired {
		t.Fatalf("judge decision incomplete: %+v", ticket.JudgeDecision)
	}
	if ticket.ExecutionLoop.Mode != "qiushi execution / validation loop" || len(ticket.ExecutionLoop.Phases) != 6 || !strings.Contains(ticket.ExecutionLoop.SkillPath, filepath.Join("skills", "qiushi-execution", "SKILL.md")) {
		t.Fatalf("execution loop contract missing from ticket: %+v", ticket.ExecutionLoop)
	}
	if ticket.ConstraintSystem.Mode != "two-level layered constraints" || len(ticket.ConstraintSystem.Rules) < 8 {
		t.Fatalf("constraint system missing from ticket: %+v", ticket.ConstraintSystem)
	}

	var workerSpec struct {
		SchemaVersion     string   `json:"schemaVersion"`
		DispatchID        string   `json:"dispatchId"`
		TaskID            string   `json:"taskId"`
		ThreadKey         string   `json:"threadKey"`
		PlanEpoch         int      `json:"planEpoch"`
		Objective         string   `json:"objective"`
		SelectedPlan      string   `json:"selectedPlan"`
		AcceptanceMarkers []string `json:"acceptanceMarkers"`
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
	if workerSpec.ThreadKey != "thread-1" || workerSpec.PlanEpoch != 3 {
		t.Fatalf("worker spec missing lineage: %+v", workerSpec)
	}
	if workerSpec.Objective == "" || workerSpec.SelectedPlan == "" || len(workerSpec.AcceptanceMarkers) != 1 {
		t.Fatalf("worker spec missing execution contract: %+v", workerSpec)
	}
	prompt, err := os.ReadFile(bundle.PromptPath)
	if err != nil {
		t.Fatalf("read prompt: %v", err)
	}
	promptText := string(prompt)
	if !strings.Contains(promptText, bundle.TicketPath) {
		t.Fatalf("prompt missing ticket path: %s", promptText)
	}
	if !strings.Contains(promptText, bundle.WorkerSpecPath) {
		t.Fatalf("prompt missing worker spec path: %s", promptText)
	}
	if !strings.Contains(promptText, bundle.PlanningTracePath) {
		t.Fatalf("prompt missing planning trace path: %s", promptText)
	}
	if !strings.Contains(promptText, "Final response:") {
		t.Fatalf("prompt missing worker close-out contract")
	}
	if !strings.Contains(promptText, "After those reads, move to execution in owned paths.") {
		t.Fatalf("prompt missing simplified execution handoff")
	}
	if !strings.Contains(promptText, "Do not recreate planning or orchestration inside this task unless the ticket is internally inconsistent.") {
		t.Fatalf("prompt missing no-replan guidance")
	}
	if !strings.Contains(promptText, "metadata-backed B3Ehive") {
		t.Fatalf("prompt missing visible B3Ehive guidance")
	}
	if !strings.Contains(promptText, "Visible orchestration layer for this dispatch:") {
		t.Fatalf("prompt missing orchestration layer section")
	}
	if !strings.Contains(promptText, "executionLoopMode: qiushi execution / validation loop") {
		t.Fatalf("prompt missing qiushi execution loop guidance")
	}
	if !strings.Contains(promptText, "Soft constraints appended after the base prompt:") {
		t.Fatalf("prompt missing soft constraints section")
	}
	if !strings.Contains(promptText, "Hard constraints verified item-by-item by runtime / verify:") {
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
	if !strings.Contains(promptText, "Policy guardrails from route reasonCodes:") {
		t.Fatalf("prompt missing policy guardrail section")
	}
	if !strings.Contains(promptText, "Bug / failure flow: reproduce or capture concrete failure evidence before editing.") {
		t.Fatalf("prompt missing bug guardrail guidance")
	}
	if !strings.Contains(promptText, ".harness/state/current.json") {
		t.Fatalf("prompt missing resume state guidance")
	}
	if !strings.Contains(promptText, filepath.Join("prompts", "spec", "apply.md")) {
		t.Fatalf("prompt missing apply workflow path")
	}
	if !strings.Contains(promptText, filepath.Join("prompts", "spec", "verify.md")) {
		t.Fatalf("prompt missing verify workflow path")
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
	if !strings.Contains(planningText, "RCA First") || !strings.Contains(planningText, "State First Resume") || !strings.Contains(planningText, "Review Before Done") {
		t.Fatalf("planning trace missing policy-derived methodology lenses: %s", planningText)
	}
	if !strings.Contains(planningText, "## Judge Decision") || !strings.Contains(planningText, "selectedFlow: debugging-first bounded packet") {
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
	if !strings.Contains(promptText, "Outer-loop memory from verify/error sidecar:") {
		t.Fatalf("prompt missing outer-loop memory section: %s", promptText)
	}
	if !strings.Contains(promptText, "feedback-summary.json") {
		t.Fatalf("prompt missing feedback summary path: %s", promptText)
	}
	if !strings.Contains(promptText, "Identify the exact acceptance mismatch before changing code again.") {
		t.Fatalf("prompt missing thinking summary: %s", promptText)
	}
}
