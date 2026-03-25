package query

import (
	"errors"
	"sort"

	"klein-harness/internal/adapter"
	"klein-harness/internal/dispatch"
	"klein-harness/internal/lease"
	"klein-harness/internal/state"
	"klein-harness/internal/tmux"
	"klein-harness/internal/verify"
)

type TaskView struct {
	Task          adapter.Task           `json:"task"`
	Dispatch      *dispatch.Ticket       `json:"dispatch,omitempty"`
	Lease         *lease.Record          `json:"lease,omitempty"`
	Completion    *verify.CompletionGate `json:"completionGate,omitempty"`
	Guard         *verify.GuardState     `json:"guardState,omitempty"`
	Tmux          *tmux.SessionState     `json:"tmux,omitempty"`
	AttachCommand string                 `json:"attachCommand,omitempty"`
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
	if task.TmuxSession != "" {
		summary, err := tmux.LoadSummary(root)
		if err == nil {
			if session, ok := summary.Sessions[task.TmuxSession]; ok {
				copy := session
				view.Tmux = &copy
				view.AttachCommand = session.AttachCommand
			}
		}
		if view.AttachCommand == "" {
			view.AttachCommand = tmux.AttachCommand(task.TmuxSession)
		}
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
