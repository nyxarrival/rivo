package model

import "time"

type Node struct {
	ID                         uint64    `gorm:"primaryKey" json:"id"`
	NodeID                     string    `gorm:"size:64;uniqueIndex;not null" json:"node_id"`
	Name                       string    `gorm:"size:128;not null" json:"name"`
	Region                     string    `gorm:"size:64" json:"region"`
	Provider                   string    `gorm:"size:64" json:"provider"`
	NetworkLine                string    `gorm:"size:128;not null;default:''" json:"network_line"`
	Tag                        string    `gorm:"size:128;not null;default:''" json:"tag"`
	PublicIP                   string    `gorm:"size:64" json:"public_ip"`
	PublicIPv6                 string    `gorm:"size:128" json:"public_ipv6"`
	PublicIPsJSON              string    `gorm:"column:public_ips_json;type:json" json:"public_ips_json"`
	IPAddressesJSON            string    `gorm:"column:ip_addresses_json;type:json" json:"ip_addresses_json"`
	Status                     string    `gorm:"size:32;not null;default:offline;index" json:"status"`
	AgentVersion               string    `gorm:"size:64" json:"agent_version"`
	SecretKeyHash              string    `gorm:"size:255;not null" json:"-"`
	SecretKeyEncrypted         string    `gorm:"type:text" json:"-"`
	LastSeenAt                 *uint64   `gorm:"type:bigint unsigned;index" json:"last_seen_at"`
	HeartbeatInterval          uint32    `gorm:"not null;default:15" json:"heartbeat_interval"`
	MetricsInterval            uint32    `gorm:"not null;default:15" json:"metrics_interval"`
	SnapshotOverride           bool      `gorm:"not null;default:false" json:"snapshot_override"`
	SnapshotEnabled            bool      `gorm:"not null;default:false" json:"snapshot_enabled"`
	SnapshotCollectProcesses   bool      `gorm:"not null;default:true" json:"snapshot_collect_processes"`
	SnapshotCollectConnections bool      `gorm:"not null;default:true" json:"snapshot_collect_connections"`
	SnapshotMaskSensitive      bool      `gorm:"not null;default:true" json:"snapshot_mask_sensitive"`
	SnapshotInterval           uint32    `gorm:"not null;default:60" json:"snapshot_interval"`
	SnapshotProcessLimit       uint32    `gorm:"not null;default:20" json:"snapshot_process_limit"`
	SnapshotConnectionLimit    uint32    `gorm:"not null;default:200" json:"snapshot_connection_limit"`
	BillingCycle               string    `gorm:"size:32;not null;default:monthly" json:"billing_cycle"`
	PriceAmount                float64   `gorm:"not null;default:0" json:"price_amount"`
	Currency                   string    `gorm:"size:16;not null;default:CNY" json:"currency"`
	ServiceStartedAt           *uint64   `gorm:"type:bigint unsigned" json:"service_started_at"`
	ServiceExpiresAt           *uint64   `gorm:"type:bigint unsigned" json:"service_expires_at"`
	TrafficLimitBytes          uint64    `gorm:"not null;default:0" json:"traffic_limit_bytes"`
	TrafficCalibrationBytes    uint64    `gorm:"not null;default:0" json:"traffic_calibration_bytes"`
	TrafficCalibrationAt       *uint64   `gorm:"type:bigint unsigned" json:"traffic_calibration_at"`
	TrafficBillingDirection    string    `gorm:"size:32;not null;default:bidirectional" json:"traffic_billing_direction"`
	TrafficResetCycle          string    `gorm:"size:32;not null;default:monthly" json:"traffic_reset_cycle"`
	CreatedAt                  time.Time `gorm:"type:timestamp;not null" json:"created_at"`
	UpdatedAt                  time.Time `gorm:"type:timestamp;not null" json:"updated_at"`
}

