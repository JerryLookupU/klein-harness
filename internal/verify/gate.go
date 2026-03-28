package verify

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"klein-harness/internal/adapter"
	"klein-harness/internal/dispatch"
	"klein-harness/internal/orchestration"
	"klein-harness/internal/state"
	"klein-harness/internal/worktree"
)

type GateCheck struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}

type CompletionGate struct {
	state.Metadata
	Status                  string               `json:"status"`
	Satisfied               bool                 `json:"satisfied"`
	RetireEligible          bool                 `json:"retireEligible"`
	Retired                 bool                 `json:"retired"`
	TaskID                  string               `json:"taskId,omitempty"`
	DispatchID              string               `json:"dispatchId,omitempty"`
	Attempt                 int                  `json:"attempt,omitempty"`
	VerificationStatus      string               `json:"verificationStatus,omitempty"`
	AssessmentOverallStatus string               `json:"assessmentOverallStatus,omitempty"`
	Summary                 string               `json:"summary,omitempty"`
	VerificationResultPath  string               `json:"verificationResultPath,omitempty"`
	ReviewRequired          bool                 `json:"reviewRequired,omitempty"`
	ReviewEvidencePath      string               `json:"reviewEvidencePath,omitempty"`
	ConstraintPath          string               `json:"constraintPath,omitempty"`
	AcceptedPacketPath      string               `json:"acceptedPacketPath,omitempty"`
	TaskContractPath        string               `json:"taskContractPath,omitempty"`
	AssessmentPath          string               `json:"assessmentPath,omitempty"`
	RecommendedNextAction   string               `json:"recommendedNextAction,omitempty"`
	MissingRequiredEvidence []string             `json:"missingRequiredEvidence,omitempty"`
	BlockingFindings        []string             `json:"blockingFindings,omitempty"`
	EvidenceRefs            []string             `json:"evidenceRefs,omitempty"`
	Checks                  map[string]GateCheck `json:"checks"`
	HardConstraintChecks    []GateCheck          `json:"hardConstraintChecks,omitempty"`
	RemainingChecks         []GateCheck          `json:"remainingChecks,omitempty"`
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
	if _, err := state.LoadJSONIfExists(paths.CompletionGateTaskPath(gate.TaskID), &existingGate); err != nil {
		return CompletionGate{}, err
	}
	if existingGate.Retired {
		gate.Retired = true
		gate.Status = "retired"
		gate.RetireEligible = false
	}
	if _, err := state.WriteSnapshot(paths.CompletionGateTaskPath(gate.TaskID), &gate, "kh-orchestrator", existingGate.Revision); err != nil {
		return CompletionGate{}, err
	}
	var aliasGate CompletionGate
	if _, err := state.LoadJSONIfExists(paths.CompletionGatePath, &aliasGate); err != nil {
		return CompletionGate{}, err
	}
	if _, err := state.WriteSnapshot(paths.CompletionGatePath, &gate, "kh-orchestrator", aliasGate.Revision); err != nil {
		return CompletionGate{}, err
	}

	guard := buildGuardState(gate)
	var existingGuard GuardState
	if _, err := state.LoadJSONIfExists(paths.GuardStateTaskPath(gate.TaskID), &existingGuard); err != nil {
		return CompletionGate{}, err
	}
	if _, err := state.WriteSnapshot(paths.GuardStateTaskPath(gate.TaskID), &guard, "kh-orchestrator", existingGuard.Revision); err != nil {
		return CompletionGate{}, err
	}
	var aliasGuard GuardState
	if _, err := state.LoadJSONIfExists(paths.GuardStatePath, &aliasGuard); err != nil {
		return CompletionGate{}, err
	}
	if _, err := state.WriteSnapshot(paths.GuardStatePath, &guard, "kh-orchestrator", aliasGuard.Revision); err != nil {
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
	packetCheck, acceptedPacketPath, packet, err := acceptedPacketCheck(paths.Root, request.TaskID, request.PlanEpoch)
	if err != nil {
		return CompletionGate{}, err
	}
	contractCheck, taskContractPath, contract, err := taskContractCheck(paths.Root, request.TaskID, request.DispatchID, request.PlanEpoch)
	if err != nil {
		return CompletionGate{}, err
	}
	contractDefinition := taskContractDefinitionCheck(contractCheck.OK, contract)
	executionTasksCheck, executionProgressRefs, err := executionTasksProgressCheck(paths.Root, request.TaskID, acceptedPacketPath)
	if err != nil {
		return CompletionGate{}, err
	}
	reviewRequired := taskReviewRequired(task, taskFound, request.ReasonCodes, ticket, ticketFound)
	reviewEvidence, err := inspectEvidence(paths.Root, reviewEvidencePath(task, taskFound))
	if err != nil {
		return CompletionGate{}, err
	}
	scorecardCheck, assessmentPath, assessment, assessmentEvidenceRefs, err := verificationScorecardCheck(paths.Root, request.TaskID, request.DispatchID, request.VerificationResultPath)
	if err != nil {
		return CompletionGate{}, err
	}
	evidenceLedgerCheck := assessmentEvidenceLedgerCheck(assessmentPath, assessment)
	requiredArtifacts, missingRequiredEvidence, evidenceRefs, err := requiredArtifactsCheck(paths, request, verifyEvidence, contractCheck.OK, contract, assessment)
	if err != nil {
		return CompletionGate{}, err
	}
	blockingFindingsCheck, blockingFindings := assessmentBlockingFindingsCheck(assessment)
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
		"completionCandidate":    completionCandidate,
		"summaryPresent":         summaryPresent,
		"verificationEvidence":   verificationEvidence,
		"requiredArtifacts":      requiredArtifacts,
		"acceptedPacket":         packetCheck,
		"orchestrationExpansion": orchestrationExpansionCheck(packetCheck.OK, packet),
		"taskContract":           contractCheck,
		"taskContractDefinition": contractDefinition,
		"verificationScorecard":  scorecardCheck,
		"evidenceLedger":         evidenceLedgerCheck,
		"blockingFindings":       blockingFindingsCheck,
		"executionTasks":         executionTasksCheck,
	}
	if reviewRequired {
		checks["reviewEvidence"] = reviewCheck
	}
	constraintPath, hardChecks, err := evaluateHardConstraintChecks(paths, request, task, taskFound, reviewRequired, verifyEvidence, reviewEvidence)
	if err != nil {
		return CompletionGate{}, err
	}
	if constraintPath != "" {
		aggregate := GateCheck{
			Name:   "hardConstraints",
			OK:     allGateChecksOK(hardChecks),
			Detail: fmt.Sprintf("path=%s evaluated=%d", constraintPath, len(hardChecks)),
		}
		checks["hardConstraints"] = aggregate
	}
	remaining := remainingChecks(checks)
	satisfied := allChecksOK(checks)
	status, recommendedNextAction := deriveCompletionStatus(request.Status, assessment.RecommendedNextAction, checks, satisfied)
	evidenceRefs = append(evidenceRefs, filterNonEmpty(
		acceptedPacketPath,
		taskContractPath,
		assessmentPath,
		verifyEvidence.Path,
		reviewEvidence.Path,
	)...)
	evidenceRefs = append(evidenceRefs, assessmentEvidenceRefs...)
	evidenceRefs = append(evidenceRefs, executionProgressRefs...)
	return CompletionGate{
		Status:                  status,
		Satisfied:               satisfied,
		RetireEligible:          satisfied,
		Retired:                 false,
		TaskID:                  request.TaskID,
		DispatchID:              request.DispatchID,
		Attempt:                 request.Attempt,
		VerificationStatus:      request.Status,
		AssessmentOverallStatus: assessment.OverallStatus,
		Summary:                 request.Summary,
		VerificationResultPath:  request.VerificationResultPath,
		ReviewRequired:          reviewRequired,
		ReviewEvidencePath:      reviewEvidencePath(task, taskFound),
		ConstraintPath:          constraintPath,
		AcceptedPacketPath:      acceptedPacketPath,
		TaskContractPath:        taskContractPath,
		AssessmentPath:          assessmentPath,
		RecommendedNextAction:   recommendedNextAction,
		MissingRequiredEvidence: uniqueStrings(missingRequiredEvidence),
		BlockingFindings:        uniqueStrings(blockingFindings),
		EvidenceRefs:            uniqueStrings(evidenceRefs),
		Checks:                  checks,
		HardConstraintChecks:    hardChecks,
		RemainingChecks:         remaining,
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
	if check, ok := gate.Checks["acceptedPacket"]; ok && !check.OK {
		errs = append(errs, ErrAcceptedPacketRequired)
	}
	if check, ok := gate.Checks["taskContract"]; ok && !check.OK {
		errs = append(errs, ErrTaskContractRequired)
	}
	if check, ok := gate.Checks["verificationScorecard"]; ok && !check.OK {
		errs = append(errs, ErrVerificationScorecardRequired)
	}
	if check, ok := gate.Checks["evidenceLedger"]; ok && !check.OK {
		errs = append(errs, ErrEvidenceLedgerRequired)
	}
	if check, ok := gate.Checks["blockingFindings"]; ok && !check.OK {
		errs = append(errs, ErrBlockingVerificationFindings)
	}
	if check, ok := gate.Checks["taskContractDefinition"]; ok && !check.OK {
		errs = append(errs, ErrTaskContractIncomplete)
	}
	if check, ok := gate.Checks["executionTasks"]; ok && !check.OK {
		errs = append(errs, ErrExecutionTasksRemaining)
	}
	if check, ok := gate.Checks["reviewEvidence"]; ok && !check.OK {
		errs = append(errs, ErrReviewEvidenceRequired)
	}
	return errorsJoin(errs...)
}

func requiredArtifactsCheck(paths adapter.Paths, request Request, verifyEvidence evidenceInfo, contractFound bool, contract orchestration.TaskContract, assessment Assessment) (GateCheck, []string, []string, error) {
	if !passedVerificationStatus(request.Status) {
		return GateCheck{
			Name:   "requiredArtifacts",
			OK:     false,
			Detail: "completion not eligible until verification status is passing",
		}, nil, nil, nil
	}
	evidenceRefs := make([]string, 0)
	missing := make([]string, 0)
	if request.DispatchID == "" {
		return GateCheck{
			Name:   "requiredArtifacts",
			OK:     verifyEvidence.Exists,
			Detail: "dispatch not bound; using verification evidence path only",
		}, evidenceRefs, nil, nil
	}
	requiredEvidence := []string{"dispatch ticket", "worker-spec", "verify.json", "worker-result.json", "handoff.md"}
	if contractFound && len(contract.RequiredEvidence) > 0 {
		requiredEvidence = contract.RequiredEvidence
	}
	details := make([]string, 0, len(requiredEvidence))
	for _, requirement := range requiredEvidence {
		ok, refs, detail, err := requirementSatisfied(paths, request, verifyEvidence, contract, assessment, requirement)
		if err != nil {
			return GateCheck{}, nil, nil, err
		}
		evidenceRefs = append(evidenceRefs, refs...)
		details = append(details, fmt.Sprintf("%s=%s", requirement, detail))
		if !ok {
			missing = append(missing, requirement)
		}
	}
	return GateCheck{
		Name:   "requiredArtifacts",
		OK:     len(missing) == 0,
		Detail: fmt.Sprintf("required=%d missing=%s checks=%s", len(requiredEvidence), strings.Join(missing, ","), strings.Join(details, "; ")),
	}, missing, evidenceRefs, nil
}

func acceptedPacketCheck(root, taskID string, planEpoch int) (GateCheck, string, orchestration.AcceptedPacket, error) {
	path := orchestration.AcceptedPacketPath(root, taskID)
	packet, err := orchestration.LoadAcceptedPacket(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return GateCheck{
				Name:   "acceptedPacket",
				OK:     false,
				Detail: "accepted packet is missing",
			}, path, orchestration.AcceptedPacket{}, nil
		}
		return GateCheck{}, path, orchestration.AcceptedPacket{}, err
	}
	ok := packet.TaskID == taskID && strings.TrimSpace(packet.PacketID) != "" && packet.SchemaVersion == "kh.accepted-packet.v1"
	detailParts := []string{fmt.Sprintf("path=%s", path), fmt.Sprintf("taskId=%s", packet.TaskID), fmt.Sprintf("packetId=%s", packet.PacketID), fmt.Sprintf("schemaVersion=%s", packet.SchemaVersion)}
	if planEpoch > 0 {
		matchEpoch := packet.PlanEpoch == planEpoch
		ok = ok && matchEpoch
		detailParts = append(detailParts, fmt.Sprintf("planEpoch=%d expected=%d", packet.PlanEpoch, planEpoch))
	}
	return GateCheck{
		Name:   "acceptedPacket",
		OK:     ok,
		Detail: strings.Join(detailParts, " "),
	}, path, packet, nil
}

