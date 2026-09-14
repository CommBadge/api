package testutil

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHelpers(t *testing.T) {
	req := NewRequest("POST", "/x", `{"a":1}`)
	if req.Header.Get("Content-Type") != "application/json" {
		t.Fatal("content-type not set")
	}
	authReq := NewAuthenticatedRequest("GET", "/x", "", "u1")
	if c, err := authReq.Cookie("session"); err != nil || c.Value != "jwt-u1" {
		t.Fatalf("auth cookie = %v, %v", c, err)
	}

	rec := httptest.NewRecorder()
	rec.WriteHeader(http.StatusOK)
	rec.Body.WriteString(`{"ok":true}`)
	resp := rec.Result()

	body, err := ReadResponseBody(resp)
	if err != nil || string(body) != `{"ok":true}` {
		t.Fatalf("ReadResponseBody = %q, %v", body, err)
	}

	rec2 := httptest.NewRecorder()
	rec2.Header().Set("Content-Type", "application/json")
	rec2.WriteHeader(http.StatusOK)
	rec2.Body.WriteString(`{"ok":true}`)
	var out map[string]bool
	if err := DecodeResponse(rec2.Result(), &out); err != nil || !out["ok"] {
		t.Fatalf("DecodeResponse = %v, %v", out, err)
	}
}