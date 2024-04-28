package run

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/mitchelldw01/omnirepo/internal/cache"
	"github.com/mitchelldw01/omnirepo/internal/exec"
	"github.com/mitchelldw01/omnirepo/internal/graph"
	"github.com/mitchelldw01/omnirepo/internal/log"
	"github.com/mitchelldw01/omnirepo/internal/service/aws"
	"github.com/mitchelldw01/omnirepo/internal/service/sys"
	"github.com/mitchelldw01/omnirepo/usercfg"
)

// Manages concurrent access to caches.
// It prevents users from accessing the cache when another user has acquired the lock.
type CacheLocker interface {
	Lock() error
	Unlock() error
}

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
	workCfg, err := usercfg.NewWorkspaceConfig()
	if err != nil {
		return err
	}

	lock, err := createCacheLock(workCfg)
	if err != nil {
		return err
	}

	if err := lock.Unlock(); err != nil {
		return err
	}

	fmt.Println("Lock removed successfully.")
	return nil
}

func runTreeCommand(tasks []string, opts Options) error {
	workCfg, targetCfgs, err := parseConfigs(opts.Target)
	if err != nil {
		return err
	}

	graph, err := createDependencyGraph(workCfg, targetCfgs, tasks, opts)
	if err != nil {
		return err
	}

	prettyJson, err := json.MarshalIndent(graph.ToMap(), "", "  ")
	if err != nil {
		return err
	}

	fmt.Println(string(prettyJson))
	return nil
}

func runRunCommand(tasks []string, opts Options) (err error) {
	workCfg, targetCfgs, err := parseConfigs(opts.Target)
	if err != nil {
		return err
	}

	lock, err := createCacheLock(workCfg)
	if err != nil {
		return err
	}
	if err := lock.Lock(); err != nil {
		return err
	}
	defer func() {
		if unlockErr := lock.Unlock(); err != nil {
			err = unlockErr
		}
	}()
	listenForInterrupts(lock)

	graph, err := createDependencyGraph(workCfg, targetCfgs, tasks, opts)
	if err != nil {
		return err
	}

	graph.ExecuteTasks()
	return nil
}

func createCacheLock(workCfg usercfg.WorkspaceConfig) (CacheLocker, error) {
	if !workCfg.RemoteCache.Enabled {
		return sys.NewSystemLock()
	}

	client, err := aws.NewDynamoClient(workCfg.Name, workCfg.RemoteCache.Region)
	if err != nil {
		return nil, err
	}

	return aws.NewAwsLock(client, workCfg.Name, workCfg.RemoteCache.Table), nil
}

func listenForInterrupts(lock CacheLocker) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	go func(lock CacheLocker) {
		<-ch
		if err := lock.Unlock(); err != nil {
			log.Fatal(err)
		}
	}(lock)
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
	if workCfg.RemoteCache.Enabled {
		ex, err = createAwsExecutor(workCfg, targetCfgs, opts.NoCache)
	} else {
		ex, err = createSystemExecutor(workCfg.Targets, targetCfgs, opts.NoCache)
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

func createAwsExecutor(
	workCfg usercfg.WorkspaceConfig,
	targetCfgs map[string]usercfg.TargetConfig,
	noCache bool,
) (graph.Executor, error) {
	trans, err := createAwsTransport(workCfg)
	if err != nil {
		return nil, err
	}

	r := cache.NewCacheReader(trans, targetCfgs, workCfg.Targets)
	w := cache.NewCacheWriter(trans, r)
	if err := cache.Init(); err != nil {
		return nil, err
	}

	return exec.NewExecutor(r, w, noCache), nil
}

func createSystemExecutor(
	targets []string,
	targetCfgs map[string]usercfg.TargetConfig,
	noCache bool,
) (graph.Executor, error) {
	trans := sys.NewSystemTransport()
	r := cache.NewCacheReader(trans, targetCfgs, targets)
	w := cache.NewCacheWriter(trans, r)
	if err := cache.Init(); err != nil {
		return nil, err
	}

	return exec.NewExecutor(r, w, noCache), nil
}

func createAwsTransport(workCfg usercfg.WorkspaceConfig) (*aws.AwsTransport, error) {
	s3Client, err := aws.NewS3Client(workCfg.Name, workCfg.RemoteCache.Region)
	if err != nil {
		return nil, err
	}
	trans := aws.NewAwsTransport(s3Client, workCfg.Name, workCfg.RemoteCache.Bucket)
	return trans, nil
}
