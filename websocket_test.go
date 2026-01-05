package rue

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestComputeAcceptKey(t *testing.T) {
	// Test vector from RFC 6455
	key := "dGhlIHNhbXBsZSBub25jZQ=="
	expected := "s3pPLMBiTxaQ9kYGzzhZRbK+xOo="

	result := computeAcceptKey(key)
	if result != expected {
		t.Errorf("computeAcceptKey(%s) = %s, want %s", key, result, expected)
	}
}

func TestWebSocketHub_RegisterUnregister(t *testing.T) {
	hub := NewWebSocketHub()

	// Create mock connections
	conn1 := &WebSocketConn{}
	conn2 := &WebSocketConn{}

	hub.Register(conn1)
	hub.Register(conn2)

	if hub.Count() != 2 {
		t.Errorf("Count() = %d, want 2", hub.Count())
	}

	hub.Unregister(conn1)

	if hub.Count() != 1 {
		t.Errorf("Count() = %d, want 1", hub.Count())
	}
}

func TestWebSocketHub_Rooms(t *testing.T) {
	hub := NewWebSocketHub()

	conn1 := &WebSocketConn{}
	conn2 := &WebSocketConn{}
	conn3 := &WebSocketConn{}

	hub.Register(conn1)
	hub.Register(conn2)
	hub.Register(conn3)

	// Join rooms
	hub.Join(conn1, "room1")
	hub.Join(conn2, "room1")
	hub.Join(conn3, "room2")

	if hub.RoomCount("room1") != 2 {
		t.Errorf("RoomCount(room1) = %d, want 2", hub.RoomCount("room1"))
	}

	if hub.RoomCount("room2") != 1 {
		t.Errorf("RoomCount(room2) = %d, want 1", hub.RoomCount("room2"))
	}

	// Leave room
	hub.Leave(conn1, "room1")

	if hub.RoomCount("room1") != 1 {
		t.Errorf("RoomCount(room1) = %d, want 1", hub.RoomCount("room1"))
	}

	// Unregister should remove from all rooms
	hub.Unregister(conn2)

	if hub.RoomCount("room1") != 0 {
		t.Errorf("RoomCount(room1) = %d, want 0", hub.RoomCount("room1"))
	}
}

func TestWebSocketHub_NonExistentRoom(t *testing.T) {
	hub := NewWebSocketHub()

	if hub.RoomCount("nonexistent") != 0 {
		t.Errorf("RoomCount(nonexistent) = %d, want 0", hub.RoomCount("nonexistent"))
	}
}

func TestDefaultWebSocketConfig(t *testing.T) {
	config := DefaultWebSocketConfig()

	if config.ReadBufferSize != 4096 {
		t.Errorf("ReadBufferSize = %d, want 4096", config.ReadBufferSize)
	}
	if config.WriteBufferSize != 4096 {
		t.Errorf("WriteBufferSize = %d, want 4096", config.WriteBufferSize)
	}
	if config.PingPeriod != 30*time.Second {
		t.Errorf("PingPeriod = %v, want 30s", config.PingPeriod)
	}
	if config.PongWait != 60*time.Second {
		t.Errorf("PongWait = %v, want 60s", config.PongWait)
	}
}

func TestWebSocketConn_IsClosed(t *testing.T) {
	conn := &WebSocketConn{}

	if conn.IsClosed() {
		t.Error("New connection should not be closed")
	}

	conn.closed = true

	if !conn.IsClosed() {
		t.Error("Connection should be closed")
	}
}

