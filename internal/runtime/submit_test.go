package runtime

import (
	"path/filepath"
	"testing"

	"klein-harness/internal/adapter"
	"klein-harness/internal/bootstrap"
	"klein-harness/internal/dispatch"
	"klein-harness/internal/route"
	"klein-harness/internal/state"
	"klein-harness/internal/worker"
)

func TestSubmitWritesIntakeThreadChangeAndTodoSummaries(t *testing.T) {
	root := t.TempDir()
	if _, err := bootstrap.Init(root); err != nil {
		t.Fatalf("init: %v", err)
	}

	result, err := Submit(SubmitRequest{
		Root:     root,
		Goal:     "Implement thread-aware intake",
		Contexts: []string{"docs/prd.md"},
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if result.Request.ThreadKey == "" || result.Task.ThreadKey == "" {
		t.Fatalf("expected thread keys on request/task: %+v %+v", result.Request, result.Task)
	}
	if result.Request.ThreadKey != result.Task.ThreadKey {
		t.Fatalf("expected request and task to share thread key: %+v %+v", result.Request, result.Task)
	}
	if result.Request.FrontDoorTriage == "" || result.Request.NormalizedIntentClass == "" || result.Request.FusionDecision == "" {
		t.Fatalf("expected intake metadata on request: %+v", result.Request)
	}
	if result.Request.TaskFamily == "" || result.Task.TaskFamily == "" {
		t.Fatalf("expected task family metadata on request/task: %+v %+v", result.Request, result.Task)
	}

	var intake IntakeSummary
	if err := state.LoadJSON(filepath.Join(root, ".harness", "state", "intake-summary.json"), &intake); err != nil {
		t.Fatalf("load intake summary: %v", err)
	}
	if intake.LatestRequestID != result.Request.RequestID || intake.LatestThreadKey != result.Task.ThreadKey {
		t.Fatalf("unexpected intake summary: %+v", intake)
	}

	var threadState ThreadState
	if err := state.LoadJSON(filepath.Join(root, ".harness", "state", "thread-state.json"), &threadState); err != nil {
		t.Fatalf("load thread state: %v", err)
	}
	thread := threadState.Threads[result.Task.ThreadKey]
	if thread.LatestTaskID != result.Task.TaskID || len(thread.TaskIDs) != 1 {
		t.Fatalf("unexpected thread state: %+v", threadState)
	}

	var change ChangeSummary
	if err := state.LoadJSON(filepath.Join(root, ".harness", "state", "change-summary.json"), &change); err != nil {
		t.Fatalf("load change summary: %v", err)
	}
	if change.LatestTaskID != result.Task.TaskID || change.TargetThreadKey != result.Task.ThreadKey {
		t.Fatalf("unexpected change summary: %+v", change)
	}

	var todo TodoSummary
	if err := state.LoadJSON(filepath.Join(root, ".harness", "state", "todo-summary.json"), &todo); err != nil {
		t.Fatalf("load todo summary: %v", err)
	}
	if todo.NextTaskID != result.Task.TaskID || todo.PendingCount != 1 {
		t.Fatalf("unexpected todo summary: %+v", todo)
	}
}

func TestSubmitAssignsRepeatedEntityCorpusFamilyAndSOP(t *testing.T) {
	root := t.TempDir()
	if _, err := bootstrap.Init(root); err != nil {
		t.Fatalf("init: %v", err)
	}

	result, err := Submit(SubmitRequest{
		Root: root,
		Goal: "生成 10 个世界上最伟大的程序员 markdown 文档，每个人不少于 2000 字",
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if result.Task.TaskFamily != "repeated_entity_corpus" || result.Task.SOPID != "sop.repeated_entity_corpus.v1" {
		t.Fatalf("expected repeated corpus family+sop, got %+v", result.Task)
	}
	if result.Request.TaskFamily != "repeated_entity_corpus" || result.Request.SOPID != "sop.repeated_entity_corpus.v1" {
		t.Fatalf("expected request to persist family+sop, got %+v", result.Request)
	}
	var runtimeState RuntimeState
	if err := state.LoadJSON(filepath.Join(root, ".harness", "state", "runtime.json"), &runtimeState); err != nil {
		t.Fatalf("load runtime state: %v", err)
	}
	if runtimeState.ActiveTaskID != result.Task.TaskID || runtimeState.ActiveTaskFamily != "repeated_entity_corpus" || runtimeState.ActiveSOPID != "sop.repeated_entity_corpus.v1" {
		t.Fatalf("expected runtime state to track active classification, got %+v", runtimeState)
	}
	if runtimeState.CurrentDispatchID != "" || runtimeState.CurrentExecutionSliceID != "" || runtimeState.CurrentTakeoverPath != "" {
		t.Fatalf("submit should not set execution refs before dispatch, got %+v", runtimeState)
	}
}

func TestSubmitReusesQueuedTaskForSameCanonicalGoal(t *testing.T) {
	root := t.TempDir()
	if _, err := bootstrap.Init(root); err != nil {
		t.Fatalf("init: %v", err)
	}

	first, err := Submit(SubmitRequest{
		Root: root,
		Goal: "Fix runtime verify bug",
	})
	if err != nil {
		t.Fatalf("first submit: %v", err)
	}
	second, err := Submit(SubmitRequest{
		Root:     root,
		Goal:     "Fix runtime   verify bug",
		Contexts: []string{"logs/run-2.txt"},
	})
	if err != nil {
		t.Fatalf("second submit: %v", err)
	}
	if second.Task.ThreadKey != first.Task.ThreadKey {
		t.Fatalf("expected second submit to bind to existing thread: first=%s second=%s", first.Task.ThreadKey, second.Task.ThreadKey)
	}
	if second.Request.FusionDecision != "accepted_existing_thread" {
		t.Fatalf("expected existing-thread fusion: %+v", second.Request)
	}
	if second.Request.NormalizedIntentClass != "context_enrichment" {
		t.Fatalf("expected context enrichment classification: %+v", second.Request)
	}
	if second.Request.BindingAction != "reused_existing_task" || second.Request.TaskID != first.Task.TaskID {
		t.Fatalf("expected second request to reuse queued task: %+v", second.Request)
	}

	var threadState ThreadState
	if err := state.LoadJSON(filepath.Join(root, ".harness", "state", "thread-state.json"), &threadState); err != nil {
		t.Fatalf("load thread state: %v", err)
	}
	thread := threadState.Threads[first.Task.ThreadKey]
	if len(thread.TaskIDs) != 1 || len(thread.RequestIDs) != 2 {
		t.Fatalf("expected one queued task and two requests on shared thread: %+v", thread)
	}

	pool, err := adapter.LoadTaskPool(root)
	if err != nil {
		t.Fatalf("load task pool: %v", err)
	}
	if len(pool.Tasks) != 1 || pool.Tasks[0].TaskID != first.Task.TaskID {
		t.Fatalf("expected only reused task in pool: %+v", pool.Tasks)
	}
}

func TestSubmitCreatesNewTaskWhenMatchedTaskRunning(t *testing.T) {
	root := t.TempDir()
	if _, err := bootstrap.Init(root); err != nil {
		t.Fatalf("init: %v", err)
	}

	first, err := Submit(SubmitRequest{
		Root: root,
		Goal: "Fix runtime verify bug",
	})
	if err != nil {
		t.Fatalf("first submit: %v", err)
	}
	if err := updateTask(root, first.Task.TaskID, func(current *adapter.Task) {
		current.Status = "running"
		current.LastDispatchID = "dispatch-1"
		current.UpdatedAt = state.NowUTC()
	}); err != nil {
		t.Fatalf("mark running task: %v", err)
	}

	second, err := Submit(SubmitRequest{
		Root: root,
		Goal: "Fix runtime verify bug",
	})
	if err != nil {
		t.Fatalf("second submit: %v", err)
	}
	if second.Task.ThreadKey != first.Task.ThreadKey {
		t.Fatalf("expected follow-up task to stay on same thread: first=%+v second=%+v", first.Task, second.Task)
	}
	if second.Task.TaskID == first.Task.TaskID {
		t.Fatalf("expected running task to force a new follow-up task: first=%+v second=%+v", first.Task, second.Task)
	}
	if second.Request.BindingAction != "created_new_task" {
		t.Fatalf("expected follow-up request to create new task: %+v", second.Request)
	}
}

func TestSubmitCarriesForwardMatchedThreadPlanEpoch(t *testing.T) {
	root := t.TempDir()
	if _, err := bootstrap.Init(root); err != nil {
		t.Fatalf("init: %v", err)
	}
	if err := adapter.UpsertTask(root, adapter.Task{
		TaskID:    "T-7",
		ThreadKey: "thread-7",
		Title:     "Refine runtime intake",
		Summary:   "Refine runtime intake",
		PlanEpoch: 4,
		Status:    "needs_replan",
	}); err != nil {
		t.Fatalf("upsert seed task: %v", err)
	}

	result, err := Submit(SubmitRequest{
		Root: root,
		Goal: "Refine runtime intake",
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if result.Request.TargetPlanEpoch != 4 {
		t.Fatalf("expected target plan epoch to reuse matched thread epoch: %+v", result.Request)
	}
	if result.Task.PlanEpoch != 4 {
		t.Fatalf("expected task plan epoch to reuse matched thread epoch: %+v", result.Task)
	}
	if result.Task.ThreadKey != "thread-7" {
		t.Fatalf("expected matched thread key, got %+v", result.Task)
	}
}

func TestSubmitContextEnrichmentReadOnlyAvoidsReplan(t *testing.T) {
	root := t.TempDir()
	if _, err := bootstrap.Init(root); err != nil {
		t.Fatalf("init: %v", err)
	}

	goal := "Refine runtime intake"
	ctx := "展示：只需读面分析，不修改代码"

	first, err := Submit(SubmitRequest{
		Root: root,
		Goal: goal,
		Kind: "",
	})
	if err != nil {
		t.Fatalf("first submit: %v", err)
	}

	second, err := Submit(SubmitRequest{
		Root:     root,
		Goal:     goal,
		Contexts: []string{ctx},
	})
	if err != nil {
		t.Fatalf("second submit: %v", err)
	}

	if tri := frontDoorTriage(goal, []string{ctx}); tri != "advisory_read_only" {
		t.Fatalf("frontDoorTriage mismatch: expected advisory_read_only, got %s", tri)
	}

	pool, err := adapter.LoadTaskPool(root)
	if err != nil {
		t.Fatalf("load task pool: %v", err)
	}
	classification := classifySubmission(SubmitRequest{
		Root:     root,
		Goal:     goal,
		Contexts: []string{ctx},
	}, pool.Tasks)
	if classification.FrontDoorTriage != "advisory_read_only" {
		t.Fatalf("classifySubmission mismatch: expected advisory_read_only, got %s", classification.FrontDoorTriage)
	}

	if second.Task.TaskID != first.Task.TaskID {
		t.Fatalf("expected context enrichment to bind to reused task: first=%s second=%s", first.Task.TaskID, second.Task.TaskID)
	}
	if second.Request.NormalizedIntentClass != "context_enrichment" {
		t.Fatalf("expected context enrichment classification: %+v", second.Request)
	}
	if second.Request.FrontDoorTriage != "advisory_read_only" {
		t.Fatalf("expected advisory_read_only for read-only context enrichment, got FrontDoorTriage=%s", second.Request.FrontDoorTriage)
	}

	routeInput, err := BuildRouteInput(root, second.Task, second.Task.PlanEpoch, false, false, "state.v9")
	if err != nil {
		t.Fatalf("build route input: %v", err)
	}
	if routeInput.ChangeAffectsExecution {
		t.Fatalf("expected read-only context enrichment to not affect execution, got ChangeAffectsExecution=true")
	}

	decision := route.Evaluate(routeInput)
	if decision.Route == "replan" || !decision.DispatchReady {
		t.Fatalf("expected route dispatch for read-only context enrichment, got decision=%+v", decision)
	}
}

func TestSubmitWritesRequestSummaryIndexAndTaskMap(t *testing.T) {
	root := t.TempDir()
	if _, err := bootstrap.Init(root); err != nil {
		t.Fatalf("init: %v", err)
	}

	first, err := Submit(SubmitRequest{
		Root: root,
		Goal: "Refine request hot state",
	})
	if err != nil {
		t.Fatalf("first submit: %v", err)
	}
	second, err := Submit(SubmitRequest{
		Root:     root,
		Goal:     "Refine request hot state",
		Contexts: []string{"docs/request.md"},
	})
	if err != nil {
		t.Fatalf("second submit: %v", err)
	}

	var summary RequestSummary
	if err := state.LoadJSON(filepath.Join(root, ".harness", "state", "request-summary.json"), &summary); err != nil {
		t.Fatalf("load request summary: %v", err)
	}
	if summary.LatestRequestID != second.Request.RequestID || summary.LatestTaskID != first.Task.TaskID || summary.ReusedTaskCount != 1 || summary.CreatedTaskCount != 1 {
		t.Fatalf("unexpected request summary: %+v", summary)
	}

	var index RequestIndex
	if err := state.LoadJSON(filepath.Join(root, ".harness", "state", "request-index.json"), &index); err != nil {
		t.Fatalf("load request index: %v", err)
	}
	if index.LatestRequestByTaskID[first.Task.TaskID] != second.Request.RequestID {
		t.Fatalf("expected latest request by task id to point at reused request: %+v", index)
	}
	if index.RequestsByID[second.Request.RequestID].TaskID != first.Task.TaskID {
		t.Fatalf("expected request index to store reused task binding: %+v", index.RequestsByID[second.Request.RequestID])
	}

	var mapping RequestTaskMap
	if err := state.LoadJSON(filepath.Join(root, ".harness", "state", "request-task-map.json"), &mapping); err != nil {
		t.Fatalf("load request-task map: %v", err)
	}
	if mapping.RequestToTask[second.Request.RequestID] != first.Task.TaskID {
		t.Fatalf("expected request to task mapping for reused request: %+v", mapping)
	}
	if len(mapping.TaskToRequests[first.Task.TaskID]) != 2 || len(mapping.ThreadToTasks[first.Task.ThreadKey]) != 1 {
		t.Fatalf("expected task/thread mappings to stay consistent under reuse: %+v", mapping)
	}
}

func TestRefreshTodoSummaryTracksPendingTasksAfterStatusChanges(t *testing.T) {
	root := t.TempDir()
	paths, err := bootstrap.Init(root)
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	seed := []adapter.Task{
		{TaskID: "T-1", ThreadKey: "thread-1", Summary: "queued", Status: "queued"},
		{TaskID: "T-2", ThreadKey: "thread-2", Summary: "running", Status: "running"},
		{TaskID: "T-3", ThreadKey: "thread-3", Summary: "done", Status: "completed"},
	}
	for _, task := range seed {
		if err := adapter.UpsertTask(root, task); err != nil {
			t.Fatalf("upsert task %s: %v", task.TaskID, err)
		}
	}

	if err := refreshTodoSummary(paths, "thread-2", "R-2"); err != nil {
		t.Fatalf("refresh todo summary: %v", err)
	}

	var todo TodoSummary
	if err := state.LoadJSON(filepath.Join(root, ".harness", "state", "todo-summary.json"), &todo); err != nil {
		t.Fatalf("load todo summary: %v", err)
	}
	if todo.PendingCount != 2 {
		t.Fatalf("expected only queued/running tasks to remain pending: %+v", todo)
	}
	if len(todo.TaskIDs) != 2 || todo.TaskIDs[0] != "T-1" || todo.TaskIDs[1] != "T-2" {
		t.Fatalf("unexpected todo task ids: %+v", todo.TaskIDs)
	}
}

func TestRefreshExecutionIndexesUpdatesThreadStateAfterRuntimeTransition(t *testing.T) {
	root := t.TempDir()
	paths, err := bootstrap.Init(root)
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	task := adapter.Task{
		TaskID:      "T-9",
		ThreadKey:   "thread-9",
		Title:       "Runtime refresh",
		Summary:     "Runtime refresh",
		PlanEpoch:   2,
		Status:      "completed",
		UpdatedAt:   "2026-03-26T10:30:00Z",
		CompletedAt: "2026-03-26T10:30:00Z",
	}
	if err := adapter.UpsertTask(root, task); err != nil {
		t.Fatalf("upsert task: %v", err)
	}

	if err := refreshExecutionIndexes(paths, task, "", "goalhash-9"); err != nil {
		t.Fatalf("refresh execution indexes: %v", err)
	}

	var threadState ThreadState
	if err := state.LoadJSON(filepath.Join(root, ".harness", "state", "thread-state.json"), &threadState); err != nil {
		t.Fatalf("load thread state: %v", err)
	}
	entry := threadState.Threads["thread-9"]
	if entry.Status != "completed" || entry.PlanEpoch != 2 || entry.LatestTaskID != "T-9" {
		t.Fatalf("unexpected thread entry after refresh: %+v", entry)
	}
}

func TestRefreshThreadStateWritesCurrentAndLatestValidPlanEpoch(t *testing.T) {
	root := t.TempDir()
	paths, err := bootstrap.Init(root)
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	task := adapter.Task{
		TaskID:    "T-8",
		ThreadKey: "thread-8",
		Summary:   "refresh thread state",
		PlanEpoch: 4,
		Status:    "queued",
		UpdatedAt: "2026-03-26T12:00:00Z",
	}

	if err := refreshThreadState(paths, task, "R-8", "goalhash-8"); err != nil {
		t.Fatalf("refresh thread state: %v", err)
	}

	var threadState ThreadState
	if err := state.LoadJSON(filepath.Join(root, ".harness", "state", "thread-state.json"), &threadState); err != nil {
		t.Fatalf("load thread state: %v", err)
	}
	entry := threadState.Threads["thread-8"]
	if entry.PlanEpoch != 4 || entry.CurrentPlanEpoch != 4 || entry.LatestValidPlanEpoch != 4 {
		t.Fatalf("expected current/latest valid epoch fields to be written together: %+v", entry)
	}
}

func TestRunOnceRoutesContextEnrichmentToNeedsReplan(t *testing.T) {
	root := t.TempDir()
	if _, err := bootstrap.Init(root); err != nil {
		t.Fatalf("init: %v", err)
	}

	first, err := Submit(SubmitRequest{
		Root: root,
		Goal: "Refine runtime intake",
	})
	if err != nil {
		t.Fatalf("first submit: %v", err)
	}
	if err := updateTask(root, first.Task.TaskID, func(current *adapter.Task) {
		current.Status = "completed"
		current.UpdatedAt = state.NowUTC()
	}); err != nil {
		t.Fatalf("complete first task: %v", err)
	}

	second, err := Submit(SubmitRequest{
		Root:     root,
		Goal:     "Refine runtime intake",
		Contexts: []string{"docs/new-context.md"},
	})
	if err != nil {
		t.Fatalf("second submit: %v", err)
	}

	result, err := RunOnce(root, RunOptions{})
	if err != nil {
		t.Fatalf("run once: %v", err)
	}
	if result.RuntimeStatus != "needs_replan" {
		t.Fatalf("expected needs_replan runtime status, got %+v", result)
	}
	if result.Route.Route != "replan" || result.Route.DispatchReady {
		t.Fatalf("expected replan route decision, got %+v", result.Route)
	}
	if !containsString(result.Route.ReasonCodes, "context_enrichment_requires_replan") {
		t.Fatalf("expected context enrichment reason code, got %+v", result.Route.ReasonCodes)
	}

	task, err := adapter.LoadTask(root, second.Task.TaskID)
	if err != nil {
		t.Fatalf("load second task: %v", err)
	}
	if task.Status != "needs_replan" {
		t.Fatalf("expected second task to move into needs_replan, got %+v", task)
	}
}

func TestBuildRouteInputCarriesIntakeSignals(t *testing.T) {
	root := t.TempDir()
	if _, err := bootstrap.Init(root); err != nil {
		t.Fatalf("init: %v", err)
	}
	first, err := Submit(SubmitRequest{
		Root: root,
		Goal: "Refine route helper",
	})
	if err != nil {
		t.Fatalf("first submit: %v", err)
	}
	second, err := Submit(SubmitRequest{
		Root:     root,
		Goal:     "Refine route helper",
		Contexts: []string{"docs/helper.md"},
	})
	if err != nil {
		t.Fatalf("second submit: %v", err)
	}

	input, err := BuildRouteInput(root, second.Task, second.Task.PlanEpoch, false, false, "state.v9")
	if err != nil {
		t.Fatalf("build route input: %v", err)
	}
	if input.FusionDecision != "accepted_existing_thread" {
		t.Fatalf("expected fusion decision from latest request, got %+v", input)
	}
	if input.NormalizedIntentClass != "context_enrichment" {
		t.Fatalf("expected context enrichment class, got %+v", input)
	}
	if input.PendingTaskCount != 1 {
		t.Fatalf("expected pending task count from todo summary, got %+v", input)
	}
	if input.RequiredSummaryVersion != "state.v9" {
		t.Fatalf("expected required summary version passthrough, got %+v", input)
	}
	if input.TaskID != second.Task.TaskID || input.TaskID != first.Task.TaskID {
		t.Fatalf("expected route input to bind to reused task, got %+v", input)
	}
}

func TestEnsureTaskClassificationBackfillsLegacyTaskMetadata(t *testing.T) {
	root := t.TempDir()
	if _, err := bootstrap.Init(root); err != nil {
		t.Fatalf("init: %v", err)
	}
	result, err := Submit(SubmitRequest{
		Root: root,
		Goal: "恢复上次中断的 session 并继续执行 verify 收口",
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if err := updateTask(root, result.Task.TaskID, func(current *adapter.Task) {
		current.TaskFamily = ""
		current.SOPID = ""
		current.UpdatedAt = state.NowUTC()
	}); err != nil {
		t.Fatalf("clear task classification: %v", err)
	}

	task, err := adapter.LoadTask(root, result.Task.TaskID)
	if err != nil {
		t.Fatalf("load task: %v", err)
	}
	task, err = ensureTaskClassification(root, task)
	if err != nil {
		t.Fatalf("ensure task classification: %v", err)
	}
	if task.TaskFamily != "repair_or_resume" || task.SOPID != "sop.development_task.v1" {
		t.Fatalf("expected legacy task to be backfilled from classifier, got %+v", task)
	}

	routeInput, err := BuildRouteInput(root, task, task.PlanEpoch, false, false, "state.v9")
	if err != nil {
		t.Fatalf("build route input: %v", err)
	}
	if routeInput.TaskFamily != "repair_or_resume" || routeInput.SOPID != "sop.development_task.v1" {
		t.Fatalf("expected route input to carry backfilled classification, got %+v", routeInput)
	}
}

func TestBindRuntimeDispatchTracksCompiledContractRefs(t *testing.T) {
	task := adapter.Task{
		TaskID:                   "T-401",
		ThreadKey:                "thread-401",
		TaskFamily:               "feature_system",
		SOPID:                    "sop.development_task.v1",
		PreferredResumeSessionID: "sess-fallback",
		OwnedPaths:               []string{"internal/runtime/**"},
	}
	ticket := dispatch.Ticket{
		DispatchID:      "dispatch-401",
		ResumeSessionID: "sess-401",
	}
	bundle := worker.DispatchBundle{
		ExecutionSliceID:   "T-401.slice.2",
		TakeoverPath:       "/repo/.harness/artifacts/T-401/dispatch-401/takeover-context.json",
		ContextLayersPath:  "/repo/.harness/artifacts/T-401/dispatch-401/context-layers.json",
		TaskGraphPath:      "/repo/.harness/artifacts/T-401/dispatch-401/task-graph.json",
		VerifySkeletonPath: "/repo/.harness/artifacts/T-401/dispatch-401/verify-skeleton.json",
		CloseoutPath:       "/repo/.harness/artifacts/T-401/dispatch-401/closeout-skeleton.json",
		HandoffPath:        "/repo/.harness/artifacts/T-401/dispatch-401/handoff.md",
		ArtifactDir:        "/repo/.harness/artifacts/T-401/dispatch-401",
	}

	current := bindRuntimeTask(RuntimeState{}, task, "/repo/.worktrees/T-401")
	current = bindRuntimeDispatch(current, task, ticket, bundle)

	if current.ActiveTaskID != "T-401" || current.ActiveTaskFamily != "feature_system" || current.ActiveSOPID != "sop.development_task.v1" {
		t.Fatalf("expected runtime state to retain active task classification, got %+v", current)
	}
	if current.CurrentDispatchID != ticket.DispatchID || current.CurrentExecutionSliceID != bundle.ExecutionSliceID || current.CurrentResumeSessionID != "sess-401" {
		t.Fatalf("expected runtime state to retain dispatch binding, got %+v", current)
	}
	if current.CurrentTakeoverPath != bundle.TakeoverPath || current.CurrentContextLayersPath != bundle.ContextLayersPath || current.CurrentTaskGraphPath != bundle.TaskGraphPath {
		t.Fatalf("expected runtime state to carry continuation contract refs, got %+v", current)
	}
	if current.CurrentVerifySkeletonPath != bundle.VerifySkeletonPath || current.CurrentCloseoutPath != bundle.CloseoutPath || current.CurrentHandoffPath != bundle.HandoffPath {
		t.Fatalf("expected runtime state to carry verify/closeout refs, got %+v", current)
	}
	if current.CurrentArtifactDir != bundle.ArtifactDir {
		t.Fatalf("expected runtime state to carry artifact dir, got %+v", current)
	}
	if cleared := clearRuntimeExecutionRefs(current); cleared.CurrentDispatchID != "" || cleared.CurrentTakeoverPath != "" || cleared.CurrentArtifactDir != "" {
		t.Fatalf("expected runtime execution refs to be clearable, got %+v", cleared)
	}
}

func TestShouldEnterAnalysisLoopDoesNotRetryBlockedByDefault(t *testing.T) {
	if ShouldEnterAnalysisLoop("", "blocked", "", nil) {
		t.Fatalf("expected blocked verification to stop instead of auto-retrying")
	}
	if ShouldEnterAnalysisLoop("", "", "task.blocked", nil) {
		t.Fatalf("expected task.blocked follow-up to stop instead of auto-retrying")
	}
	if !ShouldEnterAnalysisLoop("", "", "replan.emitted", nil) {
		t.Fatalf("expected explicit replan follow-up to stay retryable")
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
