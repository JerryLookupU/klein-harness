package orchestration

type TaskFamily string

const (
	TaskFamilyRepeatedEntityCorpus TaskFamily = "repeated_entity_corpus"
	TaskFamilySingleArtifact       TaskFamily = "single_artifact_generation"
	TaskFamilyBugfixSmall          TaskFamily = "bugfix_small"
	TaskFamilyFeatureModule        TaskFamily = "feature_module"
	TaskFamilyFeatureSystem        TaskFamily = "feature_system"
	TaskFamilyDevelopmentTask      TaskFamily = "development_task"
	TaskFamilyIntegrationExternal  TaskFamily = "integration_external"
	TaskFamilyReviewOrAudit        TaskFamily = "review_or_audit"
	TaskFamilyRepairOrResume       TaskFamily = "repair_or_resume"
	TaskFamilyUnknown              TaskFamily = "unknown"
)

const (
	SOPRepeatedEntityCorpusV1 = "sop.repeated_entity_corpus.v1"
	SOPDevelopmentTaskV1      = "sop.development_task.v1"
)

type SOPPhase struct {
	ID          string   `json:"id"`
	Description string   `json:"description"`
	ModelOutput []string `json:"modelOutput,omitempty"`
	ProgramOwns []string `json:"programOwns,omitempty"`
}

type VerifyRuleSpec struct {
	ID          string `json:"id"`
	Description string `json:"description"`
}

type CloseoutRuleSpec struct {
	ID          string `json:"id"`
	Description string `json:"description"`
}

type PhaseArtifactRef struct {
	PhaseID string   `json:"phaseId"`
	Layer   string   `json:"layer,omitempty"`
	Role    string   `json:"role,omitempty"`
	Path    string   `json:"path"`
	Notes   []string `json:"notes,omitempty"`
}

type SOPDefinition struct {
	ID                    string             `json:"id"`
	Family                TaskFamily         `json:"family"`
	Description           string             `json:"description"`
	DirectPassEligible    bool               `json:"directPassEligible"`
	Phases                []SOPPhase         `json:"phases"`
	VerifyRules           []VerifyRuleSpec   `json:"verifyRules,omitempty"`
	CloseoutRules         []CloseoutRuleSpec `json:"closeoutRules,omitempty"`
	ContinuationArtifacts []string           `json:"continuationArtifacts,omitempty"`
}

type RequestContext struct {
	TaskID     string     `json:"taskId,omitempty"`
	ThreadKey  string     `json:"threadKey,omitempty"`
	TaskFamily TaskFamily `json:"taskFamily,omitempty"`
	SOPID      string     `json:"sopId,omitempty"`
	Goal       string     `json:"goal,omitempty"`
	Summary    string     `json:"summary,omitempty"`
	Kind       string     `json:"kind,omitempty"`
	Contexts   []string   `json:"contexts,omitempty"`
	OwnedPaths []string   `json:"ownedPaths,omitempty"`
}

type SharedFlowContext struct {
	TaskFamily        TaskFamily         `json:"taskFamily,omitempty"`
	SOPID             string             `json:"sopId,omitempty"`
	Summary           string             `json:"summary,omitempty"`
	SharedContextPath string             `json:"sharedContextPath,omitempty"`
	SharedSpecRef     string             `json:"sharedSpecRef,omitempty"`
	VariableRef       string             `json:"variableRef,omitempty"`
	ScopeRef          string             `json:"scopeRef,omitempty"`
	ModulePlanRef     string             `json:"modulePlanRef,omitempty"`
	InterfaceRef      string             `json:"interfaceRef,omitempty"`
	TaskGraphRef      string             `json:"taskGraphRef,omitempty"`
	CompiledPhases    []string           `json:"compiledPhases,omitempty"`
	PhaseArtifacts    []PhaseArtifactRef `json:"phaseArtifacts,omitempty"`
	DirectPass        bool               `json:"directPass,omitempty"`
	VerifyPlanRef     string             `json:"verifyPlanRef,omitempty"`
	BoundarySummary   []string           `json:"boundarySummary,omitempty"`
}

