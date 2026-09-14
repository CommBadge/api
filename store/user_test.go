package store

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
)

var testTime = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

func ptrStr(s string) *string        { return &s }
func ptrTime(t time.Time) *time.Time { return &t }

func userCols() []string {
	return []string{"id", "username", "display_name", "email", "avatar_url",
		"twitch_id", "twitch_linked_at", "created_at", "updated_at", "shoutout_template"}
}

func userAdminCols() []string {
	c := userCols()
	return append(c, "warnings", "banned", "banned_at", "ban_reason")
}

func TestUserStore_UpsertUser(t *testing.T) {
	ctx := context.Background()
	t.Run("ok", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectExec("INSERT INTO users").
			WithArgs("u1", "kronus", "Kronus", "k@e.c", "http://a").
			WillReturnResult(pgxmock.NewResult("INSERT", 1))
		if err := NewUserStore(mock).UpsertUser(ctx, &User{ID: "u1", Username: "kronus", DisplayName: "Kronus", Email: "k@e.c", AvatarURL: "http://a"}); err != nil {
			t.Fatalf("UpsertUser: %v", err)
		}
	})
	t.Run("error", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectExec("INSERT INTO users").WillReturnError(errDB)
		if err := NewUserStore(mock).UpsertUser(ctx, &User{}); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestUserStore_GetUserByID(t *testing.T) {
	ctx := context.Background()
	t.Run("found", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("FROM users WHERE id").
			WithArgs("u1").
			WillReturnRows(pgxmock.NewRows(userCols()).AddRow("u1", "kronus", "Kronus", "k@e.c", "http://a", nil, nil, testTime, testTime, ""))
		u, err := NewUserStore(mock).GetUserByID(ctx, "u1")
		if err != nil || u == nil || u.ID != "u1" || u.TwitchID != "" {
			t.Fatalf("GetUserByID = %+v, %v", u, err)
		}
	})
	t.Run("found with twitch", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("FROM users WHERE id").
			WithArgs("u1").
			WillReturnRows(pgxmock.NewRows(userCols()).AddRow("u1", "k", "K", "e", "a", ptrStr("tw-123"), ptrTime(testTime), testTime, testTime, ""))
		u, err := NewUserStore(mock).GetUserByID(ctx, "u1")
		if err != nil || u == nil || u.TwitchID != "tw-123" {
			t.Fatalf("twitch = %q, %v", u.TwitchID, err)
		}
	})
	t.Run("not found", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("FROM users WHERE id").WithArgs("ghost").WillReturnRows(pgxmock.NewRows(userCols()))
		u, err := NewUserStore(mock).GetUserByID(ctx, "ghost")
		if err != nil || u != nil {
			t.Fatalf("expected nil, got %+v, %v", u, err)
		}
	})
	t.Run("scan error", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("FROM users WHERE id").WithArgs("u1").WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("u1"))
		if _, err := NewUserStore(mock).GetUserByID(ctx, "u1"); err == nil {
			t.Fatal("expected scan error")
		}
	})
}

