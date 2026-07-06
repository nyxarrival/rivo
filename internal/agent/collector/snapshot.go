package collector

import (
	"bufio"
	"context"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"rivo/internal/protocol"
)

const linuxClockTicks = 100

type processSocketOwner struct {
	pid  int
	name string
}

func (c *Collector) CollectSnapshot(ctx context.Context, cfg protocol.SnapshotConfig) (protocol.SnapshotPayload, error) {
	select {
	case <-ctx.Done():
		return protocol.SnapshotPayload{}, ctx.Err()
	default:
	}
	if !cfg.Enabled {
		return protocol.SnapshotPayload{Config: cfg, TCPStateCounts: map[string]uint32{}, CreatedAt: uint64(time.Now().UnixMilli())}, nil
	}

	cfg = normalizeSnapshotConfig(cfg)
	payload := protocol.SnapshotPayload{
		Config:         cfg,
		TCPStateCounts: map[string]uint32{},
		CreatedAt:      uint64(time.Now().UnixMilli()),
	}

	if cfg.CollectProcesses {
		processes, processCount, threadCount, nextCPU, err := c.collectProcesses(cfg)
		if err != nil {
			return protocol.SnapshotPayload{}, err
		}
		payload.TopProcesses = processes
		payload.ProcessCount = processCount
		payload.ThreadCount = threadCount
		c.prevProcCPU = nextCPU
		c.prevProcTime = time.Now()
	}

	if cfg.CollectConnections {
		connections, stateCounts, connectionCount, listenCount, err := collectConnections(cfg)
		if err != nil {
			return protocol.SnapshotPayload{}, err
		}
		payload.Connections = connections
		payload.TCPStateCounts = stateCounts
		payload.ConnectionCount = connectionCount
		payload.ListenCount = listenCount
	}

	return payload, nil
}

func normalizeSnapshotConfig(cfg protocol.SnapshotConfig) protocol.SnapshotConfig {
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

func (c *Collector) collectProcesses(cfg protocol.SnapshotConfig) ([]protocol.SnapshotProcess, uint32, uint32, map[int]uint64, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, 0, 0, nil, fmt.Errorf("read proc: %w", err)
	}

	users := readPasswdUsers()
	now := time.Now()
	elapsed := now.Sub(c.prevProcTime).Seconds()
	nextCPU := make(map[int]uint64)
	processes := make([]protocol.SnapshotProcess, 0, len(entries))
	var processCount uint32
	var threadCount uint32

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		proc, totalTicks, err := readProcess(pid, users, cfg.MaskSensitive)
		if err != nil {
			continue
		}
		processCount++
		threadCount += proc.ThreadCount
		nextCPU[pid] = totalTicks
		if previousTicks, ok := c.prevProcCPU[pid]; ok && totalTicks >= previousTicks && elapsed > 0 {
			proc.CPUPercent = float64(totalTicks-previousTicks) / linuxClockTicks / elapsed * 100
		}
		processes = append(processes, proc)
	}

	sort.Slice(processes, func(i, j int) bool {
		if processes[i].CPUPercent == processes[j].CPUPercent {
			return processes[i].MemoryBytes > processes[j].MemoryBytes
		}
		return processes[i].CPUPercent > processes[j].CPUPercent
	})

	limit := int(cfg.ProcessLimit)
	if len(processes) > limit {
		processes = processes[:limit]
	}
	return processes, processCount, threadCount, nextCPU, nil
}

