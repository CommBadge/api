package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type User struct {
	ID             string     `json:"id"`
	Username       string     `json:"username"`
	DisplayName    string     `json:"display_name"`
	Email          string     `json:"email"`
	AvatarURL      string     `json:"avatar_url"`
	TwitchID       string     `json:"twitch_id,omitempty"`
	TwitchLinkedAt *time.Time `json:"twitch_linked_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`

	NotificationDiscordChannelID string `json:"notification_discord_channel_id,omitempty"`
	ShoutoutTemplate             string `json:"shoutout_template"`
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

const userColumns = `id, username, display_name, email, avatar_url, twitch_id, twitch_linked_at, created_at, updated_at, shoutout_template`

func (s *UserStore) UpsertUser(ctx context.Context, user *User) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO users (id, username, display_name, email, avatar_url)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (id) DO UPDATE SET
			username = EXCLUDED.username,
			display_name = EXCLUDED.display_name,
			email = EXCLUDED.email,
			avatar_url = EXCLUDED.avatar_url,
			updated_at = now()
	`, user.ID, user.Username, user.DisplayName, user.Email, user.AvatarURL)
	return err
}

func (s *UserStore) GetUserByID(ctx context.Context, id string) (*User, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT `+userColumns+`
		FROM users WHERE id = $1
	`, id)
	u, err := scanUser(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return u, nil
}

func (s *UserStore) SaveTwitchLink(ctx context.Context, userID, twitchID string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE users SET twitch_id = $1, twitch_linked_at = now(), updated_at = now() WHERE id = $2
	`, twitchID, userID)
	return err
}

func (s *UserStore) GetTwitchID(ctx context.Context, userID string) (string, error) {
	var twitchID string
	err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(twitch_id, '') FROM users WHERE id = $1
	`, userID).Scan(&twitchID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", nil
		}
		return "", err
	}
	return twitchID, nil
}

func (s *UserStore) ClearTwitchLink(ctx context.Context, userID string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE users SET twitch_id = NULL, twitch_linked_at = NULL, updated_at = now() WHERE id = $1
	`, userID)
	return err
}

func (s *UserStore) SetNotificationChannel(ctx context.Context, userID, channelID string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE users SET notification_discord_channel_id = $1, updated_at = now() WHERE id = $2
	`, channelID, userID)
	return err
}

func (s *UserStore) SetShoutoutTemplate(ctx context.Context, userID, template string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE users SET shoutout_template = $1, updated_at = now() WHERE id = $2
	`, template, userID)
	return err
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
		SELECT `+userColumns+`,
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
		u, err := scanUserAdminView(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, *u)
	}
	return users, nil
}

func (s *UserStore) SearchUsers(ctx context.Context, query string, limit int) ([]UserAdminView, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+userColumns+`,
		       warnings, banned, banned_at, ban_reason
		FROM users
		WHERE username ILIKE $1 OR display_name ILIKE $1
		ORDER BY display_name
		LIMIT $2
	`, "%"+query+"%", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []UserAdminView
	for rows.Next() {
		u, err := scanUserAdminView(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, *u)
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

type rowScanner interface {
	Scan(dest ...interface{}) error
}

func scanUser(row rowScanner) (*User, error) {
	var twitchID *string
	u := &User{}
	err := row.Scan(&u.ID, &u.Username, &u.DisplayName, &u.Email, &u.AvatarURL,
		&twitchID, &u.TwitchLinkedAt, &u.CreatedAt, &u.UpdatedAt, &u.ShoutoutTemplate)
	if err != nil {
		return nil, err
	}
	if twitchID != nil {
		u.TwitchID = *twitchID
	}
	return u, nil
}

func scanUserAdminView(row rowScanner) (*UserAdminView, error) {
	var twitchID *string
	u := &UserAdminView{}
	err := row.Scan(&u.ID, &u.Username, &u.DisplayName, &u.Email, &u.AvatarURL,
		&twitchID, &u.TwitchLinkedAt, &u.CreatedAt, &u.UpdatedAt, &u.ShoutoutTemplate,
		&u.Warnings, &u.Banned, &u.BannedAt, &u.BanReason)
	if err != nil {
		return nil, err
	}
	if twitchID != nil {
		u.TwitchID = *twitchID
	}
	return u, nil
}
