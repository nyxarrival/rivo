package client

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"rivo/internal/agent/collector"
	agentstate "rivo/internal/agent/state"
	"rivo/internal/protocol"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

type Options struct {
	MasterAddr             string
	SecretKey              string
	AgentVersion           string
	NodeID                 string
	StateFile              string
	PublicIPEnabled        bool
	PublicIPTimeoutMS      int
	PublicIPRefreshSeconds int
	PublicIPv4Enabled      bool
	PublicIPv6Enabled      bool
	PublicIPv4Endpoints    []string
	PublicIPv6Endpoints    []string
	Logger                 *slog.Logger
}

type Client struct {
	options   Options
	collector *collector.Collector
	nodeID    string
}

func New(options Options) (*Client, error) {
	nodeID, err := resolveNodeID(options.NodeID, options.StateFile)
	if err != nil {
		return nil, err
	}
	return &Client{
		options:   options,
		collector: collector.New(),
		nodeID:    nodeID,
	}, nil
}

func (c *Client) NodeID() string {
	return c.nodeID
}

func resolveNodeID(configuredNodeID string, stateFile string) (string, error) {
	stateFile = agentstate.NormalizePath(stateFile)

	if nodeID := strings.TrimSpace(configuredNodeID); nodeID != "" {
		if err := agentstate.Save(stateFile, agentstate.State{NodeID: nodeID}); err != nil {
			return "", err
		}
		return nodeID, nil
	}

	state, err := agentstate.Read(stateFile)
	if err == nil {
		if nodeID := strings.TrimSpace(state.NodeID); nodeID != "" {
			return nodeID, nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	nodeID, err := randomNodeID()
	if err != nil {
		return "", err
	}
	if err := agentstate.Save(stateFile, agentstate.State{NodeID: nodeID}); err != nil {
		return "", err
	}
	return nodeID, nil
}

func randomNodeID() (string, error) {
	raw := make([]byte, 8)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

func (c *Client) Run(ctx context.Context) error {
	backoff := time.Second
	for {
		if err := c.connectOnce(ctx); err != nil {
			c.options.Logger.Warn("master connection failed", slog.String("error", err.Error()), slog.Duration("retry_in", backoff))
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}

		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
}

func (c *Client) connectOnce(ctx context.Context) error {
	conn, err := net.DialTimeout("tcp", c.options.MasterAddr, 5*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()

	c.options.Logger.Info("connected to master", slog.String("addr", c.options.MasterAddr))

	secretKey, err := protocol.DecodeSecretKey(c.options.SecretKey)
	if err != nil {
		return err
	}
	agentNonce, err := protocol.RandomBytes(protocol.HandshakeSize)
	if err != nil {
		return err
	}

	hostname, _ := os.Hostname()
	ipAddresses := collectIPAddresses()
	publicIPs := c.detectPublicIPs(ctx)
	c.logPublicIPProbeResult("initial", publicIPs)
	lastPublicIPProbe := time.Now()
	detectedPublicIP := preferredDetectedPublicIP(publicIPs)
	registerTimestamp := time.Now().UnixMilli()
	registerPayload := protocol.RegisterPayload{
		AgentVersion: c.agentVersion(),
		Hostname:     hostname,
		PublicIP:     detectedPublicIP,
		IPAddresses:  ipAddresses,
		PublicIPs:    publicIPs,
		Nonce:        agentNonce,
	}
	registerPayload.Auth = protocol.RegisterAuth(c.options.SecretKey, registerPayload, registerTimestamp)
	if err := protocol.WriteMessage(conn, protocol.Message{
		Type:      protocol.MessageTypeRegister,
		NodeID:    c.nodeID,
		Timestamp: registerTimestamp,
		Payload:   mustPayload(registerPayload),
	}); err != nil {
		return err
	}

	registerAck, err := c.readRegisterAck(conn)
	if err != nil {
		return err
	}
	if len(registerAck.Nonce) != protocol.HandshakeSize {
		return fmt.Errorf("invalid master nonce size: %d", len(registerAck.Nonce))
	}
	keys, err := protocol.DeriveSessionKeys(secretKey, agentNonce, registerAck.Nonce)
	if err != nil {
		return err
	}
	secure := protocol.NewSecureConn(conn, keys.AgentToMaster, keys.MasterToAgent)
	initialConfig, err := c.readInitialConfig(conn, secure)
	if err != nil {
		return err
	}

	heartbeatInterval := 15 * time.Second
	metricsInterval := 15 * time.Second
	heartbeatInterval, metricsInterval = c.applyRuntimeConfig(initialConfig, heartbeatInterval, metricsInterval)
	probeTasks := initialConfig.ProbeTasks
	lastProbeRun := make(map[uint64]time.Time)
	snapshotConfig := initialConfig.Snapshot
	var lastSnapshotRun time.Time
	_ = c.sendLog(secure, "info", fmt.Sprintf("agent connected: node_id=%s heartbeat=%s metrics=%s probe_tasks=%d public_ipv4=%d public_ipv6=%d", c.nodeID, heartbeatInterval, metricsInterval, len(probeTasks), len(publicIPs.IPv4), len(publicIPs.IPv6)))
	heartbeatTicker := time.NewTicker(heartbeatInterval)
	defer heartbeatTicker.Stop()

	metricsTicker := time.NewTicker(metricsInterval)
	defer metricsTicker.Stop()

	probeTicker := time.NewTicker(time.Second)
	defer probeTicker.Stop()

	snapshotTicker := time.NewTicker(time.Second)
	defer snapshotTicker.Stop()

	msgCh := make(chan protocol.Message, 8)
	errCh := make(chan error, 1)
	go readLoop(secure, msgCh, errCh)

	var seq uint64
	if err := c.sendMetrics(ctx, secure, &seq); err != nil {
		return err
	}
	if snapshotConfig.Enabled {
		if err := c.sendSnapshot(ctx, secure, &seq, snapshotConfig); err != nil {
			return err
		}
		lastSnapshotRun = time.Now()
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-errCh:
			return err
		case msg := <-msgCh:
			if msg.Type == protocol.MessageTypeRequestMetrics {
				if err := c.sendMetrics(ctx, secure, &seq); err != nil {
					return err
				}
				continue
			}
			if msg.Type != protocol.MessageTypeConfigUpdate {
				continue
			}
			cfg, err := protocol.PayloadTo[protocol.AgentRuntimeConfig](msg.Payload)
			if err != nil {
				c.options.Logger.Warn("decode runtime config failed", slog.String("error", err.Error()))
				continue
			}
			heartbeatInterval, metricsInterval = c.applyRuntimeConfig(cfg, heartbeatInterval, metricsInterval)
			probeTasks = cfg.ProbeTasks
			snapshotConfig = cfg.Snapshot
			heartbeatTicker.Reset(heartbeatInterval)
			metricsTicker.Reset(metricsInterval)
		case <-heartbeatTicker.C:
			if c.publicIPRefreshDue(lastPublicIPProbe) {
				publicIPs = c.detectPublicIPs(ctx)
				c.logPublicIPProbeResult("refresh", publicIPs)
				lastPublicIPProbe = time.Now()
			}
			seq++
			if err := secure.WriteMessage(protocol.Message{
				Type:      protocol.MessageTypeHeartbeat,
				NodeID:    c.nodeID,
				Seq:       seq,
				Timestamp: time.Now().UnixMilli(),
				Payload:   mustPayload(protocol.HeartbeatPayload{Status: "ok", PublicIPs: publicIPs}),
			}); err != nil {
				return err
			}
		case <-metricsTicker.C:
			if err := c.sendMetrics(ctx, secure, &seq); err != nil {
				return err
			}
		case <-probeTicker.C:
			results := c.dueProbeResults(ctx, probeTasks, lastProbeRun)
			if len(results) == 0 {
				continue
			}
			payload, err := protocol.PayloadFrom(protocol.ProbeResultsPayload{Results: results})
			if err != nil {
				c.options.Logger.Warn("encode probe results payload failed", slog.String("error", err.Error()))
				continue
			}
			seq++
			if err := secure.WriteMessage(protocol.Message{
				Type:      protocol.MessageTypeProbeResults,
				NodeID:    c.nodeID,
				Seq:       seq,
				Timestamp: time.Now().UnixMilli(),
				Payload:   payload,
			}); err != nil {
				return err
			}
		case <-snapshotTicker.C:
			if !snapshotDue(snapshotConfig, lastSnapshotRun) {
				continue
			}
			if err := c.sendSnapshot(ctx, secure, &seq, snapshotConfig); err != nil {
				return err
			}
			lastSnapshotRun = time.Now()
		}
	}
}

func (c *Client) readRegisterAck(conn net.Conn) (protocol.RegisterAckPayload, error) {
	if err := conn.SetReadDeadline(time.Now().Add(8 * time.Second)); err != nil {
		return protocol.RegisterAckPayload{}, err
	}
	defer conn.SetReadDeadline(time.Time{})

	msg, err := protocol.ReadMessage(conn)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return protocol.RegisterAckPayload{}, errors.New("master closed connection during register handshake; check that master and agent secret_key are identical and valid base64")
		}
		return protocol.RegisterAckPayload{}, err
	}
	if msg.Type != protocol.MessageTypeRegisterAck {
		return protocol.RegisterAckPayload{}, errors.New("unexpected register response: " + msg.Type)
	}

	ack, err := protocol.PayloadTo[protocol.RegisterAckPayload](msg.Payload)
	if err != nil {
		return protocol.RegisterAckPayload{}, err
	}
	c.options.Logger.Info("register ack received", slog.String("node_id", c.nodeID))
	return ack, nil
}

func (c *Client) readInitialConfig(conn net.Conn, secure *protocol.SecureConn) (protocol.AgentRuntimeConfig, error) {
	if err := conn.SetReadDeadline(time.Now().Add(8 * time.Second)); err != nil {
		return protocol.AgentRuntimeConfig{}, err
	}
	defer conn.SetReadDeadline(time.Time{})

	msg, err := secure.ReadMessage()
	if err != nil {
		return protocol.AgentRuntimeConfig{}, err
	}
	if msg.Type != protocol.MessageTypeConfigUpdate {
		return protocol.AgentRuntimeConfig{}, errors.New("unexpected initial config message: " + msg.Type)
	}

	cfg, err := protocol.PayloadTo[protocol.AgentRuntimeConfig](msg.Payload)
	if err != nil {
		return protocol.AgentRuntimeConfig{}, err
	}
	c.options.Logger.Info("runtime config received", slog.String("node_id", c.nodeID))
	return cfg, nil
}

func (c *Client) applyRuntimeConfig(cfg protocol.AgentRuntimeConfig, currentHeartbeat time.Duration, currentMetrics time.Duration) (time.Duration, time.Duration) {
	if cfg.NodeID != "" && c.nodeID == "" {
		c.nodeID = cfg.NodeID
	} else if cfg.NodeID != "" && cfg.NodeID != c.nodeID {
		c.options.Logger.Warn("master returned different node id", slog.String("local_node_id", c.nodeID), slog.String("master_node_id", cfg.NodeID))
	}

	heartbeat := currentHeartbeat
	if cfg.HeartbeatIntervalSeconds > 0 {
		heartbeat = time.Duration(cfg.HeartbeatIntervalSeconds) * time.Second
	}

	metrics := currentMetrics
	if cfg.MetricsIntervalSeconds > 0 {
		metrics = time.Duration(cfg.MetricsIntervalSeconds) * time.Second
	}

	c.options.Logger.Info(
		"runtime config applied",
		slog.String("node_id", c.nodeID),
		slog.Duration("heartbeat_interval", heartbeat),
		slog.Duration("metrics_interval", metrics),
	)
	return heartbeat, metrics
}

func (c *Client) sendMetrics(ctx context.Context, conn *protocol.SecureConn, seq *uint64) error {
	*seq = *seq + 1
	metrics, err := c.collector.Collect(ctx)
	if err != nil {
		c.options.Logger.Warn("collect metrics failed", slog.String("error", err.Error()))
		_ = c.sendLog(conn, "warn", "collect metrics failed: "+err.Error())
		return nil
	}

	payload, err := protocol.PayloadFrom(metrics)
	if err != nil {
		c.options.Logger.Warn("encode metrics payload failed", slog.String("error", err.Error()))
		return nil
	}

	return conn.WriteMessage(protocol.Message{
		Type:      protocol.MessageTypeMetrics,
		NodeID:    c.nodeID,
		Seq:       *seq,
		Timestamp: time.Now().UnixMilli(),
		Payload:   payload,
	})
}

func (c *Client) sendSnapshot(ctx context.Context, conn *protocol.SecureConn, seq *uint64, cfg protocol.SnapshotConfig) error {
	if !cfg.Enabled {
		return nil
	}
	snapshot, err := c.collector.CollectSnapshot(ctx, cfg)
	if err != nil {
		c.options.Logger.Warn("collect snapshot failed", slog.String("error", err.Error()))
		_ = c.sendLog(conn, "warn", "collect snapshot failed: "+err.Error())
		return nil
	}

	payload, err := protocol.PayloadFrom(snapshot)
	if err != nil {
		c.options.Logger.Warn("encode snapshot payload failed", slog.String("error", err.Error()))
		return nil
	}

	*seq = *seq + 1
	return conn.WriteMessage(protocol.Message{
		Type:      protocol.MessageTypeSnapshotResults,
		NodeID:    c.nodeID,
		Seq:       *seq,
		Timestamp: time.Now().UnixMilli(),
		Payload:   payload,
	})
}

func snapshotDue(cfg protocol.SnapshotConfig, lastRun time.Time) bool {
	if !cfg.Enabled {
		return false
	}
	interval := time.Duration(cfg.IntervalSeconds) * time.Second
	if interval < 15*time.Second || interval > 3600*time.Second {
		interval = 60 * time.Second
	}
	return lastRun.IsZero() || time.Since(lastRun) >= interval
}

func (c *Client) dueProbeResults(ctx context.Context, tasks []protocol.ProbeTaskConfig, lastRun map[uint64]time.Time) []protocol.ProbeResultItem {
	now := time.Now()
	results := make([]protocol.ProbeResultItem, 0, len(tasks))
	for _, task := range tasks {
		if !task.Enabled || task.ID == 0 {
			continue
		}
		interval := time.Duration(task.IntervalSeconds) * time.Second
		if interval <= 0 {
			interval = 60 * time.Second
		}
		if last := lastRun[task.ID]; !last.IsZero() && now.Sub(last) < interval {
			continue
		}
		lastRun[task.ID] = now
		results = append(results, runProbeTask(ctx, task))
	}
	return results
}

func runProbeTask(ctx context.Context, task protocol.ProbeTaskConfig) protocol.ProbeResultItem {
	timeout := time.Duration(task.TimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 3 * time.Second
	}

	latency, packetLoss, err := probeLatency(ctx, task, timeout)
	status := "success"
	errorMessage := ""
	latencyValue := &latency
	packetLossValue := &packetLoss
	if err != nil {
		status = "failed"
		errorMessage = err.Error()
		if latency <= 0 {
			latencyValue = nil
		}
	}

	return protocol.ProbeResultItem{
		TaskID:       task.ID,
		Type:         strings.ToLower(strings.TrimSpace(task.Type)),
		IPVersion:    normalizeProbeIPVersion(task.IPVersion),
		Target:       strings.TrimSpace(task.Target),
		Status:       status,
		LatencyMS:    latencyValue,
		PacketLoss:   packetLossValue,
		ErrorMessage: errorMessage,
		CreatedAt:    uint64(time.Now().UnixMilli()),
	}
}

func probeLatency(ctx context.Context, task protocol.ProbeTaskConfig, timeout time.Duration) (float64, float64, error) {
	ipVersion := normalizeProbeIPVersion(task.IPVersion)
	switch strings.ToLower(strings.TrimSpace(task.Type)) {
	case "tcp_ping":
		latency, err := tcpPing(ctx, strings.TrimSpace(task.Target), timeout, ipVersion)
		if err != nil {
			return 0, 100, err
		}
		return latency, 0, nil
	case "icmp":
		latency, err := icmpPing(ctx, strings.TrimSpace(task.Target), timeout, ipVersion)
		if err != nil {
			return 0, 100, err
		}
		return latency, 0, nil
	default:
		return 0, 100, errors.New("unsupported probe type: " + task.Type)
	}
}

func tcpPing(ctx context.Context, target string, timeout time.Duration, ipVersion string) (float64, error) {
	target, fallbackHost, usedDefaultPort := normalizeTCPTarget(target)
	if target == "" {
		return 0, errors.New("empty tcp ping target")
	}
	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	startedAt := time.Now()
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(probeCtx, tcpNetworkForIPVersion(ipVersion), target)
	if err != nil {
		if isConnectionRefused(err) {
			return float64(time.Since(startedAt).Microseconds()) / 1000, nil
		}
		if usedDefaultPort {
			if latency, pingErr := icmpPing(ctx, fallbackHost, timeout, ipVersion); pingErr == nil {
				return latency, nil
			}
		}
		return 0, err
	}
	_ = conn.Close()
	return float64(time.Since(startedAt).Microseconds()) / 1000, nil
}

func tcpNetworkForIPVersion(ipVersion string) string {
	switch normalizeProbeIPVersion(ipVersion) {
	case "ipv4":
		return "tcp4"
	case "ipv6":
		return "tcp6"
	default:
		return "tcp"
	}
}

func normalizeProbeIPVersion(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "", "auto":
		return "auto"
	case "4", "v4", "ip4", "ipv4":
		return "ipv4"
	case "6", "v6", "ip6", "ipv6":
		return "ipv6"
	default:
		return normalized
	}
}

func normalizeTCPTarget(target string) (string, string, bool) {
	target = strings.TrimSpace(target)
	if target == "" {
		return "", "", false
	}
	host, _, err := net.SplitHostPort(target)
	if err == nil {
		return target, host, false
	}
	if strings.HasPrefix(target, "[") && strings.HasSuffix(target, "]") {
		target = strings.TrimPrefix(strings.TrimSuffix(target, "]"), "[")
	}
	return net.JoinHostPort(target, "80"), target, true
}

func isConnectionRefused(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "connection refused")
}

