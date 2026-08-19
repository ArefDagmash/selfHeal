package runner

import (
	"fmt"

	"testforge/events"
)

// TestCase is a single declared test. It carries the common fields (name, type)
// plus the config each runner needs. API tests use URL + ExpectedStatus; UI
// tests use URL + Selector. One struct for both keeps things simple instead of
// building a type hierarchy.
type TestCase struct {
	Name           string
	Type           events.TestType
	URL            string
	ExpectedStatus int    // API only
	Selector       string // UI only
}

// Suite is an ordered collection of test cases. Run executes them in sequence
// and returns every TestEvent produced, in order. If Emit is set, each event
// (including transient "retrying" events) is handed to it the moment it is
// produced — that's how the live dashboard gets a real-time feed.
type Suite struct {
	Cases []TestCase
	Emit  func(events.TestEvent)
}

// Run walks the suite one case at a time, dispatches to the right runner by
// Type, and accumulates events. Each test is wrapped in RunWithRetry so a
// transient failure gets one automatic retry. It logs a start and end banner so
// the console shows overall progress even before the dashboard exists.
func (s Suite) Run() []events.TestEvent {
	fmt.Printf("=== suite start: %d test(s) ===\n", len(s.Cases))
	var all []events.TestEvent

	passed, failed := 0, 0
	for _, tc := range s.Cases {
		ev := RunWithRetry(tc, 1, s.Emit)
		all = append(all, ev)
		fmt.Println(ev.String())
		// A healed test recovered from a broken selector, so it counts as a
		// pass for the run tally (matching SaveRun's logic).
		if ev.Status == events.StatusPassed || ev.Status == events.StatusHealed {
			passed++
		} else {
			failed++
		}
	}

	fmt.Printf("=== suite end: %d passed, %d failed ===\n", passed, failed)
	return all
}

// dispatch runs a single test case once, choosing the runner by type.
func dispatch(tc TestCase) events.TestEvent {
	switch tc.Type {
	case events.TypeAPI:
		return RunAPITest(tc.Name, tc.URL, tc.ExpectedStatus)
	case events.TypeUI:
		return RunUITest(tc.Name, tc.URL, tc.Selector)
	default:
		ev := events.NewTestEvent(tc.Name, tc.Type, events.StatusFailed)
		ev.ErrorMessage = "unknown test type"
		return ev
	}
}

// RunWithRetry runs a test case, and if it fails, emits a "retrying" event and
// retries up to maxRetries times before giving up. It returns the final event
// (the outcome after retrying). Every event produced — the retrying event and
// the final event — is passed to emit (if non-nil) as soon as it exists, so a
// connected dashboard can show the test flip to "retrying" and then resolve.
func RunWithRetry(tc TestCase, maxRetries int, emit func(events.TestEvent)) events.TestEvent {
	ev := dispatch(tc)
	for attempt := 1; ev.Status == events.StatusFailed && attempt <= maxRetries; attempt++ {
		retryEv := events.NewTestEvent(tc.Name, tc.Type, events.StatusRetrying)
		retryEv.Selector = tc.Selector
		retryEv.ErrorMessage = ev.ErrorMessage
		retryEv.DurationMs = ev.DurationMs
		if emit != nil {
			emit(retryEv)
		}
		fmt.Printf("[RETRYING] %s — %s (attempt %d/%d)\n",
			tc.Name, ev.ErrorType, attempt, maxRetries)
		fmt.Println(retryEv.String())
		ev = dispatch(tc)
	}
	if emit != nil {
		emit(ev)
	}
	return ev
}
