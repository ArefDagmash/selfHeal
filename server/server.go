package server

import (
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"

	"testforge/events"
)

// Hub is a tiny pub-sub broker: dashboard clients connect over WebSocket, and
// the test runner calls Broadcast for every TestEvent so all clients see it
// live, the instant it happens.
type Hub struct {
	mu       sync.Mutex
	clients  map[*websocket.Conn]bool
	upgrader websocket.Upgrader

	// NewClient fires once per successful connection (a page load or a
	// refresh) so the runner can drive "one run per visit" instead of
	// looping on a timer regardless of whether anyone's watching. Buffered
	// to 1 with a non-blocking send: if a run is already queued, further
	// connections just ride along with it rather than piling up.
	NewClient chan struct{}
}

// NewHub creates an empty hub. CheckOrigin is permissive so the React dev
// server (a different origin/port) can connect in development.
func NewHub() *Hub {
	return &Hub{
		clients: make(map[*websocket.Conn]bool),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		NewClient: make(chan struct{}, 1),
	}
}

// Broadcast sends a single event as JSON to every connected client. Dead
// connections are dropped as we discover them.
func (h *Hub) Broadcast(ev events.TestEvent) {
	h.broadcastJSON(ev)
}

// BroadcastReport sends a whole-run report as JSON to every connected client.
// It shares the same socket and client set as Broadcast; clients tell the two
// message shapes apart by the "kind" field.
func (h *Hub) BroadcastReport(r events.RunReport) {
	h.broadcastJSON(r)
}

// BroadcastBoundary sends a RunBoundary marker as JSON to every connected
// client, signalling that every TestEvent for a run has been sent.
func (h *Hub) BroadcastBoundary(b events.RunBoundary) {
	h.broadcastJSON(b)
}

func (h *Hub) broadcastJSON(v any) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for conn := range h.clients {
		if err := conn.WriteJSON(v); err != nil {
			log.Printf("ws broadcast write error: %v", err)
			conn.Close()
			delete(h.clients, conn)
		}
	}
}

// ClientCount returns how many dashboards are currently connected.
func (h *Hub) ClientCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.clients)
}

// ServeWS upgrades an HTTP request to a WebSocket and registers the client.
// It blocks for the life of the connection, removing the client on disconnect.
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws upgrade failed: %v", err)
		return
	}
	h.mu.Lock()
	h.clients[conn] = true
	h.mu.Unlock()
	log.Println("dashboard client connected")

	select {
	case h.NewClient <- struct{}{}:
	default: // a run is already queued; this connection will see it too
	}

	// Read loop: we don't expect client messages, but reading lets us detect
	// a dropped connection so we can clean it up.
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			h.mu.Lock()
			delete(h.clients, conn)
			h.mu.Unlock()
			log.Println("dashboard client disconnected")
			conn.Close()
			return
		}
	}
}

// Start serves the /ws endpoint on addr until the process exits.
func (h *Hub) Start(addr string) error {
	http.HandleFunc("/ws", h.ServeWS)
	log.Printf("websocket server listening on %s/ws", addr)
	return http.ListenAndServe(addr, nil)
}
