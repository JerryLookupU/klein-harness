package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"klein-harness/internal/bootstrap"
	"klein-harness/internal/cli"
	"klein-harness/internal/query"
	"klein-harness/internal/runtime"
	"klein-harness/internal/tmux"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	var err error
	switch os.Args[1] {
	case "init":
		err = runInit(os.Args[2:])
	case "submit":
		err = runSubmit(os.Args[2:])
	case "tasks":
		err = runTasks(os.Args[2:])
	case "task":
		err = runTask(os.Args[2:])
	case "control":
		err = runControl(os.Args[2:])
	case "daemon":
		err = runDaemon(os.Args[2:])
	case "-h", "--help", "help":
		usage()
		return
	default:
		err = fmt.Errorf("unknown subcommand: %s", os.Args[1])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: harness <init|submit|tasks|task|control|daemon> [args...]")
}

func runInit(args []string) error {
	root, _, err := cli.ResolveRootArg(args)
	if err != nil {
		return err
	}
	paths, err := bootstrap.Init(root)
	if err != nil {
		return err
	}
	return writeJSON(map[string]any{
		"root":       paths.Root,
		"harnessDir": paths.HarnessDir,
		"status":     "initialized",
	})
}

func runSubmit(args []string) error {
	root, rest, err := cli.ResolveRootArg(args)
	if err != nil {
		return err
	}
	fs := flag.NewFlagSet("submit", flag.ContinueOnError)
	goal := fs.String("goal", "", "task goal")
	kind := fs.String("kind", "", "task kind")
	var contexts stringList
	fs.Var(&contexts, "context", "extra task context")
	if err := fs.Parse(rest); err != nil {
		return err
	}
	if strings.TrimSpace(*goal) == "" {
		return errors.New("missing --goal")
	}
	result, err := runtime.Submit(runtime.SubmitRequest{
		Root:     root,
		Goal:     *goal,
		Kind:     *kind,
		Contexts: contexts,
	})
	if err != nil {
		return err
	}
	return writeJSON(result)
}

func runTasks(args []string) error {
	root, _, err := cli.ResolveRootArg(args)
	if err != nil {
		return err
	}
	tasks, err := query.ListTasks(root)
	if err != nil {
		return err
	}
	return writeJSON(map[string]any{"tasks": tasks})
}

func runTask(args []string) error {
	root, rest, err := cli.ResolveRootArg(args)
	if err != nil {
		return err
	}
	if len(rest) == 0 {
		return errors.New("usage: harness task <ROOT> <TASK_ID>")
	}
	view, err := query.Task(root, rest[0])
	if err != nil {
		return err
	}
	return writeJSON(view)
}

func runControl(args []string) error {
	root, rest, err := cli.ResolveRootArg(args)
	if err != nil {
		return err
	}
	if len(rest) < 3 || rest[0] != "task" {
		return errors.New("usage: harness control <ROOT> task <TASK_ID> <status|attach|restart-from-stage|stop|archive>")
	}
	taskID := rest[1]
	action := rest[2]
	switch action {
	case "status":
		view, err := query.Task(root, taskID)
		if err != nil {
			return err
		}
		return writeJSON(view)
	case "attach":
		view, err := query.Task(root, taskID)
		if err != nil {
			return err
		}
		sessionName := view.Task.TmuxSession
		if sessionName == "" && view.Tmux != nil {
			sessionName = view.Tmux.SessionName
		}
		if sessionName == "" {
			return errors.New("task has no tmux session")
		}
		if interactiveTTY() {
			return tmux.AttachSession(sessionName)
		}
		return writeJSON(map[string]any{
			"taskId":        taskID,
			"sessionName":   sessionName,
			"attachCommand": view.AttachCommand,
		})
	case "restart-from-stage":
		if len(rest) < 4 {
			return errors.New("usage: harness control <ROOT> task <TASK_ID> restart-from-stage <queued|worktree_prepared|merge_queued>")
		}
		task, err := runtime.RestartFromStage(root, taskID, rest[3], "")
		if err != nil {
			return err
		}
		return writeJSON(task)
	case "stop":
		task, err := runtime.StopTask(root, taskID, "")
		if err != nil {
			return err
		}
		return writeJSON(task)
	case "archive":
		task, err := runtime.ArchiveTask(root, taskID, "")
		if err != nil {
			return err
		}
		return writeJSON(task)
	default:
		return fmt.Errorf("unknown control action: %s", action)
	}
}

func runDaemon(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: harness daemon <run-once|loop> <ROOT> [args...]")
	}
	switch args[0] {
	case "run-once":
		return runDaemonRunOnce(args[1:])
	case "loop":
		return runDaemonLoop(args[1:])
	default:
		return fmt.Errorf("unknown daemon action: %s", args[0])
	}
}

func runDaemonRunOnce(args []string) error {
	root, rest, err := cli.ResolveRootArg(args)
	if err != nil {
		return err
	}
	fs := flag.NewFlagSet("run-once", flag.ContinueOnError)
	workerID := fs.String("worker-id", "harness-daemon", "worker id")
	model := fs.String("model", "", "model override")
	approval := fs.String("approval-policy", "", "approval policy")
	sandbox := fs.String("sandbox-mode", "", "sandbox mode")
	skipGitRepoCheck := fs.Bool("skip-git-repo-check", false, "allow running outside a git repo")
	if err := fs.Parse(rest); err != nil {
		return err
	}
	result, err := runtime.RunOnce(root, runtime.RunOptions{
		WorkerID:         *workerID,
		Model:            *model,
		ApprovalPolicy:   *approval,
		SandboxMode:      *sandbox,
		SkipGitRepoCheck: *skipGitRepoCheck,
	})
	if err != nil {
		return err
	}
	return writeJSON(result)
}

func runDaemonLoop(args []string) error {
	root, rest, err := cli.ResolveRootArg(args)
	if err != nil {
		return err
	}
	fs := flag.NewFlagSet("loop", flag.ContinueOnError)
	interval := fs.Duration("interval", 30*time.Second, "loop interval")
	workerID := fs.String("worker-id", "harness-daemon", "worker id")
	model := fs.String("model", "", "model override")
	approval := fs.String("approval-policy", "", "approval policy")
	sandbox := fs.String("sandbox-mode", "", "sandbox mode")
	skipGitRepoCheck := fs.Bool("skip-git-repo-check", false, "allow running outside a git repo")
	if err := fs.Parse(rest); err != nil {
		return err
	}
	return runtime.Loop(root, *interval, runtime.RunOptions{
		WorkerID:         *workerID,
		Model:            *model,
		ApprovalPolicy:   *approval,
		SandboxMode:      *sandbox,
		SkipGitRepoCheck: *skipGitRepoCheck,
	})
}

func interactiveTTY() bool {
	info, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

func writeJSON(value any) error {
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(append(payload, '\n'))
	return err
}

type stringList []string

func (s *stringList) String() string {
	return strings.Join(*s, ",")
}

func (s *stringList) Set(value string) error {
	*s = append(*s, value)
	return nil
}
