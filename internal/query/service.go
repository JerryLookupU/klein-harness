package query

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"klein-harness/internal/adapter"
	"klein-harness/internal/dispatch"
	"klein-harness/internal/lease"
	"klein-harness/internal/orchestration"
	"klein-harness/internal/runtime"
	"klein-harness/internal/state"
	"klein-harness/internal/tmux"
	"klein-harness/internal/verify"
)

type PlanningView struct {
	DispatchTicketPath string                              `json:"dispatchTicketPath,omitempty"`
	PromptPath         string                              `json:"promptPath,omitempty"`
	PlanningTracePath  string                              `json:"planningTracePath,omitempty"`
	ConstraintPath     string                              `json:"constraintPath,omitempty"`
	AcceptedPacketPath string                              `json:"acceptedPacketPath,omitempty"`
	TaskContractPath   string                              `json:"taskContractPath,omitempty"`
	RequestContextPath string                              `json:"requestContextPath,omitempty"`
	RuntimeContextPath string                              `json:"runtimeContextPath,omitempty"`
	ContextLayersPath  string                              `json:"contextLayersPath,omitempty"`
	SharedFlowPath     string                              `json:"sharedFlowPath,omitempty"`
	SliceContextPath   string                              `json:"sliceContextPath,omitempty"`
	TaskGraphPath      string                              `json:"taskGraphPath,omitempty"`
	VerifySkeletonPath string                              `json:"verifySkeletonPath,omitempty"`
	CloseoutPath       string                              `json:"closeoutPath,omitempty"`
	HandoffPath        string                              `json:"handoffPath,omitempty"`
	TakeoverPath       string                              `json:"takeoverPath,omitempty"`
	ExecutionSliceID   string                              `json:"executionSliceId,omitempty"`
	ResumeStrategy     string                              `json:"resumeStrategy,omitempty"`
	SessionID          string                              `json:"sessionId,omitempty"`
	PromptStages       []string                            `json:"promptStages,omitempty"`
	Methodology        orchestration.MethodologyContract   `json:"methodology"`
	JudgeDecision      orchestration.JudgeDecision         `json:"judgeDecision"`
	ExecutionLoop      orchestration.ExecutionLoopContract `json:"executionLoop"`
	ConstraintSystem   orchestration.ConstraintSystem      `json:"constraintSystem"`
	PacketSynthesis    orchestration.PacketSynthesisLoop   `json:"packetSynthesis"`
	PlannerCandidates  []orchestration.PlannerCandidate    `json:"plannerCandidates,omitempty"`
	ActiveSkills       []string                            `json:"activeSkills,omitempty"`
	SkillHints         []string                            `json:"skillHints,omitempty"`
	TracePreview       []string                            `json:"tracePreview,omitempty"`
}

type TaskView struct {
	Task            adapter.Task                  `json:"task"`
	Dispatch        *dispatch.Ticket              `json:"dispatch,omitempty"`
	Lease           *lease.Record                 `json:"lease,omitempty"`
	Completion      *verify.CompletionGate        `json:"completionGate,omitempty"`
	Guard           *verify.GuardState            `json:"guardState,omitempty"`
	Release         ReleaseReadiness              `json:"release"`
	Tmux            *tmux.SessionState            `json:"tmux,omitempty"`
	Planning        *PlanningView                 `json:"planning,omitempty"`
	AcceptedPacket  *orchestration.AcceptedPacket `json:"acceptedPacket,omitempty"`
	PacketProgress  *orchestration.PacketProgress `json:"packetProgress,omitempty"`
	RemainingSlices []string                      `json:"remainingSlices,omitempty"`
	NextSliceID     string                        `json:"nextSliceId,omitempty"`
	TaskContract    *orchestration.TaskContract   `json:"taskContract,omitempty"`
	ContextLayers   *orchestration.ContextLayers  `json:"contextLayers,omitempty"`
	SharedFlow      *orchestration.SharedFlowContext `json:"sharedFlow,omitempty"`
	SliceContext    *orchestration.SliceLocalContext `json:"sliceContext,omitempty"`
	VerifySkeleton  *orchestration.VerifySkeleton `json:"verifySkeleton,omitempty"`
	Closeout        *orchestration.CloseoutSkeleton `json:"closeout,omitempty"`
	Handoff         *orchestration.HandoffContract `json:"handoff,omitempty"`
	Continuation    *orchestration.ContinuationProtocol `json:"continuation,omitempty"`
	Assessment      *verify.Assessment            `json:"assessment,omitempty"`
	Request         *runtime.RequestRecord        `json:"request,omitempty"`
	IntakeSummary   *runtime.IntakeSummary        `json:"intakeSummary,omitempty"`
	ThreadEntry     *runtime.ThreadEntry          `json:"threadEntry,omitempty"`
	ChangeSummary   *runtime.ChangeSummary        `json:"changeSummary,omitempty"`
	TodoSummary     *runtime.TodoSummary          `json:"todoSummary,omitempty"`
	OuterLoopMemory *verify.TaskFeedbackSummary   `json:"outerLoopMemory,omitempty"`
	ActiveSkills    []string                      `json:"activeSkills,omitempty"`
	SkillHints      []string                      `json:"skillHints,omitempty"`
	AttachCommand   string                        `json:"attachCommand,omitempty"`
	LogPreview      []string                      `json:"logPreview,omitempty"`
}

