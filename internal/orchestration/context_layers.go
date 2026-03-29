package orchestration

func BuildContextLayers(request RequestContext, shared SharedFlowContext, slice SliceLocalContext, control RuntimeControlContext) ContextLayers {
	return ContextLayers{
		SchemaVersion:  "kh.context-layers.v1",
		Request:        request,
		SharedFlow:     shared,
		SliceLocal:     slice,
		RuntimeControl: control,
	}
}

type ContinuationProtocolInput struct {
	TaskID                string
	DispatchID            string
	TaskFamily            TaskFamily
	SOPID                 string
	ExecutionSliceID      string
	ResumeStrategy        string
	ResumeSessionID       string
	TaskStatus            string
	ExecutionCWD          string
	WorktreePath          string
	ContextLayersPath     string
	RequestContextPath    string
	RuntimeContextPath    string
	SharedFlowContextPath string
	SliceContextPath      string
	TaskContractPath      string
	TaskGraphPath         string
	AcceptedPacketPath    string
	VerifySkeletonPath    string
	CloseoutSkeletonPath  string
	HandoffContractPath   string
	HandoffPath           string
	SessionRegistryPath   string
	ArtifactDir           string
	ReadOrder             []string
	RequiredArtifacts     []string
	OwnedPaths            []string
	AllowedWriteGlobs     []string
	ForbiddenWriteGlobs   []string
	EntryChecklist        []string
	ControlPlaneGuards    []string
}

func BuildContinuationProtocol(input ContinuationProtocolInput) ContinuationProtocol {
	return ContinuationProtocol{
		SchemaVersion:         "kh.multi-session-continuation.v1",
		ProtocolID:            "mscpv1:" + input.TaskID + ":" + input.DispatchID + ":" + input.ExecutionSliceID,
		TaskID:                input.TaskID,
		DispatchID:            input.DispatchID,
		TaskFamily:            input.TaskFamily,
		SOPID:                 input.SOPID,
		ExecutionSliceID:      input.ExecutionSliceID,
		ResumeStrategy:        input.ResumeStrategy,
		ResumeSessionID:       input.ResumeSessionID,
		TaskStatus:            input.TaskStatus,
		ExecutionCWD:          input.ExecutionCWD,
		WorktreePath:          input.WorktreePath,
		ContextLayersPath:     input.ContextLayersPath,
		RequestContextPath:    input.RequestContextPath,
		RuntimeContextPath:    input.RuntimeContextPath,
		SharedFlowContextPath: input.SharedFlowContextPath,
		SliceContextPath:      input.SliceContextPath,
		TaskContractPath:      input.TaskContractPath,
		TaskGraphPath:         input.TaskGraphPath,
		AcceptedPacketPath:    input.AcceptedPacketPath,
		VerifySkeletonPath:    input.VerifySkeletonPath,
		CloseoutSkeletonPath:  input.CloseoutSkeletonPath,
		HandoffContractPath:   input.HandoffContractPath,
		HandoffPath:           input.HandoffPath,
		SessionRegistryPath:   input.SessionRegistryPath,
		ArtifactDir:           input.ArtifactDir,
		ReadOrder:             uniqueStrings(input.ReadOrder),
		RequiredArtifacts:     uniqueStrings(input.RequiredArtifacts),
		OwnedPaths:            uniqueStrings(input.OwnedPaths),
		AllowedWriteGlobs:     uniqueStrings(input.AllowedWriteGlobs),
		ForbiddenWriteGlobs:   uniqueStrings(input.ForbiddenWriteGlobs),
		EntryChecklist:        uniqueStrings(input.EntryChecklist),
		ControlPlaneGuards:    uniqueStrings(input.ControlPlaneGuards),
	}
}

func BuildVerifySkeleton(taskID, dispatchID string, family TaskFamily, sopID, executionSliceID, executionCWD, worktreePath, contextLayersPath, handoffContractPath, closeoutSkeletonPath string, programOwns, artifacts []string, checks []VerifyCheck, notes []string) VerifySkeleton {
	return VerifySkeleton{
		SchemaVersion:        "kh.verify-skeleton.v1",
		TaskID:               taskID,
		DispatchID:           dispatchID,
		TaskFamily:           family,
		SOPID:                sopID,
		ExecutionSliceID:     executionSliceID,
		ExecutionCWD:         executionCWD,
		WorktreePath:         worktreePath,
		ContextLayersPath:    contextLayersPath,
		HandoffContract:      handoffContractPath,
		CloseoutSkeletonPath: closeoutSkeletonPath,
		ProgramOwns:          uniqueStrings(programOwns),
		RequiredArtifacts:    uniqueStrings(artifacts),
		Checks:               checks,
		Notes:                uniqueStrings(notes),
	}
}

func BuildHandoffContract(taskID, dispatchID string, family TaskFamily, sopID, executionSliceID, contextLayersPath, verifySkeletonPath, closeoutSkeletonPath string, requiredArtifacts []string, sections []HandoffSection, resumeInstructions []string) HandoffContract {
	return HandoffContract{
		SchemaVersion:        "kh.handoff-contract.v1",
		TaskID:               taskID,
		DispatchID:           dispatchID,
		TaskFamily:           family,
		SOPID:                sopID,
		ExecutionSliceID:     executionSliceID,
		ContextLayersPath:    contextLayersPath,
		VerifySkeletonPath:   verifySkeletonPath,
		CloseoutSkeletonPath: closeoutSkeletonPath,
		RequiredArtifacts:    uniqueStrings(requiredArtifacts),
		Sections:             sections,
		ResumeInstructions:   uniqueStrings(resumeInstructions),
	}
}

func BuildCloseoutSkeleton(taskID, dispatchID string, family TaskFamily, sopID, executionSliceID, contextLayersPath, verifySkeletonPath, handoffContractPath string, requiredArtifacts []string, sections []CloseoutSection, workerMustProvide, programWillFinalize, resumeChecklist []string) CloseoutSkeleton {
	return CloseoutSkeleton{
		SchemaVersion:       "kh.closeout-skeleton.v1",
		TaskID:              taskID,
		DispatchID:          dispatchID,
		TaskFamily:          family,
		SOPID:               sopID,
		ExecutionSliceID:    executionSliceID,
		ContextLayersPath:   contextLayersPath,
		VerifySkeletonPath:  verifySkeletonPath,
		HandoffContractPath: handoffContractPath,
		RequiredArtifacts:   uniqueStrings(requiredArtifacts),
		Sections:            append([]CloseoutSection(nil), sections...),
		WorkerMustProvide:   uniqueStrings(workerMustProvide),
		ProgramWillFinalize: uniqueStrings(programWillFinalize),
		ResumeChecklist:     uniqueStrings(resumeChecklist),
	}
}

func BuildTaskGraph(taskID, dispatchID string, family TaskFamily, sopID string, directPass bool, phases []string, executionTasks []ExecutionTask, sharedFlowContextPath string, notes []string) TaskGraph {
	return TaskGraph{
		SchemaVersion:         "kh.task-graph.v1",
		TaskID:                taskID,
		DispatchID:            dispatchID,
		TaskFamily:            family,
		SOPID:                 sopID,
		DirectPass:            directPass,
		Phases:                uniqueStrings(phases),
		ExecutionTasks:        append([]ExecutionTask(nil), executionTasks...),
		SharedFlowContextPath: sharedFlowContextPath,
		Notes:                 uniqueStrings(notes),
	}
}