type SliceLocalContext struct {
	ExecutionSliceID      string   `json:"executionSliceId,omitempty"`
	SliceMode             string   `json:"sliceMode,omitempty"`
	Sequence              int      `json:"sequence,omitempty"`
	TotalSlices           int      `json:"totalSlices,omitempty"`
	Title                 string   `json:"title,omitempty"`
	Summary               string   `json:"summary,omitempty"`
	AllowedWriteGlobs     []string `json:"allowedWriteGlobs,omitempty"`
	ForbiddenWriteGlobs   []string `json:"forbiddenWriteGlobs,omitempty"`
	OutputTargets         []string `json:"outputTargets,omitempty"`
	DoneCriteria          []string `json:"doneCriteria,omitempty"`
	Inputs                []string `json:"inputs,omitempty"`
	SharedFlowContextPath string   `json:"sharedFlowContextPath,omitempty"`
	TaskGraphPath         string   `json:"taskGraphPath,omitempty"`
	TaskContractPath      string   `json:"taskContractPath,omitempty"`
	PromptCompileInputs   []string `json:"promptCompileInputs,omitempty"`
	PromptReadOrder       []string `json:"promptReadOrder,omitempty"`
	PromptSharedInputs    []string `json:"promptSharedInputs,omitempty"`
	PromptRuntimeInputs   []string `json:"promptRuntimeInputs,omitempty"`
	PromptCloseoutInputs  []string `json:"promptCloseoutInputs,omitempty"`
	PromptGuardrails      []string `json:"promptGuardrails,omitempty"`
	ResumeArtifacts       []string `json:"resumeArtifacts,omitempty"`
}

type RuntimeControlContext struct {
	TaskID               string   `json:"taskId,omitempty"`
	DispatchID           string   `json:"dispatchId,omitempty"`
	LeaseID              string   `json:"leaseId,omitempty"`
	ExecutionSliceID     string   `json:"executionSliceId,omitempty"`
	AcceptedPacketID     string   `json:"acceptedPacketId,omitempty"`
	ResumeSessionID      string   `json:"resumeSessionId,omitempty"`
	ExecutionCWD         string   `json:"executionCwd,omitempty"`
	WorktreePath         string   `json:"worktreePath,omitempty"`
	AcceptedPacketPath   string   `json:"acceptedPacketPath,omitempty"`
	TaskContractPath     string   `json:"taskContractPath,omitempty"`
	TaskGraphPath        string   `json:"taskGraphPath,omitempty"`
	ContextLayersPath    string   `json:"contextLayersPath,omitempty"`
	RequestContextPath   string   `json:"requestContextPath,omitempty"`
	RuntimeContextPath   string   `json:"runtimeContextPath,omitempty"`
	VerifySkeletonPath   string   `json:"verifySkeletonPath,omitempty"`
	CloseoutSkeletonPath string   `json:"closeoutSkeletonPath,omitempty"`
	HandoffContractPath  string   `json:"handoffContractPath,omitempty"`
	TakeoverPath         string   `json:"takeoverPath,omitempty"`
	SessionRegistryPath  string   `json:"sessionRegistryPath,omitempty"`
	ArtifactDir          string   `json:"artifactDir,omitempty"`
	OwnedPaths           []string `json:"ownedPaths,omitempty"`
}

type ContextLayers struct {
	SchemaVersion  string                `json:"schemaVersion"`
	Request        RequestContext        `json:"request"`
	SharedFlow     SharedFlowContext     `json:"sharedFlow"`
	SliceLocal     SliceLocalContext     `json:"sliceLocal"`
	RuntimeControl RuntimeControlContext `json:"runtimeControl"`
}

type VerifyCheck struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	Target      string `json:"target,omitempty"`
	Description string `json:"description"`
}