func icmpPing(ctx context.Context, target string, timeout time.Duration, ipVersion string) (float64, error) {
	ipVersion = normalizeProbeIPVersion(ipVersion)
	latency, nativeErr := nativeICMPPing(ctx, target, timeout, ipVersion)
	if nativeErr == nil {
		return latency, nil
	}
	latency, fallbackErr := systemICMPPing(ctx, target, timeout, ipVersion)
	if fallbackErr == nil {
		return latency, nil
	}
	return 0, fmt.Errorf("native icmp failed: %v; system ping fallback failed: %v", nativeErr, fallbackErr)
}

func nativeICMPPing(ctx context.Context, target string, timeout time.Duration, ipVersion string) (float64, error) {
	host := normalizeICMPTarget(target)
	if host == "" {
		return 0, errors.New("empty icmp target")
	}

	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ips, err := resolveICMPIPs(probeCtx, host, ipVersion)
	if err != nil {
		return 0, err
	}
	attempts := make([]string, 0, len(ips))
	for _, ip := range ips {
		latency, err := nativeICMPPingIP(probeCtx, ip, timeout)
		if err == nil {
			return latency, nil
		}
		attempts = append(attempts, ip.String()+": "+err.Error())
	}
	if len(attempts) == 0 {
		return 0, errors.New("no icmp address resolved")
	}
	return 0, errors.New(strings.Join(attempts, "; "))
}

