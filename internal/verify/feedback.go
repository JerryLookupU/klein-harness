package verify

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"klein-harness/internal/adapter"
	"klein-harness/internal/dispatch"
	"klein-harness/internal/state"
)

type FeedbackEvent struct {
	ID                     string   `json:"id"`
	TaskID                 string   `json:"taskId"`
	DispatchID             string   `json:"dispatchId,omitempty"`
	PlanEpoch              int      `json:"planEpoch,omitempty"`
	Attempt                int      `json:"attempt,omitempty"`
	SessionID              string   `json:"sessionId,omitempty"`
	Role                   string   `json:"role,omitempty"`
	WorkerMode             string   `json:"workerMode,omitempty"`
	FeedbackType           string   `json:"feedbackType"`
	Severity               string   `json:"severity"`
	Source                 string   `json:"source"`
	Step                   string   `json:"step"`
	TriggeringAction       string   `json:"triggeringAction,omitempty"`
	Message                string   `json:"message"`
	ThinkingSummary        string   `json:"thinkingSummary,omitempty"`
	NextAction             string   `json:"nextAction,omitempty"`
	BurstStatus            string   `json:"burstStatus,omitempty"`
	VerificationStatus     string   `json:"verificationStatus,omitempty"`
	FollowUp               string   `json:"followUp,omitempty"`
	VerificationResultPath string   `json:"verificationResultPath,omitempty"`
	MissingArtifacts       []string `json:"missingArtifacts,omitempty"`
	ChangedPaths           []string `json:"changedPaths,omitempty"`
	EvidenceRefs           []string `json:"evidenceRefs,omitempty"`
	Timestamp              string   `json:"timestamp"`
}

type TaskFeedbackSummary struct {
	TaskID                string          `json:"taskId"`
	FeedbackCount         int             `json:"feedbackCount"`
	ErrorCount            int             `json:"errorCount"`
	CriticalCount         int             `json:"criticalCount"`
	LatestFeedbackType    string          `json:"latestFeedbackType,omitempty"`
	LatestSeverity        string          `json:"latestSeverity,omitempty"`
	LatestMessage         string          `json:"latestMessage,omitempty"`
	LatestThinkingSummary string          `json:"latestThinkingSummary,omitempty"`
	LatestNextAction      string          `json:"latestNextAction,omitempty"`
	LatestTimestamp       string          `json:"latestTimestamp,omitempty"`
	RecentFailures        []FeedbackEvent `json:"recentFailures,omitempty"`
}

type FeedbackSummary struct {
	SchemaVersion           string                         `json:"schemaVersion"`
	Generator               string                         `json:"generator"`
	GeneratedAt             string                         `json:"generatedAt"`
	FeedbackLogPath         string                         `json:"feedbackLogPath"`
	FeedbackEventCount      int                            `json:"feedbackEventCount"`
	ErrorCount              int                            `json:"errorCount"`
	CriticalCount           int                            `json:"criticalCount"`
	IllegalActionCount      int                            `json:"illegalActionCount"`
	TasksWithRecentFailures []string                       `json:"tasksWithRecentFailures,omitempty"`
	ByType                  map[string]int                 `json:"byType"`
	BySeverity              map[string]int                 `json:"bySeverity"`
	RecentFailures          []FeedbackEvent                `json:"recentFailures,omitempty"`
	TaskFeedbackSummary     map[string]TaskFeedbackSummary `json:"taskFeedbackSummary,omitempty"`
}

type OuterLoopMemoryInput struct {
	Task                   adapter.Task
	Ticket                 dispatch.Ticket
	SessionID              string
	BurstStatus            string
	BurstSummary           string
	VerifyStatus           string
	VerifySummary          string
	FollowUp               string
	VerificationResultPath string
	MissingArtifacts       []string
	EvidenceRefs           []string
}

func RecordOuterLoopMemory(root string, input OuterLoopMemoryInput) (FeedbackEvent, error) {
	event := classifyOuterLoopMemory(root, input)
	if event.FeedbackType == "" {
		return FeedbackEvent{}, nil
	}
	logPath, err := feedbackLogPath(root)
	if err != nil {
		return FeedbackEvent{}, err
	}
	count, err := countJSONLLines(logPath)
	if err != nil {
		return FeedbackEvent{}, err
	}
	event.ID = fmt.Sprintf("FB-%05d", count+1)
	event.Timestamp = state.NowUTC()
	payload, err := json.Marshal(event)
	if err != nil {
		return FeedbackEvent{}, err
	}
	handle, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return FeedbackEvent{}, err
	}
	defer handle.Close()
	if _, err := handle.Write(append(payload, '\n')); err != nil {
		return FeedbackEvent{}, err
	}
	if err := refreshFeedbackSummary(root); err != nil {
		return FeedbackEvent{}, err
	}
	return event, nil
}

