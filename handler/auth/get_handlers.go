package auth

import (
	"net/http"
	"time"

	"kronus.dev/commbadge_api/handler/common"
	"kronus.dev/commbadge_api/handler/contracts"
)

// Deps carries everything GetHandlers needs to build an AuthHandler. The
// provider values satisfy the contracts interfaces structurally, so the mock
// and production implementations both work.
type Deps struct {
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

// Middlewares bundles the middleware the auth routes require. The app wires
// the concrete functions; the module only declares what each route needs.
type Middlewares struct {
	// Auth protects the Twitch linking routes with session authentication.
	Auth common.Middleware
	// Body limits the logout request body.
	Body common.Middleware
}

// GetHandlers builds the AuthHandler from deps.
func GetHandlers(deps Deps) *AuthHandler {
	return &AuthHandler{
		Discord:       deps.Discord,
		Twitch:        deps.Twitch,
		EventSub:      deps.EventSub,
		Subscriptions: deps.Subscriptions,
		Sessions:      deps.Sessions,
		Users:         deps.Users,
		FrontendURL:   deps.FrontendURL,
		RedirectURL:   deps.RedirectURL,
		SessionTTL:    deps.SessionTTL,
		SecureCookies: deps.SecureCookies,
		SignJWT:       deps.SignJWT,
		VerifyJWT:     deps.VerifyJWT,
	}
}

// Routes registers the authentication endpoints on mux. The login and callback
// endpoints intentionally have no authentication middleware; the rest use the
// middleware bundles passed in mw.
func (h *AuthHandler) Routes(mux *http.ServeMux, mw Middlewares) {
	mux.HandleFunc("GET /auth/login", h.Login)
	mux.Handle("POST /auth/logout", mw.Body(http.HandlerFunc(h.Logout)))
	mux.HandleFunc("GET /auth/callback", h.Callback)

	mux.Handle("GET /auth/twitch/link", mw.Auth(http.HandlerFunc(h.TwitchLink)))
	mux.Handle("GET /auth/twitch/callback", mw.Auth(http.HandlerFunc(h.TwitchCallback)))
	mux.Handle("POST /auth/twitch/unlink", mw.Auth(http.HandlerFunc(h.TwitchUnlink)))
}