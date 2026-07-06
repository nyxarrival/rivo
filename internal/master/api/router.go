package api

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"math"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"rivo/internal/master/config"
	"rivo/internal/master/model"
	"rivo/internal/master/notify"
	"rivo/internal/master/retention"
	"rivo/internal/master/web"
	"rivo/internal/protocol"
)

type NodeOverview struct {
	model.Node
	LatestMetric          *model.NodeMetric    `json:"latest_metric,omitempty"`
	IPAddresses           protocol.IPAddresses `json:"ip_addresses"`
	PublicIPs             protocol.PublicIPs   `json:"public_ips"`
	ProbeTaskIDs          []uint64             `json:"probe_task_ids"`
	RemainingDays         int64                `json:"remaining_days"`
	RemainingValue        float64              `json:"remaining_value"`
	TrafficUsedBytes      uint64               `json:"traffic_used_bytes"`
	TrafficRemainingBytes uint64               `json:"traffic_remaining_bytes"`
}

type ProbeResultOverview struct {
	model.ProbeResult
	TaskName       string   `json:"task_name"`
	Samples        int64    `json:"samples"`
	SuccessSamples int64    `json:"success_samples"`
	FailedSamples  int64    `json:"failed_samples"`
	MinLatencyMS   *float64 `json:"min_latency_ms,omitempty"`
	MaxLatencyMS   *float64 `json:"max_latency_ms,omitempty"`
	BucketSeconds  int64    `json:"bucket_seconds,omitempty"`
	Aggregated     bool     `json:"aggregated,omitempty"`
}

type ProbeResultsResponse struct {
	Tasks         []model.ProbeTask     `json:"tasks"`
	Results       []ProbeResultOverview `json:"results"`
	GeneratedAt   int64                 `json:"generated_at"`
	RangeAnchor   int64                 `json:"range_anchor,omitempty"`
	Aggregated    bool                  `json:"aggregated,omitempty"`
	BucketSeconds int64                 `json:"bucket_seconds,omitempty"`
}

type DashboardSparklinePoint struct {
	Timestamp       uint64  `json:"ts"`
	CPUUsage        float64 `json:"cpu_usage"`
	MemUsedPercent  float64 `json:"mem_used_percent"`
	DiskUsedPercent float64 `json:"disk_used_percent"`
	NetRxBps        uint64  `json:"net_rx_bps"`
	NetTxBps        uint64  `json:"net_tx_bps"`
}

type DashboardNodeProbeStat struct {
	Samples             int64    `json:"samples"`
	SuccessSamples      int64    `json:"success_samples"`
	FailedSamples       int64    `json:"failed_samples"`
	AvailabilityPercent *float64 `json:"availability_percent,omitempty"`
	AvgLatencyMS        *float64 `json:"avg_latency_ms,omitempty"`
	PacketLossPercent   *float64 `json:"packet_loss_percent,omitempty"`
	LatencyP50MS        *float64 `json:"latency_p50_ms,omitempty"`
	LatencyP90MS        *float64 `json:"latency_p90_ms,omitempty"`
	JitterMS            *float64 `json:"jitter_ms,omitempty"`
	LatencyBaselineMS   *float64 `json:"latency_baseline_ms,omitempty"`
	LatencySpikeRatio   *float64 `json:"latency_spike_ratio,omitempty"`
}

type DashboardNodeHealthScore struct {
	Score          float64 `json:"score"`
	Grade          string  `json:"grade"`
	FreshnessScore float64 `json:"freshness_score"`
	ResourceScore  float64 `json:"resource_score"`
	LoadScore      float64 `json:"load_score"`
	NetworkScore   float64 `json:"network_score"`
	StabilityScore float64 `json:"stability_score"`
}

type DashboardSummary struct {
	NodesTotal          int64                                `json:"nodes_total"`
	NodesOnline         int64                                `json:"nodes_online"`
	NodesOffline        int64                                `json:"nodes_offline"`
	ClusterHealthScore  *float64                             `json:"cluster_health_score,omitempty"`
	AvgLatencyMS        *float64                             `json:"avg_latency_ms,omitempty"`
	AvailabilityPercent *float64                             `json:"availability_percent,omitempty"`
	ProbeSamples        int64                                `json:"probe_samples"`
	CurrentAlerts       int64                                `json:"current_alerts"`
	NodeSparklines      map[string][]DashboardSparklinePoint `json:"node_sparklines"`
	NodeProbeStats      map[string]DashboardNodeProbeStat    `json:"node_probe_stats"`
	NodeHealthScores    map[string]DashboardNodeHealthScore  `json:"node_health_scores"`
}

type DashboardEventMetric struct {
	CPUUsage        float64 `json:"cpu_usage"`
	MemUsedPercent  float64 `json:"mem_used_percent"`
	DiskUsedPercent float64 `json:"disk_used_percent"`
	NetRxBps        uint64  `json:"net_rx_bps"`
	NetTxBps        uint64  `json:"net_tx_bps"`
}

type DashboardEvent struct {
	ID        string                `json:"id"`
	EventType string                `json:"event_type"`
	Level     string                `json:"level"`
	NodeID    string                `json:"node_id"`
	NodeName  string                `json:"node_name"`
	Message   string                `json:"message"`
	CreatedAt int64                 `json:"created_at"`
	Metric    *DashboardEventMetric `json:"metric,omitempty"`
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type ConfigPublisher interface {
	PublishConfig(nodeID string) error
	RequestMetrics(nodeID string) error
}

type nodeUpdateRequest struct {
	Name                       *string        `json:"name"`
	Region                     *string        `json:"region"`
	Provider                   *string        `json:"provider"`
	NetworkLine                *string        `json:"network_line"`
	Tag                        *string        `json:"tag"`
	HeartbeatInterval          *uint32        `json:"heartbeat_interval"`
	MetricsInterval            *uint32        `json:"metrics_interval"`
	SnapshotOverride           *bool          `json:"snapshot_override"`
	SnapshotEnabled            *bool          `json:"snapshot_enabled"`
	SnapshotCollectProcesses   *bool          `json:"snapshot_collect_processes"`
	SnapshotCollectConnections *bool          `json:"snapshot_collect_connections"`
	SnapshotMaskSensitive      *bool          `json:"snapshot_mask_sensitive"`
	SnapshotInterval           *uint32        `json:"snapshot_interval"`
	SnapshotProcessLimit       *uint32        `json:"snapshot_process_limit"`
	SnapshotConnectionLimit    *uint32        `json:"snapshot_connection_limit"`
	BillingCycle               *string        `json:"billing_cycle"`
	PriceAmount                *float64       `json:"price_amount"`
	Currency                   *string        `json:"currency"`
	ServiceStartedAt           nullableUint64 `json:"service_started_at"`
	ServiceExpiresAt           nullableUint64 `json:"service_expires_at"`
	TrafficLimitBytes          *uint64        `json:"traffic_limit_bytes"`
	TrafficCalibrationBytes    *uint64        `json:"traffic_calibration_bytes"`
	TrafficBillingDirection    *string        `json:"traffic_billing_direction"`
	TrafficResetCycle          *string        `json:"traffic_reset_cycle"`
	ProbeTaskIDs               *[]uint64      `json:"probe_task_ids"`
}

type appSettings struct {
	ShowHomeSummary            bool    `json:"show_home_summary"`
	ShowBillingDetails         bool    `json:"show_billing_details"`
	ShowTrafficPlan            bool    `json:"show_traffic_plan"`
	ShowNodeTags               bool    `json:"show_node_tags"`
	MaskIPAddresses            bool    `json:"mask_ip_addresses"`
	SiteName                   string  `json:"site_name"`
	SiteDescription            string  `json:"site_description"`
	SiteAvatarURL              string  `json:"site_avatar_url"`
	UserAvatarURL              string  `json:"user_avatar_url"`
	HomeBackgroundURL          string  `json:"home_background_url"`
	ActiveTheme                string  `json:"active_theme"`
	AdminPath                  string  `json:"admin_path"`
	SnapshotEnabled            bool    `json:"snapshot_enabled"`
	SnapshotCollectProcesses   bool    `json:"snapshot_collect_processes"`
	SnapshotCollectConnections bool    `json:"snapshot_collect_connections"`
	SnapshotMaskSensitive      bool    `json:"snapshot_mask_sensitive"`
	SnapshotIntervalSeconds    int     `json:"snapshot_interval_seconds"`
	SnapshotProcessLimit       int     `json:"snapshot_process_limit"`
	SnapshotConnectionLimit    int     `json:"snapshot_connection_limit"`
	MetricsRetentionMonths     int     `json:"metrics_retention_months"`
	AssetBaseCurrency          string  `json:"asset_base_currency"`
	ExchangeRateAutoUpdate     bool    `json:"exchange_rate_auto_update"`
	WeComWebhookEnabled        bool    `json:"wecom_webhook_enabled"`
	WeComWebhookURL            string  `json:"wecom_webhook_url"`
	TelegramEnabled            bool    `json:"telegram_enabled"`
	TelegramBotToken           string  `json:"telegram_bot_token"`
	TelegramChatID             string  `json:"telegram_chat_id"`
	EmailEnabled               bool    `json:"email_enabled"`
	EmailSMTPHost              string  `json:"email_smtp_host"`
	EmailSMTPPort              int     `json:"email_smtp_port"`
	EmailSMTPSecurity          string  `json:"email_smtp_security"`
	EmailSMTPUsername          string  `json:"email_smtp_username"`
	EmailSMTPPassword          string  `json:"email_smtp_password"`
	EmailFrom                  string  `json:"email_from"`
	EmailTo                    string  `json:"email_to"`
	TrafficAlertEnabled        bool    `json:"traffic_alert_enabled"`
	TrafficAlertPercent        float64 `json:"traffic_alert_percent"`
	CPUAlertEnabled            bool    `json:"cpu_alert_enabled"`
	CPUAlertPercent            float64 `json:"cpu_alert_percent"`
	MemoryAlertEnabled         bool    `json:"memory_alert_enabled"`
	MemoryAlertPercent         float64 `json:"memory_alert_percent"`
	DiskLoadAlertEnabled       bool    `json:"disk_load_alert_enabled"`
	DiskLoadAlertPercent       float64 `json:"disk_load_alert_percent"`
	LoadAlertEnabled           bool    `json:"load_alert_enabled"`
	LoadAlertThreshold         float64 `json:"load_alert_threshold"`
	AlertIntervalMinutes       int     `json:"alert_interval_minutes"`
	OfflineAlertDelayMinutes   int     `json:"offline_alert_delay_minutes"`
	ExpiryAlertEnabled         bool    `json:"expiry_alert_enabled"`
	ExpiryAlertDays            int     `json:"expiry_alert_days"`
}

type weComTestRequest struct {
	WebhookURL string `json:"wecom_webhook_url"`
}

type telegramTestRequest struct {
	BotToken string `json:"telegram_bot_token"`
	ChatID   string `json:"telegram_chat_id"`
}

type emailTestRequest struct {
	SMTPHost     string `json:"email_smtp_host"`
	SMTPPort     int    `json:"email_smtp_port"`
	SMTPSecurity string `json:"email_smtp_security"`
	SMTPUsername string `json:"email_smtp_username"`
	SMTPPassword string `json:"email_smtp_password"`
	From         string `json:"email_from"`
	To           string `json:"email_to"`
}

type themeManifest struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
}

type themeInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	BuiltIn     bool   `json:"built_in"`
	Active      bool   `json:"active"`
}

type themeSetRequest struct {
	ID string `json:"id"`
}

type regionOptionResponse struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type probeTaskRequest struct {
	Name              string `json:"name"`
	Type              string `json:"type"`
	IPVersion         string `json:"ip_version"`
	Target            string `json:"target"`
	IntervalSeconds   uint32 `json:"interval_seconds"`
	TimeoutMS         uint32 `json:"timeout_ms"`
	Enabled           *bool  `json:"enabled"`
	AssignToAllAgents bool   `json:"assign_to_all_agents"`
}

type nullableUint64 struct {
	Set   bool
	Valid bool
	Value uint64
}

func (n *nullableUint64) UnmarshalJSON(raw []byte) error {
	n.Set = true
	if strings.TrimSpace(string(raw)) == "null" {
		n.Valid = false
		n.Value = 0
		return nil
	}

	var value uint64
	if err := json.Unmarshal(raw, &value); err != nil {
		return err
	}
	n.Valid = true
	n.Value = value
	return nil
}

