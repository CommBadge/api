package admin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"kronus.dev/commbadge_api/store"
)

const comID = "00000000-0000-0000-0000-000000000001"

func TestAdminHandler_ListCommunities(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		fx := testAdminHandler(t)
		fx.coms.On("AdminListAll", anyCtx(), 50, 0).Return([]store.Community{}, nil).Once()
		rr := httptest.NewRecorder()
		fx.h.ListCommunities(rr, authReq("GET", "/x", "", "admin-1"))
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d: %s", rr.Code, rr.Body.String())
		}
		fx.coms.AssertCalled(t, "AdminListAll", anyCtx(), 50, 0)
	})

	t.Run("store error", func(t *testing.T) {
		fx := testAdminHandler(t)
		fx.coms.On("AdminListAll", anyCtx(), 50, 0).Return(nil, errH).Once()
		rr := httptest.NewRecorder()
		fx.h.ListCommunities(rr, authReq("GET", "/x", "", "admin-1"))
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d", rr.Code)
		}
	})
}

func TestAdminHandler_DisbandCommunity(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		fx := testAdminHandler(t)
		fx.coms.On("Delete", anyCtx(), comID).Return(nil).Once()
		req := authReq("DELETE", "/x", "", "admin-1")
		req.SetPathValue("communityID", comID)
		rr := httptest.NewRecorder()
		fx.h.DisbandCommunity(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d: %s", rr.Code, rr.Body.String())
		}
		fx.coms.AssertCalled(t, "Delete", anyCtx(), comID)
	})

	t.Run("invalid id", func(t *testing.T) {
		fx := testAdminHandler(t)
		req := authReq("DELETE", "/x", "", "admin-1")
		req.SetPathValue("communityID", "not-a-uuid")
		rr := httptest.NewRecorder()
		fx.h.DisbandCommunity(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d", rr.Code)
		}
	})

	t.Run("store error", func(t *testing.T) {
		fx := testAdminHandler(t)
		fx.coms.On("Delete", anyCtx(), comID).Return(errH).Once()
		req := authReq("DELETE", "/x", "", "admin-1")
		req.SetPathValue("communityID", comID)
		rr := httptest.NewRecorder()
		fx.h.DisbandCommunity(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d", rr.Code)
		}
	})
}

func TestAdminHandler_SetMaintenance(t *testing.T) {
	t.Run("enable calls halter", func(t *testing.T) {
		fx := testAdminHandler(t)
		halted := false
		fx.h.DegradedDetector = halterFunc(func() { halted = true })
		rr := httptest.NewRecorder()
		fx.h.SetMaintenance(rr, authReq("POST", "/x", `{"enabled":true}`, "admin-1"))
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d", rr.Code)
		}
		if !halted {
			t.Fatal("expected DegradedDetector.Halt to be called")
		}
	})

	t.Run("disable does not halt", func(t *testing.T) {
		fx := testAdminHandler(t)
		halted := false
		fx.h.DegradedDetector = halterFunc(func() { halted = true })
		rr := httptest.NewRecorder()
		fx.h.SetMaintenance(rr, authReq("POST", "/x", `{"enabled":false}`, "admin-1"))
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d", rr.Code)
		}
		if halted {
			t.Fatal("DegradedDetector.Halt must not be called on disable")
		}
	})

	t.Run("nil detector", func(t *testing.T) {
		fx := testAdminHandler(t)
		rr := httptest.NewRecorder()
		fx.h.SetMaintenance(rr, authReq("POST", "/x", `{"enabled":true}`, "admin-1"))
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d", rr.Code)
		}
	})

	t.Run("bad body", func(t *testing.T) {
		fx := testAdminHandler(t)
		rr := httptest.NewRecorder()
		fx.h.SetMaintenance(rr, authReq("POST", "/x", `{oops`, "admin-1"))
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d", rr.Code)
		}
	})
}

