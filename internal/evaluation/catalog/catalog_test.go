package catalog

import "testing"

func TestCatalogLoadsAndMatches(t *testing.T) {
	c := Get()
	if len(c.Goals) == 0 || len(c.Methods) == 0 || len(c.Instruments) == 0 || len(c.Stages) == 0 {
		t.Fatalf("empty catalog: %d goals %d methods %d instruments", len(c.Goals), len(c.Methods), len(c.Instruments))
	}
	if got, ok := Match(c.MethodNames(), "think aloud TESTING"); !ok || got != "Think-aloud testing" {
		t.Fatalf("match: %q %v", got, ok)
	}
	if _, ok := Match(c.GoalNames(), "Made up goal"); ok {
		t.Fatal("unknown goal matched")
	}
	if _, ok := c.Method("think-aloud"); !ok {
		t.Fatal("method by id")
	}
	if c.Text() == "" {
		t.Fatal("empty text")
	}
}