func NewRouter(logger *slog.Logger, db *gorm.DB, cfg *config.Config, publishers ...ConfigPublisher) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	ensureDefaultRegionOptions(logger, db)

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	api := r.Group("/api")
	api.POST("/auth/login", func(c *gin.Context) {
		var req loginRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid login request"})
			return
		}

		if !credentialMatches(req.Username, cfg.Auth.Username) || !credentialMatches(req.Password, cfg.Auth.Password) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid username or password"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"token": tokenFor(cfg.Auth),
			"user": gin.H{
				"username":     cfg.Auth.Username,
				"display_name": "Master Admin",
			},
		})
	})

	api.GET("/auth/me", requireAuth(cfg.Auth), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"username":     cfg.Auth.Username,
			"display_name": "Master Admin",
		})
	})

	api.GET("/settings", func(c *gin.Context) {
		settings := loadAppSettings(db)
		settings.AdminPath = ""
		c.JSON(http.StatusOK, settings)
	})

	api.GET("/server-time", func(c *gin.Context) {
		now := time.Now()
		zoneName, offsetSeconds := now.Zone()
		c.JSON(http.StatusOK, gin.H{
			"unix_ms":               now.UnixMilli(),
			"time":                  now.Format(time.RFC3339),
			"timezone":              now.Location().String(),
			"timezone_abbreviation": zoneName,
			"utc_offset":            formatUTCOffset(offsetSeconds),
			"offset_seconds":        offsetSeconds,
		})
	})

	api.GET("/exchange-rates", func(c *gin.Context) {
		settings := loadAppSettings(db)
		rates, source, updatedAt := loadExchangeRates(settings.AssetBaseCurrency, settings.ExchangeRateAutoUpdate)
		c.JSON(http.StatusOK, gin.H{
			"base_currency": strings.ToUpper(settings.AssetBaseCurrency),
			"rates":         rates,
			"source":        source,
			"updated_at":    updatedAt,
		})
	})

	api.GET("/region-options", func(c *gin.Context) {
		c.JSON(http.StatusOK, loadRegionOptions(db))
	})

	api.GET("/dashboard/summary", func(c *gin.Context) {
		summary, err := buildDashboardSummary(db)
		if err != nil {
			logger.Error("build dashboard summary failed", slog.String("error", err.Error()))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "build dashboard summary failed"})
			return
		}

		c.JSON(http.StatusOK, summary)
	})

	api.GET("/dashboard/events", func(c *gin.Context) {
		limit := parseLimit(c.Query("limit"), 24, 100)
		events, err := buildDashboardEvents(db, limit)
		if err != nil {
			logger.Error("build dashboard events failed", slog.String("error", err.Error()))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "build dashboard events failed"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"events":       events,
			"generated_at": time.Now().UnixMilli(),
		})
	})

	api.GET("/nodes", func(c *gin.Context) {
		var nodes []model.Node
		if err := db.Order("id desc").Find(&nodes).Error; err != nil {
			logger.Error("list nodes failed", slog.String("error", err.Error()))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "list nodes failed"})
			return
		}

		overviews := make([]NodeOverview, 0, len(nodes))
		for _, node := range nodes {
			overviews = append(overviews, buildNodeOverview(db, node))
		}

		c.JSON(http.StatusOK, overviews)
	})

	api.GET("/nodes/:node_id", func(c *gin.Context) {
		nodeID := c.Param("node_id")

		var node model.Node
		if err := db.Where("node_id = ?", nodeID).First(&node).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
				return
			}
			logger.Error("get node failed", slog.String("node_id", nodeID), slog.String("error", err.Error()))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "get node failed"})
			return
		}

		c.JSON(http.StatusOK, buildNodeOverview(db, node))
	})

	api.GET("/nodes/:node_id/metrics", func(c *gin.Context) {
		nodeID := c.Param("node_id")
		rangeDuration, hasRange := parseRangeDuration(c.Query("range"))
		hours := parseLimit(c.Query("hours"), 0, 24*366)
		defaultLimit := 180
		maxLimit := 20000
		if hasRange {
			defaultLimit = rangeResultLimit(rangeDuration)
			maxLimit = defaultLimit
		} else if hours > 0 {
			defaultLimit = 20000
		}
		limit := parseLimit(c.Query("limit"), defaultLimit, maxLimit)

		var metrics []model.NodeMetric
		query := db.Where("node_id = ?", nodeID)
		if hasRange {
			since := uint64(time.Now().Add(-rangeDuration).UnixMilli())
			query = query.Where("ts >= ?", since)
		} else if hours > 0 {
			since := uint64(time.Now().Add(-time.Duration(hours) * time.Hour).UnixMilli())
			query = query.Where("ts >= ?", since)
		}
		err := query.Order("ts desc").Limit(limit).Find(&metrics).Error
		if err != nil {
			logger.Error("list node metrics failed", slog.String("node_id", nodeID), slog.String("error", err.Error()))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "list node metrics failed"})
			return
		}

		reverseMetrics(metrics)
		c.JSON(http.StatusOK, metrics)
	})

	api.POST("/nodes/:node_id/request-metrics", func(c *gin.Context) {
		nodeID := c.Param("node_id")
		if len(publishers) == 0 {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "tcp publisher unavailable"})
			return
		}

		var count int64
		if err := db.Model(&model.Node{}).Where("node_id = ?", nodeID).Count(&count).Error; err != nil {
			logger.Error("check node failed", slog.String("node_id", nodeID), slog.String("error", err.Error()))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "check node failed"})
			return
		}
		if count == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
			return
		}

		if err := publishers[0].RequestMetrics(nodeID); err != nil {
			logger.Warn("request agent metrics failed", slog.String("node_id", nodeID), slog.String("error", err.Error()))
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
			return
		}

		storeSystemEventLog(db, "master", nodeID, "info", "metrics.refresh_requested", "metrics refresh requested", nil)
		c.JSON(http.StatusOK, gin.H{"requested": true})
	})

	api.GET("/nodes/:node_id/probe-results", func(c *gin.Context) {
		nodeID := c.Param("node_id")
		rangeValue := c.Query("range")
		rangeDuration, hasRange := parseRangeDuration(rangeValue)
		hours := parseLimit(c.Query("hours"), 24, 24*366)
		if !hasRange {
			rangeDuration = time.Duration(hours) * time.Hour
		}
		defaultLimit := 20000
		maxLimit := 20000
		if hasRange {
			defaultLimit = rangeResultLimit(rangeDuration)
			maxLimit = defaultLimit
		}
		limit := parseLimit(c.Query("limit"), defaultLimit, maxLimit)
		includeInactive := isTruthyQuery(c.Query("include_inactive"))

		tasks, err := listProbeTasksForNode(db, nodeID, includeInactive)
		if err != nil {
			logger.Error("list probe tasks failed", slog.String("node_id", nodeID), slog.String("error", err.Error()))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "list probe tasks failed"})
			return
		}
		taskNames := make(map[uint64]string, len(tasks))
		taskIDs := make([]uint64, 0, len(tasks))
		for _, task := range tasks {
			taskNames[task.ID] = task.Name
			taskIDs = append(taskIDs, task.ID)
		}

		rangeAnchor := time.Now()
		if includeInactive {
			activeTaskIDs := activeProbeTaskIDs(tasks)
			if latest, ok := latestInactiveProbeResultTime(db, nodeID, activeTaskIDs); ok {
				rangeAnchor = latest
			} else if latest, ok := latestProbeResultTime(db, nodeID); ok {
				rangeAnchor = latest
			}
		}
		since := rangeAnchor.Add(-rangeDuration)

		if bucketSeconds, ok := probeAggregateBucketSeconds(rangeValue); ok {
			var overviews []ProbeResultOverview
			overviews, err = listAggregatedProbeResults(db, nodeID, since, bucketSeconds, taskNames, taskIDs, includeInactive)
			if err != nil {
				logger.Error("list aggregated probe results failed", slog.String("node_id", nodeID), slog.String("error", err.Error()))
				c.JSON(http.StatusInternalServerError, gin.H{"error": "list aggregated probe results failed"})
				return
			}
			c.JSON(http.StatusOK, ProbeResultsResponse{
				Tasks:         tasks,
				Results:       overviews,
				GeneratedAt:   time.Now().UnixMilli(),
				RangeAnchor:   rangeAnchor.UnixMilli(),
				Aggregated:    true,
				BucketSeconds: bucketSeconds,
			})
			return
		}

		var results []model.ProbeResult
		query := db.Where("node_id = ? AND created_at >= ?", nodeID, since)
		if !includeInactive {
			if len(taskIDs) == 0 {
				c.JSON(http.StatusOK, ProbeResultsResponse{
					Tasks:       tasks,
					Results:     []ProbeResultOverview{},
					GeneratedAt: time.Now().UnixMilli(),
					RangeAnchor: rangeAnchor.UnixMilli(),
				})
				return
			}
			query = query.Where("task_id IN ?", taskIDs)
		}
		err = query.
			Order("created_at desc").
			Limit(limit).
			Find(&results).Error
		if err != nil {
			logger.Error("list probe results failed", slog.String("node_id", nodeID), slog.String("error", err.Error()))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "list probe results failed"})
			return
		}
		reverseProbeResults(results)

		overviews := make([]ProbeResultOverview, 0, len(results))
		for _, result := range results {
			successSamples := int64(0)
			failedSamples := int64(1)
			if result.Status == "success" && result.LatencyMS != nil {
				successSamples = 1
				failedSamples = 0
			}
			overviews = append(overviews, ProbeResultOverview{
				ProbeResult:    result,
				TaskName:       taskNames[result.TaskID],
				Samples:        1,
				SuccessSamples: successSamples,
				FailedSamples:  failedSamples,
				MinLatencyMS:   result.LatencyMS,
				MaxLatencyMS:   result.LatencyMS,
			})
		}

		c.JSON(http.StatusOK, ProbeResultsResponse{
			Tasks:       tasks,
			Results:     overviews,
			GeneratedAt: time.Now().UnixMilli(),
			RangeAnchor: rangeAnchor.UnixMilli(),
		})
	})

	admin := api.Group("/admin", requireAuth(cfg.Auth))
	admin.GET("/config", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"http": gin.H{
				"listen_addr": cfg.HTTP.ListenAddr,
				"admin_path":  effectiveAdminPath(db, cfg.HTTP.AdminPath),
			},
			"tcp": gin.H{
				"listen_addr":           cfg.TCP.ListenAddr,
				"secret_key_configured": cfg.TCP.SecretKey != "",
			},
			"database": gin.H{
				"driver":       cfg.Database.Driver,
				"dsn":          maskDSN(cfg.Database.DSN),
				"auto_migrate": cfg.Database.AutoMigrate,
			},
			"auth": gin.H{
				"username": cfg.Auth.Username,
			},
			"log": gin.H{
				"level":          cfg.Log.Level,
				"file":           cfg.Log.File,
				"retention_days": cfg.Log.RetentionDays,
			},
		})
	})

	admin.GET("/settings", func(c *gin.Context) {
		c.JSON(http.StatusOK, loadAdminAppSettings(db, cfg.HTTP.AdminPath))
	})

	admin.GET("/themes", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"themes": listThemes(db, resolveThemesDir()),
		})
	})

	admin.POST("/themes/upload", func(c *gin.Context) {
		file, err := c.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "theme zip file is required"})
			return
		}
		theme, err := installThemeArchive(file, resolveThemesDir())
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		storeSystemEventLog(db, "master", "", "info", "theme.uploaded", "theme uploaded", map[string]any{"theme_id": theme.ID})
		c.JSON(http.StatusOK, theme)
	})

	admin.PUT("/themes/active", func(c *gin.Context) {
		var req themeSetRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid theme request"})
			return
		}
		themeID := normalizeThemeID(req.ID)
		if themeID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "theme id is required"})
			return
		}
		if !themeExists(themeID, resolveThemesDir(), web.DefaultThemeDist()) {
			c.JSON(http.StatusNotFound, gin.H{"error": "theme not found"})
			return
		}
		if err := saveAppSetting(db, "active_theme", themeID); err != nil {
			logger.Error("activate theme failed", slog.String("theme_id", themeID), slog.String("error", err.Error()))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "activate theme failed"})
			return
		}
		storeSystemEventLog(db, "master", "", "info", "theme.activated", "theme activated", map[string]any{"theme_id": themeID})
		c.JSON(http.StatusOK, gin.H{"active_theme": themeID})
	})

	admin.DELETE("/themes/:theme_id", func(c *gin.Context) {
		themeID := normalizeThemeID(c.Param("theme_id"))
		if themeID == "" || themeID == "default" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "built-in default theme cannot be deleted"})
			return
		}
		if loadAppSettings(db).ActiveTheme == themeID {
			c.JSON(http.StatusBadRequest, gin.H{"error": "active theme cannot be deleted"})
			return
		}
		if err := deleteTheme(themeID, resolveThemesDir()); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		storeSystemEventLog(db, "master", "", "info", "theme.deleted", "theme deleted", map[string]any{"theme_id": themeID})
		c.JSON(http.StatusOK, gin.H{"deleted": true})
	})

	admin.PUT("/settings", func(c *gin.Context) {
		var req appSettings
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid settings request"})
			return
		}
		req = normalizeAppSettingsForSave(req)
		if req.AdminPath == "" {
			req.AdminPath = effectiveAdminPath(db, cfg.HTTP.AdminPath)
		}
		if message := validateAppSettings(req); message != "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": message})
			return
		}
		if err := saveAppSettings(db, req); err != nil {
			logger.Error("save app settings failed", slog.String("error", err.Error()))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "save settings failed"})
			return
		}
		if result, err := retention.CleanupTelemetryData(db, req.MetricsRetentionMonths); err != nil {
			logger.Warn("cleanup telemetry data after settings update failed", slog.String("error", err.Error()))
		} else if result.DeletedMetrics > 0 || result.DeletedProbeResults > 0 {
			logger.Info(
				"telemetry data cleaned after settings update",
				slog.Int("retention_months", result.RetentionMonths),
				slog.Int64("deleted_metrics", result.DeletedMetrics),
				slog.Int64("deleted_probe_results", result.DeletedProbeResults),
			)
		}
		publishAllNodeConfigs(logger, db, publishers)
		storeSystemEventLog(db, "master", "", "info", "settings.updated", "global settings updated", nil)
		c.JSON(http.StatusOK, loadAdminAppSettings(db, cfg.HTTP.AdminPath))
	})

	admin.POST("/settings/wecom-test", func(c *gin.Context) {
		var req weComTestRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid wecom test request"})
			return
		}
		webhookURL := strings.TrimSpace(req.WebhookURL)
		if webhookURL == "" {
			webhookURL = strings.TrimSpace(loadAppSettings(db).WeComWebhookURL)
		}
		if message := validateWeComWebhookURL(webhookURL, true); message != "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": message})
			return
		}
		content := strings.Join([]string{
			"【测试消息】",
			"",
			"✅ Rivo 企业微信测试消息",
			"来源：后台管理",
			"时间：" + time.Now().Format("2006-01-02 15:04:05"),
		}, "\n")
		if err := postWeComWebhook(webhookURL, content); err != nil {
			logger.Warn("send wecom test message failed", slog.String("error", err.Error()))
			c.JSON(http.StatusBadGateway, gin.H{"error": "send wecom test message failed: " + err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	admin.POST("/settings/telegram-test", func(c *gin.Context) {
		var req telegramTestRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid telegram test request"})
			return
		}
		settings := loadAppSettings(db)
		botToken := strings.TrimSpace(req.BotToken)
		if botToken == "" {
			botToken = strings.TrimSpace(settings.TelegramBotToken)
		}
		chatID := strings.TrimSpace(req.ChatID)
		if chatID == "" {
			chatID = strings.TrimSpace(settings.TelegramChatID)
		}
		if message := validateTelegramSettings(botToken, chatID, true); message != "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": message})
			return
		}
		content := strings.Join([]string{
			"【测试消息】",
			"",
			"✅ Rivo Telegram 测试消息",
			"来源：后台管理",
			"时间：" + time.Now().Format("2006-01-02 15:04:05"),
		}, "\n")
		if err := postTelegramMessage(botToken, chatID, content); err != nil {
			logger.Warn("send telegram test message failed", slog.String("error", err.Error()))
			c.JSON(http.StatusBadGateway, gin.H{"error": "send telegram test message failed: " + err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	admin.POST("/settings/email-test", func(c *gin.Context) {
		var req emailTestRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid email test request"})
			return
		}
		settings := loadAppSettings(db)
		emailSettings := notify.EmailSettings{
			Enabled:  true,
			Host:     firstNonEmpty(req.SMTPHost, settings.EmailSMTPHost),
			Port:     firstNonZero(req.SMTPPort, settings.EmailSMTPPort),
			Security: firstNonEmpty(req.SMTPSecurity, settings.EmailSMTPSecurity),
			Username: firstNonEmpty(req.SMTPUsername, settings.EmailSMTPUsername),
			Password: firstNonEmpty(req.SMTPPassword, settings.EmailSMTPPassword),
			From:     firstNonEmpty(req.From, settings.EmailFrom),
			To:       firstNonEmpty(req.To, settings.EmailTo),
		}
		if message := notify.ValidateEmailSettings(emailSettings, true); message != "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": message})
			return
		}
		content := strings.Join([]string{
			"【测试消息】",
			"",
			"✅ Rivo 邮件测试消息",
			"来源：后台管理",
			"时间：" + time.Now().Format("2006-01-02 15:04:05"),
		}, "\n")
		if err := notify.SendEmail(emailSettings, "Rivo 邮件测试消息", content); err != nil {
			logger.Warn("send email test message failed", slog.String("error", err.Error()))
			c.JSON(http.StatusBadGateway, gin.H{"error": "send email test message failed: " + err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	admin.GET("/agent-config", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"template": strings.TrimSpace(`master_addr: "MASTER_IP:9443"
secret_key: "base64-encoded-random-key"

agent:
  node_id: ""
  state_file: "data/agent-state.json"

log:
  level: "info"
  file: "logs/agent.log"
  retention_days: 30`),
			"fields": []gin.H{
				{"name": "master_addr", "required": true, "description": "Master TCP address"},
				{"name": "secret_key", "required": true, "description": "Base64 connection key shared with master, at least 32 decoded bytes"},
				{"name": "agent.node_id", "required": false, "description": "Optional fixed agent node ID. Leave empty to auto-generate on first start."},
				{"name": "agent.state_file", "required": false, "description": "Local file used to persist auto-generated node ID."},
			},
		})
	})

	admin.PUT("/nodes/:node_id", func(c *gin.Context) {
		nodeID := c.Param("node_id")
		var req nodeUpdateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid node update request"})
			return
		}

		var current model.Node
		if err := db.Where("node_id = ?", nodeID).First(&current).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
			return
		}
		if message := validateNodeUpdateRequest(db, req, current); message != "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": message})
			return
		}

		updates := nodeUpdates(req, current)
		hasProbeTaskChanges := req.ProbeTaskIDs != nil
		if len(updates) == 0 && !hasProbeTaskChanges {
			c.JSON(http.StatusBadRequest, gin.H{"error": "empty update"})
			return
		}

		if err := db.Transaction(func(tx *gorm.DB) error {
			if len(updates) > 0 {
				if err := tx.Model(&model.Node{}).Where("node_id = ?", nodeID).Updates(updates).Error; err != nil {
					return err
				}
			}
			if hasProbeTaskChanges {
				if err := replaceNodeProbeTaskAssignments(tx, nodeID, *req.ProbeTaskIDs); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			logger.Error("update node failed", slog.String("node_id", nodeID), slog.String("error", err.Error()))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "update node failed"})
			return
		}

		var node model.Node
		if err := db.Where("node_id = ?", nodeID).First(&node).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
			return
		}

		if len(publishers) > 0 {
			if err := publishers[0].PublishConfig(nodeID); err != nil {
				logger.Warn("publish agent config failed", slog.String("node_id", nodeID), slog.String("error", err.Error()))
			}
		}

		storeSystemEventLog(db, "master", nodeID, "info", "node.config_updated", "node config updated", nil)
		c.JSON(http.StatusOK, buildNodeOverview(db, node))
	})

	admin.GET("/nodes/:node_id/snapshots/latest", func(c *gin.Context) {
		nodeID := c.Param("node_id")
		var node model.Node
		if err := db.Select("node_id").Where("node_id = ?", nodeID).First(&node).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "node not found"})
				return
			}
			logger.Error("check snapshot node failed", slog.String("node_id", nodeID), slog.String("error", err.Error()))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "check node failed"})
			return
		}

		var snapshot model.NodeSnapshot
		if err := db.Where("node_id = ?", nodeID).Order("ts desc").First(&snapshot).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusOK, gin.H{"snapshot": nil, "generated_at": time.Now().UnixMilli()})
				return
			}
			logger.Error("get latest node snapshot failed", slog.String("node_id", nodeID), slog.String("error", err.Error()))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "get snapshot failed"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"snapshot":     snapshot,
			"generated_at": time.Now().UnixMilli(),
		})
	})

	admin.GET("/probe-tasks", func(c *gin.Context) {
		var tasks []model.ProbeTask
		if err := db.Order("id desc").Find(&tasks).Error; err != nil {
			logger.Error("list probe tasks failed", slog.String("error", err.Error()))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "list probe tasks failed"})
			return
		}
		c.JSON(http.StatusOK, tasks)
	})

	admin.POST("/probe-tasks", func(c *gin.Context) {
		var req probeTaskRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid probe task request"})
			return
		}
		if message := validateProbeTaskRequest(req); message != "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": message})
			return
		}

		task := probeTaskFromRequest(req, true)
		if req.AssignToAllAgents {
			task.Enabled = true
		}
		assignedCount := 0
		if err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(&task).Error; err != nil {
				return err
			}
			if !req.AssignToAllAgents {
				return nil
			}
			count, err := assignProbeTaskToAllNodes(tx, task.ID)
			if err != nil {
				return err
			}
			assignedCount = count
			return nil
		}); err != nil {
			logger.Error("create probe task failed", slog.String("error", err.Error()))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "create probe task failed"})
			return
		}
		publishAllNodeConfigs(logger, db, publishers)
		storeSystemEventLog(db, "master", "", "info", "probe.task_created", "probe task created: "+task.Target, map[string]any{
			"task_id":              task.ID,
			"target":               task.Target,
			"type":                 task.Type,
			"ip_version":           task.IPVersion,
			"assign_to_all_agents": req.AssignToAllAgents,
			"assigned_count":       assignedCount,
		})
		c.JSON(http.StatusOK, task)
	})

	admin.PUT("/probe-tasks/:id", func(c *gin.Context) {
		taskID, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil || taskID == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid probe task id"})
			return
		}
		var req probeTaskRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid probe task request"})
			return
		}
		if message := validateProbeTaskRequest(req); message != "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": message})
			return
		}

		var task model.ProbeTask
		if err := db.First(&task, taskID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "probe task not found"})
			return
		}
		next := probeTaskFromRequest(req, task.Enabled)
		task.Name = next.Name
		task.Type = next.Type
		task.IPVersion = next.IPVersion
		task.Target = next.Target
		task.IntervalSeconds = next.IntervalSeconds
		task.TimeoutMS = next.TimeoutMS
		task.Enabled = next.Enabled

		if err := db.Save(&task).Error; err != nil {
			logger.Error("update probe task failed", slog.Uint64("task_id", taskID), slog.String("error", err.Error()))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "update probe task failed"})
			return
		}
		publishAllNodeConfigs(logger, db, publishers)
		storeSystemEventLog(db, "master", "", "info", "probe.task_updated", "probe task updated: "+task.Target, map[string]any{
			"task_id":    task.ID,
			"target":     task.Target,
			"type":       task.Type,
			"ip_version": task.IPVersion,
		})
		c.JSON(http.StatusOK, task)
	})

	admin.DELETE("/probe-tasks/:id", func(c *gin.Context) {
		taskID, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil || taskID == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid probe task id"})
			return
		}
		if err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Where("task_id = ?", taskID).Delete(&model.ProbeTaskAssignment{}).Error; err != nil {
				return err
			}
			return tx.Delete(&model.ProbeTask{}, taskID).Error
		}); err != nil {
			logger.Error("delete probe task failed", slog.Uint64("task_id", taskID), slog.String("error", err.Error()))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "delete probe task failed"})
			return
		}
		publishAllNodeConfigs(logger, db, publishers)
		storeSystemEventLog(db, "master", "", "info", "probe.task_deleted", "probe task deleted", map[string]any{
			"task_id": taskID,
		})
		c.JSON(http.StatusOK, gin.H{"deleted": true})
	})

	admin.GET("/system-logs", func(c *gin.Context) {
		limit := parseLimit(c.Query("limit"), 100, 500)
		var logs []model.SystemLog
		query := db.Model(&model.SystemLog{})
		if level := strings.TrimSpace(c.Query("level")); level != "" {
			query = query.Where("level = ?", level)
		}
		if eventType := strings.TrimSpace(c.Query("event_type")); eventType != "" {
			query = query.Where("event_type = ?", eventType)
		}
		if nodeID := strings.TrimSpace(c.Query("node_id")); nodeID != "" {
			query = query.Where("node_id = ?", nodeID)
		}
		if err := query.Order("created_at desc").Limit(limit).Find(&logs).Error; err != nil {
			logger.Error("list system logs failed", slog.String("error", err.Error()))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "list system logs failed"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"logs":         logs,
			"generated_at": time.Now().UnixMilli(),
		})
	})

	mountThemeRoutes(
		r,
		db,
		cfg.HTTP.AdminPath,
		resolveAdminPanelDir(),
		web.AdminDist(),
		resolveThemesDir(),
		resolveDefaultThemeDir(),
		web.DefaultThemeDist(),
	)
	return r
}