func taskContractCheck(root, taskID, dispatchID string, planEpoch int) (GateCheck, string, orchestration.TaskContract, error) {
	if strings.TrimSpace(dispatchID) == "" {
		return GateCheck{
			Name:   "taskContract",
			OK:     false,
			Detail: "dispatch id is missing; task contract cannot be resolved",
		}, "", orchestration.TaskContract{}, nil
	}
	path := orchestration.TaskContractPath(filepath.Join(root, ".harness", "artifacts", taskID, dispatchID))
	contract, err := orchestration.LoadTaskContract(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return GateCheck{
				Name:   "taskContract",
				OK:     false,
				Detail: "task contract is missing",
			}, path, orchestration.TaskContract{}, nil
		}
		return GateCheck{}, path, orchestration.TaskContract{}, err
	}
	ok := contract.TaskID == taskID &&
		contract.DispatchID == dispatchID &&
		strings.EqualFold(contract.ContractStatus, "accepted") &&
		strings.TrimSpace(contract.ContractID) != "" &&
		contract.SchemaVersion == "kh.task-contract.v1"
	detailParts := []string{
		fmt.Sprintf("path=%s", path),
		fmt.Sprintf("taskId=%s", contract.TaskID),
		fmt.Sprintf("dispatchId=%s", contract.DispatchID),
		fmt.Sprintf("status=%s", contract.ContractStatus),
		fmt.Sprintf("contractId=%s", contract.ContractID),
		fmt.Sprintf("schemaVersion=%s", contract.SchemaVersion),
	}
	if planEpoch > 0 {
		matchEpoch := contract.PlanEpoch == planEpoch
		ok = ok && matchEpoch
		detailParts = append(detailParts, fmt.Sprintf("planEpoch=%d expected=%d", contract.PlanEpoch, planEpoch))
	}
	return GateCheck{
		Name:   "taskContract",
		OK:     ok,
		Detail: strings.Join(detailParts, " "),
	}, path, contract, nil
}