func TestUserStore_TwitchLink(t *testing.T) {
	ctx := context.Background()
	t.Run("save", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectExec("UPDATE users SET twitch_id").WithArgs("tw-1", "u1").WillReturnResult(pgxmock.NewResult("UPDATE", 1))
		if err := NewUserStore(mock).SaveTwitchLink(ctx, "u1", "tw-1"); err != nil {
			t.Fatalf("SaveTwitchLink: %v", err)
		}
	})
	t.Run("save error", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectExec("UPDATE users SET twitch_id").WillReturnError(errDB)
		if err := NewUserStore(mock).SaveTwitchLink(ctx, "u1", "tw-1"); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("get id", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("COALESCE").WithArgs("u1").WillReturnRows(pgxmock.NewRows([]string{"twitch_id"}).AddRow("tw-1"))
		got, err := NewUserStore(mock).GetTwitchID(ctx, "u1")
		if err != nil || got != "tw-1" {
			t.Fatalf("GetTwitchID = %q, %v", got, err)
		}
	})
	t.Run("get id missing", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("COALESCE").WithArgs("u1").WillReturnRows(pgxmock.NewRows([]string{"twitch_id"}))
		got, err := NewUserStore(mock).GetTwitchID(ctx, "u1")
		if err != nil || got != "" {
			t.Fatalf("GetTwitchID = %q, %v", got, err)
		}
	})
	t.Run("get id error", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("COALESCE").WillReturnError(errDB)
		if _, err := NewUserStore(mock).GetTwitchID(ctx, "u1"); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("clear", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectExec("twitch_id = NULL").WithArgs("u1").WillReturnResult(pgxmock.NewResult("UPDATE", 1))
		if err := NewUserStore(mock).ClearTwitchLink(ctx, "u1"); err != nil {
			t.Fatalf("ClearTwitchLink: %v", err)
		}
	})
	t.Run("clear error", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectExec("twitch_id = NULL").WillReturnError(errDB)
		if err := NewUserStore(mock).ClearTwitchLink(ctx, "u1"); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestUserStore_Settings(t *testing.T) {
	ctx := context.Background()
	t.Run("set notification channel", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectExec("notification_discord_channel_id").WithArgs("ch-1", "u1").WillReturnResult(pgxmock.NewResult("UPDATE", 1))
		if err := NewUserStore(mock).SetNotificationChannel(ctx, "u1", "ch-1"); err != nil {
			t.Fatalf("SetNotificationChannel: %v", err)
		}
	})
	t.Run("set notification channel error", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectExec("notification_discord_channel_id").WillReturnError(errDB)
		if err := NewUserStore(mock).SetNotificationChannel(ctx, "u1", "ch-1"); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("set shoutout template", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectExec("shoutout_template").WithArgs("tmpl", "u1").WillReturnResult(pgxmock.NewResult("UPDATE", 1))
		if err := NewUserStore(mock).SetShoutoutTemplate(ctx, "u1", "tmpl"); err != nil {
			t.Fatalf("SetShoutoutTemplate: %v", err)
		}
	})
	t.Run("set shoutout template error", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectExec("shoutout_template").WillReturnError(errDB)
		if err := NewUserStore(mock).SetShoutoutTemplate(ctx, "u1", "tmpl"); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestUserStore_IsBanned(t *testing.T) {
	ctx := context.Background()
	t.Run("banned", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("SELECT banned FROM users").WithArgs("u1").WillReturnRows(pgxmock.NewRows([]string{"banned"}).AddRow(true))
		got, err := NewUserStore(mock).IsBanned(ctx, "u1")
		if err != nil || !got {
			t.Fatalf("IsBanned = %v, %v", got, err)
		}
	})
	t.Run("not found", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("SELECT banned FROM users").WithArgs("u1").WillReturnRows(pgxmock.NewRows([]string{"banned"}))
		got, err := NewUserStore(mock).IsBanned(ctx, "u1")
		if err != nil || got {
			t.Fatalf("IsBanned = %v, %v", got, err)
		}
	})
	t.Run("error", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("SELECT banned FROM users").WillReturnError(errDB)
		if _, err := NewUserStore(mock).IsBanned(ctx, "u1"); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestUserStore_ListUsers(t *testing.T) {
	ctx := context.Background()
	t.Run("rows", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("ORDER BY created_at DESC").
			WithArgs(10, 0).
			WillReturnRows(pgxmock.NewRows(userAdminCols()).
				AddRow("u1", "k", "K", "e", "a", nil, nil, testTime, testTime, "", 2, true, ptrTime(testTime), "spam").
				AddRow("u2", "t", "T", "e2", "a2", ptrStr("tw"), ptrTime(testTime), testTime, testTime, "tmpl", 0, false, nil, ""))
		users, err := NewUserStore(mock).ListUsers(ctx, 10, 0)
		if err != nil || len(users) != 2 {
			t.Fatalf("ListUsers = %v, %v", users, err)
		}
		if users[1].TwitchID != "tw" {
			t.Errorf("twitch = %q", users[1].TwitchID)
		}
	})
	t.Run("query error", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("ORDER BY created_at DESC").WillReturnError(errDB)
		if _, err := NewUserStore(mock).ListUsers(ctx, 10, 0); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("scan error", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("ORDER BY created_at DESC").
			WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("u1"))
		if _, err := NewUserStore(mock).ListUsers(ctx, 10, 0); err == nil {
			t.Fatal("expected scan error")
		}
	})
}

