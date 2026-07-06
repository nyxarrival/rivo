package tcp

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"rivo/internal/master/model"
	"rivo/internal/master/notify"
	"rivo/internal/protocol"
)

type Server struct {
	addr                string
	secretKey           string
	logger              *slog.Logger
	db                  *gorm.DB
	mu                  sync.RWMutex
	conns               map[string]*agentSession
	offlineNotifyTimers map[string]*time.Timer
}

type agentSession struct {
	conn   net.Conn
	secure *protocol.SecureConn
	mu     sync.Mutex
}

type weComSettings struct {
	Enabled                  bool
	WebhookURL               string
	TelegramEnabled          bool
	TelegramBotToken         string
	TelegramChatID           string
	EmailEnabled             bool
	EmailSMTPHost            string
	EmailSMTPPort            int
	EmailSMTPSecurity        string
	EmailSMTPUsername        string
	EmailSMTPPassword        string
	EmailFrom                string
	EmailTo                  string
	TrafficAlertEnabled      bool
	TrafficAlertPercent      float64
	CPUAlertEnabled          bool
	CPUAlertPercent          float64
	MemoryAlertEnabled       bool
	MemoryAlertPercent       float64
	DiskLoadAlertEnabled     bool
	DiskLoadAlertPercent     float64
	LoadAlertEnabled         bool
	LoadAlertThreshold       float64
	AlertIntervalMinutes     int
	OfflineAlertDelayMinutes int
	ExpiryAlertEnabled       bool
	ExpiryAlertDays          int
}

func NewServer(addr string, secretKey string, logger *slog.Logger, db *gorm.DB) *Server {
	return &Server{
		addr:                addr,
		secretKey:           secretKey,
		logger:              logger,
		db:                  db,
		conns:               make(map[string]*agentSession),
		offlineNotifyTimers: make(map[string]*time.Timer),
	}
}

func (s *Server) Run(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	defer ln.Close()

	s.logger.Info("master tcp server listening", slog.String("addr", s.addr))
	s.markOnlineNodesOfflineOnStart()

	for {
		conn, err := ln.Accept()
		if err != nil {
			if errors.Is(ctx.Err(), context.Canceled) {
				return ctx.Err()
			}
			s.logger.Warn("accept tcp connection failed", slog.String("error", err.Error()))
			continue
		}

		go s.handleConn(conn)
	}
}

func (s *Server) StartOfflineMonitor(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		s.markStaleNodesOffline()
		s.checkExpiryAlerts()

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer func() {
		s.unregisterConn(conn)
		conn.Close()
	}()
	s.logger.Info("agent connected", slog.String("remote_addr", conn.RemoteAddr().String()))

	msg, err := protocol.ReadMessage(conn)
	if err != nil {
		s.logger.Info("agent disconnected before register", slog.String("remote_addr", conn.RemoteAddr().String()), slog.String("error", err.Error()))
		return
	}
	if msg.Type != protocol.MessageTypeRegister {
		s.logger.Warn("first agent message is not register", slog.String("type", msg.Type), slog.String("remote_addr", conn.RemoteAddr().String()))
		return
	}
	nodeID, secure, err := s.handleRegister(conn, msg)
	if err != nil {
		s.logger.Warn("handle agent register failed", slog.String("node_id", msg.NodeID), slog.String("remote_addr", conn.RemoteAddr().String()), slog.String("error", err.Error()))
		return
	}

	for {
		msg, err := secure.ReadMessage()
		if err != nil {
			s.logger.Info("agent disconnected", slog.String("remote_addr", conn.RemoteAddr().String()), slog.String("error", err.Error()))
			return
		}
		if msg.NodeID == "" {
			msg.NodeID = nodeID
		}
		if msg.NodeID != nodeID {
			s.logger.Warn("agent sent message for different node", slog.String("session_node_id", nodeID), slog.String("message_node_id", msg.NodeID), slog.String("type", msg.Type))
			return
		}
		if err := s.handleMessage(msg); err != nil {
			s.logger.Warn("handle agent message failed", slog.String("type", msg.Type), slog.String("node_id", msg.NodeID), slog.String("error", err.Error()))
			continue
		}
	}
}

func (s *Server) handleMessage(msg protocol.Message) error {
	if msg.NodeID == "" {
		return errors.New("missing node_id")
	}

	switch msg.Type {
	case protocol.MessageTypeHeartbeat:
		return s.handleHeartbeat(msg)
	case protocol.MessageTypeMetrics:
		return s.handleMetrics(msg)
	case protocol.MessageTypeLog:
		return s.handleAgentLog(msg)
	case protocol.MessageTypeProbeResults:
		return s.handleProbeResults(msg)
	case protocol.MessageTypeSnapshotResults:
		return s.handleSnapshotResults(msg)
	default:
		s.logger.Info("agent message received", slog.String("type", msg.Type), slog.String("node_id", msg.NodeID))
		return nil
	}
}

func (s *Server) handleRegister(conn net.Conn, msg protocol.Message) (string, *protocol.SecureConn, error) {
	payload, err := protocol.PayloadTo[protocol.RegisterPayload](msg.Payload)
	if err != nil {
		return "", nil, err
	}
	if !protocol.VerifyRegisterAuth(s.secretKey, payload, msg.Timestamp) {
		return "", nil, errors.New("invalid register auth")
	}
	if len(payload.Nonce) != protocol.HandshakeSize {
		return "", nil, errors.New("invalid agent nonce")
	}
	secretKey, err := protocol.DecodeSecretKey(s.secretKey)
	if err != nil {
		return "", nil, err
	}

	reportedIPs := normalizeIPAddresses(payload.IPAddresses)
	connectionIP := normalizeUsableNodeIP(remoteIP(conn.RemoteAddr()))
	reportedPublicIPs := normalizePublicIPs(payload.PublicIPs)

	nodeID, publicIP, publicIPv6, mergedPublicIPs, err := s.registerNode(msg.NodeID, payload.PublicIP, reportedIPs, reportedPublicIPs, connectionIP, payload.AgentVersion)
	if err != nil {
		return "", nil, err
	}

	cfg, err := s.runtimeConfig(nodeID)
	if err != nil {
		return "", nil, err
	}
	masterNonce, err := protocol.RandomBytes(protocol.HandshakeSize)
	if err != nil {
		return "", nil, err
	}
	keys, err := protocol.DeriveSessionKeys(secretKey, payload.Nonce, masterNonce)
	if err != nil {
		return "", nil, err
	}
	ackPayload, err := protocol.PayloadFrom(protocol.RegisterAckPayload{
		Nonce: masterNonce,
	})
	if err != nil {
		return "", nil, err
	}

	if err := protocol.WriteMessage(conn, protocol.Message{
		Type:      protocol.MessageTypeRegisterAck,
		NodeID:    nodeID,
		Timestamp: time.Now().UnixMilli(),
		Payload:   ackPayload,
	}); err != nil {
		return "", nil, err
	}

	secure := protocol.NewSecureConn(conn, keys.MasterToAgent, keys.AgentToMaster)
	configPayload, err := protocol.PayloadFrom(cfg)
	if err != nil {
		return "", nil, err
	}
	if err := secure.WriteMessage(protocol.Message{
		Type:      protocol.MessageTypeConfigUpdate,
		NodeID:    nodeID,
		Timestamp: time.Now().UnixMilli(),
		Payload:   configPayload,
	}); err != nil {
		return "", nil, err
	}

	s.registerConn(nodeID, conn, secure)
	s.cancelOfflineWeCom(nodeID)
	displayIP := publicIP
	if displayIP == "" {
		displayIP = publicIPv6
	}
	if displayIP == "" {
		displayIP = "unknown"
	}
	s.storeSystemEventLog("agent", nodeID, "info", "agent.online", "agent registered from "+displayIP, map[string]any{
		"public_ip":       publicIP,
		"public_ipv6":     publicIPv6,
		"public_ips":      mergedPublicIPs,
		"ip_addresses":    reportedIPs,
		"agent_version":   payload.AgentVersion,
		"connection_ip":   connectionIP,
		"hostname":        payload.Hostname,
		"message_node_id": msg.NodeID,
	})
	s.notifyWeComAsync(nodeID, "online", "agent registered from "+displayIP)
	return nodeID, secure, nil
}

func (s *Server) PublishConfig(nodeID string) error {
	s.mu.RLock()
	session := s.conns[nodeID]
	s.mu.RUnlock()
	if session == nil {
		return nil
	}

	cfg, err := s.runtimeConfig(nodeID)
	if err != nil {
		return err
	}
	payload, err := protocol.PayloadFrom(cfg)
	if err != nil {
		return err
	}
	return session.write(protocol.Message{
		Type:      protocol.MessageTypeConfigUpdate,
		NodeID:    nodeID,
		Timestamp: time.Now().UnixMilli(),
		Payload:   payload,
	})
}

