package verify

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"klein-harness/internal/adapter"
	"klein-harness/internal/checkpoint"
	"klein-harness/internal/dispatch"
	"klein-harness/internal/state"
)

type GateCheck struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}

type CompletionGate struct {
	state.Metadata
	Status                 string               `json:"status"`
	Satisfied              bool                 `json:"satisfied"`
	RetireEligible         bool                 `json:"retireEligible"`
	Retired                bool                 `json:"retired"`
	TaskID                 string               `json:"taskId,omitempty"`
	DispatchID             string               `json:"dispatchId,omitempty"`
	Attempt                int                  `json:"attempt,omitempty"`
	VerificationStatus     string               `json:"verificationStatus,omitempty"`
	Summary                string               `json:"summary,omitempty"`
	VerificationResultPath string               `json:"verificationResultPath,omitempty"`
	ReviewRequired         bool                 `json:"reviewRequired,omitempty"`
	ReviewEvidencePath     string               `json:"reviewEvidencePath,omitempty"`
	EvidenceRefs           []string             `json:"evidenceRefs,omitempty"`
	Checks                 map[string]GateCheck `json:"checks"`
	RemainingChecks        []GateCheck          `json:"remainingChecks,omitempty"`
}

type GuardState struct {
	state.Metadata
	Status                  string   `json:"status"`
	SafeToArchive           bool     `json:"safeToArchive"`
	CompletionGateStatus    string   `json:"completionGateStatus"`
	CompletionGateSatisfied bool     `json:"completionGateSatisfied"`
	RetireEligible          bool     `json:"retireEligible"`
	TaskID                  string   `json:"taskId,omitempty"`
	DispatchID              string   `json:"dispatchId,omitempty"`
	Blockers                []string `json:"blockers,omitempty"`
	Warnings                []string `json:"warnings,omitempty"`
}

type evidenceInfo struct {
	Path                    string
	Exists                  bool
	NonEmpty                bool
	Meaningful              bool
	EmbeddedReviewEvidence  bool
	EmbeddedReviewReference string
}

func updateCompletionState(
	paths adapter.Paths,
	request Request,
	task adapter.Task,
	taskFound bool,
	ticket dispatch.Ticket,
	ticketFound bool,
) (CompletionGate, error) {
	gate, err := buildCompletionGate(paths, request, task, taskFound, ticket, ticketFound)
	if err != nil {
		return CompletionGate{}, err
	}
	var existingGate CompletionGate
	if _, err := state.LoadJSONIfExists(paths.CompletionGatePath, &existingGate); err != nil {
		return CompletionGate{}, err
	}
	if existingGate.Retired {
		gate.Retired = true
		gate.Status = "retired"
		gate.RetireEligible = false
	}
	if _, err := state.WriteSnapshot(paths.CompletionGatePath, &gate, "kh-orchestrator", existingGate.Revision); err != nil {
		return CompletionGate{}, err
	}

	guard := buildGuardState(gate)
	var existingGuard GuardState
	if _, err := state.LoadJSONIfExists(paths.GuardStatePath, &existingGuard); err != nil {
		return CompletionGate{}, err
	}
	if _, err := state.WriteSnapshot(paths.GuardStatePath, &guard, "kh-orchestrator", existingGuard.Revision); err != nil {
		return CompletionGate{}, err
	}
	return gate, nil
}

