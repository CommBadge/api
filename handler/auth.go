package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"time"

	"kronus.dev/commbadge_api/session"
	"kronus.dev/commbadge_api/store"
	"kronus.dev/commbadge_api/twitch"
)

type TwitchClient interface {
	AuthURL(state string) string
	Exchange(ctx context.Context, code string) (*twitch.TokenResponse, error)
	GetUser(ctx context.Context, accessToken string) (*twitch.TwitchUser, error)
	RevokeToken(ctx context.Context, accessToken string) error
}

type SessionStore interface {
	Create(ctx context.Context, session *session.Session) (string, error)
	Get(ctx context.Context, sessionID string) (*session.Session, error)
	Delete(ctx context.Context, sessionID string) error
	SetState(ctx context.Context, state string, ttl time.Duration) error
	VerifyState(ctx context.Context, state string) (bool, error)
}

type UserRepository interface {
	UpsertUser(ctx context.Context, user *store.User) error
	GetUserByID(ctx context.Context, id string) (*store.User, error)
	IsBanned(ctx context.Context, userID string) (bool, error)
	ListUsers(ctx context.Context, limit, offset int) ([]store.UserAdminView, error)
	SearchUsers(ctx context.Context, query string, limit int) ([]store.UserAdminView, error)
	GetWarningLog(ctx context.Context, userID string) ([]store.UserWarning, error)
	AddWarning(ctx context.Context, userID, adminID, reason string) error
	BanUser(ctx context.Context, userID, reason string) error
	UnbanUser(ctx context.Context, userID string) error
}

type CommunityRepository interface {
	Create(ctx context.Context, name, description, ownerID string) (*store.Community, error)
	GetByID(ctx context.Context, id string) (*store.Community, error)
	ListByUserID(ctx context.Context, userID string) ([]store.CommunityWithRole, error)
	Update(ctx context.Context, id, name, description string) error
	Delete(ctx context.Context, id string) error
	AddMember(ctx context.Context, communityID, userID, role string) error
	RemoveMember(ctx context.Context, communityID, userID string) error
	GetMemberRole(ctx context.Context, communityID, userID string) (string, error)
	GetMembers(ctx context.Context, communityID string) ([]store.CommunityMember, error)
	IsModeratorOrOwner(ctx context.Context, communityID, userID string) (bool, error)
	IsOwner(ctx context.Context, communityID, userID string) (bool, error)
	TransferOwnership(ctx context.Context, communityID, newOwnerID string) error
	RegenerateJoinLink(ctx context.Context, id string) (string, error)
	UpdateLogoURL(ctx context.Context, id, url string) error
	AdminListAll(ctx context.Context, limit, offset int) ([]store.Community, error)
}

type TicketRepository interface {
	Create(ctx context.Context, userID, subject, body string) (*store.SupportTicket, error)
	ListByUser(ctx context.Context, userID string) ([]store.SupportTicket, error)
	GetByID(ctx context.Context, id string) (*store.SupportTicket, error)
	AddMessage(ctx context.Context, ticketID, userID, body string, isAdmin bool) (*store.TicketMessage, error)
	GetMessages(ctx context.Context, ticketID string) ([]store.TicketMessage, error)
	AdminListAll(ctx context.Context, limit, offset int) ([]store.SupportTicket, error)
	AdminTicketCount(ctx context.Context) (int, error)
	AdminUpdateStatus(ctx context.Context, id, status string) error
}

type S3Repository interface {
	Upload(ctx context.Context, key string, reader io.Reader, size int64) (string, error)
	BucketExists(ctx context.Context) error
}

type AuthHandler struct {
	Twitch      TwitchClient
	Sessions    SessionStore
	Users       UserRepository
	JWTSecret   string
	FrontendURL string
	RedirectURL string
	SessionTTL  time.Duration
	IsTLS       bool
	SignJWT     func(sessionID string) (string, error)
	VerifyJWT   func(tokenString string) (string, error)
}

func randomState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
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
		respondError(w, http.StatusInternalServerError, "failed to generate state")
		return
	}

	if err := h.Sessions.SetState(r.Context(), state, 5*time.Minute); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to store state")
		return
	}

	authURL := h.Twitch.AuthURL(state)
	http.Redirect(w, r, authURL, http.StatusFound)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session")
	if err != nil {
		respondJSON(w, http.StatusOK, map[string]string{"message": "logged out"})
		return
	}

	sessionID, err := h.VerifyJWT(cookie.Value)
	if err != nil {
		respondJSON(w, http.StatusOK, map[string]string{"message": "logged out"})
		return
	}

	ses, _ := h.Sessions.Get(r.Context(), sessionID)
	if ses != nil {
		if ses.TwitchToken != "" {
			h.Twitch.RevokeToken(r.Context(), ses.TwitchToken)
		}
	}

	h.Sessions.Delete(r.Context(), sessionID)

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
		Secure:   h.IsTLS,
	})
	respondJSON(w, http.StatusOK, map[string]string{"message": "logged out"})
}

func (h *AuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	if code == "" || state == "" {
		respondError(w, http.StatusBadRequest, "missing code or state")
		return
	}

	valid, err := h.Sessions.VerifyState(r.Context(), state)
	if err != nil {
		slog.Error("state verification error", "error", err)
		respondError(w, http.StatusInternalServerError, "state verification failed")
		return
	}
	if !valid {
		respondError(w, http.StatusBadRequest, "invalid state")
		return
	}

	tokens, err := h.Twitch.Exchange(r.Context(), code)
	if err != nil {
		slog.Error("token exchange error", "error", err)
		respondError(w, http.StatusInternalServerError, "token exchange failed")
		return
	}

	twitchUser, err := h.Twitch.GetUser(r.Context(), tokens.AccessToken)
	if err != nil {
		slog.Error("get user error", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to get user")
		return
	}

	if err := h.Users.UpsertUser(r.Context(), &store.User{
		ID:          twitchUser.ID,
		Login:       twitchUser.Login,
		DisplayName: twitchUser.DisplayName,
		Email:       twitchUser.Email,
		AvatarURL:   twitchUser.AvatarURL,
	}); err != nil {
		slog.Error("upsert user error", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to save user")
		return
	}

	sessionID, err := h.Sessions.Create(r.Context(), &session.Session{
		UserID:        twitchUser.ID,
		Login:         twitchUser.Login,
		DisplayName:   twitchUser.DisplayName,
		Email:         twitchUser.Email,
		AvatarURL:     twitchUser.AvatarURL,
		TwitchToken:   tokens.AccessToken,
		TwitchRefresh: tokens.RefreshToken,
	})
	if err != nil {
		slog.Error("create session error", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to create session")
		return
	}

	jwtToken, err := h.SignJWT(sessionID)
	if err != nil {
		slog.Error("sign jwt error", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to sign token")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    jwtToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.IsTLS,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(h.SessionTTL.Seconds()),
	})
	http.Redirect(w, r, h.FrontendURL, http.StatusFound)
}
