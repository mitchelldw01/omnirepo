package run

import (
	"maps"
	"path/filepath"

	"github.com/mitchelldw01/omnirepo/usercfg"
)

type Options struct {
	Graph   bool
	Help    bool
	NoCache bool
	NoColor bool
	Remote  bool
	Target  string
	Version bool
}

func RunCommand(cmd string, tasks []string, opts Options) error {
	switch cmd {
	case "unlock":
		return runUnlockCommand()
	case "tree":
		return runTreeCommand(tasks, opts)
	default:
		return runRunCommand(tasks, opts)
	}
}

func runUnlockCommand() error {
	return nil
}

func runTreeCommand(tasks []string, opts Options) error {
	_, _ = tasks, opts
	_, _, err := parseConfigs(opts.Target)
	if err != nil {
		return err
	}
	return nil
}

func runRunCommand(tasks []string, opts Options) error {
	_, _ = tasks, opts
	return nil
}

func parseConfigs(dir string) (usercfg.WorkspaceConfig, map[string]usercfg.TargetConfig, error) {
	workCfg, err := usercfg.NewWorkspaceConfig()
	if err != nil {
		return usercfg.WorkspaceConfig{}, nil, err
	}

	var targetCfgs map[string]usercfg.TargetConfig
	if dir == "" {
		targetCfgs, err = parseAllTargetConfigs(workCfg.Targets)
	} else {
		targetCfgs, err = parseDependentTargetConfigs(dir)
	}
	if err != nil {
		return usercfg.WorkspaceConfig{}, nil, err
	}

	return workCfg, targetCfgs, nil
}

func parseAllTargetConfigs(dirs []string) (map[string]usercfg.TargetConfig, error) {
	cfgs := make(map[string]usercfg.TargetConfig, len(dirs))

	for _, dir := range dirs {
		cfg, err := usercfg.NewTargetConfig(dir)
		if err != nil {
			return nil, err
		}
		cfgs[filepath.Clean(dir)] = cfg
	}

	return cfgs, nil
}

func parseDependentTargetConfigs(dir string) (map[string]usercfg.TargetConfig, error) {
	cfgs := map[string]usercfg.TargetConfig{}
	cfg, err := usercfg.NewTargetConfig(dir)
	if err != nil {
		return nil, err
	}
	cfgs[filepath.Clean(dir)] = cfg

	for _, dep := range cfg.Dependencies {
		depCfgs, err := parseDependentTargetConfigs(dep)
		if err != nil {
			return nil, err
		}
		maps.Copy(cfgs, depCfgs)
	}

	return cfgs, nil
}