type ReleaseReadiness struct {
	Status          string   `json:"status"`
	Ready           bool     `json:"ready"`
	SafeToArchive   bool     `json:"safeToArchive"`
	NextAction      string   `json:"nextAction,omitempty"`
	BlockingReasons []string `json:"blockingReasons,omitempty"`
}

type ReleaseBoardItem struct {
	TaskID          string   `json:"taskId"`
	ThreadKey       string   `json:"threadKey,omitempty"`
	Title           string   `json:"title,omitempty"`
	TaskStatus      string   `json:"taskStatus,omitempty"`
	ReleaseStatus   string   `json:"releaseStatus"`
	Ready           bool     `json:"ready"`
	SafeToArchive   bool     `json:"safeToArchive"`
	NextAction      string   `json:"nextAction,omitempty"`
	BlockingReasons []string `json:"blockingReasons,omitempty"`
}

type ReleaseBoard struct {
	ReadyCount          int                `json:"readyCount"`
	NeedsReviewCount    int                `json:"needsReviewCount"`
	NeedsReplanCount    int                `json:"needsReplanCount"`
	AwaitingGateCount   int                `json:"awaitingGateCount"`
	BlockedCount        int                `json:"blockedCount"`
	RemainingSliceCount int                `json:"remainingSliceCount"`
	Items               []ReleaseBoardItem `json:"items"`
}

type ReleaseSnapshot struct {
	state.Metadata
	GeneratedAt      string       `json:"generatedAt"`
	Root             string       `json:"root"`
	Version          string       `json:"version"`
	ChangelogVersion string       `json:"changelogVersion,omitempty"`
	Dirty            bool         `json:"dirty"`
	Ready            bool         `json:"ready"`
	BlockingReasons  []string     `json:"blockingReasons,omitempty"`
	ReleaseBoard     ReleaseBoard `json:"releaseBoard"`
}

func ListTasks(root string) ([]adapter.Task, error) {
	pool, err := adapter.LoadTaskPool(root)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(pool.Tasks, func(i, j int) bool {
		return pool.Tasks[i].TaskID < pool.Tasks[j].TaskID
	})
	return pool.Tasks, nil
}

