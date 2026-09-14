package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"

	"kronus.dev/commbadge_api/discord"
	"kronus.dev/commbadge_api/store"
	"kronus.dev/commbadge_api/twitch"
)

func stateReq(method, path, state, code string, withCookies bool) *http.Request {
	req := httptest.NewRequest(method, path+"?code="+code+"&state="+state, nil)
	if withCookies {
		req.AddCookie(&http.Cookie{Name: stateCookieName, Value: state})
		req.AddCookie(&http.Cookie{Name: verifierCookieName, Value: "verifier"})
	}
	return req
}

func TestAuthHandler_Login(t *testing.T) {
	fx := testAuthHandler(t)
	fx.sessions.On("SetState", anyCtx(), mock.Anything, "", 5*time.Minute).Return(nil).Once()
	var capturedState, capturedChallenge string
	fx.discord.On("AuthURL", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		capturedState = args.String(0)
		capturedChallenge = args.String(1)
	}).Return("https://discord.com/oauth2/authorize?mock=1").Once()
	req := httptest.NewRequest("GET", "/auth/login", nil)
	rr := httptest.NewRecorder()
	fx.h.Login(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", rr.Code)
	}
	if loc := rr.Header().Get("Location"); loc == "" {
		t.Fatal("expected Location header")
	}

	cookies := rr.Result().Cookies()
	var verifier string
	for _, c := range cookies {
		if c.Name == verifierCookieName {
			verifier = c.Value
		}
	}
	if verifier == "" {
		t.Fatal("expected pkce verifier cookie")
	}
	if capturedChallenge != pkceS256Challenge(verifier) {
		t.Fatalf("expected S256 challenge, got %q", capturedChallenge)
	}
	if capturedState == "" {
		t.Fatal("expected state passed to discord")
	}
}

func TestAuthHandler_Login_AlreadyAuthenticated(t *testing.T) {
	fx := testAuthHandler(t)
	req := httptest.NewRequest("GET", "/auth/login", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "signed-session-1"})
	rr := httptest.NewRecorder()
	fx.h.Login(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", rr.Code)
	}
	if rr.Header().Get("Location") != "http://localhost:3000" {
		t.Fatalf("expected redirect to frontend, got %s", rr.Header().Get("Location"))
	}
}

