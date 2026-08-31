package design

import (
	"encoding/json"
	"strings"
)

type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (s *Service) SaveSpark(current Status, spark string) Status {
	current = withSections(current)
	current.Spark = strings.TrimSpace(spark)
	return current
}

func (s *Service) ApplyPrefill(current Status, prefill map[string]json.RawMessage) Status {
	current = withSections(current)
	for number := 1; number <= SectionCount; number++ {
		key := SectionKey(number)
		content, ok := prefill[key]
		if !ok || len(content) == 0 || !json.Valid(content) {
			continue
		}
		state := current.Sections[key]
		if len(state.Content) > 0 {
			continue
		}
		state.Content = content
		current.Sections[key] = state
	}
	return current
}

func (s *Service) ChoosePath(current Status, path string) (Status, error) {
	current = withSections(current)
	if path != PathExperienceFirst && path != PathMechanicsFirst {
		return Status{}, ErrInvalidPath
	}
	if current.Path != "" {
		return Status{}, ErrPathAlreadyChosen
	}
	if current.Cursor < 1 && !current.FirstPassDone {
		return Status{}, ErrSectionLocked
	}
	current.Path = path
	return current, nil
}

func (s *Service) SaveSection(current Status, number int, content json.RawMessage, complete *bool, skip bool) (Status, error) {
	current = withSections(current)
	if number < 1 || number > SectionCount {
		return Status{}, ErrInvalidSectionNumber
	}

	key := SectionKey(number)
	state := current.Sections[key]
	frontier := !current.FirstPassDone && !state.Visited

	if frontier {
		expected, err := s.nextSection(current)
		if err != nil {
			return Status{}, err
		}
		if number != expected {
			return Status{}, ErrSectionLocked
		}
	}

	state.Visited = true

	if !skip {
		if len(content) == 0 || !json.Valid(content) {
			return Status{}, ErrInvalidSectionData
		}
		state.Content = content
		if complete != nil {
			state.Complete = *complete
		}
	}

	current.Sections[key] = state

	if frontier {
		current.Cursor++
		if current.Cursor >= SectionCount {
			current.FirstPassDone = true
		}
	}

	return current, nil
}

func (s *Service) nextSection(current Status) (int, error) {
	if current.Cursor == 0 {
		return 1, nil
	}
	order := Order(current.Path)
	if order == nil {
		return 0, ErrPathNotChosen
	}
	return order[current.Cursor], nil
}

func (s *Service) Dashboard(current Status) (Dashboard, error) {
	current = withSections(current)
	if !current.FirstPassDone || !current.HasContent() {
		return Dashboard{}, ErrDashboardLocked
	}

	sections := make([]DashboardSection, 0, SectionCount)
	total := 0
	for number := 1; number <= SectionCount; number++ {
		state := current.Section(number)
		percent := SectionPercent(state)
		total += percent
		sections = append(sections, DashboardSection{
			SectionNumber: number,
			Name:          SectionName(number),
			Description:   SectionDescription(number),
			Status:        SectionStatus(state),
			Percent:       percent,
		})
	}

	return Dashboard{
		Sections:       sections,
		OverallPercent: total / SectionCount,
		FirstPassDone:  current.FirstPassDone,
		Path:           current.Path,
	}, nil
}

func withSections(current Status) Status {
	if current.Sections == nil {
		current.Sections = map[string]SectionState{}
	}
	return current
}
