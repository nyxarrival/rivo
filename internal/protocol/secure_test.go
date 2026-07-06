package protocol

import (
	"bytes"
	"testing"
)

func TestSecureConnRoundTrip(t *testing.T) {
	key := bytes.Repeat([]byte{1}, SessionKeySize)
	var wire bytes.Buffer

	sender := NewSecureConn(&wire, key, key)
	if err := sender.WriteMessage(Message{
		Type:      MessageTypeMetrics,
		NodeID:    "node-a",
		Seq:       10,
		Timestamp: 123,
		Payload: map[string]any{
			"cpu_usage": 12.5,
		},
	}); err != nil {
		t.Fatalf("write encrypted message: %v", err)
	}

	receiver := NewSecureConn(&wire, key, key)
	got, err := receiver.ReadMessage()
	if err != nil {
		t.Fatalf("read encrypted message: %v", err)
	}
	if got.Type != MessageTypeMetrics {
		t.Fatalf("Type = %q, want %q", got.Type, MessageTypeMetrics)
	}
	if got.NodeID != "node-a" {
		t.Fatalf("NodeID = %q, want %q", got.NodeID, "node-a")
	}
	if got.Seq != 10 {
		t.Fatalf("Seq = %d, want %d", got.Seq, 10)
	}
}

func TestSecureConnRejectsReplay(t *testing.T) {
	key := bytes.Repeat([]byte{1}, SessionKeySize)
	var first bytes.Buffer
	sender := NewSecureConn(&first, key, key)
	if err := sender.WriteMessage(Message{Type: MessageTypeHeartbeat, NodeID: "node-a"}); err != nil {
		t.Fatalf("write encrypted message: %v", err)
	}

	raw := append([]byte{}, first.Bytes()...)
	wire := bytes.NewBuffer(append(raw, raw...))
	receiver := NewSecureConn(wire, key, key)
	if _, err := receiver.ReadMessage(); err != nil {
		t.Fatalf("first read failed: %v", err)
	}
	if _, err := receiver.ReadMessage(); err == nil {
		t.Fatal("second read succeeded, want replay error")
	}
}

func TestRegisterAuthCoversNonce(t *testing.T) {
	payload := RegisterPayload{
		AgentVersion: "v1",
		Hostname:     "node-a",
		PublicIP:     "192.0.2.1",
		Nonce:        bytes.Repeat([]byte{1}, HandshakeSize),
	}
	payload.Auth = RegisterAuth("secret", payload, 123)
	if !VerifyRegisterAuth("secret", payload, 123) {
		t.Fatal("VerifyRegisterAuth returned false")
	}

	payload.Nonce = bytes.Repeat([]byte{2}, HandshakeSize)
	if VerifyRegisterAuth("secret", payload, 123) {
		t.Fatal("VerifyRegisterAuth returned true after nonce tampering")
	}
}

func TestRegisterAuthCoversIPAddresses(t *testing.T) {
	payload := RegisterPayload{
		AgentVersion: "v1",
		Hostname:     "node-a",
		PublicIP:     "192.0.2.1",
		IPAddresses:  IPAddresses{IPv4: []string{"192.0.2.1"}, IPv6: []string{"2001:db8::1"}},
		Nonce:        bytes.Repeat([]byte{1}, HandshakeSize),
	}
	payload.Auth = RegisterAuth("secret", payload, 123)
	if !VerifyRegisterAuth("secret", payload, 123) {
		t.Fatal("VerifyRegisterAuth returned false")
	}

	payload.IPAddresses.IPv6 = []string{"2001:db8::2"}
	if VerifyRegisterAuth("secret", payload, 123) {
		t.Fatal("VerifyRegisterAuth returned true after ip_addresses tampering")
	}
}

func TestRegisterAuthCoversPublicIPs(t *testing.T) {
	payload := RegisterPayload{
		AgentVersion: "v1",
		Hostname:     "node-a",
		PublicIP:     "192.0.2.1",
		PublicIPs: PublicIPs{
			IPv4: []PublicIPObservation{{IP: "192.0.2.1", Source: "agent_http", LastSeen: 123}},
			IPv6: []PublicIPObservation{{IP: "2001:db8::1", Source: "agent_http", LastSeen: 123}},
		},
		Nonce: bytes.Repeat([]byte{1}, HandshakeSize),
	}
	payload.Auth = RegisterAuth("secret", payload, 123)
	if !VerifyRegisterAuth("secret", payload, 123) {
		t.Fatal("VerifyRegisterAuth returned false")
	}

	payload.PublicIPs.IPv4[0].IP = "192.0.2.2"
	if VerifyRegisterAuth("secret", payload, 123) {
		t.Fatal("VerifyRegisterAuth returned true after public_ips tampering")
	}
}

func TestVerifyRegisterAuthAcceptsLegacyPayload(t *testing.T) {
	payload := RegisterPayload{
		AgentVersion: "v1",
		Hostname:     "node-a",
		PublicIP:     "192.0.2.1",
		Nonce:        bytes.Repeat([]byte{1}, HandshakeSize),
	}
	payload.Auth = registerAuth("secret", registerAuthBaseLegacy(RegisterPayload{
		AgentVersion: payload.AgentVersion,
		Hostname:     payload.Hostname,
		PublicIP:     payload.PublicIP,
		Nonce:        payload.Nonce,
	}, 123))
	if !VerifyRegisterAuth("secret", payload, 123) {
		t.Fatal("VerifyRegisterAuth rejected legacy auth")
	}
}
