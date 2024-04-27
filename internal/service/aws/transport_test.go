package aws_test

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	omniAws "github.com/mitchelldw01/omnirepo/internal/service/aws"
)

const (
	key  = "test.txt"
	body = "lorem ipsum dolor sit amet"
)

func TestReader(t *testing.T) {
	project, bucket := "omnirepo", "omnirepo"
	helper, err := newTransportTestHelper(project, bucket)
	if err != nil {
		t.Fatal(err)
	}
	if err := helper.createTestObject(); err != nil {
		t.Fatal(err)
	}

	trans := omniAws.NewAwsTransport(helper.client, project, bucket)
	r, err := trans.Reader(key)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	buf := new(strings.Builder)
	if _, err = io.Copy(buf, r); err != nil {
		t.Fatalf("failed to read from reader: %v", err)
	}

	if res := buf.String(); res != body {
		t.Fatalf("expected %q, got %q", body, res)
	}
}

func TestWriter(t *testing.T) {
	project, bucket := "omnirepo", "omnirepo"
	helper, err := newTransportTestHelper(project, bucket)
	if err != nil {
		t.Fatal(err)
	}
	if err := helper.deleteTestObject(); err != nil {
		t.Fatal(err)
	}

	trans := omniAws.NewAwsTransport(helper.client, project, bucket)
	w, err := trans.Writer(key)
	if err != nil {
		t.Fatal(err)
	}
	_, err = w.Write([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	buf, err := helper.readTestObject()
	if err != nil {
		t.Fatal(err)
	}

	if res := buf.String(); res != body {
		t.Errorf("expected %v, got %v", body, res)
	}
}

type s3EndpointResolver struct{}

func (er *s3EndpointResolver) ResolveEndpoint(service, region string) (aws.Endpoint, error) {
	return aws.Endpoint{
		PartitionID:       "aws",
		URL:               "http://localhost:9000",
		SigningRegion:     "us-east-1",
		HostnameImmutable: true,
	}, nil
}

type transportTestHelper struct {
	client  *s3.Client
	project string
	bucket  string
}

func newTransportTestHelper(project, bucket string) (*transportTestHelper, error) {
	accessKeyId := os.Getenv("MINIO_ROOT_USER")
	secretAccessKey := os.Getenv("MINIO_ROOT_PASSWORD")

	client := s3.NewFromConfig(aws.Config{
		Region:           "us-east-1",
		Credentials:      credentials.NewStaticCredentialsProvider(accessKeyId, secretAccessKey, ""),
		EndpointResolver: &s3EndpointResolver{},
	}, func(o *s3.Options) {
		o.UsePathStyle = true
	})

	if err := createTestBucket(client, bucket); err != nil {
		return nil, err
	}

	return &transportTestHelper{
		client:  client,
		project: project,
		bucket:  bucket,
	}, nil
}

func createTestBucket(client *s3.Client, bucket string) error {
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

func (tth *transportTestHelper) createTestObject() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := tth.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(tth.bucket),
		Key:    aws.String(path.Join(tth.project, key)),
		Body:   strings.NewReader(body),
	})
	if err != nil {
		return fmt.Errorf("failed to create  object: %v", err)
	}

	return nil
}

func (tth *transportTestHelper) deleteTestObject() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := tth.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(tth.bucket),
		Key:    aws.String(path.Join(tth.project, key)),
	})
	if err != nil {
		return nil
	}

	_, err = tth.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(tth.bucket),
		Key:    aws.String(path.Join(tth.project, key)),
	})
	if err == nil {
		return nil
	}

	return fmt.Errorf("failed to delete object: %v", err)
}

func (tth *transportTestHelper) readTestObject() (*strings.Builder, error) {
	r, err := tth.client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(tth.bucket),
		Key:    aws.String(path.Join(tth.project, key)),
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
