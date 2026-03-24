package codexexec

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"klein-harness/internal/a2a"
	"klein-harness/internal/adapter"
	"klein-harness/internal/auth"
	"klein-harness/internal/checkpoint"
	"klein-harness/internal/codexconfig"
	"klein-harness/internal/dispatch"
	"klein-harness/internal/instructions"
	"klein-harness/internal/lease"
	"klein-harness/internal/orchestration"
	"klein-harness/internal/route"
	"klein-harness/internal/state"
	"klein-harness/internal/tmux"
	"klein-harness/internal/worker"
)

type Request struct {
	Root               string
	Prompt             string
	HomeDir            string
	Profile            string
	Model              string
	ApprovalPolicy     string
	SandboxMode        string
	OutputLastMessage  string
	SkipGitRepoCheck   bool
	AdditionalWritable []string
	SessionID          string
	Last               bool
}

type Result struct {
	SessionID       string          `json:"sessionId"`
	NativeSessionID string          `json:"nativeSessionId,omitempty"`
	OrchestratorID  string          `json:"orchestrationSessionId"`
	TaskID          string          `json:"taskId"`
	Route           route.Decision  `json:"route"`
	Dispatch        dispatch.Ticket `json:"dispatch"`
	Burst           tmux.Result     `json:"burst"`
	ArtifactDir     string          `json:"artifactDir"`
	LastMessagePath string          `json:"lastMessagePath,omitempty"`
}

type sessionSummary struct {
	state.Metadata
	Sessions      map[string]sessionRecord `json:"sessions"`
	Order         []string                 `json:"order"`
	LastSessionID string                   `json:"lastSessionId,omitempty"`
}

type sessionRecord struct {
	ID                     string   `json:"id"`
	TaskID                 string   `json:"taskId"`
	NativeSessionID        string   `json:"nativeSessionId,omitempty"`
	OrchestrationSessionID string   `json:"orchestrationSessionId"`
	Model                  string   `json:"model"`
	ApprovalPolicy         string   `json:"approvalPolicy"`
	SandboxMode            string   `json:"sandboxMode"`
	Prompt                 string   `json:"prompt"`
	Instructions           []string `json:"instructions,omitempty"`
	CreatedAt              string   `json:"createdAt"`
	UpdatedAt              string   `json:"updatedAt"`
	LastDispatchID         string   `json:"lastDispatchId,omitempty"`
	LastStatus             string   `json:"lastStatus,omitempty"`
	LastSummary            string   `json:"lastSummary,omitempty"`
}

type sessionIndexEntry struct {
	ID         string `json:"id"`
	ThreadName string `json:"thread_name"`
	UpdatedAt  string `json:"updated_at"`
}

func Exec(request Request) (Result, error) {
	return run(request, false)
}

func Resume(request Request) (Result, error) {
	return run(request, true)
}