// Test WebSocket upgrade request validation
func TestWebSocket_UpgradeValidation(t *testing.T) {
	handler := &WebSocketHandler{}

	engine := New()
	engine.GET("/ws", WebSocket(handler))

	// Missing Upgrade header
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/ws", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Missing Upgrade: Status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	// Missing Sec-WebSocket-Key
	w = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/ws", nil)
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Missing Key: Status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// Test WebSocket frame encoding
func TestWebSocket_FrameEncoding(t *testing.T) {
	// Test small payload (< 126 bytes)
	testFrameSize(t, 10)

	// Test medium payload (126-65535 bytes)
	testFrameSize(t, 1000)

	// Test large payload (> 65535 bytes)
	testFrameSize(t, 70000)
}

func testFrameSize(t *testing.T, size int) {
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(i % 256)
	}

	// Create a pipe for testing
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	wsConn := &WebSocketConn{
		conn:   serverConn,
		reader: bufio.NewReader(serverConn),
		writer: bufio.NewWriter(serverConn),
	}

	// Write frame in goroutine
	var writeErr error
	go func() {
		writeErr = wsConn.WriteMessage(BinaryMessage, data)
	}()

	// Read and verify frame
	reader := bufio.NewReader(clientConn)

	// Read header
	header := make([]byte, 2)
	if _, err := io.ReadFull(reader, header); err != nil {
		t.Fatalf("Failed to read header: %v", err)
	}

	opcode := int(header[0] & 0x0F)
	if opcode != BinaryMessage {
		t.Errorf("Opcode = %d, want %d", opcode, BinaryMessage)
	}

	payloadLen := int(header[1] & 0x7F)

	// Read extended length if needed
	if payloadLen == 126 {
		extLen := make([]byte, 2)
		io.ReadFull(reader, extLen)
		payloadLen = int(binary.BigEndian.Uint16(extLen))
	} else if payloadLen == 127 {
		extLen := make([]byte, 8)
		io.ReadFull(reader, extLen)
		payloadLen = int(binary.BigEndian.Uint64(extLen))
	}

	if payloadLen != size {
		t.Errorf("Payload length = %d, want %d", payloadLen, size)
	}

	// Read payload using io.ReadFull to ensure we get all bytes
	payload := make([]byte, payloadLen)
	if _, err := io.ReadFull(reader, payload); err != nil {
		t.Fatalf("Failed to read payload: %v", err)
	}

	// Verify data
	for i := range payload {
		if payload[i] != data[i] {
			t.Errorf("Payload mismatch at index %d", i)
			break
		}
	}

	if writeErr != nil {
		t.Errorf("Write error: %v", writeErr)
	}
}

// Test WebSocket message types
func TestWebSocket_MessageTypes(t *testing.T) {
	tests := []struct {
		name        string
		messageType int
	}{
		{"Text", TextMessage},
		{"Binary", BinaryMessage},
		{"Ping", PingMessage},
		{"Pong", PongMessage},
		{"Close", CloseMessage},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			serverConn, clientConn := net.Pipe()
			defer serverConn.Close()
			defer clientConn.Close()

			wsConn := &WebSocketConn{
				conn:   serverConn,
				reader: bufio.NewReader(serverConn),
				writer: bufio.NewWriter(serverConn),
			}

			go wsConn.WriteMessage(tt.messageType, []byte("test"))

			reader := bufio.NewReader(clientConn)
			header := make([]byte, 2)
			reader.Read(header)

			opcode := int(header[0] & 0x0F)
			if opcode != tt.messageType {
				t.Errorf("Opcode = %d, want %d", opcode, tt.messageType)
			}
		})
	}
}

// Test WebSocket Send methods
func TestWebSocket_SendMethods(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	wsConn := &WebSocketConn{
		conn:   serverConn,
		reader: bufio.NewReader(serverConn),
		writer: bufio.NewWriter(serverConn),
	}

	// Test Send (text)
	go wsConn.Send("hello")

	reader := bufio.NewReader(clientConn)
	header := make([]byte, 2)
	reader.Read(header)

	if header[0]&0x0F != TextMessage {
		t.Error("Send should use TextMessage type")
	}
}

// Test concurrent access to WebSocket connection
func TestWebSocket_ConcurrentAccess(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	wsConn := &WebSocketConn{
		conn:   serverConn,
		reader: bufio.NewReader(serverConn),
		writer: bufio.NewWriter(serverConn),
	}

	// Drain client side
	go func() {
		buf := make([]byte, 1024)
		for {
			_, err := clientConn.Read(buf)
			if err != nil {
				return
			}
		}
	}()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			wsConn.Send("message " + string(rune('0'+n)))
		}(i)
	}

	wg.Wait()
}

// Test WebSocket Hub broadcast
func TestWebSocketHub_Broadcast(t *testing.T) {
	hub := NewWebSocketHub()

	// Create mock connections with pipes
	var conns []*WebSocketConn
	var clientConns []net.Conn

	for i := 0; i < 3; i++ {
		serverConn, clientConn := net.Pipe()
		wsConn := &WebSocketConn{
			conn:   serverConn,
			reader: bufio.NewReader(serverConn),
			writer: bufio.NewWriter(serverConn),
		}
		conns = append(conns, wsConn)
		clientConns = append(clientConns, clientConn)
		hub.Register(wsConn)
	}

	// Drain all client connections
	for _, cc := range clientConns {
		go func(c net.Conn) {
			buf := make([]byte, 1024)
			for {
				_, err := c.Read(buf)
				if err != nil {
					return
				}
			}
		}(cc)
	}

	// Broadcast should not panic
	hub.Broadcast("test message")
	hub.BroadcastBinary([]byte("binary data"))

	// Cleanup
	for i := range conns {
		conns[i].conn.Close()
		clientConns[i].Close()
	}
}

