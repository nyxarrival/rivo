package protocol

type HelloPayload struct {
	NodeID     string `json:"node_id"`
	Nonce     []byte `json:"nonce"`
	AgentName  string `json:"agent_name,omitempty"`
	Version    string `json:"version,omitempty"`
}
type HelloAckPayload struct {
	Nonce []byte `json:"nonce"`
}