type NodeMetric struct {
	ID              uint64    `gorm:"primaryKey" json:"id"`
	NodeID          string    `gorm:"size:64;index:idx_node_metric_node_ts,priority:1;not null" json:"node_id"`
	Timestamp       uint64    `gorm:"column:ts;type:bigint unsigned;index:idx_node_metric_node_ts,priority:2;index;not null" json:"ts"`
	CPUUsage        float64   `json:"cpu_usage"`
	CPUCores        uint32    `gorm:"not null;default:0" json:"cpu_cores"`
	Arch            string    `gorm:"size:32;not null;default:''" json:"arch"`
	Virtualization  string    `gorm:"size:64;not null;default:''" json:"virtualization"`
	GPU             string    `gorm:"size:255;not null;default:''" json:"gpu"`
	OSName          string    `gorm:"size:128;not null;default:''" json:"os_name"`
	Load1           float64   `json:"load1"`
	Load5           float64   `json:"load5"`
	Load15          float64   `json:"load15"`
	MemTotal        uint64    `json:"mem_total"`
	MemUsed         uint64    `json:"mem_used"`
	MemUsedPercent  float64   `json:"mem_used_percent"`
	SwapTotal       uint64    `gorm:"not null;default:0" json:"swap_total"`
	SwapUsed        uint64    `gorm:"not null;default:0" json:"swap_used"`
	SwapUsedPercent float64   `gorm:"not null;default:0" json:"swap_used_percent"`
	DiskTotal       uint64    `json:"disk_total"`
	DiskUsed        uint64    `json:"disk_used"`
	DiskUsedPercent float64   `json:"disk_used_percent"`
	NetRxBps        uint64    `json:"net_rx_bps"`
	NetTxBps        uint64    `json:"net_tx_bps"`
	NetRxBytesTotal uint64    `json:"net_rx_bytes_total"`
	NetTxBytesTotal uint64    `json:"net_tx_bytes_total"`
	UptimeSeconds   uint64    `json:"uptime_seconds"`
	CreatedAt       time.Time `gorm:"type:timestamp;not null" json:"created_at"`
}

type NodeSnapshot struct {
	ID              uint64    `gorm:"primaryKey" json:"id"`
	NodeID          string    `gorm:"size:64;index:idx_node_snapshot_node_ts,priority:1;not null" json:"node_id"`
	Timestamp       uint64    `gorm:"column:ts;type:bigint unsigned;index:idx_node_snapshot_node_ts,priority:2;index;not null" json:"ts"`
	ProcessCount    uint32    `gorm:"not null;default:0" json:"process_count"`
	ThreadCount     uint32    `gorm:"not null;default:0" json:"thread_count"`
	ConnectionCount uint32    `gorm:"not null;default:0" json:"connection_count"`
	ListenCount     uint32    `gorm:"not null;default:0" json:"listen_count"`
	TCPStateJSON    string    `gorm:"type:json" json:"tcp_state_json"`
	TopProcessJSON  string    `gorm:"type:json" json:"top_process_json"`
	ConnectionsJSON string    `gorm:"type:json" json:"connections_json"`
	CreatedAt       time.Time `gorm:"type:timestamp;not null" json:"created_at"`
}

type ProbeTask struct {
	ID              uint64    `gorm:"primaryKey" json:"id"`
	Name            string    `gorm:"size:128;not null" json:"name"`
	Type            string    `gorm:"size:32;index;not null" json:"type"`
	IPVersion       string    `gorm:"size:16;not null;default:auto" json:"ip_version"`
	Target          string    `gorm:"size:255;not null" json:"target"`
	IntervalSeconds uint32    `gorm:"not null;default:60" json:"interval_seconds"`
	TimeoutMS       uint32    `gorm:"not null;default:3000" json:"timeout_ms"`
	Enabled         bool      `gorm:"not null;default:true;index" json:"enabled"`
	CreatedAt       time.Time `gorm:"type:timestamp;not null" json:"created_at"`
	UpdatedAt       time.Time `gorm:"type:timestamp;not null" json:"updated_at"`
}

type ProbeTaskAssignment struct {
	ID        uint64    `gorm:"primaryKey" json:"id"`
	NodeID    string    `gorm:"size:64;uniqueIndex:uk_probe_task_assignment_node_task,priority:1;index;not null" json:"node_id"`
	TaskID    uint64    `gorm:"uniqueIndex:uk_probe_task_assignment_node_task,priority:2;index;not null" json:"task_id"`
	CreatedAt time.Time `gorm:"type:timestamp;not null" json:"created_at"`
	UpdatedAt time.Time `gorm:"type:timestamp;not null" json:"updated_at"`
}

