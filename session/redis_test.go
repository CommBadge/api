package session

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func closedRedis(t *testing.T) *redis.Client {
	t.Helper()
	mr := miniredis.RunT(t)
	addr := mr.Addr()
	mr.Close()
	client := redis.NewClient(&redis.Options{Addr: addr, DialTimeout: time.Second, MaxRetries: -1})
	t.Cleanup(func() { client.Close() })
	return client
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	mr := miniredis.RunT(t)
	store, err := NewStore("redis://"+mr.Addr(), time.Hour)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestNewStore(t *testing.T) {
	t.Run("invalid url", func(t *testing.T) {
		if _, err := NewStore("://bad", time.Hour); err == nil {
			t.Fatal("expected error for invalid url")
		}
	})
	t.Run("ping failure", func(t *testing.T) {
		mr := miniredis.RunT(t)
		addr := mr.Addr()
		mr.Close()
		if _, err := NewStore("redis://"+addr, time.Hour); err == nil {
			t.Fatal("expected ping error")
		}
	})
}

func TestCreateGet(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	ses := &Session{UserID: "u1", Username: "kronus", DisplayName: "Kronus", Email: "k@example.com", AvatarURL: "http://a", DiscordAccessToken: "secret-token"}
	id, err := store.Create(ctx, ses)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if id == "" {
		t.Fatal("empty session id")
	}
	got, err := store.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil || got.UserID != "u1" || got.DisplayName != "Kronus" || got.DiscordAccessToken != "secret-token" {
		t.Fatalf("Get = %+v", got)
	}

	t.Run("missing", func(t *testing.T) {
		got, err := store.Get(ctx, "ghost")
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if got != nil {
			t.Fatalf("expected nil, got %+v", got)
		}
	})

	t.Run("delete", func(t *testing.T) {
		if err := store.Delete(ctx, id); err != nil {
			t.Fatalf("Delete: %v", err)
		}
		if got, _ := store.Get(ctx, id); got != nil {
			t.Fatalf("expected nil after delete, got %+v", got)
		}
		if err := store.Delete(ctx, id); err != nil {
			t.Fatalf("Delete missing: %v", err)
		}
	})
}

func TestGet_BadJSON(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	store.client.Set(ctx, "session:bad", "{oops", time.Hour)
	if _, err := store.Get(ctx, "bad"); err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestGet_RedisError(t *testing.T) {
	store := &Store{client: closedRedis(t), ttl: time.Hour}
	if _, err := store.Get(context.Background(), "x"); err == nil {
		t.Fatal("expected redis error")
	}
}

func TestState(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	if err := store.SetState(ctx, "st-1", "user-1", time.Minute); err != nil {
		t.Fatalf("SetState: %v", err)
	}
	binding, ok, err := store.VerifyState(ctx, "st-1")
	if err != nil || !ok || binding != "user-1" {
		t.Fatalf("VerifyState = %q, %v, %v", binding, ok, err)
	}

	t.Run("single use", func(t *testing.T) {
		_, ok, err := store.VerifyState(ctx, "st-1")
		if err != nil || ok {
			t.Fatalf("expected second verify to fail, got ok=%v err=%v", ok, err)
		}
	})

	t.Run("missing", func(t *testing.T) {
		_, ok, err := store.VerifyState(ctx, "st-missing")
		if err != nil || ok {
			t.Fatalf("expected ok=false, got ok=%v err=%v", ok, err)
		}
	})

	t.Run("redis error", func(t *testing.T) {
		bad := &Store{client: closedRedis(t), ttl: time.Hour}
		if _, _, err := bad.VerifyState(ctx, "x"); err == nil {
			t.Fatal("expected redis error")
		}
	})
}

func TestPingClose(t *testing.T) {
	store := newTestStore(t)
	if err := store.Ping(context.Background()); err != nil {
		t.Fatalf("Ping: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	bad := &Store{client: closedRedis(t), ttl: time.Hour}
	if err := bad.Ping(context.Background()); err == nil {
		t.Fatal("expected ping error")
	}
}
