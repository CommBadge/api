package auth

import (
	"crypto/sha256"
	"encoding/base64"
	"regexp"
	"testing"
)

// verifierCharset is the unreserved character set required by RFC 7636 §4.1.
var verifierCharset = regexp.MustCompile(`^[A-Za-z0-9\-._~]+$`)

func TestGeneratePKCEVerifier(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 20; i++ {
		v, err := generatePKCEVerifier()
		if err != nil {
			t.Fatalf("generatePKCEVerifier: %v", err)
		}
		if len(v) < 43 || len(v) > 128 {
			t.Fatalf("verifier length %d outside 43..128", len(v))
		}
		if !verifierCharset.MatchString(v) {
			t.Fatalf("verifier %q contains characters outside the RFC 7636 charset", v)
		}
		if seen[v] {
			t.Fatalf("verifier repeated: %q", v)
		}
		seen[v] = true
	}
}

func TestPKSES256Challenge(t *testing.T) {
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	want := sha256.Sum256([]byte(verifier))
	if got := pkceS256Challenge(verifier); got != base64.RawURLEncoding.EncodeToString(want[:]) {
		t.Fatalf("expected %s, got %s", base64.RawURLEncoding.EncodeToString(want[:]), got)
	}
}

func TestPKCES256ChallengeDiffersForDifferentVerifiers(t *testing.T) {
	a, err := generatePKCEVerifier()
	if err != nil {
		t.Fatal(err)
	}
	b, err := generatePKCEVerifier()
	if err != nil {
		t.Fatal(err)
	}
	if pkceS256Challenge(a) == pkceS256Challenge(b) {
		t.Fatal("challenges for distinct verifiers must differ")
	}
}