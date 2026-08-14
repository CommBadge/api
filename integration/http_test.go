//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kronus.dev/commbadge_api/config"
	"kronus.dev/commbadge_api/discord"
	"kronus.dev/commbadge_api/internal/app"
	"kronus.dev/commbadge_api/internal/testutil"
	"kronus.dev/commbadge_api/jwt"
	"kronus.dev/commbadge_api/store"
)

type testServer struct {
	ts      *httptest.Server
	discord *testutil.MockDiscordClient
	twitch  *testutil.MockTwitchClient
}

func newTestServer(t *testing.T) *testServer {
	t.Helper()
	requireDB(t)
	requireRedis(t)

	cfg := &config.Config{
		DatabaseURL:       pgDSN,
		RedisURL:          redisURL,
		DiscordClientID:   "discord-client",
		DiscordSecret:     "discord-secret",
		TwitchClientID:    "twitch-client",
		TwitchSecret:      "twitch-secret",
		TwitchRedirectURL: "http://api.test/auth/twitch/callback",
		EventSubCallback:  "http://api.test/eventsub",
		EventSubSecret:    "eventsub-secret",
		RedirectURL:       "http://api.test/auth/callback",
		FrontendURL:       "http://frontend.test",
		JWTSecret:         "test-jwt-secret",
		SessionTTL:        time.Hour,
		CORSOrigins:       []string{"http://frontend.test"},
		RateLimitRPS:      1000000,
		RateLimitBurst:    1000000,
		MaintenanceFile:   filepath.Join(t.TempDir(), "maintenance"),
	}

	sessions := newSessionStore(t, cfg.SessionTTL)
	discordClient := testutil.NewMockDiscordClient()
	twitchClient := testutil.NewMockTwitchClient()
	s3 := testutil.NewMockS3Client()
	signer := jwt.New(cfg.JWTSecret, cfg.SessionTTL)

	h := app.Build(app.Deps{
		Cfg:      cfg,
		DB:       pgPool,
		Sessions: sessions,
		JWT:      signer,
		Discord:  discordClient,
		Twitch:   twitchClient,
		S3:       s3,
	})

	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)
	t.Cleanup(func() { truncateAll(t) })

	return &testServer{ts: ts, discord: discordClient, twitch: twitchClient}
}