func (s *Server) RequestMetrics(nodeID string) error {
	s.mu.RLock()
	session := s.conns[nodeID]
	s.mu.RUnlock()
	if session == nil {
		return errors.New("agent is not connected")
	}

	return session.write(protocol.Message{
		Type:      protocol.MessageTypeRequestMetrics,
		NodeID:    nodeID,
		Timestamp: time.Now().UnixMilli(),
	})
}

func (s *Server) handleHeartbeat(msg protocol.Message) error {
	if err := s.touchNode(msg.NodeID, ""); err != nil {
		return err
	}
	if len(msg.Payload) == 0 {
		return nil
	}
	payload, err := protocol.PayloadTo[protocol.HeartbeatPayload](msg.Payload)
	if err != nil {
		return nil
	}
	if len(payload.PublicIPs.IPv4) == 0 && len(payload.PublicIPs.IPv6) == 0 {
		return nil
	}
	return s.updateNodePublicIPs(msg.NodeID, normalizePublicIPs(payload.PublicIPs), time.Now().UnixMilli())
}

func (s *Server) handleMetrics(msg protocol.Message) error {
	payload, err := protocol.PayloadTo[protocol.MetricsPayload](msg.Payload)
	if err != nil {
		return err
	}

	ts := uint64(time.Now().UnixMilli())
	if msg.Timestamp > 0 {
		ts = uint64(msg.Timestamp)
	}

	s.logger.Debug(
		"decrypted metrics received",
		slog.String("node_id", msg.NodeID),
		slog.Uint64("seq", msg.Seq),
		slog.Uint64("ts", ts),
		slog.Float64("cpu_usage", payload.CPUUsage),
		slog.Uint64("cpu_cores", uint64(payload.CPUCores)),
		slog.String("arch", payload.Arch),
		slog.String("virtualization", payload.Virtualization),
		slog.String("gpu", payload.GPU),
		slog.String("os_name", payload.OSName),
		slog.Float64("load1", payload.Load1),
		slog.Float64("load5", payload.Load5),
		slog.Float64("load15", payload.Load15),
		slog.Uint64("mem_total", payload.MemTotal),
		slog.Uint64("mem_used", payload.MemUsed),
		slog.Float64("mem_used_percent", payload.MemUsedPercent),
		slog.Uint64("swap_total", payload.SwapTotal),
		slog.Uint64("swap_used", payload.SwapUsed),
		slog.Float64("swap_used_percent", payload.SwapUsedPercent),
		slog.Uint64("disk_total", payload.DiskTotal),
		slog.Uint64("disk_used", payload.DiskUsed),
		slog.Float64("disk_used_percent", payload.DiskUsedPercent),
		slog.Uint64("net_rx_bps", payload.NetRxBps),
		slog.Uint64("net_tx_bps", payload.NetTxBps),
		slog.Uint64("net_rx_bytes_total", payload.NetRxBytesTotal),
		slog.Uint64("net_tx_bytes_total", payload.NetTxBytesTotal),
		slog.Uint64("uptime_seconds", payload.UptimeSeconds),
	)

	metric := model.NodeMetric{
		NodeID:          msg.NodeID,
		Timestamp:       ts,
		CPUUsage:        payload.CPUUsage,
		CPUCores:        payload.CPUCores,
		Arch:            payload.Arch,
		Virtualization:  payload.Virtualization,
		GPU:             payload.GPU,
		OSName:          payload.OSName,
		Load1:           payload.Load1,
		Load5:           payload.Load5,
		Load15:          payload.Load15,
		MemTotal:        payload.MemTotal,
		MemUsed:         payload.MemUsed,
		MemUsedPercent:  payload.MemUsedPercent,
		SwapTotal:       payload.SwapTotal,
		SwapUsed:        payload.SwapUsed,
		SwapUsedPercent: payload.SwapUsedPercent,
		DiskTotal:       payload.DiskTotal,
		DiskUsed:        payload.DiskUsed,
		DiskUsedPercent: payload.DiskUsedPercent,
		NetRxBps:        payload.NetRxBps,
		NetTxBps:        payload.NetTxBps,
		NetRxBytesTotal: payload.NetRxBytesTotal,
		NetTxBytesTotal: payload.NetTxBytesTotal,
		UptimeSeconds:   payload.UptimeSeconds,
	}

	if err := s.db.Create(&metric).Error; err != nil {
		return err
	}

	if err := s.touchNode(msg.NodeID, ""); err != nil {
		return err
	}

	s.logger.Debug(
		"metrics stored",
		slog.String("node_id", msg.NodeID),
		slog.Float64("cpu_usage", payload.CPUUsage),
		slog.Float64("mem_used_percent", payload.MemUsedPercent),
	)

	s.evaluateMetricAlerts(msg.NodeID, metric)

	return nil
}

func (s *Server) handleAgentLog(msg protocol.Message) error {
	payload, err := protocol.PayloadTo[protocol.AgentLogPayload](msg.Payload)
	if err != nil {
		return err
	}
	if payload.Level == "" {
		payload.Level = "info"
	}
	s.storeSystemEventLog("agent", msg.NodeID, payload.Level, "agent.log", payload.Message, nil)
	return nil
}

func (s *Server) handleProbeResults(msg protocol.Message) error {
	payload, err := protocol.PayloadTo[protocol.ProbeResultsPayload](msg.Payload)
	if err != nil {
		return err
	}
	if len(payload.Results) == 0 {
		return nil
	}

	results := make([]model.ProbeResult, 0, len(payload.Results))
	for _, item := range payload.Results {
		createdAt := time.Now()
		if item.CreatedAt > 0 {
			createdAt = time.UnixMilli(int64(item.CreatedAt))
		}
		status := strings.TrimSpace(item.Status)
		if status == "" {
			status = "unknown"
		}
		ipVersion := strings.ToLower(strings.TrimSpace(item.IPVersion))
		if ipVersion == "" {
			ipVersion = "auto"
		}
		results = append(results, model.ProbeResult{
			TaskID:       item.TaskID,
			NodeID:       msg.NodeID,
			Type:         strings.ToLower(strings.TrimSpace(item.Type)),
			IPVersion:    ipVersion,
			Target:       strings.TrimSpace(item.Target),
			Status:       status,
			LatencyMS:    item.LatencyMS,
			PacketLoss:   item.PacketLoss,
			ErrorMessage: item.ErrorMessage,
			CreatedAt:    createdAt,
		})
	}

	if err := s.db.Create(&results).Error; err != nil {
		return err
	}
	s.logger.Debug("probe results stored", slog.String("node_id", msg.NodeID), slog.Int("count", len(results)))
	return nil
}

func (s *Server) handleSnapshotResults(msg protocol.Message) error {
	payload, err := protocol.PayloadTo[protocol.SnapshotPayload](msg.Payload)
	if err != nil {
		return err
	}

	ts := uint64(time.Now().UnixMilli())
	if payload.CreatedAt > 0 {
		ts = payload.CreatedAt
	} else if msg.Timestamp > 0 {
		ts = uint64(msg.Timestamp)
	}

	stateJSON := marshalSnapshotJSON(payload.TCPStateCounts, "{}")
	processJSON := marshalSnapshotJSON(payload.TopProcesses, "[]")
	connectionJSON := marshalSnapshotJSON(payload.Connections, "[]")
	snapshot := model.NodeSnapshot{
		NodeID:          msg.NodeID,
		Timestamp:       ts,
		ProcessCount:    payload.ProcessCount,
		ThreadCount:     payload.ThreadCount,
		ConnectionCount: payload.ConnectionCount,
		ListenCount:     payload.ListenCount,
		TCPStateJSON:    stateJSON,
		TopProcessJSON:  processJSON,
		ConnectionsJSON: connectionJSON,
	}
	if err := s.db.Create(&snapshot).Error; err != nil {
		return err
	}
	if err := s.touchNode(msg.NodeID, ""); err != nil {
		return err
	}

	s.logger.Debug(
		"snapshot stored",
		slog.String("node_id", msg.NodeID),
		slog.Uint64("ts", ts),
		slog.Uint64("process_count", uint64(payload.ProcessCount)),
		slog.Uint64("connection_count", uint64(payload.ConnectionCount)),
	)
	return nil
}