func run(request Request, resume bool) (Result, error) {
	root, err := filepath.Abs(coalesce(request.Root, "."))
	if err != nil {
		return Result{}, err
	}
	status, err := auth.LoadStatus(request.HomeDir)
	if err != nil {
		return Result{}, err
	}
	if !status.Authenticated {
		return Result{}, fmt.Errorf("codex authentication is required; run kh-codex login status to inspect current auth")
	}
	config, err := codexconfig.Load(request.HomeDir)
	if err != nil {
		return Result{}, err
	}
	effective := codexconfig.Effective(config, request.Profile, codexconfig.Profile{
		Model:          request.Model,
		ApprovalPolicy: request.ApprovalPolicy,
		SandboxMode:    request.SandboxMode,
	})
	instructionFiles, err := instructions.Discover(root, request.HomeDir)
	if err != nil {
		return Result{}, err
	}
	summaryPath := filepath.Join(root, ".harness", "state", "codex-session-summary.json")
	summary, err := loadSessionSummary(summaryPath)
	if err != nil {
		return Result{}, err
	}
	registry, err := adapter.LoadSessionRegistry(root)
	if err != nil {
		return Result{}, err
	}
	orchestratorID, registry := ensureOrchestrator(registry, effective.Model)
	session, err := selectSession(summary, request, resume, orchestratorID, effective)
	if err != nil {
		return Result{}, err
	}
	if strings.TrimSpace(request.Prompt) == "" {
		request.Prompt = session.Prompt
	}
	if strings.TrimSpace(request.Prompt) == "" {
		return Result{}, fmt.Errorf("prompt is empty")
	}
	session.Prompt = request.Prompt
	beforeIndex, err := readSessionIndex(request.HomeDir)
	if err != nil {
		return Result{}, err
	}
	task := buildTask(root, request, session, orchestratorID, effective, instructionFiles)
	if err := adapter.UpsertTask(root, task); err != nil {
		return Result{}, err
	}
	latestPlanEpoch, err := adapter.LoadLatestPlanEpoch(root, task)
	if err != nil {
		return Result{}, err
	}
	checkpointFresh, err := adapter.LoadCheckpointFresh(root, task.TaskID)
	if err != nil {
		return Result{}, err
	}
	sessionContested := false
	if task.PreferredResumeSessionID != "" {
		for _, binding := range registry.ActiveBindings {
			if binding.TaskID != task.TaskID && binding.SessionID == task.PreferredResumeSessionID {
				sessionContested = true
				break
			}
		}
	}
	decision := route.Evaluate(route.Input{
		TaskID:                    task.TaskID,
		RoleHint:                  task.RoleHint,
		Kind:                      task.Kind,
		WorkerMode:                task.WorkerMode,
		PlanEpoch:                 task.PlanEpoch,
		LatestPlanEpoch:           latestPlanEpoch,
		ResumeStrategy:            task.ResumeStrategy,
		PreferredResumeSessionID:  task.PreferredResumeSessionID,
		CandidateResumeSessionIDs: task.CandidateResumeSessionIDs,
		SessionContested:          sessionContested,
		CheckpointRequired:        task.CheckpointRequired,
		CheckpointFresh:           checkpointFresh,
		WorktreePath:              adapter.TaskCWD(mustResolve(root), task),
		OwnedPaths:                task.OwnedPaths,
		RequiredSummaryVersion:    "state.v1",
	})
	registry = upsertRoutingDecision(registry, adapter.RoutingDecisionRecord{
		TaskID:                    task.TaskID,
		OrchestrationSessionID:    orchestratorID,
		RoutingMode:               "programmatic",
		NeedsOrchestrator:         false,
		DispatchReady:             decision.DispatchReady,
		GateStatus:                decision.Route,
		GateReason:                strings.Join(decision.ReasonCodes, ", "),
		RoutingModel:              effective.Model,
		ExecutionModel:            task.ExecutionModel,
		ResumeStrategy:            task.ResumeStrategy,
		PreferredResumeSessionID:  task.PreferredResumeSessionID,
		CandidateResumeSessionIDs: slices.Clone(task.CandidateResumeSessionIDs),
		CacheAffinityKey:          "cwd:" + root + "|role:worker",
		RoutingReason:             "Claude-style orchestration loop is applied through explicit prompt stages and repo-local routing state.",
		PromptStages:              slices.Clone(task.PromptStages),
		RoutedAt:                  state.NowUTC(),
	})
	paths := mustResolve(root)
	payload, err := a2a.NewPayload(decision)
	if err != nil {
		return Result{}, err
	}
	if _, err := a2a.AppendEvent(paths.EventLogPath, a2a.Envelope{
		Kind:           "route.decided",
		IdempotencyKey: fmt.Sprintf("route:%s:%s:%d", session.ID, task.TaskID, task.PlanEpoch),
		TraceID:        session.ID,
		CausationID:    newID("route"),
		From:           "kh-codex",
		To:             "worker-supervisor-node",
		RequestID:      session.ID,
		TaskID:         task.TaskID,
		PlanEpoch:      task.PlanEpoch,
		ReasonCodes:    decision.ReasonCodes,
		Payload:        payload,
	}); err != nil {
		return Result{}, err
	}
	if err := adapter.SaveSessionRegistry(root, registry); err != nil {
		return Result{}, err
	}
	if !decision.DispatchReady {
		session.LastStatus = decision.Route
		session.LastSummary = strings.Join(decision.ReasonCodes, ", ")
		session.UpdatedAt = state.NowUTC()
		summary = upsertSummarySession(summary, session)
		if err := saveSessionSummary(summaryPath, summary); err != nil {
			return Result{}, err
		}
		return Result{
			SessionID:      session.ID,
			OrchestratorID: orchestratorID,
			TaskID:         task.TaskID,
			Route:          decision,
		}, nil
	}

	attempt, err := adapter.CountDispatchAttempts(root, task.TaskID)
	if err != nil {
		return Result{}, err
	}
	causationID := newID("route")
	ticket, _, err := dispatch.Issue(dispatch.IssueRequest{
		Root:                   root,
		RequestID:              session.ID,
		TaskID:                 task.TaskID,
		PlanEpoch:              task.PlanEpoch,
		Attempt:                attempt + 1,
		IdempotencyKey:         fmt.Sprintf("dispatch:%s:epoch_%d:attempt_%d", task.TaskID, task.PlanEpoch, attempt+1),
		CausationID:            causationID,
		ReasonCodes:            decision.ReasonCodes,
		WorkerClass:            effective.Model,
		Cwd:                    adapter.TaskCWD(mustResolve(root), task),
		Command:                adapter.DispatchCommand(task),
		PromptRef:              "prompts/worker-burst.md",
		Budget:                 dispatch.Budget{MaxTurns: 8, MaxMinutes: 20, MaxToolCalls: 30},
		LeaseTTLSec:            1800,
		RequiredSummaryVersion: decision.RequiredSummaryVersion,
		ResumeSessionID:        decision.ResumeSessionID,
		WorktreePath:           decision.WorktreePath,
		OwnedPaths:             decision.OwnedPaths,
	})
	if err != nil {
		return Result{}, err
	}
	leaseRecord, err := lease.Acquire(lease.AcquireRequest{
		Root:        root,
		TaskID:      ticket.TaskID,
		DispatchID:  ticket.DispatchID,
		WorkerID:    "kh-codex",
		TTLSeconds:  ticket.LeaseTTLSec,
		CausationID: causationID,
		ReasonCodes: []string{"kh_codex_exec"},
	})
	if err != nil {
		return Result{}, err
	}
	ticket, err = dispatch.Claim(dispatch.ClaimRequest{
		Root:        root,
		DispatchID:  ticket.DispatchID,
		WorkerID:    "kh-codex",
		LeaseID:     leaseRecord.LeaseID,
		CausationID: causationID,
		ReasonCodes: []string{"kh_codex_exec"},
	})
	if err != nil {
		return Result{}, err
	}
	bundle, err := worker.Prepare(root, ticket, leaseRecord.LeaseID)
	if err != nil {
		return Result{}, err
	}
	checkpointPath := dispatch.DefaultCheckpointPath(root, ticket.TaskID, ticket.Attempt)
	outcomePath := filepath.Join(filepath.Dir(checkpointPath), "outcome.json")
	lastMessagePath := filepath.Join(bundle.ArtifactDir, "last-message.txt")
	command := resolveCommand(ticket.Command, map[string]string{
		"SESSION_ID":        task.PreferredResumeSessionID,
		"LAST_MESSAGE_PATH": lastMessagePath,
	})
	burst, err := tmux.RunBoundedBurst(tmux.BurstRequest{
		TaskID:         ticket.TaskID,
		DispatchID:     ticket.DispatchID,
		WorkerID:       "kh-codex",
		Cwd:            ticket.Cwd,
		Command:        command,
		PromptPath:     bundle.PromptPath,
		Budget:         ticket.Budget,
		CheckpointPath: checkpointPath,
		OutcomePath:    outcomePath,
		Artifacts: []string{
			bundle.ManifestPath,
			bundle.PromptPath,
			filepath.Join(bundle.ArtifactDir, "worker-result.json"),
			filepath.Join(bundle.ArtifactDir, "verify.json"),
			filepath.Join(bundle.ArtifactDir, "handoff.md"),
			lastMessagePath,
		},
	})
	if err != nil {
		return Result{}, err
	}
	if _, err := checkpoint.IngestCheckpoint(checkpoint.IngestCheckpointRequest{
		Root:          root,
		TaskID:        ticket.TaskID,
		DispatchID:    ticket.DispatchID,
		PlanEpoch:     ticket.PlanEpoch,
		Attempt:       ticket.Attempt,
		CausationID:   causationID,
		ReasonCodes:   []string{"kh_codex_checkpoint"},
		CheckpointRef: checkpointPath,
		Status:        "checkpointed",
		Summary:       "kh-codex bounded burst checkpoint persisted",
	}); err != nil {
		return Result{}, err
	}
	nextKind := ""
	if burst.Status == "failed" || burst.Status == "timed_out" {
		nextKind = "replan"
	}
	if _, err := checkpoint.IngestOutcome(checkpoint.IngestOutcomeRequest{
		Root:          root,
		TaskID:        ticket.TaskID,
		DispatchID:    ticket.DispatchID,
		PlanEpoch:     ticket.PlanEpoch,
		Attempt:       ticket.Attempt,
		CausationID:   causationID,
		WorkerID:      "kh-codex",
		LeaseID:       leaseRecord.LeaseID,
		ReasonCodes:   []string{"kh_codex_finished"},
		Status:        burst.Status,
		Summary:       burst.Summary,
		CheckpointRef: checkpointPath,
		DiffStats: checkpoint.DiffStats{
			FilesChanged: burst.DiffStats["filesChanged"],
			Insertions:   burst.DiffStats["insertions"],
			Deletions:    burst.DiffStats["deletions"],
		},
		Artifacts:         burst.Artifacts,
		NextSuggestedKind: nextKind,
	}); err != nil {
		return Result{}, err
	}
	if _, err := dispatch.UpdateStatus(root, ticket.DispatchID, burst.Status, "kh-codex"); err != nil {
		return Result{}, err
	}
	if _, err := lease.Release(root, leaseRecord.LeaseID, causationID, []string{"kh_codex_finished"}); err != nil {
		return Result{}, err
	}

	afterIndex, err := readSessionIndex(request.HomeDir)
	if err != nil {
		return Result{}, err
	}
	nativeSessionID := detectNativeSessionID(beforeIndex, afterIndex, task.PreferredResumeSessionID)
	session.NativeSessionID = nativeSessionID
	session.LastDispatchID = ticket.DispatchID
	session.LastStatus = burst.Status
	session.LastSummary = burst.Summary
	session.UpdatedAt = state.NowUTC()
	summary = upsertSummarySession(summary, session)
	if err := saveSessionSummary(summaryPath, summary); err != nil {
		return Result{}, err
	}
	registry = upsertWorkerSession(registry, adapter.SessionRecord{
		SessionID:           nativeSessionID,
		RootSessionID:       nativeSessionID,
		BranchRootSessionID: nativeSessionID,
		SourceTaskID:        task.TaskID,
		Model:               task.ExecutionModel,
		Role:                "worker",
		Status:              burst.Status,
		LastUsedAt:          state.NowUTC(),
	})
	if registry.LastCompletedByTask == nil {
		registry.LastCompletedByTask = map[string]string{}
	}
	if nativeSessionID != "" {
		registry.LastCompletedByTask[task.TaskID] = nativeSessionID
	}
	if err := adapter.SaveSessionRegistry(root, registry); err != nil {
		return Result{}, err
	}

	if request.OutputLastMessage != "" {
		if err := os.WriteFile(request.OutputLastMessage, []byte(renderLastMessage(task.TaskID, burst, nativeSessionID)), 0o644); err != nil {
			return Result{}, err
		}
	}
	return Result{
		SessionID:       session.ID,
		NativeSessionID: nativeSessionID,
		OrchestratorID:  orchestratorID,
		TaskID:          task.TaskID,
		Route:           decision,
		Dispatch:        ticket,
		Burst:           burst,
		ArtifactDir:     bundle.ArtifactDir,
		LastMessagePath: coalesce(request.OutputLastMessage, lastMessagePath),
	}, nil
}