func resolveAdminPanelDir() string {
	candidates := []string{
		strings.TrimSpace(os.Getenv("RIVO_ADMIN_PANEL_DIR")),
		strings.TrimSpace(os.Getenv("RIVO_PANEL_DIR")),
		"panel/dist",
		"./panel/dist",
	}
	return firstExistingIndexDir(candidates)
}

func resolveDefaultThemeDir() string {
	candidates := []string{
		strings.TrimSpace(os.Getenv("RIVO_DEFAULT_THEME_DIR")),
		"theme-default/dist",
		"./theme-default/dist",
	}
	return firstExistingIndexDir(candidates)
}

func resolveThemesDir() string {
	if value := strings.TrimSpace(os.Getenv("RIVO_THEMES_DIR")); value != "" {
		return value
	}
	return "data/themes"
}

func firstExistingIndexDir(candidates []string) string {
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		indexPath := filepath.Join(candidate, "index.html")
		if info, err := os.Stat(indexPath); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

func mountAdminRoutes(r *gin.Engine, adminPath string, panelDir string, embeddedPanel fs.FS) {
	if adminPath == "" || (panelDir == "" && embeddedPanel == nil) {
		return
	}
	prefix := "/" + strings.Trim(adminPath, "/")
	handler := func(c *gin.Context) {
		requestPath := strings.TrimPrefix(c.Request.URL.Path, prefix)
		if requestPath == "" || requestPath == "/" {
			serveAdminIndex(c, panelDir, embeddedPanel, prefix)
			return
		}
		if filePath, ok := staticFilePath(panelDir, requestPath); ok {
			c.File(filePath)
			return
		}
		if panelDir != "" {
			serveAdminIndex(c, panelDir, embeddedPanel, prefix)
			return
		}
		if embeddedFileExists(embeddedPanel, strings.TrimPrefix(path.Clean("/"+requestPath), "/")) {
			serveEmbeddedSPA(c, embeddedPanel, requestPath)
			return
		}
		serveAdminIndex(c, panelDir, embeddedPanel, prefix)
	}
	r.GET(prefix, handler)
	r.GET(prefix+"/*filepath", handler)
}

func serveAdminIndex(c *gin.Context, panelDir string, embeddedPanel fs.FS, prefix string) {
	var data []byte
	var err error
	if panelDir != "" {
		data, err = os.ReadFile(filepath.Join(panelDir, "index.html"))
	} else if embeddedPanel != nil {
		data, err = fs.ReadFile(embeddedPanel, "index.html")
	} else {
		err = fs.ErrNotExist
	}
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(injectBaseHref(string(data), prefix+"/")))
}

func injectBaseHref(raw string, href string) string {
	if strings.Contains(strings.ToLower(raw), "<base ") {
		return raw
	}
	baseTag := `<base href="` + href + `">`
	lower := strings.ToLower(raw)
	headStart := strings.Index(lower, "<head")
	if headStart == -1 {
		return baseTag + raw
	}
	headEnd := strings.Index(lower[headStart:], ">")
	if headEnd == -1 {
		return baseTag + raw
	}
	insertAt := headStart + headEnd + 1
	return raw[:insertAt] + "\n    " + baseTag + raw[insertAt:]
}

func mountThemeRoutes(
	r *gin.Engine,
	db *gorm.DB,
	adminPath string,
	panelDir string,
	embeddedPanel fs.FS,
	themesDir string,
	defaultThemeDir string,
	defaultThemeFS fs.FS,
) {
	r.GET("/themes/:theme_id/*filepath", func(c *gin.Context) {
		themeID := normalizeThemeID(c.Param("theme_id"))
		if themeID == "" || themeID == "default" {
			c.JSON(http.StatusNotFound, gin.H{"error": "theme not found"})
			return
		}
		requestPath := "/" + strings.TrimPrefix(c.Param("filepath"), "/")
		if filePath, ok := runtimeThemeFilePath(themesDir, themeID, requestPath); ok {
			c.File(filePath)
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	})

	r.NoRoute(func(c *gin.Context) {
		if isAPIRoutePath(c.Request.URL.Path) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		if serveAdminRoute(c, db, adminPath, panelDir, embeddedPanel) {
			return
		}
		serveActiveTheme(c, loadAppSettings(db).ActiveTheme, themesDir, defaultThemeDir, defaultThemeFS)
	})
}

func isAPIRoutePath(requestPath string) bool {
	return requestPath == "/api" || strings.HasPrefix(requestPath, "/api/")
}

func serveAdminRoute(c *gin.Context, db *gorm.DB, fallbackAdminPath string, panelDir string, embeddedPanel fs.FS) bool {
	if panelDir == "" && embeddedPanel == nil {
		return false
	}
	adminPath := effectiveAdminPath(db, fallbackAdminPath)
	if adminPath == "" {
		return false
	}
	prefix := "/" + adminPath
	if c.Request.URL.Path != prefix && !strings.HasPrefix(c.Request.URL.Path, prefix+"/") {
		return false
	}

	requestPath := strings.TrimPrefix(c.Request.URL.Path, prefix)
	if requestPath == "" || requestPath == "/" {
		serveAdminIndex(c, panelDir, embeddedPanel, prefix)
		return true
	}
	if filePath, ok := staticFilePath(panelDir, requestPath); ok {
		c.File(filePath)
		return true
	}
	if panelDir != "" {
		serveAdminIndex(c, panelDir, embeddedPanel, prefix)
		return true
	}
	if embeddedFileExists(embeddedPanel, strings.TrimPrefix(path.Clean("/"+requestPath), "/")) {
		serveEmbeddedSPA(c, embeddedPanel, requestPath)
		return true
	}
	serveAdminIndex(c, panelDir, embeddedPanel, prefix)
	return true
}

func serveActiveTheme(c *gin.Context, themeID string, themesDir string, defaultThemeDir string, defaultThemeFS fs.FS) {
	requestPath := c.Request.URL.Path
	if normalizeThemeID(themeID) != "default" {
		if filePath, ok := runtimeThemeFilePath(themesDir, themeID, requestPath); ok {
			c.File(filePath)
			return
		}
		if filePath, ok := runtimeThemeFilePath(themesDir, themeID, "/index.html"); ok {
			c.File(filePath)
			return
		}
	}

	if filePath, ok := staticFilePath(defaultThemeDir, requestPath); ok {
		c.File(filePath)
		return
	}
	if defaultThemeDir != "" {
		c.File(filepath.Join(defaultThemeDir, "index.html"))
		return
	}
	serveEmbeddedSPA(c, defaultThemeFS, requestPath)
}

func serveEmbeddedSPA(c *gin.Context, sourceFS fs.FS, requestPath string) {
	if sourceFS == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	name := strings.TrimPrefix(path.Clean("/"+requestPath), "/")
	if name == "" || name == "." {
		name = "index.html"
	}
	if !embeddedFileExists(sourceFS, name) {
		name = "index.html"
	}
	serveEmbeddedFile(c, sourceFS, name)
}

func serveEmbeddedFile(c *gin.Context, sourceFS fs.FS, name string) {
	data, err := fs.ReadFile(sourceFS, name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	contentType := mime.TypeByExtension(filepath.Ext(name))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	c.Data(http.StatusOK, contentType, data)
}

func embeddedFileExists(sourceFS fs.FS, name string) bool {
	info, err := fs.Stat(sourceFS, name)
	return err == nil && !info.IsDir()
}

func staticFilePath(baseDir string, requestPath string) (string, bool) {
	if baseDir == "" {
		return "", false
	}
	cleaned := strings.TrimPrefix(path.Clean("/"+requestPath), "/")
	if cleaned == "" || cleaned == "." {
		return "", false
	}
	filePath := filepath.Join(baseDir, filepath.FromSlash(cleaned))
	base, err := filepath.Abs(baseDir)
	if err != nil {
		return "", false
	}
	target, err := filepath.Abs(filePath)
	if err != nil {
		return "", false
	}
	if target != base && !strings.HasPrefix(target, base+string(os.PathSeparator)) {
		return "", false
	}
	info, err := os.Stat(target)
	if err != nil || info.IsDir() {
		return "", false
	}
	return target, true
}

func runtimeThemeFilePath(themesDir string, themeID string, requestPath string) (string, bool) {
	themeID = normalizeThemeID(themeID)
	if themeID == "" || themeID == "default" {
		return "", false
	}
	return staticFilePath(filepath.Join(themesDir, themeID, "dist"), requestPath)
}

func formatUTCOffset(offsetSeconds int) string {
	sign := "+"
	if offsetSeconds < 0 {
		sign = "-"
		offsetSeconds = -offsetSeconds
	}
	hours := offsetSeconds / 3600
	minutes := (offsetSeconds % 3600) / 60
	return fmt.Sprintf("%s%02d:%02d", sign, hours, minutes)
}

func normalizeThemeID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	for _, char := range value {
		if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '-' || char == '_') {
			return ""
		}
	}
	return value
}

func listThemes(db *gorm.DB, themesDir string) []themeInfo {
	activeTheme := loadAppSettings(db).ActiveTheme
	if activeTheme == "" {
		activeTheme = "default"
	}
	themes := []themeInfo{{
		ID:          "default",
		Name:        "默认主题",
		Version:     "built-in",
		Description: "Master 内置默认展示页",
		BuiltIn:     true,
		Active:      activeTheme == "default",
	}}
	entries, err := os.ReadDir(themesDir)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			themeID := normalizeThemeID(entry.Name())
			if themeID == "" || themeID == "default" {
				continue
			}
			manifest, ok := readRuntimeThemeManifest(themesDir, themeID)
			if !ok {
				continue
			}
			themes = append(themes, themeInfo{
				ID:          themeID,
				Name:        manifest.Name,
				Version:     manifest.Version,
				Description: manifest.Description,
				BuiltIn:     false,
				Active:      activeTheme == themeID,
			})
		}
	}
	sort.Slice(themes, func(i, j int) bool {
		if themes[i].BuiltIn != themes[j].BuiltIn {
			return themes[i].BuiltIn
		}
		return strings.ToLower(themes[i].Name) < strings.ToLower(themes[j].Name)
	})
	return themes
}

func themeExists(themeID string, themesDir string, defaultThemeFS fs.FS) bool {
	themeID = normalizeThemeID(themeID)
	if themeID == "default" {
		return defaultThemeFS != nil
	}
	if themeID == "" {
		return false
	}
	_, ok := readRuntimeThemeManifest(themesDir, themeID)
	if !ok {
		return false
	}
	_, ok = runtimeThemeFilePath(themesDir, themeID, "/index.html")
	return ok
}

func readRuntimeThemeManifest(themesDir string, themeID string) (themeManifest, bool) {
	var manifest themeManifest
	manifestPath := filepath.Join(themesDir, themeID, "theme.json")
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		manifestPath = filepath.Join(themesDir, themeID, "rivo-theme.json")
		raw, err = os.ReadFile(manifestPath)
		if err != nil {
			return manifest, false
		}
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return manifest, false
	}
	manifest.ID = normalizeThemeID(manifest.ID)
	if manifest.ID == "" {
		manifest.ID = themeID
	}
	if strings.TrimSpace(manifest.Name) == "" {
		manifest.Name = manifest.ID
	}
	return manifest, true
}

func installThemeArchive(fileHeader *multipart.FileHeader, themesDir string) (themeInfo, error) {
	var empty themeInfo
	if fileHeader.Size <= 0 {
		return empty, fmt.Errorf("theme zip file is empty")
	}
	if fileHeader.Size > 200*1024*1024 {
		return empty, fmt.Errorf("theme zip file is too large")
	}
	file, err := fileHeader.Open()
	if err != nil {
		return empty, fmt.Errorf("open theme zip failed")
	}
	defer file.Close()

	reader, err := zip.NewReader(file, fileHeader.Size)
	if err != nil {
		return empty, fmt.Errorf("invalid theme zip")
	}
	manifest, err := themeManifestFromZip(reader)
	if err != nil {
		return empty, err
	}
	if manifest.ID == "default" {
		return empty, fmt.Errorf("default is reserved for the built-in theme")
	}
	if !zipHasFile(reader, "dist/index.html") {
		return empty, fmt.Errorf("theme zip must contain dist/index.html")
	}

	if err := os.MkdirAll(themesDir, 0o755); err != nil {
		return empty, fmt.Errorf("create themes directory failed")
	}
	tempDir := filepath.Join(themesDir, ".upload-"+manifest.ID+"-"+strconv.FormatInt(time.Now().UnixNano(), 10))
	finalDir := filepath.Join(themesDir, manifest.ID)
	if err := extractThemeZip(reader, tempDir); err != nil {
		_ = os.RemoveAll(tempDir)
		return empty, err
	}
	if err := os.RemoveAll(finalDir); err != nil {
		_ = os.RemoveAll(tempDir)
		return empty, fmt.Errorf("replace old theme failed")
	}
	if err := os.Rename(tempDir, finalDir); err != nil {
		_ = os.RemoveAll(tempDir)
		return empty, fmt.Errorf("install theme failed")
	}

	return themeInfo{
		ID:          manifest.ID,
		Name:        manifest.Name,
		Version:     manifest.Version,
		Description: manifest.Description,
		BuiltIn:     false,
		Active:      false,
	}, nil
}