// Test WebSocket accept key calculation (RFC 6455 compliance)
func TestWebSocket_AcceptKeyRFC6455(t *testing.T) {
	// Test vectors from RFC 6455 Section 1.3
	testCases := []struct {
		key      string
		expected string
	}{
		{"dGhlIHNhbXBsZSBub25jZQ==", "s3pPLMBiTxaQ9kYGzzhZRbK+xOo="},
	}

	for _, tc := range testCases {
		result := computeAcceptKey(tc.key)
		if result != tc.expected {
			t.Errorf("computeAcceptKey(%s) = %s, want %s", tc.key, result, tc.expected)
		}
	}
}

// Verify the accept key calculation manually
func TestWebSocket_AcceptKeyManual(t *testing.T) {
	key := "dGhlIHNhbXBsZSBub25jZQ=="
	guid := "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

	h := sha1.New()
	h.Write([]byte(key + guid))
	expected := base64.StdEncoding.EncodeToString(h.Sum(nil))

	result := computeAcceptKey(key)
	if result != expected {
		t.Errorf("Accept key mismatch: got %s, want %s", result, expected)
	}
}

// Test WebSocket connection close
func TestWebSocket_Close(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()

	wsConn := &WebSocketConn{
		conn:   serverConn,
		reader: bufio.NewReader(serverConn),
		writer: bufio.NewWriter(serverConn),
	}

	// Drain client side
	go func() {
		buf := make([]byte, 1024)
		for {
			_, err := clientConn.Read(buf)
			if err != nil {
				return
			}
		}
	}()

	// Close should not error
	err := wsConn.Close()
	if err != nil {
		t.Errorf("Close error: %v", err)
	}

	if !wsConn.IsClosed() {
		t.Error("Connection should be closed")
	}

	// Double close should not error
	err = wsConn.Close()
	if err != nil {
		t.Errorf("Double close error: %v", err)
	}
}

// Test WebSocket CloseWithCode
func TestWebSocket_CloseWithCode(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()

	wsConn := &WebSocketConn{
		conn:   serverConn,
		reader: bufio.NewReader(serverConn),
		writer: bufio.NewWriter(serverConn),
	}

	// Drain client side
	go func() {
		buf := make([]byte, 1024)
		for {
			_, err := clientConn.Read(buf)
			if err != nil {
				return
			}
		}
	}()

	err := wsConn.CloseWithCode(1001, "going away")
	if err != nil {
		t.Errorf("CloseWithCode error: %v", err)
	}

	if !wsConn.IsClosed() {
		t.Error("Connection should be closed")
	}
}

// Test writing to closed connection
func TestWebSocket_WriteAfterClose(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()

	wsConn := &WebSocketConn{
		conn:   serverConn,
		reader: bufio.NewReader(serverConn),
		writer: bufio.NewWriter(serverConn),
	}

	// Drain client side
	go func() {
		buf := make([]byte, 1024)
		for {
			_, err := clientConn.Read(buf)
			if err != nil {
				return
			}
		}
	}()

	wsConn.Close()

	err := wsConn.Send("test")
	if err != ErrWebSocketClosed {
		t.Errorf("Expected ErrWebSocketClosed, got %v", err)
	}
}

// Integration test with full HTTP server
func TestWebSocket_Integration(t *testing.T) {
	var connected bool
	var mu sync.Mutex

	handler := &WebSocketHandler{
		OnConnect: func(conn *WebSocketConn) {
			mu.Lock()
			connected = true
			mu.Unlock()
		},
		OnMessage: func(conn *WebSocketConn, messageType int, data []byte) {
			conn.Send("echo: " + string(data))
		},
	}

	engine := New()
	engine.GET("/ws", WebSocket(handler))

	server := httptest.NewServer(engine)
	defer server.Close()

	// Convert HTTP URL to WebSocket URL
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"

	// Create WebSocket client connection
	conn, err := net.Dial("tcp", strings.TrimPrefix(server.URL, "http://"))
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Send WebSocket upgrade request
	key := base64.StdEncoding.EncodeToString([]byte("test-key-12345678"))
	request := "GET /ws HTTP/1.1\r\n" +
		"Host: " + strings.TrimPrefix(server.URL, "http://") + "\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Key: " + key + "\r\n" +
		"Sec-WebSocket-Version: 13\r\n\r\n"

	conn.Write([]byte(request))

	// Read response
	reader := bufio.NewReader(conn)
	response, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	if !strings.Contains(response, "101") {
		t.Errorf("Expected 101 Switching Protocols, got: %s", response)
	}

	// Skip remaining headers
	for {
		line, _ := reader.ReadString('\n')
		if line == "\r\n" {
			break
		}
	}

	// Give server time to process
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	if !connected {
		t.Error("OnConnect was not called")
	}
	mu.Unlock()

	_ = wsURL // Used for documentation
}
