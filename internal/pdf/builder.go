package pdf

import (
	"time"

	"github.com/yifen9/gamidoc-backend/internal/project"
	"github.com/yifen9/gamidoc-backend/internal/recommendation"
	"github.com/yifen9/gamidoc-backend/internal/session"
	"github.com/yifen9/gamidoc-backend/internal/wizard"
)

type Builder struct{}

func NewBuilder() *Builder {
	return &Builder{}
}

func (b *Builder) BuildFromProject(item project.Project, recs []recommendation.Recommendation) (PlanData, error) {
	return b.build(item.Name, item.CreatedAt, item.Wizard, recs)
}

func (b *Builder) BuildFromSession(item session.Session, recs []recommendation.Recommendation) (PlanData, error) {
	return b.build("Anonymous Evaluation Plan", item.CreatedAt, item.Wizard, recs)
}

func (b *Builder) build(title string, createdAt time.Time, status wizard.Status, recs []recommendation.Recommendation) (PlanData, error) {
	var evaluationGoals []string
	var selectedMethods []string
	var selectedInstruments []string
	var nextSteps []string

	if step1, ok := wizard.DecodeStep1(status); ok {
		evaluationGoals = step1.EvaluationGoals
	}

	if step2, ok := wizard.DecodeStep2(status); ok {
		selectedMethods = step2.SelectedMethods
	}

	if step3, ok := wizard.DecodeStep3(status); ok {
		selectedInstruments = step3.SelectedInstruments
	} else {
		for _, rec := range recs {
			selectedInstruments = append(selectedInstruments, rec.Name)
		}
	}

	if step4, ok := wizard.DecodeStep4(status); ok {
		nextSteps = step4.NextSteps
	}

	return PlanData{
		Title:                  title,
		Date:                   createdAt,
		EvaluationGoals:        evaluationGoals,
		SelectedMethods:        selectedMethods,
		RecommendedInstruments: selectedInstruments,
		NextSteps:              nextSteps,
	}, nil
}