func themeManifestFromZip(reader *zip.Reader) (themeManifest, error) {
	var manifest themeManifest
	var raw []byte
	for _, file := range reader.File {
		name, ok := cleanZipPath(file.Name)
		if !ok {
			continue
		}
		if name == "theme.json" || name == "rivo-theme.json" {
			rc, err := file.Open()
			if err != nil {
				return manifest, fmt.Errorf("read theme manifest failed")
			}
			raw, err = io.ReadAll(io.LimitReader(rc, 1024*1024))
			_ = rc.Close()
			if err != nil {
				return manifest, fmt.Errorf("read theme manifest failed")
			}
			break
		}
	}
	if len(raw) == 0 {
		return manifest, fmt.Errorf("theme zip must contain theme.json")
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return manifest, fmt.Errorf("invalid theme manifest")
	}
	manifest.ID = normalizeThemeID(manifest.ID)
	manifest.Name = strings.TrimSpace(manifest.Name)
	manifest.Version = strings.TrimSpace(manifest.Version)
	manifest.Description = strings.TrimSpace(manifest.Description)
	if manifest.ID == "" {
		return manifest, fmt.Errorf("theme id is required and must contain only letters, digits, '-' or '_'")
	}
	if manifest.Name == "" {
		return manifest, fmt.Errorf("theme name is required")
	}
	if manifest.Version == "" {
		manifest.Version = "1.0.0"
	}
	return manifest, nil
}

func zipHasFile(reader *zip.Reader, expected string) bool {
	expected, ok := cleanZipPath(expected)
	if !ok {
		return false
	}
	for _, file := range reader.File {
		name, ok := cleanZipPath(file.Name)
		if !ok {
			continue
		}
		if name == expected && !file.FileInfo().IsDir() {
			return true
		}
	}
	return false
}

func extractThemeZip(reader *zip.Reader, targetDir string) error {
	base, err := filepath.Abs(targetDir)
	if err != nil {
		return fmt.Errorf("prepare theme directory failed")
	}
	for _, file := range reader.File {
		name, ok := cleanZipPath(file.Name)
		if !ok {
			return fmt.Errorf("invalid path in theme zip")
		}
		target := filepath.Join(targetDir, filepath.FromSlash(name))
		absTarget, err := filepath.Abs(target)
		if err != nil {
			return fmt.Errorf("invalid path in theme zip")
		}
		if absTarget != base && !strings.HasPrefix(absTarget, base+string(os.PathSeparator)) {
			return fmt.Errorf("invalid path in theme zip")
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(absTarget, 0o755); err != nil {
				return fmt.Errorf("extract theme failed")
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(absTarget), 0o755); err != nil {
			return fmt.Errorf("extract theme failed")
		}
		rc, err := file.Open()
		if err != nil {
			return fmt.Errorf("extract theme failed")
		}
		fileMode := file.Mode().Perm()
		if fileMode == 0 {
			fileMode = 0o644
		}
		out, err := os.OpenFile(absTarget, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, fileMode)
		if err != nil {
			_ = rc.Close()
			return fmt.Errorf("extract theme failed")
		}
		_, copyErr := io.Copy(out, io.LimitReader(rc, 200*1024*1024))
		closeErr := out.Close()
		_ = rc.Close()
		if copyErr != nil || closeErr != nil {
			return fmt.Errorf("extract theme failed")
		}
	}
	return nil
}

func cleanZipPath(name string) (string, bool) {
	if strings.Contains(name, "\\") {
		return "", false
	}
	cleaned := path.Clean(strings.TrimSpace(name))
	if cleaned == "" || cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") || strings.HasPrefix(cleaned, "/") {
		return "", false
	}
	return cleaned, true
}

func deleteTheme(themeID string, themesDir string) error {
	themeID = normalizeThemeID(themeID)
	if themeID == "" || themeID == "default" {
		return fmt.Errorf("invalid theme id")
	}
	themeDir := filepath.Join(themesDir, themeID)
	if _, err := os.Stat(themeDir); err != nil {
		return fmt.Errorf("theme not found")
	}
	if err := os.RemoveAll(themeDir); err != nil {
		return fmt.Errorf("delete theme failed")
	}
	return nil
}

func defaultAppSettings() appSettings {
	return appSettings{
		ShowHomeSummary:            true,
		ShowBillingDetails:         true,
		ShowTrafficPlan:            true,
		ShowNodeTags:               true,
		MaskIPAddresses:            false,
		SiteName:                   "Rivo Monitor",
		SiteDescription:            "Private infrastructure monitor",
		SiteAvatarURL:              "/rivo-logo.png",
		UserAvatarURL:              "",
		HomeBackgroundURL:          "",
		ActiveTheme:                "default",
		AdminPath:                  "",
		SnapshotEnabled:            false,
		SnapshotCollectProcesses:   true,
		SnapshotCollectConnections: true,
		SnapshotMaskSensitive:      true,
		SnapshotIntervalSeconds:    60,
		SnapshotProcessLimit:       20,
		SnapshotConnectionLimit:    200,
		MetricsRetentionMonths:     retention.DefaultTelemetryRetentionMonths,
		AssetBaseCurrency:          "CNY",
		ExchangeRateAutoUpdate:     true,
		WeComWebhookEnabled:        false,
		WeComWebhookURL:            "",
		TelegramEnabled:            false,
		TelegramBotToken:           "",
		TelegramChatID:             "",
		EmailEnabled:               false,
		EmailSMTPHost:              "",
		EmailSMTPPort:              587,
		EmailSMTPSecurity:          notify.EmailSecuritySTARTTLS,
		EmailSMTPUsername:          "",
		EmailSMTPPassword:          "",
		EmailFrom:                  "",
		EmailTo:                    "",
		TrafficAlertEnabled:        true,
		TrafficAlertPercent:        80,
		CPUAlertEnabled:            true,
		CPUAlertPercent:            85,
		MemoryAlertEnabled:         true,
		MemoryAlertPercent:         85,
		DiskLoadAlertEnabled:       true,
		DiskLoadAlertPercent:       90,
		LoadAlertEnabled:           true,
		LoadAlertThreshold:         5,
		AlertIntervalMinutes:       30,
		OfflineAlertDelayMinutes:   1,
		ExpiryAlertEnabled:         true,
		ExpiryAlertDays:            7,
	}
}

func loadAppSettings(db *gorm.DB) appSettings {
	settings := defaultAppSettings()
	var rows []model.AppSetting
	if err := db.Find(&rows).Error; err != nil {
		return settings
	}

	for _, row := range rows {
		value := strings.EqualFold(row.Value, "true")
		switch row.Key {
		case "show_home_summary":
			settings.ShowHomeSummary = value
		case "show_billing_details":
			settings.ShowBillingDetails = value
		case "show_traffic_plan":
			settings.ShowTrafficPlan = value
		case "show_node_tags":
			settings.ShowNodeTags = value
		case "mask_ip_addresses":
			settings.MaskIPAddresses = value
		case "site_name":
			settings.SiteName = strings.TrimSpace(row.Value)
		case "site_description":
			settings.SiteDescription = strings.TrimSpace(row.Value)
		case "site_avatar_url":
			if value := strings.TrimSpace(row.Value); value != "" {
				settings.SiteAvatarURL = value
			}
		case "user_avatar_url":
			settings.UserAvatarURL = strings.TrimSpace(row.Value)
		case "home_background_url":
			settings.HomeBackgroundURL = strings.TrimSpace(row.Value)
		case "active_theme":
			settings.ActiveTheme = normalizeThemeID(row.Value)
			if settings.ActiveTheme == "" {
				settings.ActiveTheme = "default"
			}
		case "admin_path":
			settings.AdminPath = config.NormalizeAdminPath(row.Value)
		case "snapshot_enabled":
			settings.SnapshotEnabled = value
		case "snapshot_collect_processes":
			settings.SnapshotCollectProcesses = value
		case "snapshot_collect_connections":
			settings.SnapshotCollectConnections = value
		case "snapshot_mask_sensitive":
			settings.SnapshotMaskSensitive = value
		case "snapshot_interval_seconds":
			settings.SnapshotIntervalSeconds = parseSettingInt(row.Value, settings.SnapshotIntervalSeconds)
		case "snapshot_process_limit":
			settings.SnapshotProcessLimit = parseSettingInt(row.Value, settings.SnapshotProcessLimit)
		case "snapshot_connection_limit":
			settings.SnapshotConnectionLimit = parseSettingInt(row.Value, settings.SnapshotConnectionLimit)
		case retention.MetricRetentionSettingKey:
			settings.MetricsRetentionMonths = retention.ParseTelemetryRetentionMonths(row.Value, settings.MetricsRetentionMonths)
		case "asset_base_currency":
			settings.AssetBaseCurrency = normalizeAssetCurrency(row.Value)
		case "exchange_rate_auto_update":
			settings.ExchangeRateAutoUpdate = value
		case "wecom_webhook_enabled":
			settings.WeComWebhookEnabled = value
		case "wecom_webhook_url":
			settings.WeComWebhookURL = strings.TrimSpace(row.Value)
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
			settings.AlertIntervalMinutes = parseSettingInt(row.Value, settings.AlertIntervalMinutes)
		case "offline_alert_delay_minutes":
			settings.OfflineAlertDelayMinutes = parseSettingInt(row.Value, settings.OfflineAlertDelayMinutes)
		case "expiry_alert_enabled":
			settings.ExpiryAlertEnabled = value
		case "expiry_alert_days":
			settings.ExpiryAlertDays = parseSettingInt(row.Value, settings.ExpiryAlertDays)
		}
	}
	settings.EmailSMTPSecurity = notify.NormalizeEmailSecurity(settings.EmailSMTPSecurity)
	if settings.EmailSMTPPort == 0 {
		settings.EmailSMTPPort = defaultAppSettings().EmailSMTPPort
	}
	return settings
}

func loadAdminAppSettings(db *gorm.DB, fallbackAdminPath string) appSettings {
	settings := loadAppSettings(db)
	settings.AdminPath = effectiveAdminPathFromValue(settings.AdminPath, fallbackAdminPath)
	return settings
}

func EffectiveAdminPath(db *gorm.DB, fallbackAdminPath string) string {
	return effectiveAdminPath(db, fallbackAdminPath)
}

func effectiveAdminPath(db *gorm.DB, fallbackAdminPath string) string {
	return effectiveAdminPathFromValue(loadAppSettings(db).AdminPath, fallbackAdminPath)
}

func effectiveAdminPathFromValue(value string, fallbackAdminPath string) string {
	value = config.NormalizeAdminPath(value)
	if config.ValidateAdminPath(value) == nil {
		return value
	}
	fallbackAdminPath = config.NormalizeAdminPath(fallbackAdminPath)
	if config.ValidateAdminPath(fallbackAdminPath) == nil {
		return fallbackAdminPath
	}
	return ""
}

func saveAppSettings(db *gorm.DB, settings appSettings) error {
	boolValues := map[string]bool{
		"show_home_summary":            settings.ShowHomeSummary,
		"show_billing_details":         settings.ShowBillingDetails,
		"show_traffic_plan":            settings.ShowTrafficPlan,
		"show_node_tags":               settings.ShowNodeTags,
		"mask_ip_addresses":            settings.MaskIPAddresses,
		"snapshot_enabled":             settings.SnapshotEnabled,
		"snapshot_collect_processes":   settings.SnapshotCollectProcesses,
		"snapshot_collect_connections": settings.SnapshotCollectConnections,
		"snapshot_mask_sensitive":      settings.SnapshotMaskSensitive,
		"exchange_rate_auto_update":    settings.ExchangeRateAutoUpdate,
		"wecom_webhook_enabled":        settings.WeComWebhookEnabled,
		"telegram_enabled":             settings.TelegramEnabled,
		"email_enabled":                settings.EmailEnabled,
		"traffic_alert_enabled":        settings.TrafficAlertEnabled,
		"cpu_alert_enabled":            settings.CPUAlertEnabled,
		"memory_alert_enabled":         settings.MemoryAlertEnabled,
		"disk_load_alert_enabled":      settings.DiskLoadAlertEnabled,
		"load_alert_enabled":           settings.LoadAlertEnabled,
		"expiry_alert_enabled":         settings.ExpiryAlertEnabled,
	}
	for key, value := range boolValues {
		row := model.AppSetting{Key: key, Value: strconv.FormatBool(value)}
		if err := db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "key"}},
			DoUpdates: clause.Assignments(map[string]any{"value": row.Value, "updated_at": time.Now()}),
		}).Create(&row).Error; err != nil {
			return err
		}
	}
	stringValues := map[string]string{
		"site_name":                         settings.SiteName,
		"site_description":                  strings.TrimSpace(settings.SiteDescription),
		"site_avatar_url":                   strings.TrimSpace(settings.SiteAvatarURL),
		"user_avatar_url":                   strings.TrimSpace(settings.UserAvatarURL),
		"home_background_url":               strings.TrimSpace(settings.HomeBackgroundURL),
		"active_theme":                      normalizeThemeID(settings.ActiveTheme),
		"admin_path":                        config.NormalizeAdminPath(settings.AdminPath),
		"asset_base_currency":               normalizeAssetCurrency(settings.AssetBaseCurrency),
		"wecom_webhook_url":                 strings.TrimSpace(settings.WeComWebhookURL),
		"telegram_bot_token":                strings.TrimSpace(settings.TelegramBotToken),
		"telegram_chat_id":                  strings.TrimSpace(settings.TelegramChatID),
		"email_smtp_host":                   strings.TrimSpace(settings.EmailSMTPHost),
		"email_smtp_port":                   strconv.Itoa(settings.EmailSMTPPort),
		"email_smtp_security":               notify.NormalizeEmailSecurity(settings.EmailSMTPSecurity),
		"email_smtp_username":               strings.TrimSpace(settings.EmailSMTPUsername),
		"email_smtp_password":               strings.TrimSpace(settings.EmailSMTPPassword),
		"email_from":                        strings.TrimSpace(settings.EmailFrom),
		"email_to":                          strings.TrimSpace(settings.EmailTo),
		"snapshot_interval_seconds":         strconv.Itoa(settings.SnapshotIntervalSeconds),
		"snapshot_process_limit":            strconv.Itoa(settings.SnapshotProcessLimit),
		"snapshot_connection_limit":         strconv.Itoa(settings.SnapshotConnectionLimit),
		retention.MetricRetentionSettingKey: strconv.Itoa(retention.NormalizeTelemetryRetentionMonths(settings.MetricsRetentionMonths)),
		"traffic_alert_percent":             formatSettingFloat(settings.TrafficAlertPercent),
		"cpu_alert_percent":                 formatSettingFloat(settings.CPUAlertPercent),
		"memory_alert_percent":              formatSettingFloat(settings.MemoryAlertPercent),
		"disk_load_alert_percent":           formatSettingFloat(settings.DiskLoadAlertPercent),
		"load_alert_threshold":              formatSettingFloat(settings.LoadAlertThreshold),
		"alert_interval_minutes":            strconv.Itoa(settings.AlertIntervalMinutes),
		"offline_alert_delay_minutes":       strconv.Itoa(settings.OfflineAlertDelayMinutes),
		"expiry_alert_days":                 strconv.Itoa(settings.ExpiryAlertDays),
	}
	for key, value := range stringValues {
		row := model.AppSetting{Key: key, Value: value}
		if err := db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "key"}},
			DoUpdates: clause.Assignments(map[string]any{"value": row.Value, "updated_at": time.Now()}),
		}).Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}

func saveAppSetting(db *gorm.DB, key string, value string) error {
	row := model.AppSetting{Key: key, Value: value}
	return db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.Assignments(map[string]any{"value": row.Value, "updated_at": time.Now()}),
	}).Create(&row).Error
}

