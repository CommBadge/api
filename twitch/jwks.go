package twitch

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// idTokenClaims carries the OpenID Connect claims the API relies on when
// validating the Twitch ID token.
type idTokenClaims struct {
	Nonce string `json:"nonce"`
	jwt.RegisteredClaims
}

// jwksCache holds Twitch's public signing keys, refreshed at most hourly.
type jwksCache struct {
	mu      sync.Mutex
	keys    map[string]*rsa.PublicKey
	fetched time.Time
}

type jwksSet struct {
	Keys []jwk `json:"keys"`
}

type jwk struct {
	Kty string `json:"kty"`
	Use string `json:"use"`
	Kid string `json:"kid"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// VerifyIDToken validates the OIDC ID token returned by the token endpoint
// and checks that its nonce matches the value sent in the authorization
// request. This binds the token to the login attempt that the API started
// (replay protection) and proves it was issued to this client.
func (c *Client) VerifyIDToken(ctx context.Context, idToken, nonce string) error {
	if idToken == "" {
		return fmt.Errorf("no id token returned")
	}
	if nonce == "" {
		return fmt.Errorf("missing nonce")
	}

	keys, err := c.jwksFor(ctx)
	if err != nil {
		return fmt.Errorf("fetch jwks: %w", err)
	}

	token, err := jwt.ParseWithClaims(idToken, &idTokenClaims{}, func(t *jwt.Token) (interface{}, error) {
		kid, _ := t.Header["kid"].(string)
		key, ok := keys[kid]
		if !ok {
			return nil, fmt.Errorf("unknown key id %q", kid)
		}
		return key, nil
	},
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithIssuer(c.cfg.OAuthURL+"/oauth2"),
		jwt.WithAudience(c.cfg.ClientID),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return fmt.Errorf("invalid id token: %w", err)
	}

	claims, ok := token.Claims.(*idTokenClaims)
	if !ok {
		return fmt.Errorf("invalid id token claims")
	}
	if claims.Nonce != nonce {
		return fmt.Errorf("id token nonce mismatch")
	}
	return nil
}

// jwksFor returns Twitch's public signing keys, refreshing the cached copy if
// it is absent or older than an hour.
func (c *Client) jwksFor(ctx context.Context) (map[string]*rsa.PublicKey, error) {
	c.jwks.mu.Lock()
	defer c.jwks.mu.Unlock()

	if c.jwks.keys != nil && time.Since(c.jwks.fetched) < time.Hour {
		return c.jwks.keys, nil
	}

	req, err := http.NewRequestWithContext(ctx, "GET", c.cfg.OAuthURL+"/oauth2/keys", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpCli.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jwks request failed: %s", resp.Status)
	}

	var set jwksSet
	if err := json.NewDecoder(resp.Body).Decode(&set); err != nil {
		return nil, err
	}
	if len(set.Keys) == 0 {
		return nil, fmt.Errorf("jwks contains no keys")
	}

	keys := make(map[string]*rsa.PublicKey, len(set.Keys))
	for _, k := range set.Keys {
		pub, err := parseJWK(k)
		if err != nil {
			return nil, err
		}
		keys[k.Kid] = pub
	}
	c.jwks.keys = keys
	c.jwks.fetched = time.Now()
	return keys, nil
}

func parseJWK(k jwk) (*rsa.PublicKey, error) {
	if k.Kty != "RSA" {
		return nil, fmt.Errorf("unsupported jwk key type %q", k.Kty)
	}
	if k.Use != "" && k.Use != "sig" {
		return nil, fmt.Errorf("unsupported jwk use %q", k.Use)
	}
	if k.Alg != "" && k.Alg != "RS256" {
		return nil, fmt.Errorf("unsupported jwk alg %q", k.Alg)
	}

	nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil {
		return nil, fmt.Errorf("decode n: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil {
		return nil, fmt.Errorf("decode e: %w", err)
	}
	e := 0
	for _, b := range eBytes {
		e = e<<8 | int(b)
	}
	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(nBytes),
		E: e,
	}, nil
}
