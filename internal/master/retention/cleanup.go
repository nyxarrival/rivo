package retention

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"rivo/internal/master/model"

	"gorm.io/gorm"
)

const (
	MetricRetentionSettingKey       = "metrics_retention_months"
	DefaultTelemetryRetentionMonths = 6
)

type CleanupResult struct {
	RetentionMonths      int
	Cutoff               time.Time
	DeletedMetrics       int64
	DeletedProbeResults  int64
}

func NormalizeTelemetryRetentionMonths(months int) int {
	switch months {
	case 1, 3, 6, 12:
		return months
	default:
		return DefaultTelemetryRetentionMonths
	}
}

func ParseTelemetryRetentionMonths(raw string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return NormalizeTelemetryRetentionMonths(fallback)
	}
	return NormalizeTelemetryRetentionMonths(value)
}

func ValidTelemetryRetentionMonths(months int) bool {
	return months == 1 || months == 3 || months == 6 || months == 12
}

func LoadTelemetryRetentionMonths(db *gorm.DB) int {
	var row model.AppSetting
	if err := db.Where("`key` = ?", MetricRetentionSettingKey).First(&row).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return DefaultTelemetryRetentionMonths
		}
		return DefaultTelemetryRetentionMonths
	}
	return ParseTelemetryRetentionMonths(row.Value, DefaultTelemetryRetentionMonths)
}

func CleanupTelemetryData(db *gorm.DB, months int) (CleanupResult, error) {
	months = NormalizeTelemetryRetentionMonths(months)
	cutoff := time.Now().AddDate(0, -months, 0)
	cutoffMs := uint64(cutoff.UnixMilli())

	metricTx := db.Where("ts < ?", cutoffMs).Delete(&model.NodeMetric{})
	if metricTx.Error != nil {
		return CleanupResult{}, metricTx.Error
	}
	probeTx := db.Where("created_at < ?", cutoff).Delete(&model.ProbeResult{})
	if probeTx.Error != nil {
		return CleanupResult{}, probeTx.Error
	}

	return CleanupResult{
		RetentionMonths:     months,
		Cutoff:              cutoff,
		DeletedMetrics:      metricTx.RowsAffected,
		DeletedProbeResults: probeTx.RowsAffected,
	}, nil
}

func StartTelemetryCleanup(ctx context.Context, logger *slog.Logger, db *gorm.DB) {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		runTelemetryCleanup(logger, db)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func runTelemetryCleanup(logger *slog.Logger, db *gorm.DB) {
	months := LoadTelemetryRetentionMonths(db)
	result, err := CleanupTelemetryData(db, months)
	if err != nil {
		logger.Warn("cleanup telemetry data failed", slog.String("error", err.Error()))
		return
	}
	if result.DeletedMetrics > 0 || result.DeletedProbeResults > 0 {
		logger.Info(
			"telemetry data cleaned",
			slog.Int("retention_months", result.RetentionMonths),
			slog.Time("cutoff", result.Cutoff),
			slog.Int64("deleted_metrics", result.DeletedMetrics),
			slog.Int64("deleted_probe_results", result.DeletedProbeResults),
		)
	}
}
