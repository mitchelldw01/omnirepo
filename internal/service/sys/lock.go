package sys

import "os"

type SystemLock struct {
	path string
}

func NewSystemLock() (SystemLock, error) {
	return SystemLock{path: ""}, nil
}

func (sl SystemLock) Lock() error {
	return nil
}

func (sl SystemLock) Unlock() error {
	return os.Remove(sl.path)
}
