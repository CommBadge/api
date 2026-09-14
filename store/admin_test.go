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

var errDB = errors.New("db boom")

func newMock(t *testing.T) pgxmock.PgxPoolIface {
	t.Helper()
	mock, err := pgxmock.NewPool(pgxmock.QueryMatcherOption(pgxmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("pgxmock.NewPool: %v", err)
	}
	t.Cleanup(func() { mock.Close() })
	return mock
}

func TestNewPool(t *testing.T) {
	t.Run("invalid url", func(t *testing.T) {
		if _, err := NewPool(context.Background(), "://bad"); err == nil {
			t.Fatal("expected parse error")
		}
	})
	t.Run("ping failure", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()
		if _, err := NewPool(ctx, "postgres://127.0.0.1:1/db"); err == nil {
			t.Fatal("expected ping error")
		}
	})
}

func TestAdminStore_IsAdmin(t *testing.T) {
	ctx := context.Background()
	t.Run("true", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery(regexp.QuoteMeta("SELECT EXISTS")).
			WithArgs("u1").
			WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(true))
		got, err := NewAdminStore(mock).IsAdmin(ctx, "u1")
		if err != nil || !got {
			t.Fatalf("IsAdmin = %v, %v", got, err)
		}
	})
	t.Run("false", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("SELECT EXISTS").
			WithArgs("u2").
			WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(false))
		got, err := NewAdminStore(mock).IsAdmin(ctx, "u2")
		if err != nil || got {
			t.Fatalf("IsAdmin = %v, %v", got, err)
		}
	})
	t.Run("error", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("SELECT EXISTS").WithArgs("u3").WillReturnError(errDB)
		if _, err := NewAdminStore(mock).IsAdmin(ctx, "u3"); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestAdminStore_SeedAdmins(t *testing.T) {
	ctx := context.Background()
	t.Run("success", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectExec("INSERT INTO users").WithArgs("a1").WillReturnResult(pgxmock.NewResult("INSERT", 1))
		mock.ExpectExec("INSERT INTO platform_admins").WithArgs("a1").WillReturnResult(pgxmock.NewResult("INSERT", 1))
		mock.ExpectCommit()
		mock.ExpectBegin()
		mock.ExpectExec("INSERT INTO users").WithArgs("a2").WillReturnResult(pgxmock.NewResult("INSERT", 1))
		mock.ExpectExec("INSERT INTO platform_admins").WithArgs("a2").WillReturnResult(pgxmock.NewResult("INSERT", 1))
		mock.ExpectCommit()
		if err := NewAdminStore(mock).SeedAdmins(ctx, []string{"a1", "a2"}); err != nil {
			t.Fatalf("SeedAdmins: %v", err)
		}
	})
	t.Run("begin error", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectBegin().WillReturnError(errDB)
		if err := NewAdminStore(mock).SeedAdmins(ctx, []string{"a1"}); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("insert users error", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectExec("INSERT INTO users").WillReturnError(errDB)
		mock.ExpectRollback()
		if err := NewAdminStore(mock).SeedAdmins(ctx, []string{"a1"}); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("insert admins error", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectExec("INSERT INTO users").WillReturnResult(pgxmock.NewResult("INSERT", 1))
		mock.ExpectExec("INSERT INTO platform_admins").WillReturnError(errDB)
		mock.ExpectRollback()
		if err := NewAdminStore(mock).SeedAdmins(ctx, []string{"a1"}); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("commit error", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectExec("INSERT INTO users").WillReturnResult(pgxmock.NewResult("INSERT", 1))
		mock.ExpectExec("INSERT INTO platform_admins").WillReturnResult(pgxmock.NewResult("INSERT", 1))
		mock.ExpectCommit().WillReturnError(errDB)
		if err := NewAdminStore(mock).SeedAdmins(ctx, []string{"a1"}); err == nil {
			t.Fatal("expected error")
		}
	})
}

var _ pgx.Row = (pgx.Row)(nil)
