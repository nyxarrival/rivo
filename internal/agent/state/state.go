package state

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const DefaultPath = "data/agent-state.json"

type State struct {
	MasterAddr string `json:"master_addr,omitempty"`
	SecretKey  string `json:"secret_key,omitempty"`
	NodeID     string `json:"node_id,omitempty"`
}

func NormalizePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return DefaultPath
	}
	return path
}

func Read(path string) (State, error) {
	path = NormalizePath(path)
	raw, err := os.ReadFile(path)
	if err != nil {
		return State{}, err
	}
	var state State
	if err := json.Unmarshal(raw, &state); err != nil {
		return State{}, err
	}
	return state, nil
}

func Save(path string, update State) error {
	path = NormalizePath(path)
	current, err := Read(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	if value := strings.TrimSpace(update.MasterAddr); value != "" {
		current.MasterAddr = value
	}
	if value := strings.TrimSpace(update.SecretKey); value != "" {
		current.SecretKey = value
	}
	if value := strings.TrimSpace(update.NodeID); value != "" {
		current.NodeID = value
	}

	return write(path, current)
}

func write(path string, state State) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