func (s *Server) registerNode(nodeID string, reportedPrimary string, reportedIPs protocol.IPAddresses, reportedPublicIPs protocol.PublicIPs, connectionIP string, agentVersion string) (string, string, string, protocol.PublicIPs, error) {
	if nodeID == "" {
		nextNodeID, err := s.randomNodeID()
		if err != nil {
			return "", "", "", protocol.PublicIPs{}, err
		}
		nodeID = nextNodeID
	}

	nowTime := time.Now()
	now := uint64(nowTime.UnixMilli())
	existing := s.findNodeForIPMerge(nodeID)
	mergedPublicIPs := mergeNodePublicIPs(existing, reportedPrimary, reportedIPs, reportedPublicIPs, connectionIP, nowTime.UnixMilli())
	publicIP := primaryPublicIPv4(existing, mergedPublicIPs)
	publicIPv6 := primaryPublicIPv6(existing, mergedPublicIPs)
	ipAddressesJSON := marshalIPAddresses(reportedIPs)
	publicIPsJSON := marshalPublicIPs(mergedPublicIPs)
	node := model.Node{
		NodeID:                     nodeID,
		Name:                       nodeID,
		Region:                     "default",
		PublicIP:                   publicIP,
		PublicIPv6:                 publicIPv6,
		PublicIPsJSON:              publicIPsJSON,
		IPAddressesJSON:            ipAddressesJSON,
		Status:                     "online",
		AgentVersion:               agentVersion,
		SecretKeyHash:              "pending",
		LastSeenAt:                 &now,
		HeartbeatInterval:          15,
		MetricsInterval:            15,
		SnapshotCollectProcesses:   true,
		SnapshotCollectConnections: true,
		SnapshotMaskSensitive:      true,
		SnapshotInterval:           60,
		SnapshotProcessLimit:       20,
		SnapshotConnectionLimit:    200,
		BillingCycle:               "monthly",
		Currency:                   "CNY",
		TrafficBillingDirection:    "bidirectional",
		TrafficResetCycle:          "monthly",
	}

	assignments := map[string]any{
		"status":       "online",
		"last_seen_at": now,
		"updated_at":   nowTime,
		"public_ip":    publicIP,
		"public_ipv6":  publicIPv6,
	}
	if publicIPsJSON != "" {
		assignments["public_ips_json"] = publicIPsJSON
	}
	if ipAddressesJSON != "" {
		assignments["ip_addresses_json"] = ipAddressesJSON
	}
	if agentVersion != "" {
		assignments["agent_version"] = agentVersion
	}

	if err := s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "node_id"}},
		DoUpdates: clause.Assignments(assignments),
	}).Create(&node).Error; err != nil {
		return "", "", "", protocol.PublicIPs{}, err
	}
	return nodeID, publicIP, publicIPv6, mergedPublicIPs, nil
}

func (s *Server) updateNodePublicIPs(nodeID string, reportedPublicIPs protocol.PublicIPs, seenAt int64) error {
	if strings.TrimSpace(nodeID) == "" {
		return nil
	}
	existing := s.findNodeForIPMerge(nodeID)
	merged := mergeNodePublicIPs(existing, "", protocol.IPAddresses{}, reportedPublicIPs, "", seenAt)
	publicIP := primaryPublicIPv4(existing, merged)
	publicIPv6 := primaryPublicIPv6(existing, merged)
	publicIPsJSON := marshalPublicIPs(merged)
	if publicIPsJSON == "" && publicIP == "" && publicIPv6 == "" {
		return nil
	}
	updates := map[string]any{
		"public_ip":   publicIP,
		"public_ipv6": publicIPv6,
		"updated_at":  time.Now(),
	}
	if publicIPsJSON != "" {
		updates["public_ips_json"] = publicIPsJSON
	}
	return s.db.Model(&model.Node{}).Where("node_id = ?", nodeID).Updates(updates).Error
}

func (s *Server) runtimeConfig(nodeID string) (protocol.AgentRuntimeConfig, error) {
	var node model.Node
	if err := s.db.Where("node_id = ?", nodeID).First(&node).Error; err != nil {
		return protocol.AgentRuntimeConfig{}, err
	}
	var tasks []model.ProbeTask
	if err := s.db.Table("probe_tasks").
		Select("probe_tasks.*").
		Joins("JOIN probe_task_assignments ON probe_task_assignments.task_id = probe_tasks.id").
		Where("probe_task_assignments.node_id = ? AND probe_tasks.enabled = ?", nodeID, true).
		Order("probe_task_assignments.id asc, probe_tasks.id asc").
		Scan(&tasks).Error; err != nil {
		return protocol.AgentRuntimeConfig{}, err
	}
	probeTasks := make([]protocol.ProbeTaskConfig, 0, len(tasks))
	for _, task := range tasks {
		ipVersion := strings.ToLower(strings.TrimSpace(task.IPVersion))
		if ipVersion == "" {
			ipVersion = "auto"
		}
		probeTasks = append(probeTasks, protocol.ProbeTaskConfig{
			ID:              task.ID,
			Name:            task.Name,
			Type:            task.Type,
			IPVersion:       ipVersion,
			Target:          task.Target,
			IntervalSeconds: task.IntervalSeconds,
			TimeoutMS:       task.TimeoutMS,
			Enabled:         task.Enabled,
		})
	}
	return protocol.AgentRuntimeConfig{
		NodeID:                   node.NodeID,
		HeartbeatIntervalSeconds: node.HeartbeatInterval,
		MetricsIntervalSeconds:   node.MetricsInterval,
		ProbeTasks:               probeTasks,
		Snapshot:                 s.snapshotConfig(node),
	}, nil
}

func (s *Server) snapshotConfig(node model.Node) protocol.SnapshotConfig {
	cfg := s.loadGlobalSnapshotConfig()
	if node.SnapshotOverride {
		cfg.Enabled = node.SnapshotEnabled
		cfg.CollectProcesses = node.SnapshotCollectProcesses
		cfg.CollectConnections = node.SnapshotCollectConnections
		cfg.MaskSensitive = node.SnapshotMaskSensitive
		cfg.IntervalSeconds = node.SnapshotInterval
		cfg.ProcessLimit = node.SnapshotProcessLimit
		cfg.ConnectionLimit = node.SnapshotConnectionLimit
	}
	return normalizeSnapshotRuntimeConfig(cfg)
}

func (s *Server) loadGlobalSnapshotConfig() protocol.SnapshotConfig {
	cfg := defaultSnapshotRuntimeConfig()
	var rows []model.AppSetting
	keys := []string{
		"snapshot_enabled",
		"snapshot_collect_processes",
		"snapshot_collect_connections",
		"snapshot_mask_sensitive",
		"snapshot_interval_seconds",
		"snapshot_process_limit",
		"snapshot_connection_limit",
	}
	if err := s.db.Where("`key` IN ?", keys).Find(&rows).Error; err != nil {
		s.logger.Warn("load snapshot settings failed", slog.String("error", err.Error()))
		return cfg
	}
	for _, row := range rows {
		value := strings.EqualFold(row.Value, "true")
		switch row.Key {
		case "snapshot_enabled":
			cfg.Enabled = value
		case "snapshot_collect_processes":
			cfg.CollectProcesses = value
		case "snapshot_collect_connections":
			cfg.CollectConnections = value
		case "snapshot_mask_sensitive":
			cfg.MaskSensitive = value
		case "snapshot_interval_seconds":
			cfg.IntervalSeconds = uint32(parseSettingInt(row.Value, int(cfg.IntervalSeconds)))
		case "snapshot_process_limit":
			cfg.ProcessLimit = uint32(parseSettingInt(row.Value, int(cfg.ProcessLimit)))
		case "snapshot_connection_limit":
			cfg.ConnectionLimit = uint32(parseSettingInt(row.Value, int(cfg.ConnectionLimit)))
		}
	}
	return normalizeSnapshotRuntimeConfig(cfg)
}

func defaultSnapshotRuntimeConfig() protocol.SnapshotConfig {
	return protocol.SnapshotConfig{
		Enabled:            false,
		CollectProcesses:   true,
		CollectConnections: true,
		MaskSensitive:      true,
		IntervalSeconds:    60,
		ProcessLimit:       20,
		ConnectionLimit:    200,
	}
}

func normalizeSnapshotRuntimeConfig(cfg protocol.SnapshotConfig) protocol.SnapshotConfig {
	if cfg.IntervalSeconds < 15 || cfg.IntervalSeconds > 3600 {
		cfg.IntervalSeconds = 60
	}
	if cfg.ProcessLimit == 0 || cfg.ProcessLimit > 50 {
		cfg.ProcessLimit = 20
	}
	if cfg.ConnectionLimit == 0 || cfg.ConnectionLimit > 500 {
		cfg.ConnectionLimit = 200
	}
	return cfg
}

func (s *Server) touchNode(nodeID string, agentVersion string) error {
	nowTime := time.Now()
	now := uint64(nowTime.UnixMilli())

	assignments := map[string]any{
		"status":       "online",
		"last_seen_at": now,
		"updated_at":   nowTime,
	}
	if agentVersion != "" {
		assignments["agent_version"] = agentVersion
	}

	node := model.Node{
		NodeID:                     nodeID,
		Name:                       nodeID,
		Region:                     "default",
		Status:                     "online",
		AgentVersion:               agentVersion,
		SecretKeyHash:              "pending",
		LastSeenAt:                 &now,
		HeartbeatInterval:          15,
		MetricsInterval:            15,
		SnapshotCollectProcesses:   true,
		SnapshotCollectConnections: true,
		SnapshotMaskSensitive:      true,
		SnapshotInterval:           60,
		BillingCycle:               "monthly",
		Currency:                   "CNY",
		TrafficBillingDirection:    "bidirectional",
		TrafficResetCycle:          "monthly",
	}

	err := s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "node_id"}},
		DoUpdates: clause.Assignments(assignments),
	}).Create(&node).Error
	if err == nil {
		s.cancelOfflineWeCom(nodeID)
	}
	return err
}

