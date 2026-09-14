package config

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestLoad_Defaults(t *testing.T) {
	clearEnv(t)
	t.Setenv("COMMBADGE_API_PORT", "8080")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != 8080 {
		t.Errorf("Port = %d", cfg.Port)
	}
	if cfg.SessionTTL != 24*time.Hour {
		t.Errorf("SessionTTL = %v", cfg.SessionTTL)
	}
	if cfg.RateLimitRPS != 50 || cfg.RateLimitBurst != 100 {
		t.Errorf("rate limit = %d/%d", cfg.RateLimitRPS, cfg.RateLimitBurst)
	}
	if cfg.MaintenanceFile != "/tmp/commbadge-maintenance" {
		t.Errorf("MaintenanceFile = %q", cfg.MaintenanceFile)
	}
	if cfg.MigrationMode != "auto" {
		t.Errorf("MigrationMode = %q", cfg.MigrationMode)
	}
	if !cfg.CookieSecure {
		t.Error("CookieSecure should default to true")
	}
	if cfg.TLS || cfg.S3UseSSL {
		t.Error("TLS/S3UseSSL should default to false")
	}
	if cfg.DiscordBaseURL != "https://discord.com" {
		t.Errorf("DiscordBaseURL = %q", cfg.DiscordBaseURL)
	}
	if cfg.TwitchOAuthBaseURL != "https://id.twitch.tv" {
		t.Errorf("TwitchOAuthBaseURL = %q", cfg.TwitchOAuthBaseURL)
	}
	if cfg.TwitchHelixBaseURL != "https://api.twitch.tv" {
		t.Errorf("TwitchHelixBaseURL = %q", cfg.TwitchHelixBaseURL)
	}
}

