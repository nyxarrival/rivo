package protocol

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

const (
	MessageTypeRegister        = "register"
	MessageTypeRegisterAck     = "register_ack"
	MessageTypeEncrypted       = "encrypted"
	MessageTypeHeartbeat       = "heartbeat"
	MessageTypeMetrics         = "metrics"
	MessageTypeLog             = "log"
	MessageTypeProbeResults    = "probe_results"
	MessageTypeSnapshotResults = "snapshot_results"
	MessageTypeTaskDispatch    = "task_dispatch"
	MessageTypeTaskResult      = "task_result"
	MessageTypeRequestMetrics  = "request_metrics"
	MessageTypeConfigUpdate    = "config_update"
	MessageTypeAck             = "ack"
	MessageTypeError           = "error"
)

type Message struct {
	Type      string         `json:"type"`
	RequestID string         `json:"request_id,omitempty"`
	NodeID    string         `json:"node_id,omitempty"`
	Seq       uint64         `json:"seq,omitempty"`
	Timestamp int64          `json:"timestamp"`
	Payload   map[string]any `json:"payload,omitempty"`
}

type MetricsPayload struct {
	CPUUsage        float64 `json:"cpu_usage"`
	CPUCores        uint32  `json:"cpu_cores"`
	Arch            string  `json:"arch"`
	Virtualization  string  `json:"virtualization"`
	GPU             string  `json:"gpu"`
	OSName          string  `json:"os_name"`
	Load1           float64 `json:"load1"`
	Load5           float64 `json:"load5"`
	Load15          float64 `json:"load15"`
	MemTotal        uint64  `json:"mem_total"`
	MemUsed         uint64  `json:"mem_used"`
	MemUsedPercent  float64 `json:"mem_used_percent"`
	SwapTotal       uint64  `json:"swap_total"`
	SwapUsed        uint64  `json:"swap_used"`
	SwapUsedPercent float64 `json:"swap_used_percent"`
	DiskTotal       uint64  `json:"disk_total"`
	DiskUsed        uint64  `json:"disk_used"`
	DiskUsedPercent float64 `json:"disk_used_percent"`
	NetRxBps        uint64  `json:"net_rx_bps"`
	NetTxBps        uint64  `json:"net_tx_bps"`
	NetRxBytesTotal uint64  `json:"net_rx_bytes_total"`
	NetTxBytesTotal uint64  `json:"net_tx_bytes_total"`
	UptimeSeconds   uint64  `json:"uptime_seconds"`
}

type IPAddresses struct {
	IPv4 []string `json:"ipv4,omitempty"`
	IPv6 []string `json:"ipv6,omitempty"`
}

type PublicIPObservation struct {
	IP        string `json:"ip"`
	Source    string `json:"source,omitempty"`
	FirstSeen int64  `json:"first_seen,omitempty"`
	LastSeen  int64  `json:"last_seen,omitempty"`
}

type PublicIPs struct {
	IPv4 []PublicIPObservation `json:"ipv4,omitempty"`
	IPv6 []PublicIPObservation `json:"ipv6,omitempty"`
}

type RegisterPayload struct {
	AgentVersion string      `json:"agent_version"`
	Hostname     string      `json:"hostname"`
	PublicIP     string      `json:"public_ip"`
	IPAddresses  IPAddresses `json:"ip_addresses,omitempty"`
	PublicIPs    PublicIPs   `json:"public_ips,omitempty"`
	Nonce        []byte      `json:"nonce,omitempty"`
	Auth         string      `json:"auth,omitempty"`
}

type HeartbeatPayload struct {
	Status    string    `json:"status"`
	PublicIPs PublicIPs `json:"public_ips,omitempty"`
}

type RegisterAckPayload struct {
	Nonce []byte `json:"nonce"`
}

type AgentRuntimeConfig struct {
	NodeID                   string            `json:"node_id"`
	HeartbeatIntervalSeconds uint32            `json:"heartbeat_interval_seconds"`
	MetricsIntervalSeconds   uint32            `json:"metrics_interval_seconds"`
	ProbeTasks               []ProbeTaskConfig `json:"probe_tasks,omitempty"`
	Snapshot                 SnapshotConfig    `json:"snapshot"`
}

