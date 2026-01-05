package rue

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/http"
	"sync"
	"time"
)

// WebSocket constants
const (
	// Frame types
	TextMessage   = 1
	BinaryMessage = 2
	CloseMessage  = 8
	PingMessage   = 9
	PongMessage   = 10

	// WebSocket GUID for handshake
	websocketGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
)

// WebSocket errors
var (
	ErrWebSocketUpgrade     = errors.New("websocket: upgrade failed")
	ErrWebSocketClosed      = errors.New("websocket: connection closed")
	ErrWebSocketInvalidData = errors.New("websocket: invalid data")
)

// WebSocketConn represents a WebSocket connection
type WebSocketConn struct {
	conn       net.Conn
	reader     *bufio.Reader
	writer     *bufio.Writer
	mu         sync.Mutex
	closed     bool
	closeMu    sync.RWMutex
	pingPeriod time.Duration
	pongWait   time.Duration
	data       map[string]any // Custom data storage
}

// WebSocketHandler defines callbacks for WebSocket events
type WebSocketHandler struct {
	OnConnect func(conn *WebSocketConn)
	OnMessage func(conn *WebSocketConn, messageType int, data []byte)
	OnClose   func(conn *WebSocketConn, code int, reason string)
	OnError   func(conn *WebSocketConn, err error)
	OnPing    func(conn *WebSocketConn, data []byte)
	OnPong    func(conn *WebSocketConn, data []byte)
}

// WebSocketConfig defines WebSocket configuration
type WebSocketConfig struct {
	ReadBufferSize  int
	WriteBufferSize int
	PingPeriod      time.Duration
	PongWait        time.Duration
	MaxMessageSize  int64
}

// DefaultWebSocketConfig returns default WebSocket configuration
func DefaultWebSocketConfig() WebSocketConfig {
	return WebSocketConfig{
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
		PingPeriod:      30 * time.Second,
		PongWait:        60 * time.Second,
		MaxMessageSize:  512 * 1024, // 512 KB
	}
}

// WebSocket returns a WebSocket handler middleware
func WebSocket(handler *WebSocketHandler) HandlerFunc {
	return WebSocketWithConfig(handler, DefaultWebSocketConfig())
}

// WebSocketWithConfig returns a WebSocket handler with custom config
func WebSocketWithConfig(handler *WebSocketHandler, config WebSocketConfig) HandlerFunc {
	if config.ReadBufferSize == 0 {
		config.ReadBufferSize = 4096
	}
	if config.WriteBufferSize == 0 {
		config.WriteBufferSize = 4096
	}
	if config.PingPeriod == 0 {
		config.PingPeriod = 30 * time.Second
	}
	if config.PongWait == 0 {
		config.PongWait = 60 * time.Second
	}

	return func(c *Context) {
		conn, err := upgradeWebSocket(c, config)
		if err != nil {
			if handler.OnError != nil {
				handler.OnError(nil, err)
			}
			return
		}

		// Call OnConnect
		if handler.OnConnect != nil {
			handler.OnConnect(conn)
		}

		// Start reading messages
		go conn.readLoop(handler)
	}
}

// upgradeWebSocket performs the WebSocket handshake
func upgradeWebSocket(c *Context, config WebSocketConfig) (*WebSocketConn, error) {
	// Check headers
	if c.Header("Upgrade") != "websocket" {
		c.AbortWithStatus(http.StatusBadRequest)
		return nil, ErrWebSocketUpgrade
	}

	key := c.Header("Sec-WebSocket-Key")
	if key == "" {
		c.AbortWithStatus(http.StatusBadRequest)
		return nil, ErrWebSocketUpgrade
	}

	// Calculate accept key
	acceptKey := computeAcceptKey(key)

	// Hijack the connection
	hijacker, ok := c.Writer.(http.Hijacker)
	if !ok {
		c.AbortWithStatus(http.StatusInternalServerError)
		return nil, ErrWebSocketUpgrade
	}

	conn, bufrw, err := hijacker.Hijack()
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return nil, err
	}

	// Send upgrade response
	response := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + acceptKey + "\r\n\r\n"

	if _, err := bufrw.WriteString(response); err != nil {
		conn.Close()
		return nil, err
	}
	if err := bufrw.Flush(); err != nil {
		conn.Close()
		return nil, err
	}

	wsConn := &WebSocketConn{
		conn:       conn,
		reader:     bufrw.Reader,
		writer:     bufrw.Writer,
		pingPeriod: config.PingPeriod,
		pongWait:   config.PongWait,
	}

	return wsConn, nil
}

