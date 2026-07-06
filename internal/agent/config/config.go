package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	MasterAddr string         `yaml:"master_addr"`
	SecretKey  string         `yaml:"secret_key"`
	Agent      AgentConfig    `yaml:"agent"`
	PublicIP   PublicIPConfig `yaml:"public_ip"`
	Log        LogConfig      `yaml:"log"`
}

type AgentConfig struct {
	NodeID    string `yaml:"node_id"`
	StateFile string `yaml:"state_file"`
}

type LogConfig struct {
	Level         string `yaml:"level"`
	File          string `yaml:"file"`
	RetentionDays int    `yaml:"retention_days"`
}

type PublicIPConfig struct {
	Enabled                bool     `yaml:"enabled"`
	TimeoutMS              int      `yaml:"timeout_ms"`
	RefreshIntervalSeconds int      `yaml:"refresh_interval_seconds"`
	IPv4Enabled            bool     `yaml:"ipv4_enabled"`
	IPv6Enabled            bool     `yaml:"ipv6_enabled"`
	IPv4Endpoints          []string `yaml:"ipv4_endpoints"`
	IPv6Endpoints          []string `yaml:"ipv6_endpoints"`
}

func Load(path string) (*Config, error) {
	cfg := Default()

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(raw, cfg); err != nil {
		return nil, err
	}
	cfg.Normalize()

	return cfg, nil
}

func Default() *Config {
	return &Config{
		MasterAddr: "127.0.0.1:9443",
		Agent: AgentConfig{
			StateFile: "data/agent-state.json",
		},
		PublicIP: PublicIPConfig{
			Enabled:                true,
			TimeoutMS:              3000,
			RefreshIntervalSeconds: 600,
			IPv4Enabled:            true,
			IPv6Enabled:            true,
			IPv4Endpoints:          []string{"https://api.ipify.org", "https://v4.ident.me", "https://ipv4.icanhazip.com", "https://ifconfig.me/ip"},
			IPv6Endpoints:          []string{"https://api6.ipify.org", "https://v6.ident.me", "https://ipv6.icanhazip.com", "https://ifconfig.me/ip"},
		},
		Log: LogConfig{
			Level:         "info",
			File:          "logs/agent.log",
			RetentionDays: 30,
		},
	}
}

func (c *Config) Normalize() {
	if c.MasterAddr == "" {
		c.MasterAddr = "127.0.0.1:9443"
	}
	if c.Agent.StateFile == "" {
		c.Agent.StateFile = "data/agent-state.json"
	}
	if c.PublicIP.TimeoutMS <= 0 {
		c.PublicIP.TimeoutMS = 3000
	}
	if c.PublicIP.RefreshIntervalSeconds <= 0 {
		c.PublicIP.RefreshIntervalSeconds = 600
	}
	if len(c.PublicIP.IPv4Endpoints) == 0 {
		c.PublicIP.IPv4Endpoints = []string{"https://api.ipify.org", "https://v4.ident.me", "https://ipv4.icanhazip.com", "https://ifconfig.me/ip"}
	}
	if len(c.PublicIP.IPv6Endpoints) == 0 {
		c.PublicIP.IPv6Endpoints = []string{"https://api6.ipify.org", "https://v6.ident.me", "https://ipv6.icanhazip.com", "https://ifconfig.me/ip"}
	}
	if c.Log.Level == "" {
		c.Log.Level = "info"
	}
	if c.Log.File == "" {
		c.Log.File = "logs/agent.log"
	}
	if c.Log.RetentionDays <= 0 {
		c.Log.RetentionDays = 30
	}
}