func buildCompletionGate(
	paths adapter.Paths,
	request Request,
	task adapter.Task,
	taskFound bool,
	ticket dispatch.Ticket,
	ticketFound bool,
) (CompletionGate, error) {
	statusEligible := passedVerificationStatus(request.Status)
	summaryPresent := GateCheck{
		Name:   "summaryPresent",
		OK:     strings.TrimSpace(request.Summary) != "",
		Detail: fmt.Sprintf("summaryEmpty=%t", strings.TrimSpace(request.Summary) == ""),
	}
	verifyEvidence, err := inspectEvidence(paths.Root, request.VerificationResultPath)
	if err != nil {
		return CompletionGate{}, err
	}
	verificationEvidence := GateCheck{
		Name: "verificationEvidence",
		OK:   verifyEvidence.Exists && verifyEvidence.NonEmpty && verifyEvidence.Meaningful,
		Detail: fmt.Sprintf(
			"path=%s exists=%t nonEmpty=%t meaningful=%t",
			coalescePath(verifyEvidence.Path, request.VerificationResultPath),
			verifyEvidence.Exists,
			verifyEvidence.NonEmpty,
			verifyEvidence.Meaningful,
		),
	}
	requiredArtifacts, evidenceRefs, err := requiredArtifactsCheck(paths, request, verifyEvidence)
	if err != nil {
		return CompletionGate{}, err
	}
	reviewRequired := taskReviewRequired(task, taskFound, request.ReasonCodes, ticket, ticketFound)
	reviewEvidence, err := inspectEvidence(paths.Root, reviewEvidencePath(task, taskFound))
	if err != nil {
		return CompletionGate{}, err
	}
	reviewCheck := GateCheck{
		Name:   "reviewEvidence",
		OK:     true,
		Detail: "review not required",
	}
	if reviewRequired {
		ok := (reviewEvidence.Exists && reviewEvidence.NonEmpty && reviewEvidence.Meaningful) || verifyEvidence.EmbeddedReviewEvidence
		detail := fmt.Sprintf(
			"path=%s exists=%t nonEmpty=%t meaningful=%t embedded=%t",
			coalescePath(reviewEvidence.Path, reviewEvidencePath(task, taskFound)),
			reviewEvidence.Exists,
			reviewEvidence.NonEmpty,
			reviewEvidence.Meaningful,
			verifyEvidence.EmbeddedReviewEvidence,
		)
		if verifyEvidence.EmbeddedReviewReference != "" {
			detail = detail + " source=" + verifyEvidence.EmbeddedReviewReference
		}
		reviewCheck = GateCheck{
			Name:   "reviewEvidence",
			OK:     ok,
			Detail: detail,
		}
	}
	completionCandidate := GateCheck{
		Name:   "completionCandidate",
		OK:     statusEligible,
		Detail: "status=" + request.Status,
	}
	checks := map[string]GateCheck{
		"completionCandidate":  completionCandidate,
		"summaryPresent":       summaryPresent,
		"verificationEvidence": verificationEvidence,
		"requiredArtifacts":    requiredArtifacts,
	}
	if reviewRequired {
		checks["reviewEvidence"] = reviewCheck
	}
	remaining := remainingChecks(checks)
	satisfied := allChecksOK(checks)
	status := "open"
	if satisfied {
		status = "satisfied"
	}
	evidenceRefs = append(evidenceRefs, filterNonEmpty(
		verifyEvidence.Path,
		reviewEvidence.Path,
	)...)
	return CompletionGate{
		Status:                 status,
		Satisfied:              satisfied,
		RetireEligible:         satisfied,
		Retired:                false,
		TaskID:                 request.TaskID,
		DispatchID:             request.DispatchID,
		Attempt:                request.Attempt,
		VerificationStatus:     request.Status,
		Summary:                request.Summary,
		VerificationResultPath: request.VerificationResultPath,
		ReviewRequired:         reviewRequired,
		ReviewEvidencePath:     reviewEvidencePath(task, taskFound),
		EvidenceRefs:           uniqueStrings(evidenceRefs),
		Checks:                 checks,
		RemainingChecks:        remaining,
	}, nil
}

func buildGuardState(gate CompletionGate) GuardState {
	blockers := make([]string, 0, len(gate.RemainingChecks))
	for _, check := range gate.RemainingChecks {
		blockers = append(blockers, fmt.Sprintf("%s: %s", check.Name, check.Detail))
	}
	status := "blocked"
	if gate.Retired {
		status = "archived"
	} else if gate.Satisfied {
		status = "retire_ready"
	}
	return GuardState{
		Status:                  status,
		SafeToArchive:           gate.Satisfied && !gate.Retired,
		CompletionGateStatus:    gate.Status,
		CompletionGateSatisfied: gate.Satisfied,
		RetireEligible:          gate.RetireEligible,
		TaskID:                  gate.TaskID,
		DispatchID:              gate.DispatchID,
		Blockers:                blockers,
	}
}

func completionGateError(request Request, gate CompletionGate) error {
	errs := []error{ErrCompletionGateOpen}
	basicEvidenceMissing := !gate.Checks["summaryPresent"].OK || !gate.Checks["verificationEvidence"].OK || !gate.Checks["requiredArtifacts"].OK
	if (request.Status == "already_satisfied" || request.Status == "noop_verified") && basicEvidenceMissing {
		errs = append(errs, ErrNoopWithoutEvidence)
	}
	if basicEvidenceMissing {
		errs = append(errs, ErrVerifiedWithoutEvidence)
	}
	if check, ok := gate.Checks["reviewEvidence"]; ok && !check.OK {
		errs = append(errs, ErrReviewEvidenceRequired)
	}
	return errorsJoin(errs...)
}

