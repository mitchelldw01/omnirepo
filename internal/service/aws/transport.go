package aws

import (
	"bytes"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type AwsTransport struct {
	client  *s3.Client
	bucket  string
	project string
}

func NewAwsTransport(client *s3.Client, project, bucket string) AwsTransport {
	return AwsTransport{
		client:  client,
		bucket:  bucket,
		project: project,
	}
}

func (at AwsTransport) Reader(key string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("stubbed response")), nil
}

func (at AwsTransport) Writer(key string) (io.WriteCloser, error) {
	return &stubWriteCloser{Buffer: new(bytes.Buffer)}, nil
}

type stubWriteCloser struct {
	*bytes.Buffer
}

func (swc *stubWriteCloser) Close() error {
	return nil
}
