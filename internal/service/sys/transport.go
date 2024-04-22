package sys

import (
	"bytes"
	"io"
	"strings"
)

type SystemTransport struct{}

func NewSystemTransport() SystemTransport {
	return SystemTransport{}
}

func (st SystemTransport) Reader(path string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("stubbed response")), nil
}

func (st SystemTransport) Writer(path string) (io.WriteCloser, error) {
	return &stubWriteCloser{Buffer: new(bytes.Buffer)}, nil
}

type stubWriteCloser struct {
	*bytes.Buffer
}

func (swc *stubWriteCloser) Close() error {
	return nil
}