func Task(root, taskID string) (TaskView, error) {
	task, err := adapter.LoadTask(root, taskID)
	if err != nil {
		return TaskView{}, err
	}
	view := TaskView{Task: task}
	if task.LastDispatchID != "" {
		ticket, err := dispatch.Get(root, task.LastDispatchID)
		if err == nil {
			view.Dispatch = &ticket
		}
	}
	if task.LastLeaseID != "" {
		record, err := lease.ValidateCurrent(root, task.LastLeaseID, task.TaskID, task.LastDispatchID)
		if err == nil {
			view.Lease = &record
		}
	}
	paths, err := adapter.Resolve(root)
	if err != nil {
		return TaskView{}, err
	}
	if gate, ok, err := loadTaskCompletionGate(paths, task.TaskID); err != nil {
		return TaskView{}, err
	} else if ok {
		view.Completion = &gate
	}
	if guard, ok, err := loadTaskGuardState(paths, task.TaskID); err != nil {
		return TaskView{}, err
	} else if ok {
		view.Guard = &guard
	}
	if session, ok, err := tmux.FindTaskSession(root, task.TaskID, task.TmuxSession); err == nil && ok {
		copy := session
		view.Tmux = &copy
		view.Task.TmuxSession = session.SessionName
		if view.Task.TmuxLogPath == "" {
			view.Task.TmuxLogPath = session.LogPath
		}
		view.AttachCommand = session.AttachCommand
	}
	if view.AttachCommand == "" && view.Task.TmuxSession != "" {
		view.AttachCommand = tmux.AttachCommand(view.Task.TmuxSession)
	}
	planning, err := loadPlanningView(paths.StateDir, task.TaskID)
	if err != nil {
		return TaskView{}, err
	}
	if planning != nil {
		view.Planning = planning
		view.ActiveSkills = append([]string(nil), planning.ActiveSkills...)
		view.SkillHints = append([]string(nil), planning.SkillHints...)
	}
	if packet, ok, err := loadAcceptedPacket(paths.Root, task.TaskID); err != nil {
		return TaskView{}, err
	} else if ok {
		view.AcceptedPacket = &packet
	}
	if progress, ok, err := loadPacketProgress(paths.Root, task.TaskID); err != nil {
		return TaskView{}, err
	} else if ok {
		view.PacketProgress = &progress
	}
	if task.LastDispatchID != "" {
		if contract, ok, err := loadTaskContract(paths.Root, task.TaskID, task.LastDispatchID); err != nil {
			return TaskView{}, err
		} else if ok {
			view.TaskContract = &contract
		}
		if assessment, ok, err := loadAssessment(paths.Root, task.TaskID, task.LastDispatchID); err != nil {
			return TaskView{}, err
		} else if ok {
			view.Assessment = &assessment
		}
	}
	if contextLayers, ok, err := loadContextLayers(view.Planning, view.TaskContract); err != nil {
		return TaskView{}, err
	} else if ok {
		view.ContextLayers = &contextLayers
	}
	if sharedFlow, ok, err := loadSharedFlowContext(view.Planning, view.TaskContract); err != nil {
		return TaskView{}, err
	} else if ok {
		view.SharedFlow = &sharedFlow
	}
	if sliceContext, ok, err := loadSliceContext(view.Planning, view.TaskContract); err != nil {
		return TaskView{}, err
	} else if ok {
		view.SliceContext = &sliceContext
	}
	if verifySkeleton, ok, err := loadVerifySkeleton(view.Planning, view.TaskContract); err != nil {
		return TaskView{}, err
	} else if ok {
		view.VerifySkeleton = &verifySkeleton
	}
	if closeout, ok, err := loadCloseoutSkeleton(view.Planning, view.TaskContract); err != nil {
		return TaskView{}, err
	} else if ok {
		view.Closeout = &closeout
	}
	if handoff, ok, err := loadHandoffContract(view.Planning, view.TaskContract); err != nil {
		return TaskView{}, err
	} else if ok {
		view.Handoff = &handoff
	}
	if continuation, ok, err := loadContinuation(view.Planning, view.TaskContract); err != nil {
		return TaskView{}, err
	} else if ok {
		view.Continuation = &continuation
	}
	if view.AcceptedPacket != nil {
		view.RemainingSlices = remainingExecutionSlices(*view.AcceptedPacket, view.PacketProgress)
		if len(view.RemainingSlices) > 0 {
			view.NextSliceID = view.RemainingSlices[0]
		}
	}
	if request, ok, err := loadLatestRequestForTask(paths, task.TaskID); err != nil {
		return TaskView{}, err
	} else if ok {
		view.Request = &request
	}
	if intake, ok, err := loadIntakeSummary(paths.StateDir); err != nil {
		return TaskView{}, err
	} else if ok {
		view.IntakeSummary = &intake
	}
	if thread, ok, err := loadThreadEntry(paths.StateDir, firstNonEmpty(task.ThreadKey, view.Task.ThreadKey, task.TaskID)); err != nil {
		return TaskView{}, err
	} else if ok {
		view.ThreadEntry = &thread
	}
	if change, ok, err := loadChangeSummary(paths.StateDir); err != nil {
		return TaskView{}, err
	} else if ok {
		view.ChangeSummary = &change
	}
	if todo, ok, err := loadTodoSummary(paths.StateDir); err != nil {
		return TaskView{}, err
	} else if ok {
		view.TodoSummary = &todo
	}
	if summary, err := verify.LoadFeedbackSummary(root); err == nil {
		if taskFeedback, ok := verify.CurrentTaskFeedback(summary, task.TaskID); ok {
			copy := taskFeedback
			view.OuterLoopMemory = &copy
		}
	}
	logPath := view.Task.TmuxLogPath
	if logPath == "" && view.Tmux != nil {
		logPath = view.Tmux.LogPath
	}
	if logPath != "" {
		view.LogPreview = tailPreview(logPath, 20)
	}
	view.Release = deriveReleaseReadiness(view)
	return view, nil
}

func MustTask(root, taskID string) (adapter.Task, error) {
	task, err := adapter.LoadTask(root, taskID)
	if err != nil {
		return adapter.Task{}, err
	}
	if task.TaskID == "" {
		return adapter.Task{}, errors.New("task not found")
	}
	return task, nil
}

