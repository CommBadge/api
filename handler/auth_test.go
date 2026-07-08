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
	mockTwitch := testutil.NewMockTwitchClient()
	mockSessions := testutil.NewMockSessionStore()
	mockUsers := testutil.NewMockUserStore()

	mockUsers.UpsertUser(context.Background(), &store.User{
		ID:          "user-1",
		Login:       "testuser",
		DisplayName: "TestUser",
	})

	return &AuthHandler{
		Twitch:      mockTwitch,
		Sessions:    mockSessions,
		Users:       mockUsers,
		FrontendURL: "http://localhost:3000",
		RedirectURL: "http://localhost:8080/auth/callback",
		SessionTTL:  24 * time.Hour,
		IsTLS:       false,
		SignJWT:     func(sid string) (string, error) { return "signed-" + sid, nil },
		VerifyJWT:   func(token string) (string, error) { return token[7:], nil },
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
	ses := &session.Session{UserID: "user-1", TwitchToken: "tok"}
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
}
