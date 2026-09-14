package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientIP_CanonicalizesMappedAddresses(t *testing.T) {
	rl := NewRateLimiter(50, 100, nil)

	// IPv4-mapped IPv6 and dotted-quad forms of the same address must produce
	// the same limiter key so a client cannot multiply its budget by
	// alternating representations.
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "[::ffff:1.2.3.4]:5678"
	if got := rl.clientIP(req); got != "1.2.3.4" {
		t.Errorf("mapped RemoteAddr: got %q, want %q", got, "1.2.3.4")
	}

	req2 := httptest.NewRequest("GET", "/", nil)
	req2.RemoteAddr = "1.2.3.4:5678"
	if got := rl.clientIP(req2); got != "1.2.3.4" {
		t.Errorf("plain RemoteAddr: got %q, want %q", got, "1.2.3.4")
	}
}

func TestClientIP_CanonicalizesXFFEntries(t *testing.T) {
	// When behind a trusted proxy, XFF entries are attacker-supplied strings;
	// mapped and dotted-quad spellings must collapse to one key.
	rl := NewRateLimiter(50, 100, []string{"10.0.0.0/8"})

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "10.0.0.5:8080"
	req.Header.Set("X-Forwarded-For", "::ffff:8.8.8.8")
	if got := rl.clientIP(req); got != "8.8.8.8" {
		t.Errorf("mapped XFF: got %q, want %q", got, "8.8.8.8")
	}

	req2 := httptest.NewRequest("GET", "/", nil)
	req2.RemoteAddr = "10.0.0.5:8080"
	req2.Header.Set("X-Forwarded-For", "8.8.8.8")
	if got := rl.clientIP(req2); got != "8.8.8.8" {
		t.Errorf("plain XFF: got %q, want %q", got, "8.8.8.8")
	}
}

func TestClientIP_IgnoresXFFFromUntrustedPeer(t *testing.T) {
	rl := NewRateLimiter(50, 100, nil)

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "1.2.3.4:5678"
	req.Header.Set("X-Forwarded-For", "9.9.9.9")
	if got := rl.clientIP(req); got != "1.2.3.4" {
		t.Errorf("XFF from untrusted peer must be ignored: got %q, want %q", got, "1.2.3.4")
	}
}

func TestRateLimiter_EvictsOldestWhenAtCapacity(t *testing.T) {
	rl := NewRateLimiter(1000, 10, nil)

	// years.ordination: insert exactly maxVisitors visitors with distinct
	// keys, then touch the first so "first" is no longer the least-recent.
	for i := 0; i < maxVisitors; i++ {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = addrFor(i) + ":1"
		rec := httptest.NewRecorder()
		rl.Wrap(okHandler()).ServeHTTP(rec, r)
	}
	firstIP := addrFor(0)
	rl.mu.Lock()
	rl.visitors[firstIP].lastSeen = time.Now().Add(time.Hour)
	rl.mu.Unlock()

	// The next distinct visitor forces eviction of the single oldest, which
	// must not be the recently-touched firstIP.
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = addrFor(maxVisitors) + ":1"
	rec := httptest.NewRecorder()
	rl.Wrap(okHandler()).ServeHTTP(rec, r)

	rl.mu.Lock()
	defer rl.mu.Unlock()
	if _, ok := rl.visitors[firstIP]; !ok {
		t.Fatal("recently-touched visitor must survive eviction")
	}
	if len(rl.visitors) > maxVisitors {
		t.Fatalf("visitors = %d, want <= %d", len(rl.visitors), maxVisitors)
	}
}

// addrFor maps an index to a unique dotted-quad for rate-limit testing. The
// range of maxVisitors (10000) fits within the 10.x.y.z space.
func addrFor(i int) string {
	z := i % 250
	y := (i / 250) % 250
	x := (i / (250 * 250)) % 250
	return fmt.Sprintf("10.%d.%d.%d", x, y, z)
}

func TestRateLimiter_Wrap_IndependentBudgetsBehindTrustedProxy(t *testing.T) {
	// Two distinct real clients behind a trusted proxy must each get their own
	// budget keyed by XFF, so one cannot exhaust the other's allowance.
	rl := NewRateLimiter(1, 1, []string{"10.0.0.0/8"})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	req := func(xff string, call int) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", "/api", nil)
		r.RemoteAddr = "10.0.0.5:8080"
		r.Header.Set("X-Forwarded-For", xff)
		rec := httptest.NewRecorder()
		rl.Wrap(handler).ServeHTTP(rec, r)
		return rec
	}

	if rec := req("8.8.8.8", 0); rec.Code != http.StatusNoContent {
		t.Fatalf("client A first = %d", rec.Code)
	}
	if rec := req("8.8.8.8", 0); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("client A second = %d, want 429", rec.Code)
	}
	if rec := req("9.9.9.9", 0); rec.Code != http.StatusNoContent {
		t.Fatalf("client B budget must be independent, got %d", rec.Code)
	}
}

func TestRateLimiter_Wrap_SpoofedXFFDoesNotMultiplyBudget(t *testing.T) {
	// From an untrusted peer the XFF header is ignored, so rotating it must
	// not yield fresh budgets for the same client.
	rl := NewRateLimiter(1, 1, nil)

	req := func(xff string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", "/api", nil)
		r.RemoteAddr = "8.8.8.8:1000"
		if xff != "" {
			r.Header.Set("X-Forwarded-For", xff)
		}
		rec := httptest.NewRecorder()
		rl.Wrap(okHandler()).ServeHTTP(rec, r)
		return rec
	}

	if rec := req(""); rec.Code != http.StatusNoContent {
		t.Fatalf("first = %d", rec.Code)
	}
	if rec := req("1.2.3.4"); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("spoofed xff must still be limited, got %d", rec.Code)
	}
	if rec := req("6.6.6.6"); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("second spoofed xff must still be limited, got %d", rec.Code)
	}
}