func (s *Server) storeSystemLog(service string, nodeID string, level string, message string) {
	s.storeSystemEventLog(service, nodeID, level, "system.event", message, nil)
}

func (s *Server) storeSystemEventLog(service string, nodeID string, level string, eventType string, message string, meta map[string]any) {
	if message == "" {
		return
	}
	if err := s.db.Create(&model.SystemLog{
		Service:   normalizeSystemLogValue(service, "master"),
		NodeID:    strings.TrimSpace(nodeID),
		Level:     normalizeSystemLogLevel(level),
		EventType: normalizeSystemLogValue(eventType, "system.event"),
		Message:   message,
		MetaJSON:  marshalSystemLogMeta(meta),
	}).Error; err != nil {
		s.logger.Warn("store system log failed", slog.String("error", err.Error()))
	}
}

func normalizeSystemLogValue(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func normalizeSystemLogLevel(level string) string {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "warn", "warning":
		return "warning"
	case "error":
		return "error"
	default:
		return "info"
	}
}

func marshalSystemLogMeta(meta map[string]any) string {
	if len(meta) == 0 {
		return "{}"
	}
	raw, err := json.Marshal(meta)
	if err != nil {
		return "{}"
	}
	return string(raw)
}

func marshalSnapshotJSON(value any, fallback string) string {
	raw, err := json.Marshal(value)
	if err != nil || len(raw) == 0 || string(raw) == "null" {
		return fallback
	}
	return string(raw)
}

func (s *Server) registerConn(nodeID string, conn net.Conn, secure *protocol.SecureConn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.conns[nodeID] = &agentSession{conn: conn, secure: secure}
}

func (s *Server) unregisterConn(conn net.Conn) {
	var offlineNodeID string

	s.mu.Lock()
	for nodeID, candidate := range s.conns {
		if candidate.conn == conn {
			delete(s.conns, nodeID)
			offlineNodeID = nodeID
			break
		}
	}
	s.mu.Unlock()

	if offlineNodeID != "" {
		s.setNodeOffline(offlineNodeID, "agent connection closed")
	}
}

func (s *agentSession) write(message protocol.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.secure.WriteMessage(message)
}

func (s *Server) markStaleNodesOffline() {
	var nodes []model.Node
	if err := s.db.Where("status = ?", "online").Find(&nodes).Error; err != nil {
		s.logger.Warn("scan online nodes failed", slog.String("error", err.Error()))
		return
	}

	now := uint64(time.Now().UnixMilli())
	for _, node := range nodes {
		if !nodeIsStale(node, now) {
			continue
		}
		s.setNodeOffline(node.NodeID, "heartbeat timeout")
	}
}

func (s *Server) markOnlineNodesOfflineOnStart() {
	result := s.db.Model(&model.Node{}).
		Where("status = ?", "online").
		Updates(map[string]any{
			"status":     "offline",
			"updated_at": time.Now(),
		})
	if result.Error != nil {
		s.logger.Warn("mark startup online nodes offline failed", slog.String("error", result.Error.Error()))
		return
	}
	if result.RowsAffected > 0 {
		s.logger.Info("startup online nodes marked offline", slog.Int64("count", result.RowsAffected))
	}
}

func nodeIsStale(node model.Node, now uint64) bool {
	if node.LastSeenAt == nil {
		return true
	}

	heartbeat := node.HeartbeatInterval
	if heartbeat == 0 {
		heartbeat = 15
	}
	timeoutSeconds := heartbeat * 3
	if timeoutSeconds < 30 {
		timeoutSeconds = 30
	}

	return now > *node.LastSeenAt+uint64(timeoutSeconds)*1000
}

func (s *Server) evaluateMetricAlerts(nodeID string, metric model.NodeMetric) {
	settings, err := s.loadWeComSettings()
	if err != nil {
		s.logger.Warn("load notification settings failed", slog.String("error", err.Error()))
		return
	}

	var node model.Node
	if err := s.db.Where("node_id = ?", nodeID).First(&node).Error; err != nil {
		s.logger.Warn("load node for alert failed", slog.String("node_id", nodeID), slog.String("error", err.Error()))
		return
	}

	if alertThresholdEnabled(settings.TrafficAlertEnabled, settings.TrafficAlertPercent) && node.TrafficLimitBytes > 0 {
		used, _ := s.trafficUsage(node, metric, time.Now())
		percent := float64(used) * 100 / float64(node.TrafficLimitBytes)
		message := "月流量使用率已达 " + formatFloat(percent) + "% (当前阈值 " + formatFloat(settings.TrafficAlertPercent) + "%)，已用 " + formatBytes(used) + " / " + formatBytes(node.TrafficLimitBytes)
		s.applyAlert(node, "traffic_monthly", "warning", percent >= settings.TrafficAlertPercent, message, settings.AlertIntervalMinutes)
	} else {
		s.resolveAlert(node.NodeID, "traffic_monthly")
	}
	if alertThresholdEnabled(settings.CPUAlertEnabled, settings.CPUAlertPercent) {
		message := "CPU 使用率已达 " + formatFloat(metric.CPUUsage) + "% (当前阈值 " + formatFloat(settings.CPUAlertPercent) + "%)"
		s.applyAlert(node, "cpu_usage", "warning", metric.CPUUsage >= settings.CPUAlertPercent, message, settings.AlertIntervalMinutes)
	} else {
		s.resolveAlert(node.NodeID, "cpu_usage")
	}
	if alertThresholdEnabled(settings.MemoryAlertEnabled, settings.MemoryAlertPercent) {
		message := "内存使用率已达 " + formatFloat(metric.MemUsedPercent) + "% (当前阈值 " + formatFloat(settings.MemoryAlertPercent) + "%)"
		s.applyAlert(node, "memory_usage", "warning", metric.MemUsedPercent >= settings.MemoryAlertPercent, message, settings.AlertIntervalMinutes)
	} else {
		s.resolveAlert(node.NodeID, "memory_usage")
	}
	if alertThresholdEnabled(settings.DiskLoadAlertEnabled, settings.DiskLoadAlertPercent) {
		message := "磁盘负载已达 " + formatFloat(metric.DiskUsedPercent) + "% (当前阈值 " + formatFloat(settings.DiskLoadAlertPercent) + "%)"
		s.applyAlert(node, "disk_load", "warning", metric.DiskUsedPercent >= settings.DiskLoadAlertPercent, message, settings.AlertIntervalMinutes)
	} else {
		s.resolveAlert(node.NodeID, "disk_load")
	}
	if alertThresholdEnabled(settings.LoadAlertEnabled, settings.LoadAlertThreshold) {
		message := "1 分钟负载已达 " + formatFloat(metric.Load1) + " (当前阈值 " + formatFloat(settings.LoadAlertThreshold) + ")"
		s.applyAlert(node, "load1", "warning", metric.Load1 >= settings.LoadAlertThreshold, message, settings.AlertIntervalMinutes)
	} else {
		s.resolveAlert(node.NodeID, "load1")
	}
}

func (s *Server) checkExpiryAlerts() {
	settings, err := s.loadWeComSettings()
	if err != nil {
		s.logger.Warn("load expiry alert settings failed", slog.String("error", err.Error()))
		return
	}

	var nodes []model.Node
	if err := s.db.Where("service_expires_at IS NOT NULL").Find(&nodes).Error; err != nil {
		s.logger.Warn("load nodes for expiry alerts failed", slog.String("error", err.Error()))
		return
	}

	now := time.Now()
	for _, node := range nodes {
		days, ok := daysUntil(node.ServiceExpiresAt, now)
		s.resolveAlert(node.NodeID, "expiry_monthly")
		s.resolveAlert(node.NodeID, "expiry_yearly")
		if settings.ExpiryAlertEnabled {
			message := billingCycleText(node.BillingCycle) + "服务将在 " + strconv.FormatInt(days, 10) + " 天后到期 (提前提醒 " + strconv.Itoa(settings.ExpiryAlertDays) + " 天)"
			s.applyAlert(node, "expiry", "warning", ok && days <= int64(settings.ExpiryAlertDays), message, settings.AlertIntervalMinutes)
		} else {
			s.resolveAlert(node.NodeID, "expiry")
		}
	}
}

func (s *Server) applyAlert(node model.Node, ruleType string, level string, active bool, message string, intervalMinutes int) {
	if !active {
		s.resolveAlert(node.NodeID, ruleType)
		return
	}
	if s.activateAlert(node.NodeID, ruleType, level, message, intervalMinutes) {
		s.storeSystemEventLog("master", node.NodeID, "warning", "alert.triggered", "alert triggered: "+message, map[string]any{
			"rule_type": ruleType,
			"level":     level,
		})
		s.notifyWeComAlertAsync(node.NodeID, ruleType, level, message)
	}
}

