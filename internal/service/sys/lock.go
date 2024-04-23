package sys

import "os"

type SystemLock struct {
	path string
}

func NewSystemLock() (SystemLock, error) {
	return SystemLock{path: ""}, nil
}

func (l SystemLock) Lock() error {
	return nil
}

func (l SystemLock) Unlock() error {
	return os.Remove(l.path)
}