func ReleaseStatus(root string) (ReleaseBoard, error) {
	tasks, err := ListTasks(root)
	if err != nil {
		return ReleaseBoard{}, err
	}
	board := ReleaseBoard{
		Items: make([]ReleaseBoardItem, 0, len(tasks)),
	}
	for _, task := range tasks {
		view, err := Task(root, task.TaskID)
		if err != nil {
			return ReleaseBoard{}, err
		}
		item := ReleaseBoardItem{
			TaskID:          view.Task.TaskID,
			ThreadKey:       view.Task.ThreadKey,
			Title:           view.Task.Title,
			TaskStatus:      view.Task.Status,
			ReleaseStatus:   view.Release.Status,
			Ready:           view.Release.Ready,
			SafeToArchive:   view.Release.SafeToArchive,
			NextAction:      view.Release.NextAction,
			BlockingReasons: slicesClone(view.Release.BlockingReasons),
		}
		board.Items = append(board.Items, item)
		switch view.Release.Status {
		case "release_ready":
			board.ReadyCount++
		case "needs_review":
			board.NeedsReviewCount++
		case "needs_replan":
			board.NeedsReplanCount++
		case "awaiting_gate":
			board.AwaitingGateCount++
		case "blocked":
			board.BlockedCount++
		case "more_slices_remaining":
			board.RemainingSliceCount++
		}
	}
	sort.SliceStable(board.Items, func(i, j int) bool {
		if board.Items[i].Ready != board.Items[j].Ready {
			return board.Items[i].Ready
		}
		if board.Items[i].ReleaseStatus != board.Items[j].ReleaseStatus {
			return board.Items[i].ReleaseStatus < board.Items[j].ReleaseStatus
		}
		return board.Items[i].TaskID < board.Items[j].TaskID
	})
	return board, nil
}

func ReleaseSnapshotStatus(root string) (ReleaseSnapshot, error) {
	paths, err := adapter.Resolve(root)
	if err != nil {
		return ReleaseSnapshot{}, err
	}
	board, err := ReleaseStatus(root)
	if err != nil {
		return ReleaseSnapshot{}, err
	}
	version, dirty := gitVersion(root)
	changelogVersion := changelogHeadVersion(filepath.Join(root, "CHANGELOG.md"))
	snapshot := buildReleaseSnapshot(paths.Root, version, changelogVersion, dirty, board)
	if _, err := state.WriteSnapshot(paths.ReleaseSnapshotPath, &snapshot, "harness-query", snapshot.Revision); err != nil {
		return ReleaseSnapshot{}, err
	}
	if err := state.LoadJSON(paths.ReleaseSnapshotPath, &snapshot); err != nil {
		return ReleaseSnapshot{}, err
	}
	return snapshot, nil
}

type dispatchTicketView struct {
	ResumeStrategy     string                              `json:"resumeStrategy"`
	SessionID          string                              `json:"sessionId"`
	PromptStages       []string                            `json:"promptStages"`
	PlanningTracePath  string                              `json:"planningTracePath"`
	ConstraintPath     string                              `json:"constraintPath"`
	AcceptedPacketPath string                              `json:"acceptedPacketPath"`
	TaskContractPath   string                              `json:"taskContractPath"`
	RequestContextPath string                              `json:"requestContextPath"`
	RuntimeContextPath string                              `json:"runtimeContextPath"`
	ContextLayersPath  string                              `json:"contextLayersPath"`
	SharedFlowPath     string                              `json:"sharedFlowPath"`
	SliceContextPath   string                              `json:"sliceContextPath"`
	TaskGraphPath      string                              `json:"taskGraphPath"`
	VerifySkeletonPath string                              `json:"verifySkeletonPath"`
	CloseoutPath       string                              `json:"closeoutPath"`
	HandoffPath        string                              `json:"handoffPath"`
	TakeoverPath       string                              `json:"takeoverPath"`
	ExecutionSliceID   string                              `json:"executionSliceId"`
	RuntimeRefs        map[string]string                   `json:"runtimeRefs"`
	Methodology        orchestration.MethodologyContract   `json:"methodology"`
	JudgeDecision      orchestration.JudgeDecision         `json:"judgeDecision"`
	ExecutionLoop      orchestration.ExecutionLoopContract `json:"executionLoop"`
	ConstraintSystem   orchestration.ConstraintSystem      `json:"constraintSystem"`
	PacketSynthesis    orchestration.PacketSynthesisLoop   `json:"packetSynthesis"`
	PlannerCandidates  []orchestration.PlannerCandidate    `json:"plannerCandidates"`
}

