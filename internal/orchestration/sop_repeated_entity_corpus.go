package orchestration

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"klein-harness/internal/adapter"
)

func repeatedEntityCorpusSOP() SOPDefinition {
	return SOPDefinition{
		ID:                 SOPRepeatedEntityCorpusV1,
		Family:             TaskFamilyRepeatedEntityCorpus,
		Description:        "Program-owned shared spec extraction, variable extraction, fanout compilation, verify skeleton, and closeout skeleton for repeated entity corpus tasks.",
		DirectPassEligible: false,
		Phases: []SOPPhase{
			{ID: "extract_shared_spec", Description: "Extract shared corpus contract", ModelOutput: []string{"output_root", "file_naming_rule", "required_support_files", "required_sections", "min_chars", "source_policy", "ordering_policy", "index_filename"}, ProgramOwns: []string{"shared-spec.json"}},
			{ID: "extract_variable_inputs", Description: "Extract entity roster and per-entity variables", ModelOutput: []string{"entity roster", "slug", "target file", "core angle"}, ProgramOwns: []string{"variable-inputs.json"}},
			{ID: "compile_task_graph", Description: "Compile entity slices and closeout slice", ProgramOwns: []string{"task-graph.json"}},
			{ID: "compile_worker_prompt", Description: "Compile short worker prompts", ProgramOwns: []string{"slice-context.json", "takeover-context.json"}},
			{ID: "programmatic_verify", Description: "Programmatic file and section checks", ProgramOwns: []string{"verify-skeleton.json"}},
			{ID: "closeout", Description: "Index and verification summary closeout", ProgramOwns: []string{"closeout skeleton"}},
		},
		VerifyRules: []VerifyRuleSpec{
			{ID: "files_exist", Description: "Required files and support files must exist."},
			{ID: "sections_present", Description: "Each entity file must contain required sections."},
			{ID: "min_chars", Description: "Each entity file must satisfy the minimum character count."},
		},
		CloseoutRules: []CloseoutRuleSpec{
			{ID: "index_closeout", Description: "Program composes index and verification summary artifacts."},
		},
		ContinuationArtifacts: []string{"shared-spec.json", "variable-inputs.json", "shared-flow-context.json", "slice-context.json", "verify-skeleton.json", "handoff.md"},
	}
}

func CompileRepeatedEntityCorpus(root string, task adapter.Task) CompiledFlow {
	shared := ExtractRepeatedEntitySharedSpec(task)
	vars := ExtractRepeatedEntityVariableInputs(task, shared)
	tasks := CompileRepeatedEntityTaskGraph(task, shared, vars)
	sharedCtx := buildRepeatedEntitySharedContext(task, shared, vars)
	return CompiledFlow{
		Family:                 TaskFamilyRepeatedEntityCorpus,
		SOPID:                  SOPRepeatedEntityCorpusV1,
		SharedSpec:             shared,
		VariableInputs:         vars,
		ExecutionTasks:         tasks,
		SharedTaskGroupContext: sharedCtx,
		SharedFlowContext: SharedFlowContext{
			TaskFamily:      TaskFamilyRepeatedEntityCorpus,
			SOPID:           SOPRepeatedEntityCorpusV1,
			Summary:         strings.TrimSpace(coalesce(task.Description, task.Summary, task.Title)),
			BoundarySummary: uniqueStrings(append([]string{"程序负责冻结 shared spec 和 variable inputs", "worker 只处理当前 entity slice 或 closeout slice"}, task.OwnedPaths...)),
		},
	}
}

func ExtractRepeatedEntitySharedSpec(task adapter.Task) RepeatedEntitySharedSpec {
	text := strings.Join([]string{task.Title, task.Summary, task.Description}, "\n")
	outputRoot := detectOutputRoot(text)
	if outputRoot == "" {
		outputRoot = "rundata/generated-corpus"
	}
	minChars := detectMinChars(text)
	requiredSections := detectSectionList(text)
	if len(requiredSections) == 0 {
		requiredSections = []string{"代表作品", "核心贡献", "核心思想结晶", "方法论影响", "历史位置"}
	}
	index := "INDEX.md"
	if outputRoot != "" {
		index = filepath.Join(outputRoot, "INDEX.md")
	}
	return RepeatedEntitySharedSpec{
		OutputRoot:           outputRoot,
		FileNamingRule:       "NN-slug.md",
		RequiredSupportFiles: uniqueStrings([]string{index, filepath.Join(outputRoot, "task-checklist.md"), filepath.Join(outputRoot, "verify-report.json")}),
		RequiredSections:     requiredSections,
		MinChars:             minChars,
		SourcePolicy:         "prefer authoritative biographies, academic sources, and cross-check key facts before writing",
		OrderingPolicy:       "freeze roster order before fanout",
		IndexFilename:        index,
	}
}

func ExtractRepeatedEntityVariableInputs(task adapter.Task, spec RepeatedEntitySharedSpec) RepeatedEntityVariableInputs {
	text := strings.Join([]string{task.Title, task.Summary, task.Description}, "\n")
	roster := detectEntityRoster(text)
	if len(roster) == 0 {
		count := detectEntityCount(text)
		if count == 0 {
			count = 1
		}
		for i := 0; i < count; i++ {
			roster = append(roster, fmt.Sprintf("slot-%02d", i+1))
		}
	}
	out := make([]RepeatedEntityInput, 0, len(roster))
	for i, entity := range roster {
		slug := slugify(entity)
		target := ""
		if spec.OutputRoot != "" {
			target = filepath.Join(spec.OutputRoot, fmt.Sprintf("%02d-%s.md", i+1, slug))
		}
		out = append(out, RepeatedEntityInput{
			EntityLabel: entity,
			Slug:        slug,
			TargetFile:  target,
			CoreAngle:   "核心思想结晶与工程方法",
		})
	}
	return RepeatedEntityVariableInputs{Entities: out}
}

