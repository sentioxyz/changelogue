package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
)

// SSEEvent represents a server-sent event with an event type and data payload.
type SSEEvent struct {
	Event string
	Data  string
}

// Broadcaster manages SSE client connections and fans out events to all subscribers.
type Broadcaster struct {
	mu      sync.RWMutex
	clients map[chan SSEEvent]struct{}
}

// NewBroadcaster returns a new Broadcaster ready for use.
func NewBroadcaster() *Broadcaster {
	return &Broadcaster{
		clients: make(map[chan SSEEvent]struct{}),
	}
}

// Subscribe registers a new client and returns a buffered channel to receive events.
func (b *Broadcaster) Subscribe() chan SSEEvent {
	ch := make(chan SSEEvent, 64)
	b.mu.Lock()
	b.clients[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

// Unsubscribe removes a client and closes its channel.
func (b *Broadcaster) Unsubscribe(ch chan SSEEvent) {
	b.mu.Lock()
	if _, ok := b.clients[ch]; ok {
		delete(b.clients, ch)
		close(ch)
	}
	b.mu.Unlock()
}

// Send fans out an event to all subscribed clients.
// Slow clients with full buffers are skipped (non-blocking send).
func (b *Broadcaster) Send(evt SSEEvent) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.clients {
		select {
		case ch <- evt:
		default:
			// Skip slow clients to avoid blocking the broadcaster.
		}
	}
}

// EventsHandler implements HTTP handlers for the SSE /events stream.
type EventsHandler struct {
	broadcaster *Broadcaster
}

// NewEventsHandler returns a new EventsHandler.
func NewEventsHandler(broadcaster *Broadcaster) *EventsHandler {
	return &EventsHandler{broadcaster: broadcaster}
}

// Stream handles GET /events — opens an SSE connection and streams events to the client.
func (h *EventsHandler) Stream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		RespondError(w, r, http.StatusInternalServerError, "internal_error", "Streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := h.broadcaster.Subscribe()
	defer h.broadcaster.Unsubscribe(ch)

	// Send initial connection event.
	fmt.Fprintf(w, "event: connected\ndata: {}\n\n")
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case evt, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", evt.Event, evt.Data)
			flusher.Flush()
		}
	}
}

// ListenForNotifications listens on the PostgreSQL release_events channel
// and broadcasts received notifications to all SSE clients.
func ListenForNotifications(ctx context.Context, pool *pgxpool.Pool, broadcaster *Broadcaster) {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		slog.Error("failed to acquire connection for LISTEN", "error", err)
		return
	}
	defer conn.Release()

	_, err = conn.Exec(ctx, "LISTEN release_events")
	if err != nil {
		slog.Error("failed to execute LISTEN", "error", err)
		return
	}

	slog.Info("listening for PostgreSQL notifications on release_events")

	for {
		notification, err := conn.Conn().WaitForNotification(ctx)
		if err != nil {
			if ctx.Err() != nil {
				slog.Info("stopping notification listener: context cancelled")
				return
			}
			slog.Error("error waiting for notification", "error", err)
			return
		}

		broadcaster.Send(SSEEvent{
			Event: "release",
			Data:  notification.Payload,
		})
	}
}