func loadPlanningView(stateDir, taskID string) (*PlanningView, error) {
	ticketPath := filepath.Join(stateDir, "dispatch-ticket-"+taskID+".json")
	payload, err := os.ReadFile(ticketPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var ticket dispatchTicketView
	if err := json.Unmarshal(payload, &ticket); err != nil {
		return nil, err
	}
	view := &PlanningView{
		DispatchTicketPath: ticketPath,
		PromptPath:         ticket.RuntimeRefs["promptPath"],
		PlanningTracePath:  ticket.PlanningTracePath,
		ConstraintPath:     ticket.ConstraintPath,
		AcceptedPacketPath: ticket.AcceptedPacketPath,
		TaskContractPath:   ticket.TaskContractPath,
		RequestContextPath: ticket.RequestContextPath,
		RuntimeContextPath: ticket.RuntimeContextPath,
		ContextLayersPath:  ticket.ContextLayersPath,
		SharedFlowPath:     ticket.SharedFlowPath,
		SliceContextPath:   ticket.SliceContextPath,
		TaskGraphPath:      ticket.TaskGraphPath,
		VerifySkeletonPath: ticket.VerifySkeletonPath,
		CloseoutPath:       ticket.CloseoutPath,
		HandoffPath:        ticket.HandoffPath,
		TakeoverPath:       ticket.TakeoverPath,
		ExecutionSliceID:   ticket.ExecutionSliceID,
		ResumeStrategy:     ticket.ResumeStrategy,
		SessionID:          ticket.SessionID,
		PromptStages:       ticket.PromptStages,
		Methodology:        ticket.Methodology,
		JudgeDecision:      ticket.JudgeDecision,
		ExecutionLoop:      ticket.ExecutionLoop,
		ConstraintSystem:   ticket.ConstraintSystem,
		PacketSynthesis:    ticket.PacketSynthesis,
		PlannerCandidates:  append([]orchestration.PlannerCandidate(nil), ticket.PlannerCandidates...),
		ActiveSkills:       append([]string(nil), ticket.ExecutionLoop.ActiveSkills...),
		SkillHints:         append([]string(nil), ticket.ExecutionLoop.SkillHints...),
	}
	if view.PlanningTracePath == "" {
		view.PlanningTracePath = ticket.RuntimeRefs["planningTrace"]
	}
	if view.ConstraintPath == "" {
		view.ConstraintPath = ticket.RuntimeRefs["constraints"]
	}
	if view.AcceptedPacketPath == "" {
		view.AcceptedPacketPath = ticket.RuntimeRefs["acceptedPacket"]
	}
	if view.TaskContractPath == "" {
		view.TaskContractPath = ticket.RuntimeRefs["taskContract"]
	}
	if view.RequestContextPath == "" {
		view.RequestContextPath = ticket.RuntimeRefs["requestContext"]
	}
	if view.RuntimeContextPath == "" {
		view.RuntimeContextPath = ticket.RuntimeRefs["runtimeContext"]
	}
	if view.ContextLayersPath == "" {
		view.ContextLayersPath = ticket.RuntimeRefs["contextLayers"]
	}
	if view.SharedFlowPath == "" {
		view.SharedFlowPath = ticket.RuntimeRefs["sharedFlow"]
	}
	if view.SliceContextPath == "" {
		view.SliceContextPath = ticket.RuntimeRefs["sliceContext"]
	}
	if view.TaskGraphPath == "" {
		view.TaskGraphPath = ticket.RuntimeRefs["taskGraph"]
	}
	if view.VerifySkeletonPath == "" {
		view.VerifySkeletonPath = ticket.RuntimeRefs["verifySkeleton"]
	}
	if view.CloseoutPath == "" {
		view.CloseoutPath = ticket.RuntimeRefs["closeoutSkeleton"]
	}
	if view.HandoffPath == "" {
		view.HandoffPath = ticket.RuntimeRefs["handoffContract"]
	}
	if view.TakeoverPath == "" {
		view.TakeoverPath = ticket.RuntimeRefs["takeover"]
	}
	if view.Methodology.Mode == "" {
		root := filepath.Dir(filepath.Dir(stateDir))
		view.Methodology = orchestration.DefaultMethodologyContract(root, nil)
		if view.JudgeDecision.JudgeID == "" {
			view.JudgeDecision = orchestration.DefaultJudgeDecision(view.PacketSynthesis, view.Methodology, nil)
		}
	}
	if view.JudgeDecision.JudgeID == "" {
		view.JudgeDecision = orchestration.DefaultJudgeDecision(view.PacketSynthesis, view.Methodology, nil)
	}
	if view.ExecutionLoop.Mode == "" {
		root := filepath.Dir(filepath.Dir(stateDir))
		view.ExecutionLoop = orchestration.DefaultExecutionLoopContract(root, nil)
	}
	if view.ConstraintSystem.Mode == "" {
		root := filepath.Dir(filepath.Dir(stateDir))
		view.ConstraintSystem = orchestration.DefaultConstraintSystem(root, nil)
	}
	if view.PlanningTracePath != "" {
		view.TracePreview = headPreview(view.PlanningTracePath, 18)
	}
	return view, nil
}

func loadAcceptedPacket(root, taskID string) (orchestration.AcceptedPacket, bool, error) {
	path := orchestration.AcceptedPacketPath(root, taskID)
	packet, err := orchestration.LoadAcceptedPacket(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return orchestration.AcceptedPacket{}, false, nil
		}
		return orchestration.AcceptedPacket{}, false, err
	}
	return packet, true, nil
}

func loadTaskContract(root, taskID, dispatchID string) (orchestration.TaskContract, bool, error) {
	path := orchestration.TaskContractPath(filepath.Join(root, ".harness", "artifacts", taskID, dispatchID))
	contract, err := orchestration.LoadTaskContract(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return orchestration.TaskContract{}, false, nil
		}
		return orchestration.TaskContract{}, false, err
	}
	return contract, true, nil
}