func LoadFeedbackSummary(root string) (FeedbackSummary, error) {
	path, err := feedbackSummaryPath(root)
	if err != nil {
		return FeedbackSummary{}, err
	}
	var summary FeedbackSummary
	if ok, err := state.LoadJSONIfExists(path, &summary); err != nil {
		return FeedbackSummary{}, err
	} else if !ok {
		logPath, pathErr := feedbackLogPath(root)
		if pathErr != nil {
			return FeedbackSummary{}, pathErr
		}
		return FeedbackSummary{
			SchemaVersion:       "kh.feedback-summary.v1",
			Generator:           "kh-runtime",
			GeneratedAt:         state.NowUTC(),
			FeedbackLogPath:     filepath.ToSlash(logPath),
			ByType:              map[string]int{},
			BySeverity:          map[string]int{},
			TaskFeedbackSummary: map[string]TaskFeedbackSummary{},
		}, nil
	}
	if summary.ByType == nil {
		summary.ByType = map[string]int{}
	}
	if summary.BySeverity == nil {
		summary.BySeverity = map[string]int{}
	}
	if summary.TaskFeedbackSummary == nil {
		summary.TaskFeedbackSummary = map[string]TaskFeedbackSummary{}
	}
	return summary, nil
}

func CurrentTaskFeedback(summary FeedbackSummary, taskID string) (TaskFeedbackSummary, bool) {
	item, ok := summary.TaskFeedbackSummary[taskID]
	return item, ok
}

func feedbackLogPath(root string) (string, error) {
	paths, err := adapter.Resolve(root)
	if err != nil {
		return "", err
	}
	return filepath.Join(paths.HarnessDir, "feedback-log.jsonl"), nil
}

func feedbackSummaryPath(root string) (string, error) {
	paths, err := adapter.Resolve(root)
	if err != nil {
		return "", err
	}
	return filepath.Join(paths.StateDir, "feedback-summary.json"), nil
}

func classifyOuterLoopMemory(root string, input OuterLoopMemoryInput) FeedbackEvent {
	message := strings.TrimSpace(coalesce(input.VerifySummary, input.BurstSummary))
	if message == "" {
		return FeedbackEvent{}
	}
	changed := changedPaths(root)
	if len(changed) == 0 {
		changed = changedPathsFromWorkerResult(filepath.Join(filepath.Dir(input.VerificationResultPath), "worker-result.json"))
	}
	learning, _ := loadLearningState(root)
	thought := "Re-read the failing evidence, keep one active hypothesis, and re-enter analysis before the next execution."
	nextAction := "Inspect the latest verify evidence, task-local artifacts, and tmux log before issuing the next bounded dispatch."
	feedbackType := "replan_required"
	severity := "warning"
	source := "runtime"
	step := "analysis"
	trigger := coalesce(input.FollowUp, "analysis.required")
	switch {
	case len(input.MissingArtifacts) > 0:
		feedbackType = "verification_failure"
		severity = "error"
		source = "verification"
		step = "closeout"
		trigger = "closeout artifacts missing"
		thought = "Closeout artifacts were incomplete. Analyze why verify.json, worker-result.json, or handoff.md did not land before re-execution."
		nextAction = "Before the next burst, inspect the artifact directory and require the worker to write closeout artifacts immediately after validation."
	case input.VerifyStatus == "failed" || input.VerifyStatus == "blocked":
		feedbackType = "verification_failure"
		severity = "error"
		source = "verification"
		step = "verify"
		trigger = coalesce(input.VerifyStatus, "verification_failed")
		thought = "Verification rejected the current output. Identify the exact acceptance mismatch before changing code again."
		nextAction = "Read the verification result and narrow the next execution to the smallest change that addresses the failing evidence."
	case input.BurstStatus == "timed_out":
		feedbackType = "timeout"
		severity = "error"
		source = "worker"
		step = "execute"
		trigger = "burst timed out"
		thought = "The worker timed out before converging. Use the tmux log and checkpoint to find the last concrete step reached."
		nextAction = "Trim the next slice or tighten the prompt so the worker reaches verify/closeout within budget."
	case strings.Contains(strings.ToLower(input.BurstSummary), "ownedpath") || strings.Contains(strings.ToLower(input.BurstSummary), "outside task ownedpaths"):
		feedbackType = "path_conflict"
		severity = "critical"
		source = "worker"
		step = "execute"
		trigger = "owned path violation"
		thought = "Execution crossed the authority boundary. Re-check ownedPaths and request replan instead of widening scope in-place."
		nextAction = "Keep the next attempt inside ownedPaths or change the plan through route/judge before execution."
	case input.BurstStatus == "failed":
		feedbackType = "execution_error"
		severity = "error"
		source = "worker"
		step = "execute"
		trigger = "bounded burst failed"
		thought = "Execution failed before acceptable evidence was produced. Confirm the failure symptom and keep one active hypothesis."
		nextAction = "Inspect tmux log, checkpoint, and changed files, then send one bounded repair slice back into execution."
	default:
		return FeedbackEvent{}
	}
	hints := learningHints(learning)
	if len(hints) > 0 {
		thought = thought + " Learned reminder: " + hints[0]
	}
	return FeedbackEvent{
		TaskID:                 input.Task.TaskID,
		DispatchID:             input.Ticket.DispatchID,
		PlanEpoch:              input.Ticket.PlanEpoch,
		Attempt:                input.Ticket.Attempt,
		SessionID:              input.SessionID,
		Role:                   coalesce(input.Task.RoleHint, "worker"),
		WorkerMode:             coalesce(input.Task.WorkerMode, "execution"),
		FeedbackType:           feedbackType,
		Severity:               severity,
		Source:                 source,
		Step:                   step,
		TriggeringAction:       trigger,
		Message:                message,
		ThinkingSummary:        thought,
		NextAction:             nextAction,
		BurstStatus:            input.BurstStatus,
		VerificationStatus:     input.VerifyStatus,
		FollowUp:               input.FollowUp,
		VerificationResultPath: input.VerificationResultPath,
		MissingArtifacts:       append([]string(nil), input.MissingArtifacts...),
		ChangedPaths:           changed,
		EvidenceRefs:           uniqueNonEmpty(input.EvidenceRefs...),
	}
}

