package usercfg

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type TargetConfig struct {
	Dependencies    []string                  `yaml:"dependencies"`
	WorkspaceAssets []string                  `yaml:"workspaceAssets"`
	Pipeline        map[string]PipelineConfig `yaml:"pipeline"`
}

type PipelineConfig struct {
	Command   string   `yaml:"command"`
	DependsOn []string `yaml:"dependsOn"`
	Includes  []string `yaml:"includes"`
	Excludes  []string `yaml:"excludes"`
	Outputs   []string `yaml:"outputs"`
}

func NewTargetConfig(dir string) (TargetConfig, error) {
	path := filepath.Join(dir, "omni-target.yaml")
	if _, err := os.Stat(path); err != nil {
		path = filepath.Join(dir, "omni-target.yml")
	}

	b, err := os.ReadFile(path)
	if err != nil {
		return TargetConfig{}, fmt.Errorf("failed to read %q: %v", path, err)
	}

	var cfg TargetConfig
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return TargetConfig{}, fmt.Errorf("failed to parse %q: %v", path, err)
	}

	return cfg, nil
}
