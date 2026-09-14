package tickets

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	mockcontracts "kronus.dev/commbadge_api/handler/contracts/mocks"
	"kronus.dev/commbadge_api/store"
)

func TestTicketHandler_Create(t *testing.T) {
	h := testTicketHandler(t)
	ticket := newMockTicket("ticket-1", "user-1", "Hello", "I need help")
	h.Tickets.(*mockcontracts.MockTicketRepository).
		On("Create", mock.Anything, "user-1", "Hello", "I need help").
		Return(ticket, nil).Once()

	body := `{"subject":"Hello","body":"I need help"}`
	rr := httptest.NewRecorder()
	h.Create(rr, authReq("POST", "/x", body, "user-1"))

	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	require.Equal(t, "Hello", resp["subject"])
	require.Equal(t, "I need help", resp["body"])
}

func TestTicketHandler_Create_Validation(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"empty", `{}`},
		{"missing body", `{"subject":"Hello"}`},
		{"missing subject", `{"body":"I need help"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			h := testTicketHandler(t)
			h.Create(rr, authReq("POST", "/x", tt.body, "user-1"))
			require.Equal(t, http.StatusBadRequest, rr.Code)
		})
	}

	t.Run("body too long", func(t *testing.T) {
		long := strings.Repeat("a", 2001)
		rr := httptest.NewRecorder()
		h := testTicketHandler(t)
		h.Create(rr, authReq("POST", "/x", `{"subject":"S","body":"`+long+`"}`, "user-1"))
		require.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("invalid json", func(t *testing.T) {
		rr := httptest.NewRecorder()
		h := testTicketHandler(t)
		h.Create(rr, authReq("POST", "/x", `{oops`, "user-1"))
		require.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("store error", func(t *testing.T) {
		h := testTicketHandler(t)
		h.Tickets.(*mockcontracts.MockTicketRepository).
			On("Create", mock.Anything, "user-1", "Hello", "store error").
			Return(nil, errH).Once()
		rr := httptest.NewRecorder()
		h.Create(rr, authReq("POST", "/x", `{"subject":"Hello","body":"store error"}`, "user-1"))
		require.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

func TestTicketHandler_List(t *testing.T) {
	h := testTicketHandler(t)
	tickets := []store.SupportTicket{
		{ID: "ticket-1", UserID: "user-1", Subject: "Hello", Body: "I need help", Status: "open"},
	}
	h.Tickets.(*mockcontracts.MockTicketRepository).
		On("ListByUser", mock.Anything, "user-1").
		Return(tickets, nil).Once()

	rr := httptest.NewRecorder()
	h.List(rr, authReq("GET", "/x", "", "user-1"))
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var resp []map[string]interface{}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	require.Len(t, resp, 1)
	require.Equal(t, "ticket-1", resp[0]["id"])

	t.Run("store error", func(t *testing.T) {
		h := testTicketHandler(t)
		h.Tickets.(*mockcontracts.MockTicketRepository).
			On("ListByUser", mock.Anything, "user-1").
			Return(nil, errH).Once()
		rr := httptest.NewRecorder()
		h.List(rr, authReq("GET", "/x", "", "user-1"))
		require.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

func TestTicketHandler_Get(t *testing.T) {
	ticket := newMockTicket("ticket-1", "user-1", "Hello", "I need help")
	var message *store.TicketMessage

	t.Run("owner", func(t *testing.T) {
		h := testTicketHandler(t)
		message = &store.TicketMessage{ID: "msg-1", TicketID: "ticket-1", Body: "detail", IsAdmin: false}
		h.Tickets.(*mockcontracts.MockTicketRepository).
			On("GetByID", mock.Anything, "ticket-1").
			Return(ticket, nil).Once()
		h.Tickets.(*mockcontracts.MockTicketRepository).
			On("GetMessages", mock.Anything, "ticket-1").
			Return([]store.TicketMessage{*message}, nil).Once()

		req := authReq("GET", "/x", "", "user-1")
		req.SetPathValue("ticketID", "ticket-1")
		rr := httptest.NewRecorder()
		h.Get(rr, req)
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	})

	t.Run("not owner", func(t *testing.T) {
		h := testTicketHandler(t)
		h.Tickets.(*mockcontracts.MockTicketRepository).
			On("GetByID", mock.Anything, "ticket-1").
			Return(ticket, nil).Once()

		req := authReq("GET", "/x", "", "user-2")
		req.SetPathValue("ticketID", "ticket-1")
		rr := httptest.NewRecorder()
		h.Get(rr, req)
		require.Equal(t, http.StatusForbidden, rr.Code)
	})

	t.Run("not found", func(t *testing.T) {
		h := testTicketHandler(t)
		h.Tickets.(*mockcontracts.MockTicketRepository).
			On("GetByID", mock.Anything, "absent").
			Return(nil, nil).Once()

		req := authReq("GET", "/x", "", "user-1")
		req.SetPathValue("ticketID", "absent")
		rr := httptest.NewRecorder()
		h.Get(rr, req)
		require.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("get by id error", func(t *testing.T) {
		h := testTicketHandler(t)
		h.Tickets.(*mockcontracts.MockTicketRepository).
			On("GetByID", mock.Anything, "ticket-1").
			Return(nil, errH).Once()

		req := authReq("GET", "/x", "", "user-1")
		req.SetPathValue("ticketID", "ticket-1")
		rr := httptest.NewRecorder()
		h.Get(rr, req)
		require.Equal(t, http.StatusInternalServerError, rr.Code)
	})

	t.Run("messages error", func(t *testing.T) {
		h := testTicketHandler(t)
		h.Tickets.(*mockcontracts.MockTicketRepository).
			On("GetByID", mock.Anything, "ticket-1").
			Return(ticket, nil).Once()
		h.Tickets.(*mockcontracts.MockTicketRepository).
			On("GetMessages", mock.Anything, "ticket-1").
			Return(nil, errH).Once()

		req := authReq("GET", "/x", "", "user-1")
		req.SetPathValue("ticketID", "ticket-1")
		rr := httptest.NewRecorder()
		h.Get(rr, req)
		require.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

func TestTicketHandler_AddMessage(t *testing.T) {
	ticket := newMockTicket("ticket-1", "user-1", "Hello", "I need help")
	msg := &store.TicketMessage{
		ID:        "msg-1",
		TicketID:  "ticket-1",
		UserID:    "user-1",
		Body:      "more detail",
		IsAdmin:   false,
	}

	t.Run("success", func(t *testing.T) {
		h := testTicketHandler(t)
		h.Tickets.(*mockcontracts.MockTicketRepository).
			On("GetByID", mock.Anything, "ticket-1").
			Return(ticket, nil).Once()
		h.Tickets.(*mockcontracts.MockTicketRepository).
			On("AddMessage", mock.Anything, "ticket-1", "user-1", "more detail", false).
			Return(msg, nil).Once()

		req := authReq("POST", "/x", `{"body":"more detail"}`, "user-1")
		req.SetPathValue("ticketID", "ticket-1")
		rr := httptest.NewRecorder()
		h.AddMessage(rr, req)
		require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	})

	t.Run("not owner", func(t *testing.T) {
		h := testTicketHandler(t)
		h.Tickets.(*mockcontracts.MockTicketRepository).
			On("GetByID", mock.Anything, "ticket-1").
			Return(ticket, nil).Once()

		req := authReq("POST", "/x", `{"body":"more detail"}`, "user-2")
		req.SetPathValue("ticketID", "ticket-1")
		rr := httptest.NewRecorder()
		h.AddMessage(rr, req)
		require.Equal(t, http.StatusForbidden, rr.Code)
	})

	t.Run("ticket not found", func(t *testing.T) {
		h := testTicketHandler(t)
		h.Tickets.(*mockcontracts.MockTicketRepository).
			On("GetByID", mock.Anything, "absent").
			Return(nil, nil).Once()

		req := authReq("POST", "/x", `{"body":"more detail"}`, "user-1")
		req.SetPathValue("ticketID", "absent")
		rr := httptest.NewRecorder()
		h.AddMessage(rr, req)
		require.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("empty body", func(t *testing.T) {
		h := testTicketHandler(t)
		h.Tickets.(*mockcontracts.MockTicketRepository).
			On("GetByID", mock.Anything, "ticket-1").
			Return(ticket, nil).Once()

		req := authReq("POST", "/x", `{}`, "user-1")
		req.SetPathValue("ticketID", "ticket-1")
		rr := httptest.NewRecorder()
		h.AddMessage(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("body too long", func(t *testing.T) {
		h := testTicketHandler(t)
		h.Tickets.(*mockcontracts.MockTicketRepository).
			On("GetByID", mock.Anything, "ticket-1").
			Return(ticket, nil).Once()

		long := strings.Repeat("a", 2001)
		req := authReq("POST", "/x", `{"body":"`+long+`"}`, "user-1")
		req.SetPathValue("ticketID", "ticket-1")
		rr := httptest.NewRecorder()
		h.AddMessage(rr, req)
		require.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("get ticket error", func(t *testing.T) {
		h := testTicketHandler(t)
		h.Tickets.(*mockcontracts.MockTicketRepository).
			On("GetByID", mock.Anything, "ticket-1").
			Return(nil, errH).Once()

		req := authReq("POST", "/x", `{"body":"more detail"}`, "user-1")
		req.SetPathValue("ticketID", "ticket-1")
		rr := httptest.NewRecorder()
		h.AddMessage(rr, req)
		require.Equal(t, http.StatusInternalServerError, rr.Code)
	})

	t.Run("add message error", func(t *testing.T) {
		h := testTicketHandler(t)
		h.Tickets.(*mockcontracts.MockTicketRepository).
			On("GetByID", mock.Anything, "ticket-1").
			Return(ticket, nil).Once()
		h.Tickets.(*mockcontracts.MockTicketRepository).
			On("AddMessage", mock.Anything, "ticket-1", "user-1", "more detail", false).
			Return(nil, errH).Once()

		req := authReq("POST", "/x", `{"body":"more detail"}`, "user-1")
		req.SetPathValue("ticketID", "ticket-1")
		rr := httptest.NewRecorder()
		h.AddMessage(rr, req)
		require.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}