type VerifySkeleton struct {
	SchemaVersion        string             `json:"schemaVersion"`
	TaskID               string             `json:"taskId,omitempty"`
	DispatchID           string             `json:"dispatchId,omitempty"`
	TaskFamily           TaskFamily         `json:"taskFamily,omitempty"`
	SOPID                string             `json:"sopId,omitempty"`
	ExecutionSliceID     string             `json:"executionSliceId,omitempty"`
	ExecutionCWD         string             `json:"executionCwd,omitempty"`
	WorktreePath         string             `json:"worktreePath,omitempty"`
	ContextLayersPath    string             `json:"contextLayersPath,omitempty"`
	RequestContextPath   string             `json:"requestContextPath,omitempty"`
	RuntimeContextPath   string             `json:"runtimeContextPath,omitempty"`
	TaskContractPath     string             `json:"taskContractPath,omitempty"`
	TaskGraphPath        string             `json:"taskGraphPath,omitempty"`
	HandoffContract      string             `json:"handoffContractPath,omitempty"`
	CloseoutSkeletonPath string             `json:"closeoutSkeletonPath,omitempty"`
	ProgramOwns          []string           `json:"programOwns,omitempty"`
	RequiredArtifacts    []string           `json:"requiredArtifacts,omitempty"`
	PhaseArtifacts       []PhaseArtifactRef `json:"phaseArtifacts,omitempty"`
	Checks               []VerifyCheck      `json:"checks,omitempty"`
	Notes                []string           `json:"notes,omitempty"`
}

type ContinuationProtocol struct {
	SchemaVersion         string             `json:"schemaVersion"`
	ProtocolID            string             `json:"protocolId,omitempty"`
	TaskID                string             `json:"taskId,omitempty"`
	DispatchID            string             `json:"dispatchId,omitempty"`
	TaskFamily            TaskFamily         `json:"taskFamily,omitempty"`
	SOPID                 string             `json:"sopId,omitempty"`
	ExecutionSliceID      string             `json:"executionSliceId,omitempty"`
	ResumeStrategy        string             `json:"resumeStrategy,omitempty"`
	ResumeSessionID       string             `json:"resumeSessionId,omitempty"`
	TaskStatus            string             `json:"taskStatus,omitempty"`
	ExecutionCWD          string             `json:"executionCwd,omitempty"`
	WorktreePath          string             `json:"worktreePath,omitempty"`
	ContextLayersPath     string             `json:"contextLayersPath,omitempty"`
	RequestContextPath    string             `json:"requestContextPath,omitempty"`
	RuntimeContextPath    string             `json:"runtimeContextPath,omitempty"`
	SharedFlowContextPath string             `json:"sharedFlowContextPath,omitempty"`
	SliceContextPath      string             `json:"sliceContextPath,omitempty"`
	TaskContractPath      string             `json:"taskContractPath,omitempty"`
	TaskGraphPath         string             `json:"taskGraphPath,omitempty"`
	AcceptedPacketPath    string             `json:"acceptedPacketPath,omitempty"`
	VerifySkeletonPath    string             `json:"verifySkeletonPath,omitempty"`
	CloseoutSkeletonPath  string             `json:"closeoutSkeletonPath,omitempty"`
	HandoffContractPath   string             `json:"handoffContractPath,omitempty"`
	HandoffPath           string             `json:"handoffPath,omitempty"`
	SessionRegistryPath   string             `json:"sessionRegistryPath,omitempty"`
	ArtifactDir           string             `json:"artifactDir,omitempty"`
	FileContracts         []ContinuationFile `json:"fileContracts,omitempty"`
	PhaseArtifacts        []PhaseArtifactRef `json:"phaseArtifacts,omitempty"`
	ReadOrder             []string           `json:"readOrder,omitempty"`
	RequiredArtifacts     []string           `json:"requiredArtifacts,omitempty"`
	OwnedPaths            []string           `json:"ownedPaths,omitempty"`
	AllowedWriteGlobs     []string           `json:"allowedWriteGlobs,omitempty"`
	ForbiddenWriteGlobs   []string           `json:"forbiddenWriteGlobs,omitempty"`
	EntryChecklist        []string           `json:"entryChecklist,omitempty"`
	ControlPlaneGuards    []string           `json:"controlPlaneGuards,omitempty"`
}

