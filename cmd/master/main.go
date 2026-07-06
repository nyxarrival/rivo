package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strings"
	"time"

	"rivo/internal/logging"
	"rivo/internal/master/api"
	"rivo/internal/master/config"
	"rivo/internal/master/database"
	"rivo/internal/master/model"
	"rivo/internal/master/retention"
	mastertcp "rivo/internal/master/tcp"

	"gorm.io/gorm"
)

func main() {
	configPath := flag.String("config", "configs/master.example.yaml", "master config path")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("load config failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	logging.CleanupOldFile(cfg.Log.File, cfg.Log.RetentionDays)
	logger := logging.New(cfg.Log.Level, cfg.Log.File)

	db, err := database.Open(cfg.Database.Driver, cfg.Database.DSN)
	if err != nil {
		logger.Error("open database failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	if cfg.Database.AutoMigrate {
		if err := db.AutoMigrate(
			&model.Node{},
			&model.NodeMetric{},
			&model.NodeSnapshot{},
			&model.ProbeTask{},
			&model.ProbeTaskAssignment{},
			&model.ProbeResult{},
			&model.Alert{},
			&model.NodeEvent{},
			&model.SystemLog{},
			&model.AppSetting{},
			&model.RegionOption{},
		); err != nil {
			logger.Error("auto migrate failed", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}

	logger = logging.NewFromHandlers(
		logging.NewHandler(cfg.Log.Level, cfg.Log.File),
		logging.NewCallbackHandler(cfg.Log.Level, func(_ context.Context, record slog.Record) error {
			log := systemLogFromRecord(record)
			if log == nil {
				return nil
			}
			return db.Create(log).Error
		}),
	)

	go cleanupLogs(context.Background(), logger, db, cfg.Log.RetentionDays)
	go retention.StartTelemetryCleanup(context.Background(), logger, db)

	tcpServer := mastertcp.NewServer(cfg.TCP.ListenAddr, cfg.TCP.SecretKey, logger, db)
	go tcpServer.StartOfflineMonitor(context.Background())
	go func() {
		if err := tcpServer.Run(context.Background()); err != nil {
			logger.Error("tcp server stopped", slog.String("error", err.Error()))
		}
	}()

	router := api.NewRouter(logger, db, cfg, tcpServer)
	adminPath := api.EffectiveAdminPath(db, cfg.HTTP.AdminPath)
	adminURL := adminConsoleURL(cfg.HTTP.ListenAddr, adminPath)
	logger.Info("master http server starting", slog.String("addr", cfg.HTTP.ListenAddr), slog.String("admin_url", adminURL))
	if adminURL != "" {
		fmt.Println("后台页面地址:", adminURL)
	}
	if err := router.Run(cfg.HTTP.ListenAddr); err != nil {
		logger.Error("http server stopped", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

func adminConsoleURL(listenAddr string, adminPath string) string {
	adminPath = strings.Trim(adminPath, "/")
	if adminPath == "" {
		return ""
	}
	host, port, err := net.SplitHostPort(strings.TrimSpace(listenAddr))
	if err != nil {
		return "http://" + strings.Trim(strings.TrimSpace(listenAddr), "/") + "/" + adminPath
	}
	host = strings.Trim(host, "[]")
	switch host {
	case "", "0.0.0.0", "::", "::0":
		host = "127.0.0.1"
	}
	if port == "" && strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	if port == "" {
		return "http://" + host + "/" + adminPath
	}
	return "http://" + net.JoinHostPort(host, port) + "/" + adminPath
}

func systemLogFromRecord(record slog.Record) *model.SystemLog {
	if record.Message == "decrypted metrics received" {
		return nil
	}
	eventType := eventTypeFromMessage(record.Message)
	meta := make(map[string]any)
	log := &model.SystemLog{
		Service:   "master",
		Level:     normalizeSystemLogLevel(record.Level.String()),
		EventType: eventType,
		Message:   record.Message,
		MetaJSON:  "{}",
		CreatedAt: record.Time,
	}
	record.Attrs(func(attr slog.Attr) bool {
		switch attr.Key {
		case "node_id":
			log.NodeID = attr.Value.String()
		case "event_type":
			if value := attr.Value.String(); value != "" {
				log.EventType = value
			}
		case "service":
			if value := attr.Value.String(); value != "" {
				log.Service = value
			}
		default:
			meta[attr.Key] = slogValueToAny(attr.Value)
		}
		return true
	})
	log.MetaJSON = marshalLogMeta(meta)
	return log
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

func eventTypeFromMessage(message string) string {
	switch message {
	case "master http server starting":
		return "master.http_starting"
	case "tcp server stopped":
		return "master.tcp_stopped"
	case "http server stopped":
		return "master.http_stopped"
	case "agent connected":
		return "agent.tcp_connected"
	case "agent disconnected":
		return "agent.tcp_disconnected"
	case "node marked offline":
		return "agent.offline"
	case "telemetry data cleaned", "telemetry data cleaned after settings update":
		return "telemetry.cleaned"
	case "cleanup telemetry data failed", "cleanup telemetry data after settings update failed":
		return "telemetry.cleanup_failed"
	case "store system log failed":
		return "system.log_failed"
	default:
		return "system.log"
	}
}

func slogValueToAny(value slog.Value) any {
	switch value.Kind() {
	case slog.KindString:
		return value.String()
	case slog.KindBool:
		return value.Bool()
	case slog.KindInt64:
		return value.Int64()
	case slog.KindUint64:
		return value.Uint64()
	case slog.KindFloat64:
		return value.Float64()
	case slog.KindDuration:
		return value.Duration().String()
	case slog.KindTime:
		return value.Time().Format(time.RFC3339Nano)
	case slog.KindGroup:
		group := make(map[string]any)
		for _, attr := range value.Group() {
			group[attr.Key] = slogValueToAny(attr.Value)
		}
		return group
	default:
		return value.String()
	}
}

func marshalLogMeta(meta map[string]any) string {
	if len(meta) == 0 {
		return "{}"
	}
	raw, err := json.Marshal(meta)
	if err == nil {
		return string(raw)
	}
	fallback := make(map[string]string, len(meta))
	for key, value := range meta {
		fallback[key] = fmt.Sprint(value)
	}
	raw, err = json.Marshal(fallback)
	if err != nil {
		return "{}"
	}
	return string(raw)
}

func cleanupLogs(ctx context.Context, logger *slog.Logger, db *gorm.DB, retentionDays int) {
	if retentionDays <= 0 {
		return
	}

	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		cutoff := time.Now().AddDate(0, 0, -retentionDays)
		if err := db.Where("created_at < ?", cutoff).Delete(&model.SystemLog{}).Error; err != nil {
			logger.Warn("cleanup system logs failed", slog.String("error", err.Error()))
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