func loadSessionSummary(path string) (sessionSummary, error) {
	var summary sessionSummary
	if err := state.LoadJSON(path, &summary); err != nil {
		if os.IsNotExist(err) {
			return sessionSummary{Sessions: map[string]sessionRecord{}}, nil
		}
		return sessionSummary{}, err
	}
	if summary.Sessions == nil {
		summary.Sessions = map[string]sessionRecord{}
	}
	return summary, nil
}

func saveSessionSummary(path string, summary sessionSummary) error {
	if summary.Sessions == nil {
		summary.Sessions = map[string]sessionRecord{}
	}
	_, err := state.WriteSnapshot(path, &summary, "kh-codex", summary.Revision)
	return err
}

func selectSession(summary sessionSummary, request Request, resume bool, orchestratorID string, profile codexconfig.Profile) (sessionRecord, error) {
	if resume {
		record, ok := findSession(summary, request.SessionID, request.Last)
		if !ok {
			return sessionRecord{}, fmt.Errorf("resume session not found")
		}
		if strings.TrimSpace(request.Prompt) != "" {
			record.Prompt = request.Prompt
		}
		record.UpdatedAt = state.NowUTC()
		if record.Model == "" {
			record.Model = profile.Model
		}
		if record.OrchestrationSessionID == "" {
			record.OrchestrationSessionID = orchestratorID
		}
		return record, nil
	}
	now := state.NowUTC()
	id := newID("codexsess")
	return sessionRecord{
		ID:                     id,
		TaskID:                 "task_" + id,
		OrchestrationSessionID: orchestratorID,
		Model:                  profile.Model,
		ApprovalPolicy:         profile.ApprovalPolicy,
		SandboxMode:            profile.SandboxMode,
		Prompt:                 request.Prompt,
		CreatedAt:              now,
		UpdatedAt:              now,
	}, nil
}

