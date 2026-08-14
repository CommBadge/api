//go:build integration

package integration

import (
	"context"
	"testing"

	"kronus.dev/commbadge_api/store"
)

func TestNotificationStore_CreateAndGet(t *testing.T) {
	requireDB(t)
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()
	users := store.NewUserStore(pgPool)
	notifs := store.NewNotificationStore(pgPool)

	u := testUser(t, "notif1")
	if err := users.UpsertUser(ctx, u); err != nil {
		t.Fatal(err)
	}

	cond := map[string]string{"broadcaster_user_id": "twitch-1"}
	sub, err := notifs.Create(ctx, u.ID, "stream.online", cond, "eventsub-abc")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if sub.ID == "" || sub.Status != store.SubscriptionEnabled {
		t.Fatalf("unexpected subscription: %+v", sub)
	}

	got, err := notifs.GetByID(ctx, sub.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("expected subscription")
	}
	if got.UserID != u.ID || got.TwitchEventType != "stream.online" || got.TwitchSubscriptionID != "eventsub-abc" {
		t.Fatalf("mismatch: %+v", got)
	}
	if got.Condition["broadcaster_user_id"] != "twitch-1" {
		t.Fatalf("expected condition round-trip, got %+v", got.Condition)
	}
}

func TestNotificationStore_GetByIDNotFound(t *testing.T) {
	requireDB(t)
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()
	notifs := store.NewNotificationStore(pgPool)

	got, err := notifs.GetByID(ctx, "00000000-0000-0000-0000-000000000000")
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatal("expected nil")
	}
}

func TestNotificationStore_HasActiveAndList(t *testing.T) {
	requireDB(t)
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()
	users := store.NewUserStore(pgPool)
	notifs := store.NewNotificationStore(pgPool)

	u := testUser(t, "notif2")
	if err := users.UpsertUser(ctx, u); err != nil {
		t.Fatal(err)
	}

	active, err := notifs.HasActive(ctx, u.ID, "stream.online")
	if err != nil {
		t.Fatal(err)
	}
	if active {
		t.Fatal("expected no active subscription")
	}

	cond := map[string]string{"broadcaster_user_id": "twitch-2"}
	if _, err := notifs.Create(ctx, u.ID, "stream.online", cond, "eventsub-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := notifs.Create(ctx, u.ID, "stream.offline", cond, "eventsub-2"); err != nil {
		t.Fatal(err)
	}

	active, err = notifs.HasActive(ctx, u.ID, "stream.online")
	if err != nil {
		t.Fatal(err)
	}
	if !active {
		t.Fatal("expected active subscription")
	}

	list, err := notifs.ListActiveByUser(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 active subs, got %d", len(list))
	}

	if err := notifs.MarkRevoked(ctx, list[0].ID); err != nil {
		t.Fatalf("MarkRevoked: %v", err)
	}

	active, err = notifs.HasActive(ctx, u.ID, "stream.online")
	if err != nil {
		t.Fatal(err)
	}
	if active {
		t.Fatal("expected revoked sub to not be active")
	}

	list, err = notifs.ListActiveByUser(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 active sub after revoke, got %d", len(list))
	}
}

func TestNotificationStore_PartialUniqueIndex(t *testing.T) {
	requireDB(t)
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()
	users := store.NewUserStore(pgPool)
	notifs := store.NewNotificationStore(pgPool)

	u := testUser(t, "notif3")
	if err := users.UpsertUser(ctx, u); err != nil {
		t.Fatal(err)
	}

	cond := map[string]string{"broadcaster_user_id": "twitch-3"}
	if _, err := notifs.Create(ctx, u.ID, "stream.online", cond, "eventsub-1"); err != nil {
		t.Fatal(err)
	}

	// Second enabled sub for the same user+event must fail.
	if _, err := notifs.Create(ctx, u.ID, "stream.online", cond, "eventsub-2"); err == nil {
		t.Fatal("expected duplicate enabled subscription to fail")
	}

	// A different event type for the same user is fine.
	if _, err := notifs.Create(ctx, u.ID, "stream.offline", cond, "eventsub-3"); err != nil {
		t.Fatalf("expected different event type to succeed: %v", err)
	}

	// After revoke, a new enabled sub for the same event must succeed.
	list, err := notifs.ListActiveByUser(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range list {
		if s.TwitchEventType == "stream.online" {
			if err := notifs.MarkRevoked(ctx, s.ID); err != nil {
				t.Fatal(err)
			}
		}
	}

	if _, err := notifs.Create(ctx, u.ID, "stream.online", cond, "eventsub-4"); err != nil {
		t.Fatalf("expected resubscribe after revoke to succeed: %v", err)
	}

	active, err := notifs.HasActive(ctx, u.ID, "stream.online")
	if err != nil {
		t.Fatal(err)
	}
	if !active {
		t.Fatal("expected active after resubscribe")
	}
}

func TestNotificationStore_RevokeAllByUser(t *testing.T) {
	requireDB(t)
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()
	users := store.NewUserStore(pgPool)
	notifs := store.NewNotificationStore(pgPool)

	u := testUser(t, "notif4")
	other := testUser(t, "notif5")
	for _, x := range []*store.User{u, other} {
		if err := users.UpsertUser(ctx, x); err != nil {
			t.Fatal(err)
		}
	}

	cond := map[string]string{"broadcaster_user_id": "twitch-4"}
	if _, err := notifs.Create(ctx, u.ID, "stream.online", cond, "eventsub-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := notifs.Create(ctx, u.ID, "stream.offline", cond, "eventsub-2"); err != nil {
		t.Fatal(err)
	}
	if _, err := notifs.Create(ctx, other.ID, "stream.online", cond, "eventsub-3"); err != nil {
		t.Fatal(err)
	}

	revoked, err := notifs.RevokeAllByUser(ctx, u.ID)
	if err != nil {
		t.Fatalf("RevokeAllByUser: %v", err)
	}
	if len(revoked) != 2 {
		t.Fatalf("expected 2 revoked subs, got %d", len(revoked))
	}
	for _, s := range revoked {
		if s.Status != store.SubscriptionRevoked {
			t.Fatalf("expected revoked status, got %q", s.Status)
		}
	}

	list, err := notifs.ListActiveByUser(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("expected no active subs for user, got %d", len(list))
	}

	// Other user's subscriptions are untouched.
	list, err = notifs.ListActiveByUser(ctx, other.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 active sub for other user, got %d", len(list))
	}
}
