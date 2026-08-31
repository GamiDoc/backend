package design

import (
	"encoding/json"
	"errors"
	"testing"
)

func content(t *testing.T, value string) json.RawMessage {
	t.Helper()
	raw := json.RawMessage(value)
	if !json.Valid(raw) {
		t.Fatalf("invalid test content: %s", value)
	}
	return raw
}

func traverse(t *testing.T, service *Service, path string) Status {
	t.Helper()
	status := NewInitialStatus()

	status, err := service.SaveSection(status, 1, content(t, `{"draft":"context"}`), nil, false)
	if err != nil {
		t.Fatal(err)
	}

	status, err = service.ChoosePath(status, path)
	if err != nil {
		t.Fatal(err)
	}

	order := Order(path)
	for _, number := range order[1:] {
		status, err = service.SaveSection(status, number, nil, nil, true)
		if err != nil {
			t.Fatal(err)
		}
	}

	return status
}

func TestFirstPassRequiresSectionOne(t *testing.T) {
	service := NewService()
	status := NewInitialStatus()

	if _, err := service.SaveSection(status, 2, content(t, `{"a":1}`), nil, false); !errors.Is(err, ErrSectionLocked) {
		t.Fatalf("expected ErrSectionLocked, got %v", err)
	}
}

func TestPathGate(t *testing.T) {
	service := NewService()
	status := NewInitialStatus()

	if _, err := service.ChoosePath(status, PathExperienceFirst); !errors.Is(err, ErrSectionLocked) {
		t.Fatalf("expected ErrSectionLocked, got %v", err)
	}

	status, err := service.SaveSection(status, 1, content(t, `{"draft":"context"}`), nil, false)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := service.SaveSection(status, 2, content(t, `{"a":1}`), nil, false); !errors.Is(err, ErrPathNotChosen) {
		t.Fatalf("expected ErrPathNotChosen, got %v", err)
	}

	status, err = service.ChoosePath(status, PathMechanicsFirst)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := service.ChoosePath(status, PathExperienceFirst); !errors.Is(err, ErrPathAlreadyChosen) {
		t.Fatalf("expected ErrPathAlreadyChosen, got %v", err)
	}

	if _, err := service.ChoosePath(NewInitialStatus(), "C"); !errors.Is(err, ErrInvalidPath) {
		t.Fatalf("expected ErrInvalidPath, got %v", err)
	}
}

func TestMechanicsFirstOrder(t *testing.T) {
	service := NewService()
	status := NewInitialStatus()

	status, err := service.SaveSection(status, 1, content(t, `{"draft":"context"}`), nil, false)
	if err != nil {
		t.Fatal(err)
	}

	status, err = service.ChoosePath(status, PathMechanicsFirst)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := service.SaveSection(status, 2, content(t, `{"a":1}`), nil, false); !errors.Is(err, ErrSectionLocked) {
		t.Fatalf("expected ErrSectionLocked, got %v", err)
	}

	status, err = service.SaveSection(status, 4, content(t, `{"core":"points"}`), boolPtr(true), false)
	if err != nil {
		t.Fatal(err)
	}

	if status.Cursor != 2 {
		t.Fatalf("expected cursor 2, got %d", status.Cursor)
	}
}

func TestSkipAdvancesWithoutContent(t *testing.T) {
	service := NewService()
	status := NewInitialStatus()

	status, err := service.SaveSection(status, 1, nil, nil, true)
	if err != nil {
		t.Fatal(err)
	}

	if status.Cursor != 1 {
		t.Fatalf("expected cursor 1, got %d", status.Cursor)
	}
	if len(status.Section(1).Content) != 0 {
		t.Fatal("expected no content after skip")
	}
	if !status.Section(1).Visited {
		t.Fatal("expected section marked visited")
	}
}

func TestTraversalUnlocksFreeNavigation(t *testing.T) {
	service := NewService()
	status := traverse(t, service, PathExperienceFirst)

	if !status.FirstPassDone {
		t.Fatal("expected first pass done")
	}

	status, err := service.SaveSection(status, 6, content(t, `{"impact":"co2"}`), boolPtr(true), false)
	if err != nil {
		t.Fatal(err)
	}
	if SectionStatus(status.Section(6)) != SectionStatusComplete {
		t.Fatal("expected section 6 complete")
	}
}

