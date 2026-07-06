package collector

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"rivo/internal/protocol"
)

type Collector struct {
	prevCPU      cpuSample
	prevNet      netSample
	prevTime     time.Time
	prevProcCPU  map[int]uint64
	prevProcTime time.Time
}

type cpuSample struct {
	total uint64
	idle  uint64
	valid bool
}

type netSample struct {
	rxBytes uint64
	txBytes uint64
	valid   bool
}

func New() *Collector {
	return &Collector{}
}

func (c *Collector) Collect(ctx context.Context) (protocol.MetricsPayload, error) {
	select {
	case <-ctx.Done():
		return protocol.MetricsPayload{}, ctx.Err()
	default:
	}

	now := time.Now()

	cpuUsage, cpuSample, err := readCPUUsage(c.prevCPU)
	if err != nil {
		return protocol.MetricsPayload{}, err
	}

	load1, load5, load15, err := readLoad()
	if err != nil {
		return protocol.MetricsPayload{}, err
	}

	memTotal, memUsed, memUsedPercent, err := readMemory()
	if err != nil {
		return protocol.MetricsPayload{}, err
	}
	swapTotal, swapUsed, swapUsedPercent, err := readSwap()
	if err != nil {
		return protocol.MetricsPayload{}, err
	}

	diskTotal, diskUsed, diskUsedPercent, err := readDisk("/")
	if err != nil {
		return protocol.MetricsPayload{}, err
	}

	netRxBps, netTxBps, netSample, err := c.readNetworkBPS(now)
	if err != nil {
		return protocol.MetricsPayload{}, err
	}

	uptimeSeconds, err := readUptime()
	if err != nil {
		return protocol.MetricsPayload{}, err
	}

	c.prevCPU = cpuSample
	c.prevNet = netSample
	c.prevTime = now

	return protocol.MetricsPayload{
		CPUUsage:        cpuUsage,
		CPUCores:        uint32(runtime.NumCPU()),
		Arch:            readArchitecture(),
		Virtualization:  readVirtualization(),
		GPU:             readGPU(),
		OSName:          readOSName(),
		Load1:           load1,
		Load5:           load5,
		Load15:          load15,
		MemTotal:        memTotal,
		MemUsed:         memUsed,
		MemUsedPercent:  memUsedPercent,
		SwapTotal:       swapTotal,
		SwapUsed:        swapUsed,
		SwapUsedPercent: swapUsedPercent,
		DiskTotal:       diskTotal,
		DiskUsed:        diskUsed,
		DiskUsedPercent: diskUsedPercent,
		NetRxBps:        netRxBps,
		NetTxBps:        netTxBps,
		NetRxBytesTotal: netSample.rxBytes,
		NetTxBytesTotal: netSample.txBytes,
		UptimeSeconds:   uptimeSeconds,
	}, nil
}

func readCPUUsage(prev cpuSample) (float64, cpuSample, error) {
	raw, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, cpuSample{}, fmt.Errorf("read cpu stat: %w", err)
	}

	scanner := bufio.NewScanner(strings.NewReader(string(raw)))
	if !scanner.Scan() {
		return 0, cpuSample{}, fmt.Errorf("read cpu stat: missing cpu line")
	}

	fields := strings.Fields(scanner.Text())
	if len(fields) < 8 || fields[0] != "cpu" {
		return 0, cpuSample{}, fmt.Errorf("read cpu stat: invalid cpu line")
	}

	values := make([]uint64, 0, len(fields)-1)
	for _, field := range fields[1:] {
		value, err := strconv.ParseUint(field, 10, 64)
		if err != nil {
			return 0, cpuSample{}, fmt.Errorf("parse cpu stat: %w", err)
		}
		values = append(values, value)
	}

	var total uint64
	for _, value := range values {
		total += value
	}

	idle := values[3]
	if len(values) > 4 {
		idle += values[4]
	}

	current := cpuSample{total: total, idle: idle, valid: true}
	if !prev.valid || current.total <= prev.total {
		return 0, current, nil
	}

	totalDelta := current.total - prev.total
	idleDelta := current.idle - prev.idle
	if totalDelta == 0 || idleDelta > totalDelta {
		return 0, current, nil
	}

	return float64(totalDelta-idleDelta) * 100 / float64(totalDelta), current, nil
}

func readOSName() string {
	raw, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return runtime.GOOS
	}

	scanner := bufio.NewScanner(strings.NewReader(string(raw)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		value, ok := strings.CutPrefix(line, "PRETTY_NAME=")
		if !ok {
			continue
		}
		return strings.Trim(strings.TrimSpace(value), `"`)
	}
	return runtime.GOOS
}

func readArchitecture() string {
	switch runtime.GOARCH {
	case "amd64", "386":
		return "amd"
	case "arm64", "arm":
		return "arm"
	default:
		return runtime.GOARCH
	}
}

func readVirtualization() string {
	if out, err := exec.Command("systemd-detect-virt").Output(); err == nil {
		value := strings.TrimSpace(string(out))
		if value != "" && value != "none" {
			return value
		}
	}

	for _, path := range []string{
		"/sys/class/dmi/id/product_name",
		"/sys/class/dmi/id/sys_vendor",
		"/sys/class/dmi/id/board_vendor",
	} {
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if value := virtualizationFromText(string(raw)); value != "" {
			return value
		}
	}
	return "unknown"
}

