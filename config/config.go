package config

import (
	"errors"
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
	MaintenanceFile    string
	MigrationMode      string
	Admins             []string
}

func Load() (*Config, error) {
	portStr := os.Getenv("COMMBADGE_API_PORT")
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		return nil, err
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
		MaintenanceFile:    maintFile,
		MigrationMode:      migrationMode,
		Admins:             admins,
	}, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