func (s *Server) activateAlert(nodeID string, ruleType string, level string, message string, intervalMinutes int) bool {
	now := uint64(time.Now().UnixMilli())
	var alert model.Alert
	err := s.db.Where("node_id = ? AND rule_type = ? AND status = ?", nodeID, ruleType, "active").First(&alert).Error
	if err == nil {
		shouldNotify := alert.LastTriggeredAt == nil || now >= *alert.LastTriggeredAt+uint64(normalizeAlertInterval(intervalMinutes))*60*1000
		updates := map[string]any{"message": message}
		if shouldNotify {
			updates["last_triggered_at"] = now
		}
		if err := s.db.Model(&alert).Updates(updates).Error; err != nil {
			s.logger.Warn("refresh alert failed", slog.String("node_id", nodeID), slog.String("rule_type", ruleType), slog.String("error", err.Error()))
		}
		return shouldNotify
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		s.logger.Warn("load active alert failed", slog.String("node_id", nodeID), slog.String("rule_type", ruleType), slog.String("error", err.Error()))
		return false
	}

	alert = model.Alert{
		NodeID:           nodeID,
		RuleType:         ruleType,
		Level:            level,
		Status:           "active",
		Message:          message,
		FirstTriggeredAt: &now,
		LastTriggeredAt:  &now,
	}
	if err := s.db.Create(&alert).Error; err != nil {
		s.logger.Warn("create alert failed", slog.String("node_id", nodeID), slog.String("rule_type", ruleType), slog.String("error", err.Error()))
		return false
	}
	return true
}

func (s *Server) resolveAlert(nodeID string, ruleType string) {
	now := uint64(time.Now().UnixMilli())
	if err := s.db.Model(&model.Alert{}).
		Where("node_id = ? AND rule_type = ? AND status = ?", nodeID, ruleType, "active").
		Updates(map[string]any{
			"status":      "resolved",
			"resolved_at": now,
		}).Error; err != nil {
		s.logger.Warn("resolve alert failed", slog.String("node_id", nodeID), slog.String("rule_type", ruleType), slog.String("error", err.Error()))
	}
}

func alertThresholdEnabled(enabled bool, threshold float64) bool {
	return enabled && threshold > 0
}

func (s *Server) setNodeOffline(nodeID string, reason string) {
	result := s.db.Model(&model.Node{}).
		Where("node_id = ? AND status <> ?", nodeID, "offline").
		Updates(map[string]any{
			"status":     "offline",
			"updated_at": time.Now(),
		})
	if result.Error != nil {
		s.logger.Warn("mark node offline failed", slog.String("node_id", nodeID), slog.String("error", result.Error.Error()))
		return
	}
	if result.RowsAffected == 0 {
		return
	}

	s.logger.Error("node marked offline", slog.String("event_type", "agent.offline"), slog.String("node_id", nodeID), slog.String("reason", reason))
	s.storeSystemEventLog("agent", nodeID, "warning", "agent.offline", "agent marked offline: "+reason, map[string]any{
		"reason": reason,
	})
	s.scheduleOfflineWeCom(nodeID, reason)
}

func (s *Server) scheduleOfflineWeCom(nodeID string, reason string) {
	if nodeID == "" {
		return
	}
	settings, err := s.loadWeComSettings()
	if err != nil {
		s.logger.Warn("load offline wecom settings failed", slog.String("node_id", nodeID), slog.String("error", err.Error()))
		return
	}
	if !notificationChannelEnabled(settings) {
		return
	}
	delay := time.Duration(normalizeOfflineAlertDelay(settings.OfflineAlertDelayMinutes)) * time.Minute

	s.mu.Lock()
	if existing := s.offlineNotifyTimers[nodeID]; existing != nil {
		existing.Stop()
		delete(s.offlineNotifyTimers, nodeID)
	}
	if delay <= 0 {
		s.mu.Unlock()
		s.notifyOfflineWeComIfStillOffline(nodeID, reason)
		return
	}

	var timer *time.Timer
	timer = time.AfterFunc(delay, func() {
		s.mu.Lock()
		if s.offlineNotifyTimers[nodeID] != timer {
			s.mu.Unlock()
			return
		}
		delete(s.offlineNotifyTimers, nodeID)
		s.mu.Unlock()

		s.notifyOfflineWeComIfStillOffline(nodeID, reason)
	})
	s.offlineNotifyTimers[nodeID] = timer
	s.mu.Unlock()
}

func (s *Server) notifyOfflineWeComIfStillOffline(nodeID string, reason string) {
	if !s.nodeStillOffline(nodeID) {
		return
	}
	s.notifyWeComAsync(nodeID, "offline", reason)
}

func (s *Server) cancelOfflineWeCom(nodeID string) {
	if nodeID == "" {
		return
	}

	s.mu.Lock()
	if timer := s.offlineNotifyTimers[nodeID]; timer != nil {
		timer.Stop()
		delete(s.offlineNotifyTimers, nodeID)
	}
	s.mu.Unlock()
}

func (s *Server) nodeStillOffline(nodeID string) bool {
	var node model.Node
	if err := s.db.Select("status").Where("node_id = ?", nodeID).First(&node).Error; err != nil {
		s.logger.Warn("check node offline status failed", slog.String("node_id", nodeID), slog.String("error", err.Error()))
		return false
	}
	return node.Status == "offline"
}

func (s *Server) notifyWeComAsync(nodeID string, status string, reason string) {
	go func() {
		if err := s.notifyWeCom(nodeID, status, reason); err != nil {
			s.logger.Warn("send notification failed", slog.String("node_id", nodeID), slog.String("status", status), slog.String("error", err.Error()))
		}
	}()
}

func (s *Server) notifyWeComAlertAsync(nodeID string, ruleType string, level string, message string) {
	go func() {
		if err := s.notifyWeComAlert(nodeID, ruleType, level, message); err != nil {
			s.logger.Warn("send alert notification failed", slog.String("node_id", nodeID), slog.String("rule_type", ruleType), slog.String("error", err.Error()))
		}
	}()
}

func (s *Server) notifyWeCom(nodeID string, status string, reason string) error {
	settings, err := s.loadWeComSettings()
	if err != nil {
		return err
	}
	if !notificationChannelEnabled(settings) {
		return nil
	}

	var node model.Node
	if err := s.db.Where("node_id = ?", nodeID).First(&node).Error; err != nil {
		return err
	}

	name := nodeDisplayName(node)
	now := time.Now().Format("2006-01-02 15:04:05")
	lines := []string{
		"【上线通知】",
		"",
		"🟢 Rivo 节点上线通知",
		"节点名称： " + name,
		"节点 ID： " + node.NodeID,
	}
	if strings.TrimSpace(node.Tag) != "" {
		lines = append(lines, "节点标签： "+strings.TrimSpace(node.Tag))
	}
	lines = append(lines,
		"公网 IP： "+weComNodeAddress(node, false),
		"✅ 当前状态： Online",
		"📝 上线原因： "+emptyDash(reason),
		"🕒 上线时间： "+now,
	)
	if status == "offline" {
		lines = []string{
			"【离线通知】",
			"",
			"🔴 Rivo 节点离线通知",
			"节点名称： " + name,
			"节点 ID： " + node.NodeID,
		}
		if strings.TrimSpace(node.Tag) != "" {
			lines = append(lines, "节点标签： "+strings.TrimSpace(node.Tag))
		}
		lines = append(lines,
			"公网 IP： "+weComNodeAddress(node, true),
			"⚠️ 当前状态： "+offlineWeComStatusText(settings.OfflineAlertDelayMinutes),
			"📝 离线原因： "+emptyDash(reason),
			"🕒 最后在线： "+formatUnixMilli(node.LastSeenAt),
			"🕒 通知时间： "+now,
		)
	}

	subject := "Rivo 节点上线通知 - " + name
	if status == "offline" {
		subject = "Rivo 节点离线通知 - " + name
	}
	return s.postNotification(settings, subject, strings.Join(lines, "\n"))
}

func (s *Server) notifyWeComAlert(nodeID string, ruleType string, level string, message string) error {
	settings, err := s.loadWeComSettings()
	if err != nil {
		return err
	}
	if !notificationChannelEnabled(settings) {
		return nil
	}

	var node model.Node
	if err := s.db.Where("node_id = ?", nodeID).First(&node).Error; err != nil {
		return err
	}

	lines := []string{
		"🚨 Rivo 节点 " + alertRuleLabel(ruleType),
		"",
		"节点名称： " + nodeDisplayName(node),
		"节点 ID： " + node.NodeID,
	}
	if strings.TrimSpace(node.Tag) != "" {
		lines = append(lines, "节点标签： "+strings.TrimSpace(node.Tag))
	}
	lines = append(lines,
		"公网 IP： "+weComNodeAddress(node, true),
		"⚠️ 告警级别： "+weComLevelText(level),
		"📊 告警详情： "+message,
		"🕒 触发时间： "+time.Now().Format("2006-01-02 15:04:05"),
	)
	content := strings.Join(lines, "\n")
	subject := "Rivo 节点告警 - " + nodeDisplayName(node) + " - " + alertRuleLabel(ruleType)
	return s.postNotification(settings, subject, content)
}

