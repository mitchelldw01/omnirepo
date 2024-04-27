package sys_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/mitchelldw01/omnirepo/internal/service/sys"
)

func TestLock(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("failed to determine home directory: %v", err)
	}
	path := filepath.Join(home, ".omni/cache/lock")

	t.Run("should create the lock when it doesn't exist", func(t *testing.T) {
		if err := deleteTestLock(path); err != nil {
			t.Fatal(err)
		}

		lock, err := sys.NewSystemLock()
		if err != nil {
			t.Fatal(err)
		}

		if err := lock.Lock(); err != nil {
			t.Fatal(err)
		}

		if _, err := os.ReadFile(path); os.IsNotExist(err) {
			t.Fatal("expected lock to exist")
		}
	})

	t.Run("should return an error when the lock is already acquired", func(t *testing.T) {
		if err := createTestLock(path); err != nil {
			t.Fatal(err)
		}

		lock, err := sys.NewSystemLock()
		if err != nil {
			t.Fatal(err)
		}

		if err := lock.Lock(); err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestUnlock(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("failed to determine home directory: %v", err)
	}
	path := filepath.Join(home, ".omni/cache/lock")

	t.Run("should remove the lock when it exists", func(t *testing.T) {
		if err := createTestLock(path); err != nil {
			t.Fatal(err)
		}

		lock, err := sys.NewSystemLock()
		if err != nil {
			t.Fatal(err)
		}

		if err := lock.Unlock(); err != nil {
			t.Fatal(err)
		}

		if _, err := os.Stat(path); err == nil {
			t.Fatal("expected to lock file not exist")
		}
	})

	t.Run("should return an error when the lock does not exist", func(t *testing.T) {
		if err := deleteTestLock(path); err != nil {
			t.Fatal(err)
		}

		lock, err := sys.NewSystemLock()
		if err != nil {
			t.Fatal(err)
		}

		if err := lock.Unlock(); err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func deleteTestLock(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}

	if err := os.Remove(path); err != nil {
		return fmt.Errorf("failed to delete test lock: %v", err)
	}

	return nil
}

func createTestLock(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create test lock: %v", err)
	}

	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		return fmt.Errorf("failed to create test lock: %v", err)
	}

	return nil
}
