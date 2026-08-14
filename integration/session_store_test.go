//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"kronus.dev/commbadge_api/session"
)

func TestSessionStore_RoundTrip(t *testing.T) {
	requireRedis(t)
	ctx := context.Background()
	s := newSessionStore(t, time.Hour)

	id, err := s.Create(ctx, &session.Session{
		UserID:      "user-1",
		Username:    "alice",
		DisplayName: "Alice",
		Email:       "alice@example.com",
		AvatarURL:   "https://example.com/a.png",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty session id")
	}

	got, err := s.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil {
		t.Fatal("expected session")
	}
	if got.UserID != "user-1" || got.Username != "alice" || got.DisplayName != "Alice" ||
		got.Email != "alice@example.com" || got.AvatarURL != "https://example.com/a.png" {
		t.Fatalf("round-trip mismatch: %+v", got)
	}

	if err := s.Delete(ctx, id); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	got, err = s.Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatal("expected nil after delete")
	}
}

func TestSessionStore_GetMissing(t *testing.T) {
	requireRedis(t)
	ctx := context.Background()
	s := newSessionStore(t, time.Hour)

	got, err := s.Get(ctx, "nonexistent-session-id")
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatal("expected nil")
	}
}

func TestSessionStore_StateLifecycle(t *testing.T) {
	requireRedis(t)
	ctx := context.Background()
	s := newSessionStore(t, time.Hour)

	if err := s.SetState(ctx, "state-1", 5*time.Minute); err != nil {
		t.Fatalf("SetState: %v", err)
	}

	valid, err := s.VerifyState(ctx, "state-1")
	if err != nil {
		t.Fatal(err)
	}
	if !valid {
		t.Fatal("expected state to verify")
	}

	// State is single-use: second verification must fail.
	valid, err = s.VerifyState(ctx, "state-1")
	if err != nil {
		t.Fatal(err)
	}
	if valid {
		t.Fatal("expected state to be consumed")
	}
}

func TestSessionStore_StateMissing(t *testing.T) {
	requireRedis(t)
	ctx := context.Background()
	s := newSessionStore(t, time.Hour)

	valid, err := s.VerifyState(ctx, "never-set")
	if err != nil {
		t.Fatal(err)
	}
	if valid {
		t.Fatal("expected missing state to fail")
	}
}

func TestSessionStore_StateExpiry(t *testing.T) {
	requireRedis(t)
	ctx := context.Background()
	s := newSessionStore(t, time.Hour)

	if err := s.SetState(ctx, "short-state", 50*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	time.Sleep(150 * time.Millisecond)

	valid, err := s.VerifyState(ctx, "short-state")
	if err != nil {
		t.Fatal(err)
	}
	if valid {
		t.Fatal("expected expired state to fail")
	}
}

func TestSessionStore_SessionExpiry(t *testing.T) {
	requireRedis(t)
	ctx := context.Background()
	s := newSessionStore(t, 50*time.Millisecond)

	id, err := s.Create(ctx, &session.Session{UserID: "user-1", Username: "alice"})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(150 * time.Millisecond)

	got, err := s.Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatal("expected expired session to be gone")
	}
}