func notificationChannelEnabled(settings weComSettings) bool {
	return (settings.Enabled && strings.TrimSpace(settings.WebhookURL) != "") ||
		(settings.TelegramEnabled && strings.TrimSpace(settings.TelegramBotToken) != "" && strings.TrimSpace(settings.TelegramChatID) != "") ||
		notify.EmailChannelReady(emailSettingsFromNotificationSettings(settings))
}

func (s *Server) postNotification(settings weComSettings, subject string, content string) error {
	var errs []error
	if settings.Enabled && strings.TrimSpace(settings.WebhookURL) != "" {
		if err := s.postWeCom(settings, content); err != nil {
			errs = append(errs, err)
		}
	}
	if settings.TelegramEnabled && strings.TrimSpace(settings.TelegramBotToken) != "" && strings.TrimSpace(settings.TelegramChatID) != "" {
		if err := s.postTelegram(settings, content); err != nil {
			errs = append(errs, err)
		}
	}
	emailSettings := emailSettingsFromNotificationSettings(settings)
	if notify.EmailChannelReady(emailSettings) {
		if err := notify.SendEmail(emailSettings, subject, content); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (s *Server) postWeCom(settings weComSettings, content string) error {
	body, err := json.Marshal(map[string]any{
		"msgtype": "text",
		"text": map[string]string{
			"content": content,
		},
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, settings.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errors.New("wecom webhook returned " + resp.Status)
	}
	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err == nil && result.ErrCode != 0 {
		if result.ErrMsg == "" {
			result.ErrMsg = "unknown wecom error"
		}
		return errors.New(result.ErrMsg)
	}
	return nil
}

func (s *Server) postTelegram(settings weComSettings, content string) error {
	endpoint := "https://api.telegram.org/bot" + strings.TrimSpace(settings.TelegramBotToken) + "/sendMessage"
	form := url.Values{}
	form.Set("chat_id", strings.TrimSpace(settings.TelegramChatID))
	form.Set("text", content)

	req, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	decodeErr := json.NewDecoder(resp.Body).Decode(&result)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if decodeErr == nil && result.Description != "" {
			return errors.New("telegram api returned " + resp.Status + ": " + result.Description)
		}
		return errors.New("telegram api returned " + resp.Status)
	}
	if decodeErr == nil && !result.OK {
		if result.Description == "" {
			result.Description = "unknown telegram error"
		}
		return errors.New(result.Description)
	}
	return nil
}

func (s *Server) loadWeComSettings() (weComSettings, error) {
	var rows []model.AppSetting
	keys := []string{
		"wecom_webhook_enabled",
		"wecom_webhook_url",
		"telegram_enabled",
		"telegram_bot_token",
		"telegram_chat_id",
		"email_enabled",
		"email_smtp_host",
		"email_smtp_port",
		"email_smtp_security",
		"email_smtp_username",
		"email_smtp_password",
		"email_from",
		"email_to",
		"traffic_alert_enabled",
		"traffic_alert_percent",
		"cpu_alert_enabled",
		"cpu_alert_percent",
		"memory_alert_enabled",
		"memory_alert_percent",
		"disk_load_alert_enabled",
		"disk_load_alert_percent",
		"load_alert_enabled",
		"load_alert_threshold",
		"alert_interval_minutes",
		"offline_alert_delay_minutes",
		"expiry_alert_enabled",
		"expiry_alert_days",
	}
	if err := s.db.Where("`key` IN ?", keys).Find(&rows).Error; err != nil {
		return weComSettings{}, err
	}

	settings := defaultWeComSettings()
	for _, row := range rows {
		value := strings.EqualFold(row.Value, "true")
		switch row.Key {
		case "wecom_webhook_enabled":
			settings.Enabled = value
		case "wecom_webhook_url":
			settings.WebhookURL = strings.TrimSpace(row.Value)
		case "telegram_enabled":
			settings.TelegramEnabled = value
		case "telegram_bot_token":
			settings.TelegramBotToken = strings.TrimSpace(row.Value)
		case "telegram_chat_id":
			settings.TelegramChatID = strings.TrimSpace(row.Value)
		case "email_enabled":
			settings.EmailEnabled = value
		case "email_smtp_host":
			settings.EmailSMTPHost = strings.TrimSpace(row.Value)
		case "email_smtp_port":
			settings.EmailSMTPPort = parseSettingInt(row.Value, settings.EmailSMTPPort)
		case "email_smtp_security":
			settings.EmailSMTPSecurity = notify.NormalizeEmailSecurity(row.Value)
		case "email_smtp_username":
			settings.EmailSMTPUsername = strings.TrimSpace(row.Value)
		case "email_smtp_password":
			settings.EmailSMTPPassword = strings.TrimSpace(row.Value)
		case "email_from":
			settings.EmailFrom = strings.TrimSpace(row.Value)
		case "email_to":
			settings.EmailTo = strings.TrimSpace(row.Value)
		case "traffic_alert_enabled":
			settings.TrafficAlertEnabled = value
		case "traffic_alert_percent":
			settings.TrafficAlertPercent = parseSettingFloat(row.Value, settings.TrafficAlertPercent)
		case "cpu_alert_enabled":
			settings.CPUAlertEnabled = value
		case "cpu_alert_percent":
			settings.CPUAlertPercent = parseSettingFloat(row.Value, settings.CPUAlertPercent)
		case "memory_alert_enabled":
			settings.MemoryAlertEnabled = value
		case "memory_alert_percent":
			settings.MemoryAlertPercent = parseSettingFloat(row.Value, settings.MemoryAlertPercent)
		case "disk_load_alert_enabled":
			settings.DiskLoadAlertEnabled = value
		case "disk_load_alert_percent":
			settings.DiskLoadAlertPercent = parseSettingFloat(row.Value, settings.DiskLoadAlertPercent)
		case "load_alert_enabled":
			settings.LoadAlertEnabled = value
		case "load_alert_threshold":
			settings.LoadAlertThreshold = parseSettingFloat(row.Value, settings.LoadAlertThreshold)
		case "alert_interval_minutes":
			settings.AlertIntervalMinutes = normalizeAlertInterval(parseSettingInt(row.Value, settings.AlertIntervalMinutes))
		case "offline_alert_delay_minutes":
			settings.OfflineAlertDelayMinutes = normalizeOfflineAlertDelay(parseSettingInt(row.Value, settings.OfflineAlertDelayMinutes))
		case "expiry_alert_enabled":
			settings.ExpiryAlertEnabled = value
		case "expiry_alert_days":
			settings.ExpiryAlertDays = parseSettingInt(row.Value, settings.ExpiryAlertDays)
		}
	}
	settings.EmailSMTPSecurity = notify.NormalizeEmailSecurity(settings.EmailSMTPSecurity)
	if settings.EmailSMTPPort == 0 {
		settings.EmailSMTPPort = defaultWeComSettings().EmailSMTPPort
	}
	return settings, nil
}

func defaultWeComSettings() weComSettings {
	return weComSettings{
		EmailSMTPPort:            587,
		EmailSMTPSecurity:        notify.EmailSecuritySTARTTLS,
		TrafficAlertEnabled:      true,
		TrafficAlertPercent:      80,
		CPUAlertEnabled:          true,
		CPUAlertPercent:          85,
		MemoryAlertEnabled:       true,
		MemoryAlertPercent:       85,
		DiskLoadAlertEnabled:     true,
		DiskLoadAlertPercent:     90,
		LoadAlertEnabled:         true,
		LoadAlertThreshold:       5,
		AlertIntervalMinutes:     30,
		OfflineAlertDelayMinutes: 1,
		ExpiryAlertEnabled:       true,
		ExpiryAlertDays:          7,
	}
}

func emailSettingsFromNotificationSettings(settings weComSettings) notify.EmailSettings {
	return notify.NormalizeEmailSettings(notify.EmailSettings{
		Enabled:  settings.EmailEnabled,
		Host:     settings.EmailSMTPHost,
		Port:     settings.EmailSMTPPort,
		Security: settings.EmailSMTPSecurity,
		Username: settings.EmailSMTPUsername,
		Password: settings.EmailSMTPPassword,
		From:     settings.EmailFrom,
		To:       settings.EmailTo,
	})
}

func emptyDash(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	return value
}

func formatUnixMilli(value *uint64) string {
	if value == nil || *value == 0 {
		return "-"
	}
	return time.UnixMilli(int64(*value)).Format("2006-01-02 15:04:05")
}

func nodeDisplayName(node model.Node) string {
	name := strings.TrimSpace(node.Name)
	if name == "" {
		return node.NodeID
	}
	return name
}

func weComNodeAddress(node model.Node, withRegion bool) string {
	ip := emptyDash(primaryNodePublicAddress(node))
	region := strings.TrimSpace(node.Region)
	if region == "" {
		region = "default"
	}
	address := ip + " " + regionFlag(region)
	if withRegion {
		address += " (" + region + ")"
	}
	return address
}

func primaryNodePublicAddress(node model.Node) string {
	if strings.TrimSpace(node.PublicIP) != "" {
		return strings.TrimSpace(node.PublicIP)
	}
	return strings.TrimSpace(node.PublicIPv6)
}

func regionFlag(region string) string {
	code := strings.ToUpper(strings.TrimSpace(region))
	switch code {
	case "CN":
		return "🇨🇳"
	case "HK":
		return "🇭🇰"
	case "TW":
		return "🇹🇼"
	case "JP":
		return "🇯🇵"
	case "KR":
		return "🇰🇷"
	case "SG":
		return "🇸🇬"
	case "US":
		return "🇺🇸"
	case "GB", "UK":
		return "🇬🇧"
	case "DE":
		return "🇩🇪"
	case "FR":
		return "🇫🇷"
	case "NL":
		return "🇳🇱"
	case "CA":
		return "🇨🇦"
	case "AU":
		return "🇦🇺"
	default:
		return "🌐"
	}
}

func weComLevelText(level string) string {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "critical":
		return "Critical"
	case "error":
		return "Error"
	case "info":
		return "Info"
	default:
		return "Warning"
	}
}

func alertRuleLabel(ruleType string) string {
	switch ruleType {
	case "traffic_monthly":
		return "月流量预警"
	case "cpu_usage":
		return "CPU 预警"
	case "memory_usage":
		return "内存预警"
	case "disk_load":
		return "磁盘负载预警"
	case "load1":
		return "负载预警"
	case "expiry", "expiry_monthly", "expiry_yearly":
		return "服务到期提醒"
	default:
		return "节点告警"
	}
}

func billingCycleText(cycle string) string {
	switch strings.ToLower(strings.TrimSpace(cycle)) {
	case "daily":
		return "日付"
	case "yearly":
		return "年付"
	case "one_time":
		return "一次性"
	default:
		return "月付"
	}
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', 2, 64)
}

func parseSettingFloat(raw string, fallback float64) float64 {
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return fallback
	}
	return value
}

