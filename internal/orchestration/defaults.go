package orchestration

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type PlannerAgent struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Focus     string `json:"focus"`
	PromptRef string `json:"promptRef"`
}

type JudgeAgent struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Focus      string   `json:"focus"`
	PromptRef  string   `json:"promptRef"`
	Dimensions []string `json:"dimensions"`
}

type SpecLoop struct {
	PlannerCount int            `json:"plannerCount"`
	Planners     []PlannerAgent `json:"planners"`
	Judge        JudgeAgent     `json:"judge"`
	OutputShape  []string       `json:"outputShape"`
}

const specPromptDir = "prompts/spec"

func DefaultPromptStages() []string {
	return []string{
		"context_assembly",
		"spec_parallel_planning",
		"spec_judging",
		"task_formatting",
		"execute",
		"verify",
		"handoff",
	}
}

func SpecPromptDir(root string) string {
	return filepath.Join(root, specPromptDir)
}

func SpecPromptFiles() []string {
	return []string{
		"README.md",
		"orchestrator.md",
		"propose.md",
		"proposal.md",
		"specs.md",
		"design.md",
		"tasks.md",
		"apply.md",
		"verify.md",
		"archive.md",
		"planner-architecture.md",
		"planner-delivery.md",
		"planner-risk.md",
		"judge.md",
	}
}

func SpecPromptRefs(root string) map[string]string {
	dir := SpecPromptDir(root)
	return map[string]string{
		"specPromptDir":           dir,
		"specReadme":              filepath.Join(dir, "README.md"),
		"specOrchestrator":        filepath.Join(dir, "orchestrator.md"),
		"specWorkflowPropose":     filepath.Join(dir, "propose.md"),
		"specArtifactProposal":    filepath.Join(dir, "proposal.md"),
		"specArtifactSpecs":       filepath.Join(dir, "specs.md"),
		"specArtifactDesign":      filepath.Join(dir, "design.md"),
		"specArtifactTasks":       filepath.Join(dir, "tasks.md"),
		"specWorkflowApply":       filepath.Join(dir, "apply.md"),
		"specWorkflowVerify":      filepath.Join(dir, "verify.md"),
		"specWorkflowArchive":     filepath.Join(dir, "archive.md"),
		"specPlannerArchitecture": filepath.Join(dir, "planner-architecture.md"),
		"specPlannerDelivery":     filepath.Join(dir, "planner-delivery.md"),
		"specPlannerRisk":         filepath.Join(dir, "planner-risk.md"),
		"specJudge":               filepath.Join(dir, "judge.md"),
	}
}

func DefaultSpecLoop(root string) SpecLoop {
	promptsDir := SpecPromptDir(root)
	return SpecLoop{
		PlannerCount: 3,
		Planners: []PlannerAgent{
			{
				ID:        "spec-architecture",
				Name:      "Spec Planner A",
				Focus:     "Architecture fit and bounded change shape",
				PromptRef: filepath.Join(promptsDir, "planner-architecture.md"),
			},
			{
				ID:        "spec-delivery",
				Name:      "Spec Planner B",
				Focus:     "Incremental delivery, work breakdown, and dependency order",
				PromptRef: filepath.Join(promptsDir, "planner-delivery.md"),
			},
			{
				ID:        "spec-risk",
				Name:      "Spec Planner C",
				Focus:     "Risk, verification, rollback, and phase-1 control-plane fit",
				PromptRef: filepath.Join(promptsDir, "planner-risk.md"),
			},
		},
		Judge: JudgeAgent{
			ID:        "spec-judge",
			Name:      "Spec Judge",
			Focus:     "Choose the best orchestration result and format final executable tasks",
			PromptRef: filepath.Join(promptsDir, "judge.md"),
			Dimensions: []string{
				"spec_clarity",
				"repo_fit",
				"execution_feasibility",
				"verification_completeness",
				"rollback_risk",
			},
		},
		OutputShape: []string{
			"objective",
			"constraints",
			"selected_plan",
			"rejected_alternatives",
			"execution_tasks",
			"verification_plan",
			"decision_rationale",
		},
	}
}

func DefaultTopLevelPrompt(root, userPrompt string) string {
	base := loadPromptOrFallback(
		filepath.Join(SpecPromptDir(root), "orchestrator.md"),
		`You are the Klein orchestration agent.

Use an OpenSpec-style artifact flow before execution:
- clarify proposal intent
- shape specs and design only to the depth needed
- produce executable tasks

Then use the default b3ehive-style convergence loop:
- run 3 isolated spec planners in parallel
- have 1 judge select and format the final orchestration result`,
	)
	lines := []string{
		strings.TrimSpace(base),
		"",
		"Load supporting spec prompts from prompts/spec in this order when relevant:",
	}
	for _, file := range SpecPromptFiles() {
		if file == "README.md" || file == "orchestrator.md" {
			continue
		}
		lines = append(lines, fmt.Sprintf("- prompts/spec/%s", file))
	}
	lines = append(lines,
		"",
		"User requirement:",
		strings.TrimSpace(userPrompt),
		"",
		"Final orchestration output must include:",
		"- objective",
		"- constraints",
		"- selected_plan",
		"- rejected_alternatives",
		"- execution_tasks",
		"- verification_plan",
		"- decision_rationale",
	)
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func loadPromptOrFallback(path, fallback string) string {
	payload, err := os.ReadFile(path)
	if err != nil {
		return strings.TrimSpace(fallback)
	}
	text := strings.TrimSpace(string(payload))
	if text == "" {
		return strings.TrimSpace(fallback)
	}
	return text
}
