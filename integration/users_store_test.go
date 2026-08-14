//go:build integration

package integration

import (
	"context"
	"testing"

	"kronus.dev/commbadge_api/store"
)

func TestUserStore_UpsertInsertAndGet(t *testing.T) {
	requireDB(t)
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()
	users := store.NewUserStore(pgPool)

	u := testUser(t, "alice")
	if err := users.UpsertUser(ctx, u); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}

	got, err := users.GetUserByID(ctx, u.ID)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if got == nil {
		t.Fatal("expected user, got nil")
	}
	if got.Username != u.Username || got.DisplayName != u.DisplayName || got.Email != u.Email || got.AvatarURL != u.AvatarURL {
		t.Fatalf("mismatch: %+v", got)
	}
	if got.TwitchID != "" {
		t.Fatalf("expected no twitch id, got %q", got.TwitchID)
	}
}

func TestUserStore_UpsertUpdatesOnConflict(t *testing.T) {
	requireDB(t)
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()
	users := store.NewUserStore(pgPool)

	u := testUser(t, "bob")
	if err := users.UpsertUser(ctx, u); err != nil {
		t.Fatal(err)
	}
	if err := users.SaveTwitchLink(ctx, u.ID, "twitch-bob"); err != nil {
		t.Fatal(err)
	}

	u.Username = "bob2"
	u.DisplayName = "Bob Two"
	u.Email = "bob2@example.com"
	if err := users.UpsertUser(ctx, u); err != nil {
		t.Fatalf("upsert update: %v", err)
	}

	got, err := users.GetUserByID(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Username != "bob2" || got.DisplayName != "Bob Two" {
		t.Fatalf("expected updated fields, got %+v", got)
	}
	if got.TwitchID != "twitch-bob" {
		t.Fatalf("expected twitch link preserved, got %q", got.TwitchID)
	}
}

func TestUserStore_TwitchLinkLifecycle(t *testing.T) {
	requireDB(t)
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()
	users := store.NewUserStore(pgPool)

	u := testUser(t, "carol")
	if err := users.UpsertUser(ctx, u); err != nil {
		t.Fatal(err)
	}

	twitchID, err := users.GetTwitchID(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if twitchID != "" {
		t.Fatalf("expected empty twitch id, got %q", twitchID)
	}

	if err := users.SaveTwitchLink(ctx, u.ID, "twitch-carol"); err != nil {
		t.Fatalf("SaveTwitchLink: %v", err)
	}

	twitchID, err = users.GetTwitchID(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if twitchID != "twitch-carol" {
		t.Fatalf("expected twitch-carol, got %q", twitchID)
	}

	got, err := users.GetUserByID(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.TwitchID != "twitch-carol" || got.TwitchLinkedAt == nil {
		t.Fatalf("expected twitch id + linked_at set, got %+v", got)
	}

	if err := users.ClearTwitchLink(ctx, u.ID); err != nil {
		t.Fatalf("ClearTwitchLink: %v", err)
	}

	twitchID, err = users.GetTwitchID(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if twitchID != "" {
		t.Fatalf("expected cleared twitch id, got %q", twitchID)
	}
}

func TestUserStore_GetTwitchIDMissingUser(t *testing.T) {
	requireDB(t)
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()
	users := store.NewUserStore(pgPool)

	id, err := users.GetTwitchID(ctx, "does-not-exist")
	if err != nil {
		t.Fatal(err)
	}
	if id != "" {
		t.Fatalf("expected empty, got %q", id)
	}
}

func TestUserStore_BanUnban(t *testing.T) {
	requireDB(t)
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()
	users := store.NewUserStore(pgPool)

	u := testUser(t, "dave")
	if err := users.UpsertUser(ctx, u); err != nil {
		t.Fatal(err)
	}

	banned, err := users.IsBanned(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if banned {
		t.Fatal("expected not banned by default")
	}

	if err := users.BanUser(ctx, u.ID, "spam"); err != nil {
		t.Fatalf("BanUser: %v", err)
	}
	banned, err = users.IsBanned(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !banned {
		t.Fatal("expected banned")
	}

	adminView, err := users.ListUsers(ctx, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	var found *store.UserAdminView
	for i := range adminView {
		if adminView[i].ID == u.ID {
			found = &adminView[i]
		}
	}
	if found == nil {
		t.Fatal("expected user in admin list")
	}
	if !found.Banned || found.BanReason != "spam" || found.BannedAt == nil {
		t.Fatalf("expected ban details, got %+v", found)
	}

	if err := users.UnbanUser(ctx, u.ID); err != nil {
		t.Fatalf("UnbanUser: %v", err)
	}
	banned, err = users.IsBanned(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if banned {
		t.Fatal("expected unbanned")
	}
}

func TestUserStore_IsBannedMissingUser(t *testing.T) {
	requireDB(t)
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()
	users := store.NewUserStore(pgPool)

	banned, err := users.IsBanned(ctx, "does-not-exist")
	if err != nil {
		t.Fatal(err)
	}
	if banned {
		t.Fatal("expected not banned")
	}
}

func TestUserStore_AddWarning(t *testing.T) {
	requireDB(t)
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()
	users := store.NewUserStore(pgPool)

	u := testUser(t, "erin")
	admin := testUser(t, "admin")
	if err := users.UpsertUser(ctx, u); err != nil {
		t.Fatal(err)
	}
	if err := users.UpsertUser(ctx, admin); err != nil {
		t.Fatal(err)
	}

	if err := users.AddWarning(ctx, u.ID, admin.ID, "be nice"); err != nil {
		t.Fatalf("AddWarning: %v", err)
	}

	log, err := users.GetWarningLog(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(log) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(log))
	}
	if log[0].Reason != "be nice" || log[0].AdminID != admin.ID {
		t.Fatalf("unexpected warning: %+v", log[0])
	}

	if err := users.AddWarning(ctx, u.ID, admin.ID, "again"); err != nil {
		t.Fatal(err)
	}
	list, err := users.ListUsers(ctx, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	for i := range list {
		if list[i].ID == u.ID {
			if list[i].Warnings != 2 {
				t.Fatalf("expected 2 warnings, got %d", list[i].Warnings)
			}
		}
	}
}

func TestUserStore_SearchUsers(t *testing.T) {
	requireDB(t)
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()
	users := store.NewUserStore(pgPool)

	for _, name := range []string{"AlphaBeta", "GammaDelta", "alphaother"} {
		u := testUser(t, name)
		if err := users.UpsertUser(ctx, u); err != nil {
			t.Fatal(err)
		}
	}

	results, err := users.SearchUsers(ctx, "alpha", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(results))
	}

	results, err = users.SearchUsers(ctx, "gamma", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 match, got %d", len(results))
	}
}
