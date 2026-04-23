package recommendation

import "sort"

type Engine struct {
	rules []Rule
}

func NewEngine(rules []Rule) *Engine {
	return &Engine{
		rules: rules,
	}
}

func (e *Engine) Recommend(input Input) []Recommendation {
	var result []Recommendation
	seen := map[string]bool{}

	for _, rule := range e.rules {
		if rule.ForStep != input.ForStep {
			continue
		}

		if !matchesAll(input.EvaluationGoals, rule.RequiredEvaluationGoals) {
			continue
		}

		if !matchesAny(input.ProjectType, rule.RequiredProjectTypes) {
			continue
		}

		if !matchesAny(input.Participants, rule.RequiredParticipants) {
			continue
		}

		if !matchesAny(input.DevelopmentStage, rule.RequiredDevelopmentStages) {
			continue
		}

		if !matchesAll(input.SelectedMethods, rule.RequiredMethods) {
			continue
		}

		for _, rec := range rule.Recommendations {
			if seen[rec.ID] {
				continue
			}
			seen[rec.ID] = true
			result = append(result, rec)
		}
	}

	sort.SliceStable(result, func(i, j int) bool {
		pi := priorityRank(result[i].Priority)
		pj := priorityRank(result[j].Priority)
		if pi != pj {
			return pi < pj
		}
		return result[i].Name < result[j].Name
	})

	return result
}

func matchesAll(have []string, required []string) bool {
	if len(required) == 0 {
		return true
	}

	set := map[string]bool{}
	for _, item := range have {
		set[item] = true
	}

	for _, item := range required {
		if !set[item] {
			return false
		}
	}

	return true
}

func matchesAny(have string, required []string) bool {
	if len(required) == 0 {
		return true
	}
	for _, item := range required {
		if have == item {
			return true
		}
	}
	return false
}

func priorityRank(priority string) int {
	switch priority {
	case "Recommended":
		return 0
	case "Engagement":
		return 1
	default:
		return 2
	}
}
