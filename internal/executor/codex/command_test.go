package codex

import (
	"strings"
	"testing"

	"klein-harness/internal/codexconfig"
)

func TestBuildFreshUsesNativeCodexExec(t *testing.T) {
	command := BuildFresh(codexconfig.Profile{
		Model:          "gpt-5.3-codex",
		ApprovalPolicy: "never",
		SandboxMode:    "workspace-write",
	}, BuildOptions{
		SkipGitRepoCheck:      true,
		OutputLastMessagePath: "/tmp/last-message.txt",
		AdditionalWritable:    []string{"docs", "src"},
	})
	for _, want := range []string{
		"codex exec",
		"--json",
		"--output-last-message /tmp/last-message.txt",
		"--model gpt-5.3-codex",
		"--full-auto",
		"--skip-git-repo-check",
		"--add-dir docs",
		"--add-dir src",
	} {
		if !strings.Contains(command, want) {
			t.Fatalf("expected %q in %q", want, command)
		}
	}
}

func TestBuildResumeUsesNativeResumeCommand(t *testing.T) {
	command := BuildResume(codexconfig.Profile{
		Model:          "gpt-5.4",
		ApprovalPolicy: "never",
		SandboxMode:    "danger-full-access",
	}, BuildOptions{
		SessionID:             "sess-123",
		OutputLastMessagePath: "/tmp/out.txt",
	})
	for _, want := range []string{
		"codex exec resume sess-123",
		"--output-last-message /tmp/out.txt",
		"--model gpt-5.4",
		"--dangerously-bypass-approvals-and-sandbox",
	} {
		if !strings.Contains(command, want) {
			t.Fatalf("expected %q in %q", want, command)
		}
	}
}