func normalizeICMPTarget(target string) string {
	host := strings.TrimSpace(target)
	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		host = parsedHost
	}
	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		host = strings.TrimPrefix(strings.TrimSuffix(host, "]"), "[")
	}
	return strings.TrimSpace(host)
}

func resolveICMPIPs(ctx context.Context, host string, ipVersion string) ([]net.IP, error) {
	wantIPv4 := normalizeProbeIPVersion(ipVersion) == "ipv4"
	wantIPv6 := normalizeProbeIPVersion(ipVersion) == "ipv6"
	if ip := net.ParseIP(host); ip != nil {
		if wantIPv4 && ip.To4() == nil {
			return nil, errors.New("target is not an IPv4 address")
		}
		if wantIPv6 && ip.To4() != nil {
			return nil, errors.New("target is not an IPv6 address")
		}
		return []net.IP{ip}, nil
	}

	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	ipv4Addrs := make([]net.IP, 0, len(addrs))
	ipv6Addrs := make([]net.IP, 0, len(addrs))
	for _, addr := range addrs {
		ip := addr.IP
		if ip == nil {
			continue
		}
		if ip.To4() != nil {
			if !wantIPv6 {
				ipv4Addrs = append(ipv4Addrs, ip)
			}
			continue
		}
		if !wantIPv4 {
			ipv6Addrs = append(ipv6Addrs, ip)
		}
	}
	if wantIPv4 {
		if len(ipv4Addrs) == 0 {
			return nil, errors.New("no IPv4 address resolved")
		}
		return ipv4Addrs, nil
	}
	if wantIPv6 {
		if len(ipv6Addrs) == 0 {
			return nil, errors.New("no IPv6 address resolved")
		}
		return ipv6Addrs, nil
	}
	return append(ipv4Addrs, ipv6Addrs...), nil
}

