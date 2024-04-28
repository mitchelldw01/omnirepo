package cache

import (
	"fmt"
	"os"
	"path/filepath"
)

func Init() error {
	prevDir := filepath.Join(os.TempDir(), "omni-prev-cache")
	nextDir := filepath.Join(os.TempDir(), "omni-next-cache")

	if err := os.RemoveAll(prevDir); err != nil {
		return fmt.Errorf("failed to initialize cache: %v", err)
	}

	if err := os.RemoveAll(nextDir); err != nil {
		return fmt.Errorf("failed to initialize cache: %v", err)
	}

	if err := os.Mkdir(nextDir, 0o755); err != nil {
		return fmt.Errorf("failed to initialize cache: %v", err)
	}

	return nil
}
