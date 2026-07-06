package api

import (
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

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
