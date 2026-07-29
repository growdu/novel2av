package queue

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"

	"github.com/redis/go-redis/v9"
)

// ProgressEvent is the structure we publish to channel `events:project:<id>`.
// The ai-engine worker writes via Redis PUBLISH; backend-api subscribes and
// fans out to local WebSocket clients via the EventHub.
type ProgressEvent struct {
	Type      string  `json:"type"`
	ProjectID string  `json:"project_id,omitempty"`
	JobID     string  `json:"job_id,omitempty"`
	ChapterID string  `json:"chapter_id,omitempty"`
	ShotID    string  `json:"shot_id,omitempty"`
	Step      string  `json:"step,omitempty"`
	Status    string  `json:"status,omitempty"`
	Current   int     `json:"current,omitempty"`
	Total     int     `json:"total,omitempty"`
	Message   string  `json:"message,omitempty"`
}

// EventBus subscribes to Redis Pub/Sub and forwards events into local subscribers.
type EventBus struct {
	rdb *redis.Client

	mu          sync.RWMutex
	subscribers map[string]map[chan ProgressEvent]struct{}
	cancel      context.CancelFunc
}

func NewEventBus(rdb *redis.Client) *EventBus {
	eb := &EventBus{
		rdb:         rdb,
		subscribers: make(map[string]map[chan ProgressEvent]struct{}),
	}
	ctx, cancel := context.WithCancel(context.Background())
	eb.cancel = cancel
	go eb.loop(ctx)
	return eb
}

func (eb *EventBus) Close() {
	if eb.cancel != nil {
		eb.cancel()
	}
}

func channelName(projectID string) string { return "events:project:" + projectID }

func (eb *EventBus) Subscribe(projectID string) (<-chan ProgressEvent, func()) {
	ch := make(chan ProgressEvent, 64)
	eb.mu.Lock()
	if _, ok := eb.subscribers[projectID]; !ok {
		eb.subscribers[projectID] = make(map[chan ProgressEvent]struct{})
	}
	eb.subscribers[projectID][ch] = struct{}{}
	eb.mu.Unlock()
	return ch, func() {
		eb.mu.Lock()
		delete(eb.subscribers[projectID], ch)
		eb.mu.Unlock()
		close(ch)
	}
}

func (eb *EventBus) Publish(ctx context.Context, projectID string, ev ProgressEvent) {
	body, _ := json.Marshal(ev)
	if err := eb.rdb.Publish(ctx, channelName(projectID), body).Err(); err != nil {
		slog.Warn("publish event", "err", err)
	}
}

func (eb *EventBus) loop(ctx context.Context) {
	ps := eb.rdb.PSubscribe(ctx, "events:project:*")
	defer ps.Close()
	ch := ps.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			var ev ProgressEvent
			if err := json.Unmarshal([]byte(msg.Payload), &ev); err != nil {
				continue
			}
			pid := ev.ProjectID
			if pid == "" {
				// channel name is "events:project:<id>"; strip the prefix as a fallback.
				pid = msg.Channel[len("events:project:"):]
				ev.ProjectID = pid
			}
			eb.fanOut(pid, ev)
		}
	}
}

func (eb *EventBus) fanOut(projectID string, ev ProgressEvent) {
	eb.mu.RLock()
	defer eb.mu.RUnlock()
	for ch := range eb.subscribers[projectID] {
		select {
		case ch <- ev:
		default:
		}
	}
}


// ServeWS upgrades the HTTP request to a WebSocket and forwards every event
// for the given project as JSON text frames. Goroutine exits when the client
// disconnects.
func (eb *EventBus) ServeWS(w http.ResponseWriter, r *http.Request, projectID string) {
	upgrader := websocketUpgrader()
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	ch, cancel := eb.Subscribe(projectID)
	defer cancel()
	for ev := range ch {
		body, err := json.Marshal(ev)
		if err != nil {
			continue
		}
		if err := conn.WriteMessage(1, body); err != nil { // 1 == TextMessage
			return
		}
	}
}