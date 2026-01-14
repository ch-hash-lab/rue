package rue

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

// SSEEvent represents a Server-Sent Event
type SSEEvent struct {
	ID      string
	Event   string
	Data    string
	Retry   int
	Comment string
}

// SSEClient represents an SSE client connection
type SSEClient struct {
	c       *Context
	flusher http.Flusher
	closed  bool
	closeMu sync.RWMutex
	done    chan struct{} // Used to signal keepAlive goroutine to stop
}

// SSEConfig defines SSE configuration
type SSEConfig struct {
	RetryInterval int  // Retry interval in milliseconds
	KeepAlive     bool // Send keep-alive comments
	KeepAliveTime time.Duration
}

// DefaultSSEConfig returns default SSE configuration
func DefaultSSEConfig() SSEConfig {
	return SSEConfig{
		RetryInterval: 3000,
		KeepAlive:     true,
		KeepAliveTime: 30 * time.Second,
	}
}

// SSE creates an SSE client from the context
func SSE(c *Context) (*SSEClient, error) {
	return SSEWithConfig(c, DefaultSSEConfig())
}

// SSEWithConfig creates an SSE client with custom config
func SSEWithConfig(c *Context, config SSEConfig) (*SSEClient, error) {
	// Set SSE headers
	c.SetHeader("Content-Type", "text/event-stream")
	c.SetHeader("Cache-Control", "no-cache")
	c.SetHeader("Connection", "keep-alive")
	c.SetHeader("X-Accel-Buffering", "no") // Disable nginx buffering

	// Get flusher
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("streaming not supported")
	}

	client := &SSEClient{
		c:       c,
		flusher: flusher,
		done:    make(chan struct{}),
	}

	// Send retry interval
	if config.RetryInterval > 0 {
		fmt.Fprintf(c.Writer, "retry: %d\n\n", config.RetryInterval)
		flusher.Flush()
	}

	// Start keep-alive if configured
	if config.KeepAlive && config.KeepAliveTime > 0 {
		go client.keepAlive(config.KeepAliveTime)
	}

	return client, nil
}

// Send sends an SSE event
func (s *SSEClient) Send(event *SSEEvent) error {
	if s.IsClosed() {
		return fmt.Errorf("client closed")
	}

	// Write comment
	if event.Comment != "" {
		fmt.Fprintf(s.c.Writer, ": %s\n", event.Comment)
	}

	// Write ID
	if event.ID != "" {
		fmt.Fprintf(s.c.Writer, "id: %s\n", event.ID)
	}

	// Write event type
	if event.Event != "" {
		fmt.Fprintf(s.c.Writer, "event: %s\n", event.Event)
	}

	// Write retry
	if event.Retry > 0 {
		fmt.Fprintf(s.c.Writer, "retry: %d\n", event.Retry)
	}

	// Write data
	if event.Data != "" {
		fmt.Fprintf(s.c.Writer, "data: %s\n", event.Data)
	}

	// End event
	fmt.Fprint(s.c.Writer, "\n")

	s.flusher.Flush()
	return nil
}

// SendData sends a simple data event
func (s *SSEClient) SendData(data string) error {
	return s.Send(&SSEEvent{Data: data})
}

// SendEvent sends a named event with data
func (s *SSEClient) SendEvent(event, data string) error {
	return s.Send(&SSEEvent{Event: event, Data: data})
}

// SendEventWithID sends a named event with ID and data
func (s *SSEClient) SendEventWithID(id, event, data string) error {
	return s.Send(&SSEEvent{ID: id, Event: event, Data: data})
}

// SendComment sends a comment (for keep-alive)
func (s *SSEClient) SendComment(comment string) error {
	if s.IsClosed() {
		return fmt.Errorf("client closed")
	}

	fmt.Fprintf(s.c.Writer, ": %s\n\n", comment)
	s.flusher.Flush()
	return nil
}

// Close closes the SSE connection
func (s *SSEClient) Close() {
	s.closeMu.Lock()
	if s.closed {
		s.closeMu.Unlock()
		return
	}
	s.closed = true
	close(s.done)
	s.closeMu.Unlock()
}

// IsClosed returns true if the connection is closed
func (s *SSEClient) IsClosed() bool {
	s.closeMu.RLock()
	defer s.closeMu.RUnlock()
	return s.closed
}

// Done returns a channel that's closed when the client disconnects
func (s *SSEClient) Done() <-chan struct{} {
	return s.done
}

// keepAlive sends periodic keep-alive comments
func (s *SSEClient) keepAlive(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Use request context to detect client disconnect
	ctx := s.c.Request.Context()

	for {
		select {
		case <-ticker.C:
			if s.IsClosed() {
				return
			}
			if err := s.SendComment("keep-alive"); err != nil {
				s.Close()
				return
			}
		case <-ctx.Done():
			// Client disconnected
			s.Close()
			return
		case <-s.done:
			// Explicitly closed
			return
		}
	}
}

// ============== SSE Hub ==============

// SSEHub manages SSE client connections
type SSEHub struct {
	clients map[*SSEClient]bool
	rooms   map[string]map[*SSEClient]bool
	mu      sync.RWMutex
}

// NewSSEHub creates a new SSE hub
func NewSSEHub() *SSEHub {
	return &SSEHub{
		clients: make(map[*SSEClient]bool),
		rooms:   make(map[string]map[*SSEClient]bool),
	}
}

// Register adds a client to the hub
func (h *SSEHub) Register(client *SSEClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[client] = true
}

// Unregister removes a client from the hub
func (h *SSEHub) Unregister(client *SSEClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients, client)

	// Remove from all rooms
	for _, room := range h.rooms {
		delete(room, client)
	}
}

// Join adds a client to a room
func (h *SSEHub) Join(client *SSEClient, room string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.rooms[room] == nil {
		h.rooms[room] = make(map[*SSEClient]bool)
	}
	h.rooms[room][client] = true
}

// Leave removes a client from a room
func (h *SSEHub) Leave(client *SSEClient, room string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.rooms[room] != nil {
		delete(h.rooms[room], client)
	}
}

// Broadcast sends an event to all clients
func (h *SSEHub) Broadcast(event *SSEEvent) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		if !client.IsClosed() {
			client.Send(event)
		}
	}
}

// BroadcastData sends data to all clients
func (h *SSEHub) BroadcastData(data string) {
	h.Broadcast(&SSEEvent{Data: data})
}

// BroadcastToRoom sends an event to all clients in a room
func (h *SSEHub) BroadcastToRoom(room string, event *SSEEvent) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.rooms[room] != nil {
		for client := range h.rooms[room] {
			if !client.IsClosed() {
				client.Send(event)
			}
		}
	}
}

// BroadcastDataToRoom sends data to all clients in a room
func (h *SSEHub) BroadcastDataToRoom(room, data string) {
	h.BroadcastToRoom(room, &SSEEvent{Data: data})
}

// Count returns the number of clients
func (h *SSEHub) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// RoomCount returns the number of clients in a room
func (h *SSEHub) RoomCount(room string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.rooms[room] != nil {
		return len(h.rooms[room])
	}
	return 0
}
