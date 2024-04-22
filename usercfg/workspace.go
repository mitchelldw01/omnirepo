package usercfg

import (
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
	Bucket string `yaml:"bucket"`
	Region string `yaml:"region"`
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

	return cfg, nil
}
