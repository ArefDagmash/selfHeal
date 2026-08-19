package events

import (
	"strings"
	"testing"
)

func TestNewTestEvent_HasUniqueID(t *testing.T) {
	a := NewTestEvent("a", TypeAPI, StatusRunning)
	b := NewTestEvent("b", TypeUI, StatusPassed)
	if a.ID == "" || a.ID == b.ID {
		t.Fatalf("expected unique non-empty IDs, got %q and %q", a.ID, b.ID)
	}
	if a.Metadata == nil {
		t.Fatal("expected Metadata map to be initialised")
	}
}

func TestString_EachStatusReadable(t *testing.T) {
	cases := []struct {
		name     string
		testType TestType
		status   Status
	}{
		{"api_ok", TypeAPI, StatusPassed},
		{"api_bad", TypeAPI, StatusFailed},
		{"ui_loading", TypeUI, StatusRunning},
		{"ui_healed", TypeUI, StatusHealed},
		{"ui_retry", TypeUI, StatusRetrying},
	}

	for _, c := range cases {
		e := NewTestEvent(c.name, c.testType, c.status)
		e.DurationMs = 42
		if c.status == StatusFailed {
			e.ErrorMessage = "expected 200 got 500"
		}
		out := e.String()
		t.Logf("status=%s -> %s", c.status, out)
		if !strings.Contains(out, string(c.status)) {
			t.Errorf("status %s missing from output: %q", c.status, out)
		}
		if !strings.Contains(out, c.name) {
			t.Errorf("test name %s missing from output: %q", c.name, out)
		}
	}
}
