//go:build integration

package integration

import (
	"context"
	"log"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	ctx := context.Background()
	if err := setup(ctx); err != nil {
		log.Printf("SKIP: integration setup failed (docker required?): %v", err)
		os.Exit(0)
	}
	code := m.Run()
	teardown()
	os.Exit(code)
}

func TestPostgresConnection(t *testing.T) {
	if pgPool == nil {
		t.Skip("postgres not available")
	}
	if err := pgPool.Ping(context.Background()); err != nil {
		t.Fatalf("ping failed: %v", err)
	}
}

func TestMigrationsApplied(t *testing.T) {
	if pgPool == nil {
		t.Skip("postgres not available")
	}
	var tableCount int
	err := pgPool.QueryRow(context.Background(), `
		SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public'
	`).Scan(&tableCount)
	if err != nil {
		t.Fatal(err)
	}
	if tableCount < 5 {
		t.Fatalf("expected at least 5 tables, got %d", tableCount)
	}
}

func TestUsersTableColumns(t *testing.T) {
	if pgPool == nil {
		t.Skip("postgres not available")
	}
	var hasWarnings bool
	err := pgPool.QueryRow(context.Background(), `
		SELECT EXISTS(
			SELECT 1 FROM information_schema.columns
			WHERE table_name = 'users' AND column_name = 'warnings'
		)
	`).Scan(&hasWarnings)
	if err != nil {
		t.Fatal(err)
	}
	if !hasWarnings {
		t.Fatal("expected 'warnings' column on users table")
	}

	var hasBanned bool
	err = pgPool.QueryRow(context.Background(), `
		SELECT EXISTS(
			SELECT 1 FROM information_schema.columns
			WHERE table_name = 'users' AND column_name = 'banned'
		)
	`).Scan(&hasBanned)
	if err != nil {
		t.Fatal(err)
	}
	if !hasBanned {
		t.Fatal("expected 'banned' column on users table")
	}
}
