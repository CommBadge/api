package middleware

import (
	"net/http"
	"strings"
)

type CORS struct {
	AllowedOrigins []string
	allOrigins     bool
}

func NewCORS(origins []string) *CORS {
	c := &CORS{AllowedOrigins: origins}
	for _, o := range origins {
		if o == "*" {
			c.allOrigins = true
			break
		}
	}
	return c
}

func (c *CORS) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		if origin != "" && (c.allOrigins || c.isAllowed(origin)) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (c *CORS) isAllowed(origin string) bool {
	for _, o := range c.AllowedOrigins {
		if strings.EqualFold(o, origin) {
			return true
		}
	}
	return false
}