type ProbeResult struct {
	ID           uint64    `gorm:"primaryKey" json:"id"`
	TaskID       uint64    `gorm:"index:idx_probe_result_task_created,priority:1;not null" json:"task_id"`
	NodeID       string    `gorm:"size:64;index:idx_probe_result_node_created,priority:1;not null" json:"node_id"`
	Type         string    `gorm:"size:32;not null" json:"type"`
	IPVersion    string    `gorm:"size:16;not null;default:auto" json:"ip_version"`
	Target       string    `gorm:"size:255;not null" json:"target"`
	Status       string    `gorm:"size:32;index;not null" json:"status"`
	LatencyMS    *float64  `json:"latency_ms"`
	PacketLoss   *float64  `json:"packet_loss"`
	HTTPStatus   *int      `json:"http_status"`
	ErrorMessage string    `gorm:"type:text" json:"error_message"`
	CreatedAt    time.Time `gorm:"type:timestamp;not null;index:idx_probe_result_task_created,priority:2;index:idx_probe_result_node_created,priority:2" json:"created_at"`
}

type Alert struct {
	ID               uint64    `gorm:"primaryKey" json:"id"`
	NodeID           string    `gorm:"size:64;index:idx_alert_node_status,priority:1;not null" json:"node_id"`
	RuleType         string    `gorm:"size:64;not null" json:"rule_type"`
	Level            string    `gorm:"size:32;index:idx_alert_level_status,priority:1;not null" json:"level"`
	Status           string    `gorm:"size:32;index:idx_alert_node_status,priority:2;index:idx_alert_level_status,priority:2;not null" json:"status"`
	Message          string    `gorm:"type:text" json:"message"`
	FirstTriggeredAt *uint64   `gorm:"type:bigint unsigned" json:"first_triggered_at"`
	LastTriggeredAt  *uint64   `gorm:"type:bigint unsigned" json:"last_triggered_at"`
	ResolvedAt       *uint64   `gorm:"type:bigint unsigned" json:"resolved_at"`
	CreatedAt        time.Time `gorm:"type:timestamp;not null" json:"created_at"`
	UpdatedAt        time.Time `gorm:"type:timestamp;not null" json:"updated_at"`
}

type NodeEvent struct {
	ID        uint64    `gorm:"primaryKey" json:"id"`
	NodeID    string    `gorm:"size:64;index:idx_node_event_node_created,priority:1;not null" json:"node_id"`
	EventType string    `gorm:"size:64;index:idx_node_event_type_created,priority:1;not null" json:"event_type"`
	Message   string    `gorm:"type:text" json:"message"`
	CreatedAt time.Time `gorm:"type:timestamp;not null;index:idx_node_event_node_created,priority:2;index:idx_node_event_type_created,priority:2" json:"created_at"`
}

type SystemLog struct {
	ID        uint64    `gorm:"primaryKey" json:"id"`
	Service   string    `gorm:"size:32;index:idx_system_log_service_created,priority:1;not null" json:"service"`
	NodeID    string    `gorm:"size:64;index:idx_system_log_node_created,priority:1" json:"node_id"`
	Level     string    `gorm:"size:16;index:idx_system_log_level_created,priority:1;not null" json:"level"`
	EventType string    `gorm:"size:64;index:idx_system_log_event_created,priority:1;not null" json:"event_type"`
	Message   string    `gorm:"type:text" json:"message"`
	MetaJSON  string    `gorm:"type:json" json:"meta_json"`
	CreatedAt time.Time `gorm:"type:timestamp;not null;index:idx_system_log_service_created,priority:2;index:idx_system_log_node_created,priority:2;index:idx_system_log_level_created,priority:2;index:idx_system_log_event_created,priority:2" json:"created_at"`
}

type AppSetting struct {
	ID        uint64    `gorm:"primaryKey" json:"id"`
	Key       string    `gorm:"size:96;uniqueIndex;not null" json:"key"`
	Value     string    `gorm:"type:mediumtext;not null" json:"value"`
	CreatedAt time.Time `gorm:"type:timestamp;not null" json:"created_at"`
	UpdatedAt time.Time `gorm:"type:timestamp;not null" json:"updated_at"`
}

type RegionOption struct {
	ID        uint64    `gorm:"primaryKey" json:"id"`
	Code      string    `gorm:"size:16;uniqueIndex;not null" json:"code"`
	Name      string    `gorm:"size:64;not null" json:"name"`
	SortOrder int       `gorm:"not null;default:0;index" json:"sort_order"`
	Enabled   bool      `gorm:"not null;default:true;index" json:"enabled"`
	CreatedAt time.Time `gorm:"type:timestamp;not null" json:"created_at"`
	UpdatedAt time.Time `gorm:"type:timestamp;not null" json:"updated_at"`
}
