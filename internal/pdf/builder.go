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

func (b *Builder) BuildFromProject(item project.Project, methodRecs []recommendation.Recommendation, instrumentRecs []recommendation.Recommendation) (PlanData, error) {
	return b.build(item.Name, item.CreatedAt, item.Wizard, methodRecs, instrumentRecs)
}

func (b *Builder) BuildFromSession(item session.Session, methodRecs []recommendation.Recommendation, instrumentRecs []recommendation.Recommendation) (PlanData, error) {
	return b.build("Anonymous Evaluation Plan", item.CreatedAt, item.Wizard, methodRecs, instrumentRecs)
}

func (b *Builder) build(title string, createdAt time.Time, status wizard.Status, methodRecs []recommendation.Recommendation, instrumentRecs []recommendation.Recommendation) (PlanData, error) {
	var evaluationGoals []string
	var projectType string
	var participants string
	var developmentStage string
	var selectedMethods []MethodEntry
	var selectedInstruments []InstrumentEntry
	var nextSteps []string
	var notes string

	if step1, ok := wizard.DecodeStep1(status); ok {
		evaluationGoals = step1.EvaluationGoals
		projectType = step1.ProjectType
		participants = step1.Participants
		developmentStage = step1.DevelopmentStage
	}

	if step2, ok := wizard.DecodeStep2(status); ok {
		selectedMethods = buildMethodEntries(step2.SelectedMethods, methodRecs)
	}

	if step3, ok := wizard.DecodeStep3(status); ok {
		selectedInstruments = buildInstrumentEntries(step3.SelectedInstruments, instrumentRecs)
	} else {
		selectedInstruments = buildInstrumentEntries(nil, instrumentRecs)
	}

	if step4, ok := wizard.DecodeStep4(status); ok {
		nextSteps = step4.NextSteps
		notes = step4.Notes
	}

	return PlanData{
		Title:               title,
		Date:                createdAt,
		EvaluationGoals:     evaluationGoals,
		ProjectType:         projectType,
		Participants:        participants,
		DevelopmentStage:    developmentStage,
		SelectedMethods:     selectedMethods,
		SelectedInstruments: selectedInstruments,
		NextSteps:           nextSteps,
		Notes:               notes,
	}, nil
}

func buildMethodEntries(selected []string, recs []recommendation.Recommendation) []MethodEntry {
	index := map[string]recommendation.Recommendation{}
	for _, rec := range recs {
		index[rec.ID] = rec
	}

	var result []MethodEntry
	for _, id := range selected {
		rec, ok := index[id]
		if ok {
			result = append(result, MethodEntry{
				Name:        rec.Name,
				Description: rec.Description,
				Priority:    rec.Priority,
				Rationale:   rec.Rationale,
			})
			continue
		}

		result = append(result, MethodEntry{
			Name: id,
		})
	}

	return result
}

func buildInstrumentEntries(selected []string, recs []recommendation.Recommendation) []InstrumentEntry {
	indexByName := map[string]recommendation.Recommendation{}
	indexByID := map[string]recommendation.Recommendation{}
	for _, rec := range recs {
		indexByName[rec.Name] = rec
		indexByID[rec.ID] = rec
	}

	var result []InstrumentEntry

	if len(selected) == 0 {
		for _, rec := range recs {
			result = append(result, InstrumentEntry{
				Name:        rec.Name,
				Description: rec.Description,
				Priority:    rec.Priority,
				Rationale:   rec.Rationale,
			})
		}
		return result
	}

	for _, item := range selected {
		if rec, ok := indexByName[item]; ok {
			result = append(result, InstrumentEntry{
				Name:        rec.Name,
				Description: rec.Description,
				Priority:    rec.Priority,
				Rationale:   rec.Rationale,
			})
			continue
		}
		if rec, ok := indexByID[item]; ok {
			result = append(result, InstrumentEntry{
				Name:        rec.Name,
				Description: rec.Description,
				Priority:    rec.Priority,
				Rationale:   rec.Rationale,
			})
			continue
		}
		result = append(result, InstrumentEntry{
			Name: item,
		})
	}

	return result
}
