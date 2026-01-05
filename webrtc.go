package rue

import (
	"encoding/json"
	"sync"
)

// ============== WebRTC Signaling ==============

// SignalType represents the type of signaling message
type SignalType string

const (
	SignalOffer     SignalType = "offer"
	SignalAnswer    SignalType = "answer"
	SignalCandidate SignalType = "candidate"
	SignalJoin      SignalType = "join"
	SignalLeave     SignalType = "leave"
)

// SignalMessage represents a WebRTC signaling message
type SignalMessage struct {
	Type    SignalType `json:"type"`
	From    string     `json:"from"`
	To      string     `json:"to,omitempty"`
	Room    string     `json:"room,omitempty"`
	Payload any        `json:"payload,omitempty"`
}

// SDPMessage represents an SDP offer/answer
type SDPMessage struct {
	Type string `json:"type"` // "offer" or "answer"
	SDP  string `json:"sdp"`
}

// ICECandidate represents an ICE candidate
type ICECandidate struct {
	Candidate     string `json:"candidate"`
	SDPMid        string `json:"sdpMid"`
	SDPMLineIndex int    `json:"sdpMLineIndex"`
}

// SignalingPeer represents a peer in the signaling server
type SignalingPeer struct {
	ID   string
	Conn *WebSocketConn
	Room string
}

// SignalingServer manages WebRTC signaling
type SignalingServer struct {
	peers map[string]*SignalingPeer
	rooms map[string]map[string]*SignalingPeer
	mu    sync.RWMutex

	// Callbacks
	OnPeerJoin  func(peer *SignalingPeer)
	OnPeerLeave func(peer *SignalingPeer)
	OnMessage   func(msg *SignalMessage)
}

// NewSignalingServer creates a new signaling server
func NewSignalingServer() *SignalingServer {
	return &SignalingServer{
		peers: make(map[string]*SignalingPeer),
		rooms: make(map[string]map[string]*SignalingPeer),
	}
}

// AddPeer adds a peer to the signaling server
func (s *SignalingServer) AddPeer(id string, conn *WebSocketConn) *SignalingPeer {
	s.mu.Lock()
	defer s.mu.Unlock()

	peer := &SignalingPeer{
		ID:   id,
		Conn: conn,
	}
	s.peers[id] = peer

	return peer
}

// RemovePeer removes a peer from the signaling server
func (s *SignalingServer) RemovePeer(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	peer, exists := s.peers[id]
	if !exists {
		return
	}

	// Remove from room
	if peer.Room != "" {
		if room, ok := s.rooms[peer.Room]; ok {
			delete(room, id)
			if len(room) == 0 {
				delete(s.rooms, peer.Room)
			}
		}
	}

	delete(s.peers, id)

	if s.OnPeerLeave != nil {
		s.OnPeerLeave(peer)
	}
}

// JoinRoom adds a peer to a room
func (s *SignalingServer) JoinRoom(peerID, roomID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	peer, exists := s.peers[peerID]
	if !exists {
		return
	}

	// Leave current room
	if peer.Room != "" {
		if room, ok := s.rooms[peer.Room]; ok {
			delete(room, peerID)
			if len(room) == 0 {
				delete(s.rooms, peer.Room)
			}
		}
	}

	// Join new room
	peer.Room = roomID
	if s.rooms[roomID] == nil {
		s.rooms[roomID] = make(map[string]*SignalingPeer)
	}
	s.rooms[roomID][peerID] = peer

	if s.OnPeerJoin != nil {
		s.OnPeerJoin(peer)
	}
}

// LeaveRoom removes a peer from their current room
func (s *SignalingServer) LeaveRoom(peerID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	peer, exists := s.peers[peerID]
	if !exists || peer.Room == "" {
		return
	}

	if room, ok := s.rooms[peer.Room]; ok {
		delete(room, peerID)
		if len(room) == 0 {
			delete(s.rooms, peer.Room)
		}
	}

	peer.Room = ""
}

// GetPeer returns a peer by ID
func (s *SignalingServer) GetPeer(id string) *SignalingPeer {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.peers[id]
}

// GetRoomPeers returns all peers in a room
func (s *SignalingServer) GetRoomPeers(roomID string) []*SignalingPeer {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var peers []*SignalingPeer
	if room, ok := s.rooms[roomID]; ok {
		for _, peer := range room {
			peers = append(peers, peer)
		}
	}
	return peers
}

