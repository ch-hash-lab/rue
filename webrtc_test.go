package rue

import (
	"encoding/json"
	"testing"
)

func TestSignalingServer_AddRemovePeer(t *testing.T) {
	server := NewSignalingServer()

	// Add peer
	conn := &WebSocketConn{}
	peer := server.AddPeer("peer1", conn)

	if peer.ID != "peer1" {
		t.Errorf("Peer ID = %s, want peer1", peer.ID)
	}

	if server.PeerCount() != 1 {
		t.Errorf("PeerCount() = %d, want 1", server.PeerCount())
	}

	// Get peer
	retrieved := server.GetPeer("peer1")
	if retrieved != peer {
		t.Error("GetPeer returned wrong peer")
	}

	// Remove peer
	server.RemovePeer("peer1")

	if server.PeerCount() != 0 {
		t.Errorf("PeerCount() = %d, want 0", server.PeerCount())
	}

	if server.GetPeer("peer1") != nil {
		t.Error("Peer should be removed")
	}
}

func TestSignalingServer_JoinLeaveRoom(t *testing.T) {
	server := NewSignalingServer()

	conn1 := &WebSocketConn{}
	conn2 := &WebSocketConn{}

	server.AddPeer("peer1", conn1)
	server.AddPeer("peer2", conn2)

	// Join room
	server.JoinRoom("peer1", "room1")
	server.JoinRoom("peer2", "room1")

	if server.RoomCount() != 1 {
		t.Errorf("RoomCount() = %d, want 1", server.RoomCount())
	}

	peers := server.GetRoomPeers("room1")
	if len(peers) != 2 {
		t.Errorf("Room peer count = %d, want 2", len(peers))
	}

	// Leave room
	server.LeaveRoom("peer1")

	peers = server.GetRoomPeers("room1")
	if len(peers) != 1 {
		t.Errorf("Room peer count = %d, want 1", len(peers))
	}

	// Leave last peer
	server.LeaveRoom("peer2")

	if server.RoomCount() != 0 {
		t.Errorf("RoomCount() = %d, want 0 (empty room should be removed)", server.RoomCount())
	}
}

func TestSignalingServer_SwitchRoom(t *testing.T) {
	server := NewSignalingServer()

	conn := &WebSocketConn{}
	server.AddPeer("peer1", conn)

	// Join first room
	server.JoinRoom("peer1", "room1")

	peers := server.GetRoomPeers("room1")
	if len(peers) != 1 {
		t.Errorf("Room1 peer count = %d, want 1", len(peers))
	}

	// Switch to second room
	server.JoinRoom("peer1", "room2")

	peers = server.GetRoomPeers("room1")
	if len(peers) != 0 {
		t.Errorf("Room1 peer count = %d, want 0", len(peers))
	}

	peers = server.GetRoomPeers("room2")
	if len(peers) != 1 {
		t.Errorf("Room2 peer count = %d, want 1", len(peers))
	}
}

func TestSignalingServer_RemovePeerFromRoom(t *testing.T) {
	server := NewSignalingServer()

	conn := &WebSocketConn{}
	server.AddPeer("peer1", conn)
	server.JoinRoom("peer1", "room1")

	// Remove peer should also remove from room
	server.RemovePeer("peer1")

	if server.RoomCount() != 0 {
		t.Errorf("RoomCount() = %d, want 0", server.RoomCount())
	}
}

func TestSignalingServer_HandleJoinMessage(t *testing.T) {
	server := NewSignalingServer()

	var joinedPeer *SignalingPeer
	server.OnPeerJoin = func(peer *SignalingPeer) {
		joinedPeer = peer
	}

	conn := &WebSocketConn{}
	server.AddPeer("peer1", conn)

	msg := SignalMessage{
		Type: SignalJoin,
		Room: "room1",
	}
	data, _ := json.Marshal(msg)

	err := server.HandleMessage("peer1", data)
	if err != nil {
		t.Errorf("HandleMessage error: %v", err)
	}

	if joinedPeer == nil {
		t.Error("OnPeerJoin should be called")
	}

	if joinedPeer.Room != "room1" {
		t.Errorf("Peer room = %s, want room1", joinedPeer.Room)
	}
}

func TestSignalingServer_HandleLeaveMessage(t *testing.T) {
	server := NewSignalingServer()

	conn := &WebSocketConn{}
	server.AddPeer("peer1", conn)
	server.JoinRoom("peer1", "room1")

	msg := SignalMessage{
		Type: SignalLeave,
	}
	data, _ := json.Marshal(msg)

	err := server.HandleMessage("peer1", data)
	if err != nil {
		t.Errorf("HandleMessage error: %v", err)
	}

	peer := server.GetPeer("peer1")
	if peer.Room != "" {
		t.Errorf("Peer room = %s, want empty", peer.Room)
	}
}

