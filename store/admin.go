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
		_, err := s.pool.Exec(ctx, `
			INSERT INTO platform_admins (user_id) VALUES ($1)
			ON CONFLICT (user_id) DO NOTHING
		`, id)
		if err != nil {
			return err
		}
	}
	return nil
}