func buildTask(root string, request Request, session sessionRecord, orchestratorID string, profile codexconfig.Profile, instructionFiles []instructions.File) adapter.Task {
	commandProfile := adapter.CommandProfile{
		Standard:    buildCommandProfile(false, request, profile),
		LocalCompat: buildCommandProfile(false, request, profile),
	}
	if session.NativeSessionID != "" {
		commandProfile.Standard = buildCommandProfile(true, request, profile)
		commandProfile.LocalCompat = commandProfile.Standard
	}
	summary := "Claude-style orchestration loop over Codex exec protocol"
	if len(instructionFiles) > 0 {
		summary = fmt.Sprintf("%s (%d instruction files loaded)", summary, len(instructionFiles))
	}
	description := orchestration.DefaultTopLevelPrompt(root, request.Prompt)
	return adapter.Task{
		TaskID:                    session.TaskID,
		ThreadKey:                 session.ID,
		Kind:                      "execute",
		RoleHint:                  "worker",
		Title:                     shortTitle(request.Prompt),
		Summary:                   summary,
		Description:               description,
		WorkerMode:                "execution",
		Status:                    "queued",
		PlanEpoch:                 1,
		WorktreePath:              ".",
		BranchName:                "main",
		BaseRef:                   "HEAD",
		DiffBase:                  "HEAD",
		OwnedPaths:                writablePaths(root),
		ForbiddenPaths:            []string{".git/**", ".harness/state/**", ".harness/events/**"},
		ResumeStrategy:            resumeStrategy(session.NativeSessionID),
		PreferredResumeSessionID:  session.NativeSessionID,
		CandidateResumeSessionIDs: candidateSessions(session.NativeSessionID),
		CheckpointRequired:        false,
		RoutingModel:              "gpt-5.4",
		ExecutionModel:            profile.Model,
		OrchestrationSessionID:    orchestratorID,
		PromptStages:              orchestration.DefaultPromptStages(),
		Dispatch: adapter.DispatchProfile{
			WorkspaceRoot:  root,
			WorktreePath:   ".",
			BranchName:     "main",
			BaseRef:        "HEAD",
			DiffBase:       "HEAD",
			CommandProfile: commandProfile,
		},
	}
}

