package tmux

import (
	"bytes"
	"os/exec"
	"strings"

	executortmux "klein-harness/internal/executor/tmux"
)

func CreateDetachedSession(sessionName, cwd string) error {
	return runTmux(executortmux.BuildNewSession(sessionName, cwd)...)
}

func SetRemainOnExit(sessionName string) error {
	return runTmux(executortmux.BuildSetRemainOnExit(sessionName)...)
}

func PipePane(sessionName, logPath string) error {
	return runTmux(executortmux.BuildPipePane(sessionName, logPath)...)
}

func SendCommand(sessionName, command string) error {
	return runTmux(executortmux.BuildSendKeys(sessionName, command)...)
}

func CapturePane(sessionName string) (string, error) {
	command := exec.Command("tmux", executortmux.BuildCapturePane(sessionName)...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return "", tmuxError(err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

func HasSession(sessionName string) bool {
	command := exec.Command("tmux", executortmux.BuildHasSession(sessionName)...)
	return command.Run() == nil
}

func KillSession(sessionName string) error {
	if strings.TrimSpace(sessionName) == "" || !HasSession(sessionName) {
		return nil
	}
	return runTmux(executortmux.BuildKillSession(sessionName)...)
}

func AttachSession(sessionName string) error {
	command := exec.Command("tmux", executortmux.BuildAttachSession(sessionName)...)
	command.Stdin = nil
	command.Stdout = nil
	command.Stderr = nil
	return command.Run()
}

func AttachCommand(sessionName string) string {
	return "tmux attach-session -t " + sessionName
}

func runTmux(args ...string) error {
	command := exec.Command("tmux", args...)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return tmuxError(err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

func tmuxError(err error, stderr string) error {
	if stderr == "" {
		return err
	}
	return &execError{cause: err, stderr: stderr}
}

type execError struct {
	cause  error
	stderr string
}

func (e *execError) Error() string {
	return "tmux: " + e.stderr
}

func (e *execError) Unwrap() error {
	return e.cause
}