func TestInvalidSectionData(t *testing.T) {
	service := NewService()
	status := NewInitialStatus()

	if _, err := service.SaveSection(status, 1, json.RawMessage(`{invalid`), nil, false); !errors.Is(err, ErrInvalidSectionData) {
		t.Fatalf("expected ErrInvalidSectionData, got %v", err)
	}
}

func TestDashboardGateAndPercentages(t *testing.T) {
	service := NewService()

	if _, err := service.Dashboard(NewInitialStatus()); !errors.Is(err, ErrDashboardLocked) {
		t.Fatalf("expected ErrDashboardLocked, got %v", err)
	}

	status := traverse(t, service, PathExperienceFirst)
	status, err := service.SaveSection(status, 2, content(t, `{"timeline":"weekly"}`), boolPtr(true), false)
	if err != nil {
		t.Fatal(err)
	}

	dashboard, err := service.Dashboard(status)
	if err != nil {
		t.Fatal(err)
	}

	if len(dashboard.Sections) != SectionCount {
		t.Fatalf("expected %d sections, got %d", SectionCount, len(dashboard.Sections))
	}
	if dashboard.Sections[0].Status != SectionStatusInProgress {
		t.Fatalf("expected section 1 in progress, got %s", dashboard.Sections[0].Status)
	}
	if dashboard.Sections[1].Status != SectionStatusComplete {
		t.Fatalf("expected section 2 complete, got %s", dashboard.Sections[1].Status)
	}
	if dashboard.OverallPercent != (50+100)/SectionCount {
		t.Fatalf("unexpected overall percent %d", dashboard.OverallPercent)
	}
}

func TestApplyPrefillFillsOnlyEmptySections(t *testing.T) {
	service := NewService()
	status := NewInitialStatus()

	status, err := service.SaveSection(status, 1, content(t, `{"draft":"mine"}`), nil, false)
	if err != nil {
		t.Fatal(err)
	}

	status = service.ApplyPrefill(status, map[string]json.RawMessage{
		"1": content(t, `{"draft":"generated"}`),
		"2": content(t, `{"draft":"generated"}`),
	})

	if string(status.Section(1).Content) != `{"draft":"mine"}` {
		t.Fatal("expected user content preserved")
	}
	if string(status.Section(2).Content) != `{"draft":"generated"}` {
		t.Fatal("expected empty section prefilled")
	}
}

func TestSaveSparkTrims(t *testing.T) {
	service := NewService()
	status := service.SaveSpark(NewInitialStatus(), "  a gamified commuting app  ")

	if status.Spark != "a gamified commuting app" {
		t.Fatalf("unexpected spark %q", status.Spark)
	}
}

func boolPtr(value bool) *bool {
	return &value
}

func TestFirstPassAllowsResaveOfVisitedSections(t *testing.T) {
	service := NewService()
	status := NewInitialStatus()

	status, err := service.SaveSection(status, 1, content(t, `{"draft":"v1"}`), nil, false)
	if err != nil {
		t.Fatal(err)
	}

	status, err = service.SaveSection(status, 1, content(t, `{"draft":"v2"}`), nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if status.Cursor != 1 {
		t.Fatalf("expected cursor to stay at 1, got %d", status.Cursor)
	}
	if string(status.Section(1).Content) != `{"draft":"v2"}` {
		t.Fatal("expected re-save to update content")
	}

	if _, err := service.SaveSection(status, 3, content(t, `{"a":1}`), nil, false); !errors.Is(err, ErrPathNotChosen) {
		t.Fatalf("expected ErrPathNotChosen for the frontier, got %v", err)
	}
}

func TestCompletePointerPreservesFlag(t *testing.T) {
	service := NewService()
	status := traverse(t, service, PathExperienceFirst)

	status, err := service.SaveSection(status, 2, content(t, `{"a":1}`), boolPtr(true), false)
	if err != nil {
		t.Fatal(err)
	}

	status, err = service.SaveSection(status, 2, content(t, `{"a":2}`), nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if !status.Section(2).Complete {
		t.Fatal("expected omitted complete flag to preserve completion")
	}

	status, err = service.SaveSection(status, 2, content(t, `{"a":3}`), boolPtr(false), false)
	if err != nil {
		t.Fatal(err)
	}
	if status.Section(2).Complete {
		t.Fatal("expected explicit false to clear completion")
	}
}
