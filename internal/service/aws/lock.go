package aws

import "github.com/mitchelldw01/omnirepo/usercfg"

type AwsLock struct{}

func NewAwsLock(rc usercfg.RemoteCacheConfig) (AwsLock, error) {
	return AwsLock{}, nil
}

func (al AwsLock) Lock() error {
	return nil
}

func (al AwsLock) Unlock() error {
	return nil
}
