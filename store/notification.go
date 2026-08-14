package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	SubscriptionEnabled = "enabled"
	SubscriptionRevoked = "revoked"
)

type NotificationSubscription struct {
	ID                   string            `json:"id"`
	UserID               string            `json:"user_id"`
	TwitchEventType      string            `json:"twitch_event_type"`
	Condition            map[string]string `json:"condition"`
	TwitchSubscriptionID string            `json:"twitch_subscription_id"`
	Status               string            `json:"status"`
	CreatedAt            time.Time         `json:"created_at"`
	UpdatedAt            time.Time         `json:"updated_at"`
}

type NotificationStore struct {
	pool *pgxpool.Pool
}

func NewNotificationStore(pool *pgxpool.Pool) *NotificationStore {
	return &NotificationStore{pool: pool}
}

const notificationColumns = `id, user_id, twitch_event_type, condition, twitch_subscription_id, status, created_at, updated_at`

func (s *NotificationStore) Create(ctx context.Context, userID, eventType string, condition map[string]string, twitchSubscriptionID string) (*NotificationSubscription, error) {
	condJSON, err := json.Marshal(condition)
	if err != nil {
		return nil, err
	}
	row := s.pool.QueryRow(ctx, `
		INSERT INTO notification_subscriptions (user_id, twitch_event_type, condition, twitch_subscription_id)
		VALUES ($1, $2, $3, $4)
		RETURNING `+notificationColumns,
		userID, eventType, condJSON, twitchSubscriptionID)
	return scanNotificationSubscription(row)
}

func (s *NotificationStore) GetByID(ctx context.Context, id string) (*NotificationSubscription, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT `+notificationColumns+`
		FROM notification_subscriptions WHERE id = $1
	`, id)
	sub, err := scanNotificationSubscription(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return sub, nil
}

func (s *NotificationStore) HasActive(ctx context.Context, userID, eventType string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM notification_subscriptions
			WHERE user_id = $1 AND twitch_event_type = $2 AND status = 'enabled'
		)
	`, userID, eventType).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (s *NotificationStore) ListActiveByUser(ctx context.Context, userID string) ([]NotificationSubscription, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+notificationColumns+`
		FROM notification_subscriptions
		WHERE user_id = $1 AND status = 'enabled'
		ORDER BY created_at
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []NotificationSubscription
	for rows.Next() {
		sub, err := scanNotificationSubscription(rows)
		if err != nil {
			return nil, err
		}
		subs = append(subs, *sub)
	}
	return subs, nil
}

func (s *NotificationStore) MarkRevoked(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE notification_subscriptions SET status = 'revoked', updated_at = now() WHERE id = $1
	`, id)
	return err
}

func (s *NotificationStore) RevokeAllByUser(ctx context.Context, userID string) ([]NotificationSubscription, error) {
	rows, err := s.pool.Query(ctx, `
		UPDATE notification_subscriptions
		SET status = 'revoked', updated_at = now()
		WHERE user_id = $1 AND status = 'enabled'
		RETURNING `+notificationColumns,
		userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []NotificationSubscription
	for rows.Next() {
		sub, err := scanNotificationSubscription(rows)
		if err != nil {
			return nil, err
		}
		subs = append(subs, *sub)
	}
	return subs, nil
}

type subScanner interface {
	Scan(dest ...interface{}) error
}

func scanNotificationSubscription(row subScanner) (*NotificationSubscription, error) {
	var condJSON []byte
	sub := &NotificationSubscription{}
	err := row.Scan(&sub.ID, &sub.UserID, &sub.TwitchEventType, &condJSON,
		&sub.TwitchSubscriptionID, &sub.Status, &sub.CreatedAt, &sub.UpdatedAt)
	if err != nil {
		return nil, err
	}
	sub.Condition = make(map[string]string)
	if err := json.Unmarshal(condJSON, &sub.Condition); err != nil {
		return nil, err
	}
	return sub, nil
}