func TestLoad_AllOptions(t *testing.T) {
	clearEnv(t)
	env := map[string]string{
		"COMMBADGE_API_PORT":                  "9090",
		"COMMBADGE_API_SESSION_TTL":           "2h",
		"COMMBADGE_API_RATE_LIMIT_RPS":        "10",
		"COMMBADGE_API_RATE_LIMIT_BURST":      "20",
		"COMMBADGE_API_MAINTENANCE_FILE":      "/var/run/maint",
		"COMMBADGE_API_MIGRATIONS":            "validate",
		"COMMBADGE_API_CORS_ORIGINS":          "https://a.example.com,https://b.example.com",
		"COMMBADGE_API_TRUSTED_PROXIES":       "10.0.0.0/8, 127.0.0.1/32, , 2001:db8::/32",
		"COMMBADGE_API_COOKIE_SECURE":         "false",
		"COMMBADGE_API_METRICS_USER":          "admin",
		"COMMBADGE_API_METRICS_PASSWORD":      "pw",
		"COMMBADGE_API_ADMINS":                "u1,u2",
		"COMMBADGE_API_DATABASE_URL":          "postgres://db",
		"COMMBADGE_API_REDIS_URL":             "redis://r",
		"COMMBADGE_API_DISCORD_BASE_URL":      "http://discord.test",
		"COMMBADGE_API_TWITCH_OAUTH_BASE_URL": "http://oauth.test",
		"COMMBADGE_API_TWITCH_HELIX_BASE_URL": "http://helix.test",
		"COMMBADGE_API_TLS":                   "true",
		"COMMBADGE_API_S3_USE_SSL":            "true",
	}
	for k, v := range env {
		t.Setenv(k, v)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != 9090 || cfg.SessionTTL != 2*time.Hour || cfg.RateLimitRPS != 10 || cfg.RateLimitBurst != 20 {
		t.Errorf("scalar options wrong: %+v", cfg)
	}
	if cfg.MaintenanceFile != "/var/run/maint" || cfg.MigrationMode != "validate" {
		t.Errorf("maintenance/migration wrong: %q %q", cfg.MaintenanceFile, cfg.MigrationMode)
	}
	if len(cfg.CORSOrigins) != 2 || cfg.CORSOrigins[0] != "https://a.example.com" {
		t.Errorf("CORSOrigins = %v", cfg.CORSOrigins)
	}
	if len(cfg.TrustedProxies) != 3 || cfg.TrustedProxies[0] != "10.0.0.0/8" || cfg.TrustedProxies[2] != "2001:db8::/32" {
		t.Errorf("TrustedProxies = %v", cfg.TrustedProxies)
	}
	if cfg.CookieSecure {
		t.Error("CookieSecure should be false")
	}
	if !cfg.TLS || !cfg.S3UseSSL {
		t.Error("TLS/S3UseSSL should be true")
	}
	if cfg.DiscordBaseURL != "http://discord.test" || cfg.TwitchOAuthBaseURL != "http://oauth.test" || cfg.TwitchHelixBaseURL != "http://helix.test" {
		t.Errorf("base URLs wrong: %q %q %q", cfg.DiscordBaseURL, cfg.TwitchOAuthBaseURL, cfg.TwitchHelixBaseURL)
	}
	if len(cfg.Admins) != 2 || cfg.Admins[1] != "u2" {
		t.Errorf("Admins = %v", cfg.Admins)
	}
	if cfg.MetricsUser != "admin" || cfg.MetricsPassword != "pw" {
		t.Errorf("metrics = %q %q", cfg.MetricsUser, cfg.MetricsPassword)
	}
}

func TestLoad_Errors(t *testing.T) {
	t.Run("missing port", func(t *testing.T) {
		clearEnv(t)
		if _, err := Load(); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("non-numeric port", func(t *testing.T) {
		clearEnv(t)
		t.Setenv("COMMBADGE_API_PORT", "abc")
		if _, err := Load(); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("port out of range", func(t *testing.T) {
		clearEnv(t)
		t.Setenv("COMMBADGE_API_PORT", "99999")
		if _, err := Load(); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("bad session ttl", func(t *testing.T) {
		clearEnv(t)
		t.Setenv("COMMBADGE_API_PORT", "8080")
		t.Setenv("COMMBADGE_API_SESSION_TTL", "notaduration")
		if _, err := Load(); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("bad rps", func(t *testing.T) {
		clearEnv(t)
		t.Setenv("COMMBADGE_API_PORT", "8080")
		t.Setenv("COMMBADGE_API_RATE_LIMIT_RPS", "x")
		if _, err := Load(); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("bad burst", func(t *testing.T) {
		clearEnv(t)
		t.Setenv("COMMBADGE_API_PORT", "8080")
		t.Setenv("COMMBADGE_API_RATE_LIMIT_BURST", "x")
		if _, err := Load(); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("bad migration mode", func(t *testing.T) {
		clearEnv(t)
		t.Setenv("COMMBADGE_API_PORT", "8080")
		t.Setenv("COMMBADGE_API_MIGRATIONS", "nope")
		if _, err := Load(); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("bad trusted proxy", func(t *testing.T) {
		clearEnv(t)
		t.Setenv("COMMBADGE_API_PORT", "8080")
		t.Setenv("COMMBADGE_API_TRUSTED_PROXIES", "300.0.0.0/8")
		if _, err := Load(); err == nil || !strings.Contains(err.Error(), "TRUSTED_PROXIES") {
			t.Fatalf("expected trusted proxy error, got %v", err)
		}
	})
	t.Run("bad cookie secure", func(t *testing.T) {
		clearEnv(t)
		t.Setenv("COMMBADGE_API_PORT", "8080")
		t.Setenv("COMMBADGE_API_COOKIE_SECURE", "banana")
		if _, err := Load(); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("metrics mismatch", func(t *testing.T) {
		clearEnv(t)
		t.Setenv("COMMBADGE_API_PORT", "8080")
		t.Setenv("COMMBADGE_API_METRICS_USER", "only-user")
		if _, err := Load(); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestGetenv(t *testing.T) {
	t.Setenv("CB_TEST_SET", "value")
	if got := getenv("CB_TEST_SET", "fallback"); got != "value" {
		t.Errorf("getenv = %q", got)
	}
	if got := getenv("CB_TEST_UNSET", "fallback"); got != "fallback" {
		t.Errorf("getenv fallback = %q", got)
	}
}

// clearEnv removes every COMMBADGE_API_* variable so tests are hermetic.
func clearEnv(t *testing.T) {
	t.Helper()
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "COMMBADGE_API_") {
			key := strings.SplitN(e, "=", 2)[0]
			t.Setenv(key, "")
		}
	}
}
