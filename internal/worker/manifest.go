package worker

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"klein-harness/internal/adapter"
	"klein-harness/internal/dispatch"
	"klein-harness/internal/orchestration"
)

type verificationManifest struct {
	Rules []verificationRule `json:"rules"`
}

type verificationRule struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Exec         string `json:"exec"`
	Timeout      int    `json:"timeout"`
	ReadOnlySafe bool   `json:"readOnlySafe"`
}

type DispatchBundle struct {
	ManifestPath string
	PromptPath   string
	ArtifactDir  string
}

func Prepare(root string, ticket dispatch.Ticket, leaseID string) (DispatchBundle, error) {
	paths, err := adapter.Resolve(root)
	if err != nil {
		return DispatchBundle{}, err
	}
	task, err := adapter.LoadTask(root, ticket.TaskID)
	if err != nil {
		return DispatchBundle{}, err
	}
	projectMeta, err := adapter.LoadProjectMeta(root)
	if err != nil {
		return DispatchBundle{}, err
	}
	verifyCommands, err := verificationCommands(paths.VerificationRulesPath, task.VerificationRuleIDs)
	if err != nil {
		return DispatchBundle{}, err
	}
	executionCwd := adapter.TaskCWD(paths, task)
	artifactDir := filepath.Join(paths.ArtifactsDir, task.TaskID, ticket.DispatchID)
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		return DispatchBundle{}, err
	}
	manifestPath := filepath.Join(paths.StateDir, fmt.Sprintf("dispatch-manifest-%s.json", task.TaskID))
	promptPath := filepath.Join(paths.StateDir, fmt.Sprintf("runner-prompt-%s.md", task.TaskID))
	repoRole := projectMeta.RepoRole
	if repoRole == "" {
		repoRole = "target_repo"
	}
	directTargetEditAllowed := true
	if projectMeta.DirectTargetEditAllowed != nil {
		directTargetEditAllowed = *projectMeta.DirectTargetEditAllowed
	}
	intentFingerprint := stableFingerprint(
		task.TaskID,
		fmt.Sprintf("%d", task.PlanEpoch),
		task.Title,
		task.Summary,
		strings.Join(task.OwnedPaths, "|"),
		strings.Join(task.VerificationRuleIDs, "|"),
	)
	manifest := map[string]any{
		"schemaVersion":           "kh.dispatch-manifest.v1",
		"generator":               "kh-worker-supervisor",
		"generatedAt":             nowUTC(),
		"dispatchId":              ticket.DispatchID,
		"leaseId":                 leaseID,
		"taskId":                  task.TaskID,
		"threadKey":               task.ThreadKey,
		"planEpoch":               task.PlanEpoch,
		"intentFingerprint":       intentFingerprint,
		"taskKind":                task.Kind,
		"workerMode":              task.WorkerMode,
		"roleHint":                task.RoleHint,
		"repoRole":                repoRole,
		"directTargetEditAllowed": directTargetEditAllowed,
		"projectRoot":             paths.Root,
		"executionCwd":            executionCwd,
		"worktreePath":            coalesce(task.Dispatch.WorktreePath, task.WorktreePath),
		"branchName":              coalesce(task.Dispatch.BranchName, task.BranchName),
		"diffBase":                coalesce(task.Dispatch.DiffBase, task.DiffBase, task.BaseRef),
		"resumeStrategy":          task.ResumeStrategy,
		"sessionId":               ticket.ResumeSessionID,
		"routingModel":            task.RoutingModel,
		"executionModel":          task.ExecutionModel,
		"orchestrationSessionId":  task.OrchestrationSessionID,
		"promptStages":            unique(task.PromptStages),
		"allowedWriteGlobs":       unique(task.OwnedPaths),
		"blockedWriteGlobs":       unique(task.ForbiddenPaths),
		"artifactDir":             artifactDir,
		"artifacts": map[string]string{
			"workerResult": filepath.Join(artifactDir, "worker-result.json"),
			"verify":       filepath.Join(artifactDir, "verify.json"),
			"handoff":      filepath.Join(artifactDir, "handoff.md"),
		},
		"verification": map[string]any{
			"ruleIds":  unique(task.VerificationRuleIDs),
			"commands": verifyCommands,
		},
		"specPlanning": orchestration.DefaultSpecLoop(paths.Root),
		"runtimeRefs": mergeStringMaps(
			map[string]string{
				"promptRef":  ticket.PromptRef,
				"promptPath": promptPath,
			},
			orchestration.SpecPromptRefs(paths.Root),
		),
	}
	if err := writeJSON(manifestPath, manifest); err != nil {
		return DispatchBundle{}, err
	}
	prompt := buildPrompt(manifestPath, artifactDir, task, ticket)
	if err := os.WriteFile(promptPath, []byte(prompt), 0o644); err != nil {
		return DispatchBundle{}, err
	}
	return DispatchBundle{
		ManifestPath: manifestPath,
		PromptPath:   promptPath,
		ArtifactDir:  artifactDir,
	}, nil
}