func nativeICMPPingIP(ctx context.Context, ip net.IP, timeout time.Duration) (float64, error) {
	isIPv4 := ip.To4() != nil
	conn, network, err := listenICMPPacket(isIPv4)
	if err != nil {
		return 0, err
	}
	defer conn.Close()

	deadline := time.Now().Add(timeout)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	if err := conn.SetDeadline(deadline); err != nil {
		return 0, err
	}

	id := int(time.Now().UnixNano()) & 0xffff
	seq := int(time.Now().UnixMilli()) & 0xffff
	data := []byte("rivo-icmp")
	var messageType icmp.Type = ipv4.ICMPTypeEcho
	var replyType icmp.Type = ipv4.ICMPTypeEchoReply
	protocol := 1
	if !isIPv4 {
		messageType = ipv6.ICMPTypeEchoRequest
		protocol = 58
		replyType = ipv6.ICMPTypeEchoReply
	}

	message := icmp.Message{
		Type: messageType,
		Code: 0,
		Body: &icmp.Echo{ID: id, Seq: seq, Data: data},
	}
	packet, err := message.Marshal(nil)
	if err != nil {
		return 0, err
	}

	startedAt := time.Now()
	if _, err := conn.WriteTo(packet, icmpDestination(ip, network)); err != nil {
		return 0, err
	}

	buffer := make([]byte, 1500)
	for {
		n, _, err := conn.ReadFrom(buffer)
		if err != nil {
			return 0, err
		}
		received, err := icmp.ParseMessage(protocol, buffer[:n])
		if err != nil || received.Type != replyType {
			continue
		}
		echo, ok := received.Body.(*icmp.Echo)
		if !ok {
			continue
		}
		if echo.Seq == seq && (echo.ID == id || bytes.Equal(echo.Data, data)) {
			return float64(time.Since(startedAt).Microseconds()) / 1000, nil
		}
	}
}