func loadContextLayers(planning *PlanningView, contract *orchestration.TaskContract) (orchestration.ContextLayers, bool, error) {
	var payload orchestration.ContextLayers
	return payload, loadJSONIfPresent(firstNonEmptyPlanningPath(contractPath(contract, func(c *orchestration.TaskContract) string { return c.ContextLayersPath }), planningPath(planning, func(p *PlanningView) string { return p.ContextLayersPath })), &payload)
}

func loadSharedFlowContext(planning *PlanningView, contract *orchestration.TaskContract) (orchestration.SharedFlowContext, bool, error) {
	var payload orchestration.SharedFlowContext
	return payload, loadJSONIfPresent(firstNonEmptyPlanningPath(contractPath(contract, func(c *orchestration.TaskContract) string { return c.SharedFlowContextPath }), planningPath(planning, func(p *PlanningView) string { return p.SharedFlowPath })), &payload)
}

func loadSliceContext(planning *PlanningView, contract *orchestration.TaskContract) (orchestration.SliceLocalContext, bool, error) {
	var payload orchestration.SliceLocalContext
	return payload, loadJSONIfPresent(firstNonEmptyPlanningPath(contractPath(contract, func(c *orchestration.TaskContract) string { return c.SliceContextPath }), planningPath(planning, func(p *PlanningView) string { return p.SliceContextPath })), &payload)
}

func loadVerifySkeleton(planning *PlanningView, contract *orchestration.TaskContract) (orchestration.VerifySkeleton, bool, error) {
	var payload orchestration.VerifySkeleton
	return payload, loadJSONIfPresent(firstNonEmptyPlanningPath(contractPath(contract, func(c *orchestration.TaskContract) string { return c.VerifySkeletonPath }), planningPath(planning, func(p *PlanningView) string { return p.VerifySkeletonPath })), &payload)
}

func loadCloseoutSkeleton(planning *PlanningView, contract *orchestration.TaskContract) (orchestration.CloseoutSkeleton, bool, error) {
	var payload orchestration.CloseoutSkeleton
	return payload, loadJSONIfPresent(firstNonEmptyPlanningPath(contractPath(contract, func(c *orchestration.TaskContract) string { return c.CloseoutSkeletonPath }), planningPath(planning, func(p *PlanningView) string { return p.CloseoutPath })), &payload)
}

func loadHandoffContract(planning *PlanningView, contract *orchestration.TaskContract) (orchestration.HandoffContract, bool, error) {
	var payload orchestration.HandoffContract
	return payload, loadJSONIfPresent(firstNonEmptyPlanningPath(contractPath(contract, func(c *orchestration.TaskContract) string { return c.HandoffContractPath }), planningPath(planning, func(p *PlanningView) string { return p.HandoffPath })), &payload)
}

func loadContinuation(planning *PlanningView, contract *orchestration.TaskContract) (orchestration.ContinuationProtocol, bool, error) {
	var payload orchestration.ContinuationProtocol
	return payload, loadJSONIfPresent(firstNonEmptyPlanningPath(contractPath(contract, func(c *orchestration.TaskContract) string { return c.TakeoverPath }), planningPath(planning, func(p *PlanningView) string { return p.TakeoverPath })), &payload)
}

func loadAssessment(root, taskID, dispatchID string) (verify.Assessment, bool, error) {
	path := verify.AssessmentPath(root, taskID, dispatchID)
	assessment, err := verify.LoadAssessment(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return verify.Assessment{}, false, nil
		}
		return verify.Assessment{}, false, err
	}
	return assessment, true, nil
}

func loadJSONIfPresent(path string, target any) (bool, error) {
	if strings.TrimSpace(path) == "" {
		return false, nil
	}
	ok, err := state.LoadJSONIfExists(path, target)
	if err != nil {
		return false, err
	}
	return ok, nil
}

func contractPath(contract *orchestration.TaskContract, selectPath func(*orchestration.TaskContract) string) string {
	if contract == nil {
		return ""
	}
	return selectPath(contract)
}

func planningPath(planning *PlanningView, selectPath func(*PlanningView) string) string {
	if planning == nil {
		return ""
	}
	return selectPath(planning)
}

func firstNonEmptyPlanningPath(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func loadPacketProgress(root, taskID string) (orchestration.PacketProgress, bool, error) {
	path := orchestration.PacketProgressPath(root, taskID)
	progress, err := orchestration.LoadPacketProgress(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return orchestration.PacketProgress{}, false, nil
		}
		return orchestration.PacketProgress{}, false, err
	}
	return progress, true, nil
}

func loadIntakeSummary(stateDir string) (runtime.IntakeSummary, bool, error) {
	path := filepath.Join(stateDir, "intake-summary.json")
	var summary runtime.IntakeSummary
	ok, err := state.LoadJSONIfExists(path, &summary)
	if err != nil {
		return runtime.IntakeSummary{}, false, err
	}
	return summary, ok, nil
}