func newClient(t *testing.T) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	return &http.Client{
		Jar: jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func (s *testServer) get(t *testing.T, client *http.Client, path string) *http.Response {
	t.Helper()
	resp, err := client.Get(s.ts.URL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

func (s *testServer) post(t *testing.T, client *http.Client, path, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, s.ts.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

func (s *testServer) del(t *testing.T, client *http.Client, path string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodDelete, s.ts.URL+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("DELETE %s: %v", path, err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

// login performs the Discord OAuth flow against the fake provider and returns
// a client with the session cookie.
func (s *testServer) login(t *testing.T) *http.Client {
	t.Helper()
	client := newClient(t)

	resp := s.get(t, client, "/auth/login")
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("expected login redirect, got %d", resp.StatusCode)
	}
	loc, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	state := loc.Query().Get("state")
	if state == "" {
		t.Fatal("expected state param in login redirect")
	}

	resp = s.get(t, client, "/auth/callback?code=test-code&state="+url.QueryEscape(state))
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("expected callback redirect, got %d", resp.StatusCode)
	}
	return client
}

func (s *testServer) loginAs(t *testing.T, user *discord.User) *http.Client {
	t.Helper()
	s.discord.GetUserResult = user
	return s.login(t)
}

func decodeWhoami(t *testing.T, resp *http.Response) map[string]interface{} {
	t.Helper()
	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	return body
}

func TestHTTP_Healthz(t *testing.T) {
	s := newTestServer(t)
	resp := s.get(t, newClient(t), "/healthz")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestHTTP_LoginRedirectsToDiscord(t *testing.T) {
	s := newTestServer(t)
	resp := s.get(t, newClient(t), "/auth/login")
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("expected 302, got %d", resp.StatusCode)
	}
	loc := resp.Header.Get("Location")
	if !strings.Contains(loc, "discord.com/oauth2/authorize") {
		t.Fatalf("expected discord authorize URL, got %s", loc)
	}
}

func TestHTTP_Unauthenticated(t *testing.T) {
	s := newTestServer(t)
	client := newClient(t)
	for _, path := range []string{
		"/auth/whoami",
		"/api/communities",
		"/api/tickets",
		"/api/notifications/subscriptions",
		"/auth/twitch/link",
	} {
		resp := s.get(t, client, path)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected 401 for %s, got %d", path, resp.StatusCode)
		}
	}
}

func TestHTTP_FullLoginFlow(t *testing.T) {
	s := newTestServer(t)
	client := s.login(t)

	resp := s.get(t, client, "/auth/whoami")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, readBody(resp))
	}
	body := decodeWhoami(t, resp)
	if body["id"] != "user-1" || body["username"] != "testuser" {
		t.Fatalf("unexpected whoami: %+v", body)
	}
	if body["twitch_linked"] != false {
		t.Fatalf("expected twitch_linked=false, got %v", body["twitch_linked"])
	}

	resp = s.get(t, client, "/api/communities")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for communities, got %d", resp.StatusCode)
	}
	var communities []interface{}
	if err := json.NewDecoder(resp.Body).Decode(&communities); err != nil {
		t.Fatal(err)
	}
	if len(communities) != 0 {
		t.Fatalf("expected no communities, got %d", len(communities))
	}

	resp = s.post(t, client, "/api/communities", `{"name":"Test Guild","description":"desc"}`)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 creating community, got %d: %s", resp.StatusCode, readBody(resp))
	}
	var created map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	communityID, ok := created["id"].(string)
	if !ok || communityID == "" {
		t.Fatalf("expected community id, got %+v", created)
	}

	resp = s.get(t, client, "/api/communities")
	if resp.StatusCode != http.StatusOK {
		t.Fatal("expected 200 listing communities")
	}
	if err := json.NewDecoder(resp.Body).Decode(&communities); err != nil {
		t.Fatal(err)
	}
	if len(communities) != 1 {
		t.Fatalf("expected 1 community, got %d", len(communities))
	}
}

func TestHTTP_Logout(t *testing.T) {
	s := newTestServer(t)
	client := s.login(t)

	resp := s.post(t, client, "/auth/logout", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	resp = s.get(t, client, "/auth/whoami")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 after logout, got %d", resp.StatusCode)
	}
}