func listenICMPPacket(isIPv4 bool) (*icmp.PacketConn, string, error) {
	if isIPv4 {
		if conn, err := icmp.ListenPacket("udp4", "0.0.0.0"); err == nil {
			return conn, "udp4", nil
		}
		conn, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
		return conn, "ip4:icmp", err
	}
	if conn, err := icmp.ListenPacket("udp6", "::"); err == nil {
		return conn, "udp6", nil
	}
	conn, err := icmp.ListenPacket("ip6:ipv6-icmp", "::")
	return conn, "ip6:ipv6-icmp", err
}

func icmpDestination(ip net.IP, network string) net.Addr {
	if strings.HasPrefix(network, "udp") {
		return &net.UDPAddr{IP: ip}
	}
	return &net.IPAddr{IP: ip}
}

func systemICMPPing(ctx context.Context, target string, timeout time.Duration, ipVersion string) (float64, error) {
	host := normalizeICMPTarget(target)
	if host == "" {
		return 0, errors.New("empty icmp target")
	}

	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	waitSeconds := int(timeout.Round(time.Second) / time.Second)
	if waitSeconds <= 0 {
		waitSeconds = 1
	}
	args := []string{"-c", "1", "-W", strconv.Itoa(waitSeconds)}
	if normalizeProbeIPVersion(ipVersion) == "ipv4" {
		args = append([]string{"-4"}, args...)
	}
	if normalizeProbeIPVersion(ipVersion) == "ipv6" {
		args = append([]string{"-6"}, args...)
	}
	args = append(args, host)
	cmd := exec.CommandContext(probeCtx, "ping", args...)
	output, err := cmd.CombinedOutput()
	latency, parseErr := parsePingLatency(string(output))
	if err != nil {
		if parseErr == nil {
			return latency, nil
		}
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		return 0, errors.New(message)
	}
	if parseErr != nil {
		return 0, parseErr
	}
	return latency, nil
}