func normalizeAppSettingsForSave(settings appSettings) appSettings {
	defaults := defaultAppSettings()
	settings.SiteName = strings.TrimSpace(settings.SiteName)
	settings.SiteDescription = strings.TrimSpace(settings.SiteDescription)
	settings.SiteAvatarURL = strings.TrimSpace(settings.SiteAvatarURL)
	if settings.SiteAvatarURL == "" {
		settings.SiteAvatarURL = defaults.SiteAvatarURL
	}
	settings.UserAvatarURL = strings.TrimSpace(settings.UserAvatarURL)
	settings.HomeBackgroundURL = strings.TrimSpace(settings.HomeBackgroundURL)
	settings.ActiveTheme = normalizeThemeID(settings.ActiveTheme)
	if settings.ActiveTheme == "" {
		settings.ActiveTheme = defaults.ActiveTheme
	}
	settings.AdminPath = config.NormalizeAdminPath(settings.AdminPath)
	if strings.TrimSpace(settings.AssetBaseCurrency) == "" {
		settings.AssetBaseCurrency = defaults.AssetBaseCurrency
	}
	settings.AssetBaseCurrency = normalizeAssetCurrency(settings.AssetBaseCurrency)
	settings.WeComWebhookURL = strings.TrimSpace(settings.WeComWebhookURL)
	settings.TelegramBotToken = strings.TrimSpace(settings.TelegramBotToken)
	settings.TelegramChatID = strings.TrimSpace(settings.TelegramChatID)
	settings.EmailSMTPHost = strings.TrimSpace(settings.EmailSMTPHost)
	settings.EmailSMTPSecurity = notify.NormalizeEmailSecurity(settings.EmailSMTPSecurity)
	settings.EmailSMTPUsername = strings.TrimSpace(settings.EmailSMTPUsername)
	settings.EmailSMTPPassword = strings.TrimSpace(settings.EmailSMTPPassword)
	settings.EmailFrom = strings.TrimSpace(settings.EmailFrom)
	settings.EmailTo = strings.TrimSpace(settings.EmailTo)
	if settings.EmailSMTPPort == 0 {
		settings.EmailSMTPPort = defaults.EmailSMTPPort
	}
	if settings.TrafficAlertPercent < 0 {
		settings.TrafficAlertPercent = defaults.TrafficAlertPercent
	}
	if settings.CPUAlertPercent < 0 {
		settings.CPUAlertPercent = defaults.CPUAlertPercent
	}
	if settings.MemoryAlertPercent < 0 {
		settings.MemoryAlertPercent = defaults.MemoryAlertPercent
	}
	if settings.DiskLoadAlertPercent < 0 {
		settings.DiskLoadAlertPercent = defaults.DiskLoadAlertPercent
	}
	if settings.LoadAlertThreshold < 0 {
		settings.LoadAlertThreshold = defaults.LoadAlertThreshold
	}
	if settings.SnapshotIntervalSeconds == 0 {
		settings.SnapshotIntervalSeconds = defaults.SnapshotIntervalSeconds
	}
	if settings.SnapshotProcessLimit == 0 {
		settings.SnapshotProcessLimit = defaults.SnapshotProcessLimit
	}
	if settings.SnapshotConnectionLimit == 0 {
		settings.SnapshotConnectionLimit = defaults.SnapshotConnectionLimit
	}
	settings.MetricsRetentionMonths = retention.NormalizeTelemetryRetentionMonths(settings.MetricsRetentionMonths)
	if settings.AlertIntervalMinutes == 0 {
		settings.AlertIntervalMinutes = defaults.AlertIntervalMinutes
	}
	if settings.OfflineAlertDelayMinutes < 0 {
		settings.OfflineAlertDelayMinutes = defaults.OfflineAlertDelayMinutes
	}
	if settings.ExpiryAlertDays == 0 {
		settings.ExpiryAlertDays = defaults.ExpiryAlertDays
	}
	return settings
}

func validateAppSettings(settings appSettings) string {
	if message := validateTextField("site name", settings.SiteName, 40, false); message != "" {
		return message
	}
	if message := validateTextField("site description", settings.SiteDescription, 80, true); message != "" {
		return message
	}
	if message := validateImageURL("site avatar url", settings.SiteAvatarURL); message != "" {
		return message
	}
	if message := validateImageURL("user avatar url", settings.UserAvatarURL); message != "" {
		return message
	}
	if message := validateImageURL("home background url", settings.HomeBackgroundURL); message != "" {
		return message
	}
	if settings.ActiveTheme == "" {
		return "active theme is required"
	}
	if err := config.ValidateAdminPath(settings.AdminPath); err != nil {
		return err.Error()
	}
	webhookURL := strings.TrimSpace(settings.WeComWebhookURL)
	if message := validateWeComWebhookURL(webhookURL, settings.WeComWebhookEnabled); message != "" {
		return message
	}
	if message := validateTelegramSettings(settings.TelegramBotToken, settings.TelegramChatID, settings.TelegramEnabled); message != "" {
		return message
	}
	if message := notify.ValidateEmailSettings(emailSettingsFromAppSettings(settings), settings.EmailEnabled); message != "" {
		return message
	}
	if !validPercent(settings.TrafficAlertPercent) {
		return "traffic alert percent must be between 0 and 100"
	}
	if !validPercent(settings.CPUAlertPercent) {
		return "cpu alert percent must be between 0 and 100"
	}
	if !validPercent(settings.MemoryAlertPercent) {
		return "memory alert percent must be between 0 and 100"
	}
	if !validPercent(settings.DiskLoadAlertPercent) {
		return "disk load alert percent must be between 0 and 100"
	}
	if math.IsNaN(settings.LoadAlertThreshold) || math.IsInf(settings.LoadAlertThreshold, 0) || settings.LoadAlertThreshold < 0 || settings.LoadAlertThreshold > 100 {
		return "load alert threshold must be between 0 and 100"
	}
	if !validAlertInterval(settings.AlertIntervalMinutes) {
		return "alert interval must be one of 5, 10, 30, 60, 240, 720, 1440 minutes"
	}
	if settings.OfflineAlertDelayMinutes < 0 || settings.OfflineAlertDelayMinutes > 1440 {
		return "offline alert delay must be between 0 and 1440 minutes"
	}
	if settings.ExpiryAlertDays < 1 || settings.ExpiryAlertDays > 366 {
		return "expiry alert days must be between 1 and 366"
	}
	if settings.SnapshotIntervalSeconds < 15 || settings.SnapshotIntervalSeconds > 3600 {
		return "snapshot interval must be between 15 and 3600 seconds"
	}
	if settings.SnapshotProcessLimit < 1 || settings.SnapshotProcessLimit > 50 {
		return "snapshot process limit must be between 1 and 50"
	}
	if settings.SnapshotConnectionLimit < 1 || settings.SnapshotConnectionLimit > 500 {
		return "snapshot connection limit must be between 1 and 500"
	}
	if !retention.ValidTelemetryRetentionMonths(settings.MetricsRetentionMonths) {
		return "metrics retention months must be one of 1, 3, 6, 12"
	}
	if !isAllowedValue(settings.AssetBaseCurrency, "CNY", "USD") {
		return "asset base currency must be CNY or USD"
	}
	return ""
}

func validateWeComWebhookURL(webhookURL string, required bool) string {
	webhookURL = strings.TrimSpace(webhookURL)
	if required && webhookURL == "" {
		return "wecom webhook url is required"
	}
	if webhookURL == "" {
		return ""
	}
	parsed, err := url.ParseRequestURI(webhookURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "invalid wecom webhook url"
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "wecom webhook url must use http or https"
	}
	return ""
}

func validateTelegramSettings(botToken string, chatID string, required bool) string {
	botToken = strings.TrimSpace(botToken)
	chatID = strings.TrimSpace(chatID)
	if required && botToken == "" {
		return "telegram bot token is required"
	}
	if required && chatID == "" {
		return "telegram chat id is required"
	}
	if botToken == "" && chatID == "" {
		return ""
	}
	if botToken == "" || chatID == "" {
		return "telegram bot token and chat id must be configured together"
	}
	if len(botToken) > 256 {
		return "telegram bot token is too long"
	}
	if len(chatID) > 128 {
		return "telegram chat id is too long"
	}
	if strings.ContainsAny(botToken, "/?#") || hasControlChars(botToken) {
		return "invalid telegram bot token"
	}
	if hasControlChars(chatID) {
		return "invalid telegram chat id"
	}
	return ""
}

func emailSettingsFromAppSettings(settings appSettings) notify.EmailSettings {
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstNonZero(values ...int) int {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func postWeComWebhook(webhookURL string, content string) error {
	body, err := json.Marshal(map[string]any{
		"msgtype": "text",
		"text": map[string]string{
			"content": content,
		},
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, strings.TrimSpace(webhookURL), bytes.NewReader(body))
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
		return fmt.Errorf("wecom webhook returned %s", resp.Status)
	}
	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err == nil && result.ErrCode != 0 {
		if result.ErrMsg == "" {
			result.ErrMsg = "unknown wecom error"
		}
		return fmt.Errorf("%s", result.ErrMsg)
	}
	return nil
}

func postTelegramMessage(botToken string, chatID string, content string) error {
	endpoint := "https://api.telegram.org/bot" + strings.TrimSpace(botToken) + "/sendMessage"
	form := url.Values{}
	form.Set("chat_id", strings.TrimSpace(chatID))
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
			return fmt.Errorf("telegram api returned %s: %s", resp.Status, result.Description)
		}
		return fmt.Errorf("telegram api returned %s", resp.Status)
	}
	if decodeErr == nil && !result.OK {
		if result.Description == "" {
			result.Description = "unknown telegram error"
		}
		return fmt.Errorf("%s", result.Description)
	}
	return nil
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

func formatSettingFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func validPercent(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0 && value <= 100
}

func validAlertInterval(value int) bool {
	for _, allowed := range []int{5, 10, 30, 60, 240, 720, 1440} {
		if value == allowed {
			return true
		}
	}
	return false
}

func normalizeAssetCurrency(value string) string {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	if strings.Contains(normalized, "USD") {
		return "USD"
	}
	return "CNY"
}

type exchangeAPIResponse struct {
	Rates map[string]float64 `json:"rates"`
}

func loadExchangeRates(baseCurrency string, autoUpdate bool) (map[string]float64, string, int64) {
	base := normalizeAssetCurrency(baseCurrency)
	fallback := fallbackExchangeRates(base)
	if !autoUpdate {
		return fallback, "fallback", time.Now().UnixMilli()
	}

	rates, err := fetchLiveExchangeRates(base)
	if err != nil {
		return fallback, "fallback", time.Now().UnixMilli()
	}
	for currency, fallbackRate := range fallback {
		if rates[currency] <= 0 {
			rates[currency] = fallbackRate
		}
	}
	return rates, "frankfurter", time.Now().UnixMilli()
}

func fetchLiveExchangeRates(base string) (map[string]float64, error) {
	targets := make([]string, 0, len(supportedAssetCurrencies())-1)
	for _, currency := range supportedAssetCurrencies() {
		if currency != base {
			targets = append(targets, currency)
		}
	}
	endpoint := "https://api.frankfurter.app/latest?from=" + url.QueryEscape(base) + "&to=" + url.QueryEscape(strings.Join(targets, ","))
	client := http.Client{Timeout: 4 * time.Second}
	resp, err := client.Get(endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, http.ErrBodyNotAllowed
	}

	var parsed exchangeAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}
	rates := map[string]float64{base: 1}
	for _, currency := range supportedAssetCurrencies() {
		if currency == base {
			rates[currency] = 1
			continue
		}
		if parsed.Rates[currency] > 0 {
			rates[currency] = 1 / parsed.Rates[currency]
		}
	}
	return rates, nil
}

func fallbackExchangeRates(base string) map[string]float64 {
	cnyRates := map[string]float64{
		"CNY": 1,
		"USD": 7.2,
		"EUR": 7.8,
		"GBP": 9.1,
		"HKD": 0.92,
		"JPY": 0.05,
	}
	base = normalizeAssetCurrency(base)
	baseRate := cnyRates[base]
	if baseRate <= 0 {
		baseRate = 1
	}
	rates := make(map[string]float64, len(cnyRates))
	for currency, cnyRate := range cnyRates {
		rates[currency] = cnyRate / baseRate
	}
	return rates
}

func supportedAssetCurrencies() []string {
	return []string{"CNY", "USD", "EUR", "GBP", "HKD", "JPY"}
}

func defaultRegionOptions() []model.RegionOption {
	return []model.RegionOption{
		{Code: "default", Name: "默认", SortOrder: 0, Enabled: true},
		{Code: "HK", Name: "香港", SortOrder: 10, Enabled: true},
		{Code: "US", Name: "美国", SortOrder: 20, Enabled: true},
		{Code: "JP", Name: "日本", SortOrder: 30, Enabled: true},
		{Code: "SG", Name: "新加坡", SortOrder: 40, Enabled: true},
		{Code: "TW", Name: "台湾", SortOrder: 50, Enabled: true},
		{Code: "KR", Name: "韩国", SortOrder: 60, Enabled: true},
		{Code: "CN", Name: "中国", SortOrder: 70, Enabled: true},
		{Code: "DE", Name: "德国", SortOrder: 80, Enabled: true},
		{Code: "FR", Name: "法国", SortOrder: 90, Enabled: true},
		{Code: "GB", Name: "英国", SortOrder: 100, Enabled: true},
		{Code: "NL", Name: "荷兰", SortOrder: 110, Enabled: true},
		{Code: "CA", Name: "加拿大", SortOrder: 120, Enabled: true},
		{Code: "AU", Name: "澳大利亚", SortOrder: 130, Enabled: true},
		{Code: "AE", Name: "阿联酋", SortOrder: 140, Enabled: true},
		{Code: "TH", Name: "泰国", SortOrder: 150, Enabled: true},
		{Code: "VN", Name: "越南", SortOrder: 160, Enabled: true},
		{Code: "IN", Name: "印度", SortOrder: 170, Enabled: true},
		{Code: "ID", Name: "印尼", SortOrder: 180, Enabled: true},
		{Code: "MY", Name: "马来西亚", SortOrder: 190, Enabled: true},
		{Code: "PH", Name: "菲律宾", SortOrder: 200, Enabled: true},
		{Code: "BR", Name: "巴西", SortOrder: 210, Enabled: true},
		{Code: "RU", Name: "俄罗斯", SortOrder: 220, Enabled: true},
		{Code: "TR", Name: "土耳其", SortOrder: 230, Enabled: true},
	}
}

func ensureDefaultRegionOptions(logger *slog.Logger, db *gorm.DB) {
	options := defaultRegionOptions()
	if len(options) == 0 {
		return
	}
	if err := db.Model(&model.RegionOption{}).
		Where("LOWER(code) = ?", "auto").
		Updates(map[string]any{"name": "默认", "enabled": false}).Error; err != nil {
		logger.Warn("disable legacy auto region option failed", slog.String("error", err.Error()))
	}
	if err := db.Model(&model.Node{}).
		Where("LOWER(region) = ?", "auto").
		Update("region", "default").Error; err != nil {
		logger.Warn("migrate legacy auto node region failed", slog.String("error", err.Error()))
	}
	if err := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "code"}},
		DoNothing: true,
	}).Create(&options).Error; err != nil {
		logger.Warn("seed region options failed", slog.String("error", err.Error()))
	}
}

func loadRegionOptions(db *gorm.DB) []regionOptionResponse {
	var rows []model.RegionOption
	if err := db.Where("enabled = ?", true).Order("sort_order asc, id asc").Find(&rows).Error; err != nil || len(rows) == 0 {
		rows = defaultRegionOptions()
	}

	options := make([]regionOptionResponse, 0, len(rows))
	for _, row := range rows {
		options = append(options, regionOptionResponse{
			Label: regionLabel(row),
			Value: row.Code,
		})
	}
	return options
}

func regionLabel(region model.RegionOption) string {
	if strings.EqualFold(region.Code, "default") || strings.EqualFold(region.Code, "auto") {
		return region.Name
	}
	return region.Name + " " + region.Code
}

func isRegionCodeAllowed(db *gorm.DB, code string) bool {
	normalized := normalizeRegionCode(code)
	var count int64
	if err := db.Model(&model.RegionOption{}).
		Where("code = ? AND enabled = ?", normalized, true).
		Count(&count).Error; err != nil {
		return false
	}
	return count > 0
}

func normalizeRegionCode(code string) string {
	normalized := strings.TrimSpace(code)
	if normalized == "" || strings.EqualFold(normalized, "auto") || strings.EqualFold(normalized, "default") {
		return "default"
	}
	return strings.ToUpper(normalized)
}

func probeTaskFromRequest(req probeTaskRequest, defaultEnabled bool) model.ProbeTask {
	enabled := defaultEnabled
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	task := model.ProbeTask{
		Name:            strings.TrimSpace(req.Name),
		Type:            strings.ToLower(strings.TrimSpace(req.Type)),
		IPVersion:       normalizeProbeIPVersion(req.IPVersion),
		Target:          strings.TrimSpace(req.Target),
		IntervalSeconds: req.IntervalSeconds,
		TimeoutMS:       req.TimeoutMS,
		Enabled:         enabled,
	}
	if task.Name == "" {
		task.Name = task.Target
	}
	if task.IntervalSeconds == 0 {
		task.IntervalSeconds = 60
	}
	if task.TimeoutMS == 0 {
		task.TimeoutMS = 3000
	}
	return task
}

