package orchestration

import "strings"

import "klein-harness/internal/adapter"

type SOPRegistry struct {
	byID     map[string]SOPDefinition
	byFamily map[TaskFamily][]SOPDefinition
}

func DefaultSOPRegistry() SOPRegistry {
	defs := []SOPDefinition{
		repeatedEntityCorpusSOP(),
		developmentTaskSOP(),
	}
	byID := make(map[string]SOPDefinition, len(defs))
	byFamily := map[TaskFamily][]SOPDefinition{}
	for _, def := range defs {
		byID[def.ID] = def
		byFamily[def.Family] = append(byFamily[def.Family], def)
	}
	return SOPRegistry{byID: byID, byFamily: byFamily}
}

func (r SOPRegistry) Lookup(id string) (SOPDefinition, bool) {
	item, ok := r.byID[id]
	return item, ok
}

func (r SOPRegistry) LookupByFamily(family TaskFamily) []SOPDefinition {
	return append([]SOPDefinition(nil), r.byFamily[family]...)
}

func canonicalSOPFamily(family TaskFamily) TaskFamily {
	switch family {
	case TaskFamilyBugfixSmall, TaskFamilyFeatureModule, TaskFamilyFeatureSystem, TaskFamilyDevelopmentTask, TaskFamilyIntegrationExternal, TaskFamilyRepairOrResume:
		return TaskFamilyDevelopmentTask
	default:
		return family
	}
}

func resolveSOPDefinition(task adapter.Task) (SOPDefinition, bool) {
	registry := DefaultSOPRegistry()
	if task.SOPID != "" {
		if def, ok := registry.Lookup(task.SOPID); ok {
			return def, true
		}
	}
	family := TaskFamily(task.TaskFamily)
	if family == "" || family == TaskFamilyUnknown {
		inferred, _ := ClassifyTaskFamily(task.Kind, task.Summary, []string{task.Description})
		family = inferred
	}
	candidates := registry.LookupByFamily(canonicalSOPFamily(family))
	if len(candidates) == 0 {
		return SOPDefinition{}, false
	}
	return candidates[0], true
}

func ClassifyTaskFamily(kind, goal string, contexts []string) (TaskFamily, string) {
	lower := strings.ToLower(strings.Join(append([]string{kind, goal}, contexts...), "\n"))
	switch {
	case hasAny(lower, "review", "audit", "审查", "审计", "代码评审"):
		return TaskFamilyReviewOrAudit, ""
	case hasAny(lower, "resume", "repair", "恢复", "重试", "断点", "继续执行"):
		return TaskFamilyRepairOrResume, ""
	case hasAny(lower, "integration", "third-party", "第三方", "支付", "oauth", "sso", "接入"):
		return TaskFamilyIntegrationExternal, SOPDevelopmentTaskV1
	case hasAny(lower, "bug", "fix", "regression", "error", "failure", "报错", "修复"):
		return TaskFamilyBugfixSmall, SOPDevelopmentTaskV1
	case hasAny(lower, "system", "架构", "framework", "pipeline", "引擎", "flow"):
		return TaskFamilyFeatureSystem, SOPDevelopmentTaskV1
	case hasAny(lower, "prd", "需求", "接口", "联调", "测试", "开发", "refactor", "重构"):
		return TaskFamilyDevelopmentTask, SOPDevelopmentTaskV1
	case hasAny(lower, "module", "模块", "组件", "page", "页面"):
		return TaskFamilyFeatureModule, SOPDevelopmentTaskV1
	case looksLikeRepeatedEntityCorpus(lower):
		return TaskFamilyRepeatedEntityCorpus, SOPRepeatedEntityCorpusV1
	case hasAny(lower, "markdown", "文档", "报告", "单文档", "single artifact"):
		return TaskFamilySingleArtifact, ""
	default:
		return TaskFamilyDevelopmentTask, SOPDevelopmentTaskV1
	}
}

func looksLikeRepeatedEntityCorpus(lower string) bool {
	if hasAny(lower, "人物", "程序员", "哲学家", "角色卡", "entity", "roster", "名单", "世界上最伟大", "不少于", "每个人", "每位") {
		return true
	}
	return hasAny(lower, " 位", " 个", " 名") && hasAny(lower, "markdown", "文档", "生成")
}

func hasAny(haystack string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(haystack, needle) {
			return true
		}
	}
	return false
}

func CompileFlowForTask(root string, task adapter.Task) CompiledFlow {
	def, ok := resolveSOPDefinition(task)
	if !ok {
		return CompiledFlow{}
	}
	switch def.ID {
	case SOPRepeatedEntityCorpusV1:
		return CompileRepeatedEntityCorpus(root, task)
	case SOPDevelopmentTaskV1:
		return CompileDevelopmentTask(task)
	default:
		return CompileDevelopmentTask(task)
	}
}
