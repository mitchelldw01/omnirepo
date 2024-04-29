package cache_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mitchelldw01/omnirepo/internal/cache"
	"github.com/mitchelldw01/omnirepo/internal/graph"
	"github.com/mitchelldw01/omnirepo/internal/service/sys"
	"github.com/mitchelldw01/omnirepo/usercfg"
)

func TestGetCachedResult(t *testing.T) {
	prev, err := createPrevCacheDir()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(prev)

	dir, name := "dir", "name"
	exp := cache.NewTaskResult("logs", false)
	if err := createTestTaskResult(prev, dir, name, exp); err != nil {
		t.Fatal(err)
	}

	trans := sys.NewSystemTransport()
	cr := cache.NewCacheReader(trans, map[string]usercfg.TargetConfig{}, []string{}, false)
	res, err := cr.GetCachedResult(dir, name)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(exp, res) {
		t.Fatalf("expected %v, got %v", exp, res)
	}
}

func TestValidate(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current directory: %v", err)
	}

	t.Run("should return true when the cache is valid", func(t *testing.T) {
		prev, err := createPrevCacheDir()
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(prev)
		defer func() {
			if err := os.Chdir(wd); err != nil {
				t.Logf("failed to reset working directory: %v", err)
			}
		}()

		work, err := createTestWorkspace()
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(work)

		cr := cache.NewCacheReader(trans, configs, []string{"foo", "bar"}, false)
		valid, err := cr.Validate(node, deps)
		if err != nil {
			t.Fatal(err)
		}

		if valid != true {
			t.Fatalf("expected %v, got %v", true, valid)
		}
	})

	t.Run("should return false when the workspace cache is invalid", func(t *testing.T) {
		prev, err := createPrevCacheDir()
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(prev)
		defer func() {
			if err := os.Chdir(wd); err != nil {
				t.Logf("failed to reset working directory: %v", err)
			}
		}()

		work, err := createTestWorkspace()
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(work)

		path := filepath.Join(work, "workspace.txt")
		if err := os.WriteFile(path, []byte("test"), 0o644); err != nil {
			t.Fatalf("failed to invalidate workspace cache: %v", err)
		}

		cr := cache.NewCacheReader(trans, configs, []string{"foo", "bar"}, false)
		valid, err := cr.Validate(node, deps)
		if err != nil {
			t.Fatal(err)
		}

		if valid != false {
			t.Fatalf("expected %v, got %v", false, valid)
		}
	})

	t.Run("should return false when the target cache is invalid", func(t *testing.T) {
		prev, err := createPrevCacheDir()
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(prev)
		defer func() {
			if err := os.Chdir(wd); err != nil {
				t.Logf("failed to reset working directory: %v", err)
			}
		}()

		work, err := createTestWorkspace()
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(work)

		path := filepath.Join(work, "foo/include.txt")
		if err := os.WriteFile(path, []byte("test"), 0o644); err != nil {
			t.Fatalf("failed to invalidate target cache: %v", err)
		}

		cr := cache.NewCacheReader(trans, configs, []string{"foo", "bar"}, false)
		valid, err := cr.Validate(node, deps)
		if err != nil {
			t.Fatal(err)
		}

		if valid != false {
			t.Fatalf("expected %v, got %v", false, valid)
		}
	})

	t.Run("should return false when the cache of a dependency is invalid", func(t *testing.T) {
		prev, err := createPrevCacheDir()
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(prev)
		defer func() {
			if err := os.Chdir(wd); err != nil {
				t.Logf("failed to reset working directory: %v", err)
			}
		}()

		work, err := createTestWorkspace()
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(work)

		path := filepath.Join(work, "bar/include.txt")
		if err := os.WriteFile(path, []byte("test"), 0o644); err != nil {
			t.Fatalf("failed to invalidate target cache: %v", err)
		}

		cr := cache.NewCacheReader(trans, configs, []string{"foo", "bar"}, false)
		depNode := graph.NewNode("test", "bar", configs["bar"].Pipeline["test"])
		if _, err := cr.Validate(depNode, map[string]struct{}{}); err != nil {
			t.Fatal(err)
		}

		valid, err := cr.Validate(node, deps)
		if err != nil {
			t.Fatal(err)
		}

		if valid != false {
			t.Fatalf("expected %v, got %v", false, valid)
		}
	})

	t.Run("should return true when only an excluded file is invalid", func(t *testing.T) {
		prev, err := createPrevCacheDir()
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(prev)
		defer func() {
			if err := os.Chdir(wd); err != nil {
				t.Logf("failed to reset working directory: %v", err)
			}
		}()

		work, err := createTestWorkspace()
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(work)

		path := filepath.Join(work, "foo/exclude.txt")
		if err := os.WriteFile(path, []byte("test"), 0o644); err != nil {
			t.Fatalf("failed to invalidate target cache: %v", err)
		}

		cr := cache.NewCacheReader(trans, configs, []string{"foo", "bar"}, false)
		valid, err := cr.Validate(node, deps)
		if err != nil {
			t.Fatal(err)
		}

		if valid != true {
			t.Fatalf("expected %v, got %v", true, valid)
		}
	})
}

func createPrevCacheDir() (string, error) {
	tmp := filepath.Join(os.TempDir(), "omni-prev-cache")
	if err := os.RemoveAll(tmp); err != nil {
		return "", err
	}

	if err := os.MkdirAll(tmp, 0o755); err != nil {
		return "", err
	}

	return tmp, nil
}

func createTestTaskResult(tmp, dir, name string, res cache.TaskResult) error {
	path := filepath.Join(tmp, dir, "results", name+".json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("failed to create test directories: %v", err)
	}

	b, err := json.Marshal(res)
	if err != nil {
		return fmt.Errorf("failed to marshal test task result: %v", err)
	}

	if err := os.WriteFile(path, b, 0o644); err != nil {
		return fmt.Errorf("failed to write test file: %v", err)
	}

	return nil
}