type ContinuationFile struct {
	ID       string   `json:"id"`
	Layer    string   `json:"layer,omitempty"`
	Role     string   `json:"role,omitempty"`
	Path     string   `json:"path"`
	Required bool     `json:"required"`
	ReadRank int      `json:"readRank,omitempty"`
	Notes    []string `json:"notes,omitempty"`
}

type HandoffSection struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required"`
}

type HandoffContract struct {
	SchemaVersion        string           `json:"schemaVersion"`
	TaskID               string           `json:"taskId,omitempty"`
	DispatchID           string           `json:"dispatchId,omitempty"`
	TaskFamily           TaskFamily       `json:"taskFamily,omitempty"`
	SOPID                string           `json:"sopId,omitempty"`
	ExecutionSliceID     string           `json:"executionSliceId,omitempty"`
	ContextLayersPath    string           `json:"contextLayersPath,omitempty"`
	TaskContractPath     string           `json:"taskContractPath,omitempty"`
	TaskGraphPath        string           `json:"taskGraphPath,omitempty"`
	VerifySkeletonPath   string           `json:"verifySkeletonPath,omitempty"`
	CloseoutSkeletonPath string           `json:"closeoutSkeletonPath,omitempty"`
	RequiredArtifacts    []string         `json:"requiredArtifacts,omitempty"`
	Sections             []HandoffSection `json:"sections,omitempty"`
	ResumeInstructions   []string         `json:"resumeInstructions,omitempty"`
}

type CloseoutSection struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required"`
}

type CloseoutSkeleton struct {
	SchemaVersion       string             `json:"schemaVersion"`
	TaskID              string             `json:"taskId,omitempty"`
	DispatchID          string             `json:"dispatchId,omitempty"`
	TaskFamily          TaskFamily         `json:"taskFamily,omitempty"`
	SOPID               string             `json:"sopId,omitempty"`
	ExecutionSliceID    string             `json:"executionSliceId,omitempty"`
	ContextLayersPath   string             `json:"contextLayersPath,omitempty"`
	TaskContractPath    string             `json:"taskContractPath,omitempty"`
	TaskGraphPath       string             `json:"taskGraphPath,omitempty"`
	VerifySkeletonPath  string             `json:"verifySkeletonPath,omitempty"`
	HandoffContractPath string             `json:"handoffContractPath,omitempty"`
	RequiredArtifacts   []string           `json:"requiredArtifacts,omitempty"`
	PhaseArtifacts      []PhaseArtifactRef `json:"phaseArtifacts,omitempty"`
	Sections            []CloseoutSection  `json:"sections,omitempty"`
	WorkerMustProvide   []string           `json:"workerMustProvide,omitempty"`
	ProgramWillFinalize []string           `json:"programWillFinalize,omitempty"`
	ResumeChecklist     []string           `json:"resumeChecklist,omitempty"`
}

type TaskGraph struct {
	SchemaVersion         string          `json:"schemaVersion"`
	TaskID                string          `json:"taskId,omitempty"`
	DispatchID            string          `json:"dispatchId,omitempty"`
	TaskFamily            TaskFamily      `json:"taskFamily,omitempty"`
	SOPID                 string          `json:"sopId,omitempty"`
	DirectPass            bool            `json:"directPass,omitempty"`
	CompileMode           string          `json:"compileMode,omitempty"`
	DirectPassReason      string          `json:"directPassReason,omitempty"`
	Phases                []string        `json:"phases,omitempty"`
	ExecutionTasks        []ExecutionTask `json:"executionTasks,omitempty"`
	SharedFlowContextPath string          `json:"sharedFlowContextPath,omitempty"`
	ResumeProtocol        string          `json:"resumeProtocol,omitempty"`
	ReplanTriggers        []string        `json:"replanTriggers,omitempty"`
	ProgramOwnedNotes     []string        `json:"programOwnedNotes,omitempty"`
	Notes                 []string        `json:"notes,omitempty"`
}

