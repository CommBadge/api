// Package common holds the tiny HTTP helpers shared by the handler modules:
// JSON response writing, request body decoding, and community ID validation.
package common

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
)

// RespondJSON writes data as a JSON response with the given status code. The
// X-Content-Type-Options header is set so browsers never sniff the body.
func RespondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// RespondError writes msg as the "error" field of a JSON error response.
func RespondError(w http.ResponseWriter, status int, msg string) {
	RespondJSON(w, status, map[string]string{"error": msg})
}

// DecodeJSON decodes a JSON request body into v.
func DecodeJSON(r *http.Request, v interface{}) error {
	return json.NewDecoder(r.Body).Decode(v)
}

// Middleware is a function that wraps an http.Handler. Handler modules use it
// to declare which middleware a route requires without depending on the
// concrete middleware package.
type Middleware func(http.Handler) http.Handler

// ValidateCommunityID reads the {communityID} path value and returns it if it
// parses as a UUID; otherwise it writes a 400 response and returns false.
func ValidateCommunityID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.PathValue("communityID")
	if _, err := uuid.Parse(id); err != nil {
		RespondError(w, http.StatusBadRequest, "invalid community id")
		return "", false
	}
	return id, true
}