var pingTimePattern = regexp.MustCompile(`time[=<]([0-9.]+)\s*ms`)

func parsePingLatency(output string) (float64, error) {
	match := pingTimePattern.FindStringSubmatch(output)
	if len(match) < 2 {
		return 0, errors.New("ping latency not found")
	}
	return strconv.ParseFloat(match[1], 64)
}

func (c *Client) sendLog(conn *protocol.SecureConn, level string, message string) error {
	payload, err := protocol.PayloadFrom(protocol.AgentLogPayload{Level: level, Message: message})
	if err != nil {
		return err
	}
	return conn.WriteMessage(protocol.Message{
		Type:      protocol.MessageTypeLog,
		NodeID:    c.nodeID,
		Timestamp: time.Now().UnixMilli(),
		Payload:   payload,
	})
}

func (c *Client) agentVersion() string {
	if c.options.AgentVersion == "" {
		return "dev"
	}
	return c.options.AgentVersion
}

func readLoop(conn *protocol.SecureConn, msgCh chan<- protocol.Message, errCh chan<- error) {
	for {
		msg, err := conn.ReadMessage()
		if err != nil {
			errCh <- err
			return
		}
		msgCh <- msg
	}
}

func localIPFromConn(conn net.Conn) string {
	addr, ok := conn.LocalAddr().(*net.TCPAddr)
	if !ok || addr.IP == nil {
		return ""
	}
	return normalizeUsableIP(addr.IP)
}

