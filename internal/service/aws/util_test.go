package aws_test

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

const (
	bucket  = "omni"
	project = "omni"
)

type endpointResolver struct{}

func (er *endpointResolver) ResolveEndpoint(service, region string) (aws.Endpoint, error) {
	return aws.Endpoint{
		PartitionID:       "aws",
		URL:               "http://localhost:9000",
		SigningRegion:     "us-east-1",
		HostnameImmutable: true,
	}, nil
}

type awsTester struct {
	client *s3.Client
}

func newAwsTester() (*awsTester, error) {
	accessKeyId := os.Getenv("MINIO_ROOT_USER")
	secretAccessKey := os.Getenv("MINIO_ROOT_PASSWORD")

	client := s3.NewFromConfig(aws.Config{
		Region:           "us-east-1",
		Credentials:      credentials.NewStaticCredentialsProvider(accessKeyId, secretAccessKey, ""),
		EndpointResolver: &endpointResolver{},
	}, func(o *s3.Options) {
		o.UsePathStyle = true
	})

	if err := createTestBucket(client); err != nil {
		return nil, err
	}

	return &awsTester{client: client}, nil
}

func createTestBucket(client *s3.Client) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(bucket),
	}); err == nil {
		return nil
	}

	_, err := client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		return fmt.Errorf("failed to create test bucket: %v", err)
	}
	return nil
}

func (t *awsTester) createTestObject() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := t.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(path.Join(project, key)),
		Body:   strings.NewReader(body),
	})
	if err != nil {
		return fmt.Errorf("failed to create  object: %v", err)
	}

	return nil
}

func (t *awsTester) deleteTestObject() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := t.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(path.Join(project, key)),
	})
	if err != nil {
		return nil
	}

	_, err = t.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(path.Join(project, key)),
	})
	if err == nil {
		return nil
	}

	return fmt.Errorf("failed to delete object: %v", err)
}

func (t *awsTester) readTestObject() (*strings.Builder, error) {
	r, err := t.client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(path.Join(project, key)),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get object: %v", err)
	}
	defer r.Body.Close()

	buf := new(strings.Builder)
	_, err = io.Copy(buf, r.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	return buf, nil
}
