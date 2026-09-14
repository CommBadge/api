package settings

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

type settingsTestHandler struct {
	h       *SettingsHandler
	users   *mockcontracts.MockUserRepository
	sessions *mockcontracts.MockSessionStore
	coms    *mockcontracts.MockCommunityRepository
	discord *mockcontracts.MockDiscordClient
}

func testSettingsHandler(t *testing.T) *settingsTestHandler {
	t.Helper()
	users := mockcontracts.NewMockUserRepository(t)
	sessions := mockcontracts.NewMockSessionStore(t)
	coms := mockcontracts.NewMockCommunityRepository(t)
	disc := mockcontracts.NewMockDiscordClient(t)
	return &settingsTestHandler{
		h: &SettingsHandler{
			Users:           users,
			Sessions:        sessions,
			Communities:     coms,
			Discord:         disc,
			DiscordBotToken: "mock-bot-token",
		},
		users:    users,
		sessions: sessions,
		coms:     coms,
		discord:  disc,
	}
}

const (
	guildID   = "200000000000000001"
	channelID = "300000000000000006"
)