func parseSettingInt(raw string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return fallback
	}
	return value
}

func normalizeAlertInterval(value int) int {
	for _, allowed := range []int{5, 10, 30, 60, 240, 720, 1440} {
		if value == allowed {
			return value
		}
	}
	return 30
}

func normalizeOfflineAlertDelay(value int) int {
	if value < 0 {
		return 1
	}
	if value > 1440 {
		return 1440
	}
	return value
}

func offlineWeComStatusText(delayMinutes int) string {
	delayMinutes = normalizeOfflineAlertDelay(delayMinutes)
	if delayMinutes == 0 {
		return "Offline（立即通知）"
	}
	return "Offline（持续 " + strconv.Itoa(delayMinutes) + " 分钟）"
}

func formatBytes(value uint64) string {
	const unit = 1024
	if value < unit {
		return strconv.FormatUint(value, 10) + " B"
	}
	units := []string{"KB", "MB", "GB", "TB", "PB"}
	size := float64(value)
	index := -1
	for size >= unit && index < len(units)-1 {
		size /= unit
		index++
	}
	return formatFloat(size) + " " + units[index]
}

func daysUntil(value *uint64, now time.Time) (int64, bool) {
	if value == nil || *value == 0 {
		return 0, false
	}
	remaining := int64(*value) - now.UnixMilli()
	if remaining <= 0 {
		return 0, false
	}
	dayMillis := int64(24 * time.Hour / time.Millisecond)
	return (remaining + dayMillis - 1) / dayMillis, true
}

func (s *Server) trafficUsage(node model.Node, metric model.NodeMetric, now time.Time) (uint64, uint64) {
	direction := normalizeTrafficBillingDirection(node.TrafficBillingDirection)
	used := metricTrafficTotal(metric, direction)
	if cycleStart := trafficCycleStart(node, now); cycleStart > 0 {
		used = s.trafficUsedSince(node.NodeID, metric, cycleStart, direction, used)
	}
	if trafficCalibrationApplies(node, now) {
		used = saturatingAddUint64(used, node.TrafficCalibrationBytes)
	}
	if node.TrafficLimitBytes == 0 {
		return used, 0
	}
	if used >= node.TrafficLimitBytes {
		return used, 0
	}
	return used, node.TrafficLimitBytes - used
}

func trafficCalibrationApplies(node model.Node, now time.Time) bool {
	if node.TrafficCalibrationBytes == 0 {
		return false
	}
	cycleStart := trafficCycleStart(node, now)
	if cycleStart == 0 {
		return true
	}
	if node.TrafficCalibrationAt == nil {
		return true
	}
	return *node.TrafficCalibrationAt >= cycleStart
}

func normalizeTrafficBillingDirection(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "outbound":
		return "outbound"
	default:
		return "bidirectional"
	}
}

func saturatingAddUint64(left uint64, right uint64) uint64 {
	if ^uint64(0)-left < right {
		return ^uint64(0)
	}
	return left + right
}

func metricTrafficTotal(metric model.NodeMetric, direction string) uint64 {
	if normalizeTrafficBillingDirection(direction) == "outbound" {
		return metric.NetTxBytesTotal
	}
	return metric.NetRxBytesTotal + metric.NetTxBytesTotal
}

func trafficCycleStart(node model.Node, now time.Time) uint64 {
	year, month, day := now.Date()
	location := now.Location()
	switch strings.ToLower(node.TrafficResetCycle) {
	case "daily":
		return uint64(time.Date(year, month, day, 0, 0, 0, 0, location).UnixMilli())
	case "yearly":
		if node.ServiceStartedAt != nil && *node.ServiceStartedAt > 0 {
			return uint64(anchoredYearlyCycleStart(time.UnixMilli(int64(*node.ServiceStartedAt)), now).UnixMilli())
		}
		return uint64(time.Date(year, time.January, 1, 0, 0, 0, 0, location).UnixMilli())
	case "never":
		return 0
	default:
		if node.ServiceStartedAt != nil && *node.ServiceStartedAt > 0 {
			return uint64(anchoredMonthlyCycleStart(time.UnixMilli(int64(*node.ServiceStartedAt)), now).UnixMilli())
		}
		return uint64(time.Date(year, month, 1, 0, 0, 0, 0, location).UnixMilli())
	}
}

func anchoredMonthlyCycleStart(anchor time.Time, now time.Time) time.Time {
	location := now.Location()
	anchor = anchor.In(location)
	hour, minute, second := anchor.Clock()
	day := anchor.Day()
	candidate := anchoredDate(now.Year(), now.Month(), day, hour, minute, second, anchor.Nanosecond(), location)
	if candidate.After(now) {
		previousYear, previousMonth := previousCalendarMonth(now.Year(), now.Month())
		candidate = anchoredDate(previousYear, previousMonth, day, hour, minute, second, anchor.Nanosecond(), location)
	}
	if candidate.Before(anchor) {
		return anchor
	}
	return candidate
}

func anchoredYearlyCycleStart(anchor time.Time, now time.Time) time.Time {
	location := now.Location()
	anchor = anchor.In(location)
	hour, minute, second := anchor.Clock()
	day := anchor.Day()
	month := anchor.Month()
	candidate := anchoredDate(now.Year(), month, day, hour, minute, second, anchor.Nanosecond(), location)
	if candidate.After(now) {
		candidate = anchoredDate(now.Year()-1, month, day, hour, minute, second, anchor.Nanosecond(), location)
	}
	if candidate.Before(anchor) {
		return anchor
	}
	return candidate
}

func anchoredDate(year int, month time.Month, day int, hour int, minute int, second int, nanosecond int, location *time.Location) time.Time {
	return time.Date(year, month, minInt(day, daysInMonth(year, month)), hour, minute, second, nanosecond, location)
}

func previousCalendarMonth(year int, month time.Month) (int, time.Month) {
	if month == time.January {
		return year - 1, time.December
	}
	return year, month - 1
}

func daysInMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.Local).Day()
}

func minInt(left int, right int) int {
	if left < right {
		return left
	}
	return right
}

func (s *Server) trafficUsedSince(nodeID string, latest model.NodeMetric, cycleStart uint64, direction string, fallback uint64) uint64 {
	var baseline model.NodeMetric
	err := s.db.Where("node_id = ? AND ts <= ?", nodeID, cycleStart).Order("ts desc").First(&baseline).Error
	if err != nil {
		err = s.db.Where("node_id = ? AND ts >= ? AND ts <= ?", nodeID, cycleStart, latest.Timestamp).Order("ts asc").First(&baseline).Error
	}
	if err != nil || baseline.ID == latest.ID {
		return fallback
	}

	baselineTotal := metricTrafficTotal(baseline, direction)
	latestTotal := metricTrafficTotal(latest, direction)
	if latestTotal < baselineTotal {
		return latestTotal
	}
	return latestTotal - baselineTotal
}

func remoteIP(addr net.Addr) string {
	if addr == nil {
		return ""
	}
	host, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		return addr.String()
	}
	return host
}

