package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"klein-harness/internal/auth"
	"klein-harness/internal/codexexec"
	"klein-harness/internal/instructions"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		return
	}
	var err error
	switch os.Args[1] {
	case "login":
		err = runLogin(os.Args[2:])
	case "instructions":
		err = runInstructions(os.Args[2:])
	case "exec":
		err = runExec(os.Args[2:])
	case "resume":
		err = runResume(os.Args[2:])
	case "review", "sandbox", "features":
		err = fmt.Errorf("%s is not implemented yet; see docs/dev/openai-codex-parity-blueprint.md", os.Args[1])
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
	fmt.Fprintln(os.Stderr, "usage: kh-codex <login|instructions|exec|resume|review|sandbox|features> [args...]")
}

func runLogin(args []string) error {
	subcommand := ""
	if len(args) > 0 && args[0] == "status" {
		subcommand = "status"
		args = args[1:]
	}
	fs := flag.NewFlagSet("login", flag.ContinueOnError)
	withAPIKey := fs.String("with-api-key", "", "save API key to CODEX_HOME/auth.json")
	homeDir := fs.String("codex-home", "", "override codex home directory")
	jsonOutput := fs.Bool("json", false, "print JSON status")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *withAPIKey != "" {
		path, err := auth.SaveAPIKey(*homeDir, *withAPIKey)
		if err != nil {
			return err
		}
		return writeOutput(*jsonOutput, map[string]any{
			"ok":       true,
			"mode":     "api_key",
			"savedTo":  path,
			"provider": "openai",
		})
	}
	if subcommand == "" && len(fs.Args()) > 0 {
		return fmt.Errorf("unsupported login subcommand: %s", fs.Args()[0])
	}
	status, err := auth.LoadStatus(*homeDir)
	if err != nil {
		return err
	}
	return writeOutput(*jsonOutput, status)
}

func runInstructions(args []string) error {
	fs := flag.NewFlagSet("instructions", flag.ContinueOnError)
	homeDir := fs.String("codex-home", "", "override codex home directory")
	jsonOutput := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	startDir := "."
	if len(fs.Args()) > 0 {
		startDir = fs.Args()[0]
	}
	files, err := instructions.Discover(startDir, *homeDir)
	if err != nil {
		return err
	}
	return writeOutput(*jsonOutput, map[string]any{"files": files})
}

func runExec(args []string) error {
	resumeMode := false
	if len(args) > 0 && args[0] == "resume" {
		resumeMode = true
		args = args[1:]
	}
	fs := flag.NewFlagSet("exec", flag.ContinueOnError)
	root := fs.String("C", ".", "working root")
	model := fs.String("m", "", "model")
	homeDir := fs.String("codex-home", "", "override codex home directory")
	profile := fs.String("profile", "", "config profile")
	approval := fs.String("ask-for-approval", "", "approval policy")
	sandbox := fs.String("sandbox", "", "sandbox mode")
	jsonOutput := fs.Bool("json", false, "print JSONL events")
	outputLastMessage := fs.String("o", "", "write final synthesized last message")
	skipGitRepoCheck := fs.Bool("skip-git-repo-check", false, "allow running outside a git repo")
	last := fs.Bool("last", false, "resume the last logical session")
	if err := fs.Parse(args); err != nil {
		return err
	}
	remaining := fs.Args()
	request := codexexec.Request{
		Root:              *root,
		HomeDir:           *homeDir,
		Profile:           *profile,
		Model:             *model,
		ApprovalPolicy:    *approval,
		SandboxMode:       *sandbox,
		OutputLastMessage: *outputLastMessage,
		SkipGitRepoCheck:  *skipGitRepoCheck,
		Last:              *last,
	}
	var result codexexec.Result
	var err error
	if resumeMode {
		promptArgs := remaining
		if !*last && len(remaining) > 0 && remaining[0] != "-" {
			request.SessionID = remaining[0]
			promptArgs = remaining[1:]
		}
		request.Prompt, err = readPrompt(promptArgs)
		if err != nil {
			return err
		}
		result, err = codexexec.Resume(request)
	} else {
		request.Prompt, err = readPrompt(remaining)
		if err != nil {
			return err
		}
		result, err = codexexec.Exec(request)
	}
	if err != nil {
		return err
	}
	return writeExecOutput(result, *jsonOutput)
}

func runResume(args []string) error {
	return runExec(append([]string{"resume"}, args...))
}

func readPrompt(args []string) (string, error) {
	if len(args) > 0 {
		if args[0] == "-" {
			return readPromptFromStdin()
		}
		return strings.TrimSpace(strings.Join(args, " ")), nil
	}
	return readPromptFromStdin()
}

func readPromptFromStdin() (string, error) {
	payload, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(payload)), nil
}

func writeExecOutput(result codexexec.Result, jsonOutput bool) error {
	if !jsonOutput {
		return writeOutput(false, map[string]any{
			"sessionId":            result.SessionID,
			"nativeSessionId":      result.NativeSessionID,
			"orchestrationSession": result.OrchestratorID,
			"taskId":               result.TaskID,
			"status":               result.Burst.Status,
			"summary":              result.Burst.Summary,
			"artifactDir":          result.ArtifactDir,
		})
	}
	events := []map[string]any{
		{
			"type":                  "session.prepared",
			"session_id":            result.SessionID,
			"native_session_id":     result.NativeSessionID,
			"orchestration_session": result.OrchestratorID,
			"task_id":               result.TaskID,
		},
		{
			"type":           "route.decided",
			"session_id":     result.SessionID,
			"task_id":        result.TaskID,
			"route":          result.Route.Route,
			"dispatch_ready": result.Route.DispatchReady,
			"reason_codes":   result.Route.ReasonCodes,
		},
		{
			"type":         "dispatch.issued",
			"dispatch_id":  result.Dispatch.DispatchID,
			"session_id":   result.SessionID,
			"task_id":      result.TaskID,
			"worker_class": result.Dispatch.WorkerClass,
			"command":      result.Dispatch.Command,
		},
		{
			"type":              "worker.outcome",
			"session_id":        result.SessionID,
			"native_session_id": result.NativeSessionID,
			"dispatch_id":       result.Dispatch.DispatchID,
			"task_id":           result.TaskID,
			"status":            result.Burst.Status,
			"summary":           result.Burst.Summary,
			"artifact_dir":      result.ArtifactDir,
			"last_message_path": result.LastMessagePath,
		},
	}
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	for _, event := range events {
		if err := encoder.Encode(event); err != nil {
			return err
		}
	}
	_, err := os.Stdout.Write(buffer.Bytes())
	return err
}

func writeOutput(asJSON bool, value any) error {
	if asJSON {
		payload, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return err
		}
		_, err = os.Stdout.Write(append(payload, '\n'))
		return err
	}
	switch typed := value.(type) {
	case auth.Status:
		if !typed.Authenticated {
			_, err := fmt.Fprintln(os.Stdout, "not authenticated")
			return err
		}
		_, err := fmt.Fprintf(os.Stdout, "authenticated via %s (%s)\n", typed.Mode, typed.Source)
		return err
	case map[string]any:
		payload, err := json.MarshalIndent(typed, "", "  ")
		if err != nil {
			return err
		}
		_, err = os.Stdout.Write(append(payload, '\n'))
		return err
	default:
		_, err := fmt.Fprintln(os.Stdout, value)
		return err
	}
}
