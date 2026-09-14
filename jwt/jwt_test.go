package jwt

import (
	"crypto/rand"
	"crypto/rsa"
	"strings"
	"testing"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v5"
)

func TestSignVerifyRoundtrip(t *testing.T) {
	j := New("supersecret", time.Hour)
	token, err := j.Sign("session-123")
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if token == "" {
		t.Fatal("empty token")
	}
	got, err := j.Verify(token)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if got != "session-123" {
		t.Fatalf("session id = %q", got)
	}
}

func TestVerify_WrongSecret(t *testing.T) {
	token, _ := New("secret-a", time.Hour).Sign("sid")
	if _, err := New("secret-b", time.Hour).Verify(token); err == nil {
		t.Fatal("expected error for wrong secret")
	}
}

func TestVerify_NonHMAC(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	token := jwtlib.NewWithClaims(jwtlib.SigningMethodRS256, Claims{
		SessionID: "sid",
		RegisteredClaims: jwtlib.RegisteredClaims{
			ExpiresAt: jwtlib.NewNumericDate(time.Now().Add(time.Hour)),
		},
	})
	signed, err := token.SignedString(key)
	if err != nil {
		t.Fatalf("SignedString: %v", err)
	}
	if _, err := New("secret", time.Hour).Verify(signed); err == nil {
		t.Fatal("expected error for non-HMAC token")
	}
}

func TestVerify_Malformed(t *testing.T) {
	if _, err := New("secret", time.Hour).Verify("not.a.jwt"); err == nil {
		t.Fatal("expected error for malformed token")
	}
}

func TestVerify_Expired(t *testing.T) {
	claims := Claims{
		SessionID: "sid",
		RegisteredClaims: jwtlib.RegisteredClaims{
			ExpiresAt: jwtlib.NewNumericDate(time.Now().Add(-time.Hour)),
		},
	}
	token := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte("secret"))
	if err != nil {
		t.Fatalf("SignedString: %v", err)
	}
	if _, err := New("secret", time.Hour).Verify(signed); err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("expected expired error, got %v", err)
	}
}