func executionTasksProgressCheck(root, taskID, acceptedPacketPath string) (GateCheck, []string, error) {
	packet, err := orchestration.LoadAcceptedPacket(acceptedPacketPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return GateCheck{
				Name:   "executionTasks",
				OK:     false,
				Detail: "accepted packet is missing; execution task progress cannot be evaluated",
			}, []string{acceptedPacketPath}, nil
		}
		return GateCheck{}, nil, err
	}
	if len(packet.ExecutionTasks) <= 1 {
		return GateCheck{
			Name:   "executionTasks",
			OK:     true,
			Detail: fmt.Sprintf("executionTasks=%d", len(packet.ExecutionTasks)),
		}, []string{acceptedPacketPath}, nil
	}
	progressPath := orchestration.PacketProgressPath(root, taskID)
	progress, err := orchestration.LoadPacketProgress(progressPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return GateCheck{
				Name:   "executionTasks",
				OK:     false,
				Detail: fmt.Sprintf("executionTasks=%d completed=0 remaining=%d", len(packet.ExecutionTasks), len(packet.ExecutionTasks)),
			}, []string{acceptedPacketPath, progressPath}, nil
		}
		return GateCheck{}, nil, err
	}
	if progress.PlanEpoch != 0 && progress.PlanEpoch != packet.PlanEpoch {
		return GateCheck{
			Name:   "executionTasks",
			OK:     false,
			Detail: fmt.Sprintf("executionTasks=%d staleProgressPlanEpoch=%d expected=%d", len(packet.ExecutionTasks), progress.PlanEpoch, packet.PlanEpoch),
		}, []string{acceptedPacketPath, progressPath}, nil
	}
	if strings.TrimSpace(progress.AcceptedPacketID) != "" && progress.AcceptedPacketID != packet.PacketID {
		return GateCheck{
			Name:   "executionTasks",
			OK:     false,
			Detail: fmt.Sprintf("executionTasks=%d staleProgressPacket=%s expected=%s", len(packet.ExecutionTasks), progress.AcceptedPacketID, packet.PacketID),
		}, []string{acceptedPacketPath, progressPath}, nil
	}
	completed := map[string]struct{}{}
	for _, id := range progress.CompletedSliceIDs {
		completed[id] = struct{}{}
	}
	remaining := make([]string, 0)
	for _, item := range packet.ExecutionTasks {
		if _, ok := completed[item.ID]; !ok {
			remaining = append(remaining, item.ID)
		}
	}
	return GateCheck{
		Name:   "executionTasks",
		OK:     len(remaining) == 0,
		Detail: fmt.Sprintf("executionTasks=%d completed=%d remaining=%s", len(packet.ExecutionTasks), len(progress.CompletedSliceIDs), strings.Join(remaining, ",")),
	}, []string{acceptedPacketPath, progressPath}, nil
}

