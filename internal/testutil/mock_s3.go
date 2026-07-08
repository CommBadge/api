package testutil

import (
	"context"
	"io"
	"sync"
)

type MockS3Client struct {
	mu          sync.Mutex
	UploadURL   string
	UploadErr   error
	BucketFound bool
	BucketErr   error
}

func NewMockS3Client() *MockS3Client {
	return &MockS3Client{
		UploadURL:   "https://s3.example.com/bucket/key.png",
		BucketFound: true,
	}
}

func (m *MockS3Client) Upload(ctx context.Context, key string, reader io.Reader, size int64) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.UploadURL, m.UploadErr
}

func (m *MockS3Client) BucketExists(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.BucketFound {
		return m.BucketErr
	}
	return nil
}