// computeAcceptKey computes the Sec-WebSocket-Accept key
func computeAcceptKey(key string) string {
	h := sha1.New()
	h.Write([]byte(key + websocketGUID))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// readLoop reads messages from the WebSocket connection
func (c *WebSocketConn) readLoop(handler *WebSocketHandler) {
	defer func() {
		c.Close()
		if handler.OnClose != nil {
			handler.OnClose(c, 1000, "")
		}
	}()

	for {
		messageType, data, err := c.ReadMessage()
		if err != nil {
			if handler.OnError != nil && !c.IsClosed() {
				handler.OnError(c, err)
			}
			return
		}

		switch messageType {
		case TextMessage, BinaryMessage:
			if handler.OnMessage != nil {
				handler.OnMessage(c, messageType, data)
			}
		case PingMessage:
			if handler.OnPing != nil {
				handler.OnPing(c, data)
			}
			c.Pong(data)
		case PongMessage:
			if handler.OnPong != nil {
				handler.OnPong(c, data)
			}
		case CloseMessage:
			return
		}
	}
}

// ReadMessage reads a message from the WebSocket connection
func (c *WebSocketConn) ReadMessage() (messageType int, data []byte, err error) {
	if c.IsClosed() {
		return 0, nil, ErrWebSocketClosed
	}

	// Read frame header
	header := make([]byte, 2)
	if _, err := io.ReadFull(c.reader, header); err != nil {
		return 0, nil, err
	}

	// Parse header
	fin := header[0]&0x80 != 0
	opcode := int(header[0] & 0x0F)
	masked := header[1]&0x80 != 0
	payloadLen := int(header[1] & 0x7F)

	// Extended payload length
	if payloadLen == 126 {
		extLen := make([]byte, 2)
		if _, err := io.ReadFull(c.reader, extLen); err != nil {
			return 0, nil, err
		}
		payloadLen = int(binary.BigEndian.Uint16(extLen))
	} else if payloadLen == 127 {
		extLen := make([]byte, 8)
		if _, err := io.ReadFull(c.reader, extLen); err != nil {
			return 0, nil, err
		}
		payloadLen = int(binary.BigEndian.Uint64(extLen))
	}

	// Read masking key (if present)
	var maskKey []byte
	if masked {
		maskKey = make([]byte, 4)
		if _, err := io.ReadFull(c.reader, maskKey); err != nil {
			return 0, nil, err
		}
	}

	// Read payload
	payload := make([]byte, payloadLen)
	if _, err := io.ReadFull(c.reader, payload); err != nil {
		return 0, nil, err
	}

	// Unmask payload
	if masked {
		for i := range payload {
			payload[i] ^= maskKey[i%4]
		}
	}

	// Handle fragmentation (simplified - only handle single frames)
	if !fin {
		// For simplicity, we don't handle fragmented messages
		return 0, nil, ErrWebSocketInvalidData
	}

	return opcode, payload, nil
}

// WriteMessage writes a message to the WebSocket connection
func (c *WebSocketConn) WriteMessage(messageType int, data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.IsClosed() {
		return ErrWebSocketClosed
	}

	return c.writeFrame(messageType, data)
}

// writeFrame writes a WebSocket frame
func (c *WebSocketConn) writeFrame(opcode int, data []byte) error {
	// Build frame header
	var header []byte
	payloadLen := len(data)

	// First byte: FIN + opcode
	firstByte := byte(0x80 | opcode) // FIN = 1

	if payloadLen < 126 {
		header = []byte{firstByte, byte(payloadLen)}
	} else if payloadLen < 65536 {
		header = make([]byte, 4)
		header[0] = firstByte
		header[1] = 126
		binary.BigEndian.PutUint16(header[2:], uint16(payloadLen))
	} else {
		header = make([]byte, 10)
		header[0] = firstByte
		header[1] = 127
		binary.BigEndian.PutUint64(header[2:], uint64(payloadLen))
	}

	// Write header
	if _, err := c.writer.Write(header); err != nil {
		return err
	}

	// Write payload
	if _, err := c.writer.Write(data); err != nil {
		return err
	}

	return c.writer.Flush()
}

// Send sends a text message
func (c *WebSocketConn) Send(message string) error {
	return c.WriteMessage(TextMessage, []byte(message))
}

// SendBinary sends a binary message
func (c *WebSocketConn) SendBinary(data []byte) error {
	return c.WriteMessage(BinaryMessage, data)
}

// Ping sends a ping message
func (c *WebSocketConn) Ping(data []byte) error {
	return c.WriteMessage(PingMessage, data)
}

// Pong sends a pong message
func (c *WebSocketConn) Pong(data []byte) error {
	return c.WriteMessage(PongMessage, data)
}

// Close closes the WebSocket connection
func (c *WebSocketConn) Close() error {
	c.closeMu.Lock()
	defer c.closeMu.Unlock()

	if c.closed {
		return nil
	}
	c.closed = true

	// Send close frame
	c.mu.Lock()
	c.writeFrame(CloseMessage, []byte{0x03, 0xE8}) // 1000 = normal closure
	c.mu.Unlock()

	return c.conn.Close()
}

// CloseWithCode closes the connection with a specific code and reason
func (c *WebSocketConn) CloseWithCode(code int, reason string) error {
	c.closeMu.Lock()
	defer c.closeMu.Unlock()

	if c.closed {
		return nil
	}
	c.closed = true

	// Build close frame payload
	payload := make([]byte, 2+len(reason))
	binary.BigEndian.PutUint16(payload, uint16(code))
	copy(payload[2:], reason)

	c.mu.Lock()
	c.writeFrame(CloseMessage, payload)
	c.mu.Unlock()

	return c.conn.Close()
}

// IsClosed returns true if the connection is closed
func (c *WebSocketConn) IsClosed() bool {
	c.closeMu.RLock()
	defer c.closeMu.RUnlock()
	return c.closed
}

// RemoteAddr returns the remote address
func (c *WebSocketConn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

// LocalAddr returns the local address
func (c *WebSocketConn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

// ============== WebSocket Hub ==============

// WebSocketHub manages WebSocket connections
type WebSocketHub struct {
	connections map[*WebSocketConn]bool
	rooms       map[string]map[*WebSocketConn]bool
	mu          sync.RWMutex
}

// NewWebSocketHub creates a new WebSocket hub
func NewWebSocketHub() *WebSocketHub {
	return &WebSocketHub{
		connections: make(map[*WebSocketConn]bool),
		rooms:       make(map[string]map[*WebSocketConn]bool),
	}
}

// Register adds a connection to the hub
func (h *WebSocketHub) Register(conn *WebSocketConn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.connections[conn] = true
}

// Unregister removes a connection from the hub
func (h *WebSocketHub) Unregister(conn *WebSocketConn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.connections, conn)

	// Remove from all rooms
	for _, room := range h.rooms {
		delete(room, conn)
	}
}

// Join adds a connection to a room
func (h *WebSocketHub) Join(conn *WebSocketConn, room string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.rooms[room] == nil {
		h.rooms[room] = make(map[*WebSocketConn]bool)
	}
	h.rooms[room][conn] = true
}

// Leave removes a connection from a room
func (h *WebSocketHub) Leave(conn *WebSocketConn, room string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.rooms[room] != nil {
		delete(h.rooms[room], conn)
	}
}

// Broadcast sends a message to all connections
func (h *WebSocketHub) Broadcast(message string) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for conn := range h.connections {
		conn.Send(message)
	}
}

// BroadcastBinary sends binary data to all connections
func (h *WebSocketHub) BroadcastBinary(data []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for conn := range h.connections {
		conn.SendBinary(data)
	}
}

// BroadcastToRoom sends a message to all connections in a room
func (h *WebSocketHub) BroadcastToRoom(room, message string) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.rooms[room] != nil {
		for conn := range h.rooms[room] {
			conn.Send(message)
		}
	}
}

// BroadcastBinaryToRoom sends binary data to all connections in a room
func (h *WebSocketHub) BroadcastBinaryToRoom(room string, data []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.rooms[room] != nil {
		for conn := range h.rooms[room] {
			conn.SendBinary(data)
		}
	}
}

// Count returns the number of connections
func (h *WebSocketHub) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.connections)
}

// RoomCount returns the number of connections in a room
func (h *WebSocketHub) RoomCount(room string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.rooms[room] != nil {
		return len(h.rooms[room])
	}
	return 0
}
