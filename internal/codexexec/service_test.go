package codexexec

import (
	"strings"
	"testing"

	"klein-harness/internal/codexconfig"
	"klein-harness/internal/instructions"
)

func TestBuildCommandProfileUsesCodexProtocol(t *testing.T) {
	profile := codexconfig.Profile{
		Model:          "gpt-5.4",
		ApprovalPolicy: "never",
		SandboxMode:    "workspace-write",
	}
	command := buildCommandProfile(false, Request{SkipGitRepoCheck: true}, profile)
	for _, want := range []string{
		"codex exec",
		"--json",
		"--output-last-message <LAST_MESSAGE_PATH>",
		"--model gpt-5.4",
		"--full-auto",
		"--skip-git-repo-check",
	} {
		if !strings.Contains(command, want) {
			t.Fatalf("command missing %q: %s", want, command)
		}
	}
	resumeCommand := buildCommandProfile(true, Request{}, profile)
	if !strings.Contains(resumeCommand, "codex exec resume <SESSION_ID>") {
		t.Fatalf("resume command missing session placeholder: %s", resumeCommand)
	}
}

func TestDetectNativeSessionIDPrefersNewEntry(t *testing.T) {
	before := []sessionIndexEntry{
		{ID: "sess-1"},
	}
	after := []sessionIndexEntry{
		{ID: "sess-1"},
		{ID: "sess-2"},
	}
	if got := detectNativeSessionID(before, after, ""); got != "sess-2" {
		t.Fatalf("unexpected detected session id: %s", got)
	}
	if got := detectNativeSessionID(before, before, "fallback"); got != "fallback" {
		t.Fatalf("unexpected fallback session id: %s", got)
	}
}

func TestBuildTaskUsesDefaultSpecConvergencePrompt(t *testing.T) {
	task := buildTask("/repo", Request{Prompt: "Add orchestration convergence."}, sessionRecord{
		ID:                     "sess-1",
		TaskID:                 "task-1",
		OrchestrationSessionID: "orch-1",
	}, "orch-1", codexconfig.Profile{
		Model:          "gpt-5.4",
		ApprovalPolicy: "never",
		SandboxMode:    "workspace-write",
	}, []instructions.File{{Path: "/repo/AGENTS.md"}})
	if !strings.Contains(task.Description, "OpenSpec") || !strings.Contains(task.Description, "b3ehive") {
		t.Fatalf("task description missing orchestration defaults: %s", task.Description)
	}
	if len(task.PromptStages) < 4 || task.PromptStages[1] != "spec_parallel_planning" {
		t.Fatalf("unexpected prompt stages: %+v", task.PromptStages)
	}
}