func virtualizationFromText(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch {
	case strings.Contains(value, "kvm"):
		return "kvm"
	case strings.Contains(value, "qemu"):
		return "qemu"
	case strings.Contains(value, "vmware"):
		return "vmware"
	case strings.Contains(value, "virtualbox"):
		return "virtualbox"
	case strings.Contains(value, "hyper-v") || strings.Contains(value, "microsoft corporation"):
		return "hyper-v"
	case strings.Contains(value, "xen"):
		return "xen"
	case strings.Contains(value, "openvz"):
		return "openvz"
	case strings.Contains(value, "parallels"):
		return "parallels"
	default:
		return ""
	}
}

func readGPU() string {
	out, err := exec.Command("lspci").Output()
	if err != nil {
		return "none"
	}

	var devices []string
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		lower := strings.ToLower(line)
		if !strings.Contains(lower, "vga compatible controller") &&
			!strings.Contains(lower, "3d controller") &&
			!strings.Contains(lower, "display controller") {
			continue
		}
		if index := strings.Index(line, ": "); index >= 0 && index+2 < len(line) {
			line = line[index+2:]
		}
		devices = append(devices, line)
		if len(devices) >= 2 {
			break
		}
	}
	if len(devices) == 0 {
		return "none"
	}
	return strings.Join(devices, "; ")
}

func readLoad() (float64, float64, float64, error) {
	raw, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, 0, 0, fmt.Errorf("read loadavg: %w", err)
	}

	fields := strings.Fields(string(raw))
	if len(fields) < 3 {
		return 0, 0, 0, fmt.Errorf("read loadavg: invalid content")
	}

	load1, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, 0, 0, err
	}
	load5, err := strconv.ParseFloat(fields[1], 64)
	if err != nil {
		return 0, 0, 0, err
	}
	load15, err := strconv.ParseFloat(fields[2], 64)
	if err != nil {
		return 0, 0, 0, err
	}

	return load1, load5, load15, nil
}

func readMemory() (uint64, uint64, float64, error) {
	values, err := readMemInfo()
	if err != nil {
		return 0, 0, 0, fmt.Errorf("read meminfo: %w", err)
	}

	total := values["MemTotal"]
	available := values["MemAvailable"]
	if total == 0 || available > total {
		return 0, 0, 0, fmt.Errorf("read meminfo: invalid total or available memory")
	}

	used := total - available
	return total, used, float64(used) * 100 / float64(total), nil
}

func readSwap() (uint64, uint64, float64, error) {
	values, err := readMemInfo()
	if err != nil {
		return 0, 0, 0, fmt.Errorf("read meminfo: %w", err)
	}

	total := values["SwapTotal"]
	free := values["SwapFree"]
	if total == 0 {
		return 0, 0, 0, nil
	}
	if free > total {
		return 0, 0, 0, fmt.Errorf("read meminfo: invalid swap total or free")
	}

	used := total - free
	return total, used, float64(used) * 100 / float64(total), nil
}

func readMemInfo() (map[string]uint64, error) {
	raw, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return nil, err
	}

	values := map[string]uint64{}
	scanner := bufio.NewScanner(strings.NewReader(string(raw)))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}

		key := strings.TrimSuffix(fields[0], ":")
		value, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			continue
		}
		values[key] = value * 1024
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return values, nil
}

func readDisk(path string) (uint64, uint64, float64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, 0, 0, fmt.Errorf("read disk stat: %w", err)
	}

	total := uint64(stat.Blocks) * uint64(stat.Bsize)
	free := uint64(stat.Bavail) * uint64(stat.Bsize)
	if total == 0 || free > total {
		return 0, 0, 0, fmt.Errorf("read disk stat: invalid total or free disk")
	}

	used := total - free
	return total, used, float64(used) * 100 / float64(total), nil
}

func (c *Collector) readNetworkBPS(now time.Time) (uint64, uint64, netSample, error) {
	current, err := readNetworkSample()
	if err != nil {
		return 0, 0, netSample{}, err
	}

	if !c.prevNet.valid || c.prevTime.IsZero() {
		return 0, 0, current, nil
	}

	elapsed := now.Sub(c.prevTime).Seconds()
	if elapsed <= 0 || current.rxBytes < c.prevNet.rxBytes || current.txBytes < c.prevNet.txBytes {
		return 0, 0, current, nil
	}

	rxBps := uint64(float64(current.rxBytes-c.prevNet.rxBytes) * 8 / elapsed)
	txBps := uint64(float64(current.txBytes-c.prevNet.txBytes) * 8 / elapsed)

	return rxBps, txBps, current, nil
}

func readNetworkSample() (netSample, error) {
	raw, err := os.ReadFile("/proc/net/dev")
	if err != nil {
		return netSample{}, fmt.Errorf("read net dev: %w", err)
	}

	var rxBytes uint64
	var txBytes uint64

	scanner := bufio.NewScanner(strings.NewReader(string(raw)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.Contains(line, ":") {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		iface := strings.TrimSpace(parts[0])
		if iface == "lo" {
			continue
		}

		fields := strings.Fields(parts[1])
		if len(fields) < 16 {
			continue
		}

		rx, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			continue
		}
		tx, err := strconv.ParseUint(fields[8], 10, 64)
		if err != nil {
			continue
		}

		rxBytes += rx
		txBytes += tx
	}
	if err := scanner.Err(); err != nil {
		return netSample{}, err
	}

	return netSample{rxBytes: rxBytes, txBytes: txBytes, valid: true}, nil
}

func readUptime() (uint64, error) {
	raw, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0, fmt.Errorf("read uptime: %w", err)
	}

	fields := strings.Fields(string(raw))
	if len(fields) == 0 {
		return 0, fmt.Errorf("read uptime: invalid content")
	}

	value, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, err
	}

	return uint64(value), nil
}
