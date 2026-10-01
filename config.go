package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// Target is one thing we want to monitor.
type Target struct {
	Name string `json:"name"`
	Host string `json:"host"`
	Port int    `json:"port"`
}

// Config is the whole targets.json file.
type Config struct {
	TimeoutMS int      `json:"timeout_ms"`
	Targets   []Target `json:"targets"`
}

func (c Config) Timeout() time.Duration {
	if c.TimeoutMS <= 0 {
		return 3 * time.Second
	}
	return time.Duration(c.TimeoutMS) * time.Millisecond
}

func loadConfig(path string) (Config, error) {
	var cfg Config
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("reading %s: %w", path, err)
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parsing %s: %w", path, err)
	}
	if len(cfg.Targets) == 0 {
		return cfg, fmt.Errorf("%s has no targets", path)
	}
	return cfg, nil
}