func CompileRepeatedEntityTaskGraph(task adapter.Task, spec RepeatedEntitySharedSpec, vars RepeatedEntityVariableInputs) []ExecutionTask {
	tasks := make([]ExecutionTask, 0, len(vars.Entities)+1)
	for i, entity := range vars.Entities {
		tasks = append(tasks, ExecutionTask{
			ID:            fmt.Sprintf("%s.slice.%d", task.TaskID, i+1),
			Title:         fmt.Sprintf("处理对象 %02d", i+1),
			Summary:       fmt.Sprintf("只处理 `%s`，输出到 %s。", entity.EntityLabel, entity.TargetFile),
			TaskGroupID:   task.TaskID + ".group",
			BatchLabel:    fmt.Sprintf("entity-%02d", i+1),
			EntityBatch:   []string{entity.EntityLabel},
			OutputTargets: uniqueStrings([]string{entity.TargetFile}),
			DoneCriteria: uniqueStrings([]string{
				fmt.Sprintf("完成 `%s` 的正文", entity.EntityLabel),
				fmt.Sprintf("满足不少于 %d 字", spec.MinChars),
				"verify evidence 已记录",
			}),
		})
	}
	tasks = append(tasks, ExecutionTask{
		ID:            fmt.Sprintf("%s.slice.%d", task.TaskID, len(vars.Entities)+1),
		Title:         "完成收口与索引",
		Summary:       fmt.Sprintf("生成索引与 verify 汇总，目标 %s。", spec.IndexFilename),
		TaskGroupID:   task.TaskID + ".group",
		BatchLabel:    "closeout",
		OutputTargets: uniqueStrings(append([]string{spec.IndexFilename}, spec.RequiredSupportFiles...)),
		DoneCriteria:  uniqueStrings([]string{"closeout artifacts 完整", "programmatic verify 完整"}),
	})
	return tasks
}

func buildRepeatedEntitySharedContext(task adapter.Task, spec RepeatedEntitySharedSpec, vars RepeatedEntityVariableInputs) *SharedTaskGroupContext {
	entities := make([]string, 0, len(vars.Entities))
	for _, item := range vars.Entities {
		entities = append(entities, item.EntityLabel)
	}
	return &SharedTaskGroupContext{
		GroupID: task.TaskID + ".group",
		Summary: strings.TrimSpace(coalesce(task.Description, task.Summary, task.Title)),
		EntitySelection: EntitySelection{
			SubjectLabel:      "entity",
			TargetCount:       len(vars.Entities),
			SelectionMode:     "program_frozen_roster",
			SelectionCriteria: []string{"先冻结名单，再 fanout 给单对象 worker"},
			Entities:          entities,
		},
		ContentContract: ContentContract{
			OutputDir:        spec.OutputRoot,
			IndexFile:        filepath.Base(spec.IndexFilename),
			FileExtension:    ".md",
			FileNamingRule:   spec.FileNamingRule,
			RequiredSections: spec.RequiredSections,
			MinChars:         spec.MinChars,
			FormatConstraints: []string{
				"一对象一文件",
				"索引与 verify 汇总由程序 closeout 生成",
			},
		},
		SourcePlan: SourcePlan{
			ResearchGoal:         "freeze shared corpus contract before per-entity writing",
			PreferredSourceTypes: []string{"authoritative biographies", "academic references", "institutional sources"},
			RequiredCrossCheck:   true,
			Notes:                []string{spec.SourcePolicy},
		},
		SharedPrompt: []string{
			"先用 shared spec，再处理当前 entity slice。",
			"如果 roster 或 shared spec 不完整，回报 planning drift，而不是自行重规划。",
		},
	}
}

func detectOutputRoot(text string) string {
	match := regexp.MustCompile(`(?:在|到)\s+([^\s]+)\s+(?:中|里|内|下)`).FindStringSubmatch(text)
	if len(match) == 2 {
		return strings.TrimSpace(match[1])
	}
	return ""
}

func detectMinChars(text string) int {
	match := regexp.MustCompile(`不少于\s*(\d+)\s*字`).FindStringSubmatch(text)
	if len(match) != 2 {
		return 0
	}
	var n int
	fmt.Sscanf(match[1], "%d", &n)
	return n
}

func detectSectionList(text string) []string {
	match := regexp.MustCompile(`(?:包括|包含)\s*([^\n]+)`).FindStringSubmatch(text)
	if len(match) != 2 {
		return nil
	}
	parts := strings.FieldsFunc(match[1], func(r rune) bool { return strings.ContainsRune("、，,；;。", r) })
	return uniqueStrings(parts)
}

func detectEntityRoster(text string) []string {
	match := regexp.MustCompile(`(?:包括|名单[:：]|分别是)\s*([^\n]+)`).FindStringSubmatch(text)
	if len(match) != 2 {
		return nil
	}
	parts := strings.FieldsFunc(match[1], func(r rune) bool { return strings.ContainsRune("、，,；;|/", r) })
	return uniqueStrings(parts)
}

func detectEntityCount(text string) int {
	match := regexp.MustCompile(`(\d+)\s*(?:位|个|名)`).FindStringSubmatch(text)
	if len(match) != 2 {
		return 0
	}
	var n int
	fmt.Sscanf(match[1], "%d", &n)
	return n
}

func slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, " ", "-")
	value = strings.ReplaceAll(value, "/", "-")
	value = regexp.MustCompile(`[^a-z0-9\p{Han}\-]+`).ReplaceAllString(value, "")
	value = strings.Trim(value, "-")
	if value == "" {
		return "entry"
	}
	return value
}

func coalesce(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
