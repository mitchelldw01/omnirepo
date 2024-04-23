package aws_test

import (
	"io"
	"strings"
	"testing"

	omniAws "github.com/mitchelldw01/omnirepo/internal/service/aws"
)

const (
	key  = "test.txt"
	body = "lorem ipsum dolor sit amet"
)

func TestReader(t *testing.T) {
	tester, err := newAwsTester()
	if err != nil {
		t.Fatal(err)
	}
	if err := tester.createTestObject(); err != nil {
		t.Fatal(err)
	}

	trans, err := omniAws.NewAwsTransport(tester.client, project, bucket)
	if err != nil {
		t.Fatal(err)
	}
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
	tester, err := newAwsTester()
	if err != nil {
		t.Fatal(err)
	}
	if err := tester.deleteTestObject(); err != nil {
		t.Fatal(err)
	}

	trans, err := omniAws.NewAwsTransport(tester.client, project, bucket)
	if err != nil {
		t.Fatal(err)
	}
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

	buf, err := tester.readTestObject()
	if err != nil {
		t.Fatal(err)
	}

	if res := buf.String(); res != body {
		t.Errorf("expected %v, got %v", body, res)
	}
}
