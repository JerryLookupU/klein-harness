package orchestration

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	"klein-harness/internal/state"
)

type RejectedAlternative struct {
	CandidateID string `json:"candidateId"`
	Reason      string `json:"reason"`
}

type EntitySelection struct {
	SubjectLabel      string   `json:"subjectLabel,omitempty"`
	TargetCount       int      `json:"targetCount,omitempty"`
	SelectionMode     string   `json:"selectionMode,omitempty"`
	SelectionCriteria []string `json:"selectionCriteria,omitempty"`
	Entities          []string `json:"entities,omitempty"`
	Notes             []string `json:"notes,omitempty"`
}

type ContentContract struct {
	OutputDir         string   `json:"outputDir,omitempty"`
	OutputFile        string   `json:"outputFile,omitempty"`
	IndexFile         string   `json:"indexFile,omitempty"`
	FileExtension     string   `json:"fileExtension,omitempty"`
	FileNamingRule    string   `json:"fileNamingRule,omitempty"`
	RequiredFields    []string `json:"requiredFields,omitempty"`
	RequiredSections  []string `json:"requiredSections,omitempty"`
	MinChars          int      `json:"minChars,omitempty"`
	FormatConstraints []string `json:"formatConstraints,omitempty"`
}

type SourcePlan struct {
	ResearchGoal         string   `json:"researchGoal,omitempty"`
	PreferredSourceTypes []string `json:"preferredSourceTypes,omitempty"`
	SearchHints          []string `json:"searchHints,omitempty"`
	RequiredCrossCheck   bool     `json:"requiredCrossCheck,omitempty"`
	Notes                []string `json:"notes,omitempty"`
}

type SharedTaskGroupContext struct {
	GroupID           string          `json:"groupId,omitempty"`
	Summary           string          `json:"summary,omitempty"`
	EntitySelection   EntitySelection `json:"entitySelection,omitempty"`
	ContentContract   ContentContract `json:"contentContract,omitempty"`
	SourcePlan        SourcePlan      `json:"sourcePlan,omitempty"`
	SharedPrompt      []string        `json:"sharedPrompt,omitempty"`
	OperatorTaskList  []string        `json:"operatorTaskList,omitempty"`
	VerificationFocus []string        `json:"verificationFocus,omitempty"`
}

type ExecutionTask struct {
	ID                   string   `json:"id"`
	Title                string   `json:"title"`
	Summary              string   `json:"summary"`
	TaskGroupID          string   `json:"taskGroupId,omitempty"`
	BatchLabel           string   `json:"batchLabel,omitempty"`
	EntityBatch          []string `json:"entityBatch,omitempty"`
	OutputTargets        []string `json:"outputTargets,omitempty"`
	SharedContextSummary string   `json:"sharedContextSummary,omitempty"`
	InScope              []string `json:"inScope,omitempty"`
	DoneCriteria         []string `json:"doneCriteria,omitempty"`
	RequiredEvidence     []string `json:"requiredEvidence,omitempty"`
	VerificationSteps    []string `json:"verificationSteps,omitempty"`
}

