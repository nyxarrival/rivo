package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"

	"rivo/internal/agent/client"
	"rivo/internal/agent/config"
	agentstate "rivo/internal/agent/state"
	"rivo/internal/logging"
)

// version is injected by go build -ldflags "-X main.version=...".
var version = "dev"

func main() {
	configPath := flag.String("config", "configs/agent.example.yaml", "agent config path")
	masterAddr := flag.String("master", "", "master TCP address, for example 192.168.1.10:9443")
	flag.StringVar(masterAddr, "master-addr", "", "master TCP address")
	secretKey := flag.String("secret-key", "", "agent and master connection secret key")
	nodeID := flag.String("node-id", "", "fixed node ID")
	stateFile := flag.String("state-file", "", "agent state file path")
	logLevel := flag.String("log-level", "", "log level")
	logFile := flag.String("log-file", "", "log file path")
	logRetentionDays := flag.Int("log-retention-days", 0, "log retention days")
	showVersion := flag.Bool("version", false, "print agent version")
	flag.Parse()

	if *showVersion {
		_, _ = os.Stdout.WriteString(version + "\n")
		return
	}

	passed := passedFlags()
	opts := cliOptions{
		masterAddr:       *masterAddr,
		secretKey:        *secretKey,
		nodeID:           *nodeID,
		stateFile:        *stateFile,
		logLevel:         *logLevel,
		logFile:          *logFile,
		logRetentionDays: *logRetentionDays,
	}
	cfg, err := loadConfig(*configPath, passed)
	if err != nil {
		slog.Error("load config failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	if passed["state-file"] {
		cfg.Agent.StateFile = strings.TrimSpace(opts.stateFile)
	}
	cfg.Normalize()
	if err := applyPersistedState(cfg, passed); err != nil {
		slog.Error("load agent state failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	applyCLIOptions(cfg, opts, passed)
	cfg.Normalize()
	if cfg.MasterAddr == "" {
		slog.Error("master address is required", slog.String("hint", "set master_addr in config or pass --master"))
		os.Exit(1)
	}
	if cfg.SecretKey == "" {
		slog.Error("secret key is required", slog.String("hint", "set secret_key in config or pass --secret-key"))
		os.Exit(1)
	}

	logging.CleanupOldFile(cfg.Log.File, cfg.Log.RetentionDays)
	logger := logging.New(cfg.Log.Level, cfg.Log.File)
	c, err := client.New(client.Options{
		MasterAddr:             cfg.MasterAddr,
		SecretKey:              cfg.SecretKey,
		AgentVersion:           version,
		NodeID:                 cfg.Agent.NodeID,
		StateFile:              cfg.Agent.StateFile,
		PublicIPEnabled:        cfg.PublicIP.Enabled,
		PublicIPTimeoutMS:      cfg.PublicIP.TimeoutMS,
		PublicIPRefreshSeconds: cfg.PublicIP.RefreshIntervalSeconds,
		PublicIPv4Enabled:      cfg.PublicIP.IPv4Enabled,
		PublicIPv6Enabled:      cfg.PublicIP.IPv6Enabled,
		PublicIPv4Endpoints:    cfg.PublicIP.IPv4Endpoints,
		PublicIPv6Endpoints:    cfg.PublicIP.IPv6Endpoints,
		Logger:                 logger,
	})
	if err != nil {
		logger.Error("init agent failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	if err := saveFinalState(cfg, c.NodeID()); err != nil {
		logger.Error("save agent state failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	if err := c.Run(context.Background()); err != nil {
		logger.Error("agent stopped", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

type cliOptions struct {
	masterAddr       string
	secretKey        string
	nodeID           string
	stateFile        string
	logLevel         string
	logFile          string
	logRetentionDays int
}

func passedFlags() map[string]bool {
	passed := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) {
		passed[f.Name] = true
	})
	return passed
}

func loadConfig(path string, passed map[string]bool) (*config.Config, error) {
	cfg, err := config.Load(path)
	if err == nil {
		return cfg, nil
	}
	if passed["config"] || !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	cfg = config.Default()
	cfg.Normalize()
	return cfg, nil
}

func applyCLIOptions(cfg *config.Config, opts cliOptions, passed map[string]bool) {
	if passed["master"] || passed["master-addr"] {
		cfg.MasterAddr = normalizeMasterAddr(opts.masterAddr)
	}
	if passed["secret-key"] {
		cfg.SecretKey = strings.TrimSpace(opts.secretKey)
	}
	if passed["node-id"] {
		cfg.Agent.NodeID = strings.TrimSpace(opts.nodeID)
	}
	if passed["state-file"] {
		cfg.Agent.StateFile = strings.TrimSpace(opts.stateFile)
	}
	if passed["log-level"] {
		cfg.Log.Level = strings.TrimSpace(opts.logLevel)
	}
	if passed["log-file"] {
		cfg.Log.File = strings.TrimSpace(opts.logFile)
	}
	if passed["log-retention-days"] {
		cfg.Log.RetentionDays = opts.logRetentionDays
	}
}

func applyPersistedState(cfg *config.Config, passed map[string]bool) error {
	state, err := agentstate.Read(cfg.Agent.StateFile)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}

	if !passed["config"] {
		if state.MasterAddr != "" && !passed["master"] && !passed["master-addr"] {
			cfg.MasterAddr = state.MasterAddr
		}
		if state.SecretKey != "" && !passed["secret-key"] {
			cfg.SecretKey = state.SecretKey
		}
	}
	if state.NodeID != "" && cfg.Agent.NodeID == "" && !passed["node-id"] {
		cfg.Agent.NodeID = state.NodeID
	}

	return nil
}

func saveFinalState(cfg *config.Config, nodeID string) error {
	return agentstate.Save(cfg.Agent.StateFile, agentstate.State{
		MasterAddr: cfg.MasterAddr,
		SecretKey:  cfg.SecretKey,
		NodeID:     nodeID,
	})
}

func normalizeMasterAddr(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return value
	}
	if _, _, err := net.SplitHostPort(value); err == nil {
		return value
	}
	if strings.Count(value, ":") == 0 {
		return net.JoinHostPort(value, strconv.Itoa(9443))
	}
	if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
		return value + ":9443"
	}
	return value
}
