package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type User struct {
	ID          string    `json:"id"`
	Login       string    `json:"login"`
	DisplayName string    `json:"display_name"`
	Email       string    `json:"email"`
	AvatarURL   string    `json:"avatar_url"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type UserAdminView struct {
	User
	Warnings  int        `json:"warnings"`
	Banned    bool       `json:"banned"`
	BannedAt  *time.Time `json:"banned_at,omitempty"`
	BanReason string     `json:"ban_reason,omitempty"`
}

type UserWarning struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	AdminID   string    `json:"admin_id"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"created_at"`
}

type UserStore struct {
	pool *pgxpool.Pool
}

func NewUserStore(pool *pgxpool.Pool) *UserStore {
	return &UserStore{pool: pool}
}

func (s *UserStore) UpsertUser(ctx context.Context, user *User) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO users (id, login, display_name, email, avatar_url)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (id) DO UPDATE SET
			login = EXCLUDED.login,
			display_name = EXCLUDED.display_name,
			email = EXCLUDED.email,
			avatar_url = EXCLUDED.avatar_url,
			updated_at = now()
	`, user.ID, user.Login, user.DisplayName, user.Email, user.AvatarURL)
	return err
}

func (s *UserStore) GetUserByID(ctx context.Context, id string) (*User, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, login, display_name, email, avatar_url, created_at, updated_at
		FROM users WHERE id = $1
	`, id)
	u := &User{}
	err := row.Scan(&u.ID, &u.Login, &u.DisplayName, &u.Email, &u.AvatarURL, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (s *UserStore) IsBanned(ctx context.Context, userID string) (bool, error) {
	var banned bool
	err := s.pool.QueryRow(ctx, `SELECT banned FROM users WHERE id = $1`, userID).Scan(&banned)
	if err != nil {
		if err == pgx.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return banned, nil
}

func (s *UserStore) ListUsers(ctx context.Context, limit, offset int) ([]UserAdminView, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, login, display_name, email, avatar_url, created_at, updated_at,
		       warnings, banned, banned_at, ban_reason
		FROM users
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []UserAdminView
	for rows.Next() {
		var u UserAdminView
		if err := rows.Scan(&u.ID, &u.Login, &u.DisplayName, &u.Email, &u.AvatarURL, &u.CreatedAt, &u.UpdatedAt,
			&u.Warnings, &u.Banned, &u.BannedAt, &u.BanReason); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

func (s *UserStore) SearchUsers(ctx context.Context, query string, limit int) ([]UserAdminView, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, login, display_name, email, avatar_url, created_at, updated_at,
		       warnings, banned, banned_at, ban_reason
		FROM users
		WHERE login ILIKE $1 OR display_name ILIKE $1
		ORDER BY display_name
		LIMIT $2
	`, "%"+query+"%", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []UserAdminView
	for rows.Next() {
		var u UserAdminView
		if err := rows.Scan(&u.ID, &u.Login, &u.DisplayName, &u.Email, &u.AvatarURL, &u.CreatedAt, &u.UpdatedAt,
			&u.Warnings, &u.Banned, &u.BannedAt, &u.BanReason); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}

func (s *UserStore) GetWarningLog(ctx context.Context, userID string) ([]UserWarning, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, admin_id, reason, created_at
		FROM user_warnings
		WHERE user_id = $1
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var warnings []UserWarning
	for rows.Next() {
		var w UserWarning
		if err := rows.Scan(&w.ID, &w.UserID, &w.AdminID, &w.Reason, &w.CreatedAt); err != nil {
			return nil, err
		}
		warnings = append(warnings, w)
	}
	return warnings, nil
}

func (s *UserStore) AddWarning(ctx context.Context, userID, adminID, reason string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		INSERT INTO user_warnings (user_id, admin_id, reason) VALUES ($1, $2, $3)
	`, userID, adminID, reason); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
		UPDATE users SET warnings = warnings + 1, updated_at = now() WHERE id = $1
	`, userID); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (s *UserStore) BanUser(ctx context.Context, userID, reason string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE users SET banned = true, ban_reason = $1, banned_at = now(), updated_at = now() WHERE id = $2
	`, reason, userID)
	return err
}

func (s *UserStore) UnbanUser(ctx context.Context, userID string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE users SET banned = false, ban_reason = '', banned_at = NULL, updated_at = now() WHERE id = $1
	`, userID)
	return err
}
