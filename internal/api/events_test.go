package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestBroadcasterSubscribeAndSend(t *testing.T) {
	b := NewBroadcaster()
	ch := b.Subscribe()

	evt := SSEEvent{Event: "release", Data: `{"version":"1.0.0"}`}
	b.Send(evt)

	select {
	case got := <-ch:
		if got.Event != evt.Event {
			t.Fatalf("expected event=%s, got %s", evt.Event, got.Event)
		}
		if got.Data != evt.Data {
			t.Fatalf("expected data=%s, got %s", evt.Data, got.Data)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}

	b.Unsubscribe(ch)
}

func TestBroadcasterUnsubscribe(t *testing.T) {
	b := NewBroadcaster()
	ch := b.Subscribe()
	b.Unsubscribe(ch)

	// After unsubscribe, the channel should be closed.
	_, ok := <-ch
	if ok {
		t.Fatal("expected channel to be closed after unsubscribe")
	}

	// Verify client was removed from the map.
	b.mu.RLock()
	count := len(b.clients)
	b.mu.RUnlock()
	if count != 0 {
		t.Fatalf("expected 0 clients after unsubscribe, got %d", count)
	}
}

func TestBroadcasterDoubleUnsubscribe(t *testing.T) {
	b := NewBroadcaster()
	ch := b.Subscribe()
	b.Unsubscribe(ch)
	// Second unsubscribe should not panic.
	b.Unsubscribe(ch)
}

func TestBroadcasterSlowClient(t *testing.T) {
	b := NewBroadcaster()
	ch := b.Subscribe()

	// Fill the channel buffer (capacity 64).
	for i := 0; i < 64; i++ {
		b.Send(SSEEvent{Event: "release", Data: `{"i":"fill"}`})
	}

	// The next send should not block — it should skip the slow client.
	done := make(chan struct{})
	go func() {
		b.Send(SSEEvent{Event: "release", Data: `{"i":"overflow"}`})
		close(done)
	}()

	select {
	case <-done:
		// Success: Send did not block.
	case <-time.After(time.Second):
		t.Fatal("Send blocked on slow client")
	}

	b.Unsubscribe(ch)
}

func TestBroadcasterMultipleClients(t *testing.T) {
	b := NewBroadcaster()
	ch1 := b.Subscribe()
	ch2 := b.Subscribe()

	evt := SSEEvent{Event: "release", Data: `{"version":"2.0.0"}`}
	b.Send(evt)

	for i, ch := range []chan SSEEvent{ch1, ch2} {
		select {
		case got := <-ch:
			if got.Event != evt.Event || got.Data != evt.Data {
				t.Fatalf("client %d: unexpected event: %+v", i, got)
			}
		case <-time.After(time.Second):
			t.Fatalf("client %d: timed out waiting for event", i)
		}
	}

	b.Unsubscribe(ch1)
	b.Unsubscribe(ch2)
}

func TestEventsHandlerStream(t *testing.T) {
	b := NewBroadcaster()
	h := NewEventsHandler(b)

	// Use a cancellable context so we can end the SSE stream.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/events", nil)
	r = r.WithContext(ctx)

	// Run the handler in a goroutine since it blocks.
	done := make(chan struct{})
	go func() {
		h.Stream(w, r)
		close(done)
	}()

	// Give the handler time to set up and send the initial connected event.
	time.Sleep(50 * time.Millisecond)

	// Send an event through the broadcaster.
	b.Send(SSEEvent{Event: "release", Data: `{"version":"3.0.0"}`})

	// Give the handler time to write the event.
	time.Sleep(50 * time.Millisecond)

	// Cancel the context to end the stream.
	cancel()

	select {
	case <-done:
		// Handler exited cleanly.
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not exit after context cancellation")
	}

	body := w.Body.String()

	// Check SSE headers.
	if ct := w.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("expected Content-Type=text/event-stream, got %s", ct)
	}
	if cc := w.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Fatalf("expected Cache-Control=no-cache, got %s", cc)
	}

	// Check initial connection event.
	if !strings.Contains(body, "event: connected") {
		t.Fatal("expected initial connected event in response body")
	}

	// Check that the release event was written.
	if !strings.Contains(body, "event: release") {
		t.Fatalf("expected release event in response body, got: %s", body)
	}
	if !strings.Contains(body, `data: {"version":"3.0.0"}`) {
		t.Fatalf("expected release data in response body, got: %s", body)
	}
}