func loadThreadEntry(stateDir, threadKey string) (runtime.ThreadEntry, bool, error) {
	path := filepath.Join(stateDir, "thread-state.json")
	var threadState runtime.ThreadState
	ok, err := state.LoadJSONIfExists(path, &threadState)
	if err != nil {
		return runtime.ThreadEntry{}, false, err
	}
	if !ok || len(threadState.Threads) == 0 {
		return runtime.ThreadEntry{}, false, nil
	}
	thread, exists := threadState.Threads[threadKey]
	if !exists {
		return runtime.ThreadEntry{}, false, nil
	}
	return thread, true, nil
}

func loadChangeSummary(stateDir string) (runtime.ChangeSummary, bool, error) {
	path := filepath.Join(stateDir, "change-summary.json")
	var summary runtime.ChangeSummary
	ok, err := state.LoadJSONIfExists(path, &summary)
	if err != nil {
		return runtime.ChangeSummary{}, false, err
	}
	return summary, ok, nil
}

func loadTodoSummary(stateDir string) (runtime.TodoSummary, bool, error) {
	path := filepath.Join(stateDir, "todo-summary.json")
	var summary runtime.TodoSummary
	ok, err := state.LoadJSONIfExists(path, &summary)
	if err != nil {
		return runtime.TodoSummary{}, false, err
	}
	return summary, ok, nil
}

func loadLatestRequestForTask(paths adapter.Paths, taskID string) (runtime.RequestRecord, bool, error) {
	index, ok, err := loadRequestIndex(paths.RequestIndexPath)
	if err != nil {
		return runtime.RequestRecord{}, false, err
	}
	if ok {
		if requestID := index.LatestRequestByTaskID[taskID]; requestID != "" {
			if record, exists := index.RequestsByID[requestID]; exists {
				return record, true, nil
			}
		}
	}
	return loadLatestRequestForTaskFromQueue(paths.QueuePath, taskID)
}

func loadLatestRequestForTaskFromQueue(queuePath, taskID string) (runtime.RequestRecord, bool, error) {
	payload, err := os.ReadFile(queuePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return runtime.RequestRecord{}, false, nil
		}
		return runtime.RequestRecord{}, false, err
	}
	lines := normalizedLines(string(payload))
	for index := len(lines) - 1; index >= 0; index-- {
		var record runtime.RequestRecord
		if err := json.Unmarshal([]byte(lines[index]), &record); err != nil {
			continue
		}
		if record.TaskID != taskID {
			continue
		}
		return record, true, nil
	}
	return runtime.RequestRecord{}, false, nil
}

func loadTaskCompletionGate(paths adapter.Paths, taskID string) (verify.CompletionGate, bool, error) {
	for _, path := range []string{paths.CompletionGateTaskPath(taskID), paths.CompletionGatePath} {
		var gate verify.CompletionGate
		ok, err := state.LoadJSONIfExists(path, &gate)
		if err != nil {
			return verify.CompletionGate{}, false, err
		}
		if ok && gate.TaskID == taskID {
			return gate, true, nil
		}
	}
	return verify.CompletionGate{}, false, nil
}

func loadTaskGuardState(paths adapter.Paths, taskID string) (verify.GuardState, bool, error) {
	for _, path := range []string{paths.GuardStateTaskPath(taskID), paths.GuardStatePath} {
		var guard verify.GuardState
		ok, err := state.LoadJSONIfExists(path, &guard)
		if err != nil {
			return verify.GuardState{}, false, err
		}
		if ok && guard.TaskID == taskID {
			return guard, true, nil
		}
	}
	return verify.GuardState{}, false, nil
}

func loadRequestIndex(path string) (runtime.RequestIndex, bool, error) {
	var index runtime.RequestIndex
	ok, err := state.LoadJSONIfExists(path, &index)
	if err != nil {
		return runtime.RequestIndex{}, false, err
	}
	if index.RequestsByID == nil {
		index.RequestsByID = map[string]runtime.RequestRecord{}
	}
	if index.LatestRequestByTaskID == nil {
		index.LatestRequestByTaskID = map[string]string{}
	}
	return index, ok, nil
}

