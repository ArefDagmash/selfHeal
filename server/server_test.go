package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"testforge/events"
)

// TestHubBroadcastLive confirms a client connected to /ws receives an event as
// JSON the moment it is broadcast, not batched at the end.
func TestHubBroadcastLive(t *testing.T) {
	hub := NewHub()
	srv := httptest.NewServer(http.HandlerFunc(hub.ServeWS))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Wait until the hub has registered the client.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if hub.ClientCount() == 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if hub.ClientCount() != 1 {
		t.Fatal("client was not registered with the hub")
	}

	ev := events.NewTestEvent("live_check", events.TypeAPI, events.StatusPassed)
	ev.DurationMs = 7
	hub.Broadcast(ev)

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var got events.TestEvent
	if err := conn.ReadJSON(&got); err != nil {
		t.Fatalf("read: %v", err)
	}
	if got.TestName != "live_check" || got.Status != events.StatusPassed {
		t.Fatalf("unexpected event: %+v", got)
	}
	t.Logf("received live event: %s", got.String())
}

// TestHubBroadcastReport confirms a RunReport goes out over the same socket
// as TestEvents, tagged with kind="report" so a client can tell them apart.
func TestHubBroadcastReport(t *testing.T) {
	hub := NewHub()
	srv := httptest.NewServer(http.HandlerFunc(hub.ServeWS))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if hub.ClientCount() == 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if hub.ClientCount() != 1 {
		t.Fatal("client was not registered with the hub")
	}

	hub.BroadcastReport(events.NewRunReport("2 tests self-healed, 1 needs a human.", 4, 1, 2))

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var got events.RunReport
	if err := conn.ReadJSON(&got); err != nil {
		t.Fatalf("read: %v", err)
	}
	if got.Kind != "report" || got.Passed != 4 || got.Failed != 1 || got.Healed != 2 {
		t.Fatalf("unexpected report: %+v", got)
	}
}

// TestHubNewClientSignalsOnConnect confirms each successful connection fires
// NewClient exactly once, which is what drives "one run per dashboard visit"
// in the runner instead of a timer loop.
func TestHubNewClientSignalsOnConnect(t *testing.T) {
	hub := NewHub()
	srv := httptest.NewServer(http.HandlerFunc(hub.ServeWS))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	select {
	case <-hub.NewClient:
	case <-time.After(2 * time.Second):
		t.Fatal("expected a NewClient signal after connecting")
	}

	select {
	case <-hub.NewClient:
		t.Fatal("did not expect a second NewClient signal without a second connection")
	case <-time.After(100 * time.Millisecond):
	}
}

// TestHubNewClientSignalDoesNotBlockConnect confirms a second connection
// arriving while a signal is already queued (buffer full) doesn't hang
// ServeWS — the non-blocking send in ServeWS should just drop it.
func TestHubNewClientSignalDoesNotBlockConnect(t *testing.T) {
	hub := NewHub()
	srv := httptest.NewServer(http.HandlerFunc(hub.ServeWS))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"

	conn1, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial 1: %v", err)
	}
	defer conn1.Close()

	// Don't drain hub.NewClient yet, so its buffer (size 1) stays full for
	// the second connection below.
	done := make(chan struct{})
	go func() {
		conn2, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Errorf("dial 2: %v", err)
			close(done)
			return
		}
		defer conn2.Close()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("second connection did not complete — ServeWS likely blocked on a full NewClient channel")
	}
}
