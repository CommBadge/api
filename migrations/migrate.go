// Package migrations embeds and applies the SQL schema migrations for the
// CommBadge API. Applied versions are recorded by goose in the
// goose_db_version table, so startup can cheaply validate that the schema is
// at the expected version.
package migrations

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"

	"github.com/pressly/goose/v3"

	// Register the "pgx" database/sql driver used by goose.
	_ "github.com/jackc/pgx/v5/stdlib"
)

//go:embed *.sql
var embedFS embed.FS

const driverName = "pgx"

func openDB(ctx context.Context, databaseURL string) (*sql.DB, error) {
	db, err := sql.Open(driverName, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return db, nil
}

func newProvider(db *sql.DB) (*goose.Provider, error) {
	p, err := goose.NewProvider(goose.DialectPostgres, db, embedFS, goose.WithSlog(slog.Default()))
	if err != nil {
		return nil, fmt.Errorf("create migration provider: %w", err)
	}
	return p, nil
}

// Latest returns the highest migration version embedded in the binary.
func Latest() (int64, error) {
	var latest int64
	names, err := fs.Glob(embedFS, "*.sql")
	if err != nil {
		return 0, fmt.Errorf("list embedded migrations: %w", err)
	}
	for _, name := range names {
		v, err := goose.NumericComponent(name)
		if err != nil {
			return 0, fmt.Errorf("parse migration file %q: %w", name, err)
		}
		if v > latest {
			latest = v
		}
	}
	return latest, nil
}

// Apply runs any pending migrations in order, recording each applied version
// in the goose_db_version table, and returns the resulting schema version
// along with the number of migrations that were applied.
func Apply(ctx context.Context, databaseURL string) (version int64, applied int, err error) {
	db, err := openDB(ctx, databaseURL)
	if err != nil {
		return 0, 0, err
	}
	defer db.Close()

	p, err := newProvider(db)
	if err != nil {
		return 0, 0, err
	}
	defer p.Close()

	results, err := p.Up(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("apply migrations: %w", err)
	}
	version, err = p.GetDBVersion(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("get database version: %w", err)
	}
	return version, len(results), nil
}

// Version reports the schema version currently recorded in the database.
func Version(ctx context.Context, databaseURL string) (int64, error) {
	db, err := openDB(ctx, databaseURL)
	if err != nil {
		return 0, err
	}
	defer db.Close()

	p, err := newProvider(db)
	if err != nil {
		return 0, err
	}
	defer p.Close()

	v, err := p.GetDBVersion(ctx)
	if err != nil {
		return 0, fmt.Errorf("get database version: %w", err)
	}
	return v, nil
}

// Validate ensures the database schema matches the migrations embedded in
// this binary. It returns an error if migrations are pending or if the
// database is ahead of the binary (e.g. a downgraded deployment).
func Validate(ctx context.Context, databaseURL string) error {
	db, err := openDB(ctx, databaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	p, err := newProvider(db)
	if err != nil {
		return err
	}
	defer p.Close()

	target, err := Latest()
	if err != nil {
		return err
	}

	pending, err := p.HasPending(ctx)
	if err != nil {
		return fmt.Errorf("check pending migrations: %w", err)
	}
	if pending {
		return fmt.Errorf("schema out of date: migrations pending, database must be at version %d", target)
	}

	current, err := p.GetDBVersion(ctx)
	if err != nil {
		return fmt.Errorf("get database version: %w", err)
	}
	if current != target {
		return fmt.Errorf("schema version mismatch: database at %d, binary requires %d", current, target)
	}
	return nil
}
