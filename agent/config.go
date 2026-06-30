package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	ServerURL  string `json:"server_url"`
	MachineID  string `json:"machine_id"`
	APIKey     string `json:"api_key"`
	Name       string `json:"name"`
	PollWait   int    `json:"poll_wait"`
	MaxTimeout int    `json:"max_timeout"`
	LogFile    string `json:"log_file"`
}

func DefaultConfig() *Config {
	exe, _ := os.Executable()
	dir := filepath.Dir(exe)

	return &Config{
		ServerURL:  "http://127.0.0.1:9090",
		MachineID:  "",
		APIKey:     "",
		Name:       "",
		PollWait:   60,
		MaxTimeout: 90,
		LogFile:    filepath.Join(dir, "agent.log"),
	}
}

func LoadConfig(path string) (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config file not found: %s (create one from the template)", path)
		}
		return nil, fmt.Errorf("read config: %w", err)
	}

	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if cfg.ServerURL == "" {
		return nil, fmt.Errorf("server_url is required")
	}
	if cfg.MachineID == "" {
		return nil, fmt.Errorf("machine_id is required")
	}
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("api_key is required")
	}
	if cfg.PollWait < 1 {
		cfg.PollWait = 60
	}
	if cfg.PollWait > 300 {
		cfg.PollWait = 300
	}
	if cfg.MaxTimeout < cfg.PollWait+10 {
		cfg.MaxTimeout = cfg.PollWait + 10
	}

	return cfg, nil
}

func WriteConfigTemplate(path string) error {
	cfg := DefaultConfig()
	cfg.Name, _ = os.Hostname()

	dir := filepath.Dir(path)
	os.MkdirAll(dir, 0755)

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