func verificationScorecardCheck(root, taskID, dispatchID, verificationResultPath string) (GateCheck, string, Assessment, []string, error) {
	path := resolveEvidencePath(root, verificationResultPath)
	if strings.TrimSpace(path) == "" && strings.TrimSpace(dispatchID) != "" {
		path = AssessmentPath(root, taskID, dispatchID)
	}
	assessment, err := LoadAssessment(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return GateCheck{
				Name:   "verificationScorecard",
				OK:     false,
				Detail: "verification scorecard is missing",
			}, path, Assessment{}, nil, nil
		}
		return GateCheck{}, path, Assessment{}, nil, err
	}
	scorecard := assessment.Scorecard
	failed := failedScorecardDimensions(scorecard)
	ok := len(scorecard) > 0 && len(failed) == 0
	detail := fmt.Sprintf("path=%s dimensions=%d failed=%s", path, len(scorecard), strings.Join(failed, ","))
	return GateCheck{
		Name:   "verificationScorecard",
		OK:     ok,
		Detail: detail,
	}, path, assessment, []string{path}, nil
}

func evaluateHardConstraintChecks(paths adapter.Paths, request Request, task adapter.Task, taskFound, reviewRequired bool, verifyEvidence, reviewEvidence evidenceInfo) (string, []GateCheck, error) {
	if strings.TrimSpace(request.TaskID) == "" {
		return "", nil, nil
	}
	constraintPath := orchestration.ConstraintSnapshotPath(paths.Root, request.TaskID)
	snapshot, err := orchestration.LoadConstraintSnapshot(constraintPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil, nil
		}
		return "", nil, err
	}
	artifactDir := filepath.Join(paths.ArtifactsDir, request.TaskID, request.DispatchID)
	checks := make([]GateCheck, 0, len(snapshot.HardRules))
	for _, rule := range snapshot.HardRules {
		check := GateCheck{
			Name:   rule.ID,
			OK:     true,
			Detail: "validated by shared hard-constraint gate",
		}
		switch rule.VerificationMode {
		case "schema_validation":
			check.Detail = "validated upstream by planning packet generation"
		case "route_gate":
			check.Detail = "validated upstream by route-first runtime dispatch"
		case "owned_path_audit":
			if !taskFound || len(task.OwnedPaths) == 0 {
				check.Detail = "task ownedPaths unavailable; owned path audit deferred to runtime boundary gate"
				break
			}
			ok, detail := ownedPathAuditCheck(artifactDir, task.OwnedPaths)
			check.OK = ok
			check.Detail = detail
		case "owned_path_nonempty_gate":
			if !taskFound || len(task.OwnedPaths) == 0 {
				check.OK = false
				check.Detail = "task ownedPaths unavailable; evolution gate requires ownedPaths"
				break
			}
			ok, detail, err := ownedPathNonEmptyAuditCheck(artifactDir, task.OwnedPaths)
			if err != nil {
				return constraintPath, nil, err
			}
			check.OK = ok
			check.Detail = detail
		case "closeout_artifact_gate":
			ok, detail, err := closeoutArtifactCheck(artifactDir)
			if err != nil {
				return constraintPath, nil, err
			}
			check.OK = ok
			check.Detail = detail
		case "verify_evidence_gate":
			check.OK = verifyEvidence.Exists && verifyEvidence.NonEmpty && verifyEvidence.Meaningful
			check.Detail = fmt.Sprintf("path=%s exists=%t nonEmpty=%t meaningful=%t", coalescePath(verifyEvidence.Path, request.VerificationResultPath), verifyEvidence.Exists, verifyEvidence.NonEmpty, verifyEvidence.Meaningful)
		case "review_evidence_gate":
			if !reviewRequired {
				check.Detail = "review not required"
			} else {
				check.OK = (reviewEvidence.Exists && reviewEvidence.NonEmpty && reviewEvidence.Meaningful) || verifyEvidence.EmbeddedReviewEvidence
				check.Detail = fmt.Sprintf("path=%s exists=%t nonEmpty=%t meaningful=%t embedded=%t", coalescePath(reviewEvidence.Path, reviewEvidencePath(task, taskFound)), reviewEvidence.Exists, reviewEvidence.NonEmpty, reviewEvidence.Meaningful, verifyEvidence.EmbeddedReviewEvidence)
			}
		case "runtime_followup_gate":
			check.Detail = fmt.Sprintf("status=%s followUp=%s", request.Status, request.FollowUp)
			if passedVerificationStatus(request.Status) {
				check.OK = request.FollowUp == "" || request.FollowUp == "task.completed"
			} else {
				check.OK = request.FollowUp != "task.completed"
			}
		default:
			check.Detail = "shared hard constraint loaded; no runtime evaluator registered yet"
		}
		checks = append(checks, check)
	}
	return constraintPath, checks, nil
}

