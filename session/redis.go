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
	UserID      string `json:"user_id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	AvatarURL   string `json:"avatar_url"`

	// DiscordAccessToken is the OAuth access token obtained at login. It is
	// used to verify Discord channel permissions against the Discord API when
	// notification targets are configured. It is never exposed to clients.
	DiscordAccessToken string `json:"discord_access_token,omitempty"`
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

// SetState stores an OAuth CSRF state token under the given binding. The
// binding identifies the party the state was issued for (e.g. a user id); the
// callback must return it so the server can reject states used out of
// context.
func (s *Store) SetState(ctx context.Context, state, binding string, ttl time.Duration) error {
	return s.client.Set(ctx, "state:"+state, binding, ttl).Err()
}

// VerifyState atomically consumes a state token and returns the binding it
// was stored with. The GETDEL is atomic, so a state can only be redeemed
// once even under concurrent requests.
func (s *Store) VerifyState(ctx context.Context, state string) (string, bool, error) {
	binding, err := s.client.GetDel(ctx, "state:"+state).Result()
	if err != nil {
		if err == redis.Nil {
			return "", false, nil
		}
		return "", false, err
	}
	return binding, true, nil
}

func (s *Store) Ping(ctx context.Context) error {
	return s.client.Ping(ctx).Err()
}

func (s *Store) Close() error {
	return s.client.Close()
}
