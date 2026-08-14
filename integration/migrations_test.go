//go:build integration

package integration

import (
	"context"
	"testing"

	"kronus.dev/commbadge_api/migrations"
)

func TestGooseDBVersionTable(t *testing.T) {
	if pgPool == nil {
		t.Skip("postgres not available")
	}
	var exists bool
	err := pgPool.QueryRow(context.Background(), `
		SELECT EXISTS(
			SELECT 1 FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = 'goose_db_version'
		)
	`).Scan(&exists)
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Fatal("expected goose_db_version table to exist")
	}
}

func TestSchemaVersionMatches(t *testing.T) {
	if pgPool == nil || pgDSN == "" {
		t.Skip("postgres not available")
	}
	ctx := context.Background()

	want, err := migrations.Latest()
	if err != nil {
		t.Fatal(err)
	}

	got, err := migrations.Version(ctx, pgDSN)
	if err != nil {
		t.Fatal(err)
	}

	if got != want {
		t.Fatalf("schema version = %d, want %d", got, want)
	}

	if err := migrations.Validate(ctx, pgDSN); err != nil {
		t.Fatalf("validate: %v", err)
	}
}