func requiredArtifactsCheck(paths adapter.Paths, request Request, verifyEvidence evidenceInfo) (GateCheck, []string, error) {
	if !passedVerificationStatus(request.Status) {
		return GateCheck{
			Name:   "requiredArtifacts",
			OK:     false,
			Detail: "completion not eligible until verification status is passing",
		}, nil, nil
	}
	evidenceRefs := make([]string, 0)
	if request.DispatchID == "" {
		return GateCheck{
			Name:   "requiredArtifacts",
			OK:     verifyEvidence.Exists,
			Detail: "dispatch not bound; using verification evidence path only",
		}, evidenceRefs, nil
	}

	var summary checkpoint.Summary
	if ok, err := state.LoadJSONIfExists(paths.CheckpointSummaryPath, &summary); err != nil {
		return GateCheck{}, nil, err
	} else if ok {
		taskState, ok := summary.Tasks[request.TaskID]
		if ok && taskState.LatestOutcome.DispatchID == request.DispatchID {
			artifacts := uniqueStrings(taskState.LatestOutcome.Artifacts)
			if len(artifacts) > 0 {
				hasVerify := pathInList(verifyEvidence.Path, artifacts) && verifyEvidence.Exists
				hasWorkerResult, err := firstArtifactWithContent(rootOrPath(paths.Root, artifacts, "worker-result.json"))
				if err != nil {
					return GateCheck{}, nil, err
				}
				hasHandoff, err := firstArtifactWithContent(rootOrPath(paths.Root, artifacts, "handoff.md"))
				if err != nil {
					return GateCheck{}, nil, err
				}
				evidenceRefs = append(evidenceRefs, artifacts...)
				return GateCheck{
					Name:   "requiredArtifacts",
					OK:     hasVerify && (hasWorkerResult || hasHandoff),
					Detail: fmt.Sprintf("checkpointArtifacts=%d hasVerify=%t hasWorkerResult=%t hasHandoff=%t", len(artifacts), hasVerify, hasWorkerResult, hasHandoff),
				}, evidenceRefs, nil
			}
		}
	}

	artifactDir := filepath.Join(paths.ArtifactsDir, request.TaskID, request.DispatchID)
	info, err := os.Stat(artifactDir)
	if err != nil {
		if os.IsNotExist(err) {
			return GateCheck{
				Name:   "requiredArtifacts",
				OK:     verifyEvidence.Exists,
				Detail: "artifact dir missing; using verification evidence path only",
			}, evidenceRefs, nil
		}
		return GateCheck{}, nil, err
	}
	if !info.IsDir() {
		return GateCheck{
			Name:   "requiredArtifacts",
			OK:     false,
			Detail: "artifactDir is not a directory",
		}, evidenceRefs, nil
	}
	workerResult := filepath.Join(artifactDir, "worker-result.json")
	handoff := filepath.Join(artifactDir, "handoff.md")
	hasWorkerResult, err := fileHasContent(workerResult)
	if err != nil {
		return GateCheck{}, nil, err
	}
	hasHandoff, err := fileHasContent(handoff)
	if err != nil {
		return GateCheck{}, nil, err
	}
	evidenceRefs = append(evidenceRefs, filterNonEmpty(workerResult, handoff)...)
	return GateCheck{
		Name:   "requiredArtifacts",
		OK:     verifyEvidence.Exists && (hasWorkerResult || hasHandoff),
		Detail: fmt.Sprintf("artifactDir=%s hasWorkerResult=%t hasHandoff=%t", artifactDir, hasWorkerResult, hasHandoff),
	}, evidenceRefs, nil
}

func rootOrPath(root string, artifacts []string, base string) []string {
	paths := make([]string, 0, len(artifacts))
	for _, artifact := range artifacts {
		if filepath.Base(artifact) != base {
			continue
		}
		paths = append(paths, resolveEvidencePath(root, artifact))
	}
	return paths
}

func firstArtifactWithContent(paths []string) (bool, error) {
	for _, path := range paths {
		ok, err := fileHasContent(path)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}
	return false, nil
}

func inspectEvidence(root, rawPath string) (evidenceInfo, error) {
	path := resolveEvidencePath(root, rawPath)
	if path == "" {
		return evidenceInfo{}, nil
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return evidenceInfo{Path: path}, nil
		}
		return evidenceInfo{}, err
	}
	info := evidenceInfo{
		Path:     path,
		Exists:   true,
		NonEmpty: len(strings.TrimSpace(string(payload))) > 0,
	}
	if !info.NonEmpty {
		return info, nil
	}
	var decoded any
	if err := json.Unmarshal(payload, &decoded); err == nil {
		info.Meaningful = jsonHasMeaningfulEvidence(decoded)
		info.EmbeddedReviewEvidence = jsonHasReviewEvidence(decoded)
		if info.EmbeddedReviewEvidence {
			info.EmbeddedReviewReference = "verificationResultPath"
		}
		return info, nil
	}
	info.Meaningful = true
	return info, nil
}