func TestUserStore_SearchUsers(t *testing.T) {
	ctx := context.Background()
	t.Run("rows", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("ILIKE").
			WithArgs("%kron%", 10).
			WillReturnRows(pgxmock.NewRows(userAdminCols()).
				AddRow("u1", "kronus", "Kronus", "e", "a", nil, nil, testTime, testTime, "", 0, false, nil, ""))
		users, err := NewUserStore(mock).SearchUsers(ctx, "kron", 10)
		if err != nil || len(users) != 1 {
			t.Fatalf("SearchUsers = %v, %v", users, err)
		}
	})
	t.Run("query error", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("ILIKE").WillReturnError(errDB)
		if _, err := NewUserStore(mock).SearchUsers(ctx, "kron", 10); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestUserStore_GetWarningLog(t *testing.T) {
	ctx := context.Background()
	t.Run("rows", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("FROM user_warnings").
			WithArgs("u1").
			WillReturnRows(pgxmock.NewRows([]string{"id", "user_id", "admin_id", "reason", "created_at"}).
				AddRow("w1", "u1", "a1", "spam", testTime))
		ws, err := NewUserStore(mock).GetWarningLog(ctx, "u1")
		if err != nil || len(ws) != 1 || ws[0].Reason != "spam" {
			t.Fatalf("GetWarningLog = %v, %v", ws, err)
		}
	})
	t.Run("query error", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("FROM user_warnings").WillReturnError(errDB)
		if _, err := NewUserStore(mock).GetWarningLog(ctx, "u1"); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("scan error", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("FROM user_warnings").
			WithArgs("u1").
			WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("w1"))
		if _, err := NewUserStore(mock).GetWarningLog(ctx, "u1"); err == nil {
			t.Fatal("expected scan error")
		}
	})
}

func TestUserStore_AddWarning(t *testing.T) {
	ctx := context.Background()
	t.Run("success", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectExec("INSERT INTO user_warnings").WithArgs("u1", "a1", "spam").WillReturnResult(pgxmock.NewResult("INSERT", 1))
		mock.ExpectExec("warnings = warnings").WithArgs("u1").WillReturnResult(pgxmock.NewResult("UPDATE", 1))
		mock.ExpectCommit()
		if err := NewUserStore(mock).AddWarning(ctx, "u1", "a1", "spam"); err != nil {
			t.Fatalf("AddWarning: %v", err)
		}
	})
	t.Run("begin error", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectBegin().WillReturnError(errDB)
		if err := NewUserStore(mock).AddWarning(ctx, "u1", "a1", "spam"); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("insert error", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectExec("INSERT INTO user_warnings").WillReturnError(errDB)
		mock.ExpectRollback()
		if err := NewUserStore(mock).AddWarning(ctx, "u1", "a1", "spam"); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("update error", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectExec("INSERT INTO user_warnings").WillReturnResult(pgxmock.NewResult("INSERT", 1))
		mock.ExpectExec("warnings = warnings").WillReturnError(errDB)
		mock.ExpectRollback()
		if err := NewUserStore(mock).AddWarning(ctx, "u1", "a1", "spam"); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("commit error", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectExec("INSERT INTO user_warnings").WillReturnResult(pgxmock.NewResult("INSERT", 1))
		mock.ExpectExec("warnings = warnings").WillReturnResult(pgxmock.NewResult("UPDATE", 1))
		mock.ExpectCommit().WillReturnError(errDB)
		if err := NewUserStore(mock).AddWarning(ctx, "u1", "a1", "spam"); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestUserStore_BanUnban(t *testing.T) {
	ctx := context.Background()
	t.Run("ban", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectExec("banned = true").WithArgs("spam", "u1").WillReturnResult(pgxmock.NewResult("UPDATE", 1))
		if err := NewUserStore(mock).BanUser(ctx, "u1", "spam"); err != nil {
			t.Fatalf("BanUser: %v", err)
		}
	})
	t.Run("ban error", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectExec("banned = true").WillReturnError(errDB)
		if err := NewUserStore(mock).BanUser(ctx, "u1", "spam"); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("unban", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectExec("banned = false").WithArgs("u1").WillReturnResult(pgxmock.NewResult("UPDATE", 1))
		if err := NewUserStore(mock).UnbanUser(ctx, "u1"); err != nil {
			t.Fatalf("UnbanUser: %v", err)
		}
	})
	t.Run("unban error", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectExec("banned = false").WillReturnError(errDB)
		if err := NewUserStore(mock).UnbanUser(ctx, "u1"); err == nil {
			t.Fatal("expected error")
		}
	})
}

var _ pgx.Rows = (pgx.Rows)(nil)
var _ = errors.New
var _ = regexp.QuoteMeta
