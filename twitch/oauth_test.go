package twitch

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestAuthURL_IncludesPKCEAndNonce(t *testing.T) {
	c := NewClient(&Config{ClientID: "cid", ClientSecret: "secret", RedirectURL: "http://localhost:8080/auth/twitch/callback"})
	u, err := url.Parse(c.AuthURL("state-1", "challenge-1"))
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if q.Get("code_challenge") != "challenge-1" {
		t.Fatalf("expected code_challenge, got %q", q.Get("code_challenge"))
	}
	if q.Get("code_challenge_method") != "S256" {
		t.Fatalf("expected S256 method, got %q", q.Get("code_challenge_method"))
	}
	if q.Get("nonce") != "state-1" {
		t.Fatalf("expected nonce=state, got %q", q.Get("nonce"))
	}
	if q.Get("state") != "state-1" {
		t.Fatalf("expected state, got %q", q.Get("state"))
	}
}

func TestExchange_SendsCodeVerifier(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"access_token":"tok"}`))
	}))
	defer srv.Close()

	c := NewClient(&Config{ClientID: "cid", ClientSecret: "secret", RedirectURL: "http://localhost:8080/auth/twitch/callback", OAuthURL: srv.URL})
	if _, err := c.Exchange(context.Background(), "the-code", "the-verifier"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gotBody, "code=the-code") {
		t.Fatalf("expected code in body, got %q", gotBody)
	}
	if !strings.Contains(gotBody, "code_verifier=the-verifier") {
		t.Fatalf("expected code_verifier in body, got %q", gotBody)
	}
}
