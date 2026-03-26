package verify

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"klein-harness/internal/adapter"
	"klein-harness/internal/dispatch"
	"klein-harness/internal/state"
)

const (
	patternCloseoutComplete         = "closeout_complete"
	patternMissingCloseoutArtifacts = "missing_closeout_artifacts"
	patternMissingVerifyArtifact    = "missing_verify_artifact"
	patternMissingWorkerResult      = "missing_worker_result_artifact"
	patternMissingHandoffArtifact   = "missing_handoff_artifact"

	learningPromotionThreshold = 2
)

type HookChecklistItem struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Required bool   `json:"required"`
	Status   string `json:"status"`
	Detail   string `json:"detail,omitempty"`
}

type HookSpec struct {
	Name            string              `json:"name"`
	Enabled         bool                `json:"enabled"`
	Event           string              `json:"event"`
	Action          string              `json:"action"`
	Status          string              `json:"status"`
	Summary         string              `json:"summary,omitempty"`
	Checklist       []HookChecklistItem `json:"checklist,omitempty"`
	LearnedPatterns []string            `json:"learnedPatterns,omitempty"`
}

type HookPlan struct {
	Hooks             []HookSpec `json:"hooks"`
	AcceptanceMarkers []string   `json:"acceptanceMarkers"`
	LearningHints     []string   `json:"learningHints,omitempty"`
}

type LearningPattern struct {
	Count      int    `json:"count"`
	Promoted   bool   `json:"promoted"`
	LastSeenAt string `json:"lastSeenAt,omitempty"`
}

type LearningState struct {
	SchemaVersion string                     `json:"schemaVersion"`
	Generator     string                     `json:"generator"`
	UpdatedAt     string                     `json:"updatedAt"`
	Patterns      map[string]LearningPattern `json:"patterns"`
}

type CloseoutResult struct {
	Generated              bool     `json:"generated"`
	MissingArtifacts       []string `json:"missingArtifacts,omitempty"`
	GeneratedArtifacts     []string `json:"generatedArtifacts,omitempty"`
	Status                 string   `json:"status,omitempty"`
	Summary                string   `json:"summary,omitempty"`
	VerificationResultPath string   `json:"verificationResultPath,omitempty"`
}

func BuildHookPlan(root string, task adapter.Task, ticket dispatch.Ticket, verifyCommands []map[string]any) HookPlan {
	learning, _ := loadLearningState(root)
	acceptanceMarkers := uniqueNonEmpty(task.VerificationRuleIDs...)
	if len(acceptanceMarkers) == 0 {
		acceptanceMarkers = []string{
			"artifact:worker-result",
			"artifact:verify",
			"artifact:handoff",
		}
	}
	promotedCloseout := learning.Patterns[patternMissingCloseoutArtifacts].Promoted
	preflightAction := "warn"
	if promotedCloseout {
		preflightAction = "block"
	}
	preflightSummary := "dispatch preflight confirms the worker has an explicit closeout contract"
	if promotedCloseout {
		preflightSummary = "promoted closeout guard: recent runs missed verification artifacts, so closeout completeness is now a blocking expectation"
	}
	hooks := []HookSpec{
		{
			Name:    "verification-preflight",
			Enabled: true,
			Event:   "dispatch",
			Action:  preflightAction,
			Status:  "active",
			Summary: preflightSummary,
			Checklist: []HookChecklistItem{
				{
					ID:       "owned-paths",
					Title:    "owned paths are explicit",
					Required: true,
					Status:   passFail(len(task.OwnedPaths) > 0),
					Detail:   fmt.Sprintf("ownedPaths=%d", len(task.OwnedPaths)),
				},
				{
					ID:       "acceptance-markers",
					Title:    "acceptance markers are declared",
					Required: true,
					Status:   passFail(len(acceptanceMarkers) > 0),
					Detail:   fmt.Sprintf("markers=%s", strings.Join(acceptanceMarkers, ", ")),
				},
				{
					ID:       "verification-strategy",
					Title:    "verification strategy is declared",
					Required: true,
					Status:   passFail(len(verifyCommands) > 0 || len(task.VerificationRuleIDs) > 0),
					Detail:   fmt.Sprintf("commands=%d ruleIds=%d", len(verifyCommands), len(task.VerificationRuleIDs)),
				},
				{
					ID:       "required-artifacts",
					Title:    "required closeout artifacts are declared",
					Required: true,
					Status:   "pass",
					Detail:   "worker-result.json, verify.json, handoff.md",
				},
			},
			LearnedPatterns: learnedPatterns(learning),
		},
		{
			Name:    "verification-artifact-before-closeout",
			Enabled: true,
			Event:   "closeout",
			Action:  "block",
			Status:  "active",
			Summary: "if business output exists but closeout artifacts are missing, runtime must block completion and synthesize a hook report",
			Checklist: []HookChecklistItem{
				{ID: "worker-result", Title: "worker-result.json present", Required: true, Status: "required"},
				{ID: "verify", Title: "verify.json present", Required: true, Status: "required"},
				{ID: "handoff", Title: "handoff.md present", Required: true, Status: "required"},
			},
			LearnedPatterns: learnedPatterns(learning),
		},
		{
			Name:    "verification-check-before-stop",
			Enabled: true,
			Event:   "stop",
			Action:  preflightAction,
			Status:  "active",
			Summary: "final checklist before declaring the worker run terminal",
			Checklist: []HookChecklistItem{
				{ID: "verify-evidence", Title: "verification evidence recorded", Required: true, Status: "required"},
				{ID: "handoff", Title: "handoff recorded", Required: true, Status: "required"},
				{ID: "runtime-closeout", Title: "runtime completion gate left to runtime", Required: true, Status: "required"},
			},
			LearnedPatterns: learnedPatterns(learning),
		},
	}
	return HookPlan{
		Hooks:             hooks,
		AcceptanceMarkers: acceptanceMarkers,
		LearningHints:     learningHints(learning),
	}
}

