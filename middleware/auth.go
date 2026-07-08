package middleware

import (
	"context"
	"net/http"

	"kronus.dev/commbadge_api/session"
)

type AdminChecker interface {
	IsAdmin(ctx context.Context, userID string) (bool, error)
}

type BannedChecker interface {
	IsBanned(ctx context.Context, userID string) (bool, error)
}

type SessionFinder interface {
	Get(ctx context.Context, sessionID string) (*session.Session, error)
}

type contextKey string

const (
	ContextUserID  contextKey = "user_id"
	ContextEmail   contextKey = "email"
	ContextSession contextKey = "session_id"
)

func GetUserID(ctx context.Context) string {
	if v, ok := ctx.Value(ContextUserID).(string); ok {
		return v
	}
	return ""
}

func GetSessionID(ctx context.Context) string {
	if v, ok := ctx.Value(ContextSession).(string); ok {
		return v
	}
	return ""
}

type AuthMiddleware struct {
	VerifyJWT func(tokenString string) (string, error)
	Sessions  SessionFinder
}

func (m *AuthMiddleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session")
		if err != nil {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		sessionID, err := m.VerifyJWT(cookie.Value)
		if err != nil {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		ses, err := m.Sessions.Get(r.Context(), sessionID)
		if err != nil || ses == nil {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), ContextSession, sessionID)
		ctx = context.WithValue(ctx, ContextUserID, ses.UserID)
		ctx = context.WithValue(ctx, ContextEmail, ses.Email)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (m *AuthMiddleware) RequireAdmin(checker AdminChecker) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return m.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID := GetUserID(r.Context())
			isAdmin, err := checker.IsAdmin(r.Context(), userID)
			if err != nil || !isAdmin {
				http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		}))
	}
}

func (m *AuthMiddleware) RequireNotBanned(checker BannedChecker) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return m.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID := GetUserID(r.Context())
			banned, err := checker.IsBanned(r.Context(), userID)
			if err != nil {
				http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
				return
			}
			if banned {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-Content-Type-Options", "nosniff")
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte(`{"error":"account banned"}`))
				return
			}
			next.ServeHTTP(w, r)
		}))
	}
}