type AcceptedPacket struct {
	SchemaVersion                 string                  `json:"schemaVersion"`
	Revision                      int64                   `json:"revision"`
	Generator                     string                  `json:"generator"`
	GeneratedAt                   string                  `json:"generatedAt"`
	TaskID                        string                  `json:"taskId"`
	TaskFamily                    TaskFamily              `json:"taskFamily,omitempty"`
	SOPID                         string                  `json:"sopId,omitempty"`
	ThreadKey                     string                  `json:"threadKey"`
	PlanEpoch                     int                     `json:"planEpoch"`
	PacketID                      string                  `json:"packetId"`
	Objective                     string                  `json:"objective"`
	Constraints                   []string                `json:"constraints"`
	FlowSelection                 string                  `json:"flowSelection"`
	PolicyTagsApplied             []string                `json:"policyTagsApplied,omitempty"`
	SelectedPlan                  string                  `json:"selectedPlan"`
	RejectedAlternatives          []RejectedAlternative   `json:"rejectedAlternatives,omitempty"`
	SharedContext                 *SharedTaskGroupContext `json:"sharedContext,omitempty"`
	SharedFlowContextPath         string                  `json:"sharedFlowContextPath,omitempty"`
	TaskGraphPath                 string                  `json:"taskGraphPath,omitempty"`
	VariableInputsPath            string                  `json:"variableInputsPath,omitempty"`
	ExecutionTasks                []ExecutionTask         `json:"executionTasks"`
	VerificationPlan              map[string]any          `json:"verificationPlan"`
	DecisionRationale             string                  `json:"decisionRationale"`
	OwnedPaths                    []string                `json:"ownedPaths,omitempty"`
	TaskBudgets                   map[string]any          `json:"taskBudgets,omitempty"`
	AcceptanceMarkers             []string                `json:"acceptanceMarkers,omitempty"`
	OrchestrationExpansionPending bool                    `json:"orchestrationExpansionPending,omitempty"`
	OrchestrationExpansionReason  string                  `json:"orchestrationExpansionReason,omitempty"`
	OrchestrationExpansionSource  string                  `json:"orchestrationExpansionSource,omitempty"`
	ReplanTriggers                []string                `json:"replanTriggers,omitempty"`
	RollbackHints                 []string                `json:"rollbackHints,omitempty"`
	AcceptedAt                    string                  `json:"acceptedAt"`
	AcceptedBy                    string                  `json:"acceptedBy"`
}

type VerificationChecklistItem struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Required bool   `json:"required"`
	Status   string `json:"status,omitempty"`
	Detail   string `json:"detail,omitempty"`
}

type TaskContract struct {
	SchemaVersion         string                      `json:"schemaVersion"`
	Revision              int64                       `json:"revision"`
	Generator             string                      `json:"generator"`
	GeneratedAt           string                      `json:"generatedAt"`
	ContractID            string                      `json:"contractId"`
	TaskID                string                      `json:"taskId"`
	TaskFamily            TaskFamily                  `json:"taskFamily,omitempty"`
	SOPID                 string                      `json:"sopId,omitempty"`
	DispatchID            string                      `json:"dispatchId"`
	ThreadKey             string                      `json:"threadKey"`
	PlanEpoch             int                         `json:"planEpoch"`
	ExecutionSliceID      string                      `json:"executionSliceId"`
	ExecutionCWD          string                      `json:"executionCwd,omitempty"`
	WorktreePath          string                      `json:"worktreePath,omitempty"`
	Objective             string                      `json:"objective"`
	InScope               []string                    `json:"inScope,omitempty"`
	OutOfScope            []string                    `json:"outOfScope,omitempty"`
	AllowedWriteGlobs     []string                    `json:"allowedWriteGlobs,omitempty"`
	ForbiddenWriteGlobs   []string                    `json:"forbiddenWriteGlobs,omitempty"`
	DoneCriteria          []string                    `json:"doneCriteria,omitempty"`
	AcceptanceMarkers     []string                    `json:"acceptanceMarkers,omitempty"`
	VerificationChecklist []VerificationChecklistItem `json:"verificationChecklist,omitempty"`
	RequiredEvidence      []string                    `json:"requiredEvidence,omitempty"`
	ReviewRequired        bool                        `json:"reviewRequired"`
	ContractStatus        string                      `json:"contractStatus"`
	ProposedBy            string                      `json:"proposedBy"`
	AcceptedBy            string                      `json:"acceptedBy"`
	AcceptedAt            string                      `json:"acceptedAt"`
	AcceptedPacketPath    string                      `json:"acceptedPacketPath,omitempty"`
	SharedFlowContextPath string                      `json:"sharedFlowContextPath,omitempty"`
	TaskGraphPath         string                      `json:"taskGraphPath,omitempty"`
	SliceContextPath      string                      `json:"sliceContextPath,omitempty"`
	ContextLayersPath     string                      `json:"contextLayersPath,omitempty"`
	VerifySkeletonPath    string                      `json:"verifySkeletonPath,omitempty"`
	CloseoutSkeletonPath  string                      `json:"closeoutSkeletonPath,omitempty"`
	HandoffContractPath   string                      `json:"handoffContractPath,omitempty"`
	TakeoverPath          string                      `json:"takeoverPath,omitempty"`
}