func validateProbeTaskRequest(req probeTaskRequest) string {
	if message := validateTextField("probe name", req.Name, 64, true); message != "" {
		return message
	}
	probeType := strings.ToLower(strings.TrimSpace(req.Type))
	if !isAllowedValue(probeType, "tcp_ping", "icmp") {
		return "invalid probe type"
	}
	if !isAllowedValue(normalizeProbeIPVersion(req.IPVersion), "auto", "ipv4", "ipv6") {
		return "invalid ip version"
	}
	if strings.TrimSpace(req.Target) == "" {
		return "probe target is required"
	}
	if message := validateTextField("probe target", req.Target, 255, false); message != "" {
		return message
	}
	if req.IntervalSeconds > 0 && req.IntervalSeconds < 3 {
		return "probe interval must be at least 3 seconds"
	}
	if req.TimeoutMS > 0 && (req.TimeoutMS < 100 || req.TimeoutMS > 30000) {
		return "probe timeout must be between 100 and 30000 ms"
	}
	return ""
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

func publishAllNodeConfigs(logger *slog.Logger, db *gorm.DB, publishers []ConfigPublisher) {
	if len(publishers) == 0 {
		return
	}
	var nodes []model.Node
	if err := db.Select("node_id").Find(&nodes).Error; err != nil {
		logger.Warn("list nodes for config publish failed", slog.String("error", err.Error()))
		return
	}
	for _, node := range nodes {
		if err := publishers[0].PublishConfig(node.NodeID); err != nil {
			logger.Warn("publish agent config failed", slog.String("node_id", node.NodeID), slog.String("error", err.Error()))
		}
	}
}

func listAssignedProbeTasks(db *gorm.DB, nodeID string) ([]model.ProbeTask, error) {
	return listProbeTasksForNode(db, nodeID, true)
}

func listProbeTasksForNode(db *gorm.DB, nodeID string, includeInactive bool) ([]model.ProbeTask, error) {
	var tasks []model.ProbeTask
	query := db.Table("probe_tasks").
		Select("probe_tasks.*").
		Joins("JOIN probe_task_assignments ON probe_task_assignments.task_id = probe_tasks.id").
		Where("probe_task_assignments.node_id = ?", nodeID)
	if !includeInactive {
		query = query.Where("probe_tasks.enabled = ?", true)
	}
	err := query.
		Order("probe_task_assignments.id asc, probe_tasks.id asc").
		Scan(&tasks).Error
	return tasks, err
}

func activeProbeTaskIDs(tasks []model.ProbeTask) []uint64 {
	ids := make([]uint64, 0, len(tasks))
	for _, task := range tasks {
		if task.Enabled {
			ids = append(ids, task.ID)
		}
	}
	return ids
}

type dashboardProbeAggregateRow struct {
	NodeID         string   `gorm:"column:node_id"`
	Samples        int64    `gorm:"column:samples"`
	SuccessSamples int64    `gorm:"column:success_samples"`
	AvgLatencyMS   *float64 `gorm:"column:avg_latency_ms"`
	PacketLoss     *float64 `gorm:"column:packet_loss"`
}

func buildDashboardSummary(db *gorm.DB) (DashboardSummary, error) {
	var nodes []model.Node
	if err := db.Find(&nodes).Error; err != nil {
		return DashboardSummary{}, err
	}

	now := time.Now()
	total := int64(len(nodes))
	var online, offline int64
	nodeIDs := make([]string, 0, len(nodes))
	for _, node := range nodes {
		nodeIDs = append(nodeIDs, node.NodeID)
		switch node.Status {
		case "online":
			online++
		case "offline":
			offline++
		}
	}

	activeAlertCounts, criticalAlertCounts, currentAlerts, criticalAlerts, err := dashboardActiveAlertCounts(db)
	if err != nil {
		return DashboardSummary{}, err
	}

	probeSince := now.Add(-24 * time.Hour)
	baselineSince := now.Add(-7 * 24 * time.Hour)
	nodeProbeStats, avgLatency, availability, probeSamples, err := dashboardProbeStats(db, probeSince, baselineSince)
	if err != nil {
		return DashboardSummary{}, err
	}

	sparklineSince := uint64(now.Add(-time.Hour).UnixMilli())
	sparklines, err := dashboardSparklines(db, nodeIDs, sparklineSince)
	if err != nil {
		return DashboardSummary{}, err
	}
	latestMetrics := dashboardLatestMetrics(db, nodes)
	nodeHealthScores := dashboardNodeHealthScores(nodes, latestMetrics, nodeProbeStats, sparklines, activeAlertCounts, criticalAlertCounts, now)
	clusterHealthScore := dashboardClusterHealthScore(nodes, nodeHealthScores, criticalAlerts)

	return DashboardSummary{
		NodesTotal:          total,
		NodesOnline:         online,
		NodesOffline:        offline,
		ClusterHealthScore:  clusterHealthScore,
		AvgLatencyMS:        avgLatency,
		AvailabilityPercent: availability,
		ProbeSamples:        probeSamples,
		CurrentAlerts:       currentAlerts,
		NodeSparklines:      sparklines,
		NodeProbeStats:      nodeProbeStats,
		NodeHealthScores:    nodeHealthScores,
	}, nil
}

func buildDashboardEvents(db *gorm.DB, limit int) ([]DashboardEvent, error) {
	if limit <= 0 {
		limit = 24
	}

	nodeNames, err := dashboardEventNodeNames(db)
	if err != nil {
		return nil, err
	}

	queryLimit := limit * 3
	if queryLimit < 24 {
		queryLimit = 24
	}

	var logs []model.SystemLog
	eventTypes := []string{"agent.online", "agent.offline", "alert.triggered", "metrics.refresh_requested"}
	if err := db.Where("event_type IN ?", eventTypes).Order("created_at desc").Limit(queryLimit).Find(&logs).Error; err != nil {
		return nil, err
	}

	var metrics []model.NodeMetric
	if err := db.Order("ts desc").Limit(queryLimit).Find(&metrics).Error; err != nil {
		return nil, err
	}

	events := make([]DashboardEvent, 0, len(logs)+len(metrics))
	for _, log := range logs {
		events = append(events, DashboardEvent{
			ID:        "log-" + strconv.FormatUint(log.ID, 10),
			EventType: log.EventType,
			Level:     log.Level,
			NodeID:    log.NodeID,
			NodeName:  dashboardEventNodeName(nodeNames, log.NodeID),
			Message:   dashboardSystemLogMessage(log),
			CreatedAt: log.CreatedAt.UnixMilli(),
		})
	}

	for _, metric := range metrics {
		createdAt := normalizeDashboardEventTimestamp(metric.Timestamp, metric.CreatedAt)
		events = append(events, DashboardEvent{
			ID:        "metric-" + strconv.FormatUint(metric.ID, 10),
			EventType: "metric.reported",
			Level:     "info",
			NodeID:    metric.NodeID,
			NodeName:  dashboardEventNodeName(nodeNames, metric.NodeID),
			Message:   fmt.Sprintf("上报指标，CPU %.1f%% · MEM %.1f%% · DISK %.1f%%。", metric.CPUUsage, metric.MemUsedPercent, metric.DiskUsedPercent),
			CreatedAt: createdAt,
			Metric: &DashboardEventMetric{
				CPUUsage:        metric.CPUUsage,
				MemUsedPercent:  metric.MemUsedPercent,
				DiskUsedPercent: metric.DiskUsedPercent,
				NetRxBps:        metric.NetRxBps,
				NetTxBps:        metric.NetTxBps,
			},
		})
	}

	sort.Slice(events, func(i, j int) bool {
		if events[i].CreatedAt == events[j].CreatedAt {
			return events[i].ID > events[j].ID
		}
		return events[i].CreatedAt > events[j].CreatedAt
	})
	if len(events) > limit {
		events = events[:limit]
	}
	return events, nil
}

func dashboardEventNodeNames(db *gorm.DB) (map[string]string, error) {
	var nodes []model.Node
	if err := db.Select("node_id, name").Find(&nodes).Error; err != nil {
		return nil, err
	}
	names := make(map[string]string, len(nodes))
	for _, node := range nodes {
		names[node.NodeID] = strings.TrimSpace(node.Name)
	}
	return names, nil
}

func dashboardEventNodeName(names map[string]string, nodeID string) string {
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		return ""
	}
	if name := strings.TrimSpace(names[nodeID]); name != "" {
		return name
	}
	return nodeID
}

func dashboardSystemLogMessage(log model.SystemLog) string {
	switch log.EventType {
	case "agent.online":
		return "Agent 已上线，心跳链路已建立。"
	case "agent.offline":
		return "Agent 已离线，心跳链路断开。"
	case "alert.triggered":
		message := strings.TrimSpace(strings.TrimPrefix(log.Message, "alert triggered:"))
		if message == "" {
			message = "触发告警"
		}
		return "告警触发：" + message
	case "metrics.refresh_requested":
		return "已下发立即上报指标请求。"
	default:
		return strings.TrimSpace(log.Message)
	}
}

func normalizeDashboardEventTimestamp(value uint64, fallback time.Time) int64 {
	if value == 0 {
		return fallback.UnixMilli()
	}
	if value < 1000000000000 {
		return int64(value * 1000)
	}
	return int64(value)
}

func dashboardProbeStats(db *gorm.DB, since time.Time, baselineSince time.Time) (map[string]DashboardNodeProbeStat, *float64, *float64, int64, error) {
	var rows []dashboardProbeAggregateRow
	err := db.Raw(`
SELECT
  probe_results.node_id AS node_id,
  COUNT(*) AS samples,
  SUM(CASE WHEN probe_results.status = 'success' AND probe_results.latency_ms IS NOT NULL THEN 1 ELSE 0 END) AS success_samples,
  AVG(CASE WHEN probe_results.status = 'success' AND probe_results.latency_ms IS NOT NULL THEN probe_results.latency_ms ELSE NULL END) AS avg_latency_ms,
  AVG(COALESCE(probe_results.packet_loss, CASE WHEN probe_results.status = 'success' THEN 0 ELSE 100 END)) AS packet_loss
FROM probe_results
JOIN probe_task_assignments ON probe_task_assignments.node_id = probe_results.node_id AND probe_task_assignments.task_id = probe_results.task_id
JOIN probe_tasks ON probe_tasks.id = probe_results.task_id AND probe_tasks.enabled = ?
WHERE probe_results.created_at >= ?
GROUP BY probe_results.node_id
`, true, since).Scan(&rows).Error
	if err != nil {
		return nil, nil, nil, 0, err
	}

	currentLatencySamples, err := dashboardProbeLatencySamples(db, since)
	if err != nil {
		return nil, nil, nil, 0, err
	}
	baselineLatencySamples, err := dashboardProbeLatencySamples(db, baselineSince)
	if err != nil {
		return nil, nil, nil, 0, err
	}

	stats := make(map[string]DashboardNodeProbeStat, len(rows))
	var totalSamples int64
	var totalSuccess int64
	var weightedLatencyTotal float64
	var latencySamples int64
	for _, row := range rows {
		failedSamples := row.Samples - row.SuccessSamples
		if failedSamples < 0 {
			failedSamples = 0
		}
		var availability *float64
		if row.Samples > 0 {
			value := float64(row.SuccessSamples) / float64(row.Samples) * 100
			availability = &value
		}
		latencyP50 := percentileFloat64(currentLatencySamples[row.NodeID], 50)
		latencyP90 := percentileFloat64(currentLatencySamples[row.NodeID], 90)
		var jitter *float64
		if latencyP50 != nil && latencyP90 != nil {
			value := *latencyP90 - *latencyP50
			if value < 0 {
				value = 0
			}
			jitter = &value
		}
		latencyBaseline := percentileFloat64(baselineLatencySamples[row.NodeID], 20)
		var latencySpikeRatio *float64
		if latencyP50 != nil && latencyBaseline != nil && *latencyBaseline > 0 {
			value := *latencyP50 / *latencyBaseline
			latencySpikeRatio = &value
		}
		stats[row.NodeID] = DashboardNodeProbeStat{
			Samples:             row.Samples,
			SuccessSamples:      row.SuccessSamples,
			FailedSamples:       failedSamples,
			AvailabilityPercent: availability,
			AvgLatencyMS:        row.AvgLatencyMS,
			PacketLossPercent:   row.PacketLoss,
			LatencyP50MS:        latencyP50,
			LatencyP90MS:        latencyP90,
			JitterMS:            jitter,
			LatencyBaselineMS:   latencyBaseline,
			LatencySpikeRatio:   latencySpikeRatio,
		}
		totalSamples += row.Samples
		totalSuccess += row.SuccessSamples
		if row.AvgLatencyMS != nil && row.SuccessSamples > 0 {
			weightedLatencyTotal += *row.AvgLatencyMS * float64(row.SuccessSamples)
			latencySamples += row.SuccessSamples
		}
	}

	var globalLatency *float64
	if latencySamples > 0 {
		value := weightedLatencyTotal / float64(latencySamples)
		globalLatency = &value
	}
	var globalAvailability *float64
	if totalSamples > 0 {
		value := float64(totalSuccess) / float64(totalSamples) * 100
		globalAvailability = &value
	}
	return stats, globalLatency, globalAvailability, totalSamples, nil
}

type dashboardLatencySampleRow struct {
	NodeID    string  `gorm:"column:node_id"`
	LatencyMS float64 `gorm:"column:latency_ms"`
}

func dashboardProbeLatencySamples(db *gorm.DB, since time.Time) (map[string][]float64, error) {
	var rows []dashboardLatencySampleRow
	err := db.Raw(`
SELECT
  probe_results.node_id AS node_id,
  probe_results.latency_ms AS latency_ms
FROM probe_results
JOIN probe_task_assignments ON probe_task_assignments.node_id = probe_results.node_id AND probe_task_assignments.task_id = probe_results.task_id
JOIN probe_tasks ON probe_tasks.id = probe_results.task_id AND probe_tasks.enabled = ?
WHERE probe_results.created_at >= ?
  AND probe_results.status = ?
  AND probe_results.latency_ms IS NOT NULL
`, true, since, "success").Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	samples := make(map[string][]float64)
	for _, row := range rows {
		if row.LatencyMS < 0 {
			continue
		}
		samples[row.NodeID] = append(samples[row.NodeID], row.LatencyMS)
	}
	return samples, nil
}

func percentileFloat64(values []float64, percentile float64) *float64 {
	if len(values) == 0 {
		return nil
	}
	copied := append([]float64(nil), values...)
	sort.Float64s(copied)
	if percentile <= 0 {
		return float64Ptr(copied[0])
	}
	if percentile >= 100 {
		return float64Ptr(copied[len(copied)-1])
	}
	position := percentile / 100 * float64(len(copied)-1)
	lower := int(math.Floor(position))
	upper := int(math.Ceil(position))
	if lower == upper {
		return float64Ptr(copied[lower])
	}
	weight := position - float64(lower)
	value := copied[lower]*(1-weight) + copied[upper]*weight
	return &value
}

func float64Ptr(value float64) *float64 {
	return &value
}

func dashboardActiveAlertCounts(db *gorm.DB) (map[string]int64, map[string]int64, int64, int64, error) {
	var alerts []model.Alert
	if err := db.Select("node_id", "level").Where("status = ?", "active").Find(&alerts).Error; err != nil {
		return nil, nil, 0, 0, err
	}
	activeCounts := make(map[string]int64)
	criticalCounts := make(map[string]int64)
	var criticalAlerts int64
	for _, alert := range alerts {
		activeCounts[alert.NodeID]++
		if strings.EqualFold(alert.Level, "critical") {
			criticalCounts[alert.NodeID]++
			criticalAlerts++
		}
	}
	return activeCounts, criticalCounts, int64(len(alerts)), criticalAlerts, nil
}

func dashboardLatestMetrics(db *gorm.DB, nodes []model.Node) map[string]*model.NodeMetric {
	metrics := make(map[string]*model.NodeMetric, len(nodes))
	nodeIDs := make([]string, 0, len(nodes))
	for _, node := range nodes {
		nodeID := strings.TrimSpace(node.NodeID)
		if nodeID == "" {
			continue
		}
		nodeIDs = append(nodeIDs, nodeID)
	}
	if len(nodeIDs) == 0 {
		return metrics
	}

	var rows []model.NodeMetric
	if err := db.
		Raw(`
SELECT node_metrics.*
FROM node_metrics
JOIN (
  SELECT node_id, MAX(ts) AS max_ts
  FROM node_metrics
  WHERE node_id IN ?
  GROUP BY node_id
) latest ON latest.node_id = node_metrics.node_id AND latest.max_ts = node_metrics.ts
`, nodeIDs).
		Scan(&rows).Error; err != nil {
		return metrics
	}
	for index := range rows {
		metric := &rows[index]
		if existing := metrics[metric.NodeID]; existing == nil || metric.ID > existing.ID {
			metrics[metric.NodeID] = metric
		}
	}
	return metrics
}

func dashboardNodeHealthScores(nodes []model.Node, latestMetrics map[string]*model.NodeMetric, probeStats map[string]DashboardNodeProbeStat, sparklines map[string][]DashboardSparklinePoint, activeAlertCounts map[string]int64, criticalAlertCounts map[string]int64, now time.Time) map[string]DashboardNodeHealthScore {
	scores := make(map[string]DashboardNodeHealthScore, len(nodes))
	for _, node := range nodes {
		metric := latestMetrics[node.NodeID]
		probeStat := probeStats[node.NodeID]
		if node.Status != "online" {
			scores[node.NodeID] = DashboardNodeHealthScore{
				Score: 0,
				Grade: "offline",
			}
			continue
		}

		freshness := dashboardFreshnessScore(node, metric, now)
		resource := dashboardResourceScore(metric, sparklines[node.NodeID])
		load := dashboardLoadScore(metric)
		network := dashboardNetworkScore(probeStat)
		stability := dashboardStabilityScore(activeAlertCounts[node.NodeID], criticalAlertCounts[node.NodeID])
		total := clampFloat64(freshness+resource+load+network+stability, 0, 100)
		total = math.Round(total*10) / 10
		scores[node.NodeID] = DashboardNodeHealthScore{
			Score:          total,
			Grade:          dashboardHealthGrade(total),
			FreshnessScore: math.Round(freshness*10) / 10,
			ResourceScore:  math.Round(resource*10) / 10,
			LoadScore:      math.Round(load*10) / 10,
			NetworkScore:   math.Round(network*10) / 10,
			StabilityScore: math.Round(stability*10) / 10,
		}
	}
	return scores
}

func dashboardFreshnessScore(node model.Node, metric *model.NodeMetric, now time.Time) float64 {
	score := 6.0
	if metric == nil || metric.Timestamp == 0 {
		return score
	}
	intervalSeconds := int64(node.MetricsInterval)
	if intervalSeconds <= 0 {
		intervalSeconds = 15
	}
	ageMS := now.UnixMilli() - int64(metric.Timestamp)
	if ageMS < 0 {
		return score + 4
	}
	intervalMS := intervalSeconds * 1000
	if ageMS <= intervalMS*2 {
		return score + 4
	}
	if ageMS <= intervalMS*5 {
		return score + 2
	}
	return score
}

