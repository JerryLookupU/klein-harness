package tmux

import "fmt"

func BuildNewSession(sessionName, cwd string) []string {
	args := []string{"new-session", "-d", "-s", sessionName}
	if cwd != "" {
		args = append(args, "-c", cwd)
	}
	return args
}

func BuildSetRemainOnExit(sessionName string) []string {
	return []string{"set-option", "-t", sessionName, "remain-on-exit", "on"}
}

func BuildPipePane(sessionName, logPath string) []string {
	return []string{"pipe-pane", "-o", "-t", sessionName, fmt.Sprintf("cat >> %s", shellQuote(logPath))}
}

func BuildSendKeys(sessionName, command string) []string {
	return []string{"send-keys", "-t", sessionName, command, "C-m"}
}

func BuildCapturePane(sessionName string) []string {
	return []string{"capture-pane", "-p", "-t", sessionName}
}

func BuildHasSession(sessionName string) []string {
	return []string{"has-session", "-t", sessionName}
}

func BuildKillSession(sessionName string) []string {
	return []string{"kill-session", "-t", sessionName}
}

func BuildAttachSession(sessionName string) []string {
	return []string{"attach-session", "-t", sessionName}
}

func shellQuote(value string) string {
	return "'" + replaceSingleQuotes(value) + "'"
}

func replaceSingleQuotes(value string) string {
	result := ""
	for _, r := range value {
		if r == '\'' {
			result += `'\''`
			continue
		}
		result += string(r)
	}
	return result
}
