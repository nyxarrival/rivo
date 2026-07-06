package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	HTTP     HTTPConfig     `yaml:"http"`
	TCP      TCPConfig      `yaml:"tcp"`
	Database DatabaseConfig `yaml:"database"`
	Auth     AuthConfig     `yaml:"auth"`
	Log      LogConfig      `yaml:"log"`
}

type HTTPConfig struct {
	ListenAddr string `yaml:"listen_addr"`
	AdminPath  string `yaml:"admin_path"`
}

type TCPConfig struct {
	ListenAddr string `yaml:"listen_addr"`
	SecretKey  string `yaml:"secret_key"`
}

type DatabaseConfig struct {
	Driver      string `yaml:"driver"`
	DSN         string `yaml:"dsn"`
	AutoMigrate bool   `yaml:"auto_migrate"`
}

type AuthConfig struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type LogConfig struct {
	Level         string `yaml:"level"`
	File          string `yaml:"file"`
	RetentionDays int    `yaml:"retention_days"`
}

func Load(path string) (*Config, error) {
	cfg := &Config{
		HTTP: HTTPConfig{ListenAddr: ":8080"},
		TCP:  TCPConfig{ListenAddr: ":9443"},
		Auth: AuthConfig{
			Username: "admin",
			Password: "change-me-admin-password",
		},
		Log: LogConfig{
			Level:         "info",
			File:          "logs/master.log",
			RetentionDays: 30,
		},
		Database: DatabaseConfig{
			Driver:      "mysql",
			AutoMigrate: true,
		},
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(raw, cfg); err != nil {
		return nil, err
	}
	if cfg.TCP.ListenAddr == "" {
		cfg.TCP.ListenAddr = ":9443"
	}
	if cfg.Database.Driver == "" {
		cfg.Database.Driver = "mysql"
	}
	applyEnvOverrides(cfg)
	cfg.HTTP.AdminPath = NormalizeAdminPath(cfg.HTTP.AdminPath)
	cfg.Database.Driver = strings.ToLower(strings.TrimSpace(cfg.Database.Driver))
	if cfg.Database.Driver == "" {
		cfg.Database.Driver = "mysql"
	}
	if cfg.Database.Driver == "sqlite" && strings.TrimSpace(cfg.Database.DSN) == "" {
		cfg.Database.DSN = "data/rivo.db"
	}
	if err := ValidateAdminPath(cfg.HTTP.AdminPath); err != nil {
		return nil, err
	}

	return cfg, nil
}

func applyEnvOverrides(cfg *Config) {
	if value := strings.TrimSpace(os.Getenv("RIVO_DATABASE_DRIVER")); value != "" {
		cfg.Database.Driver = value
	}
	if value := strings.TrimSpace(os.Getenv("RIVO_DATABASE_DSN")); value != "" {
		cfg.Database.DSN = value
	}
	if value := strings.TrimSpace(os.Getenv("RIVO_DATABASE_AUTO_MIGRATE")); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			cfg.Database.AutoMigrate = parsed
		}
	}
	if value := strings.TrimSpace(os.Getenv("RIVO_ADMIN_PATH")); value != "" {
		cfg.HTTP.AdminPath = value
	}
}

func NormalizeAdminPath(value string) string {
	return strings.Trim(strings.TrimSpace(value), "/")
}

func ValidateAdminPath(value string) error {
	if len(value) <= 5 {
		return fmt.Errorf("http.admin_path must be longer than 5 characters")
	}
	for _, char := range value {
		if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9')) {
			return fmt.Errorf("http.admin_path must contain only letters and digits")
		}
	}
	switch strings.ToLower(value) {
	case "api", "healthz", "themes":
		return fmt.Errorf("http.admin_path cannot use a reserved path")
	}
	return nil
}