func allGateChecksOK(checks []GateCheck) bool {
	for _, check := range checks {
		if !check.OK {
			return false
		}
	}
	return true
}

func ownedPathAuditCheck(artifactDir string, ownedPaths []string) (bool, string) {
	payload, err := os.ReadFile(filepath.Join(artifactDir, "worker-result.json"))
	if err != nil {
		return false, "worker-result.json missing for owned path audit"
	}
	var decoded struct {
		ChangedPaths []string `json:"changedPaths"`
	}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return false, "worker-result.json unreadable for owned path audit"
	}
	for _, changedPath := range decoded.ChangedPaths {
		allowed := false
		for _, ownedPath := range ownedPaths {
			if worktree.PathOverlap(ownedPath, changedPath) || worktree.PathOverlap(changedPath, ownedPath) {
				allowed = true
				break
			}
		}
		if !allowed {
			return false, "changed path outside ownedPaths: " + changedPath
		}
	}
	return true, fmt.Sprintf("changedPaths=%d", len(decoded.ChangedPaths))
}

func ownedPathNonEmptyAuditCheck(artifactDir string, ownedPaths []string) (bool, string, error) {
	payload, err := os.ReadFile(filepath.Join(artifactDir, "worker-result.json"))
	if err != nil {
		return false, "worker-result.json missing for owned path evolution gate", err
	}
	var decoded struct {
		ChangedPaths []string `json:"changedPaths"`
	}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return false, "worker-result.json unreadable for owned path evolution gate", err
	}
	if len(decoded.ChangedPaths) == 0 {
		return false, "changedPaths is empty; path_conflict evolution gate requires non-empty evidence", nil
	}
	for _, changedPath := range decoded.ChangedPaths {
		allowed := false
		for _, ownedPath := range ownedPaths {
			if worktree.PathOverlap(ownedPath, changedPath) || worktree.PathOverlap(changedPath, ownedPath) {
				allowed = true
				break
			}
		}
		if !allowed {
			return false, "changed path outside ownedPaths: " + changedPath, nil
		}
	}
	return true, fmt.Sprintf("changedPaths=%d nonEmpty=true", len(decoded.ChangedPaths)), nil
}

