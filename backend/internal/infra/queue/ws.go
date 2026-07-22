package queue

import "github.com/gorilla/websocket"

// websocketUpgrader is shared; default 1 MiB buffer, no origin check (the
// public API is gated behind our own reverse proxy + CORS).
func websocketUpgrader() websocket.Upgrader {
	return websocket.Upgrader{
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
		CheckOrigin:     func(_ /* origin */ string) bool { return true },
	}
}