func EnsureCloseoutArtifacts(root string, task adapter.Task, ticket dispatch.Ticket, artifactDir, logPath string, diffStats map[string]int, burstStatus, burstSummary string) (CloseoutResult, error) {
	required := map[string]string{
		"worker-result.json": filepath.Join(artifactDir, "worker-result.json"),
		"verify.json":        filepath.Join(artifactDir, "verify.json"),
		"handoff.md":         filepath.Join(artifactDir, "handoff.md"),
	}
	missing := make([]string, 0)
	generated := make([]string, 0)
	for name, path := range required {
		ok, err := fileHasContent(path)
		if err != nil {
			return CloseoutResult{}, err
		}
		if !ok {
			missing = append(missing, name)
		}
	}
	verifyPath := required["verify.json"]
	if len(missing) == 0 {
		_ = recordLearning(root, patternCloseoutComplete)
		return CloseoutResult{
			VerificationResultPath: verifyPath,
		}, nil
	}

	changedPaths := changedPaths(root)
	if len(changedPaths) == 0 {
		changedPaths = changedPathsFromWorkerResult(required["worker-result.json"])
	}

	status := "blocked"
	if burstStatus == "failed" || burstStatus == "timed_out" {
		status = "failed"
	}
	if len(changedPaths) == 0 && diffStats["filesChanged"] == 0 && status == "blocked" {
		status = "failed"
	}
	summary := fmt.Sprintf("closeout hook blocked completion because required artifacts were missing: %s", strings.Join(missing, ", "))
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		return CloseoutResult{}, err
	}

	if containsString(missing, "verify.json") {
		verifyPayload := map[string]any{
			"status":        status,
			"overallStatus": status,
			"summary":       summary,
			"commands": []map[string]any{
				{
					"name":   "git diff --name-only",
					"status": "observed",
					"output": changedPaths,
				},
			},
			"evidenceRefs": uniqueNonEmpty(
				verifyPath,
				required["worker-result.json"],
				required["handoff.md"],
				logPath,
			),
			"findings": []map[string]any{
				{
					"severity": "critical",
					"kind":     "closeout_missing_artifacts",
					"summary":  summary,
					"missing":  missing,
				},
			},
			"hook": map[string]any{
				"name":    "verification-artifact-before-closeout",
				"event":   "closeout",
				"action":  "block",
				"status":  "generated_by_runtime",
				"missing": missing,
			},
		}
		if err := writeJSONFile(verifyPath, verifyPayload); err != nil {
			return CloseoutResult{}, err
		}
		generated = append(generated, verifyPath)
	}
	if containsString(missing, "worker-result.json") {
		workerPayload := map[string]any{
			"dispatchId":        ticket.DispatchID,
			"taskId":            task.TaskID,
			"threadKey":         task.ThreadKey,
			"planEpoch":         task.PlanEpoch,
			"status":            status,
			"summary":           summary,
			"changedPaths":      changedPaths,
			"producedArtifacts": uniqueNonEmpty(generated...),
			"acceptanceEvidence": uniqueNonEmpty(
				logPath,
				verifyPath,
			),
			"nextSuggestedKind": "replan",
		}
		if err := writeJSONFile(required["worker-result.json"], workerPayload); err != nil {
			return CloseoutResult{}, err
		}
		generated = append(generated, required["worker-result.json"])
	}
	if containsString(missing, "handoff.md") {
		handoff := strings.Join([]string{
			"# Runtime Closeout Hook",
			"",
			"- status: " + status,
			"- summary: " + summary,
			"- dispatchId: " + ticket.DispatchID,
			"- changedPaths: " + strings.Join(changedPaths, ", "),
			"- logPath: " + logPath,
			"",
			"Runtime synthesized this handoff because the worker run exited without the required closeout artifacts.",
		}, "\n")
		if err := os.WriteFile(required["handoff.md"], []byte(handoff+"\n"), 0o644); err != nil {
			return CloseoutResult{}, err
		}
		generated = append(generated, required["handoff.md"])
	}

	patterns := []string{patternMissingCloseoutArtifacts}
	if containsString(missing, "verify.json") {
		patterns = append(patterns, patternMissingVerifyArtifact)
	}
	if containsString(missing, "worker-result.json") {
		patterns = append(patterns, patternMissingWorkerResult)
	}
	if containsString(missing, "handoff.md") {
		patterns = append(patterns, patternMissingHandoffArtifact)
	}
	for _, pattern := range patterns {
		_ = recordLearning(root, pattern)
	}
	return CloseoutResult{
		Generated:              true,
		MissingArtifacts:       missing,
		GeneratedArtifacts:     generated,
		Status:                 status,
		Summary:                summary,
		VerificationResultPath: verifyPath,
	}, nil
}

