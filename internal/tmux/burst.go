package tmux

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"klein-harness/internal/dispatch"
)

type BurstRequest struct {
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
	}); err != nil {
		return result, err
	}
	maxMinutes := request.Budget.MaxMinutes
	if maxMinutes <= 0 {
		maxMinutes = 20
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(maxMinutes)*time.Minute)
	defer cancel()

	commandText := request.Command
	if request.PromptPath != "" {
		commandText = fmt.Sprintf("%s < %s", request.Command, shellQuote(request.PromptPath))
	}
	command := exec.CommandContext(ctx, "/bin/sh", "-lc", commandText)
	command.Dir = request.Cwd
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	runErr := command.Run()
	finished := time.Now().UTC()
	result.FinishedAt = finished.Format(time.RFC3339)
	result.DurationSec = int64(finished.Sub(now).Seconds())
	result.Stdout = stdout.String()
	result.Stderr = stderr.String()
	result.ExitCode = exitCode(runErr)
	switch {
	case ctx.Err() == context.DeadlineExceeded:
		result.Status = "timed_out"
		result.Summary = "bounded burst exceeded maxMinutes"
	case runErr != nil:
		result.Status = "failed"
		result.Summary = "bounded burst failed"
	default:
		result.Status = "succeeded"
		result.Summary = "bounded burst completed"
	}
	result.DiffStats = collectDiffStats(request.Cwd)
	result.Artifacts = append(result.Artifacts, request.Artifacts...)
	result.Artifacts = append(result.Artifacts, request.CheckpointPath, request.OutcomePath)
	if err := writeJSON(request.OutcomePath, result); err != nil {
		return result, err
	}
	return result, nil
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode()
	}
	return 1
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