type SnapshotConfig struct {
	Enabled            bool   `json:"enabled"`
	CollectProcesses   bool   `json:"collect_processes"`
	CollectConnections bool   `json:"collect_connections"`
	MaskSensitive      bool   `json:"mask_sensitive"`
	IntervalSeconds    uint32 `json:"interval_seconds"`
	ProcessLimit       uint32 `json:"process_limit"`
	ConnectionLimit    uint32 `json:"connection_limit"`
}

type AgentLogPayload struct {
	Level   string `json:"level"`
	Message string `json:"message"`
}

type ProbeTaskConfig struct {
	ID              uint64 `json:"id"`
	Name            string `json:"name"`
	Type            string `json:"type"`
	IPVersion       string `json:"ip_version,omitempty"`
	Target          string `json:"target"`
	IntervalSeconds uint32 `json:"interval_seconds"`
	TimeoutMS       uint32 `json:"timeout_ms"`
	Enabled         bool   `json:"enabled"`
}

type ProbeResultsPayload struct {
	Results []ProbeResultItem `json:"results"`
}

type ProbeResultItem struct {
	TaskID       uint64   `json:"task_id"`
	Type         string   `json:"type"`
	IPVersion    string   `json:"ip_version,omitempty"`
	Target       string   `json:"target"`
	Status       string   `json:"status"`
	LatencyMS    *float64 `json:"latency_ms,omitempty"`
	PacketLoss   *float64 `json:"packet_loss,omitempty"`
	ErrorMessage string   `json:"error_message,omitempty"`
	CreatedAt    uint64   `json:"created_at"`
}

type SnapshotPayload struct {
	Config          SnapshotConfig       `json:"config"`
	ProcessCount    uint32               `json:"process_count"`
	ThreadCount     uint32               `json:"thread_count"`
	ConnectionCount uint32               `json:"connection_count"`
	ListenCount     uint32               `json:"listen_count"`
	TCPStateCounts  map[string]uint32    `json:"tcp_state_counts"`
	TopProcesses    []SnapshotProcess    `json:"top_processes,omitempty"`
	Connections     []SnapshotConnection `json:"connections,omitempty"`
	CreatedAt       uint64               `json:"created_at"`
}

type SnapshotProcess struct {
	PID         int     `json:"pid"`
	Name        string  `json:"name"`
	User        string  `json:"user"`
	State       string  `json:"state"`
	CPUPercent  float64 `json:"cpu_percent"`
	MemoryBytes uint64  `json:"memory_bytes"`
	ThreadCount uint32  `json:"thread_count"`
	Command     string  `json:"command"`
}

type SnapshotConnection struct {
	Protocol    string `json:"protocol"`
	LocalAddr   string `json:"local_addr"`
	LocalPort   uint16 `json:"local_port"`
	RemoteAddr  string `json:"remote_addr"`
	RemotePort  uint16 `json:"remote_port"`
	State       string `json:"state"`
	PID         int    `json:"pid,omitempty"`
	ProcessName string `json:"process_name,omitempty"`
}

func EncodeMessage(message Message) ([]byte, error) {
	return json.Marshal(message)
}

func DecodeMessage(raw []byte) (Message, error) {
	var message Message
	err := json.Unmarshal(raw, &message)
	return message, err
}

func PayloadFrom(value any) (map[string]any, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}

	return payload, nil
}

func PayloadTo[T any](payload map[string]any) (T, error) {
	var value T

	raw, err := json.Marshal(payload)
	if err != nil {
		return value, err
	}

	err = json.Unmarshal(raw, &value)
	return value, err
}

func ReadMessage(r io.Reader) (Message, error) {
	raw, err := ReadFrame(r)
	if err != nil {
		return Message{}, err
	}
	return DecodeMessage(raw)
}

func WriteMessage(w io.Writer, message Message) error {
	raw, err := EncodeMessage(message)
	if err != nil {
		return err
	}
	return WriteFrame(w, raw)
}

func RegisterAuth(secretKey string, payload RegisterPayload, timestamp int64) string {
	if secretKey == "" {
		return ""
	}
	mac := hmac.New(sha256.New, []byte(secretKey))
	_, _ = mac.Write([]byte(registerAuthBase(payload, timestamp)))
	return hex.EncodeToString(mac.Sum(nil))
}