func learningHints(state LearningState) []string {
	hints := make([]string, 0)
	if state.Patterns[patternMissingCloseoutArtifacts].Count > 0 {
		hints = append(hints, "Recent runs missed closeout artifacts; write verify.json, worker-result.json, and handoff.md immediately after validation.")
	}
	if state.Patterns[patternMissingVerifyArtifact].Promoted {
		hints = append(hints, "verify.json is now a promoted blocking artifact because multiple runs exited without verification evidence.")
	}
	return hints
}

func learnedPatterns(state LearningState) []string {
	keys := make([]string, 0, len(state.Patterns))
	for key, pattern := range state.Patterns {
		if pattern.Count == 0 {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func recordLearning(root, key string) error {
	learning, err := loadLearningState(root)
	if err != nil {
		return err
	}
	if learning.Patterns == nil {
		learning.Patterns = map[string]LearningPattern{}
	}
	pattern := learning.Patterns[key]
	pattern.Count++
	pattern.LastSeenAt = state.NowUTC()
	if pattern.Count >= learningPromotionThreshold {
		pattern.Promoted = true
	}
	learning.Patterns[key] = pattern
	learning.SchemaVersion = "kh.verification-learning.v1"
	learning.Generator = "kh-verification-hooks"
	learning.UpdatedAt = state.NowUTC()
	return saveLearningState(root, learning)
}

func loadLearningState(root string) (LearningState, error) {
	path, err := learningStatePath(root)
	if err != nil {
		return LearningState{}, err
	}
	var learning LearningState
	if ok, err := state.LoadJSONIfExists(path, &learning); err != nil {
		return LearningState{}, err
	} else if !ok {
		return LearningState{
			SchemaVersion: "kh.verification-learning.v1",
			Generator:     "kh-verification-hooks",
			UpdatedAt:     state.NowUTC(),
			Patterns:      map[string]LearningPattern{},
		}, nil
	}
	if learning.Patterns == nil {
		learning.Patterns = map[string]LearningPattern{}
	}
	return learning, nil
}

func saveLearningState(root string, learning LearningState) error {
	path, err := learningStatePath(root)
	if err != nil {
		return err
	}
	return writeJSONFile(path, learning)
}

func learningStatePath(root string) (string, error) {
	paths, err := adapter.Resolve(root)
	if err != nil {
		return "", err
	}
	return filepath.Join(paths.StateDir, "verification-learning.json"), nil
}

func changedPaths(root string) []string {
	command := exec.Command("/bin/sh", "-lc", "git diff --name-only")
	command.Dir = root
	output, err := command.Output()
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, filepath.ToSlash(line))
	}
	sort.Strings(out)
	return out
}

func changedPathsFromWorkerResult(path string) []string {
	payload, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var decoded struct {
		ChangedPaths []string `json:"changedPaths"`
	}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return nil
	}
	out := make([]string, 0, len(decoded.ChangedPaths))
	for _, item := range decoded.ChangedPaths {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		out = append(out, filepath.ToSlash(item))
	}
	sort.Strings(out)
	return out
}

func passFail(ok bool) string {
	if ok {
		return "pass"
	}
	return "fail"
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func uniqueNonEmpty(values ...string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func writeJSONFile(path string, value any) error {
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(payload, '\n'), 0o644)
}