func TestAuthHandler_Logout(t *testing.T) {
	fx := testAuthHandler(t)
	fx.sessions.On("Delete", anyCtx(), "session-1").Return(nil).Once()
	req := httptest.NewRequest("POST", "/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "signed-session-1"})
	rr := httptest.NewRecorder()
	fx.h.Logout(rr, req)

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
	fx := testAuthHandler(t)
	req := httptest.NewRequest("POST", "/auth/logout", nil)
	rr := httptest.NewRecorder()
	fx.h.Logout(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestAuthHandler_Login_StateError(t *testing.T) {
	fx := testAuthHandler(t)
	fx.sessions.On("SetState", anyCtx(), mock.Anything, "", 5*time.Minute).Return(errH).Once()
	req := httptest.NewRequest("GET", "/auth/login", nil)
	rr := httptest.NewRecorder()
	fx.h.Login(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestAuthHandler_Logout_InvalidJWT(t *testing.T) {
	fx := testAuthHandler(t)
	fx.h.VerifyJWT = func(string) (string, error) { return "", errH }
	req := httptest.NewRequest("POST", "/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "not-signed"})
	rr := httptest.NewRecorder()
	fx.h.Logout(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	fx.sessions.AssertNotCalled(t, "Delete", anyCtx(), anyCtx())
}

func TestAuthHandler_Callback_MissingParams(t *testing.T) {
	fx := testAuthHandler(t)
	req := httptest.NewRequest("GET", "/auth/callback", nil)
	rr := httptest.NewRecorder()
	fx.h.Callback(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestAuthHandler_Callback_InvalidState(t *testing.T) {
	fx := testAuthHandler(t)
	req := httptest.NewRequest("GET", "/auth/callback?code=abc&state=bad", nil)
	rr := httptest.NewRecorder()
	fx.h.Callback(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestAuthHandler_Callback_Success(t *testing.T) {
	fx := testAuthHandler(t)
	fx.sessions.On("VerifyState", anyCtx(), "valid-state").Return("", true, nil).Once()
	fx.discord.On("Exchange", anyCtx(), "abc", "test-verifier").Return(&discord.TokenResponse{AccessToken: "mock-access"}, nil).Once()
	fx.discord.On("GetUser", anyCtx(), "mock-access").Return(&discord.User{ID: "user-1", Username: "testuser", GlobalName: "TestUser", Email: "test@example.com", Avatar: "avatarhash"}, nil).Once()
	fx.users.On("UpsertUser", anyCtx(), mock.Anything).Return(nil).Once()
	fx.sessions.On("Create", anyCtx(), mock.Anything).Return("session-1", nil).Once()
	req := httptest.NewRequest("GET", "/auth/callback?code=abc&state=valid-state", nil)
	req.AddCookie(&http.Cookie{Name: stateCookieName, Value: "valid-state"})
	req.AddCookie(&http.Cookie{Name: verifierCookieName, Value: "test-verifier"})
	rr := httptest.NewRecorder()
	fx.h.Callback(rr, req)

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
	fx.discord.AssertCalled(t, "Exchange", anyCtx(), "abc", "test-verifier")
	fx.discord.AssertCalled(t, "GetUser", anyCtx(), "mock-access")
	fx.users.AssertCalled(t, "UpsertUser", anyCtx(), mock.Anything)
	fx.sessions.AssertCalled(t, "Create", anyCtx(), mock.Anything)
}

func TestAuthHandler_Callback_MissingVerifier(t *testing.T) {
	fx := testAuthHandler(t)
	fx.sessions.On("VerifyState", anyCtx(), "valid-state").Return("", true, nil).Once()
	req := httptest.NewRequest("GET", "/auth/callback?code=abc&state=valid-state", nil)
	req.AddCookie(&http.Cookie{Name: stateCookieName, Value: "valid-state"})
	rr := httptest.NewRecorder()
	fx.h.Callback(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 without verifier cookie, got %d", rr.Code)
	}
}

func TestAuthHandler_Callback_StateCookieMismatch(t *testing.T) {
	fx := testAuthHandler(t)
	req := httptest.NewRequest("GET", "/auth/callback?code=abc&state=valid-state", nil)
	req.AddCookie(&http.Cookie{Name: stateCookieName, Value: "attacker-state"})
	rr := httptest.NewRecorder()
	fx.h.Callback(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
	fx.sessions.AssertNotCalled(t, "VerifyState", anyCtx(), anyCtx())
}

func TestAuthHandler_Callback_StateReplayRejected(t *testing.T) {
	fx := testAuthHandler(t)
	fx.sessions.On("VerifyState", anyCtx(), "valid-state").Return("", true, nil).Once()
	fx.sessions.On("VerifyState", anyCtx(), "valid-state").Return("", false, nil).Once()
	fx.discord.On("Exchange", anyCtx(), "abc", "test-verifier").Return(&discord.TokenResponse{AccessToken: "mock-access"}, nil).Once()
	fx.discord.On("GetUser", anyCtx(), "mock-access").Return(&discord.User{ID: "user-1", Username: "testuser"}, nil).Once()
	fx.users.On("UpsertUser", anyCtx(), mock.Anything).Return(nil).Once()
	fx.sessions.On("Create", anyCtx(), mock.Anything).Return("session-1", nil).Once()
	newReq := func() *http.Request {
		req := httptest.NewRequest("GET", "/auth/callback?code=abc&state=valid-state", nil)
		req.AddCookie(&http.Cookie{Name: stateCookieName, Value: "valid-state"})
		req.AddCookie(&http.Cookie{Name: verifierCookieName, Value: "test-verifier"})
		return req
	}

	rr := httptest.NewRecorder()
	fx.h.Callback(rr, newReq())
	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302 on first use, got %d", rr.Code)
	}

	rr = httptest.NewRecorder()
	fx.h.Callback(rr, newReq())
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on state replay, got %d", rr.Code)
	}
}

func TestAuthHandler_Callback_Branches(t *testing.T) {
	t.Run("verify state error", func(t *testing.T) {
		fx := testAuthHandler(t)
		fx.sessions.On("VerifyState", anyCtx(), "st").Return("", false, errH).Once()
		req := stateReq("GET", "/cb", "st", "abc", true)
		rr := httptest.NewRecorder()
		fx.h.Callback(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d", rr.Code)
		}
	})
	t.Run("state bound to user", func(t *testing.T) {
		fx := testAuthHandler(t)
		fx.sessions.On("VerifyState", anyCtx(), "st").Return("some-user", true, nil).Once()
		req := stateReq("GET", "/cb", "st", "abc", true)
		rr := httptest.NewRecorder()
		fx.h.Callback(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d", rr.Code)
		}
	})
	t.Run("exchange error", func(t *testing.T) {
		fx := testAuthHandler(t)
		fx.sessions.On("VerifyState", anyCtx(), "st").Return("", true, nil).Once()
		fx.discord.On("Exchange", anyCtx(), "abc", "verifier").Return(nil, errH).Once()
		req := stateReq("GET", "/cb", "st", "abc", true)
		rr := httptest.NewRecorder()
		fx.h.Callback(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d", rr.Code)
		}
	})
	t.Run("get user error", func(t *testing.T) {
		fx := testAuthHandler(t)
		fx.sessions.On("VerifyState", anyCtx(), "st").Return("", true, nil).Once()
		fx.discord.On("Exchange", anyCtx(), "abc", "verifier").Return(&discord.TokenResponse{AccessToken: "mock-access"}, nil).Once()
		fx.discord.On("GetUser", anyCtx(), "mock-access").Return(nil, errH).Once()
		req := stateReq("GET", "/cb", "st", "abc", true)
		rr := httptest.NewRecorder()
		fx.h.Callback(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d", rr.Code)
		}
	})
	t.Run("upsert user error", func(t *testing.T) {
		fx := testAuthHandler(t)
		fx.sessions.On("VerifyState", anyCtx(), "st").Return("", true, nil).Once()
		fx.discord.On("Exchange", anyCtx(), "abc", "verifier").Return(&discord.TokenResponse{AccessToken: "mock-access"}, nil).Once()
		fx.discord.On("GetUser", anyCtx(), "mock-access").Return(&discord.User{ID: "user-1", Username: "testuser"}, nil).Once()
		fx.users.On("UpsertUser", anyCtx(), mock.Anything).Return(errH).Once()
		req := stateReq("GET", "/cb", "st", "abc", true)
		rr := httptest.NewRecorder()
		fx.h.Callback(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d", rr.Code)
		}
	})
	t.Run("sign jwt error", func(t *testing.T) {
		fx := testAuthHandler(t)
		fx.h.SignJWT = func(string) (string, error) { return "", errH }
		fx.sessions.On("VerifyState", anyCtx(), "st").Return("", true, nil).Once()
		fx.discord.On("Exchange", anyCtx(), "abc", "verifier").Return(&discord.TokenResponse{AccessToken: "mock-access"}, nil).Once()
		fx.discord.On("GetUser", anyCtx(), "mock-access").Return(&discord.User{ID: "user-1", Username: "testuser"}, nil).Once()
		fx.users.On("UpsertUser", anyCtx(), mock.Anything).Return(nil).Once()
		fx.sessions.On("Create", anyCtx(), mock.Anything).Return("session-1", nil).Once()
		req := stateReq("GET", "/cb", "st", "abc", true)
		rr := httptest.NewRecorder()
		fx.h.Callback(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d", rr.Code)
		}
	})
}

func TestAuthHandler_TwitchLink(t *testing.T) {
	fx := testAuthHandler(t)
	fx.sessions.On("SetState", anyCtx(), mock.Anything, "", 5*time.Minute).Return(nil).Once()
	var capturedChallenge string
	fx.twitch.On("AuthURL", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		capturedChallenge = args.String(1)
	}).Return("https://id.twitch.tv/oauth2/authorize?mock=1").Once()
	req := httptest.NewRequest("GET", "/auth/twitch/link", nil)
	rr := httptest.NewRecorder()
	fx.h.TwitchLink(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", rr.Code)
	}
	if loc := rr.Header().Get("Location"); loc == "" {
		t.Fatal("expected Location header")
	}
	cookies := rr.Result().Cookies()
	var verifier string
	for _, c := range cookies {
		if c.Name == verifierCookieName {
			verifier = c.Value
		}
	}
	if verifier == "" {
		t.Fatal("expected pkce verifier cookie")
	}
	if capturedChallenge != pkceS256Challenge(verifier) {
		t.Fatalf("expected S256 challenge, got %q", capturedChallenge)
	}
}

func TestAuthHandler_TwitchLink_StateError(t *testing.T) {
	fx := testAuthHandler(t)
	fx.sessions.On("SetState", anyCtx(), mock.Anything, "", 5*time.Minute).Return(errH).Once()
	req := httptest.NewRequest("GET", "/auth/twitch/link", nil)
	rr := httptest.NewRecorder()
	fx.h.TwitchLink(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rr.Code)
	}
}

func TestAuthHandler_TwitchCallback_Unauthorized(t *testing.T) {
	fx := testAuthHandler(t)
	req := httptest.NewRequest("GET", "/auth/twitch/callback?code=abc&state=x", nil)
	rr := httptest.NewRecorder()
	fx.h.TwitchCallback(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestAuthHandler_TwitchCallback_Success(t *testing.T) {
	fx := testAuthHandler(t)
	fx.sessions.On("VerifyState", anyCtx(), "valid-state").Return("user-1", true, nil).Once()
	fx.twitch.On("Exchange", anyCtx(), "abc", "test-verifier").Return(&twitch.TokenResponse{AccessToken: "mock-access", IDToken: "mock-id-token"}, nil).Once()
	fx.twitch.On("VerifyIDToken", anyCtx(), "mock-id-token", "valid-state").Return(nil).Once()
	fx.twitch.On("GetUser", anyCtx(), "mock-access").Return(&twitch.TwitchUser{ID: "twitch-user-1", Login: "teststreamer"}, nil).Once()
	fx.users.On("SaveTwitchLink", anyCtx(), "user-1", "twitch-user-1").Return(nil).Once()
	req := httptest.NewRequest("GET", "/auth/twitch/callback?code=abc&state=valid-state", nil)
	req.AddCookie(&http.Cookie{Name: stateCookieName, Value: "valid-state"})
	req.AddCookie(&http.Cookie{Name: verifierCookieName, Value: "test-verifier"})
	ctx := authContext(req.Context(), "user-1")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	fx.h.TwitchCallback(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", rr.Code)
	}
	fx.users.AssertCalled(t, "SaveTwitchLink", anyCtx(), "user-1", "twitch-user-1")
	fx.twitch.AssertCalled(t, "Exchange", anyCtx(), "abc", "test-verifier")
	fx.twitch.AssertCalled(t, "VerifyIDToken", anyCtx(), "mock-id-token", "valid-state")
}

func TestAuthHandler_TwitchCallback_InvalidIDToken(t *testing.T) {
	fx := testAuthHandler(t)
	fx.sessions.On("VerifyState", anyCtx(), "valid-state").Return("user-1", true, nil).Once()
	fx.twitch.On("Exchange", anyCtx(), "abc", "test-verifier").Return(&twitch.TokenResponse{AccessToken: "mock-access", IDToken: "mock-id-token"}, nil).Once()
	fx.twitch.On("VerifyIDToken", anyCtx(), "mock-id-token", "valid-state").Return(errH).Once()
	req := httptest.NewRequest("GET", "/auth/twitch/callback?code=abc&state=valid-state", nil)
	req.AddCookie(&http.Cookie{Name: stateCookieName, Value: "valid-state"})
	req.AddCookie(&http.Cookie{Name: verifierCookieName, Value: "test-verifier"})
	ctx := authContext(req.Context(), "user-1")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	fx.h.TwitchCallback(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on invalid id token, got %d", rr.Code)
	}
	fx.users.AssertNotCalled(t, "SaveTwitchLink", anyCtx(), anyCtx(), anyCtx())
}

func TestAuthHandler_TwitchCallback_MissingVerifier(t *testing.T) {
	fx := testAuthHandler(t)
	fx.sessions.On("VerifyState", anyCtx(), "valid-state").Return("user-1", true, nil).Once()
	req := httptest.NewRequest("GET", "/auth/twitch/callback?code=abc&state=valid-state", nil)
	req.AddCookie(&http.Cookie{Name: stateCookieName, Value: "valid-state"})
	ctx := authContext(req.Context(), "user-1")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	fx.h.TwitchCallback(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 without verifier cookie, got %d", rr.Code)
	}
}

func TestAuthHandler_TwitchCallback_StateBoundToDifferentUser(t *testing.T) {
	fx := testAuthHandler(t)
	fx.sessions.On("VerifyState", anyCtx(), "valid-state").Return("other-user", true, nil).Once()
	req := httptest.NewRequest("GET", "/auth/twitch/callback?code=abc&state=valid-state", nil)
	req.AddCookie(&http.Cookie{Name: stateCookieName, Value: "valid-state"})
	ctx := authContext(req.Context(), "user-1")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	fx.h.TwitchCallback(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestAuthHandler_TwitchCallback_Branches(t *testing.T) {
	t.Run("missing params", func(t *testing.T) {
		fx := testAuthHandler(t)
		req := httptest.NewRequest("GET", "/auth/twitch/callback", nil)
		req = req.WithContext(authContext(req.Context(), "user-1"))
		rr := httptest.NewRecorder()
		fx.h.TwitchCallback(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d", rr.Code)
		}
	})
	t.Run("state cookie mismatch", func(t *testing.T) {
		fx := testAuthHandler(t)
		req := httptest.NewRequest("GET", "/auth/twitch/callback?code=abc&state=st", nil)
		req.AddCookie(&http.Cookie{Name: stateCookieName, Value: "other"})
		req = req.WithContext(authContext(req.Context(), "user-1"))
		rr := httptest.NewRecorder()
		fx.h.TwitchCallback(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d", rr.Code)
		}
		fx.sessions.AssertNotCalled(t, "VerifyState", anyCtx(), anyCtx())
	})
	t.Run("verify state error", func(t *testing.T) {
		fx := testAuthHandler(t)
		fx.sessions.On("VerifyState", anyCtx(), "st").Return("", false, errH).Once()
		req := stateReq("GET", "/cb", "st", "abc", true)
		req = req.WithContext(authContext(req.Context(), "user-1"))
		rr := httptest.NewRecorder()
		fx.h.TwitchCallback(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d", rr.Code)
		}
	})
	t.Run("exchange error", func(t *testing.T) {
		fx := testAuthHandler(t)
		fx.sessions.On("VerifyState", anyCtx(), "st").Return("user-1", true, nil).Once()
		fx.twitch.On("Exchange", anyCtx(), "abc", "verifier").Return(nil, errH).Once()
		req := stateReq("GET", "/cb", "st", "abc", true)
		req = req.WithContext(authContext(req.Context(), "user-1"))
		rr := httptest.NewRecorder()
		fx.h.TwitchCallback(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d", rr.Code)
		}
	})
	t.Run("get user error", func(t *testing.T) {
		fx := testAuthHandler(t)
		fx.sessions.On("VerifyState", anyCtx(), "st").Return("user-1", true, nil).Once()
		fx.twitch.On("Exchange", anyCtx(), "abc", "verifier").Return(&twitch.TokenResponse{AccessToken: "mock-access"}, nil).Once()
		fx.twitch.On("VerifyIDToken", anyCtx(), "", "st").Return(nil).Once()
		fx.twitch.On("GetUser", anyCtx(), "mock-access").Return(nil, errH).Once()
		req := stateReq("GET", "/cb", "st", "abc", true)
		req = req.WithContext(authContext(req.Context(), "user-1"))
		rr := httptest.NewRecorder()
		fx.h.TwitchCallback(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d", rr.Code)
		}
	})
	t.Run("save twitch link error", func(t *testing.T) {
		fx := testAuthHandler(t)
		fx.sessions.On("VerifyState", anyCtx(), "st").Return("user-1", true, nil).Once()
		fx.twitch.On("Exchange", anyCtx(), "abc", "verifier").Return(&twitch.TokenResponse{AccessToken: "mock-access", IDToken: "mock-id-token"}, nil).Once()
		fx.twitch.On("VerifyIDToken", anyCtx(), "mock-id-token", "st").Return(nil).Once()
		fx.twitch.On("GetUser", anyCtx(), "mock-access").Return(&twitch.TwitchUser{ID: "twitch-user-1"}, nil).Once()
		fx.users.On("SaveTwitchLink", anyCtx(), "user-1", "twitch-user-1").Return(errH).Once()
		req := stateReq("GET", "/cb", "st", "abc", true)
		req = req.WithContext(authContext(req.Context(), "user-1"))
		rr := httptest.NewRecorder()
		fx.h.TwitchCallback(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d", rr.Code)
		}
	})
}

func TestAuthHandler_TwitchUnlink_Unauthorized(t *testing.T) {
	fx := testAuthHandler(t)
	req := httptest.NewRequest("POST", "/auth/twitch/unlink", nil)
	rr := httptest.NewRecorder()
	fx.h.TwitchUnlink(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestAuthHandler_TwitchUnlink_Success(t *testing.T) {
	fx := testAuthHandler(t)
	subs := []store.NotificationSubscription{{ID: "sub-1", TwitchSubscriptionID: "eventsub-1"}}
	fx.subs.On("RevokeAllByUser", anyCtx(), "user-1").Return(subs, nil).Once()
	fx.eventSub.On("GetAppAccessToken", anyCtx()).Return("mock-app-token", nil).Once()
	fx.eventSub.On("DeleteSubscription", anyCtx(), "mock-app-token", "eventsub-1").Return(nil).Once()
	fx.users.On("ClearTwitchLink", anyCtx(), "user-1").Return(nil).Once()

	req := authReq("POST", "/auth/twitch/unlink", "", "user-1")
	rr := httptest.NewRecorder()
	fx.h.TwitchUnlink(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	fx.users.AssertCalled(t, "ClearTwitchLink", anyCtx(), "user-1")
	fx.subs.AssertCalled(t, "RevokeAllByUser", anyCtx(), "user-1")
	fx.eventSub.AssertCalled(t, "DeleteSubscription", anyCtx(), "mock-app-token", "eventsub-1")
}

func TestAuthHandler_TwitchUnlink_NoSubs(t *testing.T) {
	fx := testAuthHandler(t)
	fx.subs.On("RevokeAllByUser", anyCtx(), "user-1").Return([]store.NotificationSubscription{}, nil).Once()
	fx.users.On("ClearTwitchLink", anyCtx(), "user-1").Return(nil).Once()

	req := authReq("POST", "/auth/twitch/unlink", "", "user-1")
	rr := httptest.NewRecorder()
	fx.h.TwitchUnlink(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	fx.eventSub.AssertNotCalled(t, "GetAppAccessToken", anyCtx())
}

func TestAuthHandler_TwitchUnlink_Branches(t *testing.T) {
	t.Run("revoke error", func(t *testing.T) {
		fx := testAuthHandler(t)
		fx.subs.On("RevokeAllByUser", anyCtx(), "user-1").Return(nil, errH).Once()
		req := authReq("POST", "/x", "", "user-1")
		rr := httptest.NewRecorder()
		fx.h.TwitchUnlink(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d", rr.Code)
		}
	})
	t.Run("app token error", func(t *testing.T) {
		fx := testAuthHandler(t)
		subs := []store.NotificationSubscription{{ID: "sub-1", TwitchSubscriptionID: "sub-1"}}
		fx.subs.On("RevokeAllByUser", anyCtx(), "user-1").Return(subs, nil).Once()
		fx.eventSub.On("GetAppAccessToken", anyCtx()).Return("", errH).Once()
		req := authReq("POST", "/x", "", "user-1")
		rr := httptest.NewRecorder()
		fx.h.TwitchUnlink(rr, req)
		if rr.Code != http.StatusBadGateway {
			t.Fatalf("status = %d", rr.Code)
		}
	})
	t.Run("delete sub error", func(t *testing.T) {
		fx := testAuthHandler(t)
		subs := []store.NotificationSubscription{{ID: "sub-1", TwitchSubscriptionID: "sub-1"}}
		fx.subs.On("RevokeAllByUser", anyCtx(), "user-1").Return(subs, nil).Once()
		fx.eventSub.On("GetAppAccessToken", anyCtx()).Return("mock-app-token", nil).Once()
		fx.eventSub.On("DeleteSubscription", anyCtx(), "mock-app-token", "sub-1").Return(errH).Once()
		req := authReq("POST", "/x", "", "user-1")
		rr := httptest.NewRecorder()
		fx.h.TwitchUnlink(rr, req)
		if rr.Code != http.StatusBadGateway {
			t.Fatalf("status = %d", rr.Code)
		}
	})
	t.Run("clear link error", func(t *testing.T) {
		fx := testAuthHandler(t)
		subs := []store.NotificationSubscription{{ID: "sub-1", TwitchSubscriptionID: "sub-1"}}
		fx.subs.On("RevokeAllByUser", anyCtx(), "user-1").Return(subs, nil).Once()
		fx.eventSub.On("GetAppAccessToken", anyCtx()).Return("mock-app-token", nil).Once()
		fx.eventSub.On("DeleteSubscription", anyCtx(), "mock-app-token", "sub-1").Return(nil).Once()
		fx.users.On("ClearTwitchLink", anyCtx(), "user-1").Return(errH).Once()
		req := authReq("POST", "/x", "", "user-1")
		rr := httptest.NewRecorder()
		fx.h.TwitchUnlink(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d", rr.Code)
		}
	})
}