func writablePaths(root string) []string {
	entries, err := os.ReadDir(root)
	if err != nil {
		return []string{"README.md"}
	}
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if name == ".git" || name == ".harness" {
			continue
		}
		if entry.IsDir() {
			paths = append(paths, name+"/**")
			continue
		}
		paths = append(paths, name)
	}
	if len(paths) == 0 {
		return []string{"README.md"}
	}
	return paths
}

func buildCommandProfile(resume bool, request Request, profile codexconfig.Profile) string {
	args := []string{"codex", "exec"}
	if resume {
		args = append(args, "resume", "<SESSION_ID>")
	}
	args = append(args, "--json", "--output-last-message", "<LAST_MESSAGE_PATH>", "--model", profile.Model)
	switch {
	case profile.SandboxMode == "danger-full-access" && profile.ApprovalPolicy == "never":
		args = append(args, "--dangerously-bypass-approvals-and-sandbox")
	case profile.SandboxMode == "workspace-write" && (profile.ApprovalPolicy == "on-request" || profile.ApprovalPolicy == "never"):
		args = append(args, "--full-auto")
	default:
		args = append(args, "--sandbox", profile.SandboxMode)
	}
	if request.SkipGitRepoCheck {
		args = append(args, "--skip-git-repo-check")
	}
	for _, dir := range request.AdditionalWritable {
		if strings.TrimSpace(dir) == "" {
			continue
		}
		args = append(args, "--add-dir", dir)
	}
	return strings.Join(args, " ")
}

func ensureOrchestrator(registry adapter.SessionRegistry, model string) (string, adapter.SessionRegistry) {
	now := state.NowUTC()
	if registry.OrchestrationSessionID == "" {
		registry.OrchestrationSessionID = newID("orch")
	}
	record := adapter.SessionRecord{
		SessionID:  registry.OrchestrationSessionID,
		Model:      coalesce(model, "gpt-5.4"),
		Role:       "orchestrator",
		Status:     "active",
		Purpose:    "Claude-style orchestration, routing fallback, prompt refinement, and resume decisions",
		LastUsedAt: now,
	}
	registry.OrchestrationSessions = upsertSessionRecord(registry.OrchestrationSessions, record)
	if registry.LastCompletedByTask == nil {
		registry.LastCompletedByTask = map[string]string{}
	}
	return registry.OrchestrationSessionID, registry
}

