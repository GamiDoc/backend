package recommendation

type Rule struct {
	ForStep                   int              `json:"forStep"`
	RequiredEvaluationGoals   []string         `json:"requiredEvaluationGoals"`
	RequiredProjectTypes      []string         `json:"requiredProjectTypes"`
	RequiredParticipants      []string         `json:"requiredParticipants"`
	RequiredDevelopmentStages []string         `json:"requiredDevelopmentStages"`
	RequiredMethods           []string         `json:"requiredMethods"`
	Recommendations           []Recommendation `json:"recommendations"`
}
