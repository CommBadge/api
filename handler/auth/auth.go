// Package auth implements the Discord/Twitch authentication flows and the
// session lifecycle endpoints.
package auth

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"time"

	"kronus.dev/commbadge_api/handler/common"
	"kronus.dev/commbadge_api/handler/contracts"
	"kronus.dev/commbadge_api/middleware"
	"kronus.dev/commbadge_api/session"
	"kronus.dev/commbadge_api/store"
)

// AuthHandler implements the login, logout, callback, and Twitch linking
// endpoints. All state is injected so tests can substitute mocks.
type AuthHandler struct {
	Discord       contracts.DiscordClient
	Twitch        contracts.TwitchClient
	EventSub      contracts.TwitchEventSubClient
	Sessions      contracts.SessionStore
	Users         contracts.UserRepository
	Subscriptions contracts.SubscriptionRepository
	FrontendURL   string
	RedirectURL   string
	SessionTTL    time.Duration
	SecureCookies bool
	SignJWT       func(sessionID string) (string, error)
	VerifyJWT     func(tokenString string) (string, error)
}

// stateCookieName holds the CSRF state across the OAuth redirect round-trip.
// Binding the state to an HttpOnly cookie (in addition to the URL parameter)
// prevents login CSRF: an attacker cannot plant a known state in the victim's
// browser, so a victim-initiated login can never complete with the attacker's
// state and code.
const stateCookieName = "oauth_state"

func randomState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (h *AuthHandler) setStateCookie(w http.ResponseWriter, state string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name:     stateCookieName,
		Value:    state,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.SecureCookies,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
	})
}

func (h *AuthHandler) clearStateCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     stateCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   h.SecureCookies,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session")
	if err == nil && cookie.Value != "" {
		if _, err := h.VerifyJWT(cookie.Value); err == nil {
			http.Redirect(w, r, h.FrontendURL, http.StatusFound)
			return
		}
	}

	state, err := randomState()
	if err != nil {
		common.RespondError(w, http.StatusInternalServerError, "failed to generate state")
		return
	}

	if err := h.Sessions.SetState(r.Context(), state, "", 5*time.Minute); err != nil {
		common.RespondError(w, http.StatusInternalServerError, "failed to store state")
		return
	}

	h.setStateCookie(w, state, int((5 * time.Minute).Seconds()))

	verifier, err := generatePKCEVerifier()
	if err != nil {
		common.RespondError(w, http.StatusInternalServerError, "failed to generate pkce verifier")
		return
	}
	h.setVerifierCookie(w, verifier, int((5 * time.Minute).Seconds()))

	authURL := h.Discord.AuthURL(state, pkceS256Challenge(verifier))
	http.Redirect(w, r, authURL, http.StatusFound)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session")
	if err != nil {
		common.RespondJSON(w, http.StatusOK, map[string]string{"message": "logged out"})
		return
	}

	sessionID, err := h.VerifyJWT(cookie.Value)
	if err != nil {
		common.RespondJSON(w, http.StatusOK, map[string]string{"message": "logged out"})
		return
	}

	h.Sessions.Delete(r.Context(), sessionID)

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
		Secure:   h.SecureCookies,
	})
	common.RespondJSON(w, http.StatusOK, map[string]string{"message": "logged out"})
}

func (h *AuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	if code == "" || state == "" {
		common.RespondError(w, http.StatusBadRequest, "missing code or state")
		return
	}

	// The state must match the HttpOnly cookie set when the flow started;
	// otherwise an attacker-supplied state (login CSRF) is rejected.
	stateCookie, err := r.Cookie(stateCookieName)
	if err != nil || stateCookie.Value == "" || stateCookie.Value != state {
		h.clearStateCookie(w)
		common.RespondError(w, http.StatusBadRequest, "invalid state")
		return
	}

	binding, valid, err := h.Sessions.VerifyState(r.Context(), state)
	if err != nil {
		slog.Error("state verification error", "error", err)
		common.RespondError(w, http.StatusInternalServerError, "state verification failed")
		return
	}
	if !valid || binding != "" {
		h.clearStateCookie(w)
		common.RespondError(w, http.StatusBadRequest, "invalid state")
		return
	}
	h.clearStateCookie(w)

	// The PKCE verifier must be present and is single-use: it is cleared once
	// consumed, so a replayed callback cannot reuse it.
	verifierCookie, err := r.Cookie(verifierCookieName)
	if err != nil || verifierCookie.Value == "" {
		h.clearVerifierCookie(w)
		common.RespondError(w, http.StatusBadRequest, "missing pkce verifier")
		return
	}
	h.clearVerifierCookie(w)

	tokens, err := h.Discord.Exchange(r.Context(), code, verifierCookie.Value)
	if err != nil {
		slog.Error("token exchange error", "error", err)
		common.RespondError(w, http.StatusInternalServerError, "token exchange failed")
		return
	}

	discordUser, err := h.Discord.GetUser(r.Context(), tokens.AccessToken)
	if err != nil {
		slog.Error("get user error", "error", err)
		common.RespondError(w, http.StatusInternalServerError, "failed to get user")
		return
	}

	if err := h.Users.UpsertUser(r.Context(), &store.User{
		ID:          discordUser.ID,
		Username:    discordUser.Username,
		DisplayName: discordUser.DisplayName(),
		Email:       discordUser.Email,
		AvatarURL:   discordUser.AvatarURL(),
	}); err != nil {
		slog.Error("upsert user error", "error", err)
		common.RespondError(w, http.StatusInternalServerError, "failed to save user")
		return
	}

	sessionID, err := h.Sessions.Create(r.Context(), &session.Session{
		UserID:             discordUser.ID,
		Username:           discordUser.Username,
		DisplayName:        discordUser.DisplayName(),
		Email:              discordUser.Email,
		AvatarURL:          discordUser.AvatarURL(),
		DiscordAccessToken: tokens.AccessToken,
	})
	if err != nil {
		slog.Error("create session error", "error", err)
		common.RespondError(w, http.StatusInternalServerError, "failed to create session")
		return
	}

	jwtToken, err := h.SignJWT(sessionID)
	if err != nil {
		slog.Error("sign jwt error", "error", err)
		common.RespondError(w, http.StatusInternalServerError, "failed to sign token")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    jwtToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.SecureCookies,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(h.SessionTTL.Seconds()),
	})
	http.Redirect(w, r, h.FrontendURL, http.StatusFound)
}

