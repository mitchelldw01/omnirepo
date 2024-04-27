package sys

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type SystemTransport struct {
	workspace string
}

func NewSystemTransport(workspace string) SystemTransport {
	return SystemTransport{workspace: workspace}
}

func (st SystemTransport) Reader(path string) (io.ReadCloser, error) {
	r, err := os.Open(filepath.Join(".omni/cache", st.workspace, path))
	if err != nil {
		return nil, fmt.Errorf("failed to read cache asset: %v", err)
	}
	return r, nil
}

func (st SystemTransport) Writer(path string) (io.WriteCloser, error) {
	dst := filepath.Join(".omni/cache", st.workspace, path)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return nil, fmt.Errorf("failed to write cache asset: %v", err)
	}

	return os.Create(dst)
}
