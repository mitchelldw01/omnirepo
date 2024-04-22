package run

import (
	"maps"
	"path/filepath"

	"github.com/mitchelldw01/omnirepo/internal/cache"
	"github.com/mitchelldw01/omnirepo/internal/exec"
	"github.com/mitchelldw01/omnirepo/internal/graph"
	"github.com/mitchelldw01/omnirepo/internal/service/aws"
	"github.com/mitchelldw01/omnirepo/internal/service/sys"
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
	workCfg, targetCfgs, err := parseConfigs(opts.Target)
	if err != nil {
		return err
	}

	graph, err := createDependencyGraph(workCfg, targetCfgs, tasks, opts)
	if err != nil {
		return err
	}

	graph.ExecuteTasks()
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
	targetCfgs := make(map[string]usercfg.TargetConfig, len(dirs))

	for _, dir := range dirs {
		cfg, err := usercfg.NewTargetConfig(dir)
		if err != nil {
			return nil, err
		}
		targetCfgs[filepath.Clean(dir)] = cfg
	}

	return targetCfgs, nil
}

func parseDependentTargetConfigs(dir string) (map[string]usercfg.TargetConfig, error) {
	targetCfgs := map[string]usercfg.TargetConfig{}
	cfg, err := usercfg.NewTargetConfig(dir)
	if err != nil {
		return nil, err
	}
	targetCfgs[filepath.Clean(dir)] = cfg

	for _, dep := range cfg.Dependencies {
		depCfgs, err := parseDependentTargetConfigs(dep)
		if err != nil {
			return nil, err
		}
		maps.Copy(targetCfgs, depCfgs)
	}

	return targetCfgs, nil
}

func createDependencyGraph(
	workCfg usercfg.WorkspaceConfig,
	targetCfgs map[string]usercfg.TargetConfig,
	tasks []string,
	opts Options,
) (*graph.DependencyGraph, error) {
	var ex graph.Executor
	var err error
	if workCfg.RemoteCache.Bucket == "" {
		ex, err = createSystemExecutor(targetCfgs, opts.NoCache)
	} else {
		ex, err = createAwsExecutor(workCfg, targetCfgs, opts.NoCache)
	}
	if err != nil {
		return nil, err
	}

	graph := graph.NewDependencyGraph(ex, targetCfgs)
	if err := graph.PopulateNodes(tasks, opts.Target); err != nil {
		return nil, err
	}

	return graph, nil
}

func createSystemExecutor(targetCfgs map[string]usercfg.TargetConfig, noCache bool) (graph.Executor, error) {
	trans := sys.NewSystemTransport()
	locker, err := sys.NewSystemLock()
	if err != nil {
		return nil, err
	}

	cacher := cache.NewCache(trans, locker, targetCfgs)
	return exec.NewExecutor(cacher, noCache), nil
}

func createAwsExecutor(
	workCfg usercfg.WorkspaceConfig,
	targetCfg map[string]usercfg.TargetConfig,
	noCache bool,
) (graph.Executor, error) {
	client, err := aws.NewAwsClient(workCfg.Name, workCfg.RemoteCache.Region)
	if err != nil {
		return nil, err
	}
	trans := aws.NewAwsTransport(client, workCfg.Name, workCfg.RemoteCache.Bucket)

	locker, err := aws.NewAwsLock(workCfg.RemoteCache)
	if err != nil {
		return nil, err
	}

	cacher := cache.NewCache(trans, locker, targetCfg)
	return exec.NewExecutor(cacher, noCache), nil
}
