package query

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"klein-harness/internal/adapter"
	"klein-harness/internal/dispatch"
	"klein-harness/internal/lease"
	"klein-harness/internal/orchestration"
	"klein-harness/internal/state"
	"klein-harness/internal/tmux"
	"klein-harness/internal/verify"
)

type PlanningView struct {
	DispatchTicketPath string                              `json:"dispatchTicketPath,omitempty"`
	PromptPath         string                              `json:"promptPath,omitempty"`
	PlanningTracePath  string                              `json:"planningTracePath,omitempty"`
	ResumeStrategy     string                              `json:"resumeStrategy,omitempty"`
	SessionID          string                              `json:"sessionId,omitempty"`
	PromptStages       []string                            `json:"promptStages,omitempty"`
	Methodology        orchestration.MethodologyContract   `json:"methodology"`
	JudgeDecision      orchestration.JudgeDecision         `json:"judgeDecision"`
	ExecutionLoop      orchestration.ExecutionLoopContract `json:"executionLoop"`
	ConstraintSystem   orchestration.ConstraintSystem      `json:"constraintSystem"`
	PacketSynthesis    orchestration.PacketSynthesisLoop   `json:"packetSynthesis"`
	TracePreview       []string                            `json:"tracePreview,omitempty"`
}

type TaskView struct {
	Task            adapter.Task                `json:"task"`
	Dispatch        *dispatch.Ticket            `json:"dispatch,omitempty"`
	Lease           *lease.Record               `json:"lease,omitempty"`
	Completion      *verify.CompletionGate      `json:"completionGate,omitempty"`
	Guard           *verify.GuardState          `json:"guardState,omitempty"`
	Tmux            *tmux.SessionState          `json:"tmux,omitempty"`
	Planning        *PlanningView               `json:"planning,omitempty"`
	OuterLoopMemory *verify.TaskFeedbackSummary `json:"outerLoopMemory,omitempty"`
	AttachCommand   string                      `json:"attachCommand,omitempty"`
	LogPreview      []string                    `json:"logPreview,omitempty"`
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
	var gate verify.CompletionGate
	if ok, err := state.LoadJSONIfExists(paths.CompletionGatePath, &gate); err != nil {
		return TaskView{}, err
	} else if ok && gate.TaskID == task.TaskID {
		view.Completion = &gate
	}
	var guard verify.GuardState
	if ok, err := state.LoadJSONIfExists(paths.GuardStatePath, &guard); err != nil {
		return TaskView{}, err
	} else if ok && guard.TaskID == task.TaskID {
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

type dispatchTicketView struct {
	ResumeStrategy    string                              `json:"resumeStrategy"`
	SessionID         string                              `json:"sessionId"`
	PromptStages      []string                            `json:"promptStages"`
	PlanningTracePath string                              `json:"planningTracePath"`
	RuntimeRefs       map[string]string                   `json:"runtimeRefs"`
	Methodology       orchestration.MethodologyContract   `json:"methodology"`
	JudgeDecision     orchestration.JudgeDecision         `json:"judgeDecision"`
	ExecutionLoop     orchestration.ExecutionLoopContract `json:"executionLoop"`
	ConstraintSystem  orchestration.ConstraintSystem      `json:"constraintSystem"`
	PacketSynthesis   orchestration.PacketSynthesisLoop   `json:"packetSynthesis"`
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
		ResumeStrategy:     ticket.ResumeStrategy,
		SessionID:          ticket.SessionID,
		PromptStages:       ticket.PromptStages,
		Methodology:        ticket.Methodology,
		JudgeDecision:      ticket.JudgeDecision,
		ExecutionLoop:      ticket.ExecutionLoop,
		ConstraintSystem:   ticket.ConstraintSystem,
		PacketSynthesis:    ticket.PacketSynthesis,
	}
	if view.PlanningTracePath == "" {
		view.PlanningTracePath = ticket.RuntimeRefs["planningTrace"]
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
