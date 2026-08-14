//go:build integration

package integration

import (
	"context"
	"strings"
	"testing"

	"kronus.dev/commbadge_api/store"
)

func TestCommunityStore_CreateAndGet(t *testing.T) {
	requireDB(t)
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()
	users := store.NewUserStore(pgPool)
	communities := store.NewCommunityStore(pgPool)

	owner := testUser(t, "owner1")
	if err := users.UpsertUser(ctx, owner); err != nil {
		t.Fatal(err)
	}

	c, err := communities.Create(ctx, "Gaming Den", "A place to game", owner.ID)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if c.ID == "" || c.JoinLinkID == "" {
		t.Fatalf("expected generated id and join link, got %+v", c)
	}
	if c.OwnerID != owner.ID {
		t.Fatalf("expected owner %s, got %s", owner.ID, c.OwnerID)
	}

	got, err := communities.GetByID(ctx, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("expected community")
	}
	if got.Name != "Gaming Den" || got.Description != "A place to game" {
		t.Fatalf("mismatch: %+v", got)
	}

	// The handler adds the owner as a member separately; the store Create alone
	// does not insert into community_members.
	list, err := communities.ListByUserID(ctx, owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("expected 0 communities before membership, got %d", len(list))
	}

	if err := communities.AddMember(ctx, c.ID, owner.ID, "owner"); err != nil {
		t.Fatalf("AddMember owner: %v", err)
	}
	list, err = communities.ListByUserID(ctx, owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 community for owner, got %d", len(list))
	}
	if list[0].ID != c.ID || list[0].Role != "owner" {
		t.Fatalf("unexpected list entry: %+v", list[0])
	}
}

func TestCommunityStore_CreateDuplicateName(t *testing.T) {
	requireDB(t)
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()
	users := store.NewUserStore(pgPool)
	communities := store.NewCommunityStore(pgPool)

	owner := testUser(t, "owner2")
	if err := users.UpsertUser(ctx, owner); err != nil {
		t.Fatal(err)
	}

	if _, err := communities.Create(ctx, "Unique Name", "", owner.ID); err != nil {
		t.Fatalf("first create: %v", err)
	}
	if _, err := communities.Create(ctx, "Unique Name", "", owner.ID); err == nil {
		t.Fatal("expected duplicate name error")
	}
}

