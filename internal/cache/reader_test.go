package cache_test

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
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
	tmp, err := createTempDir()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	dir, name := "dir", "name"
	exp := cache.NewTaskResult("logs", false)
	if err := createTestTaskResult(tmp, dir, name, exp); err != nil {
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
	configs := map[string]usercfg.TargetConfig{
		"foo": {
			WorkspaceAssets: []string{"workspace.txt"},
			Pipeline: map[string]usercfg.PipelineConfig{
				"test": {
					DependsOn: []string{"^test"},
					Includes:  []string{"*.txt"},
					Excludes:  []string{"exclude.txt"},
					Outputs:   []string{"output.txt"},
				},
			},
		},
		"bar": {
			WorkspaceAssets: []string{"workspace.txt"},
			Pipeline: map[string]usercfg.PipelineConfig{
				"test": {
					Includes: []string{"*.txt"},
				},
			},
		},
	}

	deps := map[string]struct{}{"bar:test": {}}
	node := graph.NewNode("test", "foo", configs["foo"].Pipeline["test"])
	trans := sys.NewSystemTransport()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current directory: %v", err)
	}

	t.Run("should return true when the cache is valid", func(t *testing.T) {
		tmp, err := createTempDir()
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tmp)
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
		tmp, err := createTempDir()
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tmp)
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
		tmp, err := createTempDir()
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tmp)
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
		tmp, err := createTempDir()
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tmp)
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
		tmp, err := createTempDir()
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tmp)
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

func createTempDir() (string, error) {
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

func createTestWorkspace() (string, error) {
	dst, err := os.MkdirTemp("", "test-")
	if err != nil {
		return "", fmt.Errorf("failed to create test directory: %v", err)
	}

	if err := copyTestDirectory("../../testdata", dst); err != nil {
		return "", fmt.Errorf("failed to copy test directory: %v", err)
	}

	if err := os.Chdir(dst); err != nil {
		return "", fmt.Errorf("failed to change directory: %v", err)
	}

	return dst, nil
}

func copyTestDirectory(src, dst string) error {
	return filepath.Walk(src, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dst, rel)

		if !info.IsDir() {
			return copyTestFile(path, dstPath)
		}

		return os.MkdirAll(dstPath, 0o755)
	})
}

func copyTestFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}