func refreshFeedbackSummary(root string) error {
	events, err := loadFeedbackEvents(root)
	if err != nil {
		return err
	}
	logPath, err := feedbackLogPath(root)
	if err != nil {
		return err
	}
	summary := FeedbackSummary{
		SchemaVersion:       "kh.feedback-summary.v1",
		Generator:           "kh-runtime",
		GeneratedAt:         state.NowUTC(),
		FeedbackLogPath:     filepath.ToSlash(logPath),
		ByType:              map[string]int{},
		BySeverity:          map[string]int{},
		TaskFeedbackSummary: map[string]TaskFeedbackSummary{},
	}
	for _, event := range events {
		summary.FeedbackEventCount++
		summary.ByType[event.FeedbackType]++
		summary.BySeverity[event.Severity]++
		if severityRank(event.Severity) >= severityRank("error") {
			summary.ErrorCount++
		}
		if event.Severity == "critical" {
			summary.CriticalCount++
		}
		if event.FeedbackType == "illegal_action" {
			summary.IllegalActionCount++
		}
		taskSummary := summary.TaskFeedbackSummary[event.TaskID]
		taskSummary.TaskID = event.TaskID
		taskSummary.FeedbackCount++
		if severityRank(event.Severity) >= severityRank("error") {
			taskSummary.ErrorCount++
			taskSummary.RecentFailures = append(taskSummary.RecentFailures, event)
		}
		if event.Severity == "critical" {
			taskSummary.CriticalCount++
		}
		taskSummary.LatestFeedbackType = event.FeedbackType
		taskSummary.LatestSeverity = event.Severity
		taskSummary.LatestMessage = event.Message
		taskSummary.LatestThinkingSummary = event.ThinkingSummary
		taskSummary.LatestNextAction = event.NextAction
		taskSummary.LatestTimestamp = event.Timestamp
		summary.TaskFeedbackSummary[event.TaskID] = taskSummary
		if severityRank(event.Severity) >= severityRank("error") {
			summary.RecentFailures = append(summary.RecentFailures, event)
		}
	}
	for taskID, taskSummary := range summary.TaskFeedbackSummary {
		if len(taskSummary.RecentFailures) > 0 {
			summary.TasksWithRecentFailures = append(summary.TasksWithRecentFailures, taskID)
		}
		if len(taskSummary.RecentFailures) > 3 {
			taskSummary.RecentFailures = taskSummary.RecentFailures[len(taskSummary.RecentFailures)-3:]
		}
		summary.TaskFeedbackSummary[taskID] = taskSummary
	}
	sort.Strings(summary.TasksWithRecentFailures)
	if len(summary.RecentFailures) > 5 {
		summary.RecentFailures = summary.RecentFailures[len(summary.RecentFailures)-5:]
	}
	path, err := feedbackSummaryPath(root)
	if err != nil {
		return err
	}
	return writeJSONFile(path, summary)
}

func loadFeedbackEvents(root string) ([]FeedbackEvent, error) {
	path, err := feedbackLogPath(root)
	if err != nil {
		return nil, err
	}
	handle, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer handle.Close()
	out := make([]FeedbackEvent, 0)
	scanner := bufio.NewScanner(handle)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var event FeedbackEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}
		out = append(out, event)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func countJSONLLines(path string) (int, error) {
	handle, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	defer handle.Close()
	count := 0
	scanner := bufio.NewScanner(handle)
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) != "" {
			count++
		}
	}
	return count, scanner.Err()
}

func severityRank(value string) int {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "critical":
		return 3
	case "error":
		return 2
	case "warning":
		return 1
	default:
		return 0
	}
}

func coalesce(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
