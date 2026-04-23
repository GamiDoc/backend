package recommendation

import (
	"errors"

	"github.com/gamidoc/backend/internal/wizard"
)

var ErrInvalidRecommendationStep = errors.New("invalid recommendation step")

type Service struct {
	engine *Engine
}

type Input struct {
	ForStep          int
	EvaluationGoals  []string
	ProjectType      string
	Participants     string
	DevelopmentStage string
	SelectedMethods  []string
}

func NewService(engine *Engine) *Service {
	return &Service{
		engine: engine,
	}
}

func (s *Service) Recommend(status wizard.Status, forStep int) (Result, error) {
	if forStep < 2 || forStep > 4 {
		return Result{}, ErrInvalidRecommendationStep
	}

	input := Input{
		ForStep: forStep,
	}

	if step1, ok := wizard.DecodeStep1(status); ok {
		input.EvaluationGoals = step1.EvaluationGoals
		input.ProjectType = step1.ProjectType
		input.Participants = step1.Participants
		input.DevelopmentStage = step1.DevelopmentStage
	}

	if step2, ok := wizard.DecodeStep2(status); ok {
		input.SelectedMethods = step2.SelectedMethods
	}

	return Result{
		ForStep:         forStep,
		Recommendations: s.engine.Recommend(input),
	}, nil
}
