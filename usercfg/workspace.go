package usercfg

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type WorkspaceConfig struct {
	Name        string            `yaml:"name"`
	Targets     []string          `yaml:"targets"`
	RemoteCache RemoteCacheConfig `yaml:"remoteCache"`
}

type RemoteCacheConfig struct {
	Enabled bool   `yaml:"enabled"`
	Bucket  string `yaml:"bucket"`
	Table   string `yaml:"table"`
	Region  string `yaml:"region"`
}

func NewWorkspaceConfig() (WorkspaceConfig, error) {
	path := "omni-workspace.yaml"
	if _, err := os.Stat(path); err != nil {
		path = "omni-workspace.yml"
	}

	b, err := os.ReadFile(path)
	if err != nil {
		return WorkspaceConfig{}, fmt.Errorf("failed to read %q: %v", path, err)
	}

	var cfg WorkspaceConfig
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return WorkspaceConfig{}, fmt.Errorf("failed to parse %q: %v", path, err)
	}

	return cfg, validateWorkspaceConfig(cfg)
}

func validateWorkspaceConfig(cfg WorkspaceConfig) error {
	if cfg.Name == "" {
		return errors.New("workspace name is not defined in config")
	}
	if !cfg.RemoteCache.Enabled {
		return nil
	}
	if cfg.RemoteCache.Bucket == "" {
		return fmt.Errorf("bucket name is not defined in workspace config")
	}
	if cfg.RemoteCache.Table == "" {
		return fmt.Errorf("table name is not defined in workspace config")
	}
	return nil
}
