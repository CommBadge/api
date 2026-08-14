//go:build integration

package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"kronus.dev/commbadge_api/session"
	"kronus.dev/commbadge_api/store"
)

const truncateTables = `notification_subscriptions, user_warnings, ticket_messages, support_tickets, community_members, communities, platform_admins, users`

func truncateAll(t *testing.T) {
	t.Helper()
	if pgPool == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := pgPool.Exec(ctx, "TRUNCATE "+truncateTables+" RESTART IDENTITY CASCADE"); err != nil {
		t.Fatalf("truncate: %v", err)
	}
}

func requireDB(t *testing.T) {
	t.Helper()
	if pgPool == nil {
		t.Skip("postgres not available")
	}
}

func requireRedis(t *testing.T) {
	t.Helper()
	if redisURL == "" {
		t.Skip("redis not available")
	}
}

var userSeq int

func testUser(t *testing.T, username string) *store.User {
	t.Helper()
	userSeq++
	if username == "" {
		username = fmt.Sprintf("user%d", userSeq)
	}
	return &store.User{
		ID:          fmt.Sprintf("id-%s-%d", username, userSeq),
		Username:    username,
		DisplayName: "Display " + username,
		Email:       username + "@example.com",
		AvatarURL:   "https://example.com/" + username + ".png",
	}
}

func newSessionStore(t *testing.T, ttl time.Duration) *session.Store {
	t.Helper()
	requireRedis(t)
	s, err := session.NewStore(redisURL, ttl)
	if err != nil {
		t.Fatalf("session.NewStore: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
