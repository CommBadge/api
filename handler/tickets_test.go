package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"kronus.dev/commbadge_api/internal/testutil"
)

func testTicketHandler(t *testing.T) *TicketHandler {
	t.Helper()
	return &TicketHandler{
		Tickets: testutil.NewMockTicketStore(),
	}
}

func TestTicketHandler_Create(t *testing.T) {
	h := testTicketHandler(t)
	body := `{"subject":"Help","body":"I need help with something"}`
	req := httptest.NewRequest("POST", "/tickets", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := authContext(req.Context(), "user-1")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	h.Create(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}
	var ticket struct {
		ID      string `json:"id"`
		Subject string `json:"subject"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &ticket); err != nil {
		t.Fatal(err)
	}
	if ticket.Subject != "Help" {
		t.Fatalf("expected Help, got %s", ticket.Subject)
	}
}

func TestTicketHandler_Create_Validation(t *testing.T) {
	h := testTicketHandler(t)
	tests := []struct {
		name string
		body string
		code int
	}{
		{"empty body", `{"subject":"","body":""}`, http.StatusBadRequest},
		{"missing subject", `{"subject":"","body":"text"}`, http.StatusBadRequest},
		{"missing body", `{"subject":"S","body":""}`, http.StatusBadRequest},
		{"body too long", `{"subject":"S","body":"` + string(make([]byte, 2001)) + `"}`, http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/tickets", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			ctx := authContext(req.Context(), "user-1")
			req = req.WithContext(ctx)
			rr := httptest.NewRecorder()
			h.Create(rr, req)
			if rr.Code != tt.code {
				t.Fatalf("expected %d, got %d: %s", tt.code, rr.Code, rr.Body.String())
			}
		})
	}
}

func TestTicketHandler_List(t *testing.T) {
	h := testTicketHandler(t)
	h.Tickets.Create(nil, "user-1", "Test", "Body")
	h.Tickets.Create(nil, "user-1", "Test2", "Body2")

	req := httptest.NewRequest("GET", "/tickets", nil)
	ctx := authContext(req.Context(), "user-1")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	h.List(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var tickets []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &tickets); err != nil {
		t.Fatal(err)
	}
	if len(tickets) != 2 {
		t.Fatalf("expected 2 tickets, got %d", len(tickets))
	}
}

func TestTicketHandler_Get_Owner(t *testing.T) {
	h := testTicketHandler(t)
	ticket, _ := h.Tickets.Create(nil, "user-1", "Test", "Body")
	req := httptest.NewRequest("GET", "/tickets/"+ticket.ID, nil)
	req.SetPathValue("ticketID", ticket.ID)
	ctx := authContext(req.Context(), "user-1")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	h.Get(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestTicketHandler_Get_Forbidden(t *testing.T) {
	h := testTicketHandler(t)
	ticket, _ := h.Tickets.Create(nil, "user-1", "Test", "Body")
	req := httptest.NewRequest("GET", "/tickets/"+ticket.ID, nil)
	req.SetPathValue("ticketID", ticket.ID)
	ctx := authContext(req.Context(), "user-2")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	h.Get(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
}
