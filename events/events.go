package events

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// TestType describes what kind of test produced the event.
type TestType string

const (
	TypeAPI TestType = "api"
	TypeUI  TestType = "ui"
)

// Status is the lifecycle state of a test as seen by the dashboard.
type Status string

const (
	StatusRunning  Status = "running"
	StatusPassed   Status = "passed"
	StatusFailed   Status = "failed"
	StatusHealed   Status = "healed"
	StatusRetrying Status = "retrying"
)

// ErrorType classifies *why* a test failed, so later phases (self-healing, AI
// suggestions) can decide how to react. A passing test carries "none".
type ErrorType string

const (
	ErrNone             ErrorType = "none"
	ErrTimeout          ErrorType = "timeout"
	ErrAssertion        ErrorType = "assertion"
	ErrNetwork          ErrorType = "network"
	ErrSelectorNotFound ErrorType = "selector_not_found"
)

// TestEvent is the single contract shared by the runner, storage, and
// dashboard. Every part of the system produces or consumes this shape, so
// changing it ripples everywhere — define it once and keep it stable.
type TestEvent struct {
	ID           string            `json:"id"`
	TestName     string            `json:"test_name"`
	Type         TestType          `json:"type"`
	Status       Status            `json:"status"`
	Timestamp    time.Time         `json:"timestamp"`
	DurationMs   int64             `json:"duration_ms"`
	ErrorType    ErrorType         `json:"error_type,omitempty"`
	ErrorMessage string            `json:"error_message,omitempty"`
	Selector     string            `json:"selector,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// RunReport is a whole-run recap, broadcast once after a suite finishes (as
// opposed to TestEvent, which fires per test). It's a different JSON shape on
// the wire — Kind is always "report" — so a client reading the same socket
// can tell the two apart without a second connection or endpoint.
type RunReport struct {
	Kind      string    `json:"kind"`
	ID        string    `json:"id"`
	Summary   string    `json:"summary"`
	Passed    int       `json:"passed"`
	Failed    int       `json:"failed"`
	Healed    int       `json:"healed"`
	Timestamp time.Time `json:"timestamp"`
}

// NewRunReport builds a RunReport with a fresh UUID and the current timestamp.
func NewRunReport(summary string, passed, failed, healed int) RunReport {
	return RunReport{
		Kind:      "report",
		ID:        uuid.NewString(),
		Summary:   summary,
		Passed:    passed,
		Failed:    failed,
		Healed:    healed,
		Timestamp: time.Now(),
	}
}

// RunBoundary marks "every TestEvent for this run has been sent" — broadcast
// right after the suite finishes, before the (possibly slow) AI suggestion
// and summary calls. Test durations vary a lot (headless Chrome cold starts,
// retries), so a client can't reliably infer "this run is done" just from a
// quiet gap in the event stream; this gives it an explicit signal instead.
type RunBoundary struct {
	Kind  string `json:"kind"` // always "run_end"
	ID    string `json:"id"`
	Count int    `json:"count"`
}

// NewRunBoundary builds a RunBoundary with a fresh UUID.
func NewRunBoundary(count int) RunBoundary {
	return RunBoundary{Kind: "run_end", ID: uuid.NewString(), Count: count}
}

// NewTestEvent builds a TestEvent with a fresh UUID and the current timestamp.
// Callers fill in the test-specific fields after construction.
func NewTestEvent(testName string, testType TestType, status Status) TestEvent {
	return TestEvent{
		ID:        uuid.NewString(),
		TestName:  testName,
		Type:      testType,
		Status:    status,
		Timestamp: time.Now(),
		Metadata:  map[string]string{},
	}
}

// String renders a single-line, human-readable summary used by the early
// stdout logging before the dashboard exists.
func (e TestEvent) String() string {
	base := fmt.Sprintf("[%s] %s (%s) — %s in %dms",
		e.statusGlyph(), e.TestName, e.Type, e.Status, e.DurationMs)
	if e.Status == StatusFailed && e.ErrorType != ErrNone {
		base += fmt.Sprintf(" — %s", e.ErrorType)
	}
	if e.ErrorMessage != "" {
		base += fmt.Sprintf(" — %s", e.ErrorMessage)
	}
	return base
}

// statusGlyph gives a tiny visual marker per status for the log line.
func (e TestEvent) statusGlyph() string {
	switch e.Status {
	case StatusPassed:
		return "✓"
	case StatusFailed:
		return "✗"
	case StatusHealed:
		return "✚"
	case StatusRetrying:
		return "↻"
	default:
		return "•"
	}
}