func TestSignalingServer_Callbacks(t *testing.T) {
	server := NewSignalingServer()

	var joinCalled, leaveCalled, messageCalled bool

	server.OnPeerJoin = func(peer *SignalingPeer) {
		joinCalled = true
	}
	server.OnPeerLeave = func(peer *SignalingPeer) {
		leaveCalled = true
	}
	server.OnMessage = func(msg *SignalMessage) {
		messageCalled = true
	}

	conn := &WebSocketConn{}
	server.AddPeer("peer1", conn)
	server.JoinRoom("peer1", "room1")

	if !joinCalled {
		t.Error("OnPeerJoin should be called")
	}

	msg := SignalMessage{Type: SignalOffer}
	data, _ := json.Marshal(msg)
	server.HandleMessage("peer1", data)

	if !messageCalled {
		t.Error("OnMessage should be called")
	}

	server.RemovePeer("peer1")

	if !leaveCalled {
		t.Error("OnPeerLeave should be called")
	}
}

func TestSignalMessage_JSON(t *testing.T) {
	msg := SignalMessage{
		Type:    SignalOffer,
		From:    "peer1",
		To:      "peer2",
		Room:    "room1",
		Payload: map[string]string{"sdp": "test"},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded SignalMessage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.Type != SignalOffer {
		t.Errorf("Type = %s, want offer", decoded.Type)
	}
	if decoded.From != "peer1" {
		t.Errorf("From = %s, want peer1", decoded.From)
	}
	if decoded.To != "peer2" {
		t.Errorf("To = %s, want peer2", decoded.To)
	}
}

func TestSDPMessage(t *testing.T) {
	sdp := SDPMessage{
		Type: "offer",
		SDP:  "v=0\r\no=- 123 456 IN IP4 127.0.0.1\r\n",
	}

	data, err := json.Marshal(sdp)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded SDPMessage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.Type != "offer" {
		t.Errorf("Type = %s, want offer", decoded.Type)
	}
}

func TestICECandidate(t *testing.T) {
	candidate := ICECandidate{
		Candidate:     "candidate:1 1 UDP 2130706431 192.168.1.1 54321 typ host",
		SDPMid:        "0",
		SDPMLineIndex: 0,
	}

	data, err := json.Marshal(candidate)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded ICECandidate
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.SDPMid != "0" {
		t.Errorf("SDPMid = %s, want 0", decoded.SDPMid)
	}
}

func TestWebSocketConn_SetGet(t *testing.T) {
	conn := &WebSocketConn{}

	conn.Set("key1", "value1")
	conn.Set("key2", 123)

	val1, ok := conn.Get("key1")
	if !ok || val1 != "value1" {
		t.Errorf("Get(key1) = %v, want value1", val1)
	}

	val2, ok := conn.Get("key2")
	if !ok || val2 != 123 {
		t.Errorf("Get(key2) = %v, want 123", val2)
	}

	_, ok = conn.Get("nonexistent")
	if ok {
		t.Error("Get(nonexistent) should return false")
	}
}

func TestSignalingServer_WebSocketHandler(t *testing.T) {
	server := NewSignalingServer()
	handler := server.WebSocketHandler()

	if handler.OnConnect == nil {
		t.Error("OnConnect should be set")
	}
	if handler.OnMessage == nil {
		t.Error("OnMessage should be set")
	}
	if handler.OnClose == nil {
		t.Error("OnClose should be set")
	}
}

func TestGeneratePeerID(t *testing.T) {
	id1 := generatePeerID()
	id2 := generatePeerID()

	if id1 == id2 {
		t.Error("Generated IDs should be unique")
	}

	if len(id1) == 0 {
		t.Error("Generated ID should not be empty")
	}
}

func TestSignalingServer_GetRoomPeers_Empty(t *testing.T) {
	server := NewSignalingServer()

	peers := server.GetRoomPeers("nonexistent")
	if len(peers) != 0 {
		t.Errorf("GetRoomPeers(nonexistent) = %d, want 0", len(peers))
	}
}

func TestSignalingServer_LeaveRoom_NotInRoom(t *testing.T) {
	server := NewSignalingServer()

	conn := &WebSocketConn{}
	server.AddPeer("peer1", conn)

	// Should not panic
	server.LeaveRoom("peer1")
	server.LeaveRoom("nonexistent")
}