func closeoutArtifactCheck(artifactDir string) (bool, string, error) {
	required := []string{"worker-result.json", "verify.json", "handoff.md"}
	missing := make([]string, 0)
	for _, name := range required {
		ok, err := fileHasContent(filepath.Join(artifactDir, name))
		if err != nil {
			return false, "", err
		}
		if !ok {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return false, "missing=" + strings.Join(missing, ", "), nil
	}
	return true, "all required closeout artifacts present", nil
}

func taskContractDefinitionCheck(contractFound bool, contract orchestration.TaskContract) GateCheck {
	if !contractFound {
		return GateCheck{
			Name:   "taskContractDefinition",
			OK:     false,
			Detail: "task contract unavailable for definition check",
		}
	}
	ok := len(contract.InScope) > 0 &&
		len(contract.DoneCriteria) > 0 &&
		len(contract.VerificationChecklist) > 0 &&
		len(contract.RequiredEvidence) > 0 &&
		strings.TrimSpace(contract.ExecutionSliceID) != "" &&
		strings.TrimSpace(contract.AcceptedPacketPath) != "" &&
		strings.TrimSpace(contract.SharedFlowContextPath) != "" &&
		strings.TrimSpace(contract.TaskGraphPath) != "" &&
		strings.TrimSpace(contract.SliceContextPath) != "" &&
		strings.TrimSpace(contract.ContextLayersPath) != "" &&
		strings.TrimSpace(contract.VerifySkeletonPath) != "" &&
		strings.TrimSpace(contract.CloseoutSkeletonPath) != "" &&
		strings.TrimSpace(contract.HandoffContractPath) != "" &&
		strings.TrimSpace(contract.TakeoverPath) != ""
	return GateCheck{
		Name: "taskContractDefinition",
		OK:   ok,
		Detail: fmt.Sprintf(
			"executionSliceId=%s inScope=%d doneCriteria=%d checklist=%d requiredEvidence=%d acceptedPacketPath=%t sharedFlow=%t taskGraph=%t sliceContext=%t contextLayers=%t verifySkeleton=%t closeoutSkeleton=%t handoffContract=%t takeover=%t",
			contract.ExecutionSliceID,
			len(contract.InScope),
			len(contract.DoneCriteria),
			len(contract.VerificationChecklist),
			len(contract.RequiredEvidence),
			strings.TrimSpace(contract.AcceptedPacketPath) != "",
			strings.TrimSpace(contract.SharedFlowContextPath) != "",
			strings.TrimSpace(contract.TaskGraphPath) != "",
			strings.TrimSpace(contract.SliceContextPath) != "",
			strings.TrimSpace(contract.ContextLayersPath) != "",
			strings.TrimSpace(contract.VerifySkeletonPath) != "",
			strings.TrimSpace(contract.CloseoutSkeletonPath) != "",
			strings.TrimSpace(contract.HandoffContractPath) != "",
			strings.TrimSpace(contract.TakeoverPath) != "",
		),
	}
}

func orchestrationExpansionCheck(packetFound bool, packet orchestration.AcceptedPacket) GateCheck {
	if !packetFound {
		return GateCheck{
			Name:   "orchestrationExpansion",
			OK:     true,
			Detail: "accepted packet unavailable; orchestration expansion check skipped",
		}
	}
	if !packet.OrchestrationExpansionPending {
		return GateCheck{
			Name:   "orchestrationExpansion",
			OK:     true,
			Detail: "orchestration task graph is fully materialized",
		}
	}
	return GateCheck{
		Name:   "orchestrationExpansion",
		OK:     false,
		Detail: fmt.Sprintf("pending=%t reason=%s source=%s", packet.OrchestrationExpansionPending, packet.OrchestrationExpansionReason, packet.OrchestrationExpansionSource),
	}
}

func assessmentEvidenceLedgerCheck(path string, assessment Assessment) GateCheck {
	return GateCheck{
		Name:   "evidenceLedger",
		OK:     len(assessment.EvidenceLedger) > 0,
		Detail: fmt.Sprintf("path=%s entries=%d", path, len(assessment.EvidenceLedger)),
	}
}

func assessmentBlockingFindingsCheck(assessment Assessment) (GateCheck, []string) {
	blocking := blockingFindingsFromAssessment(assessment)
	return GateCheck{
		Name:   "blockingFindings",
		OK:     len(blocking) == 0,
		Detail: fmt.Sprintf("blocking=%s", strings.Join(blocking, "; ")),
	}, blocking
}

func requirementSatisfied(paths adapter.Paths, request Request, verifyEvidence evidenceInfo, contract orchestration.TaskContract, assessment Assessment, requirement string) (bool, []string, string, error) {
	artifactDir := filepath.Join(paths.ArtifactsDir, request.TaskID, request.DispatchID)
	checkPath := func(path string) (bool, []string, string, error) {
		ok, err := fileHasContent(path)
		if err != nil {
			return false, nil, "", err
		}
		return ok, filterNonEmpty(path), fmt.Sprintf("path=%s exists=%t", path, ok), nil
	}
	switch normalizeRequirementKey(requirement) {
	case "dispatch_ticket":
		return checkPath(filepath.Join(paths.StateDir, fmt.Sprintf("dispatch-ticket-%s.json", request.TaskID)))
	case "worker_spec":
		return checkPath(filepath.Join(artifactDir, "worker-spec.json"))
	case "verify_json":
		ok := verifyEvidence.Exists && verifyEvidence.NonEmpty && verifyEvidence.Meaningful
		return ok, filterNonEmpty(verifyEvidence.Path), fmt.Sprintf("path=%s exists=%t meaningful=%t", verifyEvidence.Path, verifyEvidence.Exists, verifyEvidence.Meaningful), nil
	case "worker_result":
		return checkPath(filepath.Join(artifactDir, "worker-result.json"))
	case "handoff":
		return checkPath(filepath.Join(artifactDir, "handoff.md"))
	case "accepted_packet":
		path := resolveEvidencePath(paths.Root, firstNonEmptyVerify(contract.AcceptedPacketPath, orchestration.AcceptedPacketPath(paths.Root, request.TaskID)))
		return checkPath(path)
	case "task_contract":
		return checkPath(orchestration.TaskContractPath(artifactDir))
	default:
		if looksLikeFileEvidence(requirement) {
			ok, refs, detail, err := checkPath(filepath.Join(artifactDir, requirement))
			if err != nil {
				return false, nil, "", err
			}
			if ok {
				return ok, refs, detail, nil
			}
		}
		ok, refs, detail := evidenceLedgerSatisfies(requirement, assessment)
		return ok, refs, detail, nil
	}
}

func normalizeRequirementKey(requirement string) string {
	key := strings.ToLower(strings.TrimSpace(requirement))
	replacer := strings.NewReplacer("-", "_", " ", "_", ".", "_", "/", "_")
	key = replacer.Replace(key)
	switch key {
	case "dispatch_ticket", "dispatch":
		return "dispatch_ticket"
	case "worker_spec", "workerspec":
		return "worker_spec"
	case "verify_json", "verify", "verification_result":
		return "verify_json"
	case "worker_result_json", "worker_result":
		return "worker_result"
	case "handoff_md", "handoff":
		return "handoff"
	case "accepted_packet", "accepted_packet_truth", "packet":
		return "accepted_packet"
	case "task_contract", "contract", "task_contract_json":
		return "task_contract"
	default:
		return key
	}
}

func evidenceLedgerSatisfies(requirement string, assessment Assessment) (bool, []string, string) {
	needle := strings.ToLower(strings.TrimSpace(requirement))
	refs := make([]string, 0)
	for _, entry := range assessment.EvidenceLedger {
		if valueMatchesRequirement(needle, entry) {
			refs = append(refs, evidenceEntryRefs(entry)...)
		}
	}
	if len(refs) > 0 {
		refs = uniqueStrings(refs)
		return true, refs, fmt.Sprintf("matched ledger refs=%s", strings.Join(refs, ","))
	}
	return false, nil, "not found in evidence ledger"
}

func valueMatchesRequirement(needle string, value any) bool {
	switch typed := value.(type) {
	case string:
		lower := strings.ToLower(strings.TrimSpace(typed))
		return lower != "" && (strings.Contains(lower, needle) || filepath.Base(lower) == needle)
	case []any:
		for _, item := range typed {
			if valueMatchesRequirement(needle, item) {
				return true
			}
		}
	case []string:
		for _, item := range typed {
			if valueMatchesRequirement(needle, item) {
				return true
			}
		}
	case map[string]any:
		for _, item := range typed {
			if valueMatchesRequirement(needle, item) {
				return true
			}
		}
	}
	return false
}

func evidenceEntryRefs(entry map[string]any) []string {
	refs := make([]string, 0)
	if path := strings.TrimSpace(stringValue(entry["path"])); path != "" {
		refs = append(refs, path)
	}
	switch typed := entry["artifacts"].(type) {
	case []any:
		for _, item := range typed {
			if text := coalesceString(item); text != "" {
				refs = append(refs, text)
			}
		}
	case []string:
		refs = append(refs, typed...)
	}
	return uniqueStrings(refs)
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
	for _, key := range []string{"completionCandidate", "summaryPresent", "verificationEvidence", "requiredArtifacts", "acceptedPacket", "taskContract", "taskContractDefinition", "verificationScorecard", "evidenceLedger", "blockingFindings", "executionTasks", "reviewEvidence"} {
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

func deriveCompletionStatus(requestStatus, assessmentAction string, checks map[string]GateCheck, satisfied bool) (string, string) {
	if satisfied {
		return "satisfied", "archive"
	}
	if requestStatus == "blocked" {
		return "blocked", "unblock"
	}
	if failedGateCheck(checks, "reviewEvidence") || assessmentAction == "review" {
		return "needs_review", "review"
	}
	if failedGateCheck(checks, "orchestrationExpansion") {
		return "needs_replan", "replan"
	}
	if failedGateCheck(checks, "executionTasks") {
		return "needs_replan", "replan"
	}
	if failedGateCheck(checks, "verificationScorecard") ||
		failedGateCheck(checks, "requiredArtifacts") ||
		failedGateCheck(checks, "evidenceLedger") ||
		failedGateCheck(checks, "blockingFindings") ||
		failedGateCheck(checks, "acceptedPacket") ||
		failedGateCheck(checks, "taskContract") ||
		failedGateCheck(checks, "taskContractDefinition") {
		if assessmentAction == "repair" {
			return "needs_replan", "repair"
		}
		return "needs_replan", firstNonEmptyVerify(assessmentAction, "replan")
	}
	return "open", firstNonEmptyVerify(assessmentAction, "satisfy_gate")
}

func failedGateCheck(checks map[string]GateCheck, name string) bool {
	check, ok := checks[name]
	return ok && !check.OK
}

func isBlockingFinding(finding map[string]any) bool {
	severity := strings.ToLower(strings.TrimSpace(stringValue(finding["severity"])))
	switch severity {
	case "critical", "high", "error":
		return true
	}
	if priority, ok := finding["priority"].(float64); ok && priority <= 1 {
		return true
	}
	status := strings.ToLower(strings.TrimSpace(stringValue(finding["status"])))
	return status == "blocking" || status == "open_blocker"
}

func findingSummary(finding map[string]any) string {
	return firstNonEmptyVerify(
		strings.TrimSpace(stringValue(finding["summary"])),
		strings.TrimSpace(stringValue(finding["title"])),
		strings.TrimSpace(stringValue(finding["kind"])),
		"blocking finding",
	)
}

func looksLikeFileEvidence(requirement string) bool {
	return strings.Contains(requirement, ".") || strings.Contains(requirement, "/")
}

func firstNonEmptyVerify(values ...string) string {
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
