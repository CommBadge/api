package store

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type AdminStore struct {
	pool *pgxpool.Pool
}

func NewAdminStore(pool *pgxpool.Pool) *AdminStore {
	return &AdminStore{pool: pool}
}

func (s *AdminStore) IsAdmin(ctx context.Context, userID string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM platform_admins WHERE user_id = $1)
	`, userID).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (s *AdminStore) SeedAdmins(ctx context.Context, adminIDs []string) error {
	for _, id := range adminIDs {
		tx, err := s.pool.Begin(ctx)
		if err != nil {
			return err
		}
		// platform_admins.user_id references users(id); ensure the row exists
		// so a fresh database can be bootstrapped before any login. A real
		// login later upserts the full profile.
		if _, err := tx.Exec(ctx, `
			INSERT INTO users (id, username, display_name) VALUES ($1, $1, $1)
			ON CONFLICT (id) DO NOTHING
		`, id); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO platform_admins (user_id) VALUES ($1)
			ON CONFLICT (user_id) DO NOTHING
		`, id); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
	}
	return nil
}