func readProcess(pid int, users map[string]string, maskSensitive bool) (protocol.SnapshotProcess, uint64, error) {
	statPath := filepath.Join("/proc", strconv.Itoa(pid), "stat")
	raw, err := os.ReadFile(statPath)
	if err != nil {
		return protocol.SnapshotProcess{}, 0, err
	}
	stat := string(raw)
	open := strings.Index(stat, "(")
	close := strings.LastIndex(stat, ")")
	if open < 0 || close <= open {
		return protocol.SnapshotProcess{}, 0, fmt.Errorf("invalid process stat")
	}
	name := stat[open+1 : close]
	fields := strings.Fields(strings.TrimSpace(stat[close+1:]))
	if len(fields) < 22 {
		return protocol.SnapshotProcess{}, 0, fmt.Errorf("short process stat")
	}

	utime := parseUintField(fields[11])
	stime := parseUintField(fields[12])
	threads := uint32(parseUintField(fields[17]))
	rssPages := parseUintField(fields[21])
	uid := readProcessUID(pid)
	user := uid
	if value := users[uid]; value != "" {
		user = value
	}
	command := readProcessCommand(pid, name)
	if maskSensitive {
		command = name
	}

	return protocol.SnapshotProcess{
		PID:         pid,
		Name:        name,
		User:        user,
		State:       fields[0],
		MemoryBytes: rssPages * uint64(os.Getpagesize()),
		ThreadCount: threads,
		Command:     command,
	}, utime + stime, nil
}

func parseUintField(value string) uint64 {
	parsed, _ := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
	return parsed
}

func readProcessCommand(pid int, fallback string) string {
	raw, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "cmdline"))
	if err != nil || len(raw) == 0 {
		return fallback
	}
	command := strings.TrimSpace(strings.ReplaceAll(string(raw), "\x00", " "))
	if command == "" {
		return fallback
	}
	if len(command) > 180 {
		return command[:180] + "..."
	}
	return command
}

func readProcessUID(pid int) string {
	raw, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "status"))
	if err != nil {
		return ""
	}
	scanner := bufio.NewScanner(strings.NewReader(string(raw)))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 2 && fields[0] == "Uid:" {
			return fields[1]
		}
	}
	return ""
}

func readPasswdUsers() map[string]string {
	users := map[string]string{}
	raw, err := os.ReadFile("/etc/passwd")
	if err != nil {
		return users
	}
	scanner := bufio.NewScanner(strings.NewReader(string(raw)))
	for scanner.Scan() {
		parts := strings.Split(scanner.Text(), ":")
		if len(parts) >= 3 {
			users[parts[2]] = parts[0]
		}
	}
	return users
}

func collectConnections(cfg protocol.SnapshotConfig) ([]protocol.SnapshotConnection, map[string]uint32, uint32, uint32, error) {
	owners := socketOwners()
	stateCounts := map[string]uint32{}
	connections := []protocol.SnapshotConnection{}
	var total uint32
	var listen uint32

	files := []struct {
		path     string
		protocol string
		ipv6     bool
	}{
		{"/proc/net/tcp", "tcp", false},
		{"/proc/net/tcp6", "tcp6", true},
		{"/proc/net/udp", "udp", false},
		{"/proc/net/udp6", "udp6", true},
	}
	for _, file := range files {
		items, counts, itemTotal, itemListen := readConnectionFile(file.path, file.protocol, file.ipv6, owners, cfg.MaskSensitive)
		total += itemTotal
		listen += itemListen
		for state, count := range counts {
			stateCounts[state] += count
		}
		connections = append(connections, items...)
	}

	sort.Slice(connections, func(i, j int) bool {
		if connections[i].State == connections[j].State {
			return connections[i].LocalPort < connections[j].LocalPort
		}
		return connections[i].State < connections[j].State
	})
	limit := int(cfg.ConnectionLimit)
	if len(connections) > limit {
		connections = connections[:limit]
	}
	return connections, stateCounts, total, listen, nil
}

func socketOwners() map[string]processSocketOwner {
	owners := map[string]processSocketOwner{}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return owners
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		name := readProcessName(pid)
		fdDir := filepath.Join("/proc", entry.Name(), "fd")
		fds, err := os.ReadDir(fdDir)
		if err != nil {
			continue
		}
		for _, fd := range fds {
			target, err := os.Readlink(filepath.Join(fdDir, fd.Name()))
			if err != nil {
				continue
			}
			if strings.HasPrefix(target, "socket:[") && strings.HasSuffix(target, "]") {
				inode := strings.TrimSuffix(strings.TrimPrefix(target, "socket:["), "]")
				owners[inode] = processSocketOwner{pid: pid, name: name}
			}
		}
	}
	return owners
}

