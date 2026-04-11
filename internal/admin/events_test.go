package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestEventBus_PublishSubscribe(t *testing.T) {
	eb := newEventBus()

	ch := eb.subscribe()
	defer eb.unsubscribe(ch)

	event := EventItem{
		ID:        "test-1",
		Type:      "info",
		Message:   "test event",
		Timestamp: time.Now().Format(time.RFC3339),
	}

	eb.Publish(event)

	select {
	case received := <-ch:
		if received.ID != "test-1" {
			t.Errorf("expected ID test-1, got %s", received.ID)
		}
		if received.Message != "test event" {
			t.Errorf("expected message 'test event', got %s", received.Message)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestEventBus_SubscriberCount(t *testing.T) {
	eb := newEventBus()

	if eb.SubscriberCount() != 0 {
		t.Errorf("expected 0 subscribers, got %d", eb.SubscriberCount())
	}

	ch1 := eb.subscribe()
	if eb.SubscriberCount() != 1 {
		t.Errorf("expected 1 subscriber, got %d", eb.SubscriberCount())
	}

	ch2 := eb.subscribe()
	if eb.SubscriberCount() != 2 {
		t.Errorf("expected 2 subscribers, got %d", eb.SubscriberCount())
	}

	eb.unsubscribe(ch1)
	if eb.SubscriberCount() != 1 {
		t.Errorf("expected 1 subscriber after unsubscribe, got %d", eb.SubscriberCount())
	}

	eb.unsubscribe(ch2)
	if eb.SubscriberCount() != 0 {
		t.Errorf("expected 0 subscribers after all unsubscribe, got %d", eb.SubscriberCount())
	}
}

func TestEventBus_FullBuffer(t *testing.T) {
	eb := newEventBus()

	ch := eb.subscribe()
	defer eb.unsubscribe(ch)

	for i := 0; i < 20; i++ {
		eb.Publish(EventItem{ID: "fill", Type: "info", Message: "fill"})
	}

	// This should not block even though buffer is full
	eb.Publish(EventItem{ID: "overflow", Type: "info", Message: "overflow"})

	drained := 0
	timeout := time.After(time.Second)
	for {
		select {
		case <-ch:
			drained++
		case <-timeout:
			goto done
		}
	}
done:
	if drained == 0 {
		t.Error("expected to drain at least one event")
	}
}

func TestStreamEvents_MethodNotAllowed(t *testing.T) {
	cfg := &Config{
		Address:     "127.0.0.1:0",
		PoolManager: &mockPoolManager{},
		Router:      &mockRouter{},
		Metrics:     NewDefaultMetrics(nil),
	}
	srv, _ := NewServer(cfg)
	srv.eventBus = newEventBus()

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/events/stream", nil)
	w := httptest.NewRecorder()

	srv.setupRoutes()
	handler := srv.server.Handler

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

func TestServer_PublishEvent(t *testing.T) {
	cfg := &Config{
		Address:     "127.0.0.1:0",
		PoolManager: &mockPoolManager{},
		Router:      &mockRouter{},
		Metrics:     NewDefaultMetrics(nil),
	}
	srv, _ := NewServer(cfg)
	srv.eventBus = newEventBus()

	ch := srv.eventBus.subscribe()
	defer srv.eventBus.unsubscribe(ch)

	srv.PublishEvent(EventItem{
		ID:      "test-publish",
		Type:    "success",
		Message: "published",
	})

	select {
	case event := <-ch:
		if event.ID != "test-publish" {
			t.Errorf("expected ID test-publish, got %s", event.ID)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for published event")
	}
}

func TestEventItem_JSON(t *testing.T) {
	event := EventItem{
		ID:        "json-test",
		Type:      "warning",
		Message:   "backend unhealthy",
		Timestamp: "2026-04-11T12:00:00Z",
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	var decoded EventItem
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	if decoded.ID != event.ID {
		t.Errorf("expected ID %s, got %s", event.ID, decoded.ID)
	}
	if decoded.Type != event.Type {
		t.Errorf("expected Type %s, got %s", event.Type, decoded.Type)
	}
}