type PacketProgress struct {
	SchemaVersion     string   `json:"schemaVersion"`
	Revision          int64    `json:"revision"`
	Generator         string   `json:"generator"`
	UpdatedAt         string   `json:"updatedAt"`
	TaskID            string   `json:"taskId"`
	ThreadKey         string   `json:"threadKey,omitempty"`
	PlanEpoch         int      `json:"planEpoch"`
	AcceptedPacketID  string   `json:"acceptedPacketId,omitempty"`
	CompletedSliceIDs []string `json:"completedSliceIds,omitempty"`
	LastDispatchID    string   `json:"lastDispatchId,omitempty"`
}

func AcceptedPacketPath(root, taskID string) string {
	return filepath.Join(root, ".harness", "state", "accepted-packet-"+taskID+".json")
}

func TaskContractPath(artifactDir string) string {
	return filepath.Join(artifactDir, "task-contract.json")
}

func PacketProgressPath(root, taskID string) string {
	return filepath.Join(root, ".harness", "state", "packet-progress-"+taskID+".json")
}

func WriteAcceptedPacket(path string, packet AcceptedPacket) error {
	currentRevision, err := state.CurrentRevision(path)
	if err != nil {
		return err
	}
	return WriteAcceptedPacketCAS(path, packet, currentRevision)
}

func LoadAcceptedPacket(path string) (AcceptedPacket, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return AcceptedPacket{}, err
	}
	var packet AcceptedPacket
	if err := json.Unmarshal(payload, &packet); err != nil {
		return AcceptedPacket{}, err
	}
	return packet, nil
}

func WriteTaskContract(path string, contract TaskContract) error {
	currentRevision, err := state.CurrentRevision(path)
	if err != nil {
		return err
	}
	return WriteTaskContractCAS(path, contract, currentRevision)
}

func LoadTaskContract(path string) (TaskContract, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return TaskContract{}, err
	}
	var contract TaskContract
	if err := json.Unmarshal(payload, &contract); err != nil {
		return TaskContract{}, err
	}
	return contract, nil
}

func WritePacketProgress(path string, progress PacketProgress) error {
	currentRevision, err := state.CurrentRevision(path)
	if err != nil {
		return err
	}
	return WritePacketProgressCAS(path, progress, currentRevision)
}

func LoadPacketProgress(path string) (PacketProgress, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return PacketProgress{}, err
	}
	var progress PacketProgress
	if err := json.Unmarshal(payload, &progress); err != nil {
		return PacketProgress{}, err
	}
	return progress, nil
}

func WriteAcceptedPacketCAS(path string, packet AcceptedPacket, expectedRevision int64) error {
	return writeRuntimeObjectCAS(path, &packet, expectedRevision)
}

func WriteTaskContractCAS(path string, contract TaskContract, expectedRevision int64) error {
	return writeRuntimeObjectCAS(path, &contract, expectedRevision)
}

func WritePacketProgressCAS(path string, progress PacketProgress, expectedRevision int64) error {
	return writeRuntimeObjectCAS(path, &progress, expectedRevision)
}

func writeRuntimeObjectCAS(path string, value any, expectedRevision int64) error {
	currentRevision, err := state.CurrentRevision(path)
	if err != nil {
		return err
	}
	if currentRevision != expectedRevision {
		return fmt.Errorf("%w: expected %d got %d", state.ErrCASConflict, expectedRevision, currentRevision)
	}
	if err := setRuntimeObjectRevision(value, currentRevision+1); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(payload, '\n'), 0o644)
}

func setRuntimeObjectRevision(document any, revision int64) error {
	value := reflect.ValueOf(document)
	if value.Kind() != reflect.Pointer || value.IsNil() {
		return fmt.Errorf("runtime object must be a non-nil pointer")
	}
	target := value.Elem()
	if target.Kind() != reflect.Struct {
		return fmt.Errorf("runtime object must point to a struct")
	}
	field := target.FieldByName("Revision")
	if !field.IsValid() || !field.CanSet() || field.Kind() != reflect.Int64 {
		return fmt.Errorf("runtime object is missing settable Revision field")
	}
	field.SetInt(revision)
	return nil
}