func dashboardResourceScore(metric *model.NodeMetric, points []DashboardSparklinePoint) float64 {
	if metric == nil {
		return 0
	}
	return scoreCPUUsage(metric.CPUUsage) +
		scoreMemoryUsage(metric.MemUsedPercent) +
		scoreDiskUsage(metric.DiskUsedPercent) +
		scoreSwapUsage(metric.SwapUsedPercent) +
		scoreResourceTrend(points)
}

func scoreCPUUsage(value float64) float64 {
	if value <= 60 {
		return 8
	}
	if value <= 80 {
		return 6
	}
	if value <= 90 {
		return 3
	}
	return 0
}

func scoreMemoryUsage(value float64) float64 {
	if value <= 70 {
		return 8
	}
	if value <= 85 {
		return 5
	}
	if value <= 95 {
		return 2
	}
	return 0
}

func scoreDiskUsage(value float64) float64 {
	if value <= 70 {
		return 8
	}
	if value <= 85 {
		return 5
	}
	if value <= 95 {
		return 2
	}
	return 0
}

func scoreSwapUsage(value float64) float64 {
	if value <= 5 {
		return 3
	}
	if value <= 20 {
		return 2
	}
	if value <= 50 {
		return 1
	}
	return 0
}

func scoreResourceTrend(points []DashboardSparklinePoint) float64 {
	if len(points) < 4 {
		return 2
	}
	mid := len(points) / 2
	first := averageDashboardPressure(points[:mid])
	second := averageDashboardPressure(points[mid:])
	increase := second - first
	if increase <= 5 {
		return 3
	}
	if increase <= 12 {
		return 2
	}
	if increase <= 20 {
		return 1
	}
	return 0
}

func averageDashboardPressure(points []DashboardSparklinePoint) float64 {
	if len(points) == 0 {
		return 0
	}
	var total float64
	for _, point := range points {
		total += math.Max(point.CPUUsage, math.Max(point.MemUsedPercent, point.DiskUsedPercent))
	}
	return total / float64(len(points))
}

func dashboardLoadScore(metric *model.NodeMetric) float64 {
	if metric == nil {
		return 0
	}
	cores := float64(metric.CPUCores)
	if cores <= 0 {
		cores = 1
	}
	loadRatio := metric.Load5 / cores
	loadScore := 0.0
	switch {
	case loadRatio <= 0.7:
		loadScore = 15
	case loadRatio <= 1.0:
		loadScore = 12
	case loadRatio <= 1.5:
		loadScore = 8
	case loadRatio <= 2.0:
		loadScore = 4
	default:
		loadScore = 0
	}

	burstScore := 0.0
	if metric.Load5 <= 0 {
		if metric.Load1 <= cores*0.7 {
			burstScore = 5
		} else {
			burstScore = 3
		}
	} else if metric.Load1 <= metric.Load5*1.3 {
		burstScore = 5
	} else if metric.Load1 <= metric.Load5*1.8 {
		burstScore = 3
	} else if metric.CPUUsage > 90 {
		burstScore = 0
	} else {
		burstScore = 1
	}
	return loadScore + burstScore
}

func dashboardNetworkScore(stat DashboardNodeProbeStat) float64 {
	if stat.Samples <= 0 {
		return 17.5
	}
	return scoreAvailability(stat.AvailabilityPercent) +
		scorePacketLoss(stat.PacketLossPercent) +
		scoreJitter(stat.JitterMS) +
		scoreLatencySpike(stat.LatencySpikeRatio)
}

func scoreAvailability(value *float64) float64 {
	if value == nil {
		return 6
	}
	switch {
	case *value >= 99.5:
		return 12
	case *value >= 99.0:
		return 10
	case *value >= 97.0:
		return 7
	case *value >= 95.0:
		return 4
	default:
		return 0
	}
}

func scorePacketLoss(value *float64) float64 {
	if value == nil {
		return 6
	}
	switch {
	case *value <= 0:
		return 12
	case *value <= 0.5:
		return 10
	case *value <= 1:
		return 8
	case *value <= 3:
		return 5
	case *value <= 8:
		return 2
	default:
		return 0
	}
}

func scoreJitter(value *float64) float64 {
	if value == nil {
		return 3.5
	}
	switch {
	case *value <= 10:
		return 7
	case *value <= 25:
		return 5
	case *value <= 50:
		return 3
	case *value <= 100:
		return 1
	default:
		return 0
	}
}

func scoreLatencySpike(value *float64) float64 {
	if value == nil {
		return 2
	}
	switch {
	case *value <= 1.3:
		return 4
	case *value <= 1.8:
		return 3
	case *value <= 2.5:
		return 1
	default:
		return 0
	}
}

func dashboardStabilityScore(activeAlerts int64, criticalAlerts int64) float64 {
	if criticalAlerts > 0 {
		return 0
	}
	if activeAlerts > 0 {
		return 1
	}
	return 5
}

func dashboardClusterHealthScore(nodes []model.Node, nodeScores map[string]DashboardNodeHealthScore, criticalAlerts int64) *float64 {
	if len(nodes) == 0 {
		return nil
	}
	var totalScore float64
	var offlineCount float64
	for _, node := range nodes {
		totalScore += nodeScores[node.NodeID].Score
		if node.Status != "online" {
			offlineCount++
		}
	}
	averageScore := totalScore / float64(len(nodes))
	offlinePenalty := math.Sqrt(offlineCount/float64(len(nodes))) * 20
	criticalPenalty := math.Min(10, float64(criticalAlerts)*2)
	value := clampFloat64(averageScore-offlinePenalty-criticalPenalty, 0, 100)
	value = math.Round(value*10) / 10
	return &value
}

func dashboardHealthGrade(score float64) string {
	switch {
	case score >= 95:
		return "excellent"
	case score >= 85:
		return "healthy"
	case score >= 70:
		return "attention"
	case score >= 50:
		return "risk"
	default:
		return "critical"
	}
}

func clampFloat64(value float64, minValue float64, maxValue float64) float64 {
	return math.Max(minValue, math.Min(maxValue, value))
}

func dashboardSparklines(db *gorm.DB, nodeIDs []string, since uint64) (map[string][]DashboardSparklinePoint, error) {
	sparklines := make(map[string][]DashboardSparklinePoint, len(nodeIDs))
	for _, nodeID := range nodeIDs {
		sparklines[nodeID] = []DashboardSparklinePoint{}
	}
	if len(nodeIDs) == 0 {
		return sparklines, nil
	}

	var metrics []model.NodeMetric
	err := db.Where("node_id IN ? AND ts >= ?", nodeIDs, since).
		Order("node_id asc, ts asc").
		Find(&metrics).Error
	if err != nil {
		return nil, err
	}

	grouped := make(map[string][]model.NodeMetric, len(nodeIDs))
	for _, metric := range metrics {
		grouped[metric.NodeID] = append(grouped[metric.NodeID], metric)
	}
	for _, nodeID := range nodeIDs {
		sparklines[nodeID] = downsampleDashboardSparkline(grouped[nodeID], 18)
	}
	return sparklines, nil
}

func downsampleDashboardSparkline(metrics []model.NodeMetric, maxPoints int) []DashboardSparklinePoint {
	if len(metrics) == 0 || maxPoints <= 0 {
		return []DashboardSparklinePoint{}
	}

	points := make([]DashboardSparklinePoint, 0, minInt(len(metrics), maxPoints))
	if len(metrics) <= maxPoints {
		for _, metric := range metrics {
			points = append(points, dashboardSparklinePoint(metric))
		}
		return points
	}

	lastIndex := len(metrics) - 1
	for i := 0; i < maxPoints; i++ {
		index := int(math.Round(float64(i) * float64(lastIndex) / float64(maxPoints-1)))
		if index < 0 {
			index = 0
		}
		if index > lastIndex {
			index = lastIndex
		}
		points = append(points, dashboardSparklinePoint(metrics[index]))
	}
	return points
}

func dashboardSparklinePoint(metric model.NodeMetric) DashboardSparklinePoint {
	return DashboardSparklinePoint{
		Timestamp:       metric.Timestamp,
		CPUUsage:        metric.CPUUsage,
		MemUsedPercent:  metric.MemUsedPercent,
		DiskUsedPercent: metric.DiskUsedPercent,
		NetRxBps:        metric.NetRxBps,
		NetTxBps:        metric.NetTxBps,
	}
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func latestProbeResultTime(db *gorm.DB, nodeID string) (time.Time, bool) {
	var result model.ProbeResult
	if err := db.Select("created_at").
		Where("node_id = ?", nodeID).
		Order("created_at desc").
		First(&result).Error; err != nil {
		return time.Time{}, false
	}
	return result.CreatedAt, true
}

func latestInactiveProbeResultTime(db *gorm.DB, nodeID string, activeTaskIDs []uint64) (time.Time, bool) {
	var result model.ProbeResult
	query := db.Select("created_at").Where("node_id = ?", nodeID)
	if len(activeTaskIDs) > 0 {
		query = query.Where("task_id NOT IN ?", activeTaskIDs)
	}
	if err := query.Order("created_at desc").First(&result).Error; err != nil {
		return time.Time{}, false
	}
	return result.CreatedAt, true
}

func replaceNodeProbeTaskAssignments(db *gorm.DB, nodeID string, taskIDs []uint64) error {
	if err := db.Where("node_id = ?", nodeID).Delete(&model.ProbeTaskAssignment{}).Error; err != nil {
		return err
	}

	unique := make([]uint64, 0, len(taskIDs))
	seen := make(map[uint64]struct{}, len(taskIDs))
	for _, taskID := range taskIDs {
		if taskID == 0 {
			continue
		}
		if _, exists := seen[taskID]; exists {
			continue
		}
		seen[taskID] = struct{}{}
		unique = append(unique, taskID)
	}
	if len(unique) == 0 {
		return nil
	}

	assignments := make([]model.ProbeTaskAssignment, 0, len(unique))
	for _, taskID := range unique {
		assignments = append(assignments, model.ProbeTaskAssignment{
			NodeID: nodeID,
			TaskID: taskID,
		})
	}
	return db.Create(&assignments).Error
}

func assignProbeTaskToAllNodes(db *gorm.DB, taskID uint64) (int, error) {
	if taskID == 0 {
		return 0, nil
	}

	var nodes []model.Node
	if err := db.Select("node_id").Find(&nodes).Error; err != nil {
		return 0, err
	}
	if len(nodes) == 0 {
		return 0, nil
	}

	assignments := make([]model.ProbeTaskAssignment, 0, len(nodes))
	for _, node := range nodes {
		nodeID := strings.TrimSpace(node.NodeID)
		if nodeID == "" {
			continue
		}
		assignments = append(assignments, model.ProbeTaskAssignment{
			NodeID: nodeID,
			TaskID: taskID,
		})
	}
	if len(assignments) == 0 {
		return 0, nil
	}

	result := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "node_id"}, {Name: "task_id"}},
		DoNothing: true,
	}).Create(&assignments)
	if result.Error != nil {
		return 0, result.Error
	}
	return int(result.RowsAffected), nil
}

func probeTaskIDsForNode(db *gorm.DB, nodeID string) []uint64 {
	var assignments []model.ProbeTaskAssignment
	if err := db.Where("node_id = ?", nodeID).Order("task_id asc").Find(&assignments).Error; err != nil {
		return []uint64{}
	}
	ids := make([]uint64, 0, len(assignments))
	for _, assignment := range assignments {
		ids = append(ids, assignment.TaskID)
	}
	return ids
}

func buildNodeOverview(db *gorm.DB, node model.Node) NodeOverview {
	now := time.Now()
	remainingDays, remainingValue := remainingBilling(node, now)
	metric := latestMetric(db, node.NodeID)
	trafficUsed, trafficRemaining := trafficUsage(db, node, metric, now)
	return NodeOverview{
		Node:                  node,
		LatestMetric:          metric,
		IPAddresses:           parseIPAddressesJSON(node.IPAddressesJSON),
		PublicIPs:             parsePublicIPsJSON(node.PublicIPsJSON),
		ProbeTaskIDs:          probeTaskIDsForNode(db, node.NodeID),
		RemainingDays:         remainingDays,
		RemainingValue:        remainingValue,
		TrafficUsedBytes:      trafficUsed,
		TrafficRemainingBytes: trafficRemaining,
	}
}

func parseIPAddressesJSON(raw string) protocol.IPAddresses {
	var addresses protocol.IPAddresses
	if strings.TrimSpace(raw) == "" {
		return addresses
	}
	if err := json.Unmarshal([]byte(raw), &addresses); err != nil {
		return protocol.IPAddresses{}
	}
	return addresses
}

func parsePublicIPsJSON(raw string) protocol.PublicIPs {
	var addresses protocol.PublicIPs
	if strings.TrimSpace(raw) == "" {
		return addresses
	}
	if err := json.Unmarshal([]byte(raw), &addresses); err != nil {
		return protocol.PublicIPs{}
	}
	return addresses
}

func latestMetric(db *gorm.DB, nodeID string) *model.NodeMetric {
	var metric model.NodeMetric
	err := db.Where("node_id = ?", nodeID).Order("ts desc").First(&metric).Error
	if err != nil {
		return nil
	}
	return &metric
}

func parseLimit(raw string, fallback int, max int) int {
	if raw == "" {
		return fallback
	}

	limit, err := strconv.Atoi(raw)
	if err != nil || limit <= 0 {
		return fallback
	}
	if limit > max {
		return max
	}
	return limit
}

func isTruthyQuery(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func parseRangeDuration(raw string) (time.Duration, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "default", "1h":
		return time.Hour, true
	case "4h":
		return 4 * time.Hour, true
	case "12h":
		return 12 * time.Hour, true
	case "1d":
		return 24 * time.Hour, true
	case "7d":
		return 7 * 24 * time.Hour, true
	case "1m":
		return 30 * 24 * time.Hour, true
	case "3m":
		return 90 * 24 * time.Hour, true
	case "6m":
		return 180 * 24 * time.Hour, true
	case "1y":
		return 365 * 24 * time.Hour, true
	default:
		return 0, false
	}
}

func probeAggregateBucketSeconds(raw string) (int64, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1m":
		return int64((6 * time.Hour) / time.Second), true
	case "3m":
		return int64((12 * time.Hour) / time.Second), true
	case "6m":
		return int64((24 * time.Hour) / time.Second), true
	case "1y":
		return int64((48 * time.Hour) / time.Second), true
	default:
		return 0, false
	}
}

type probeAggregateRow struct {
	TaskID         uint64   `gorm:"column:task_id"`
	NodeID         string   `gorm:"column:node_id"`
	Type           string   `gorm:"column:type"`
	IPVersion      string   `gorm:"column:ip_version"`
	Target         string   `gorm:"column:target"`
	BucketUnix     int64    `gorm:"column:bucket_unix"`
	Samples        int64    `gorm:"column:samples"`
	SuccessSamples int64    `gorm:"column:success_samples"`
	LatencyMS      *float64 `gorm:"column:latency_ms"`
	MinLatencyMS   *float64 `gorm:"column:min_latency_ms"`
	MaxLatencyMS   *float64 `gorm:"column:max_latency_ms"`
	PacketLoss     *float64 `gorm:"column:packet_loss"`
}

func listAggregatedProbeResults(db *gorm.DB, nodeID string, since time.Time, bucketSeconds int64, taskNames map[uint64]string, taskIDs []uint64, includeInactive bool) ([]ProbeResultOverview, error) {
	if !includeInactive && len(taskIDs) == 0 {
		return []ProbeResultOverview{}, nil
	}

	var rows []probeAggregateRow
	filterSQL := ""
	args := []any{bucketSeconds, bucketSeconds, nodeID, since}
	if !includeInactive {
		filterSQL = " AND task_id IN ?"
		args = append(args, taskIDs)
	}
	err := db.Raw(fmt.Sprintf(`
SELECT
  task_id,
  node_id,
  type,
  ip_version,
  target,
  %s AS bucket_unix,
  COUNT(*) AS samples,
  SUM(CASE WHEN status = 'success' AND latency_ms IS NOT NULL THEN 1 ELSE 0 END) AS success_samples,
  AVG(CASE WHEN status = 'success' AND latency_ms IS NOT NULL THEN latency_ms ELSE NULL END) AS latency_ms,
  MIN(CASE WHEN status = 'success' AND latency_ms IS NOT NULL THEN latency_ms ELSE NULL END) AS min_latency_ms,
  MAX(CASE WHEN status = 'success' AND latency_ms IS NOT NULL THEN latency_ms ELSE NULL END) AS max_latency_ms,
  AVG(COALESCE(packet_loss, CASE WHEN status = 'success' THEN 0 ELSE 100 END)) AS packet_loss
FROM probe_results
WHERE node_id = ? AND created_at >= ?%s
GROUP BY task_id, node_id, type, ip_version, target, bucket_unix
ORDER BY bucket_unix ASC
`, probeBucketUnixExpr(db), filterSQL), args...).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	overviews := make([]ProbeResultOverview, 0, len(rows))
	for _, row := range rows {
		failedSamples := row.Samples - row.SuccessSamples
		if failedSamples < 0 {
			failedSamples = 0
		}
		status := "failed"
		if row.SuccessSamples > 0 && row.LatencyMS != nil {
			status = "success"
		}
		packetLoss := row.PacketLoss
		if packetLoss == nil && row.Samples > 0 {
			value := float64(failedSamples) / float64(row.Samples) * 100
			packetLoss = &value
		}
		overviews = append(overviews, ProbeResultOverview{
			ProbeResult: model.ProbeResult{
				TaskID:     row.TaskID,
				NodeID:     row.NodeID,
				Type:       row.Type,
				IPVersion:  row.IPVersion,
				Target:     row.Target,
				Status:     status,
				LatencyMS:  row.LatencyMS,
				PacketLoss: packetLoss,
				CreatedAt:  time.Unix(row.BucketUnix, 0),
			},
			TaskName:       taskNames[row.TaskID],
			Samples:        row.Samples,
			SuccessSamples: row.SuccessSamples,
			FailedSamples:  failedSamples,
			MinLatencyMS:   row.MinLatencyMS,
			MaxLatencyMS:   row.MaxLatencyMS,
			BucketSeconds:  bucketSeconds,
			Aggregated:     true,
		})
	}
	return overviews, nil
}

