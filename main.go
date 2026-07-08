package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"kronus.dev/commbadge_api/config"
	"kronus.dev/commbadge_api/handler"
	"kronus.dev/commbadge_api/jwt"
	"kronus.dev/commbadge_api/middleware"
	"kronus.dev/commbadge_api/session"
	"kronus.dev/commbadge_api/store"
	"kronus.dev/commbadge_api/twitch"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config load failed", "error", err)
		os.Exit(1)
	}

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	if cfg.TLS {
		if _, err := os.Stat(cfg.TLSCert); err != nil {
			slog.Error("TLS cert not found", "path", cfg.TLSCert)
			os.Exit(1)
		}
		if _, err := os.Stat(cfg.TLSKey); err != nil {
			slog.Error("TLS key not found", "path", cfg.TLSKey)
			os.Exit(1)
		}
	}

	ctx := context.Background()

	pool, err := store.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("database pool init failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	sessions, err := session.NewStore(cfg.RedisURL, cfg.SessionTTL)
	if err != nil {
		slog.Error("redis init failed", "error", err)
		os.Exit(1)
	}
	defer sessions.Close()

	var s3 *store.S3Client
	if cfg.S3Endpoint != "" {
		s3, err = store.NewS3Client(cfg.S3Endpoint, cfg.S3AccessKey, cfg.S3SecretKey, cfg.S3Bucket, cfg.S3Region, cfg.S3UseSSL)
		if err != nil {
			slog.Error("s3 init failed", "error", err)
			os.Exit(1)
		}
		if err := s3.BucketExists(ctx); err != nil {
			slog.Error("s3 bucket check failed", "error", err)
			os.Exit(1)
		}
	}

	j := jwt.New(cfg.JWTSecret, cfg.SessionTTL)

	userStore := store.NewUserStore(pool)
	communityStore := store.NewCommunityStore(pool)
	adminStore := store.NewAdminStore(pool)
	ticketStore := store.NewTicketStore(pool)

	if len(cfg.Admins) > 0 {
		if err := adminStore.SeedAdmins(ctx, cfg.Admins); err != nil {
			slog.Error("admin seeding failed", "error", err)
			os.Exit(1)
		}
		slog.Info("admins seeded", "count", len(cfg.Admins))
	}

	twitchClient := twitch.NewClient(&twitch.Config{
		ClientID:     cfg.TwitchClientID,
		ClientSecret: cfg.TwitchSecret,
		RedirectURL:  cfg.RedirectURL,
	})

	authHandler := &handler.AuthHandler{
		Twitch:      twitchClient,
		Sessions:    sessions,
		Users:       userStore,
		JWTSecret:   cfg.JWTSecret,
		FrontendURL: cfg.FrontendURL,
		RedirectURL: cfg.RedirectURL,
		SessionTTL:  cfg.SessionTTL,
		IsTLS:       cfg.TLS,
		SignJWT:     j.Sign,
		VerifyJWT:   j.Verify,
	}

	whoamiHandler := &handler.WhoamiHandler{
		Users:       userStore,
		Communities: communityStore,
	}

	communityHandler := &handler.CommunityHandler{
		Communities: communityStore,
		S3:          s3,
	}

	healthHandler := &handler.HealthHandler{
		DB:        pool,
		Sessions:  sessions,
		S3:        s3,
		MaintFile: cfg.MaintenanceFile,
	}

	ticketHandler := &handler.TicketHandler{
		Tickets: ticketStore,
	}

	adminHandler := &handler.AdminHandler{
		AdminStore:       adminStore,
		CommunityStore:   communityStore,
		TicketStore:      ticketStore,
		UserStore:        userStore,
	}

	authMW := &middleware.AuthMiddleware{
		VerifyJWT: j.Verify,
		Sessions:  sessions,
	}

	apiAuth := authMW.RequireAuth
	apiNotBanned := authMW.RequireNotBanned(userStore)
	apiAdmin := authMW.RequireAdmin(adminStore)

	jsonBody := middleware.LimitBodySize(1024)
	uploadBody := middleware.LimitBodySize(2_500_000)

	corsMW := middleware.NewCORS(cfg.CORSOrigins)
	rateLimiter := middleware.NewRateLimiter(cfg.RateLimitRPS, cfg.RateLimitBurst)
	degradedDetector := middleware.NewDegradedDetector(pool, sessions, cfg.MaintenanceFile)

	// ── Routes ──────────────────────────────────────────────

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

	mux.Handle("/api/tickets/", middleware.NewChain(
		apiAuth,
		apiNotBanned,
		jsonBody,
	).Then(http.StripPrefix("/api/tickets", ticketMux)))

	mux.Handle("/api/admin/", middleware.NewChain(
		apiAdmin,
		apiNotBanned,
		jsonBody,
	).Then(http.StripPrefix("/api/admin", adminMux)))

	mux.HandleFunc("GET /healthz", healthHandler.Healthz)

	mux.Handle("GET /metrics", promhttp.Handler())

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello World! %s", time.Now())
	})

	// ── Middleware stack (outermost first) ────────────────────

	var h http.Handler = mux
	h = middleware.Logging(h)
	h = rateLimiter.Wrap(h)
	h = degradedDetector.Wrap(h)
	h = corsMW.Wrap(h)

	srv := &http.Server{
		Addr:         ":" + fmt.Sprintf("%d", cfg.Port),
		Handler:      h,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		slog.Info("listening", "addr", srv.Addr)
		var err error
		if cfg.TLS {
			err = srv.ListenAndServeTLS(cfg.TLSCert, cfg.TLSKey)
		} else {
			err = srv.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-stop
	slog.Info("shutting down...")

	degradedDetector.Halt()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown error", "error", err)
		os.Exit(1)
	}
	slog.Info("server stopped")
}