func readProcessName(pid int) string {
	raw, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "comm"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
}

func readConnectionFile(path string, protoName string, ipv6 bool, owners map[string]processSocketOwner, maskSensitive bool) ([]protocol.SnapshotConnection, map[string]uint32, uint32, uint32) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, map[string]uint32{}, 0, 0
	}
	scanner := bufio.NewScanner(strings.NewReader(string(raw)))
	if !scanner.Scan() {
		return nil, map[string]uint32{}, 0, 0
	}

	connections := []protocol.SnapshotConnection{}
	counts := map[string]uint32{}
	var total uint32
	var listen uint32
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 10 {
			continue
		}
		state := connectionState(fields[3], protoName)
		localAddr, localPort := parseEndpoint(fields[1], ipv6)
		remoteAddr, remotePort := parseEndpoint(fields[2], ipv6)
		if maskSensitive {
			localAddr = maskAddress(localAddr)
			remoteAddr = maskAddress(remoteAddr)
		}
		owner := owners[fields[9]]
		if maskSensitive {
			owner.name = ""
		}
		total++
		counts[state]++
		if state == "LISTEN" {
			listen++
		}
		connections = append(connections, protocol.SnapshotConnection{
			Protocol:    protoName,
			LocalAddr:   localAddr,
			LocalPort:   localPort,
			RemoteAddr:  remoteAddr,
			RemotePort:  remotePort,
			State:       state,
			PID:         owner.pid,
			ProcessName: owner.name,
		})
	}
	return connections, counts, total, listen
}

func parseEndpoint(value string, ipv6 bool) (string, uint16) {
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return value, 0
	}
	port64, _ := strconv.ParseUint(parts[1], 16, 16)
	if ipv6 {
		return parseIPv6Hex(parts[0]), uint16(port64)
	}
	return parseIPv4Hex(parts[0]), uint16(port64)
}

func parseIPv4Hex(value string) string {
	raw, err := hex.DecodeString(value)
	if err != nil || len(raw) != 4 {
		return value
	}
	return net.IPv4(raw[3], raw[2], raw[1], raw[0]).String()
}

func parseIPv6Hex(value string) string {
	raw, err := hex.DecodeString(value)
	if err != nil || len(raw) != 16 {
		return value
	}
	for i := 0; i < len(raw); i += 4 {
		raw[i], raw[i+3] = raw[i+3], raw[i]
		raw[i+1], raw[i+2] = raw[i+2], raw[i+1]
	}
	return net.IP(raw).String()
}

func connectionState(value string, protocolName string) string {
	if strings.HasPrefix(protocolName, "udp") {
		return "UDP"
	}
	states := map[string]string{
		"01": "ESTABLISHED",
		"02": "SYN_SENT",
		"03": "SYN_RECV",
		"04": "FIN_WAIT1",
		"05": "FIN_WAIT2",
		"06": "TIME_WAIT",
		"07": "CLOSE",
		"08": "CLOSE_WAIT",
		"09": "LAST_ACK",
		"0A": "LISTEN",
		"0B": "CLOSING",
	}
	if state, ok := states[strings.ToUpper(value)]; ok {
		return state
	}
	return strings.ToUpper(value)
}

func maskAddress(value string) string {
	if value == "" || value == "0.0.0.0" || value == "::" {
		return value
	}
	if ip := net.ParseIP(value); ip != nil {
		if ipv4 := ip.To4(); ipv4 != nil {
			return fmt.Sprintf("%d.x.x.x", ipv4[0])
		}
		return "xxxx:xxxx:xxxx:xxxx"
	}
	return value
}