func normalizeIPAddresses(addresses protocol.IPAddresses) protocol.IPAddresses {
	var result protocol.IPAddresses
	seen := make(map[string]struct{})
	for _, value := range addresses.IPv4 {
		appendNormalizedIP(&result, seen, value)
	}
	for _, value := range addresses.IPv6 {
		appendNormalizedIP(&result, seen, value)
	}
	return result
}

func normalizePublicIPs(addresses protocol.PublicIPs) protocol.PublicIPs {
	var result protocol.PublicIPs
	for _, value := range addresses.IPv4 {
		appendPublicIPObservation(&result, value, 0)
	}
	for _, value := range addresses.IPv6 {
		appendPublicIPObservation(&result, value, 0)
	}
	return result
}

func appendNormalizedIP(result *protocol.IPAddresses, seen map[string]struct{}, raw string) {
	normalized := normalizeUsableNodeIP(raw)
	if normalized == "" {
		return
	}
	if _, exists := seen[normalized]; exists {
		return
	}
	seen[normalized] = struct{}{}
	if net.ParseIP(normalized).To4() != nil {
		result.IPv4 = append(result.IPv4, normalized)
		return
	}
	result.IPv6 = append(result.IPv6, normalized)
}

func normalizeUsableNodeIP(raw string) string {
	ip := net.ParseIP(strings.TrimSpace(raw))
	if ip == nil {
		return ""
	}
	if ip.IsLoopback() || ip.IsUnspecified() || ip.IsMulticast() || ip.IsLinkLocalMulticast() || ip.IsLinkLocalUnicast() {
		return ""
	}
	if ipv4 := ip.To4(); ipv4 != nil {
		return ipv4.String()
	}
	if ipv6 := ip.To16(); ipv6 != nil {
		return ipv6.String()
	}
	return ""
}

func normalizePublicNodeIP(raw string) string {
	normalized := normalizeUsableNodeIP(raw)
	if normalized == "" {
		return ""
	}
	ip := net.ParseIP(normalized)
	if ip == nil || !isPublicNodeIP(ip) {
		return ""
	}
	return normalized
}

func isPublicNodeIP(ip net.IP) bool {
	return ip.IsGlobalUnicast() &&
		!ip.IsPrivate() &&
		!ip.IsLoopback() &&
		!ip.IsUnspecified() &&
		!ip.IsMulticast() &&
		!ip.IsLinkLocalMulticast() &&
		!ip.IsLinkLocalUnicast() &&
		!isCGNATNodeIPv4(ip)
}

func isCGNATNodeIPv4(ip net.IP) bool {
	ipv4 := ip.To4()
	return ipv4 != nil && ipv4[0] == 100 && ipv4[1]&0b1100_0000 == 64
}

func selectPrimaryNodeIP(reportedPrimary string, addresses protocol.IPAddresses, connectionIP string) string {
	if ip := firstPublicNodeIP([]string{reportedPrimary}); ip != "" {
		return ip
	}
	if ip := firstPublicNodeIP(addresses.IPv4); ip != "" {
		return ip
	}
	if ip := firstPublicNodeIP(addresses.IPv6); ip != "" {
		return ip
	}
	if ip := firstPublicNodeIP([]string{connectionIP}); ip != "" {
		return ip
	}
	if ip := normalizeUsableNodeIP(reportedPrimary); ip != "" {
		return ip
	}
	if len(addresses.IPv4) > 0 {
		return addresses.IPv4[0]
	}
	if len(addresses.IPv6) > 0 {
		return addresses.IPv6[0]
	}
	return connectionIP
}

func firstPublicNodeIP(values []string) string {
	for _, value := range values {
		normalized := normalizePublicNodeIP(value)
		if normalized == "" {
			continue
		}
		return normalized
	}
	return ""
}

func (s *Server) findNodeForIPMerge(nodeID string) model.Node {
	var node model.Node
	if strings.TrimSpace(nodeID) == "" {
		return node
	}
	_ = s.db.Select("node_id", "public_ip", "public_ipv6", "public_ips_json").Where("node_id = ?", nodeID).First(&node).Error
	return node
}

func mergeNodePublicIPs(existing model.Node, reportedPrimary string, _ protocol.IPAddresses, reportedPublicIPs protocol.PublicIPs, connectionIP string, seenAt int64) protocol.PublicIPs {
	merged := parsePublicIPsForTCP(existing.PublicIPsJSON)
	appendPublicIPObservation(&merged, protocol.PublicIPObservation{IP: existing.PublicIP, Source: "existing"}, 0)
	appendPublicIPObservation(&merged, protocol.PublicIPObservation{IP: existing.PublicIPv6, Source: "existing"}, 0)
	for _, item := range reportedPublicIPs.IPv4 {
		appendPublicIPObservation(&merged, item, seenAt)
	}
	for _, item := range reportedPublicIPs.IPv6 {
		appendPublicIPObservation(&merged, item, seenAt)
	}
	appendPublicIPObservation(&merged, protocol.PublicIPObservation{IP: reportedPrimary, Source: "agent_reported"}, seenAt)
	appendPublicIPObservation(&merged, protocol.PublicIPObservation{IP: connectionIP, Source: "master_remote"}, seenAt)
	return merged
}

func appendPublicIPObservation(result *protocol.PublicIPs, item protocol.PublicIPObservation, seenAt int64) {
	normalized := normalizePublicNodeIP(item.IP)
	if normalized == "" {
		return
	}
	source := strings.TrimSpace(item.Source)
	if source == "" {
		source = "unknown"
	}
	firstSeen := item.FirstSeen
	if firstSeen <= 0 {
		firstSeen = item.LastSeen
	}
	if firstSeen <= 0 {
		firstSeen = seenAt
	}
	lastSeen := item.LastSeen
	if lastSeen <= 0 {
		lastSeen = seenAt
	}
	if lastSeen < firstSeen {
		lastSeen = firstSeen
	}

	list := &result.IPv6
	if net.ParseIP(normalized).To4() != nil {
		list = &result.IPv4
	}
	for index := range *list {
		if (*list)[index].IP != normalized {
			continue
		}
		if (*list)[index].FirstSeen <= 0 || (firstSeen > 0 && firstSeen < (*list)[index].FirstSeen) {
			(*list)[index].FirstSeen = firstSeen
		}
		if lastSeen > (*list)[index].LastSeen {
			(*list)[index].LastSeen = lastSeen
		}
		if (*list)[index].Source == "" || (*list)[index].Source == "unknown" {
			(*list)[index].Source = source
		}
		return
	}
	*list = append(*list, protocol.PublicIPObservation{
		IP:        normalized,
		Source:    source,
		FirstSeen: firstSeen,
		LastSeen:  lastSeen,
	})
}

func primaryPublicIPv4(existing model.Node, addresses protocol.PublicIPs) string {
	if ip := normalizePublicNodeIP(existing.PublicIP); ip != "" && net.ParseIP(ip).To4() != nil {
		return ip
	}
	if len(addresses.IPv4) > 0 {
		return addresses.IPv4[0].IP
	}
	return ""
}

func primaryPublicIPv6(existing model.Node, addresses protocol.PublicIPs) string {
	if ip := normalizePublicNodeIP(existing.PublicIPv6); ip != "" && net.ParseIP(ip).To4() == nil {
		return ip
	}
	if ip := normalizePublicNodeIP(existing.PublicIP); ip != "" && net.ParseIP(ip).To4() == nil {
		return ip
	}
	if len(addresses.IPv6) > 0 {
		return addresses.IPv6[0].IP
	}
	return ""
}

func marshalIPAddresses(addresses protocol.IPAddresses) string {
	if len(addresses.IPv4) == 0 && len(addresses.IPv6) == 0 {
		return ""
	}
	raw, err := json.Marshal(addresses)
	if err != nil {
		return ""
	}
	return string(raw)
}

func marshalPublicIPs(addresses protocol.PublicIPs) string {
	if len(addresses.IPv4) == 0 && len(addresses.IPv6) == 0 {
		return ""
	}
	raw, err := json.Marshal(addresses)
	if err != nil {
		return ""
	}
	return string(raw)
}

func parsePublicIPsForTCP(raw string) protocol.PublicIPs {
	var addresses protocol.PublicIPs
	if strings.TrimSpace(raw) == "" {
		return addresses
	}
	if err := json.Unmarshal([]byte(raw), &addresses); err != nil {
		return protocol.PublicIPs{}
	}
	return normalizePublicIPs(addresses)
}

func (s *Server) randomNodeID() (string, error) {
	for range 8 {
		raw := make([]byte, 8)
		if _, err := rand.Read(raw); err != nil {
			return "", err
		}
		nodeID := hex.EncodeToString(raw)
		var count int64
		if err := s.db.Model(&model.Node{}).Where("node_id = ?", nodeID).Count(&count).Error; err != nil {
			return "", err
		}
		if count == 0 {
			return nodeID, nil
		}
	}
	return "", errors.New("generate random node_id failed")
}
