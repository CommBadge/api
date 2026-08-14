//go:build integration

package integration

import (
	"context"
	"testing"

	"kronus.dev/commbadge_api/store"
)

func TestTicketStore_CreateAndList(t *testing.T) {
	requireDB(t)
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()
	users := store.NewUserStore(pgPool)
	tickets := store.NewTicketStore(pgPool)

	u := testUser(t, "ticket1")
	if err := users.UpsertUser(ctx, u); err != nil {
		t.Fatal(err)
	}

	ticket, err := tickets.Create(ctx, u.ID, "Can't login", "Help needed please")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if ticket.ID == "" || ticket.Status != "open" {
		t.Fatalf("unexpected ticket: %+v", ticket)
	}

	list, err := tickets.ListByUser(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 ticket, got %d", len(list))
	}

	got, err := tickets.GetByID(ctx, ticket.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.Subject != "Can't login" || got.Body != "Help needed please" {
		t.Fatalf("mismatch: %+v", got)
	}
}

func TestTicketStore_GetByIDNotFound(t *testing.T) {
	requireDB(t)
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()
	tickets := store.NewTicketStore(pgPool)

	got, err := tickets.GetByID(ctx, "00000000-0000-0000-0000-000000000000")
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatal("expected nil")
	}
}

func TestTicketStore_Messages(t *testing.T) {
	requireDB(t)
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()
	users := store.NewUserStore(pgPool)
	tickets := store.NewTicketStore(pgPool)

	u := testUser(t, "ticket2")
	if err := users.UpsertUser(ctx, u); err != nil {
		t.Fatal(err)
	}

	ticket, err := tickets.Create(ctx, u.ID, "Bug", "It broke")
	if err != nil {
		t.Fatal(err)
	}

	userMsg, err := tickets.AddMessage(ctx, ticket.ID, u.ID, "Still broken", false)
	if err != nil {
		t.Fatalf("AddMessage: %v", err)
	}
	if userMsg.IsAdmin {
		t.Fatal("expected is_admin=false")
	}

	adminMsg, err := tickets.AddMessage(ctx, ticket.ID, u.ID, "We're on it", true)
	if err != nil {
		t.Fatalf("AddMessage admin: %v", err)
	}
	if !adminMsg.IsAdmin {
		t.Fatal("expected is_admin=true")
	}

	msgs, err := tickets.GetMessages(ctx, ticket.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Body != "Still broken" || msgs[1].Body != "We're on it" {
		t.Fatalf("unexpected message order/content: %+v", msgs)
	}
}

func TestTicketStore_AdminListCountStatus(t *testing.T) {
	requireDB(t)
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()
	users := store.NewUserStore(pgPool)
	tickets := store.NewTicketStore(pgPool)

	u := testUser(t, "ticket3")
	if err := users.UpsertUser(ctx, u); err != nil {
		t.Fatal(err)
	}

	ticket, err := tickets.Create(ctx, u.ID, "Question", "Is this thing on?")
	if err != nil {
		t.Fatal(err)
	}

	count, err := tickets.AdminTicketCount(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected count 1, got %d", count)
	}

	adminList, err := tickets.AdminListAll(ctx, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(adminList) != 1 {
		t.Fatalf("expected 1 in admin list, got %d", len(adminList))
	}

	if err := tickets.AdminUpdateStatus(ctx, ticket.ID, "resolved"); err != nil {
		t.Fatalf("AdminUpdateStatus: %v", err)
	}
	got, err := tickets.GetByID(ctx, ticket.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "resolved" {
		t.Fatalf("expected resolved, got %q", got.Status)
	}
}
