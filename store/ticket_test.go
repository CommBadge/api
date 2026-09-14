package store

import (
	"context"
	"testing"

	"github.com/pashagolub/pgxmock/v4"
)

func ticketCols() []string {
	return []string{"id", "user_id", "subject", "body", "status", "created_at", "updated_at"}
}

func msgCols() []string {
	return []string{"id", "ticket_id", "user_id", "body", "is_admin", "created_at"}
}

func TestTicketStore_Create(t *testing.T) {
	ctx := context.Background()
	t.Run("ok", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("INSERT INTO support_tickets").
			WithArgs("u1", "subj", "body").
			WillReturnRows(pgxmock.NewRows(ticketCols()).
				AddRow("t1", "u1", "subj", "body", "open", testTime, testTime))
		tk, err := NewTicketStore(mock).Create(ctx, "u1", "subj", "body")
		if err != nil || tk == nil || tk.ID != "t1" {
			t.Fatalf("Create = %+v, %v", tk, err)
		}
	})
	t.Run("error", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("INSERT INTO support_tickets").WillReturnError(errDB)
		if _, err := NewTicketStore(mock).Create(ctx, "u1", "subj", "body"); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestTicketStore_ListByUser(t *testing.T) {
	ctx := context.Background()
	t.Run("rows", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("WHERE user_id").WithArgs("u1").
			WillReturnRows(pgxmock.NewRows(ticketCols()).
				AddRow("t1", "u1", "subj", "body", "open", testTime, testTime))
		ts, err := NewTicketStore(mock).ListByUser(ctx, "u1")
		if err != nil || len(ts) != 1 {
			t.Fatalf("ListByUser = %v, %v", ts, err)
		}
	})
	t.Run("query error", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("WHERE user_id").WillReturnError(errDB)
		if _, err := NewTicketStore(mock).ListByUser(ctx, "u1"); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("scan error", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("WHERE user_id").
			WithArgs("u1").
			WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("t1"))
		if _, err := NewTicketStore(mock).ListByUser(ctx, "u1"); err == nil {
			t.Fatal("expected scan error")
		}
	})
}

func TestTicketStore_GetByID(t *testing.T) {
	ctx := context.Background()
	t.Run("found", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("FROM support_tickets WHERE id").WithArgs("t1").
			WillReturnRows(pgxmock.NewRows(ticketCols()).
				AddRow("t1", "u1", "subj", "body", "open", testTime, testTime))
		tk, err := NewTicketStore(mock).GetByID(ctx, "t1")
		if err != nil || tk == nil {
			t.Fatalf("GetByID = %+v, %v", tk, err)
		}
	})
	t.Run("not found", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("FROM support_tickets WHERE id").WithArgs("t1").WillReturnRows(pgxmock.NewRows(ticketCols()))
		tk, err := NewTicketStore(mock).GetByID(ctx, "t1")
		if err != nil || tk != nil {
			t.Fatalf("expected nil, got %+v, %v", tk, err)
		}
	})
	t.Run("error", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("FROM support_tickets WHERE id").WillReturnError(errDB)
		if _, err := NewTicketStore(mock).GetByID(ctx, "t1"); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestTicketStore_AddMessage(t *testing.T) {
	ctx := context.Background()
	t.Run("ok", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("INSERT INTO ticket_messages").
			WithArgs("t1", "u1", "hi", true).
			WillReturnRows(pgxmock.NewRows(msgCols()).
				AddRow("m1", "t1", "u1", "hi", true, testTime))
		mock.ExpectExec("UPDATE support_tickets SET updated_at").WithArgs("t1").WillReturnResult(pgxmock.NewResult("UPDATE", 1))
		m, err := NewTicketStore(mock).AddMessage(ctx, "t1", "u1", "hi", true)
		if err != nil || m == nil || m.ID != "m1" {
			t.Fatalf("AddMessage = %+v, %v", m, err)
		}
	})
	t.Run("insert error", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("INSERT INTO ticket_messages").WillReturnError(errDB)
		if _, err := NewTicketStore(mock).AddMessage(ctx, "t1", "u1", "hi", true); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestTicketStore_GetMessages(t *testing.T) {
	ctx := context.Background()
	t.Run("rows", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("FROM ticket_messages").WithArgs("t1").
			WillReturnRows(pgxmock.NewRows(msgCols()).
				AddRow("m1", "t1", "u1", "hi", true, testTime))
		ms, err := NewTicketStore(mock).GetMessages(ctx, "t1")
		if err != nil || len(ms) != 1 {
			t.Fatalf("GetMessages = %v, %v", ms, err)
		}
	})
	t.Run("query error", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("FROM ticket_messages").WillReturnError(errDB)
		if _, err := NewTicketStore(mock).GetMessages(ctx, "t1"); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("scan error", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("FROM ticket_messages").
			WithArgs("t1").
			WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("m1"))
		if _, err := NewTicketStore(mock).GetMessages(ctx, "t1"); err == nil {
			t.Fatal("expected scan error")
		}
	})
}

func TestTicketStore_AdminListAll(t *testing.T) {
	ctx := context.Background()
	t.Run("rows", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("ORDER BY created_at DESC").
			WithArgs(10, 0).
			WillReturnRows(pgxmock.NewRows(ticketCols()).
				AddRow("t1", "u1", "subj", "body", "open", testTime, testTime))
		ts, err := NewTicketStore(mock).AdminListAll(ctx, 10, 0)
		if err != nil || len(ts) != 1 {
			t.Fatalf("AdminListAll = %v, %v", ts, err)
		}
	})
	t.Run("query error", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("ORDER BY created_at DESC").WillReturnError(errDB)
		if _, err := NewTicketStore(mock).AdminListAll(ctx, 10, 0); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("scan error", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("ORDER BY created_at DESC").
			WithArgs(10, 0).
			WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("t1"))
		if _, err := NewTicketStore(mock).AdminListAll(ctx, 10, 0); err == nil {
			t.Fatal("expected scan error")
		}
	})
}

func TestTicketStore_AdminTicketCount(t *testing.T) {
	ctx := context.Background()
	t.Run("ok", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("SELECT COUNT").WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(5))
		got, err := NewTicketStore(mock).AdminTicketCount(ctx)
		if err != nil || got != 5 {
			t.Fatalf("AdminTicketCount = %d, %v", got, err)
		}
	})
	t.Run("error", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectQuery("SELECT COUNT").WillReturnError(errDB)
		if _, err := NewTicketStore(mock).AdminTicketCount(ctx); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestTicketStore_AdminUpdateStatus(t *testing.T) {
	ctx := context.Background()
	t.Run("ok", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectExec("UPDATE support_tickets SET status").WithArgs("closed", "t1").WillReturnResult(pgxmock.NewResult("UPDATE", 1))
		if err := NewTicketStore(mock).AdminUpdateStatus(ctx, "t1", "closed"); err != nil {
			t.Fatalf("AdminUpdateStatus: %v", err)
		}
	})
	t.Run("error", func(t *testing.T) {
		mock := newMock(t)
		mock.ExpectExec("UPDATE support_tickets SET status").WillReturnError(errDB)
		if err := NewTicketStore(mock).AdminUpdateStatus(ctx, "t1", "closed"); err == nil {
			t.Fatal("expected error")
		}
	})
}
