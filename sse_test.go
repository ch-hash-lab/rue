package rue

import (
	"bufio"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSSE_Headers(t *testing.T) {
	engine := New()
	engine.GET("/events", func(c *Context) {
		client, err := SSE(c)
		if err != nil {
			t.Errorf("SSE error: %v", err)
			return
		}
		defer client.Close()

		client.SendData("test")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/events", nil)
	engine.ServeHTTP(w, req)

	// Check headers
	if w.Header().Get("Content-Type") != "text/event-stream" {
		t.Errorf("Content-Type = %s, want text/event-stream", w.Header().Get("Content-Type"))
	}
	if w.Header().Get("Cache-Control") != "no-cache" {
		t.Errorf("Cache-Control = %s, want no-cache", w.Header().Get("Cache-Control"))
	}
	if w.Header().Get("Connection") != "keep-alive" {
		t.Errorf("Connection = %s, want keep-alive", w.Header().Get("Connection"))
	}
}

func TestSSE_SendData(t *testing.T) {
	engine := New()
	engine.GET("/events", func(c *Context) {
		client, err := SSE(c)
		if err != nil {
			t.Errorf("SSE error: %v", err)
			return
		}
		defer client.Close()

		client.SendData("hello world")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/events", nil)
	engine.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "data: hello world") {
		t.Errorf("Body should contain 'data: hello world', got: %s", body)
	}
}

func TestSSE_SendEvent(t *testing.T) {
	engine := New()
	engine.GET("/events", func(c *Context) {
		client, err := SSE(c)
		if err != nil {
			t.Errorf("SSE error: %v", err)
			return
		}
		defer client.Close()

		client.SendEvent("message", "hello")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/events", nil)
	engine.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "event: message") {
		t.Errorf("Body should contain 'event: message', got: %s", body)
	}
	if !strings.Contains(body, "data: hello") {
		t.Errorf("Body should contain 'data: hello', got: %s", body)
	}
}

func TestSSE_SendEventWithID(t *testing.T) {
	engine := New()
	engine.GET("/events", func(c *Context) {
		client, err := SSE(c)
		if err != nil {
			t.Errorf("SSE error: %v", err)
			return
		}
		defer client.Close()

		client.SendEventWithID("123", "update", "data")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/events", nil)
	engine.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "id: 123") {
		t.Errorf("Body should contain 'id: 123', got: %s", body)
	}
	if !strings.Contains(body, "event: update") {
		t.Errorf("Body should contain 'event: update', got: %s", body)
	}
}

func TestSSE_SendComment(t *testing.T) {
	engine := New()
	engine.GET("/events", func(c *Context) {
		client, err := SSE(c)
		if err != nil {
			t.Errorf("SSE error: %v", err)
			return
		}
		defer client.Close()

		client.SendComment("this is a comment")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/events", nil)
	engine.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, ": this is a comment") {
		t.Errorf("Body should contain comment, got: %s", body)
	}
}

func TestSSE_FullEvent(t *testing.T) {
	engine := New()
	engine.GET("/events", func(c *Context) {
		client, err := SSE(c)
		if err != nil {
			t.Errorf("SSE error: %v", err)
			return
		}
		defer client.Close()

		client.Send(&SSEEvent{
			ID:      "evt-1",
			Event:   "notification",
			Data:    "You have a new message",
			Retry:   5000,
			Comment: "notification event",
		})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/events", nil)
	engine.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, ": notification event") {
		t.Errorf("Body should contain comment")
	}
	if !strings.Contains(body, "id: evt-1") {
		t.Errorf("Body should contain id")
	}
	if !strings.Contains(body, "event: notification") {
		t.Errorf("Body should contain event")
	}
	if !strings.Contains(body, "retry: 5000") {
		t.Errorf("Body should contain retry")
	}
	if !strings.Contains(body, "data: You have a new message") {
		t.Errorf("Body should contain data")
	}
}

func TestSSE_RetryInterval(t *testing.T) {
	config := SSEConfig{
		RetryInterval: 5000,
		KeepAlive:     false,
	}

	engine := New()
	engine.GET("/events", func(c *Context) {
		client, err := SSEWithConfig(c, config)
		if err != nil {
			t.Errorf("SSE error: %v", err)
			return
		}
		defer client.Close()

		client.SendData("test")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/events", nil)
	engine.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "retry: 5000") {
		t.Errorf("Body should contain 'retry: 5000', got: %s", body)
	}
}

func TestSSE_IsClosed(t *testing.T) {
	client := &SSEClient{}

	if client.IsClosed() {
		t.Error("New client should not be closed")
	}

	client.Close()

	if !client.IsClosed() {
		t.Error("Client should be closed after Close()")
	}
}

func TestSSEHub_RegisterUnregister(t *testing.T) {
	hub := NewSSEHub()

	client1 := &SSEClient{}
	client2 := &SSEClient{}

	hub.Register(client1)
	hub.Register(client2)

	if hub.Count() != 2 {
		t.Errorf("Count() = %d, want 2", hub.Count())
	}

	hub.Unregister(client1)

	if hub.Count() != 1 {
		t.Errorf("Count() = %d, want 1", hub.Count())
	}
}

func TestSSEHub_Rooms(t *testing.T) {
	hub := NewSSEHub()

	client1 := &SSEClient{}
	client2 := &SSEClient{}
	client3 := &SSEClient{}

	hub.Register(client1)
	hub.Register(client2)
	hub.Register(client3)

	hub.Join(client1, "room1")
	hub.Join(client2, "room1")
	hub.Join(client3, "room2")

	if hub.RoomCount("room1") != 2 {
		t.Errorf("RoomCount(room1) = %d, want 2", hub.RoomCount("room1"))
	}

	if hub.RoomCount("room2") != 1 {
		t.Errorf("RoomCount(room2) = %d, want 1", hub.RoomCount("room2"))
	}

	hub.Leave(client1, "room1")

	if hub.RoomCount("room1") != 1 {
		t.Errorf("RoomCount(room1) = %d, want 1", hub.RoomCount("room1"))
	}

	// Unregister should remove from all rooms
	hub.Unregister(client2)

	if hub.RoomCount("room1") != 0 {
		t.Errorf("RoomCount(room1) = %d, want 0", hub.RoomCount("room1"))
	}
}

func TestSSEHub_NonExistentRoom(t *testing.T) {
	hub := NewSSEHub()

	if hub.RoomCount("nonexistent") != 0 {
		t.Errorf("RoomCount(nonexistent) = %d, want 0", hub.RoomCount("nonexistent"))
	}
}

func TestDefaultSSEConfig(t *testing.T) {
	config := DefaultSSEConfig()

	if config.RetryInterval != 3000 {
		t.Errorf("RetryInterval = %d, want 3000", config.RetryInterval)
	}
	if !config.KeepAlive {
		t.Error("KeepAlive should be true")
	}
	if config.KeepAliveTime != 30*time.Second {
		t.Errorf("KeepAliveTime = %v, want 30s", config.KeepAliveTime)
	}
}

// Integration test with real HTTP server
func TestSSE_Integration(t *testing.T) {
	engine := New()
	engine.GET("/events", func(c *Context) {
		config := SSEConfig{
			RetryInterval: 1000,
			KeepAlive:     false,
		}
		client, err := SSEWithConfig(c, config)
		if err != nil {
			return
		}
		defer client.Close()

		// Send multiple events
		for i := 0; i < 3; i++ {
			client.SendEventWithID(
				string(rune('1'+i)),
				"counter",
				string(rune('0'+i)),
			)
		}
	})

	server := httptest.NewServer(engine)
	defer server.Close()

	resp, err := http.Get(server.URL + "/events")
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer resp.Body.Close()

	if resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Errorf("Content-Type = %s, want text/event-stream", resp.Header.Get("Content-Type"))
	}

	// Read events
	reader := bufio.NewReader(resp.Body)
	var events []string

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		events = append(events, line)
	}

	// Should have received events
	if len(events) < 3 {
		t.Errorf("Expected at least 3 event lines, got %d", len(events))
	}
}

// Test SSE event format parsing
func TestSSE_EventFormat(t *testing.T) {
	engine := New()
	engine.GET("/events", func(c *Context) {
		client, err := SSEWithConfig(c, SSEConfig{RetryInterval: 0, KeepAlive: false})
		if err != nil {
			return
		}
		defer client.Close()

		client.Send(&SSEEvent{
			ID:    "1",
			Event: "test",
			Data:  "hello",
		})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/events", nil)
	engine.ServeHTTP(w, req)

	body := w.Body.String()

	// Parse SSE format
	lines := strings.Split(body, "\n")
	var hasID, hasEvent, hasData bool

	for _, line := range lines {
		if strings.HasPrefix(line, "id: ") {
			hasID = true
			if line != "id: 1" {
				t.Errorf("ID line = %s, want 'id: 1'", line)
			}
		}
		if strings.HasPrefix(line, "event: ") {
			hasEvent = true
			if line != "event: test" {
				t.Errorf("Event line = %s, want 'event: test'", line)
			}
		}
		if strings.HasPrefix(line, "data: ") {
			hasData = true
			if line != "data: hello" {
				t.Errorf("Data line = %s, want 'data: hello'", line)
			}
		}
	}

	if !hasID {
		t.Error("Missing ID field")
	}
	if !hasEvent {
		t.Error("Missing event field")
	}
	if !hasData {
		t.Error("Missing data field")
	}
}

// Test sending to closed client
func TestSSE_SendToClosed(t *testing.T) {
	client := &SSEClient{}
	client.Close()

	err := client.SendData("test")
	if err == nil {
		t.Error("Expected error when sending to closed client")
	}

	err = client.SendComment("test")
	if err == nil {
		t.Error("Expected error when sending comment to closed client")
	}
}
