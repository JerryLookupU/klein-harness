package tmux

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"klein-harness/internal/adapter"
	"klein-harness/internal/dispatch"
	"klein-harness/internal/state"
)

type BurstRequest struct {
	Root           string
	TaskID         string
	DispatchID     string
	WorkerID       string
	Cwd            string
	Command        string
	PromptPath     string
	Budget         dispatch.Budget
	CheckpointPath string
	OutcomePath    string
	Artifacts      []string
	SessionName    string
	LogPath        string
}

type Result struct {
	SchemaVersion string         `json:"schemaVersion"`
	Generator     string         `json:"generator"`
	GeneratedAt   string         `json:"generatedAt"`
	Status        string         `json:"status"`
	Summary       string         `json:"summary"`
	ExitCode      int            `json:"exitCode"`
	StartedAt     string         `json:"startedAt"`
	FinishedAt    string         `json:"finishedAt"`
	DurationSec   int64          `json:"durationSec"`
	Stdout        string         `json:"stdout"`
	Stderr        string         `json:"stderr"`
	DiffStats     map[string]int `json:"diffStats"`
	Artifacts     []string       `json:"artifacts"`
	SessionName   string         `json:"sessionName,omitempty"`
	LogPath       string         `json:"logPath,omitempty"`
}

func RunBoundedBurst(request BurstRequest) (Result, error) {
	now := time.Now().UTC()
	result := Result{
		SchemaVersion: "1.0",
		Generator:     "kh-worker-supervisor",
		GeneratedAt:   now.Format(time.RFC3339),
		StartedAt:     now.Format(time.RFC3339),
		DiffStats: map[string]int{
			"filesChanged": 0,
			"insertions":   0,
			"deletions":    0,
		},
		Artifacts: []string{},
	}
	if err := os.MkdirAll(filepath.Dir(request.CheckpointPath), 0o755); err != nil {
		return result, err
	}
	if err := os.MkdirAll(filepath.Dir(request.OutcomePath), 0o755); err != nil {
		return result, err
	}
	if request.SessionName == "" {
		request.SessionName = defaultSessionName(request.Root, request.Cwd, request.TaskID, request.DispatchID)
	}
	if request.LogPath == "" {
		request.LogPath = defaultLogPath(request)
	}
	if err := os.MkdirAll(filepath.Dir(request.LogPath), 0o755); err != nil {
		return result, err
	}
	result.SessionName = request.SessionName
	result.LogPath = request.LogPath
	if err := writeTmuxSummary(request.Root, SessionState{
		SessionName:   request.SessionName,
		TaskID:        request.TaskID,
		DispatchID:    request.DispatchID,
		WorkerID:      request.WorkerID,
		Cwd:           request.Cwd,
		LogPath:       request.LogPath,
		CheckpointRef: request.CheckpointPath,
		OutcomeRef:    request.OutcomePath,
		Status:        "starting",
		StartedAt:     result.StartedAt,
		AttachCommand: AttachCommand(request.SessionName),
	}); err != nil {
		return result, err
	}
	if err := writeJSON(request.CheckpointPath, map[string]any{
		"schemaVersion": "1.0",
		"generator":     "kh-worker-supervisor",
		"generatedAt":   result.GeneratedAt,
		"taskId":        request.TaskID,
		"dispatchId":    request.DispatchID,
		"workerId":      request.WorkerID,
		"cwd":           request.Cwd,
		"command":       request.Command,
		"promptPath":    request.PromptPath,
		"sessionName":   request.SessionName,
		"logPath":       request.LogPath,
	}); err != nil {
		return result, err
	}
	maxMinutes := request.Budget.MaxMinutes
	if maxMinutes <= 0 {
		maxMinutes = 20
	}
	exitCodePath := filepath.Join(filepath.Dir(request.OutcomePath), "tmux-exit-code")
	runnerPath := filepath.Join(filepath.Dir(request.OutcomePath), "tmux-run.sh")
	if err := os.WriteFile(runnerPath, []byte(buildRunnerScript(request.Cwd, request.Command, request.PromptPath, exitCodePath)), 0o755); err != nil {
		return result, err
	}
	if err := CreateDetachedSession(request.SessionName, request.Cwd); err != nil {
		return result, err
	}
	if err := SetRemainOnExit(request.SessionName); err != nil {
		return result, err
	}
	if err := PipePane(request.SessionName, request.LogPath); err != nil {
		return result, err
	}
	if err := SendCommand(request.SessionName, "sh "+shellQuote(runnerPath)); err != nil {
		return result, err
	}
	if err := writeTmuxSummary(request.Root, SessionState{
		SessionName:   request.SessionName,
		TaskID:        request.TaskID,
		DispatchID:    request.DispatchID,
		WorkerID:      request.WorkerID,
		Cwd:           request.Cwd,
		LogPath:       request.LogPath,
		CheckpointRef: request.CheckpointPath,
		OutcomeRef:    request.OutcomePath,
		Status:        "running",
		StartedAt:     result.StartedAt,
		AttachCommand: AttachCommand(request.SessionName),
	}); err != nil {
		return result, err
	}
	exitCode, timedOut, waitErr := waitForExit(exitCodePath, time.Duration(maxMinutes)*time.Minute)
	if timedOut {
		_ = KillSession(request.SessionName)
		result.ExitCode = 124
		result.Status = "timed_out"
		result.Summary = "bounded burst exceeded maxMinutes"
	} else if waitErr != nil {
		return result, waitErr
	} else {
		result.ExitCode = exitCode
	}
	finished := time.Now().UTC()
	result.FinishedAt = finished.Format(time.RFC3339)
	result.DurationSec = int64(finished.Sub(now).Seconds())
	result.Stdout = readLogTail(request.LogPath)
	if result.Status == "" {
		switch {
		case result.ExitCode != 0:
			result.Status = "failed"
			result.Summary = "bounded burst failed"
		default:
			result.Status = "succeeded"
			result.Summary = "bounded burst completed"
		}
	}
	if result.Status == "failed" {
		result.Status = "failed"
		result.Summary = "bounded burst failed"
	}
	result.DiffStats = collectDiffStats(request.Cwd)
	result.Artifacts = append(result.Artifacts, request.Artifacts...)
	result.Artifacts = append(result.Artifacts, request.CheckpointPath, request.OutcomePath, request.LogPath, runnerPath)
	if err := writeJSON(request.OutcomePath, result); err != nil {
		return result, err
	}
	if err := writeTmuxSummary(request.Root, SessionState{
		SessionName:   request.SessionName,
		TaskID:        request.TaskID,
		DispatchID:    request.DispatchID,
		WorkerID:      request.WorkerID,
		Cwd:           request.Cwd,
		LogPath:       request.LogPath,
		CheckpointRef: request.CheckpointPath,
		OutcomeRef:    request.OutcomePath,
		Status:        result.Status,
		StartedAt:     result.StartedAt,
		FinishedAt:    result.FinishedAt,
		ExitCode:      result.ExitCode,
		AttachCommand: AttachCommand(request.SessionName),
	}); err != nil {
		return result, err
	}
	return result, nil
}