func verificationCommands(path string, ruleIDs []string) ([]map[string]any, error) {
	if len(ruleIDs) == 0 {
		return []map[string]any{}, nil
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []map[string]any{}, nil
		}
		return nil, err
	}
	var manifest verificationManifest
	if err := json.Unmarshal(payload, &manifest); err != nil {
		return nil, err
	}
	index := map[string]verificationRule{}
	for _, rule := range manifest.Rules {
		if rule.ID != "" {
			index[rule.ID] = rule
		}
	}
	commands := make([]map[string]any, 0, len(ruleIDs))
	for _, ruleID := range ruleIDs {
		rule, ok := index[ruleID]
		if !ok {
			continue
		}
		commands = append(commands, map[string]any{
			"ruleId":       rule.ID,
			"title":        rule.Title,
			"exec":         rule.Exec,
			"timeout":      rule.Timeout,
			"readOnlySafe": rule.ReadOnlySafe,
		})
	}
	return commands, nil
}

func buildPrompt(manifestPath, artifactDir string, task adapter.Task, ticket dispatch.Ticket) string {
	lines := []string{
		"You are the Klein worker for exactly one bound task inside a repo-local closed-loop runtime.",
		"",
		"Read order:",
		fmt.Sprintf("1. Read the dispatch manifest first: %s", manifestPath),
		"2. If task-local artifacts already exist, read worker-result.json, verify.json, handoff.md, and referenced compact handoff logs.",
		"3. Read only the files explicitly referenced by the manifest before expanding your search.",
		"",
		"Hard authority rules:",
		"- Never create or mutate thread keys, request ids, task ids, plan epochs, leases, or global `.harness/state/*` ledgers.",
		"- Never edit files outside the bound worktree.",
		"- Never edit paths outside `allowedWriteGlobs`.",
		"- Never edit `blockedWriteGlobs`.",
		"- Never merge, rebase, push, archive, delete branches, or delete worktrees.",
		"- Never decide that the loop is complete. You may only decide the terminal outcome of this worker run.",
		"",
		"Execution style:",
		"- Fix root causes, not symptoms.",
		"- Keep changes minimal, focused, and consistent with the existing codebase.",
		"- Read a file before editing it.",
		"- Before each meaningful tool/action group, briefly state your immediate intent.",
		"- Follow the orchestration loop in order: context assembly -> targeted research -> plan -> execute -> verify -> handoff.",
		"- Do not skip directly from the request text to edits when the referenced files have not been read yet.",
		"- When the task starts from a requirement or spec request, first run the default 3+1 spec loop from the manifest: 3 parallel spec planners, then 1 judge/formatter subagent.",
		"- The judge must choose the final orchestration result using evidence, not blend all plans together by default.",
		"",
		"Verification:",
		"- Run verify commands from the manifest in order.",
		"- Start with the narrowest relevant validation, then broader checks when required.",
		"- Record each command, exit code, and output path in verify.json.",
		"",
		"Required artifacts before exit:",
		fmt.Sprintf("- %s", filepath.Join(artifactDir, "worker-result.json")),
		fmt.Sprintf("- %s", filepath.Join(artifactDir, "verify.json")),
		fmt.Sprintf("- %s", filepath.Join(artifactDir, "handoff.md")),
		"",
		"Task focus:",
		fmt.Sprintf("- taskId: %s", task.TaskID),
		fmt.Sprintf("- planEpoch: %d", task.PlanEpoch),
		fmt.Sprintf("- roleHint: %s", task.RoleHint),
		fmt.Sprintf("- taskKind: %s", task.Kind),
		fmt.Sprintf("- workerMode: %s", task.WorkerMode),
		fmt.Sprintf("- routingModel: %s", task.RoutingModel),
		fmt.Sprintf("- executionModel: %s", task.ExecutionModel),
		fmt.Sprintf("- orchestrationSessionId: %s", task.OrchestrationSessionID),
		fmt.Sprintf("- promptStages: %s", strings.Join(task.PromptStages, ", ")),
		fmt.Sprintf("- title: %s", task.Title),
		fmt.Sprintf("- summary: %s", task.Summary),
		fmt.Sprintf("- description: %s", task.Description),
		fmt.Sprintf("- ownedPaths: %s", strings.Join(task.OwnedPaths, ", ")),
		fmt.Sprintf("- verificationRuleIds: %s", strings.Join(task.VerificationRuleIDs, ", ")),
		fmt.Sprintf("- promptRef: %s", ticket.PromptRef),
		fmt.Sprintf("- specPromptDir: %s", filepath.Join("prompts", "spec")),
		fmt.Sprintf("- specReadme: %s", filepath.Join("prompts", "spec", "README.md")),
		fmt.Sprintf("- specOrchestrator: %s", filepath.Join("prompts", "spec", "orchestrator.md")),
		fmt.Sprintf("- specWorkflowPropose: %s", filepath.Join("prompts", "spec", "propose.md")),
		fmt.Sprintf("- specArtifactProposal: %s", filepath.Join("prompts", "spec", "proposal.md")),
		fmt.Sprintf("- specArtifactSpecs: %s", filepath.Join("prompts", "spec", "specs.md")),
		fmt.Sprintf("- specArtifactDesign: %s", filepath.Join("prompts", "spec", "design.md")),
		fmt.Sprintf("- specArtifactTasks: %s", filepath.Join("prompts", "spec", "tasks.md")),
		fmt.Sprintf("- specWorkflowApply: %s", filepath.Join("prompts", "spec", "apply.md")),
		fmt.Sprintf("- specWorkflowVerify: %s", filepath.Join("prompts", "spec", "verify.md")),
		fmt.Sprintf("- specWorkflowArchive: %s", filepath.Join("prompts", "spec", "archive.md")),
		fmt.Sprintf("- specPlannerArchitecture: %s", filepath.Join("prompts", "spec", "planner-architecture.md")),
		fmt.Sprintf("- specPlannerDelivery: %s", filepath.Join("prompts", "spec", "planner-delivery.md")),
		fmt.Sprintf("- specPlannerRisk: %s", filepath.Join("prompts", "spec", "planner-risk.md")),
		fmt.Sprintf("- specJudge: %s", filepath.Join("prompts", "spec", "judge.md")),
		"",
		"Final response:",
		"- Be brief.",
		"- Report only the terminal worker outcome and the key artifact path(s).",
		"- Do not claim global completion.",
	}
	return strings.Join(lines, "\n") + "\n"
}

func stableFingerprint(parts ...string) string {
	hash := sha256.Sum256([]byte(strings.Join(parts, "\x1f")))
	return hex.EncodeToString(hash[:16])
}

func unique(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func coalesce(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func nowUTC() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func writeJSON(path string, value any) error {
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(payload, '\n'), 0o644)
}

func mergeStringMaps(parts ...map[string]string) map[string]string {
	merged := map[string]string{}
	for _, part := range parts {
		for key, value := range part {
			merged[key] = value
		}
	}
	return merged
}