func remainingExecutionSlices(packet orchestration.AcceptedPacket, progress *orchestration.PacketProgress) []string {
	tasks := packet.ExecutionTasks
	if len(tasks) == 0 {
		return nil
	}
	completed := map[string]struct{}{}
	if progress != nil && progress.PlanEpoch == packet.PlanEpoch &&
		(strings.TrimSpace(progress.AcceptedPacketID) == "" || progress.AcceptedPacketID == packet.PacketID) {
		for _, id := range progress.CompletedSliceIDs {
			completed[id] = struct{}{}
		}
	}
	remaining := make([]string, 0, len(tasks))
	for _, task := range tasks {
		if _, ok := completed[task.ID]; ok {
			continue
		}
		remaining = append(remaining, task.ID)
	}
	return remaining
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func deriveReleaseReadiness(view TaskView) ReleaseReadiness {
	readiness := ReleaseReadiness{
		Status:     "not_ready",
		NextAction: "verify",
	}
	if view.Guard != nil {
		readiness.SafeToArchive = view.Guard.SafeToArchive
		readiness.BlockingReasons = append(readiness.BlockingReasons, view.Guard.Blockers...)
	}
	if len(view.RemainingSlices) > 0 {
		readiness.BlockingReasons = append(readiness.BlockingReasons, "remaining execution slices: "+strings.Join(view.RemainingSlices, ", "))
	}
	switch {
	case view.Completion != nil && view.Completion.Retired:
		readiness.Status = "archived"
		readiness.NextAction = ""
		return readiness
	case view.Guard != nil && view.Guard.SafeToArchive:
		readiness.Status = "release_ready"
		readiness.Ready = true
		readiness.SafeToArchive = true
		readiness.NextAction = "archive"
		return readiness
	case view.Completion != nil && view.Completion.Status == "needs_review":
		readiness.Status = "needs_review"
		readiness.NextAction = firstNonEmpty(view.Completion.RecommendedNextAction, "review")
		return readiness
	case len(view.RemainingSlices) > 0:
		readiness.Status = "more_slices_remaining"
		readiness.NextAction = "replan"
		return readiness
	case view.Completion != nil && view.Completion.Status == "needs_replan":
		readiness.Status = "needs_replan"
		readiness.NextAction = firstNonEmpty(view.Completion.RecommendedNextAction, "replan")
		return readiness
	case view.Task.Status == "needs_replan":
		readiness.Status = "needs_replan"
		readiness.NextAction = "replan"
		return readiness
	case view.Completion != nil && view.Completion.Status == "blocked":
		readiness.Status = "blocked"
		readiness.NextAction = firstNonEmpty(view.Completion.RecommendedNextAction, "unblock")
		return readiness
	case view.Task.Status == "blocked":
		readiness.Status = "blocked"
		readiness.NextAction = "unblock"
		return readiness
	case view.Task.VerificationStatus == "passed" && (view.Completion == nil || !view.Completion.Satisfied):
		readiness.Status = "awaiting_gate"
		readiness.NextAction = "satisfy_gate"
		return readiness
	case view.Task.VerificationStatus == "":
		readiness.Status = "verification_pending"
		readiness.NextAction = "verify"
		return readiness
	default:
		readiness.Status = "in_progress"
		readiness.NextAction = "continue"
		return readiness
	}
}

func slicesClone(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, len(values))
	copy(out, values)
	return out
}

func buildReleaseSnapshot(root, version, changelogVersion string, dirty bool, board ReleaseBoard) ReleaseSnapshot {
	blocking := make([]string, 0)
	if dirty {
		blocking = append(blocking, "git worktree has uncommitted changes")
	}
	if board.BlockedCount > 0 {
		blocking = append(blocking, "blocked tasks remain on the release board")
	}
	if board.NeedsReplanCount > 0 {
		blocking = append(blocking, "tasks still need replan")
	}
	if board.NeedsReviewCount > 0 {
		blocking = append(blocking, "tasks still need review")
	}
	if board.AwaitingGateCount > 0 {
		blocking = append(blocking, "some tasks passed verification but still await completion gate satisfaction")
	}
	if board.RemainingSliceCount > 0 {
		blocking = append(blocking, "some accepted packets still have remaining execution slices")
	}
	ready := len(blocking) == 0 && board.ReadyCount > 0
	return ReleaseSnapshot{
		GeneratedAt:      state.NowUTC(),
		Root:             root,
		Version:          version,
		ChangelogVersion: changelogVersion,
		Dirty:            dirty,
		Ready:            ready,
		BlockingReasons:  blocking,
		ReleaseBoard:     board,
	}
}

func gitVersion(root string) (string, bool) {
	cmd := exec.Command("git", "-C", root, "describe", "--tags", "--always", "--dirty")
	output, err := cmd.Output()
	if err != nil {
		return "unversioned", false
	}
	version := strings.TrimSpace(string(output))
	return version, strings.Contains(version, "-dirty")
}

func changelogHeadVersion(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "## ") {
			continue
		}
		line = strings.TrimPrefix(line, "## ")
		parts := strings.SplitN(line, " - ", 2)
		if len(parts) == 0 {
			return ""
		}
		return strings.TrimSpace(parts[0])
	}
	return ""
}

func headPreview(path string, limit int) []string {
	payload, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	lines := normalizedLines(string(payload))
	if len(lines) > limit {
		lines = lines[:limit]
	}
	return lines
}

func tailPreview(path string, limit int) []string {
	payload, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	lines := normalizedLines(string(payload))
	if len(lines) > limit {
		lines = lines[len(lines)-limit:]
	}
	return lines
}

func normalizedLines(text string) []string {
	raw := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	lines := make([]string, 0, len(raw))
	for _, line := range raw {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lines = append(lines, trimmed)
	}
	return lines
}