func collectDiffStats(cwd string) map[string]int {
	stats := map[string]int{"filesChanged": 0, "insertions": 0, "deletions": 0}
	command := exec.Command("/bin/sh", "-lc", "git diff --numstat")
	command.Dir = cwd
	output, err := command.Output()
	if err != nil {
		return stats
	}
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}
		stats["filesChanged"]++
		if value, err := strconv.Atoi(parts[0]); err == nil {
			stats["insertions"] += value
		}
		if value, err := strconv.Atoi(parts[1]); err == nil {
			stats["deletions"] += value
		}
	}
	return stats
}

func writeJSON(path string, value any) error {
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(payload, '\n'), 0o644)
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

func buildRunnerScript(cwd, command, promptPath, exitCodePath string) string {
	lines := []string{
		"#!/bin/sh",
		"set +e",
		"cd " + shellQuote(cwd),
	}
	if promptPath != "" {
		lines = append(lines, "/bin/sh -c "+shellQuote(command)+" < "+shellQuote(promptPath))
	} else {
		lines = append(lines, "/bin/sh -c "+shellQuote(command))
	}
	lines = append(lines,
		"code=$?",
		"printf '%s' \"$code\" > "+shellQuote(exitCodePath),
		"exit \"$code\"",
	)
	return strings.Join(lines, "\n") + "\n"
}

func waitForExit(path string, timeout time.Duration) (int, bool, error) {
	deadline := time.Now().Add(timeout)
	for {
		payload, err := os.ReadFile(path)
		if err == nil {
			code, parseErr := strconv.Atoi(strings.TrimSpace(string(payload)))
			if parseErr != nil {
				return 1, false, parseErr
			}
			return code, false, nil
		}
		if err != nil && !os.IsNotExist(err) {
			return 1, false, err
		}
		if time.Now().After(deadline) {
			return 124, true, nil
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func defaultSessionName(root, cwd, taskID, dispatchID string) string {
	scope := strings.TrimSpace(root)
	if scope == "" {
		scope = strings.TrimSpace(cwd)
	}
	base := fmt.Sprintf("kh_%s_%s_%s", scopeToken(scope), sanitize(taskID), sanitize(dispatchID))
	if len(base) > 80 {
		return base[:80]
	}
	return base
}

func defaultLogPath(request BurstRequest) string {
	if strings.TrimSpace(request.Root) != "" {
		paths, err := adapter.Resolve(request.Root)
		if err == nil {
			return filepath.Join(paths.TmuxLogsDir, request.TaskID, request.DispatchID+".log")
		}
	}
	return filepath.Join(filepath.Dir(request.OutcomePath), "tmux.log")
}

func sanitize(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "session"
	}
	replacer := strings.NewReplacer("/", "_", "\\", "_", ":", "_", ".", "_", " ", "_")
	return replacer.Replace(value)
}

func scopeToken(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "scope"
	}
	sum := sha1.Sum([]byte(value))
	return hex.EncodeToString(sum[:])[:8]
}

func readLogTail(path string) string {
	payload, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(payload)
}

func writeTmuxSummary(root string, session SessionState) error {
	if strings.TrimSpace(root) == "" {
		return nil
	}
	paths, err := adapter.Resolve(root)
	if err != nil {
		return err
	}
	summary, err := LoadSummary(root)
	if err != nil {
		return err
	}
	summary.Sessions[session.SessionName] = session
	if strings.TrimSpace(session.TaskID) != "" {
		summary.LatestByTask[session.TaskID] = session.SessionName
	}
	_, err = state.WriteSnapshot(paths.TmuxSummaryPath, &summary, "kh-worker-supervisor", summary.Revision)
	return err
}