func upsertRoutingDecision(registry adapter.SessionRegistry, decision adapter.RoutingDecisionRecord) adapter.SessionRegistry {
	for index, existing := range registry.RoutingDecisions {
		if existing.TaskID == decision.TaskID {
			registry.RoutingDecisions[index] = decision
			return registry
		}
	}
	registry.RoutingDecisions = append(registry.RoutingDecisions, decision)
	return registry
}

func upsertWorkerSession(registry adapter.SessionRegistry, record adapter.SessionRecord) adapter.SessionRegistry {
	if record.SessionID == "" {
		return registry
	}
	registry.Sessions = upsertSessionRecord(registry.Sessions, record)
	return registry
}

func upsertSessionRecord(records []adapter.SessionRecord, record adapter.SessionRecord) []adapter.SessionRecord {
	for index, existing := range records {
		if existing.SessionID == record.SessionID {
			records[index] = record
			return records
		}
	}
	return append(records, record)
}

func upsertSummarySession(summary sessionSummary, record sessionRecord) sessionSummary {
	if summary.Sessions == nil {
		summary.Sessions = map[string]sessionRecord{}
	}
	summary.Sessions[record.ID] = record
	if !slices.Contains(summary.Order, record.ID) {
		summary.Order = append(summary.Order, record.ID)
	}
	summary.LastSessionID = record.ID
	return summary
}

func findSession(summary sessionSummary, sessionID string, last bool) (sessionRecord, bool) {
	if last || sessionID == "" {
		if summary.LastSessionID == "" {
			return sessionRecord{}, false
		}
		record, ok := summary.Sessions[summary.LastSessionID]
		return record, ok
	}
	record, ok := summary.Sessions[sessionID]
	return record, ok
}

func readSessionIndex(homeDir string) ([]sessionIndexEntry, error) {
	path := filepath.Join(resolveCodexHome(homeDir), "session_index.jsonl")
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()
	entries := []sessionIndexEntry{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var entry sessionIndexEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			return nil, err
		}
		if entry.ID != "" {
			entries = append(entries, entry)
		}
	}
	return entries, scanner.Err()
}

func detectNativeSessionID(before, after []sessionIndexEntry, fallback string) string {
	seen := map[string]struct{}{}
	for _, entry := range before {
		seen[entry.ID] = struct{}{}
	}
	for index := len(after) - 1; index >= 0; index-- {
		entry := after[index]
		if _, ok := seen[entry.ID]; ok {
			continue
		}
		return entry.ID
	}
	return fallback
}

func renderLastMessage(taskID string, burst tmux.Result, nativeSessionID string) string {
	lines := []string{
		fmt.Sprintf("task %s finished with status %s", taskID, burst.Status),
		burst.Summary,
	}
	if nativeSessionID != "" {
		lines = append(lines, "native session id: "+nativeSessionID)
	}
	return strings.Join(lines, "\n") + "\n"
}

func shortTitle(prompt string) string {
	trimmed := strings.TrimSpace(prompt)
	if trimmed == "" {
		return "Codex task"
	}
	line := strings.Split(trimmed, "\n")[0]
	if len(line) > 80 {
		return line[:80]
	}
	return line
}

func resumeStrategy(nativeSessionID string) string {
	if nativeSessionID != "" {
		return "resume"
	}
	return "fresh"
}

func candidateSessions(nativeSessionID string) []string {
	if nativeSessionID == "" {
		return nil
	}
	return []string{nativeSessionID}
}

func resolveCommand(command string, replacements map[string]string) string {
	resolved := command
	for key, value := range replacements {
		resolved = strings.ReplaceAll(resolved, "<"+key+">", value)
	}
	return resolved
}

func resolveCodexHome(homeDir string) string {
	if homeDir != "" {
		return homeDir
	}
	if value := os.Getenv("CODEX_HOME"); value != "" {
		return value
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".codex"
	}
	return filepath.Join(home, ".codex")
}

func newID(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UTC().UnixNano())
}

func mustResolve(root string) adapter.Paths {
	paths, err := adapter.Resolve(root)
	if err != nil {
		panic(err)
	}
	return paths
}

func coalesce(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
