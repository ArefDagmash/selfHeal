package runner

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"testforge/events"
)

// TestRunAPITest_FailurePath confirms a wrong expected status yields a failed
// event with an assertion error classification rather than panicking.
func TestRunAPITest_FailurePath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ev := RunAPITest("api_wrong_status", srv.URL, 500)
	if ev.Status != events.StatusFailed {
		t.Fatalf("expected failed, got %s (%s)", ev.Status, ev.ErrorMessage)
	}
	if ev.ErrorType != events.ErrAssertion {
		t.Fatalf("expected assertion error type, got %s", ev.ErrorType)
	}
	if ev.ErrorMessage == "" {
		t.Fatal("expected an error message on failure")
	}
	t.Logf("got expected failure: %s", ev.String())
}

// TestRunAPITest_NetworkError confirms a refused connection is classified as a
// network error, not an assertion error.
func TestRunAPITest_NetworkError(t *testing.T) {
	ev := RunAPITest("api_dead_host", "http://localhost:1", 200)
	if ev.Status != events.StatusFailed {
		t.Fatalf("expected failed, got %s", ev.Status)
	}
	if ev.ErrorType != events.ErrNetwork {
		t.Fatalf("expected network error type, got %s", ev.ErrorType)
	}
	t.Logf("got expected failure: %s", ev.String())
}

// TestRunUITest_SelectorNotFound confirms a selector that doesn't exist on the
// page produces a failed event classified as selector_not_found (the seed for
// later self-healing).
func TestRunUITest_SelectorNotFound(t *testing.T) {
	ev := RunUITest("example_missing_element", "https://example.com", "#this-id-does-not-exist-xyz")
	if ev.Status != events.StatusFailed {
		t.Fatalf("expected failed, got %s (%s)", ev.Status, ev.ErrorMessage)
	}
	if ev.ErrorType != events.ErrSelectorNotFound {
		t.Fatalf("expected selector_not_found, got %s", ev.ErrorType)
	}
	t.Logf("got expected failure: %s", ev.String())
}

// TestRunWithRetry_RetriesOnce confirms a persistently failing test is retried
// exactly once and still ends failed.
func TestRunWithRetry_RetriesOnce(t *testing.T) {
	tc := TestCase{Name: "retry_demo", Type: events.TypeUI,
		URL: "https://example.com", Selector: "#nope"}
	final := RunWithRetry(tc, 1, nil)
	if final.Status != events.StatusFailed {
		t.Fatalf("expected final status failed, got %s", final.Status)
	}
	t.Logf("final after retry: %s", final.String())
}

// shopPage serves a tiny HTML page whose button class was renamed from the
// old "submit" to "submit-btn" — the drift self-healing should recover from.
func shopPage() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintln(w, `<!doctype html><html><body>`)
		fmt.Fprintln(w, `<button class="submit-btn" id="buy">Buy now</button>`)
		fmt.Fprintln(w, `</body></html>`)
	}))
}

// TestRunUITest_HealsToClass confirms a missing selector whose class was renamed
// is auto-healed to the surviving class.
func TestRunUITest_HealsToClass(t *testing.T) {
	srv := shopPage()
	defer srv.Close()
	ev := RunUITest("shop_heal", srv.URL, ".submit")
	if ev.Status != events.StatusHealed {
		t.Fatalf("expected healed, got %s (%s)", ev.Status, ev.ErrorMessage)
	}
	if ev.Metadata["old_selector"] != ".submit" || ev.Metadata["new_selector"] != ".submit-btn" {
		t.Fatalf("unexpected heal metadata: %+v", ev.Metadata)
	}
	t.Logf("healed: %s -> %s", ev.Metadata["old_selector"], ev.Metadata["new_selector"])
}

// TestRunUITest_NoHealWhenNoMatch confirms a selector with no plausible match
// stays a permanent selector_not_found failure.
func TestRunUITest_NoHealWhenNoMatch(t *testing.T) {
	srv := shopPage()
	defer srv.Close()
	ev := RunUITest("shop_noheal", srv.URL, ".zzz-no-such-class")
	if ev.Status != events.StatusFailed || ev.ErrorType != events.ErrSelectorNotFound {
		t.Fatalf("expected failed/selector_not_found, got %s/%s", ev.Status, ev.ErrorType)
	}
	t.Logf("correctly not healed: %s", ev.String())
}
