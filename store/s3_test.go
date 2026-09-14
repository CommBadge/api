package store

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestNewS3Client(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		c, err := NewS3Client("minio.local:9000", "ak", "sk", "bucket", "us-east-1", false)
		if err != nil {
			t.Fatalf("NewS3Client: %v", err)
		}
		if c.bucket != "bucket" || c.useSSL {
			t.Fatalf("client = %+v", c)
		}
	})
	t.Run("empty endpoint", func(t *testing.T) {
		if _, err := NewS3Client("", "ak", "sk", "bucket", "us-east-1", false); err == nil {
			t.Fatal("expected error for empty endpoint")
		}
	})
}

func TestS3BucketExists(t *testing.T) {
	c, err := NewS3Client("minio.local:9000", "ak", "sk", "bucket", "us-east-1", false)
	if err != nil {
		t.Fatalf("NewS3Client: %v", err)
	}
	// No reachable server: the call must surface a connection error, not panic.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err = c.BucketExists(ctx)
	if err == nil {
		t.Fatal("expected error for unreachable bucket")
	}
	if !strings.Contains(err.Error(), "BucketExists") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestS3Upload(t *testing.T) {
	c, err := NewS3Client("minio.local:9000", "ak", "sk", "bucket", "us-east-1", false)
	if err != nil {
		t.Fatalf("NewS3Client: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := c.Upload(ctx, "k", strings.NewReader("data"), 4); err == nil {
		t.Fatal("expected error for unreachable bucket")
	}

	ssl, err := NewS3Client("minio.local:9000", "ak", "sk", "bucket", "us-east-1", true)
	if err != nil {
		t.Fatalf("NewS3Client: %v", err)
	}
	ctx2, cancel2 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel2()
	if _, err := ssl.Upload(ctx2, "k", strings.NewReader("data"), 4); err == nil {
		t.Fatal("expected error for unreachable bucket over ssl")
	}
}
