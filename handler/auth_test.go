package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"kronus.dev/commbadge_api/internal/testutil"
	"kronus.dev/commbadge_api/session"
	"kronus.dev/commbadge_api/store"
)

func testAuthHandler(t *testing.T) *AuthHandler {
	t.Helper()
	mockDiscord := testutil.NewMockDiscordClient()
	mockTwitch := testutil.NewMockTwitchClient()
	mockSessions := testutil.NewMockSessionStore()
	mockUsers := testutil.NewMockUserStore()
	mockSubs := testutil.NewMockNotificationStore()

	mockUsers.UpsertUser(context.Background(), &store.User{
		ID:          "user-1",
		Username:    "testuser",
		DisplayName: "TestUser",
	})

	return &AuthHandler{
		Discord:       mockDiscord,
		Twitch:        mockTwitch,
		EventSub:      mockTwitch,
		Sessions:      mockSessions,
		Users:         mockUsers,
		Subscriptions: mockSubs,
		FrontendURL:   "http://localhost:3000",
		RedirectURL:   "http://localhost:8080/auth/callback",
		SessionTTL:    24 * time.Hour,
		IsTLS:         false,
		SignJWT:       func(sid string) (string, error) { return "signed-" + sid, nil },
		VerifyJWT:     func(token string) (string, error) { return token[7:], nil },
	}
}

func TestAuthHandler_Login(t *testing.T) {
	h := testAuthHandler(t)
	req := httptest.NewRequest("GET", "/auth/login", nil)
	rr := httptest.NewRecorder()
	h.Login(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", rr.Code)
	}
	loc := rr.Header().Get("Location")
	if loc == "" {
		t.Fatal("expected Location header")
	}
}

func TestAuthHandler_Login_AlreadyAuthenticated(t *testing.T) {
	h := testAuthHandler(t)
	ses := &session.Session{UserID: "user-1"}
	sid, _ := h.Sessions.Create(context.Background(), ses)
	req := httptest.NewRequest("GET", "/auth/login", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "signed-" + sid})
	rr := httptest.NewRecorder()
	h.Login(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", rr.Code)
	}
	if rr.Header().Get("Location") != h.FrontendURL {
		t.Fatalf("expected redirect to frontend, got %s", rr.Header().Get("Location"))
	}
}

func TestAuthHandler_Logout(t *testing.T) {
	h := testAuthHandler(t)
	ses := &session.Session{UserID: "user-1"}
	sid, _ := h.Sessions.Create(context.Background(), ses)
	req := httptest.NewRequest("POST", "/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "signed-" + sid})
	rr := httptest.NewRecorder()
	h.Logout(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["message"] != "logged out" {
		t.Fatalf("expected logged out, got %s", resp["message"])
	}
}

func TestAuthHandler_Logout_WithoutSession(t *testing.T) {
	h := testAuthHandler(t)
	req := httptest.NewRequest("POST", "/auth/logout", nil)
	rr := httptest.NewRecorder()
	h.Logout(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestAuthHandler_Callback_MissingParams(t *testing.T) {
	h := testAuthHandler(t)
	req := httptest.NewRequest("GET", "/auth/callback", nil)
	rr := httptest.NewRecorder()
	h.Callback(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestAuthHandler_Callback_InvalidState(t *testing.T) {
	h := testAuthHandler(t)
	req := httptest.NewRequest("GET", "/auth/callback?code=abc&state=bad", nil)
	rr := httptest.NewRecorder()
	h.Callback(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestAuthHandler_Callback_Success(t *testing.T) {
	h := testAuthHandler(t)
	_ = h.Sessions.SetState(context.Background(), "valid-state", 5*time.Minute)
	req := httptest.NewRequest("GET", "/auth/callback?code=abc&state=valid-state", nil)
	rr := httptest.NewRecorder()
	h.Callback(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", rr.Code)
	}
	cookies := rr.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "session" {
			sessionCookie = c
		}
	}
	if sessionCookie == nil || sessionCookie.Value == "" {
		t.Fatal("expected session cookie")
	}
}

func TestAuthHandler_TwitchLink(t *testing.T) {
	h := testAuthHandler(t)
	req := httptest.NewRequest("GET", "/auth/twitch/link", nil)
	rr := httptest.NewRecorder()
	h.TwitchLink(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", rr.Code)
	}
	if loc := rr.Header().Get("Location"); loc == "" {
		t.Fatal("expected Location header")
	}
}

func TestAuthHandler_TwitchCallback_Unauthorized(t *testing.T) {
	h := testAuthHandler(t)
	req := httptest.NewRequest("GET", "/auth/twitch/callback?code=abc&state=x", nil)
	rr := httptest.NewRecorder()
	h.TwitchCallback(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestAuthHandler_TwitchCallback_Success(t *testing.T) {
	h := testAuthHandler(t)
	_ = h.Sessions.SetState(context.Background(), "valid-state", 5*time.Minute)
	req := httptest.NewRequest("GET", "/auth/twitch/callback?code=abc&state=valid-state", nil)
	ctx := authContext(req.Context(), "user-1")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	h.TwitchCallback(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", rr.Code)
	}
	twitchID, err := h.Users.GetTwitchID(context.Background(), "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if twitchID != "twitch-user-1" {
		t.Fatalf("expected twitch-user-1, got %q", twitchID)
	}
}

func TestAuthHandler_TwitchUnlink_Unauthorized(t *testing.T) {
	h := testAuthHandler(t)
	req := httptest.NewRequest("POST", "/auth/twitch/unlink", nil)
	rr := httptest.NewRecorder()
	h.TwitchUnlink(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestAuthHandler_TwitchUnlink_Success(t *testing.T) {
	h := testAuthHandler(t)
	ctx := context.Background()
	_ = h.Users.SaveTwitchLink(ctx, "user-1", "twitch-user-1")
	_, _ = h.Subscriptions.Create(ctx, "user-1", "stream.online", map[string]string{"broadcaster_user_id": "twitch-user-1"}, "eventsub-1")

	req := httptest.NewRequest("POST", "/auth/twitch/unlink", nil)
	req = req.WithContext(authContext(req.Context(), "user-1"))
	rr := httptest.NewRecorder()
	h.TwitchUnlink(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	twitchID, _ := h.Users.GetTwitchID(ctx, "user-1")
	if twitchID != "" {
		t.Fatalf("expected empty twitch link, got %q", twitchID)
	}
	subs, _ := h.Subscriptions.ListActiveByUser(ctx, "user-1")
	if len(subs) != 0 {
		t.Fatalf("expected no active subscriptions, got %d", len(subs))
	}
	if _, ok := h.Subscriptions.(*testutil.MockNotificationStore); !ok {
		t.Fatal("unexpected subscriptions store")
	}
}

func TestAuthHandler_TwitchUnlink_NoSubs(t *testing.T) {
	h := testAuthHandler(t)
	ctx := context.Background()
	_ = h.Users.SaveTwitchLink(ctx, "user-1", "twitch-user-1")

	req := httptest.NewRequest("POST", "/auth/twitch/unlink", nil)
	req = req.WithContext(authContext(req.Context(), "user-1"))
	rr := httptest.NewRecorder()
	h.TwitchUnlink(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	twitchID, _ := h.Users.GetTwitchID(ctx, "user-1")
	if twitchID != "" {
		t.Fatalf("expected empty twitch link, got %q", twitchID)
	}
}