func (h *AuthHandler) TwitchLink(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	state, err := randomState()
	if err != nil {
		common.RespondError(w, http.StatusInternalServerError, "failed to generate state")
		return
	}

	if err := h.Sessions.SetState(r.Context(), state, userID, 5*time.Minute); err != nil {
		common.RespondError(w, http.StatusInternalServerError, "failed to store state")
		return
	}

	h.setStateCookie(w, state, int((5 * time.Minute).Seconds()))

	verifier, err := generatePKCEVerifier()
	if err != nil {
		common.RespondError(w, http.StatusInternalServerError, "failed to generate pkce verifier")
		return
	}
	h.setVerifierCookie(w, verifier, int((5 * time.Minute).Seconds()))

	http.Redirect(w, r, h.Twitch.AuthURL(state, pkceS256Challenge(verifier)), http.StatusFound)
}

func (h *AuthHandler) TwitchCallback(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		common.RespondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	if code == "" || state == "" {
		common.RespondError(w, http.StatusBadRequest, "missing code or state")
		return
	}

	// The state is bound to both the HttpOnly cookie and the authenticated
	// user. A state minted by an attacker (or for a different account) cannot
	// be used to link a Twitch account to this user.
	stateCookie, err := r.Cookie(stateCookieName)
	if err != nil || stateCookie.Value == "" || stateCookie.Value != state {
		h.clearStateCookie(w)
		common.RespondError(w, http.StatusBadRequest, "invalid state")
		return
	}

	binding, valid, err := h.Sessions.VerifyState(r.Context(), state)
	if err != nil {
		slog.Error("state verification error", "error", err)
		common.RespondError(w, http.StatusInternalServerError, "state verification failed")
		return
	}
	if !valid || binding != userID {
		h.clearStateCookie(w)
		common.RespondError(w, http.StatusBadRequest, "invalid state")
		return
	}
	h.clearStateCookie(w)

	verifierCookie, err := r.Cookie(verifierCookieName)
	if err != nil || verifierCookie.Value == "" {
		h.clearVerifierCookie(w)
		common.RespondError(w, http.StatusBadRequest, "missing pkce verifier")
		return
	}
	h.clearVerifierCookie(w)

	tokens, err := h.Twitch.Exchange(r.Context(), code, verifierCookie.Value)
	if err != nil {
		slog.Error("twitch token exchange error", "error", err)
		common.RespondError(w, http.StatusInternalServerError, "token exchange failed")
		return
	}

	// The ID token must be present and carry the nonce (= state) that was sent
	// in the authorization request. This proves the token was issued to this
	// login attempt and this client, rejecting replays and tokens issued for
	// other purposes.
	if err := h.Twitch.VerifyIDToken(r.Context(), tokens.IDToken, state); err != nil {
		slog.Error("twitch id token verification failed", "error", err)
		common.RespondError(w, http.StatusUnauthorized, "invalid twitch id token")
		return
	}

	twitchUser, err := h.Twitch.GetUser(r.Context(), tokens.AccessToken)
	if err != nil {
		slog.Error("twitch get user error", "error", err)
		common.RespondError(w, http.StatusInternalServerError, "failed to get twitch user")
		return
	}

	if err := h.Users.SaveTwitchLink(r.Context(), userID, twitchUser.ID); err != nil {
		slog.Error("save twitch link error", "error", err)
		common.RespondError(w, http.StatusInternalServerError, "failed to link twitch account")
		return
	}

	http.Redirect(w, r, h.FrontendURL, http.StatusFound)
}

func (h *AuthHandler) TwitchUnlink(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		common.RespondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	subs, err := h.Subscriptions.RevokeAllByUser(r.Context(), userID)
	if err != nil {
		slog.Error("revoke subscriptions error", "error", err)
		common.RespondError(w, http.StatusInternalServerError, "failed to revoke subscriptions")
		return
	}

	if len(subs) > 0 {
		appToken, err := h.EventSub.GetAppAccessToken(r.Context())
		if err != nil {
			slog.Error("app token error", "error", err)
			common.RespondError(w, http.StatusBadGateway, "failed to reach twitch")
			return
		}
		for _, sub := range subs {
			if err := h.EventSub.DeleteSubscription(r.Context(), appToken, sub.TwitchSubscriptionID); err != nil {
				slog.Error("delete subscription error", "error", err)
				common.RespondError(w, http.StatusBadGateway, "failed to revoke subscription")
				return
			}
		}
	}

	if err := h.Users.ClearTwitchLink(r.Context(), userID); err != nil {
		slog.Error("clear twitch link error", "error", err)
		common.RespondError(w, http.StatusInternalServerError, "failed to unlink twitch account")
		return
	}

	common.RespondJSON(w, http.StatusOK, map[string]string{"message": "twitch account unlinked"})
}