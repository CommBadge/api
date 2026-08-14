package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"kronus.dev/commbadge_api/config"
	"kronus.dev/commbadge_api/discord"
	"kronus.dev/commbadge_api/handler"
	"kronus.dev/commbadge_api/internal/app"
	"kronus.dev/commbadge_api/jwt"
	"kronus.dev/commbadge_api/migrations"
	"kronus.dev/commbadge_api/session"
	"kronus.dev/commbadge_api/store"
	"kronus.dev/commbadge_api/twitch"
)

func main() {
	migrateOnly := flag.Bool("migrate", false, "apply pending database migrations and exit")
	flag.Parse()

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

	switch cfg.MigrationMode {
	case "validate":
		if err := migrations.Validate(ctx, cfg.DatabaseURL); err != nil {
			slog.Error("schema validation failed", "error", err)
			os.Exit(1)
		}
		slog.Info("schema validated")
	case "skip":
		// leave the schema untouched
	default: // auto
		version, applied, err := migrations.Apply(ctx, cfg.DatabaseURL)
		if err != nil {
			slog.Error("migration failed", "error", err)
			os.Exit(1)
		}
		slog.Info("migrations applied", "version", version, "applied", applied)
	}

	if *migrateOnly {
		slog.Info("migration run complete")
		os.Exit(0)
	}

	sessions, err := session.NewStore(cfg.RedisURL, cfg.SessionTTL)
	if err != nil {
		slog.Error("redis init failed", "error", err)
		os.Exit(1)
	}
	defer sessions.Close()

	var s3 handler.S3Repository
	if cfg.S3Endpoint != "" {
		c, err := store.NewS3Client(cfg.S3Endpoint, cfg.S3AccessKey, cfg.S3SecretKey, cfg.S3Bucket, cfg.S3Region, cfg.S3UseSSL)
		if err != nil {
			slog.Error("s3 init failed", "error", err)
			os.Exit(1)
		}
		if err := c.BucketExists(ctx); err != nil {
			slog.Error("s3 bucket check failed", "error", err)
			os.Exit(1)
		}
		s3 = c
	}

	j := jwt.New(cfg.JWTSecret, cfg.SessionTTL)

	adminStore := store.NewAdminStore(pool)
	if len(cfg.Admins) > 0 {
		if err := adminStore.SeedAdmins(ctx, cfg.Admins); err != nil {
			slog.Error("admin seeding failed", "error", err)
			os.Exit(1)
		}
		slog.Info("admins seeded", "count", len(cfg.Admins))
	}

	discordClient := discord.NewClient(&discord.Config{
		ClientID:     cfg.DiscordClientID,
		ClientSecret: cfg.DiscordSecret,
		RedirectURL:  cfg.RedirectURL,
		BaseURL:      cfg.DiscordBaseURL,
	})

	twitchClient := twitch.NewClient(&twitch.Config{
		ClientID:     cfg.TwitchClientID,
		ClientSecret: cfg.TwitchSecret,
		RedirectURL:  cfg.TwitchRedirectURL,
		OAuthURL:     cfg.TwitchOAuthBaseURL,
		HelixURL:     cfg.TwitchHelixBaseURL,
	})

	handler := app.Build(app.Deps{
		Cfg:      cfg,
		DB:       pool,
		Sessions: sessions,
		JWT:      j,
		Discord:  discordClient,
		Twitch:   twitchClient,
		S3:       s3,
	})

	srv := &http.Server{
		Addr:         ":" + fmt.Sprintf("%d", cfg.Port),
		Handler:      handler,
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

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown error", "error", err)
		os.Exit(1)
	}
	slog.Info("server stopped")
}
