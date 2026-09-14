package tickets

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mockcontracts "kronus.dev/commbadge_api/handler/contracts/mocks"
	"kronus.dev/commbadge_api/middleware"
	"kronus.dev/commbadge_api/store"
)

var errH = errors.New("handler boom")

func authContext(ctx context.Context, userID string) context.Context {
	ctx = context.WithValue(ctx, middleware.ContextUserID, userID)
	ctx = context.WithValue(ctx, middleware.ContextSession, "test-session")
	return ctx
}

func authReq(method, path, body, userID string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req.WithContext(authContext(req.Context(), userID))
}

func decodeJSONBody(rr *httptest.ResponseRecorder, v interface{}) error {
	return json.Unmarshal(rr.Body.Bytes(), v)
}

func testTicketHandler(t *testing.T) *TicketHandler {
	t.Helper()
	return &TicketHandler{Tickets: mockcontracts.NewMockTicketRepository(t)}
}

// newMockTicket returns a ticket record with a fixed ID.
func newMockTicket(id, userID, subject, body string) *store.SupportTicket {
	return &store.SupportTicket{
		ID:     id,
		UserID: userID,
		Subject: subject,
		Body:   body,
		Status: "open",
	}
}
