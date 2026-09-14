package app

import (
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"kronus.dev/commbadge_api/config"
	"kronus.dev/commbadge_api/handler/admin"
	"kronus.dev/commbadge_api/handler/auth"
	"kronus.dev/commbadge_api/handler/community"
	"kronus.dev/commbadge_api/handler/contracts"
	"kronus.dev/commbadge_api/handler/health"
	"kronus.dev/commbadge_api/handler/notifications"
	"kronus.dev/commbadge_api/handler/settings"
	"kronus.dev/commbadge_api/handler/tickets"
	"kronus.dev/commbadge_api/handler/whoami"
	"kronus.dev/commbadge_api/jwt"
	"kronus.dev/commbadge_api/middleware"
	"kronus.dev/commbadge_api/session"
	"kronus.dev/commbadge_api/store"
)

// TwitchProvider is the union of the Twitch OAuth/identity surface and the
// EventSub subscription surface. The production client and the test mock both
// implement all of it, so a single value is shared by the auth module (OAuth)
// and the notifications/auth modules (EventSub).
type TwitchProvider interface {
	contracts.TwitchClient
	contracts.TwitchEventSubClient
}

type Deps struct {
	Cfg      *config.Config
	DB       *pgxpool.Pool
	Sessions *session.Store
	JWT      *jwt.JWT
	Discord  contracts.DiscordClient
	Twitch   TwitchProvider
	S3       contracts.S3Repository
}

// Build wires stores, handlers, middleware, and routes into a single
// http.Handler. It mirrors the production assembly so integration tests can
// build the real stack with fake external providers.
func Build(deps Deps) http.Handler {
	userStore := store.NewUserStore(deps.DB)
	communityStore := store.NewCommunityStore(deps.DB)
	adminStore := store.NewAdminStore(deps.DB)
	ticketStore := store.NewTicketStore(deps.DB)
	notificationStore := store.NewNotificationStore(deps.DB)

	authHandler := auth.GetHandlers(auth.Deps{
		Discord:       deps.Discord,
		Twitch:        deps.Twitch,
		EventSub:      deps.Twitch,
		Subscriptions: notificationStore,
		Sessions:      deps.Sessions,
		Users:         userStore,
		FrontendURL:   deps.Cfg.FrontendURL,
		RedirectURL:   deps.Cfg.RedirectURL,
		SessionTTL:    deps.Cfg.SessionTTL,
		SecureCookies: deps.Cfg.TLS || deps.Cfg.CookieSecure,
		SignJWT:       deps.JWT.Sign,
		VerifyJWT:     deps.JWT.Verify,
	})

	whoamiHandler := whoami.GetHandlers(whoami.Deps{
		Users:       userStore,
		Communities: communityStore,
	})

	communityHandler := community.GetHandlers(community.Deps{
		Communities: communityStore,
		S3:          deps.S3,
	})

	settingsHandler := settings.GetHandlers(settings.Deps{
		Users:           userStore,
		Communities:     communityStore,
		Discord:         deps.Discord,
		DiscordBotToken: deps.Cfg.DiscordBotToken,
		Sessions:        deps.Sessions,
	})

	notificationHandler := notifications.GetHandlers(notifications.Deps{
		Users:       userStore,
		Subs:        notificationStore,
		EventSub:    deps.Twitch,
		CallbackURL: deps.Cfg.EventSubCallback,
		Secret:      deps.Cfg.EventSubSecret,
	})

	healthHandler := health.GetHandlers(health.Deps{
		DB:        deps.DB,
		Sessions:  deps.Sessions,
		S3:        deps.S3,
		MaintFile: deps.Cfg.MaintenanceFile,
	})

	ticketHandler := tickets.GetHandlers(tickets.Deps{
		Tickets: ticketStore,
	})

	// DegradedDetector stays nil in production: enabling maintenance is purely
	// an admin action here, while the app-level detector owns the actual halt.
	adminHandler := admin.GetHandlers(admin.Deps{
		AdminStore:     adminStore,
		CommunityStore: communityStore,
		TicketStore:    ticketStore,
		UserStore:      userStore,
	})

	authMW := &middleware.AuthMiddleware{
		VerifyJWT: deps.JWT.Verify,
		Sessions:  deps.Sessions,
	}

	apiAuth := authMW.RequireAuth
	apiNotBanned := authMW.RequireNotBanned(userStore)
	apiAdmin := authMW.RequireAdmin(adminStore)

	jsonBody := middleware.LimitBodySize(1024)
	uploadBody := middleware.LimitBodySize(2_500_000)

	corsMW := middleware.NewCORS(deps.Cfg.CORSOrigins)
	rateLimiter := middleware.NewRateLimiter(deps.Cfg.RateLimitRPS, deps.Cfg.RateLimitBurst, deps.Cfg.TrustedProxies)
	degradedDetector := middleware.NewDegradedDetector(deps.DB, deps.Sessions, deps.Cfg.MaintenanceFile)

	apiMux := http.NewServeMux()
	communityHandler.RegisterAPI(apiMux)
	settingsHandler.RegisterAPI(apiMux)

	uploadMux := http.NewServeMux()
	communityHandler.RegisterUpload(uploadMux)

	ticketMux := http.NewServeMux()
	ticketHandler.Register(ticketMux)

	adminMux := http.NewServeMux()
	adminHandler.Register(adminMux)

	notifMux := http.NewServeMux()
	notificationHandler.Register(notifMux)

	mux := http.NewServeMux()

	authHandler.Routes(mux, auth.Middlewares{
		Auth: apiAuth,
		Body: jsonBody,
	})

	// The whoami route is only served to authenticated, non-banned users.
	whoamiAuth := func(h http.Handler) http.Handler {
		return middleware.NewChain(apiAuth, apiNotBanned).Then(h)
	}
	whoamiHandler.Routes(mux, whoamiAuth)

	healthHandler.Register(mux)

	mux.Handle("/api/", middleware.NewChain(
		apiAuth,
		apiNotBanned,
		jsonBody,
	).Then(http.StripPrefix("/api", apiMux)))

	mux.Handle("/api/upload/", middleware.NewChain(
		apiAuth,
		apiNotBanned,
		uploadBody,
	).Then(http.StripPrefix("/api/upload", uploadMux)))

	ticketsChain := middleware.NewChain(
		apiAuth,
		apiNotBanned,
		jsonBody,
	).Then(http.StripPrefix("/api", ticketMux))
	mux.Handle("/api/tickets", ticketsChain)
	mux.Handle("/api/tickets/", ticketsChain)

	mux.Handle("/api/admin/", middleware.NewChain(
		apiAdmin,
		apiNotBanned,
		jsonBody,
	).Then(http.StripPrefix("/api", adminMux)))

	mux.Handle("/api/notifications/", middleware.NewChain(
		apiAuth,
		apiNotBanned,
		jsonBody,
	).Then(http.StripPrefix("/api/notifications", notifMux)))

	// /metrics discloses runtime internals, so it is only served when explicit
	// credentials are configured, and always behind Basic authentication.
	if deps.Cfg.MetricsUser != "" && deps.Cfg.MetricsPassword != "" {
		mux.Handle("GET /metrics", middleware.BasicAuth(deps.Cfg.MetricsUser, deps.Cfg.MetricsPassword, promhttp.Handler()))
	} else {
		mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, r *http.Request) {
			http.NotFound(w, r)
		})
	}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello World! %s", time.Now())
	})

	var h http.Handler = mux
	h = middleware.Logging(h)
	h = rateLimiter.Wrap(h)
	h = degradedDetector.Wrap(h)
	h = corsMW.Wrap(h)

	return h
}