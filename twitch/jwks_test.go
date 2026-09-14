package twitch

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// newJWTTestClient starts a fake Twitch JWKS endpoint and returns a Client
// pointed at it. The returned handler reports how many times the JWKS was
// fetched (for cache assertions).
func newJWTTestClient(t *testing.T, key *rsa.PublicKey, kid string, fetchCount *int) (*Client, string) {
	t.Helper()

	jwksHandler := func(w http.ResponseWriter, r *http.Request) {
		*fetchCount++
		n := base64.RawURLEncoding.EncodeToString(key.N.Bytes())
		e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes())
		_ = json.NewEncoder(w).Encode(jwksSet{Keys: []jwk{
			{Kty: "RSA", Use: "sig", Kid: kid, Alg: "RS256", N: n, E: e},
		}})
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/oauth2/keys", jwksHandler)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	// Point the client at the fake JWKS server; the issuer is derived from the
	// same base URL, mirroring how the real Twitch endpoints are derived.
	issuer := srv.URL + "/oauth2"
	c := NewClient(&Config{
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		OAuthURL:     srv.URL,
		HelixURL:     "https://api.twitch.tv",
	})
	c.httpCli = srv.Client()
	return c, issuer
}

func signIDToken(t *testing.T, key *rsa.PrivateKey, kid, issuer, audience, nonce string, exp time.Time) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, &idTokenClaims{
		Nonce: nonce,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Audience:  jwt.ClaimStrings{audience},
			Subject:   "12345678",
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(exp),
		},
	})
	tok.Header["kid"] = kid
	signed, err := tok.SignedString(key)
	if err != nil {
		t.Fatalf("sign id token: %v", err)
	}
	return signed
}

func TestVerifyIDToken_Success(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	var fetches int
	c, issuer := newJWTTestClient(t, &key.PublicKey, "test-key", &fetches)

	token := signIDToken(t, key, "test-key", issuer, "test-client", "abc123", time.Now().Add(time.Hour))
	if err := c.VerifyIDToken(context.Background(), token, "abc123"); err != nil {
		t.Fatalf("expected valid token, got %v", err)
	}

	// A second verification must reuse the cached keys.
	if err := c.VerifyIDToken(context.Background(), token, "abc123"); err != nil {
		t.Fatalf("expected valid token on second verify, got %v", err)
	}
	if fetches != 1 {
		t.Fatalf("expected 1 jwks fetch, got %d", fetches)
	}
}

func TestVerifyIDToken_NonceMismatch(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	var fetches int
	c, issuer := newJWTTestClient(t, &key.PublicKey, "test-key", &fetches)

	token := signIDToken(t, key, "test-key", issuer, "test-client", "expected-nonce", time.Now().Add(time.Hour))
	if err := c.VerifyIDToken(context.Background(), token, "other-nonce"); err == nil {
		t.Fatal("expected nonce mismatch error")
	}
}

func TestVerifyIDToken_UnknownKey(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	otherKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	var fetches int
	c, issuer := newJWTTestClient(t, &key.PublicKey, "test-key", &fetches)

	token := signIDToken(t, otherKey, "other-key", issuer, "test-client", "abc123", time.Now().Add(time.Hour))
	if err := c.VerifyIDToken(context.Background(), token, "abc123"); err == nil {
		t.Fatal("expected unknown key error")
	}
}

func TestVerifyIDToken_Expired(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	var fetches int
	c, issuer := newJWTTestClient(t, &key.PublicKey, "test-key", &fetches)

	token := signIDToken(t, key, "test-key", issuer, "test-client", "abc123", time.Now().Add(-time.Hour))
	if err := c.VerifyIDToken(context.Background(), token, "abc123"); err == nil {
		t.Fatal("expected expired token error")
	}
}

func TestVerifyIDToken_WrongAudience(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	var fetches int
	c, issuer := newJWTTestClient(t, &key.PublicKey, "test-key", &fetches)

	token := signIDToken(t, key, "test-key", issuer, "some-other-client", "abc123", time.Now().Add(time.Hour))
	if err := c.VerifyIDToken(context.Background(), token, "abc123"); err == nil {
		t.Fatal("expected audience error")
	}
}

func TestVerifyIDToken_EmptyToken(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	var fetches int
	c, _ := newJWTTestClient(t, &key.PublicKey, "test-key", &fetches)

	if err := c.VerifyIDToken(context.Background(), "", "abc123"); err == nil {
		t.Fatal("expected error for empty id token")
	}
}
