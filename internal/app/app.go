package app

import (
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"kronus.dev/commbadge_api/config"
	"kronus.dev/commbadge_api/handler"
	"kronus.dev/commbadge_api/jwt"
	"kronus.dev/commbadge_api/middleware"
	"kronus.dev/commbadge_api/session"
	"kronus.dev/commbadge_api/store"
)

type TwitchProvider interface {
	handler.TwitchClient
	handler.TwitchEventSubClient
}

type Deps struct {
	Cfg      *config.Config
	DB       *pgxpool.Pool
	Sessions *session.Store
	JWT      *jwt.JWT
	Discord  handler.DiscordClient
	Twitch   TwitchProvider
	S3       handler.S3Repository
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

	authHandler := &handler.AuthHandler{
		Discord:       deps.Discord,
		Twitch:        deps.Twitch,
		EventSub:      deps.Twitch,
		Subscriptions: notificationStore,
		Sessions:      deps.Sessions,
		Users:         userStore,
		FrontendURL:   deps.Cfg.FrontendURL,
		RedirectURL:   deps.Cfg.RedirectURL,
		SessionTTL:    deps.Cfg.SessionTTL,
		IsTLS:         deps.Cfg.TLS,
		SignJWT:       deps.JWT.Sign,
		VerifyJWT:     deps.JWT.Verify,
	}

	whoamiHandler := &handler.WhoamiHandler{
		Users:       userStore,
		Communities: communityStore,
	}

	communityHandler := &handler.CommunityHandler{
		Communities: communityStore,
		S3:          deps.S3,
	}

	settingsHandler := &handler.SettingsHandler{
		Users:       userStore,
		Communities: communityStore,
	}

	notificationHandler := &handler.NotificationHandler{
		Users:       userStore,
		Subs:        notificationStore,
		EventSub:    deps.Twitch,
		CallbackURL: deps.Cfg.EventSubCallback,
		Secret:      deps.Cfg.EventSubSecret,
	}

	healthHandler := &handler.HealthHandler{
		DB:        deps.DB,
		Sessions:  deps.Sessions,
		S3:        deps.S3,
		MaintFile: deps.Cfg.MaintenanceFile,
	}

	ticketHandler := &handler.TicketHandler{
		Tickets: ticketStore,
	}

	adminHandler := &handler.AdminHandler{
		AdminStore:     adminStore,
		CommunityStore: communityStore,
		TicketStore:    ticketStore,
		UserStore:      userStore,
	}

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
	rateLimiter := middleware.NewRateLimiter(deps.Cfg.RateLimitRPS, deps.Cfg.RateLimitBurst)
	degradedDetector := middleware.NewDegradedDetector(deps.DB, deps.Sessions, deps.Cfg.MaintenanceFile)

	apiMux := http.NewServeMux()
	apiMux.HandleFunc("GET /communities", communityHandler.ListUserCommunities)
	apiMux.HandleFunc("POST /communities", communityHandler.CreateCommunity)
	apiMux.HandleFunc("GET /communities/{communityID}", communityHandler.GetCommunity)
	apiMux.HandleFunc("PUT /communities/{communityID}", communityHandler.UpdateCommunity)
	apiMux.HandleFunc("DELETE /communities/{communityID}", communityHandler.DeleteCommunity)
	apiMux.HandleFunc("POST /communities/{communityID}/join", communityHandler.JoinCommunity)
	apiMux.HandleFunc("POST /communities/{communityID}/moderate", communityHandler.ModerateCommunity)
	apiMux.HandleFunc("POST /communities/{communityID}/leave", communityHandler.LeaveCommunity)
	apiMux.HandleFunc("PUT /communities/{communityID}/join-link", communityHandler.RegenerateJoinLink)
	apiMux.HandleFunc("PATCH /communities/{communityID}/discord", settingsHandler.SetCommunityDiscord)
	apiMux.HandleFunc("PATCH /me/discord-notification", settingsHandler.SetNotificationChannel)
	apiMux.HandleFunc("PATCH /me/shoutout-template", settingsHandler.SetShoutoutTemplate)

	uploadMux := http.NewServeMux()
	uploadMux.HandleFunc("POST /logo/{communityID}", communityHandler.UploadLogo)

	ticketMux := http.NewServeMux()
	ticketMux.HandleFunc("POST /tickets", ticketHandler.Create)
	ticketMux.HandleFunc("GET /tickets", ticketHandler.List)
	ticketMux.HandleFunc("GET /tickets/{ticketID}", ticketHandler.Get)
	ticketMux.HandleFunc("POST /tickets/{ticketID}/messages", ticketHandler.AddMessage)

	adminMux := http.NewServeMux()
	adminMux.HandleFunc("GET /admin/communities", adminHandler.ListCommunities)
	adminMux.HandleFunc("DELETE /admin/communities/{communityID}", adminHandler.DisbandCommunity)
	adminMux.HandleFunc("POST /admin/maintenance", adminHandler.SetMaintenance)
	adminMux.HandleFunc("GET /admin/tickets/count", adminHandler.TicketCount)
	adminMux.HandleFunc("GET /admin/tickets", adminHandler.ListTickets)
	adminMux.HandleFunc("GET /admin/tickets/{ticketID}", adminHandler.GetTicket)
	adminMux.HandleFunc("POST /admin/tickets/{ticketID}/reply", adminHandler.ReplyTicket)
	adminMux.HandleFunc("PATCH /admin/tickets/{ticketID}/status", adminHandler.UpdateTicketStatus)
	adminMux.HandleFunc("GET /admin/users", adminHandler.ListUsers)
	adminMux.HandleFunc("GET /admin/users/search", adminHandler.SearchUsers)
	adminMux.HandleFunc("GET /admin/users/{userID}/warnings", adminHandler.GetUserWarnings)
	adminMux.HandleFunc("POST /admin/users/{userID}/warn", adminHandler.WarnUser)
	adminMux.HandleFunc("POST /admin/users/{userID}/ban", adminHandler.BanUser)
	adminMux.HandleFunc("POST /admin/users/{userID}/unban", adminHandler.UnbanUser)

	mux := http.NewServeMux()

	mux.HandleFunc("GET /auth/login", authHandler.Login)
	mux.Handle("POST /auth/logout", jsonBody(http.HandlerFunc(authHandler.Logout)))
	mux.HandleFunc("GET /auth/callback", authHandler.Callback)

	mux.Handle("GET /auth/twitch/link", apiAuth(http.HandlerFunc(authHandler.TwitchLink)))
	mux.Handle("GET /auth/twitch/callback", apiAuth(http.HandlerFunc(authHandler.TwitchCallback)))
	mux.Handle("POST /auth/twitch/unlink", apiAuth(http.HandlerFunc(authHandler.TwitchUnlink)))

	mux.Handle("GET /auth/whoami", middleware.NewChain(
		apiAuth,
		apiNotBanned,
	).Then(http.HandlerFunc(whoamiHandler.Whoami)))

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

	notifMux := http.NewServeMux()
	notifMux.HandleFunc("POST /subscriptions", notificationHandler.Subscribe)
	notifMux.HandleFunc("GET /subscriptions", notificationHandler.List)
	notifMux.HandleFunc("DELETE /subscriptions/{id}", notificationHandler.Delete)

	mux.Handle("/api/notifications/", middleware.NewChain(
		apiAuth,
		apiNotBanned,
		jsonBody,
	).Then(http.StripPrefix("/api/notifications", notifMux)))

	mux.HandleFunc("GET /healthz", healthHandler.Healthz)

	mux.Handle("GET /metrics", promhttp.Handler())

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
