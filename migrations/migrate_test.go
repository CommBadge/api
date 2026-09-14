package migrations

import (
	"context"
	"testing"
	"time"
)

func TestLatest(t *testing.T) {
	v, err := Latest()
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if v <= 0 {
		t.Fatalf("expected positive version, got %d", v)
	}
}

func TestOpenDB_PingError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if _, err := openDB(ctx, "postgres://127.0.0.1:1/db"); err == nil {
		t.Fatal("expected ping error")
	}
}

func TestNewProvider_NilDB(t *testing.T) {
	if _, err := newProvider(nil); err == nil {
		t.Fatal("expected error for nil db")
	}
}

func TestApply_BadURL(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if _, _, err := Apply(ctx, "postgres://127.0.0.1:1/db"); err == nil {
		t.Fatal("expected error")
	}
}

func TestVersion_BadURL(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if _, err := Version(ctx, "postgres://127.0.0.1:1/db"); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidate_BadURL(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if err := Validate(ctx, "postgres://127.0.0.1:1/db"); err == nil {
		t.Fatal("expected error")
	}
}
