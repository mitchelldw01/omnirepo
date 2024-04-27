package cache

import (
	"errors"
	"os"

	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

func isNotExistError(err error) bool {
	var noSuchKeyError *types.NoSuchKey
	return os.IsNotExist(err) || errors.As(err, &noSuchKeyError)
}