func collectIPAddresses() protocol.IPAddresses {
	var result protocol.IPAddresses
	seen := make(map[string]struct{})
	interfaces, err := net.Interfaces()
	if err != nil {
		return result
	}

	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ip := ipFromAddr(addr)
			normalized := normalizeUsableIP(ip)
			if normalized == "" {
				continue
			}
			if _, exists := seen[normalized]; exists {
				continue
			}
			seen[normalized] = struct{}{}
			if net.ParseIP(normalized).To4() != nil {
				result.IPv4 = append(result.IPv4, normalized)
				continue
			}
			result.IPv6 = append(result.IPv6, normalized)
		}
	}
	return result
}

func ipFromAddr(addr net.Addr) net.IP {
	switch value := addr.(type) {
	case *net.IPNet:
		return value.IP
	case *net.IPAddr:
		return value.IP
	default:
		return nil
	}
}

func normalizeUsableIP(ip net.IP) string {
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

func preferredLocalIP(addresses protocol.IPAddresses) string {
	if ip := firstPublicIP(addresses.IPv4); ip != "" {
		return ip
	}
	if ip := firstPublicIP(addresses.IPv6); ip != "" {
		return ip
	}
	if len(addresses.IPv4) > 0 {
		return addresses.IPv4[0]
	}
	if len(addresses.IPv6) > 0 {
		return addresses.IPv6[0]
	}
	return ""
}

func preferredDetectedPublicIP(addresses protocol.PublicIPs) string {
	if len(addresses.IPv4) > 0 {
		return strings.TrimSpace(addresses.IPv4[0].IP)
	}
	if len(addresses.IPv6) > 0 {
		return strings.TrimSpace(addresses.IPv6[0].IP)
	}
	return ""
}

func (c *Client) publicIPRefreshDue(lastProbe time.Time) bool {
	if !c.options.PublicIPEnabled {
		return false
	}
	interval := time.Duration(c.options.PublicIPRefreshSeconds) * time.Second
	if interval <= 0 {
		interval = 10 * time.Minute
	}
	return time.Since(lastProbe) >= interval
}

func (c *Client) logPublicIPProbeResult(stage string, addresses protocol.PublicIPs) {
	if c.options.Logger == nil || !c.options.PublicIPEnabled {
		return
	}
	c.options.Logger.Info(
		"public ip probe completed",
		slog.String("stage", stage),
		slog.Int("ipv4_count", len(addresses.IPv4)),
		slog.Int("ipv6_count", len(addresses.IPv6)),
	)
}

func firstPublicIP(values []string) string {
	for _, value := range values {
		ip := net.ParseIP(value)
		if ip == nil || !isPublicIP(ip) {
			continue
		}
		return value
	}
	return ""
}

func mustPayload(value any) map[string]any {
	payload, err := protocol.PayloadFrom(value)
	if err != nil {
		return map[string]any{}
	}
	return payload
}
