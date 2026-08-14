package migrations

import (
	"database/sql"
	"testing"
)

func TestLatest(t *testing.T) {
	got, err := Latest()
	if err != nil {
		t.Fatal(err)
	}
	if got != 3 {
		t.Fatalf("Latest() = %d, want 3", got)
	}
}

func TestLatest_EmbeddedFiles(t *testing.T) {
	names, err := embedFS.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 3 {
		t.Fatalf("expected exactly 3 embedded migrations, got %d", len(names))
	}
	if names[0].Name() != "001_init.sql" {
		t.Fatalf("unexpected first embedded file: %s", names[0].Name())
	}
	if names[1].Name() != "002_notification_targets.sql" {
		t.Fatalf("unexpected second embedded file: %s", names[1].Name())
	}
	if names[2].Name() != "003_shoutout_template.sql" {
		t.Fatalf("unexpected third embedded file: %s", names[2].Name())
	}
}

func TestProviderCollectsEmbeddedMigrations(t *testing.T) {
	db, err := sql.Open(driverName, "postgres://unused/unused")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	p, err := newProvider(db)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	sources := p.ListSources()
	if len(sources) != 3 {
		t.Fatalf("expected 3 embedded migration sources, got %d", len(sources))
	}
	if sources[0].Version != 1 {
		t.Fatalf("expected source version 1, got %d", sources[0].Version)
	}
	if sources[1].Version != 2 {
		t.Fatalf("expected source version 2, got %d", sources[1].Version)
	}
	if sources[2].Version != 3 {
		t.Fatalf("expected source version 3, got %d", sources[2].Version)
	}
}