func TestCommunityStore_MembershipRoles(t *testing.T) {
	requireDB(t)
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()
	users := store.NewUserStore(pgPool)
	communities := store.NewCommunityStore(pgPool)

	owner := testUser(t, "owner3")
	member := testUser(t, "member3")
	if err := users.UpsertUser(ctx, owner); err != nil {
		t.Fatal(err)
	}
	if err := users.UpsertUser(ctx, member); err != nil {
		t.Fatal(err)
	}

	c, err := communities.Create(ctx, "Membership Test", "", owner.ID)
	if err != nil {
		t.Fatal(err)
	}

	role, err := communities.GetMemberRole(ctx, c.ID, member.ID)
	if err != nil {
		t.Fatal(err)
	}
	if role != "" {
		t.Fatalf("expected no role, got %q", role)
	}

	if err := communities.AddMember(ctx, c.ID, member.ID, "member"); err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	role, err = communities.GetMemberRole(ctx, c.ID, member.ID)
	if err != nil {
		t.Fatal(err)
	}
	if role != "member" {
		t.Fatalf("expected member role, got %q", role)
	}

	if err := communities.AddMember(ctx, c.ID, member.ID, "moderator"); err != nil {
		t.Fatalf("AddMember upsert: %v", err)
	}
	role, err = communities.GetMemberRole(ctx, c.ID, member.ID)
	if err != nil {
		t.Fatal(err)
	}
	if role != "moderator" {
		t.Fatalf("expected role updated to moderator, got %q", role)
	}

	members, err := communities.GetMembers(ctx, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != 1 {
		t.Fatalf("expected 1 member, got %d", len(members))
	}

	if err := communities.RemoveMember(ctx, c.ID, member.ID); err != nil {
		t.Fatalf("RemoveMember: %v", err)
	}
	role, err = communities.GetMemberRole(ctx, c.ID, member.ID)
	if err != nil {
		t.Fatal(err)
	}
	if role != "" {
		t.Fatalf("expected removed, got %q", role)
	}
}

func TestCommunityStore_IsOwnerAndModerator(t *testing.T) {
	requireDB(t)
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()
	users := store.NewUserStore(pgPool)
	communities := store.NewCommunityStore(pgPool)

	owner := testUser(t, "owner4")
	mod := testUser(t, "mod4")
	plain := testUser(t, "plain4")
	for _, u := range []*store.User{owner, mod, plain} {
		if err := users.UpsertUser(ctx, u); err != nil {
			t.Fatal(err)
		}
	}

	c, err := communities.Create(ctx, "Roles Test", "", owner.ID)
	if err != nil {
		t.Fatal(err)
	}

	isOwner, err := communities.IsOwner(ctx, c.ID, owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !isOwner {
		t.Fatal("expected owner")
	}

	canMod, err := communities.IsModeratorOrOwner(ctx, c.ID, mod.ID)
	if err != nil {
		t.Fatal(err)
	}
	if canMod {
		t.Fatal("expected mod not able to moderate yet")
	}

	if err := communities.AddModerator(ctx, c.ID, mod.ID); err != nil {
		t.Fatalf("AddModerator: %v", err)
	}
	canMod, err = communities.IsModeratorOrOwner(ctx, c.ID, mod.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !canMod {
		t.Fatal("expected mod able to moderate")
	}

	canMod, err = communities.IsModeratorOrOwner(ctx, c.ID, plain.ID)
	if err != nil {
		t.Fatal(err)
	}
	if canMod {
		t.Fatal("expected plain member not able to moderate")
	}

	isOwner, err = communities.IsOwner(ctx, c.ID, plain.ID)
	if err != nil {
		t.Fatal(err)
	}
	if isOwner {
		t.Fatal("expected plain member not owner")
	}

	if err := communities.TransferOwnership(ctx, c.ID, plain.ID); err != nil {
		t.Fatalf("TransferOwnership: %v", err)
	}
	isOwner, err = communities.IsOwner(ctx, c.ID, plain.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !isOwner {
		t.Fatal("expected plain member to become owner")
	}
}

func TestCommunityStore_RegenerateJoinLink(t *testing.T) {
	requireDB(t)
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()
	users := store.NewUserStore(pgPool)
	communities := store.NewCommunityStore(pgPool)

	owner := testUser(t, "owner5")
	if err := users.UpsertUser(ctx, owner); err != nil {
		t.Fatal(err)
	}

	c, err := communities.Create(ctx, "Join Link Test", "", owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.JoinLinkID) != 8 {
		t.Fatalf("expected 8-char join link, got %q", c.JoinLinkID)
	}

	newLink, err := communities.RegenerateJoinLink(ctx, c.ID)
	if err != nil {
		t.Fatalf("RegenerateJoinLink: %v", err)
	}
	if newLink == c.JoinLinkID {
		t.Fatal("expected new join link to differ")
	}

	got, err := communities.GetByID(ctx, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.JoinLinkID != newLink {
		t.Fatalf("expected %s, got %s", newLink, got.JoinLinkID)
	}
}

func TestCommunityStore_UpdateLogoAndDelete(t *testing.T) {
	requireDB(t)
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()
	users := store.NewUserStore(pgPool)
	communities := store.NewCommunityStore(pgPool)

	owner := testUser(t, "owner6")
	if err := users.UpsertUser(ctx, owner); err != nil {
		t.Fatal(err)
	}

	c, err := communities.Create(ctx, "Delete Test", "desc", owner.ID)
	if err != nil {
		t.Fatal(err)
	}

	if err := communities.UpdateLogoURL(ctx, c.ID, "https://example.com/logo.png"); err != nil {
		t.Fatalf("UpdateLogoURL: %v", err)
	}
	got, err := communities.GetByID(ctx, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.LogoURL != "https://example.com/logo.png" {
		t.Fatalf("expected logo url, got %q", got.LogoURL)
	}

	if err := communities.Delete(ctx, c.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	got, err = communities.GetByID(ctx, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatal("expected nil after delete")
	}
}

func TestCommunityStore_Update(t *testing.T) {
	requireDB(t)
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()
	users := store.NewUserStore(pgPool)
	communities := store.NewCommunityStore(pgPool)

	owner := testUser(t, "owner7")
	if err := users.UpsertUser(ctx, owner); err != nil {
		t.Fatal(err)
	}

	c, err := communities.Create(ctx, "Original", "old desc", owner.ID)
	if err != nil {
		t.Fatal(err)
	}

	if err := communities.Update(ctx, c.ID, "Renamed", "new desc"); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := communities.GetByID(ctx, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Renamed" || got.Description != "new desc" {
		t.Fatalf("expected updated community, got %+v", got)
	}
}

func TestCommunityStore_JoinLinkAlphabet(t *testing.T) {
	// Guard against ambiguous characters that are excluded from the alphabet.
	allowed := "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghjkmnpqrstuvwxyz23456789"
	for _, ch := range allowed {
		if strings.ContainsRune("0O1Il", ch) {
			t.Fatalf("ambiguous character %q in join link alphabet", ch)
		}
	}
}
