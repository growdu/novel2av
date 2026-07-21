package service

import (
	"context"
	"encoding/json"
	"sync"
)

// Event is the payload published to subscribers (WebSocket clients today,
// could be analytics tomorrow).
type Event struct {
	Type      string `json:"type"`
	ProjectID string `json:"project_id"`
	JobID     string `json:"job_id,omitempty"`
	Step      string `json:"step,omitempty"`
	Status    string `json:"status,omitempty"`
	Current   int    `json:"current,omitempty"`
	Total     int    `json:"total,omitempty"`
	Message   string `json:"message,omitempty"`
}

// EventHub is an in-process fan-out. The Redis Pub/Sub bridge is wired in
// infra/queue; this hub lets multiple WS handlers share the stream.
type EventHub struct {
	mu          sync.RWMutex
	subscribers map[string]map[chan Event]struct{}
}

func newEventHub() *EventHub {
	return &EventHub{subscribers: make(map[string]map[chan Event]struct{})}
}

func (h *EventHub) Subscribe(projectID string) (<-chan Event, func()) {
	ch := make(chan Event, 32)
	h.mu.Lock()
	if _, ok := h.subscribers[projectID]; !ok {
		h.subscribers[projectID] = make(map[chan Event]struct{})
	}
	h.subscribers[projectID][ch] = struct{}{}
	h.mu.Unlock()
	return ch, func() {
		h.mu.Lock()
		delete(h.subscribers[projectID], ch)
		h.mu.Unlock()
		close(ch)
	}
}

func (h *EventHub) Publish(_ context.Context, projectID string, e Event) {
	e.ProjectID = projectID
	body, _ := json.Marshal(e)
	_ = body // forward to Redis here
	h.mu.RLock()
	for ch := range h.subscribers[projectID] {
		select {
		case ch <- e:
		default:
		}
	}
	h.mu.RUnlock()
}
