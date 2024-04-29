package cache_test

import (
	"archive/tar"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/klauspost/compress/zstd"
	"github.com/mitchelldw01/omnirepo/internal/cache"
	"github.com/mitchelldw01/omnirepo/internal/graph"
	"github.com/mitchelldw01/omnirepo/internal/service/sys"
	"github.com/mitchelldw01/omnirepo/usercfg"
)

var (
	configs = map[string]usercfg.TargetConfig{
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
	deps  = map[string]struct{}{"bar:test": {}}
	node  = graph.NewNode("test", "foo", configs["foo"].Pipeline["test"])
	trans = sys.NewSystemTransport()
)

func TestWriteTaskResult(t *testing.T) {
	trans := sys.NewSystemTransport()
	cr := cache.NewCacheReader(trans, nil, nil, false)
	cw := cache.NewCacheWriter(trans, cr)

	dir, name := "dir", "name"
	exp := cache.NewTaskResult("logs", false)
	if err := cw.WriteTaskResult(dir, name, exp); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(os.TempDir(), "omni-next-cache", dir, "results", name+".json")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read task result: %v", err)
	}

	var res cache.TaskResult
	if err := json.Unmarshal(b, &res); err != nil {
		t.Fatalf("failed to marshal task result: %v", err)
	}

	if !reflect.DeepEqual(exp, res) {
		t.Fatalf("expected %v, got %v", exp, res)
	}
}

func TestUpdate(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current directory: %v", err)
	}

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
	if _, err := cr.Validate(node, deps); err != nil {
		t.Fatal(err)
	}

	// remove task output to test that it gets restored
	output := filepath.Join(work, "foo/output.txt")
	if err := os.Remove(output); err != nil {
		t.Fatalf("failed to remove %q: %v", output, err)
	}

	cw := cache.NewCacheWriter(trans, cr)
	if err := cw.Update(); err != nil {
		t.Fatal(err)
	}

	t.Run("should create the workspace.json", func(t *testing.T) {
		path := filepath.Join(work, ".omni/cache/workspace.json")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Fatalf("expected %q to exist", path)
		}
	})

	t.Run("should create the foo-meta.tar.zst with the correct contents", func(t *testing.T) {
		path := filepath.Join(work, ".omni/cache/foo-meta.tar.zst")
		headers := []string{"inputs.json", "outputs/output.txt", "results/test.json"}
		ok, err := checkTarZstContents(path, headers)
		if err != nil {
			t.Fatalf("failed to verify tar contents: %v", err)
		}
		if !ok {
			t.Fatal(err)
		}
	})

	t.Run("should create the bar-meta.tar.zst with the correct contents", func(t *testing.T) {
		path := filepath.Join(work, ".omni/cache/bar-meta.tar.zst")
		headers := []string{"inputs.json", "results/test.json"}
		ok, err := checkTarZstContents(path, headers)
		if err != nil {
			t.Fatalf("failed to verify tar contents: %v", err)
		}
		if !ok {
			t.Fatal(err)
		}
	})

	t.Run("should restore cached outputs", func(t *testing.T) {
		if _, err := os.Stat(output); os.IsNotExist(err) {
			t.Fatalf("expected %q to exist", output)
		}
	})
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

func checkTarZstContents(path string, headers []string) (bool, error) {
	reader, decoder, file, err := setupTarZstReader(path)
	if err != nil {
		return false, err
	}
	defer file.Close()
	defer decoder.Close()

	pathMap, err := populatePathMapFromTar(reader)
	if err != nil {
		return false, err
	}

	if ok := checkAllContentsExist(pathMap, headers); !ok {
		return false, err
	}

	return true, nil
}

func setupTarZstReader(path string) (*tar.Reader, *zstd.Decoder, *os.File, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to open file: %v", err)
	}

	decoder, err := zstd.NewReader(file)
	if err != nil {
		file.Close()
		return nil, nil, nil, fmt.Errorf("failed to create zstd reader: %v", err)
	}

	return tar.NewReader(decoder), decoder, file, nil
}

func populatePathMapFromTar(r *tar.Reader) (map[string]struct{}, error) {
	pathMap := make(map[string]struct{})
	for {
		header, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read tar entry: %w", err)
		}
		pathMap[header.Name] = struct{}{}
	}
	return pathMap, nil
}

func checkAllContentsExist(pathMap map[string]struct{}, headers []string) bool {
	for _, header := range headers {
		if _, exists := pathMap[header]; !exists {
			return false
		}
	}
	return true
}
