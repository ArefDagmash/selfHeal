package runner

import (
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"testforge/events"
)

// RunAPITest performs a single HTTP GET against url and returns a TestEvent
// whose status depends on whether the response status code matches
// expectedStatus. This is the simplest possible API check: "did the endpoint
// return the status I expected?"
func RunAPITest(name, url string, expectedStatus int) events.TestEvent {
	start := time.Now()
	ev := events.NewTestEvent(name, events.TypeAPI, events.StatusRunning)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	duration := time.Since(start)
	ev.DurationMs = duration.Milliseconds()

	if err != nil {
		ev.Status = events.StatusFailed
		ev.ErrorMessage = "request failed: " + err.Error()
		// A client timeout is a timeout; anything else (DNS, refused, TLS) is
		// a network-level failure.
		if isTimeoutError(err) {
			ev.ErrorType = events.ErrTimeout
		} else {
			ev.ErrorType = events.ErrNetwork
		}
		return ev
	}
	defer resp.Body.Close()
	// Drain so the connection can be reused; we don't inspect the body.
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode == expectedStatus {
		ev.Status = events.StatusPassed
		ev.ErrorType = events.ErrNone
	} else {
		ev.Status = events.StatusFailed
		ev.ErrorType = events.ErrAssertion
		ev.ErrorMessage = "expected status " + strconv.Itoa(expectedStatus) +
			" but got " + strconv.Itoa(resp.StatusCode)
	}
	return ev
}

// isTimeoutError reports whether err looks like an HTTP client timeout rather
// than a hard network failure.
func isTimeoutError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "Client.Timeout") ||
		strings.Contains(msg, "context deadline exceeded") ||
		strings.Contains(msg, "i/o timeout")
}