func TestAdminHandler_Tickets(t *testing.T) {
	t.Run("list", func(t *testing.T) {
		fx := testAdminHandler(t)
		fx.tickets.On("AdminListAll", anyCtx(), 50, 0).Return([]store.SupportTicket{}, nil).Once()
		rr := httptest.NewRecorder()
		fx.h.ListTickets(rr, authReq("GET", "/x", "", "admin-1"))
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d: %s", rr.Code, rr.Body.String())
		}

		fx2 := testAdminHandler(t)
		fx2.tickets.On("AdminListAll", anyCtx(), 50, 0).Return(nil, errH).Once()
		rr = httptest.NewRecorder()
		fx2.h.ListTickets(rr, authReq("GET", "/x", "", "admin-1"))
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d", rr.Code)
		}
	})

	t.Run("get", func(t *testing.T) {
		fx := testAdminHandler(t)
		fx.tickets.On("GetByID", anyCtx(), "ticket-1").Return(&store.SupportTicket{ID: "ticket-1", UserID: "user-1"}, nil).Once()
		fx.tickets.On("GetMessages", anyCtx(), "ticket-1").Return([]store.TicketMessage{{ID: "msg-1", TicketID: "ticket-1", Body: "B"}}, nil).Once()
		req := authReq("GET", "/x", "", "admin-1")
		req.SetPathValue("ticketID", "ticket-1")
		rr := httptest.NewRecorder()
		fx.h.GetTicket(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d", rr.Code)
		}
		fx.tickets.AssertCalled(t, "GetByID", anyCtx(), "ticket-1")
		fx.tickets.AssertCalled(t, "GetMessages", anyCtx(), "ticket-1")

		fx2 := testAdminHandler(t)
		fx2.tickets.On("GetByID", anyCtx(), "absent").Return(nil, nil).Once()
		req = authReq("GET", "/x", "", "admin-1")
		req.SetPathValue("ticketID", "absent")
		rr = httptest.NewRecorder()
		fx2.h.GetTicket(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d", rr.Code)
		}

		fx3 := testAdminHandler(t)
		fx3.tickets.On("GetByID", anyCtx(), "ticket-1").Return(nil, errH).Once()
		req = authReq("GET", "/x", "", "admin-1")
		req.SetPathValue("ticketID", "ticket-1")
		rr = httptest.NewRecorder()
		fx3.h.GetTicket(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d", rr.Code)
		}
	})

	t.Run("reply", func(t *testing.T) {
		fx := testAdminHandler(t)
		fx.tickets.On("GetByID", anyCtx(), "ticket-1").Return(&store.SupportTicket{ID: "ticket-1", UserID: "user-1"}, nil).Once()
		fx.tickets.On("AddMessage", anyCtx(), "ticket-1", "admin-1", "handled", true).Return(&store.TicketMessage{ID: "msg-1", TicketID: "ticket-1", Body: "handled", IsAdmin: true}, nil).Once()
		req := authReq("POST", "/x", `{"body":"handled"}`, "admin-1")
		req.SetPathValue("ticketID", "ticket-1")
		rr := httptest.NewRecorder()
		fx.h.ReplyTicket(rr, req)
		if rr.Code != http.StatusCreated {
			t.Fatalf("status = %d: %s", rr.Code, rr.Body.String())
		}

		fx.tickets.On("GetByID", anyCtx(), "ticket-1").Return(&store.SupportTicket{ID: "ticket-1", UserID: "user-1"}, nil).Once()
		req = authReq("POST", "/x", `{}`, "admin-1")
		req.SetPathValue("ticketID", "ticket-1")
		rr = httptest.NewRecorder()
		fx.h.ReplyTicket(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d", rr.Code)
		}

		fx.tickets.On("GetByID", anyCtx(), "absent").Return(nil, nil).Once()
		req = authReq("POST", "/x", `{"body":"handled"}`, "admin-1")
		req.SetPathValue("ticketID", "absent")
		rr = httptest.NewRecorder()
		fx.h.ReplyTicket(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d", rr.Code)
		}
	})

	t.Run("count", func(t *testing.T) {
		fx := testAdminHandler(t)
		fx.tickets.On("AdminTicketCount", anyCtx()).Return(2, nil).Once()
		rr := httptest.NewRecorder()
		fx.h.TicketCount(rr, authReq("GET", "/x", "", "admin-1"))
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d", rr.Code)
		}
		if !strings.Contains(rr.Body.String(), `"count":2`) {
			t.Fatalf("body = %s", rr.Body.String())
		}

		fx2 := testAdminHandler(t)
		fx2.tickets.On("AdminTicketCount", anyCtx()).Return(0, errH).Once()
		rr = httptest.NewRecorder()
		fx2.h.TicketCount(rr, authReq("GET", "/x", "", "admin-1"))
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d", rr.Code)
		}
	})

	t.Run("update status", func(t *testing.T) {
		fx := testAdminHandler(t)
		fx.tickets.On("GetByID", anyCtx(), "ticket-1").Return(&store.SupportTicket{ID: "ticket-1", UserID: "user-1"}, nil).Once()
		fx.tickets.On("AdminUpdateStatus", anyCtx(), "ticket-1", "closed").Return(nil).Once()
		req := authReq("PATCH", "/x", `{"status":"closed"}`, "admin-1")
		req.SetPathValue("ticketID", "ticket-1")
		rr := httptest.NewRecorder()
		fx.h.UpdateTicketStatus(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d: %s", rr.Code, rr.Body.String())
		}

		fx.tickets.On("GetByID", anyCtx(), "ticket-1").Return(&store.SupportTicket{ID: "ticket-1", UserID: "user-1"}, nil).Once()
		req = authReq("PATCH", "/x", `{}`, "admin-1")
		req.SetPathValue("ticketID", "ticket-1")
		rr = httptest.NewRecorder()
		fx.h.UpdateTicketStatus(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d", rr.Code)
		}

		fx.tickets.On("GetByID", anyCtx(), "absent").Return(nil, nil).Once()
		req = authReq("PATCH", "/x", `{"status":"closed"}`, "admin-1")
		req.SetPathValue("ticketID", "absent")
		rr = httptest.NewRecorder()
		fx.h.UpdateTicketStatus(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d", rr.Code)
		}

		fx2 := testAdminHandler(t)
		fx2.tickets.On("GetByID", anyCtx(), "ticket-1").Return(&store.SupportTicket{ID: "ticket-1", UserID: "user-1"}, nil).Once()
		fx2.tickets.On("AdminUpdateStatus", anyCtx(), "ticket-1", "closed").Return(errH).Once()
		req = authReq("PATCH", "/x", `{"status":"closed"}`, "admin-1")
		req.SetPathValue("ticketID", "ticket-1")
		rr = httptest.NewRecorder()
		fx2.h.UpdateTicketStatus(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d", rr.Code)
		}
	})
}

