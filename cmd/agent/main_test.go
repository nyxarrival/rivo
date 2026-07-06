package main

import (
	"path/filepath"
	"testing"

	"rivo/internal/agent/config"
	agentstate "rivo/internal/agent/state"
)

func TestApplyPersistedStateUsesStateWithoutExplicitConfig(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "agent-state.json")
	if err := agentstate.Save(stateFile, agentstate.State{
		MasterAddr: "10.0.0.1:9443",
		SecretKey:  "secret",
		NodeID:     "node-a",
	}); err != nil {
		t.Fatalf("save state: %v", err)
	}

	cfg := config.Default()
	cfg.MasterAddr = "127.0.0.1:9443"
	cfg.SecretKey = "from-config"
	cfg.Agent.StateFile = stateFile

	if err := applyPersistedState(cfg, map[string]bool{}); err != nil {
		t.Fatalf("apply persisted state: %v", err)
	}

	if cfg.MasterAddr != "10.0.0.1:9443" {
		t.Fatalf("MasterAddr = %q, want %q", cfg.MasterAddr, "10.0.0.1:9443")
	}
	if cfg.SecretKey != "secret" {
		t.Fatalf("SecretKey = %q, want %q", cfg.SecretKey, "secret")
	}
	if cfg.Agent.NodeID != "node-a" {
		t.Fatalf("NodeID = %q, want %q", cfg.Agent.NodeID, "node-a")
	}
}

func TestApplyPersistedStateKeepsExplicitConfigConnection(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "agent-state.json")
	if err := agentstate.Save(stateFile, agentstate.State{
		MasterAddr: "10.0.0.1:9443",
		SecretKey:  "secret",
		NodeID:     "node-a",
	}); err != nil {
		t.Fatalf("save state: %v", err)
	}

	cfg := config.Default()
	cfg.MasterAddr = "127.0.0.1:9443"
	cfg.SecretKey = "from-config"
	cfg.Agent.StateFile = stateFile

	if err := applyPersistedState(cfg, map[string]bool{"config": true}); err != nil {
		t.Fatalf("apply persisted state: %v", err)
	}

	if cfg.MasterAddr != "127.0.0.1:9443" {
		t.Fatalf("MasterAddr = %q, want %q", cfg.MasterAddr, "127.0.0.1:9443")
	}
	if cfg.SecretKey != "from-config" {
		t.Fatalf("SecretKey = %q, want %q", cfg.SecretKey, "from-config")
	}
	if cfg.Agent.NodeID != "node-a" {
		t.Fatalf("NodeID = %q, want %q", cfg.Agent.NodeID, "node-a")
	}
}

func TestSaveFinalStatePersistsResolvedConfig(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "agent-state.json")
	cfg := config.Default()
	cfg.MasterAddr = "master.example.com:9443"
	cfg.SecretKey = "secret"
	cfg.Agent.StateFile = stateFile

	if err := saveFinalState(cfg, "node-a"); err != nil {
		t.Fatalf("save final state: %v", err)
	}

	state, err := agentstate.Read(stateFile)
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	if state.MasterAddr != "master.example.com:9443" {
		t.Fatalf("MasterAddr = %q, want %q", state.MasterAddr, "master.example.com:9443")
	}
	if state.SecretKey != "secret" {
		t.Fatalf("SecretKey = %q, want %q", state.SecretKey, "secret")
	}
	if state.NodeID != "node-a" {
		t.Fatalf("NodeID = %q, want %q", state.NodeID, "node-a")
	}
}