func jsonHasMeaningfulEvidence(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		for _, key := range []string{"results", "commands", "evidence", "evidenceRefs", "passedRuleIds", "acceptanceEvidence", "reviewEvidence"} {
			if nonEmptyJSONValue(typed[key]) {
				return true
			}
		}
		if nonEmptyString(typed["summary"]) && (nonEmptyString(typed["status"]) || nonEmptyString(typed["overallStatus"])) {
			return true
		}
		for _, item := range typed {
			if nonEmptyJSONValue(item) {
				return true
			}
		}
	case []any:
		return len(typed) > 0
	case string:
		return strings.TrimSpace(typed) != ""
	}
	return false
}

func jsonHasReviewEvidence(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		for _, key := range []string{"reviewEvidence", "reviewEvidencePath", "reviewSummary", "reviewFindings"} {
			if nonEmptyJSONValue(typed[key]) {
				return true
			}
		}
		if approved, ok := typed["reviewApproved"].(bool); ok && approved {
			return true
		}
		if status := strings.ToLower(strings.TrimSpace(stringValue(typed["reviewStatus"]))); status == "approved" || status == "verified" || status == "passed" {
			return true
		}
	}
	return false
}

func taskReviewRequired(task adapter.Task, taskFound bool, reasonCodes []string, ticket dispatch.Ticket, ticketFound bool) bool {
	if taskFound && task.ReviewRequired {
		return true
	}
	if hasReasonCode(reasonCodes, "review_required", "policy_review_required") {
		return true
	}
	if ticketFound && hasReasonCode(ticket.ReasonCodes, "review_required", "policy_review_required") {
		return true
	}
	return false
}

func reviewEvidencePath(task adapter.Task, taskFound bool) string {
	if !taskFound {
		return ""
	}
	return strings.TrimSpace(task.ReviewEvidencePath)
}

func passedVerificationStatus(status string) bool {
	switch status {
	case "passed", "succeeded", "verified", "already_satisfied", "noop_verified":
		return true
	default:
		return false
	}
}

func resolveEvidencePath(root, rawPath string) string {
	if strings.TrimSpace(rawPath) == "" {
		return ""
	}
	if filepath.IsAbs(rawPath) {
		return filepath.Clean(rawPath)
	}
	return filepath.Join(root, rawPath)
}

func remainingChecks(checks map[string]GateCheck) []GateCheck {
	out := make([]GateCheck, 0, len(checks))
	for _, key := range []string{"completionCandidate", "summaryPresent", "verificationEvidence", "requiredArtifacts", "reviewEvidence"} {
		check, ok := checks[key]
		if ok && !check.OK {
			out = append(out, check)
		}
	}
	return out
}

func allChecksOK(checks map[string]GateCheck) bool {
	for _, check := range checks {
		if !check.OK {
			return false
		}
	}
	return true
}

func pathInList(path string, values []string) bool {
	if path == "" {
		return false
	}
	clean := filepath.Clean(path)
	for _, value := range values {
		if filepath.Clean(value) == clean {
			return true
		}
	}
	return false
}

func basenameInList(name string, values []string) bool {
	for _, value := range values {
		if filepath.Base(value) == name {
			return true
		}
	}
	return false
}

func fileHasContent(path string) (bool, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return len(strings.TrimSpace(string(payload))) > 0, nil
}

func nonEmptyJSONValue(value any) bool {
	switch typed := value.(type) {
	case nil:
		return false
	case string:
		return strings.TrimSpace(typed) != ""
	case []any:
		return len(typed) > 0
	case map[string]any:
		return len(typed) > 0
	case bool:
		return typed
	default:
		return true
	}
}

func nonEmptyString(value any) bool {
	return strings.TrimSpace(stringValue(value)) != ""
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return fmt.Sprint(typed)
	}
}

func uniqueStrings(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
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

func filterNonEmpty(values ...string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			out = append(out, value)
		}
	}
	return out
}

func coalescePath(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func hasReasonCode(values []string, wants ...string) bool {
	for _, value := range values {
		for _, want := range wants {
			if value == want {
				return true
			}
		}
	}
	return false
}

func errorsJoin(errs ...error) error {
	filtered := make([]error, 0, len(errs))
	for _, err := range errs {
		if err != nil {
			filtered = append(filtered, err)
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	if len(filtered) == 1 {
		return filtered[0]
	}
	return errors.Join(filtered...)
}
