package worker

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"klein-harness/internal/dispatch"
)

func TestPrepareWritesManifestAndPrompt(t *testing.T) {
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
		DispatchID:      "dispatch_T-1_3_1",
		TaskID:          "T-1",
		PlanEpoch:       3,
		PromptRef:       "prompts/worker-burst.md",
		ResumeSessionID: "sess-1",
	}, "lease-1")
	if err != nil {
		t.Fatalf("prepare bundle: %v", err)
	}

	var manifest struct {
		DispatchID              string `json:"dispatchId"`
		LeaseID                 string `json:"leaseId"`
		TaskID                  string `json:"taskId"`
		RepoRole                string `json:"repoRole"`
		DirectTargetEditAllowed bool   `json:"directTargetEditAllowed"`
		ArtifactDir             string `json:"artifactDir"`
		SpecPlanning            struct {
			PlannerCount int `json:"plannerCount"`
			Judge        struct {
				ID string `json:"id"`
			} `json:"judge"`
		} `json:"specPlanning"`
		Verification struct {
			Commands []map[string]any `json:"commands"`
		} `json:"verification"`
	}
	payload, err := os.ReadFile(bundle.ManifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	if err := json.Unmarshal(payload, &manifest); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}
	if manifest.DispatchID != "dispatch_T-1_3_1" || manifest.LeaseID != "lease-1" || manifest.TaskID != "T-1" {
		t.Fatalf("unexpected manifest identity: %+v", manifest)
	}
	if manifest.RepoRole != "body_repo" || manifest.DirectTargetEditAllowed {
		t.Fatalf("project meta not propagated: %+v", manifest)
	}
	if manifest.ArtifactDir != bundle.ArtifactDir {
		t.Fatalf("artifact dir mismatch: manifest=%s bundle=%s", manifest.ArtifactDir, bundle.ArtifactDir)
	}
	if len(manifest.Verification.Commands) != 1 {
		t.Fatalf("expected one verification command, got %d", len(manifest.Verification.Commands))
	}
	if manifest.SpecPlanning.PlannerCount != 3 || manifest.SpecPlanning.Judge.ID != "spec-judge" {
		t.Fatalf("spec planning contract missing: %+v", manifest.SpecPlanning)
	}
	prompt, err := os.ReadFile(bundle.PromptPath)
	if err != nil {
		t.Fatalf("read prompt: %v", err)
	}
	promptText := string(prompt)
	if !strings.Contains(promptText, bundle.ManifestPath) {
		t.Fatalf("prompt missing manifest path: %s", promptText)
	}
	if !strings.Contains(promptText, "Final response:") {
		t.Fatalf("prompt missing worker close-out contract")
	}
	if !strings.Contains(promptText, "context assembly -> targeted research -> plan -> execute -> verify -> handoff") {
		t.Fatalf("prompt missing orchestration loop")
	}
	if !strings.Contains(promptText, "default 3+1 spec loop") {
		t.Fatalf("prompt missing 3+1 spec loop")
	}
	if !strings.Contains(promptText, filepath.Join("prompts", "spec", "orchestrator.md")) {
		t.Fatalf("prompt missing spec orchestrator path")
	}
	if !strings.Contains(promptText, filepath.Join("prompts", "spec", "proposal.md")) {
		t.Fatalf("prompt missing spec proposal path")
	}
}
