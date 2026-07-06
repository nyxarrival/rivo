package state

import (
	"path/filepath"
	"testing"
)

func TestSaveMergesExistingState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent-state.json")

	if err := Save(path, State{
		MasterAddr: "10.0.0.1:9443",
		SecretKey:  "secret",
		NodeID:     "node-a",
	}); err != nil {
		t.Fatalf("save initial state: %v", err)
	}
	if err := Save(path, State{NodeID: "node-b"}); err != nil {
		t.Fatalf("save partial state: %v", err)
	}

	got, err := Read(path)
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	if got.MasterAddr != "10.0.0.1:9443" {
		t.Fatalf("MasterAddr = %q, want %q", got.MasterAddr, "10.0.0.1:9443")
	}
	if got.SecretKey != "secret" {
		t.Fatalf("SecretKey = %q, want %q", got.SecretKey, "secret")
	}
	if got.NodeID != "node-b" {
		t.Fatalf("NodeID = %q, want %q", got.NodeID, "node-b")
	}
}

func TestNormalizePath(t *testing.T) {
	if got := NormalizePath("  "); got != DefaultPath {
		t.Fatalf("NormalizePath(empty) = %q, want %q", got, DefaultPath)
	}
	if got := NormalizePath(" custom.json "); got != "custom.json" {
		t.Fatalf("NormalizePath(custom) = %q, want %q", got, "custom.json")
	}
}
