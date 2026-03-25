package codex

import (
	"strings"

	"klein-harness/internal/codexconfig"
)

type BuildOptions struct {
	SessionID             string
	SkipGitRepoCheck      bool
	OutputLastMessagePath string
	AdditionalWritable    []string
}

func BuildFresh(profile codexconfig.Profile, options BuildOptions) string {
	return build(false, profile, options)
}

func BuildResume(profile codexconfig.Profile, options BuildOptions) string {
	return build(true, profile, options)
}

func build(resume bool, profile codexconfig.Profile, options BuildOptions) string {
	args := []string{"codex", "exec"}
	if resume {
		sessionID := strings.TrimSpace(options.SessionID)
		if sessionID == "" {
			sessionID = "<SESSION_ID>"
		}
		args = append(args, "resume", sessionID)
	}
	lastMessagePath := strings.TrimSpace(options.OutputLastMessagePath)
	if lastMessagePath == "" {
		lastMessagePath = "<LAST_MESSAGE_PATH>"
	}
	args = append(args, "--json", "--output-last-message", lastMessagePath)
	if strings.TrimSpace(profile.Model) != "" {
		args = append(args, "--model", profile.Model)
	}
	switch {
	case profile.SandboxMode == "danger-full-access" && profile.ApprovalPolicy == "never":
		args = append(args, "--dangerously-bypass-approvals-and-sandbox")
	case profile.SandboxMode == "workspace-write" && (profile.ApprovalPolicy == "never" || profile.ApprovalPolicy == "on-request" || profile.ApprovalPolicy == ""):
		args = append(args, "--full-auto")
	case strings.TrimSpace(profile.SandboxMode) != "":
		args = append(args, "--sandbox", profile.SandboxMode)
	}
	if options.SkipGitRepoCheck {
		args = append(args, "--skip-git-repo-check")
	}
	for _, path := range options.AdditionalWritable {
		if strings.TrimSpace(path) == "" {
			continue
		}
		args = append(args, "--add-dir", path)
	}
	return strings.Join(args, " ")
}
