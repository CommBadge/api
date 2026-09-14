package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
)

// verifierCookieName holds the PKCE code_verifier across the OAuth redirect
// round-trip. Like the state cookie it is HttpOnly and single-use: the
// verifier is cleared once the callback consumes it, so a replayed callback
// cannot reuse it.
const verifierCookieName = "oauth_verifier"

// pkceVerifierBytes is the number of random bytes backing a PKCE verifier
// (RFC 7636 §4.1). 32 bytes produce a 43-character verifier, within the
// allowed 43..128 character range.
const pkceVerifierBytes = 32

// generatePKCEVerifier returns a cryptographically random code_verifier.
func generatePKCEVerifier() (string, error) {
	b := make([]byte, pkceVerifierBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// pkceS256Challenge derives the S256 code_challenge for a code_verifier
// (RFC 7636 §4.2).
func pkceS256Challenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func (h *AuthHandler) setVerifierCookie(w http.ResponseWriter, verifier string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name:     verifierCookieName,
		Value:    verifier,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.SecureCookies,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
	})
}

func (h *AuthHandler) clearVerifierCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     verifierCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   h.SecureCookies,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}