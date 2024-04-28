package sys_test

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mitchelldw01/omnirepo/internal/service/sys"
)

const (
	key  = "test.txt"
	body = "lorem ipsum dolor sit amet"
)

func TestReader(t *testing.T) {
	t.Run("should read the file when it exists", func(t *testing.T) {
		dir, err := changeWorkingDirectory()
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(dir)

		if err := createTestFile(dir); err != nil {
			t.Fatal(err)
		}

		r, err := sys.NewSystemTransport().Reader(key)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Close()

		buf := strings.Builder{}
		if _, err = io.Copy(&buf, r); err != nil {
			t.Fatalf("failed to read from reader: %v", err)
		}

		if res := buf.String(); res != body {
			t.Errorf("expected %q, got %q", body, res)
		}
	})

	t.Run("should return an error when the file does not exist", func(t *testing.T) {
		dir, err := changeWorkingDirectory()
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(dir)

		_, err = sys.NewSystemTransport().Reader(key)
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})
}

func TestWriter(t *testing.T) {
	dir, err := changeWorkingDirectory()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	w, err := sys.NewSystemTransport().Writer(key)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	_, err = w.Write([]byte(body))
	if err != nil {
		t.Fatalf("failed to write to file: %v", err)
	}

	b, err := os.ReadFile(filepath.Join(".omni/cache", key))
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	res := string(b)
	if body != res {
		t.Errorf("expected %q, got %q", body, res)
	}
}

func changeWorkingDirectory() (string, error) {
	dir, err := os.MkdirTemp("", "omnirepo-")
	if err != nil {
		return "", fmt.Errorf("failed to create test directory: %v", err)
	}

	if err := os.Chdir(dir); err != nil {
		return "", fmt.Errorf("failed to change directory: %v", err)
	}

	return dir, nil
}

func createTestFile(dir string) error {
	path := filepath.Join(dir, ".omni/cache", key)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("failed to create test directory: %v", err)
	}

	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return fmt.Errorf("failed to create test file: %v", err)
	}

	return nil
}
