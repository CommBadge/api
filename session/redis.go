package session

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type Session struct {
	UserID         string `json:"user_id"`
	Login          string `json:"login"`
	DisplayName    string `json:"display_name"`
	Email          string `json:"email"`
	AvatarURL      string `json:"avatar_url"`
	TwitchToken    string `json:"twitch_token"`
	TwitchRefresh  string `json:"twitch_refresh"`
}

type Store struct {
	client *redis.Client
	ttl    time.Duration
}

func NewStore(redisURL string, ttl time.Duration) (*Store, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("redis.ParseURL: %w", err)
	}
	client := redis.NewClient(opts)
	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("redis.Ping: %w", err)
	}
	return &Store{client: client, ttl: ttl}, nil
}

func (s *Store) Create(ctx context.Context, session *Session) (string, error) {
	sessionID := uuid.New().String()
	data, err := json.Marshal(session)
	if err != nil {
		return "", fmt.Errorf("json.Marshal: %w", err)
	}
	if err := s.client.Set(ctx, "session:"+sessionID, data, s.ttl).Err(); err != nil {
		return "", fmt.Errorf("redis.Set: %w", err)
	}
	return sessionID, nil
}

func (s *Store) Get(ctx context.Context, sessionID string) (*Session, error) {
	data, err := s.client.Get(ctx, "session:"+sessionID).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, fmt.Errorf("redis.Get: %w", err)
	}
	var ses Session
	if err := json.Unmarshal(data, &ses); err != nil {
		return nil, fmt.Errorf("json.Unmarshal: %w", err)
	}
	return &ses, nil
}

func (s *Store) Delete(ctx context.Context, sessionID string) error {
	return s.client.Del(ctx, "session:"+sessionID).Err()
}

func (s *Store) SetState(ctx context.Context, state string, ttl time.Duration) error {
	return s.client.Set(ctx, "state:"+state, "1", ttl).Err()
}

func (s *Store) VerifyState(ctx context.Context, state string) (bool, error) {
	err := s.client.Get(ctx, "state:"+state).Err()
	if err != nil {
		if err == redis.Nil {
			return false, nil
		}
		return false, err
	}
	s.client.Del(ctx, "state:"+state)
	return true, nil
}

func (s *Store) Ping(ctx context.Context) error {
	return s.client.Ping(ctx).Err()
}

func (s *Store) Close() error {
	return s.client.Close()
}