func TestAdminHandler_Users(t *testing.T) {
	t.Run("list", func(t *testing.T) {
		fx := testAdminHandler(t)
		fx.users.On("ListUsers", anyCtx(), 10, 0).Return([]store.UserAdminView{{User: store.User{ID: "u1", Username: "alice"}}}, nil).Once()
		rr := httptest.NewRecorder()
		fx.h.ListUsers(rr, authReq("GET", "/x?limit=10&offset=0", "", "admin-1"))
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d", rr.Code)
		}
	})

	t.Run("list invalid pagination falls back", func(t *testing.T) {
		fx := testAdminHandler(t)
		fx.users.On("ListUsers", anyCtx(), 50, 0).Return([]store.UserAdminView{}, nil).Once()
		rr := httptest.NewRecorder()
		fx.h.ListUsers(rr, authReq("GET", "/x?limit=9999&offset=-1", "", "admin-1"))
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d", rr.Code)
		}
		fx.users.AssertCalled(t, "ListUsers", anyCtx(), 50, 0)
	})

	t.Run("list store error", func(t *testing.T) {
		fx := testAdminHandler(t)
		fx.users.On("ListUsers", anyCtx(), 50, 0).Return(nil, errH).Once()
		rr := httptest.NewRecorder()
		fx.h.ListUsers(rr, authReq("GET", "/x", "", "admin-1"))
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d", rr.Code)
		}
	})

	t.Run("search", func(t *testing.T) {
		fx := testAdminHandler(t)
		fx.users.On("SearchUsers", anyCtx(), "alice", 50).Return([]store.UserAdminView{{User: store.User{ID: "u1", Username: "alice"}}}, nil).Once()
		rr := httptest.NewRecorder()
		fx.h.SearchUsers(rr, authReq("GET", "/x?q=alice", "", "admin-1"))
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d: %s", rr.Code, rr.Body.String())
		}

		rr = httptest.NewRecorder()
		fx.h.SearchUsers(rr, authReq("GET", "/x", "", "admin-1"))
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d", rr.Code)
		}

		fx2 := testAdminHandler(t)
		fx2.users.On("SearchUsers", anyCtx(), "alice", 50).Return(nil, errH).Once()
		rr = httptest.NewRecorder()
		fx2.h.SearchUsers(rr, authReq("GET", "/x?q=alice", "", "admin-1"))
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d", rr.Code)
		}
	})

	t.Run("warnings", func(t *testing.T) {
		fx := testAdminHandler(t)
		fx.users.On("GetWarningLog", anyCtx(), "u1").Return([]store.UserWarning{{UserID: "u1", Reason: "spam"}}, nil).Once()
		req := authReq("GET", "/x", "", "admin-1")
		req.SetPathValue("userID", "u1")
		rr := httptest.NewRecorder()
		fx.h.GetUserWarnings(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d", rr.Code)
		}

		fx.users.On("AddWarning", anyCtx(), "u1", "admin-1", "spam").Return(nil).Once()
		req = authReq("POST", "/x", `{"reason":"spam"}`, "admin-1")
		req.SetPathValue("userID", "u1")
		rr = httptest.NewRecorder()
		fx.h.WarnUser(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d", rr.Code)
		}

		req = authReq("POST", "/x", `{"reason":""}`, "admin-1")
		req.SetPathValue("userID", "u1")
		rr = httptest.NewRecorder()
		fx.h.WarnUser(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d", rr.Code)
		}
	})

	t.Run("warnings store error", func(t *testing.T) {
		fx := testAdminHandler(t)
		fx.users.On("GetWarningLog", anyCtx(), "u1").Return(nil, errH).Once()
		req := authReq("GET", "/x", "", "admin-1")
		req.SetPathValue("userID", "u1")
		rr := httptest.NewRecorder()
		fx.h.GetUserWarnings(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d", rr.Code)
		}

		fx.users.On("AddWarning", anyCtx(), "u1", "admin-1", "spam").Return(errH).Once()
		req = authReq("POST", "/x", `{"reason":"spam"}`, "admin-1")
		req.SetPathValue("userID", "u1")
		rr = httptest.NewRecorder()
		fx.h.WarnUser(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d", rr.Code)
		}
	})

	t.Run("ban unban", func(t *testing.T) {
		fx := testAdminHandler(t)
		fx.users.On("BanUser", anyCtx(), "u1", "spam").Return(nil).Once()
		req := authReq("POST", "/x", `{"reason":"spam"}`, "admin-1")
		req.SetPathValue("userID", "u1")
		rr := httptest.NewRecorder()
		fx.h.BanUser(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d", rr.Code)
		}

		req = authReq("POST", "/x", `{"reason":""}`, "admin-1")
		req.SetPathValue("userID", "u1")
		rr = httptest.NewRecorder()
		fx.h.BanUser(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d", rr.Code)
		}

		fx.users.On("UnbanUser", anyCtx(), "u1").Return(nil).Once()
		req = authReq("POST", "/x", "", "admin-1")
		req.SetPathValue("userID", "u1")
		rr = httptest.NewRecorder()
		fx.h.UnbanUser(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d", rr.Code)
		}

		fx2 := testAdminHandler(t)
		fx2.users.On("BanUser", anyCtx(), "u1", "spam").Return(errH).Once()
		req = authReq("POST", "/x", `{"reason":"spam"}`, "admin-1")
		req.SetPathValue("userID", "u1")
		rr = httptest.NewRecorder()
		fx2.h.BanUser(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d", rr.Code)
		}

		fx2.users.On("UnbanUser", anyCtx(), "u1").Return(errH).Once()
		req = authReq("POST", "/x", "", "admin-1")
		req.SetPathValue("userID", "u1")
		rr = httptest.NewRecorder()
		fx2.h.UnbanUser(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d", rr.Code)
		}
	})
}

func TestAdminHandler_ReplyTicket_MessageError(t *testing.T) {
	fx := testAdminHandler(t)
	fx.tickets.On("GetByID", anyCtx(), "ticket-1").Return(&store.SupportTicket{ID: "ticket-1", UserID: "user-1"}, nil).Once()
	fx.tickets.On("AddMessage", anyCtx(), "ticket-1", "admin-1", "handled", true).Return(nil, errH).Once()
	req := authReq("POST", "/x", `{"body":"handled"}`, "admin-1")
	req.SetPathValue("ticketID", "ticket-1")
	rr := httptest.NewRecorder()
	fx.h.ReplyTicket(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rr.Code)
	}
}