func probeBucketUnixExpr(db *gorm.DB) string {
	if db != nil && db.Dialector != nil && db.Dialector.Name() == "sqlite" {
		return "CAST(CAST(strftime('%s', created_at) AS INTEGER) / ? AS INTEGER) * ?"
	}
	return "FLOOR(UNIX_TIMESTAMP(created_at) / ?) * ?"
}

func rangeResultLimit(duration time.Duration) int {
	switch {
	case duration <= time.Hour:
		return 2000
	case duration <= 4*time.Hour:
		return 4000
	case duration <= 24*time.Hour:
		return 8000
	default:
		return 12000
	}
}

func reverseMetrics(metrics []model.NodeMetric) {
	for left, right := 0, len(metrics)-1; left < right; left, right = left+1, right-1 {
		metrics[left], metrics[right] = metrics[right], metrics[left]
	}
}

func reverseProbeResults(results []model.ProbeResult) {
	for left, right := 0, len(results)-1; left < right; left, right = left+1, right-1 {
		results[left], results[right] = results[right], results[left]
	}
}

func validateNodeUpdateRequest(db *gorm.DB, req nodeUpdateRequest, current model.Node) string {
	if req.Name != nil {
		if message := validateTextField("node name", *req.Name, 64, true); message != "" {
			return message
		}
	}
	if req.Region != nil && !isRegionCodeAllowed(db, *req.Region) {
		return "invalid region code"
	}
	if req.Provider != nil {
		if message := validateTextField("provider", *req.Provider, 64, true); message != "" {
			return message
		}
	}
	if req.NetworkLine != nil {
		if message := validateTextField("network line", *req.NetworkLine, 128, true); message != "" {
			return message
		}
	}
	if req.HeartbeatInterval != nil && *req.HeartbeatInterval < 3 {
		return "heartbeat interval must be at least 3 seconds"
	}
	if req.MetricsInterval != nil && *req.MetricsInterval < 3 {
		return "metrics interval must be at least 3 seconds"
	}
	if req.SnapshotInterval != nil && (*req.SnapshotInterval < 15 || *req.SnapshotInterval > 3600) {
		return "snapshot interval must be between 15 and 3600 seconds"
	}
	if req.SnapshotProcessLimit != nil && (*req.SnapshotProcessLimit < 1 || *req.SnapshotProcessLimit > 50) {
		return "snapshot process limit must be between 1 and 50"
	}
	if req.SnapshotConnectionLimit != nil && (*req.SnapshotConnectionLimit < 1 || *req.SnapshotConnectionLimit > 500) {
		return "snapshot connection limit must be between 1 and 500"
	}
	if req.BillingCycle != nil && !isAllowedValue(*req.BillingCycle, "daily", "monthly", "yearly", "one_time") {
		return "invalid billing cycle"
	}
	if req.Currency != nil && !isAllowedValue(*req.Currency, supportedAssetCurrencies()...) {
		return "invalid currency"
	}
	if req.TrafficResetCycle != nil && !isAllowedValue(*req.TrafficResetCycle, "daily", "monthly", "yearly", "never") {
		return "invalid traffic reset cycle"
	}
	if req.TrafficBillingDirection != nil && !isAllowedValue(*req.TrafficBillingDirection, "bidirectional", "outbound") {
		return "invalid traffic billing direction"
	}
	if req.PriceAmount != nil {
		if math.IsNaN(*req.PriceAmount) || math.IsInf(*req.PriceAmount, 0) {
			return "price amount must be a valid number"
		}
		if *req.PriceAmount < 0 {
			return "price amount cannot be negative"
		}
		if math.Abs(*req.PriceAmount*100-math.Round(*req.PriceAmount*100)) > 0.000001 {
			return "price amount supports at most two decimals"
		}
	}
	if req.Tag != nil {
		if message := validateNodeTag(*req.Tag); message != "" {
			return message
		}
	}
	serviceStartedAt := current.ServiceStartedAt
	if req.ServiceStartedAt.Set {
		serviceStartedAt = nil
		if req.ServiceStartedAt.Valid {
			value := req.ServiceStartedAt.Value
			serviceStartedAt = &value
		}
	}
	serviceExpiresAt := current.ServiceExpiresAt
	if req.ServiceExpiresAt.Set {
		serviceExpiresAt = nil
		if req.ServiceExpiresAt.Valid {
			value := req.ServiceExpiresAt.Value
			serviceExpiresAt = &value
		}
	}
	if serviceStartedAt != nil && serviceExpiresAt != nil && *serviceStartedAt > *serviceExpiresAt {
		return "service start date cannot be after end date"
	}
	trafficLimitBytes := current.TrafficLimitBytes
	if req.TrafficLimitBytes != nil {
		trafficLimitBytes = *req.TrafficLimitBytes
	}
	trafficCalibrationBytes := current.TrafficCalibrationBytes
	if req.TrafficCalibrationBytes != nil {
		trafficCalibrationBytes = *req.TrafficCalibrationBytes
	}
	if trafficLimitBytes > 0 && trafficCalibrationBytes > trafficLimitBytes {
		return "traffic calibration cannot be greater than traffic limit"
	}
	if req.ProbeTaskIDs != nil {
		if message := validateProbeTaskIDs(db, *req.ProbeTaskIDs); message != "" {
			return message
		}
	}
	return ""
}

func validateProbeTaskIDs(db *gorm.DB, taskIDs []uint64) string {
	seen := make(map[uint64]struct{}, len(taskIDs))
	ids := make([]uint64, 0, len(taskIDs))
	for _, taskID := range taskIDs {
		if taskID == 0 {
			return "invalid probe task id"
		}
		if _, exists := seen[taskID]; exists {
			continue
		}
		seen[taskID] = struct{}{}
		ids = append(ids, taskID)
	}
	if len(ids) == 0 {
		return ""
	}

	var count int64
	if err := db.Model(&model.ProbeTask{}).Where("id IN ?", ids).Count(&count).Error; err != nil {
		return "check probe tasks failed"
	}
	if count != int64(len(ids)) {
		return "probe task not found"
	}
	return ""
}

func validateNodeTag(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	if strings.ContainsFunc(value, unicode.IsSpace) {
		return "tag cannot contain whitespace"
	}
	tags := splitNodeTags(value)
	for _, tag := range tags {
		if tag == "" {
			return "tag segment cannot be empty"
		}
	}
	if len(tags) > 5 {
		return "tag supports at most 5 items"
	}
	totalLength := 0
	for _, tag := range tags {
		totalLength += len([]rune(tag))
	}
	if totalLength > 25 {
		return "tag total length cannot exceed 25 characters"
	}
	return ""
}

func validateTextField(label string, raw string, maxRunes int, allowEmpty bool) string {
	value := strings.TrimSpace(raw)
	if !allowEmpty && value == "" {
		return label + " is required"
	}
	if maxRunes > 0 && len([]rune(value)) > maxRunes {
		return fmt.Sprintf("%s cannot exceed %d characters", label, maxRunes)
	}
	if strings.ContainsFunc(value, func(r rune) bool {
		return r < 32 || r == 127
	}) {
		return label + " cannot contain control characters"
	}
	return ""
}

func hasControlChars(value string) bool {
	return strings.ContainsFunc(value, func(r rune) bool {
		return r < 32 || r == 127
	})
}

func validateImageURL(label string, raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	if message := validateTextField(label, value, 2048, true); message != "" {
		return message
	}
	if strings.HasPrefix(value, "/") && !strings.HasPrefix(value, "//") {
		parsed, err := url.ParseRequestURI(value)
		if err == nil && parsed.Host == "" && strings.HasPrefix(parsed.Path, "/") {
			return ""
		}
		return label + " must be a valid same-origin absolute path or http(s) URL"
	}
	lowerValue := strings.ToLower(value)
	if !strings.HasPrefix(lowerValue, "http://") && !strings.HasPrefix(lowerValue, "https://") {
		return label + " must start with http://, https://, or /"
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || !isAllowedValue(strings.ToLower(parsed.Scheme), "http", "https") {
		return label + " must be a valid http(s) URL"
	}
	return ""
}

func normalizeNodeTag(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	return strings.Join(splitNodeTags(value), "/")
}

func splitNodeTags(raw string) []string {
	return strings.FieldsFunc(raw, func(r rune) bool {
		return r == '/' || r == ',' || r == '，' || r == '、'
	})
}

func isAllowedValue(value string, allowed ...string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	for _, item := range allowed {
		if normalized == strings.ToLower(strings.TrimSpace(item)) {
			return true
		}
	}
	return false
}

func normalizeTrafficBillingDirection(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "outbound":
		return "outbound"
	default:
		return "bidirectional"
	}
}

func nodeUpdates(req nodeUpdateRequest, current model.Node) map[string]any {
	updates := make(map[string]any)
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Region != nil {
		updates["region"] = normalizeRegionCode(*req.Region)
	}
	if req.Provider != nil {
		updates["provider"] = *req.Provider
	}
	if req.NetworkLine != nil {
		updates["network_line"] = strings.TrimSpace(*req.NetworkLine)
	}
	if req.Tag != nil {
		updates["tag"] = normalizeNodeTag(*req.Tag)
	}
	if req.HeartbeatInterval != nil && *req.HeartbeatInterval > 0 {
		updates["heartbeat_interval"] = *req.HeartbeatInterval
	}
	if req.MetricsInterval != nil && *req.MetricsInterval > 0 {
		updates["metrics_interval"] = *req.MetricsInterval
	}
	if req.SnapshotOverride != nil {
		updates["snapshot_override"] = *req.SnapshotOverride
	}
	if req.SnapshotEnabled != nil {
		updates["snapshot_enabled"] = *req.SnapshotEnabled
	}
	if req.SnapshotCollectProcesses != nil {
		updates["snapshot_collect_processes"] = *req.SnapshotCollectProcesses
	}
	if req.SnapshotCollectConnections != nil {
		updates["snapshot_collect_connections"] = *req.SnapshotCollectConnections
	}
	if req.SnapshotMaskSensitive != nil {
		updates["snapshot_mask_sensitive"] = *req.SnapshotMaskSensitive
	}
	if req.SnapshotInterval != nil && *req.SnapshotInterval > 0 {
		updates["snapshot_interval"] = *req.SnapshotInterval
	}
	if req.SnapshotProcessLimit != nil && *req.SnapshotProcessLimit > 0 {
		updates["snapshot_process_limit"] = *req.SnapshotProcessLimit
	}
	if req.SnapshotConnectionLimit != nil && *req.SnapshotConnectionLimit > 0 {
		updates["snapshot_connection_limit"] = *req.SnapshotConnectionLimit
	}
	if req.BillingCycle != nil {
		updates["billing_cycle"] = strings.ToLower(strings.TrimSpace(*req.BillingCycle))
	}
	if req.PriceAmount != nil {
		updates["price_amount"] = *req.PriceAmount
	}
	if req.Currency != nil {
		updates["currency"] = *req.Currency
	}
	if req.ServiceStartedAt.Set {
		if req.ServiceStartedAt.Valid {
			updates["service_started_at"] = req.ServiceStartedAt.Value
		} else {
			updates["service_started_at"] = nil
		}
	}
	if req.ServiceExpiresAt.Set {
		if req.ServiceExpiresAt.Valid {
			updates["service_expires_at"] = req.ServiceExpiresAt.Value
		} else {
			updates["service_expires_at"] = nil
		}
	}
	if req.TrafficLimitBytes != nil {
		updates["traffic_limit_bytes"] = *req.TrafficLimitBytes
	}
	if req.TrafficCalibrationBytes != nil {
		updates["traffic_calibration_bytes"] = *req.TrafficCalibrationBytes
		if *req.TrafficCalibrationBytes == 0 {
			updates["traffic_calibration_at"] = nil
		} else if current.TrafficCalibrationAt == nil || current.TrafficCalibrationBytes != *req.TrafficCalibrationBytes {
			updates["traffic_calibration_at"] = uint64(time.Now().UnixMilli())
		}
	}
	if req.TrafficResetCycle != nil {
		updates["traffic_reset_cycle"] = strings.ToLower(strings.TrimSpace(*req.TrafficResetCycle))
	}
	if req.TrafficBillingDirection != nil {
		updates["traffic_billing_direction"] = normalizeTrafficBillingDirection(*req.TrafficBillingDirection)
	}
	return updates
}

func remainingBilling(node model.Node, now time.Time) (int64, float64) {
	if node.ServiceExpiresAt == nil || *node.ServiceExpiresAt == 0 {
		return 0, 0
	}

	remainingMS := int64(*node.ServiceExpiresAt) - now.UnixMilli()
	if remainingMS <= 0 {
		return 0, 0
	}

	days := (remainingMS + int64(24*time.Hour/time.Millisecond) - 1) / int64(24*time.Hour/time.Millisecond)
	if strings.EqualFold(node.BillingCycle, "one_time") {
		if node.ServiceStartedAt == nil || *node.ServiceStartedAt >= *node.ServiceExpiresAt {
			return days, node.PriceAmount
		}

		totalMS := int64(*node.ServiceExpiresAt) - int64(*node.ServiceStartedAt)
		if totalMS <= 0 {
			return days, 0
		}
		return days, node.PriceAmount * float64(remainingMS) / float64(totalMS)
	}

	value := float64(days) * dailyPrice(node.BillingCycle, node.PriceAmount)
	return days, value
}

func dailyPrice(cycle string, amount float64) float64 {
	switch strings.ToLower(cycle) {
	case "daily":
		return amount
	case "one_time":
		return 0
	case "yearly":
		return amount / 365
	default:
		return amount / 30
	}
}

func trafficUsage(db *gorm.DB, node model.Node, metric *model.NodeMetric, now time.Time) (uint64, uint64) {
	var used uint64
	direction := normalizeTrafficBillingDirection(node.TrafficBillingDirection)
	if metric == nil {
		used = 0
	} else {
		used = metricTrafficTotal(*metric, direction)
		if cycleStart := trafficCycleStart(node, now); cycleStart > 0 {
			used = trafficUsedSince(db, node.NodeID, *metric, cycleStart, direction, used)
		}
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

func trafficUsedSince(db *gorm.DB, nodeID string, latest model.NodeMetric, cycleStart uint64, direction string, fallback uint64) uint64 {
	var baseline model.NodeMetric
	err := db.Where("node_id = ? AND ts <= ?", nodeID, cycleStart).Order("ts desc").First(&baseline).Error
	if err != nil {
		err = db.Where("node_id = ? AND ts >= ? AND ts <= ?", nodeID, cycleStart, latest.Timestamp).Order("ts asc").First(&baseline).Error
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

func storeSystemLog(db *gorm.DB, service string, nodeID string, level string, message string) {
	storeSystemEventLog(db, service, nodeID, level, "system.event", message, nil)
}

func storeSystemEventLog(db *gorm.DB, service string, nodeID string, level string, eventType string, message string, meta map[string]any) {
	if strings.TrimSpace(message) == "" {
		return
	}
	_ = db.Create(&model.SystemLog{
		Service:   normalizeSystemLogValue(service, "master"),
		NodeID:    strings.TrimSpace(nodeID),
		Level:     normalizeSystemLogLevel(level),
		EventType: normalizeSystemLogValue(eventType, "system.event"),
		Message:   message,
		MetaJSON:  marshalSystemLogMeta(meta),
	}).Error
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

func requireAuth(auth config.AuthConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		token, ok := strings.CutPrefix(header, "Bearer ")
		if !ok || !credentialMatches(token, tokenFor(auth)) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		c.Next()
	}
}

func tokenFor(auth config.AuthConfig) string {
	sum := sha256.Sum256([]byte(auth.Username + "\x00" + auth.Password))
	return hex.EncodeToString(sum[:])
}

func credentialMatches(got string, want string) bool {
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

func maskDSN(dsn string) string {
	at := strings.Index(dsn, "@")
	colon := strings.Index(dsn, ":")
	if at <= 0 || colon <= 0 || colon > at {
		return "********"
	}

	return dsn[:colon+1] + "********" + dsn[at:]
}