// SendTo sends a message to a specific peer
func (s *SignalingServer) SendTo(peerID string, msg *SignalMessage) error {
	s.mu.RLock()
	peer, exists := s.peers[peerID]
	s.mu.RUnlock()

	if !exists {
		return nil
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return peer.Conn.Send(string(data))
}

// Broadcast sends a message to all peers in a room
func (s *SignalingServer) Broadcast(roomID string, msg *SignalMessage, excludePeerID string) {
	s.mu.RLock()
	room, exists := s.rooms[roomID]
	s.mu.RUnlock()

	if !exists {
		return
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	for id, peer := range room {
		if id != excludePeerID {
			peer.Conn.Send(string(data))
		}
	}
}

// HandleMessage processes a signaling message
func (s *SignalingServer) HandleMessage(peerID string, data []byte) error {
	var msg SignalMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return err
	}

	msg.From = peerID

	if s.OnMessage != nil {
		s.OnMessage(&msg)
	}

	switch msg.Type {
	case SignalJoin:
		if msg.Room != "" {
			s.JoinRoom(peerID, msg.Room)
			// Notify other peers in the room
			s.Broadcast(msg.Room, &SignalMessage{
				Type: SignalJoin,
				From: peerID,
				Room: msg.Room,
			}, peerID)
		}

	case SignalLeave:
		peer := s.GetPeer(peerID)
		if peer != nil && peer.Room != "" {
			room := peer.Room
			s.LeaveRoom(peerID)
			// Notify other peers
			s.Broadcast(room, &SignalMessage{
				Type: SignalLeave,
				From: peerID,
				Room: room,
			}, peerID)
		}

	case SignalOffer, SignalAnswer, SignalCandidate:
		// Forward to specific peer
		if msg.To != "" {
			s.SendTo(msg.To, &msg)
		}
	}

	return nil
}

// WebSocketHandler returns a WebSocket handler for signaling
func (s *SignalingServer) WebSocketHandler() *WebSocketHandler {
	return &WebSocketHandler{
		OnConnect: func(conn *WebSocketConn) {
			// Generate peer ID (in real app, this would come from auth)
			peerID := generatePeerID()
			conn.Set("peerID", peerID)
			s.AddPeer(peerID, conn)

			// Send peer ID to client
			msg := SignalMessage{
				Type:    "connected",
				Payload: map[string]string{"peerId": peerID},
			}
			data, _ := json.Marshal(msg)
			conn.Send(string(data))
		},
		OnMessage: func(conn *WebSocketConn, messageType int, data []byte) {
			peerID, _ := conn.Get("peerID")
			if id, ok := peerID.(string); ok {
				s.HandleMessage(id, data)
			}
		},
		OnClose: func(conn *WebSocketConn, code int, reason string) {
			peerID, _ := conn.Get("peerID")
			if id, ok := peerID.(string); ok {
				peer := s.GetPeer(id)
				if peer != nil && peer.Room != "" {
					// Notify room peers
					s.Broadcast(peer.Room, &SignalMessage{
						Type: SignalLeave,
						From: id,
						Room: peer.Room,
					}, id)
				}
				s.RemovePeer(id)
			}
		},
	}
}

// PeerCount returns the number of connected peers
func (s *SignalingServer) PeerCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.peers)
}

// RoomCount returns the number of active rooms
func (s *SignalingServer) RoomCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.rooms)
}

// Helper to store data on WebSocketConn
func (c *WebSocketConn) Set(key string, value any) {
	// Use a simple approach - store in a map
	if c.data == nil {
		c.data = make(map[string]any)
	}
	c.data[key] = value
}

func (c *WebSocketConn) Get(key string) (any, bool) {
	if c.data == nil {
		return nil, false
	}
	val, ok := c.data[key]
	return val, ok
}

// Add data field to WebSocketConn (need to update websocket.go)
var (
	peerIDCounter int
	peerIDMu      sync.Mutex
)

func generatePeerID() string {
	peerIDMu.Lock()
	defer peerIDMu.Unlock()
	peerIDCounter++
	return "peer-" + string(rune('0'+peerIDCounter%10)) + string(rune('0'+peerIDCounter/10%10))
}
