package admin

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"

	mockcontracts "kronus.dev/commbadge_api/handler/contracts/mocks"
	"kronus.dev/commbadge_api/middleware"
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

func anyCtx() interface{} {
	return mock.Anything
}

type adminTestHandler struct {
	h       *AdminHandler
	admin   *mockcontracts.MockAdminRepository
	coms    *mockcontracts.MockCommunityRepository
	tickets *mockcontracts.MockTicketRepository
	users   *mockcontracts.MockUserRepository
}

func testAdminHandler(t *testing.T) *adminTestHandler {
	t.Helper()
	admin := mockcontracts.NewMockAdminRepository(t)
	coms := mockcontracts.NewMockCommunityRepository(t)
	tickets := mockcontracts.NewMockTicketRepository(t)
	users := mockcontracts.NewMockUserRepository(t)
	return &adminTestHandler{
		h: &AdminHandler{
			AdminStore:       admin,
			CommunityStore:   coms,
			TicketStore:      tickets,
			UserStore:        users,
			DegradedDetector: nil,
		},
		admin:   admin,
		coms:    coms,
		tickets: tickets,
		users:   users,
	}
}

type halterFunc func()

func (f halterFunc) Halt() { f() }