func VerifyRegisterAuth(secretKey string, payload RegisterPayload, timestamp int64) bool {
	if secretKey == "" {
		return true
	}
	want := RegisterAuth(secretKey, RegisterPayload{
		AgentVersion: payload.AgentVersion,
		Hostname:     payload.Hostname,
		PublicIP:     payload.PublicIP,
		IPAddresses:  payload.IPAddresses,
		PublicIPs:    payload.PublicIPs,
		Nonce:        payload.Nonce,
	}, timestamp)
	got, err := hex.DecodeString(payload.Auth)
	if err != nil {
		return false
	}
	wantRaw, err := hex.DecodeString(want)
	if err != nil {
		return false
	}
	if hmac.Equal(got, wantRaw) {
		return true
	}
	if len(payload.PublicIPs.IPv4) > 0 || len(payload.PublicIPs.IPv6) > 0 {
		return false
	}
	legacyWithAddresses := registerAuth(secretKey, registerAuthBaseWithoutPublicIPs(RegisterPayload{
		AgentVersion: payload.AgentVersion,
		Hostname:     payload.Hostname,
		PublicIP:     payload.PublicIP,
		IPAddresses:  payload.IPAddresses,
		Nonce:        payload.Nonce,
	}, timestamp))
	legacyWithAddressesRaw, err := hex.DecodeString(legacyWithAddresses)
	if err != nil {
		return false
	}
	if hmac.Equal(got, legacyWithAddressesRaw) {
		return true
	}
	if len(payload.IPAddresses.IPv4) > 0 || len(payload.IPAddresses.IPv6) > 0 {
		return false
	}

	// Keep accepting agents built before ip_addresses was added to the signed payload.
	legacyWant := registerAuth(secretKey, registerAuthBaseLegacy(RegisterPayload{
		AgentVersion: payload.AgentVersion,
		Hostname:     payload.Hostname,
		PublicIP:     payload.PublicIP,
		Nonce:        payload.Nonce,
	}, timestamp))
	legacyWantRaw, err := hex.DecodeString(legacyWant)
	if err != nil {
		return false
	}
	return hmac.Equal(got, legacyWantRaw)
}

func registerAuthBase(payload RegisterPayload, timestamp int64) string {
	nonce := base64.StdEncoding.EncodeToString(payload.Nonce)
	return fmt.Sprintf("%d|%s|%s|%s|%s|%s|%s", timestamp, payload.AgentVersion, payload.Hostname, payload.PublicIP, ipAddressesAuthPart(payload.IPAddresses), publicIPsAuthPart(payload.PublicIPs), nonce)
}

func registerAuthBaseWithoutPublicIPs(payload RegisterPayload, timestamp int64) string {
	nonce := base64.StdEncoding.EncodeToString(payload.Nonce)
	return fmt.Sprintf("%d|%s|%s|%s|%s|%s", timestamp, payload.AgentVersion, payload.Hostname, payload.PublicIP, ipAddressesAuthPart(payload.IPAddresses), nonce)
}

func registerAuthBaseLegacy(payload RegisterPayload, timestamp int64) string {
	nonce := base64.StdEncoding.EncodeToString(payload.Nonce)
	return fmt.Sprintf("%d|%s|%s|%s|%s", timestamp, payload.AgentVersion, payload.Hostname, payload.PublicIP, nonce)
}

func registerAuth(secretKey string, base string) string {
	mac := hmac.New(sha256.New, []byte(secretKey))
	_, _ = mac.Write([]byte(base))
	return hex.EncodeToString(mac.Sum(nil))
}

func ipAddressesAuthPart(addresses IPAddresses) string {
	return strings.Join(addresses.IPv4, ",") + ";" + strings.Join(addresses.IPv6, ",")
}

func publicIPsAuthPart(addresses PublicIPs) string {
	return publicIPObservationsAuthPart(addresses.IPv4) + ";" + publicIPObservationsAuthPart(addresses.IPv6)
}

func publicIPObservationsAuthPart(values []PublicIPObservation) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, strings.Join([]string{
			value.IP,
			value.Source,
			fmt.Sprint(value.FirstSeen),
			fmt.Sprint(value.LastSeen),
		}, ","))
	}
	return strings.Join(parts, "|")
}
