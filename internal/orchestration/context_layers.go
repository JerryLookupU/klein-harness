package orchestration

func BuildContextLayers(request RequestContext, shared SharedFlowContext, slice SliceLocalContext, control RuntimeControlContext) ContextLayers {
	return ContextLayers{
		Request:        request,
		SharedFlow:     shared,
		SliceLocal:     slice,
		RuntimeControl: control,
	}
}

func BuildContinuationProtocol(taskID, dispatchID, sharedFlowContextPath, sliceContextPath, verifySkeletonPath, handoffPath, sessionRegistryPath string, allowed, forbidden []string) ContinuationProtocol {
	return ContinuationProtocol{
		SchemaVersion:         "kh.multi-session-continuation.v1",
		TaskID:                taskID,
		DispatchID:            dispatchID,
		SharedFlowContextPath: sharedFlowContextPath,
		SliceContextPath:      sliceContextPath,
		VerifySkeletonPath:    verifySkeletonPath,
		HandoffPath:           handoffPath,
		SessionRegistryPath:   sessionRegistryPath,
		AllowedWriteGlobs:     uniqueStrings(allowed),
		ForbiddenWriteGlobs:   uniqueStrings(forbidden),
	}
}

func BuildVerifySkeleton(taskID, dispatchID string, family TaskFamily, sopID string, artifacts []string, checks []VerifyCheck, notes []string) VerifySkeleton {
	return VerifySkeleton{
		SchemaVersion:     "kh.verify-skeleton.v1",
		TaskID:            taskID,
		DispatchID:        dispatchID,
		TaskFamily:        family,
		SOPID:             sopID,
		RequiredArtifacts: uniqueStrings(artifacts),
		Checks:            checks,
		Notes:             uniqueStrings(notes),
	}
}
