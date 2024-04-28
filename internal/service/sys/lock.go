package sys

import (
	"fmt"
	"os"
	"path/filepath"
)

type SystemLock struct {
	path string
}

func NewSystemLock() (SystemLock, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return SystemLock{}, fmt.Errorf("failed to initialize cache lock: %v", err)
	}

	path := filepath.Join(home, ".omni/cache/lock")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return SystemLock{}, fmt.Errorf("failed to create cache directory %q: %v", path, err)
	}

	return SystemLock{path: path}, nil
}

func (l SystemLock) Lock() error {
	file, err := os.OpenFile(l.path, os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("lock is already acquired... run 'omni unlock' to cancel")
		}
		return err
	}

	file.Close()
	return nil
}

func (l SystemLock) Unlock() error {
	err := os.Remove(l.path)
	if os.IsNotExist(err) {
		return fmt.Errorf("cannot free lock because it is not currently acquired")
	}
	if err != nil {
		return fmt.Errorf("failed to unlock cache: %v", err)
	}

	return nil
}
