package middleware

import (
	"crypto/subtle"
	"net/http"
)

// BasicAuth protects a handler with HTTP Basic authentication using
// constant-time credential comparison. It is used for the /metrics endpoint,
// which would otherwise disclose runtime internals to anyone who can reach
// the API.
func BasicAuth(username, password string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok ||
			subtle.ConstantTimeCompare([]byte(user), []byte(username)) != 1 ||
			subtle.ConstantTimeCompare([]byte(pass), []byte(password)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="commbadge-metrics"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