func TestHTTP_TwitchLinkAndNotifications(t *testing.T) {
	s := newTestServer(t)
	client := s.login(t)

	// Link twitch account.
	resp := s.get(t, client, "/auth/twitch/link")
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("expected twitch link redirect, got %d", resp.StatusCode)
	}
	loc, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	state := loc.Query().Get("state")
	if state == "" {
		t.Fatal("expected state in twitch link redirect")
	}

	resp = s.get(t, client, "/auth/twitch/callback?code=test-code&state="+url.QueryEscape(state))
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("expected twitch callback redirect, got %d", resp.StatusCode)
	}

	resp = s.get(t, client, "/auth/whoami")
	body := decodeWhoami(t, resp)
	if body["twitch_linked"] != true {
		t.Fatalf("expected twitch_linked=true, got %v", body["twitch_linked"])
	}

	// Subscribe.
	resp = s.post(t, client, "/api/notifications/subscriptions", `{"event_type":"stream.online"}`)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 subscribing, got %d: %s", resp.StatusCode, readBody(resp))
	}
	var sub store.NotificationSubscription
	if err := json.NewDecoder(resp.Body).Decode(&sub); err != nil {
		t.Fatal(err)
	}
	if sub.ID == "" || sub.TwitchSubscriptionID != "eventsub-1" {
		t.Fatalf("unexpected subscription: %+v", sub)
	}
	if s.twitch.CreateSubCallCount != 1 {
		t.Fatalf("expected 1 eventsub create, got %d", s.twitch.CreateSubCallCount)
	}

	// Duplicate subscribe is rejected.
	resp = s.post(t, client, "/api/notifications/subscriptions", `{"event_type":"stream.online"}`)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 duplicate subscribe, got %d", resp.StatusCode)
	}

	// List shows one subscription.
	resp = s.get(t, client, "/api/notifications/subscriptions")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 listing, got %d", resp.StatusCode)
	}
	var subs []store.NotificationSubscription
	if err := json.NewDecoder(resp.Body).Decode(&subs); err != nil {
		t.Fatal(err)
	}
	if len(subs) != 1 {
		t.Fatalf("expected 1 subscription, got %d", len(subs))
	}

	// Delete subscription.
	resp = s.del(t, client, "/api/notifications/subscriptions/"+sub.ID)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 deleting, got %d: %s", resp.StatusCode, readBody(resp))
	}
	if s.twitch.DeleteSubCallCount != 1 {
		t.Fatalf("expected 1 eventsub delete, got %d", s.twitch.DeleteSubCallCount)
	}

	// Unlink twitch account.
	resp = s.post(t, client, "/auth/twitch/unlink", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 unlink, got %d: %s", resp.StatusCode, readBody(resp))
	}
	resp = s.get(t, client, "/auth/whoami")
	body = decodeWhoami(t, resp)
	if body["twitch_linked"] != false {
		t.Fatalf("expected twitch_linked=false, got %v", body["twitch_linked"])
	}

	// Cannot subscribe without a linked twitch account.
	resp = s.post(t, client, "/api/notifications/subscriptions", `{"event_type":"stream.offline"}`)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 subscribe without twitch, got %d", resp.StatusCode)
	}
}

func TestHTTP_SubscribeRequiresTwitchLink(t *testing.T) {
	s := newTestServer(t)
	client := s.login(t)

	resp := s.post(t, client, "/api/notifications/subscriptions", `{"event_type":"stream.online"}`)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", resp.StatusCode, readBody(resp))
	}
}

func TestHTTP_InvalidEventType(t *testing.T) {
	s := newTestServer(t)
	client := s.login(t)
	resp := s.post(t, client, "/api/notifications/subscriptions", `{"event_type":"chat.message"}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestHTTP_AdminFlow(t *testing.T) {
	s := newTestServer(t)
	ctx := context.Background()

	// user-1 is an admin; user-2 is not.
	adminClient := s.login(t)
	userClient := s.loginAs(t, &discord.User{ID: "user-2", Username: "user2", GlobalName: "User Two"})

	if err := store.NewAdminStore(pgPool).SeedAdmins(ctx, []string{"user-1"}); err != nil {
		t.Fatalf("SeedAdmins: %v", err)
	}

	resp := s.get(t, adminClient, "/api/admin/users")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for admin, got %d: %s", resp.StatusCode, readBody(resp))
	}
	var users []interface{}
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		t.Fatal(err)
	}

	resp = s.get(t, userClient, "/api/admin/users")
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin, got %d", resp.StatusCode)
	}
}

func TestHTTP_BannedUser(t *testing.T) {
	s := newTestServer(t)
	ctx := context.Background()

	client := s.login(t)

	if err := store.NewUserStore(pgPool).BanUser(ctx, "user-1", "spam"); err != nil {
		t.Fatal(err)
	}

	resp := s.get(t, client, "/auth/whoami")
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for banned user, got %d", resp.StatusCode)
	}

	resp = s.get(t, client, "/api/communities")
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for banned user on api, got %d", resp.StatusCode)
	}
}

func readBody(resp *http.Response) string {
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "<read error>"
	}
	return string(data)
}
