package api

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"rivo/internal/master/config"
	"rivo/internal/master/model"
)

func TestLatestInactiveProbeResultTimeIgnoresCurrentActiveTasks(t *testing.T) {
	db := newProbeResultTestDB(t)
	base := time.Date(2026, 6, 30, 8, 0, 0, 0, time.UTC)
	insertProbeResult(t, db, "node-a", 1, base.Add(2*time.Hour))
	insertProbeResult(t, db, "node-a", 2, base.Add(30*time.Minute))
	insertProbeResult(t, db, "node-a", 2, base.Add(10*time.Minute))

	got, ok := latestInactiveProbeResultTime(db, "node-a", []uint64{1})
	if !ok {
		t.Fatal("latestInactiveProbeResultTime returned false")
	}
	want := base.Add(30 * time.Minute)
	if got.Unix() != want.Unix() {
		t.Fatalf("latest inactive result = %s, want %s", got, want)
	}
}

func TestLatestInactiveProbeResultTimeUsesAllResultsWithoutActiveTasks(t *testing.T) {
	db := newProbeResultTestDB(t)
	base := time.Date(2026, 6, 30, 8, 0, 0, 0, time.UTC)
	insertProbeResult(t, db, "node-a", 1, base.Add(20*time.Minute))
	insertProbeResult(t, db, "node-a", 2, base.Add(50*time.Minute))

	got, ok := latestInactiveProbeResultTime(db, "node-a", nil)
	if !ok {
		t.Fatal("latestInactiveProbeResultTime returned false")
	}
	want := base.Add(50 * time.Minute)
	if got.Unix() != want.Unix() {
		t.Fatalf("latest inactive result = %s, want %s", got, want)
	}
}

func TestDeleteNodeEndpointDeletesAssociatedData(t *testing.T) {
	db := newSQLiteMemoryDB(t)
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
		&model.RegionOption{},
		&model.AppSetting{},
	); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	nodeID := "node-delete"
	now := time.Now()
	latency := 12.5
	metricTS := uint64(now.UnixMilli())
	if err := db.Create(&model.Node{NodeID: nodeID, Name: "Delete Me", CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatalf("insert node: %v", err)
	}
	if err := db.Create(&model.NodeMetric{NodeID: nodeID, Timestamp: metricTS, CreatedAt: now}).Error; err != nil {
		t.Fatalf("insert metric: %v", err)
	}
	if err := db.Create(&model.NodeSnapshot{NodeID: nodeID, Timestamp: metricTS, CreatedAt: now}).Error; err != nil {
		t.Fatalf("insert snapshot: %v", err)
	}
	if err := db.Create(&model.ProbeTask{ID: 1, Target: "example.com:443", CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatalf("insert probe task: %v", err)
	}
	if err := db.Create(&model.ProbeTaskAssignment{NodeID: nodeID, TaskID: 1, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatalf("insert assignment: %v", err)
	}
	if err := db.Create(&model.ProbeResult{NodeID: nodeID, TaskID: 1, Target: "example.com:443", Status: "success", LatencyMS: &latency, CreatedAt: now}).Error; err != nil {
		t.Fatalf("insert probe result: %v", err)
	}
	if err := db.Create(&model.Alert{NodeID: nodeID, RuleType: "cpu", Level: "warning", Status: "active", CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatalf("insert alert: %v", err)
	}
	if err := db.Create(&model.NodeEvent{NodeID: nodeID, EventType: "agent.online", CreatedAt: now}).Error; err != nil {
		t.Fatalf("insert node event: %v", err)
	}
	if err := db.Create(&model.SystemLog{Service: "agent", NodeID: nodeID, Level: "info", EventType: "agent.online", Message: "online", CreatedAt: now}).Error; err != nil {
		t.Fatalf("insert system log: %v", err)
	}

	cfg := &config.Config{Auth: config.AuthConfig{Username: "admin", Password: "secret"}}
	router := NewRouter(slog.New(slog.NewTextHandler(io.Discard, nil)), db, cfg)
	req := httptest.NewRequest(http.MethodDelete, "/api/admin/nodes/"+nodeID, nil)
	req.Header.Set("Authorization", "Bearer "+tokenFor(cfg.Auth))
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body.String())
	}

	assertNoRowsForNode(t, db, &model.Node{}, nodeID)
	assertNoRowsForNode(t, db, &model.NodeMetric{}, nodeID)
	assertNoRowsForNode(t, db, &model.NodeSnapshot{}, nodeID)
	assertNoRowsForNode(t, db, &model.ProbeTaskAssignment{}, nodeID)
	assertNoRowsForNode(t, db, &model.ProbeResult{}, nodeID)
	assertNoRowsForNode(t, db, &model.Alert{}, nodeID)
	assertNoRowsForNode(t, db, &model.NodeEvent{}, nodeID)

	var logs []model.SystemLog
	if err := db.Where("node_id = ?", nodeID).Find(&logs).Error; err != nil {
		t.Fatalf("query system logs: %v", err)
	}
	if len(logs) != 1 || logs[0].EventType != "node.deleted" {
		t.Fatalf("system logs after delete = %#v, want deletion audit log only", logs)
	}
}

func newProbeResultTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := newSQLiteMemoryDB(t)
	if err := db.AutoMigrate(&model.ProbeResult{}); err != nil {
		t.Fatalf("migrate probe results: %v", err)
	}
	return db
}

func newSQLiteMemoryDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	return db
}

func assertNoRowsForNode(t *testing.T, db *gorm.DB, modelValue any, nodeID string) {
	t.Helper()
	var count int64
	if err := db.Model(modelValue).Where("node_id = ?", nodeID).Count(&count).Error; err != nil {
		t.Fatalf("count %T: %v", modelValue, err)
	}
	if count != 0 {
		t.Fatalf("%T rows for %s = %d, want 0", modelValue, nodeID, count)
	}
}

func insertProbeResult(t *testing.T, db *gorm.DB, nodeID string, taskID uint64, createdAt time.Time) {
	t.Helper()
	latency := 10.0
	if err := db.Create(&model.ProbeResult{
		TaskID:    taskID,
		NodeID:    nodeID,
		Type:      "tcp",
		IPVersion: "ipv4",
		Target:    "example.com:80",
		Status:    "success",
		LatencyMS: &latency,
		CreatedAt: createdAt,
	}).Error; err != nil {
		t.Fatalf("insert probe result: %v", err)
	}
}
