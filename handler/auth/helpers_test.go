package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

type authTestHandler struct {
	h        *AuthHandler
	discord  *mockcontracts.MockDiscordClient
	twitch   *mockcontracts.MockTwitchClient
	eventSub *mockcontracts.MockTwitchEventSubClient
	sessions *mockcontracts.MockSessionStore
	users    *mockcontracts.MockUserRepository
	subs     *mockcontracts.MockSubscriptionRepository
}

func testAuthHandler(t *testing.T) *authTestHandler {
	t.Helper()
	discord := mockcontracts.NewMockDiscordClient(t)
	twitch := mockcontracts.NewMockTwitchClient(t)
	eventSub := mockcontracts.NewMockTwitchEventSubClient(t)
	sessions := mockcontracts.NewMockSessionStore(t)
	users := mockcontracts.NewMockUserRepository(t)
	subs := mockcontracts.NewMockSubscriptionRepository(t)
	return &authTestHandler{
		h: &AuthHandler{
			Discord:       discord,
			Twitch:        twitch,
			EventSub:      eventSub,
			Sessions:      sessions,
			Users:         users,
			Subscriptions: subs,
			FrontendURL:   "http://localhost:3000",
			RedirectURL:   "http://localhost:8080/auth/callback",
			SessionTTL:    24 * time.Hour,
			SecureCookies: false,
			SignJWT:       func(sid string) (string, error) { return "signed-" + sid, nil },
			VerifyJWT:     func(token string) (string, error) { return token[7:], nil },
		},
		discord:  discord,
		twitch:   twitch,
		eventSub: eventSub,
		sessions: sessions,
		users:    users,
		subs:     subs,
	}
}