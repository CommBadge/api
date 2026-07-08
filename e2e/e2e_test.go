//go:build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

var (
	baseURL   string
	httpCli   *http.Client
)

func setup() {
	baseURL = os.Getenv("COMMBADGE_E2E_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	httpCli = &http.Client{Timeout: 15 * time.Second}
}

func request(method, path, body string) (*http.Response, error) {
	req, err := http.NewRequest(method, baseURL+path, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return httpCli.Do(req)
}

func TestE2E_Healthz(t *testing.T) {
	setup()
	resp, err := request("GET", "/healthz", "")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	t.Logf("health: %+v", body)
}

func TestE2E_Root(t *testing.T) {
	setup()
	resp, err := request("GET", "/", "")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestE2E_Metrics(t *testing.T) {
	setup()
	resp, err := request("GET", "/metrics", "")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestE2E_AuthRedirect(t *testing.T) {
	setup()
	resp, err := request("GET", "/auth/login", "")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusFound {
		t.Fatalf("expected 302, got %d", resp.StatusCode)
	}
	loc := resp.Header.Get("Location")
	if !strings.Contains(loc, "id.twitch.tv") {
		t.Fatalf("expected twitch redirect, got %s", loc)
	}
}

func TestE2E_Unauthenticated(t *testing.T) {
	setup()
	tests := []struct {
		path string
		code int
	}{
		{"/auth/whoami", http.StatusUnauthorized},
		{"/api/communities", http.StatusUnauthorized},
		{"/api/tickets", http.StatusUnauthorized},
		{"/api/admin/users", http.StatusUnauthorized},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			resp, err := request("GET", tt.path, "")
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			if resp.StatusCode != tt.code {
				t.Fatalf("expected status %d for %s, got %d", tt.code, tt.path, resp.StatusCode)
			}
		})
	}
}

func TestE2E_NotFound(t *testing.T) {
	setup()
	resp, err := request("GET", "/nonexistent-route", "")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 (catch-all), got %d", resp.StatusCode)
	}
}

func TestE2E_AdminTicketCount(t *testing.T) {
	setup()
	resp, err := request("GET", "/admin/tickets/count", "")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized && resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: %d", resp.StatusCode)
	}

	if resp.StatusCode == http.StatusOK {
		var body map[string]int
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		t.Logf("ticket count: %d", body["count"])
	}
}

func TestE2E_SetupAsAdmin(t *testing.T) {
	// Helper test to verify the app is configured correctly.
	// This test always passes and logs the environment.
	setup()

	clientID := os.Getenv("COMMBADGE_API_TWITCH_CLIENT_ID")
	if clientID == "" {
		clientID = "(not set)"
	}

	t.Logf("E2E test against: %s", baseURL)
	t.Logf("Twitch client ID: %s", clientID)

	fmt.Printf("To run e2e tests against a Kubernetes cluster:\n")
	fmt.Printf("  export COMMBADGE_E2E_BASE_URL=https://api.yourdomain.com\n")
	fmt.Printf("  go test -tags=e2e ./e2e/... -v\n")
}
