package aws

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
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

func (t AwsTransport) Reader(key string) (io.ReadCloser, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	res, err := t.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(t.bucket),
		Key:    aws.String(path.Join(t.project, key)),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to read cache asset: %v", err)
	}

	return res.Body, nil
}

func (t AwsTransport) Writer(key string) (io.WriteCloser, error) {
	tmp, err := os.CreateTemp("", "aws-")
	if err != nil {
		return nil, fmt.Errorf("failed to write cache asset: %v", err)
	}

	return &AwsUploader{
		client: t.client,
		file:   tmp,
		bucket: t.bucket,
		key:    path.Join(t.project, key),
	}, nil
}

type AwsUploader struct {
	client *s3.Client
	file   *os.File
	bucket string
	key    string
}

func (u *AwsUploader) Write(b []byte) (int, error) {
	return u.file.Write(b)
}

func (u *AwsUploader) Close() error {
	defer u.file.Close()
	if _, err := u.file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to write cache asset: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	_, err := u.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(u.bucket),
		Key:    aws.String(u.key),
		Body:   u.file,
	})
	if err != nil {
		return fmt.Errorf("failed to write cache asset: %v", err)
	}

	return nil
}
