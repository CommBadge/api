package store

import (
	"context"
	"testing"

	"github.com/pashagolub/pgxmock/v4"
)

func subCols() []string {
	return []string{"id", "user_id", "twitch_event_type", "condition",
		"twitch_subscription_id", "status", "created_at", "updated_at"}
}

func TestNotificationStore_Create(t *testing.T) {
	ctx := context.Background()
	t.Run("ok", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("INSERT INTO notification_subscriptions").
			WithArgs("u1", "stream.online", []byte(`{"broadcaster_user_id":"123"}`), "sub-tw").
			WillReturnRows(pgxmock.NewRows(subCols()).
				AddRow("n1", "u1", "stream.online", []byte(`{"broadcaster_user_id":"123"}`), "sub-tw", "enabled", testTime, testTime))
		sub, err := NewNotificationStore(mock).Create(ctx, "u1", "stream.online", map[string]string{"broadcaster_user_id": "123"}, "sub-tw")
		if err != nil || sub == nil || sub.ID != "n1" || sub.Condition["broadcaster_user_id"] != "123" {
			t.Fatalf("Create = %+v, %v", sub, err)
		}
	})
	t.Run("query error", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("INSERT INTO notification_subscriptions").WillReturnError(errDB)
		if _, err := NewNotificationStore(mock).Create(ctx, "u1", "stream.online", map[string]string{}, "sub-tw"); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("unmarshal error", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("INSERT INTO notification_subscriptions").
			WillReturnRows(pgxmock.NewRows(subCols()).
				AddRow("n1", "u1", "stream.online", []byte(`not-json`), "sub-tw", "enabled", testTime, testTime))
		if _, err := NewNotificationStore(mock).Create(ctx, "u1", "stream.online", map[string]string{}, "sub-tw"); err == nil {
			t.Fatal("expected unmarshal error")
		}
	})
}

func TestNotificationStore_GetByID(t *testing.T) {
	ctx := context.Background()
	t.Run("found", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("FROM notification_subscriptions WHERE id").WithArgs("n1").
			WillReturnRows(pgxmock.NewRows(subCols()).
				AddRow("n1", "u1", "stream.online", []byte(`{}`), "sub-tw", "enabled", testTime, testTime))
		sub, err := NewNotificationStore(mock).GetByID(ctx, "n1")
		if err != nil || sub == nil {
			t.Fatalf("GetByID = %+v, %v", sub, err)
		}
	})
	t.Run("not found", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("FROM notification_subscriptions WHERE id").WithArgs("n1").WillReturnRows(pgxmock.NewRows(subCols()))
		sub, err := NewNotificationStore(mock).GetByID(ctx, "n1")
		if err != nil || sub != nil {
			t.Fatalf("expected nil, got %+v, %v", sub, err)
		}
	})
	t.Run("error", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("FROM notification_subscriptions WHERE id").WillReturnError(errDB)
		if _, err := NewNotificationStore(mock).GetByID(ctx, "n1"); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestNotificationStore_HasActive(t *testing.T) {
	ctx := context.Background()
	t.Run("true", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("SELECT EXISTS").
			WithArgs("u1", "stream.online").
			WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(true))
		got, err := NewNotificationStore(mock).HasActive(ctx, "u1", "stream.online")
		if err != nil || !got {
			t.Fatalf("HasActive = %v, %v", got, err)
		}
	})
	t.Run("false", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("SELECT EXISTS").
			WithArgs("u1", "stream.online").
			WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(false))
		got, err := NewNotificationStore(mock).HasActive(ctx, "u1", "stream.online")
		if err != nil || got {
			t.Fatalf("HasActive = %v, %v", got, err)
		}
	})
	t.Run("error", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("SELECT EXISTS").WillReturnError(errDB)
		if _, err := NewNotificationStore(mock).HasActive(ctx, "u1", "stream.online"); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestNotificationStore_ListActiveByUser(t *testing.T) {
	ctx := context.Background()
	t.Run("rows", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("status = 'enabled'").
			WithArgs("u1").
			WillReturnRows(pgxmock.NewRows(subCols()).
				AddRow("n1", "u1", "stream.online", []byte(`{}`), "tw", "enabled", testTime, testTime))
		subs, err := NewNotificationStore(mock).ListActiveByUser(ctx, "u1")
		if err != nil || len(subs) != 1 {
			t.Fatalf("ListActiveByUser = %v, %v", subs, err)
		}
	})
	t.Run("query error", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("status = 'enabled'").WillReturnError(errDB)
		if _, err := NewNotificationStore(mock).ListActiveByUser(ctx, "u1"); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("unmarshal error", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("status = 'enabled'").
			WithArgs("u1").
			WillReturnRows(pgxmock.NewRows(subCols()).
				AddRow("n1", "u1", "stream.online", []byte(`bad`), "tw", "enabled", testTime, testTime))
		if _, err := NewNotificationStore(mock).ListActiveByUser(ctx, "u1"); err == nil {
			t.Fatal("expected unmarshal error")
		}
	})
}

func TestNotificationStore_MarkRevoked(t *testing.T) {
	ctx := context.Background()
	t.Run("ok", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectExec("status = 'revoked'").WithArgs("n1").WillReturnResult(pgxmock.NewResult("UPDATE", 1))
		if err := NewNotificationStore(mock).MarkRevoked(ctx, "n1"); err != nil {
			t.Fatalf("MarkRevoked: %v", err)
		}
	})
	t.Run("error", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectExec("status = 'revoked'").WillReturnError(errDB)
		if err := NewNotificationStore(mock).MarkRevoked(ctx, "n1"); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestNotificationStore_RevokeAllByUser(t *testing.T) {
	ctx := context.Background()
	t.Run("rows", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("UPDATE notification_subscriptions").
			WithArgs("u1").
			WillReturnRows(pgxmock.NewRows(subCols()).
				AddRow("n1", "u1", "stream.online", []byte(`{}`), "tw", "revoked", testTime, testTime))
		subs, err := NewNotificationStore(mock).RevokeAllByUser(ctx, "u1")
		if err != nil || len(subs) != 1 {
			t.Fatalf("RevokeAllByUser = %v, %v", subs, err)
		}
	})
	t.Run("query error", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("UPDATE notification_subscriptions").WillReturnError(errDB)
		if _, err := NewNotificationStore(mock).RevokeAllByUser(ctx, "u1"); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("unmarshal error", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("UPDATE notification_subscriptions").
			WithArgs("u1").
			WillReturnRows(pgxmock.NewRows(subCols()).
				AddRow("n1", "u1", "stream.online", []byte(`bad`), "tw", "revoked", testTime, testTime))
		if _, err := NewNotificationStore(mock).RevokeAllByUser(ctx, "u1"); err == nil {
			t.Fatal("expected unmarshal error")
		}
	})
}