type TaskGraphCompileSpec struct {
	CompileMode       string   `json:"compileMode,omitempty"`
	DirectPassReason  string   `json:"directPassReason,omitempty"`
	ResumeProtocol    string   `json:"resumeProtocol,omitempty"`
	ReplanTriggers    []string `json:"replanTriggers,omitempty"`
	ProgramOwnedNotes []string `json:"programOwnedNotes,omitempty"`
}

type RepeatedEntitySharedSpec struct {
	OutputRoot           string   `json:"output_root,omitempty"`
	FileNamingRule       string   `json:"file_naming_rule,omitempty"`
	RequiredSupportFiles []string `json:"required_support_files,omitempty"`
	RequiredSections     []string `json:"required_sections,omitempty"`
	MinChars             int      `json:"min_chars,omitempty"`
	SourcePolicy         string   `json:"source_policy,omitempty"`
	OrderingPolicy       string   `json:"ordering_policy,omitempty"`
	IndexFilename        string   `json:"index_filename,omitempty"`
}

type RepeatedEntityInput struct {
	EntityLabel string `json:"entity_label"`
	Slug        string `json:"slug"`
	TargetFile  string `json:"target_file,omitempty"`
	CoreAngle   string `json:"core_angle,omitempty"`
}

type RepeatedEntityVariableInputs struct {
	Entities []RepeatedEntityInput `json:"entities,omitempty"`
}

type DevelopmentRequirementSpec struct {
	Goal            string   `json:"goal,omitempty"`
	InScope         []string `json:"in_scope,omitempty"`
	OutOfScope      []string `json:"out_of_scope,omitempty"`
	SuccessCriteria []string `json:"success_criteria,omitempty"`
	UserFlows       []string `json:"user_flows,omitempty"`
	Constraints     []string `json:"constraints,omitempty"`
	Risks           []string `json:"risks,omitempty"`
}

type DevelopmentArchitectureContract struct {
	TargetModules []string `json:"target_modules,omitempty"`
	NewPaths      []string `json:"new_paths,omitempty"`
	AffectedPaths []string `json:"affected_paths,omitempty"`
	ReusePoints   []string `json:"reuse_points,omitempty"`
	BoundaryRules []string `json:"boundary_rules,omitempty"`
	CoreTypes     []string `json:"core_types,omitempty"`
	CoreServices  []string `json:"core_services,omitempty"`
	DataFlow      []string `json:"data_flow,omitempty"`
	StateFlow     []string `json:"state_flow,omitempty"`
	ErrorFlow     []string `json:"error_flow,omitempty"`
}

type DevelopmentInterfaceContract struct {
	APIEndpoints    []string `json:"api_endpoints,omitempty"`
	RequestShapes   []string `json:"request_shapes,omitempty"`
	ResponseShapes  []string `json:"response_shapes,omitempty"`
	DomainModels    []string `json:"domain_models,omitempty"`
	ValidationRules []string `json:"validation_rules,omitempty"`
}

type CompiledFlow struct {
	Family                 TaskFamily                       `json:"family"`
	SOPID                  string                           `json:"sopId,omitempty"`
	SharedSpec             any                              `json:"sharedSpec,omitempty"`
	VariableInputs         any                              `json:"variableInputs,omitempty"`
	RequirementSpec        *DevelopmentRequirementSpec      `json:"requirementSpec,omitempty"`
	ArchitectureContract   *DevelopmentArchitectureContract `json:"architectureContract,omitempty"`
	InterfaceContract      *DevelopmentInterfaceContract    `json:"interfaceContract,omitempty"`
	TaskGraphCompile       TaskGraphCompileSpec             `json:"taskGraphCompile,omitempty"`
	ExecutionTasks         []ExecutionTask                  `json:"executionTasks,omitempty"`
	SharedTaskGroupContext *SharedTaskGroupContext          `json:"sharedTaskGroupContext,omitempty"`
	SharedFlowContext      SharedFlowContext                `json:"sharedFlowContext"`
}
