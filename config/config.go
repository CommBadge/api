package config

import (
	"errors"
	"fmt"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port               int
	DatabaseURL        string
	RedisURL           string
	DiscordClientID    string
	DiscordSecret      string
	DiscordBotToken    string
	DiscordBaseURL     string
	TwitchClientID     string
	TwitchSecret       string
	TwitchRedirectURL  string
	TwitchOAuthBaseURL string
	TwitchHelixBaseURL string
	EventSubCallback   string
	EventSubSecret     string
	RedirectURL        string
	FrontendURL        string
	JWTSecret          string
	SessionTTL         time.Duration
	TLS                bool
	CookieSecure       bool
	TLSCert            string
	TLSKey             string
	S3Endpoint         string
	S3AccessKey        string
	S3SecretKey        string
	S3Bucket           string
	S3Region           string
	S3UseSSL           bool
	CORSOrigins        []string
	RateLimitRPS       int
	RateLimitBurst     int
	TrustedProxies     []string
	MaintenanceFile    string
	MigrationMode      string
	Admins             []string
	MetricsUser        string
	MetricsPassword    string
}

func Load() (*Config, error) {
	portStr := os.Getenv("COMMBADGE_API_PORT")
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		return nil, fmt.Errorf("invalid COMMBADGE_API_PORT %q: must be an integer between 1 and 65535", portStr)
	}

	sessionTTL := 24 * time.Hour
	if v := os.Getenv("COMMBADGE_API_SESSION_TTL"); v != "" {
		sessionTTL, err = time.ParseDuration(v)
		if err != nil {
			return nil, err
		}
	}

	rps := 50
	if v := os.Getenv("COMMBADGE_API_RATE_LIMIT_RPS"); v != "" {
		rps, err = strconv.Atoi(v)
		if err != nil {
			return nil, err
		}
	}

	burst := 100
	if v := os.Getenv("COMMBADGE_API_RATE_LIMIT_BURST"); v != "" {
		burst, err = strconv.Atoi(v)
		if err != nil {
			return nil, err
		}
	}

	maintFile := "/tmp/commbadge-maintenance"
	if v := os.Getenv("COMMBADGE_API_MAINTENANCE_FILE"); v != "" {
		maintFile = v
	}

	migrationMode := "auto"
	if v := os.Getenv("COMMBADGE_API_MIGRATIONS"); v != "" {
		switch v {
		case "auto", "validate", "skip":
			migrationMode = v
		default:
			return nil, errors.New(`COMMBADGE_API_MIGRATIONS must be one of "auto", "validate", or "skip"`)
		}
	}

	var corsOrigins []string
	if v := os.Getenv("COMMBADGE_API_CORS_ORIGINS"); v != "" {
		corsOrigins = strings.Split(v, ",")
	}

	var trustedProxies []string
	if v := os.Getenv("COMMBADGE_API_TRUSTED_PROXIES"); v != "" {
		for _, p := range strings.Split(v, ",") {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			if _, err := netip.ParsePrefix(p); err != nil {
				return nil, fmt.Errorf("invalid COMMBADGE_API_TRUSTED_PROXIES entry %q: %w", p, err)
			}
			trustedProxies = append(trustedProxies, p)
		}
	}

	// Secure cookies are the safe default: without TLS the session and
	// oauth_state cookies would be transmitted in plaintext. Set
	// COMMBADGE_API_COOKIE_SECURE=false explicitly for local HTTP testing.
	cookieSecure := true
	if v := os.Getenv("COMMBADGE_API_COOKIE_SECURE"); v != "" {
		cookieSecure, err = strconv.ParseBool(v)
		if err != nil {
			return nil, fmt.Errorf("invalid COMMBADGE_API_COOKIE_SECURE: %w", err)
		}
	}

	metricsUser := os.Getenv("COMMBADGE_API_METRICS_USER")
	metricsPassword := os.Getenv("COMMBADGE_API_METRICS_PASSWORD")
	if (metricsUser == "") != (metricsPassword == "") {
		return nil, fmt.Errorf("COMMBADGE_API_METRICS_USER and COMMBADGE_API_METRICS_PASSWORD must be set together")
	}

	var admins []string
	if v := os.Getenv("COMMBADGE_API_ADMINS"); v != "" {
		admins = strings.Split(v, ",")
	}

	return &Config{
		Port:               port,
		DatabaseURL:        os.Getenv("COMMBADGE_API_DATABASE_URL"),
		RedisURL:           os.Getenv("COMMBADGE_API_REDIS_URL"),
		DiscordClientID:    os.Getenv("COMMBADGE_API_DISCORD_CLIENT_ID"),
		DiscordSecret:      os.Getenv("COMMBADGE_API_DISCORD_CLIENT_SECRET"),
		DiscordBotToken:    os.Getenv("COMMBADGE_API_DISCORD_BOT_TOKEN"),
		DiscordBaseURL:     getenv("COMMBADGE_API_DISCORD_BASE_URL", "https://discord.com"),
		TwitchClientID:     os.Getenv("COMMBADGE_API_TWITCH_CLIENT_ID"),
		TwitchSecret:       os.Getenv("COMMBADGE_API_TWITCH_CLIENT_SECRET"),
		TwitchRedirectURL:  os.Getenv("COMMBADGE_API_TWITCH_REDIRECT_URL"),
		TwitchOAuthBaseURL: getenv("COMMBADGE_API_TWITCH_OAUTH_BASE_URL", "https://id.twitch.tv"),
		TwitchHelixBaseURL: getenv("COMMBADGE_API_TWITCH_HELIX_BASE_URL", "https://api.twitch.tv"),
		EventSubCallback:   os.Getenv("COMMBADGE_API_EVENTSUB_CALLBACK_URL"),
		EventSubSecret:     os.Getenv("COMMBADGE_API_EVENTSUB_SECRET"),
		RedirectURL:        os.Getenv("COMMBADGE_API_REDIRECT_URL"),
		FrontendURL:        os.Getenv("COMMBADGE_API_FRONTEND_URL"),
		JWTSecret:          os.Getenv("COMMBADGE_API_JWT_SECRET"),
		SessionTTL:         sessionTTL,
		TLS:                os.Getenv("COMMBADGE_API_TLS") == "true",
		CookieSecure:       cookieSecure,
		TLSCert:            os.Getenv("COMMBADGE_API_CERT"),
		TLSKey:             os.Getenv("COMMBADGE_API_KEY"),
		S3Endpoint:         os.Getenv("COMMBADGE_API_S3_ENDPOINT"),
		S3AccessKey:        os.Getenv("COMMBADGE_API_S3_ACCESS_KEY"),
		S3SecretKey:        os.Getenv("COMMBADGE_API_S3_SECRET_KEY"),
		S3Bucket:           os.Getenv("COMMBADGE_API_S3_BUCKET"),
		S3Region:           os.Getenv("COMMBADGE_API_S3_REGION"),
		S3UseSSL:           os.Getenv("COMMBADGE_API_S3_USE_SSL") == "true",
		CORSOrigins:        corsOrigins,
		RateLimitRPS:       rps,
		RateLimitBurst:     burst,
		TrustedProxies:     trustedProxies,
		MaintenanceFile:    maintFile,
		MigrationMode:      migrationMode,
		Admins:             admins,
		MetricsUser:        metricsUser,
		MetricsPassword:    metricsPassword,
	}, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
