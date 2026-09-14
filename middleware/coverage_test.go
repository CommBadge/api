package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"kronus.dev/commbadge_api/session"
)

type fakeSessions struct {
	ses *session.Session
	err error
}

func (f *fakeSessions) Get(ctx context.Context, sessionID string) (*session.Session, error) {
	return f.ses, f.err
}

type fakeChecker struct {
	val bool
	err error
}

func (f *fakeChecker) IsAdmin(ctx context.Context, userID string) (bool, error)  { return f.val, f.err }
func (f *fakeChecker) IsBanned(ctx context.Context, userID string) (bool, error) { return f.val, f.err }

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
}

func newAuth() *AuthMiddleware {
	return &AuthMiddleware{
		VerifyJWT: func(token string) (string, error) {
			if token == "bad" {
				return "", errors.New("invalid")
			}
			return "sid-1", nil
		},
		Sessions: &fakeSessions{ses: &session.Session{UserID: "u1", Email: "a@b.c"}},
	}
}

func TestGetUserIDGetSessionID(t *testing.T) {
	if got := GetUserID(context.Background()); got != "" {
		t.Errorf("GetUserID = %q", got)
	}
	if got := GetSessionID(context.Background()); got != "" {
		t.Errorf("GetSessionID = %q", got)
	}
	ctx := context.WithValue(context.Background(), ContextUserID, "u9")
	ctx = context.WithValue(ctx, ContextSession, "s9")
	if got := GetUserID(ctx); got != "u9" {
		t.Errorf("GetUserID = %q", got)
	}
	if got := GetSessionID(ctx); got != "s9" {
		t.Errorf("GetSessionID = %q", got)
	}
}

func TestRequireAuth(t *testing.T) {
	m := newAuth()

	do := func(t *testing.T, cookie string, auth *AuthMiddleware) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		if cookie != "" {
			req.AddCookie(&http.Cookie{Name: "session", Value: cookie})
		}
		rec := httptest.NewRecorder()
		auth.RequireAuth(okHandler()).ServeHTTP(rec, req)
		return rec
	}

	t.Run("no cookie", func(t *testing.T) {
		if rec := do(t, "", m); rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d", rec.Code)
		}
	})
	t.Run("invalid jwt", func(t *testing.T) {
		if rec := do(t, "bad", m); rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d", rec.Code)
		}
	})
	t.Run("session error", func(t *testing.T) {
		failing := &AuthMiddleware{VerifyJWT: m.VerifyJWT, Sessions: &fakeSessions{err: errors.New("boom")}}
		if rec := do(t, "good", failing); rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d", rec.Code)
		}
	})
	t.Run("nil session", func(t *testing.T) {
		missing := &AuthMiddleware{VerifyJWT: m.VerifyJWT, Sessions: &fakeSessions{}}
		if rec := do(t, "good", missing); rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d", rec.Code)
		}
	})
	t.Run("success sets context", func(t *testing.T) {
		var userID, email, sessionID string
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID = GetUserID(r.Context())
			email = r.Context().Value(ContextEmail).(string)
			sessionID = GetSessionID(r.Context())
			w.WriteHeader(http.StatusNoContent)
		})
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.AddCookie(&http.Cookie{Name: "session", Value: "tok"})
		rec := httptest.NewRecorder()
		m.RequireAuth(next).ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d", rec.Code)
		}
		if userID != "u1" || email != "a@b.c" || sessionID != "sid-1" {
			t.Fatalf("context = %q %q %q", userID, email, sessionID)
		}
	})
}

func TestRequireAdmin(t *testing.T) {
	m := newAuth()

	do := func(t *testing.T, checker AdminChecker) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/admin", nil)
		req.AddCookie(&http.Cookie{Name: "session", Value: "tok"})
		rec := httptest.NewRecorder()
		m.RequireAdmin(checker)(okHandler()).ServeHTTP(rec, req)
		return rec
	}

	t.Run("not admin", func(t *testing.T) {
		if rec := do(t, &fakeChecker{val: false}); rec.Code != http.StatusForbidden {
			t.Fatalf("status = %d", rec.Code)
		}
	})
	t.Run("checker error", func(t *testing.T) {
		if rec := do(t, &fakeChecker{err: errors.New("boom")}); rec.Code != http.StatusForbidden {
			t.Fatalf("status = %d", rec.Code)
		}
	})
	t.Run("admin", func(t *testing.T) {
		if rec := do(t, &fakeChecker{val: true}); rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d", rec.Code)
		}
	})
}

func TestRequireNotBanned(t *testing.T) {
	m := newAuth()

	do := func(t *testing.T, checker BannedChecker) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{Name: "session", Value: "tok"})
		rec := httptest.NewRecorder()
		m.RequireNotBanned(checker)(okHandler()).ServeHTTP(rec, req)
		return rec
	}

	t.Run("checker error", func(t *testing.T) {
		if rec := do(t, &fakeChecker{err: errors.New("boom")}); rec.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d", rec.Code)
		}
	})
	t.Run("banned", func(t *testing.T) {
		rec := do(t, &fakeChecker{val: true})
		if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "banned") {
			t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
		}
	})
	t.Run("not banned", func(t *testing.T) {
		if rec := do(t, &fakeChecker{}); rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d", rec.Code)
		}
	})
}

func TestBasicAuth(t *testing.T) {
	h := BasicAuth("admin", "pw", okHandler())

	do := func(req *http.Request) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	t.Run("no credentials", func(t *testing.T) {
		rec := do(httptest.NewRequest(http.MethodGet, "/metrics", nil))
		if rec.Code != http.StatusUnauthorized || rec.Header().Get("WWW-Authenticate") == "" {
			t.Fatalf("status = %d", rec.Code)
		}
	})
	t.Run("wrong user", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		req.SetBasicAuth("evil", "pw")
		if rec := do(req); rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d", rec.Code)
		}
	})
	t.Run("wrong password", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		req.SetBasicAuth("admin", "nope")
		if rec := do(req); rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d", rec.Code)
		}
	})
	t.Run("valid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		req.SetBasicAuth("admin", "pw")
		if rec := do(req); rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d", rec.Code)
		}
	})
}

func TestLimitBodySize(t *testing.T) {
	readAll := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 4096)
		_, err := r.Body.Read(buf)
		if err != nil {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	h := LimitBodySize(16)(readAll)

	t.Run("oversize", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(strings.Repeat("a", 64)))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("status = %d", rec.Code)
		}
	})
	t.Run("fits", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("small"))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d", rec.Code)
		}
	})
}

func TestChainOrder(t *testing.T) {
	var order []string
	mark := func(name string) Middleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				order = append(order, name+"-in")
				next.ServeHTTP(w, r)
				order = append(order, name+"-out")
			})
		}
	}
	last := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		order = append(order, "handler")
		w.WriteHeader(http.StatusNoContent)
	})
	h := NewChain(mark("a"), mark("b"), mark("c")).Then(last)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	want := "a-in,b-in,c-in,handler,c-out,b-out,a-out"
	if got := strings.Join(order, ","); got != want {
		t.Fatalf("order = %s, want %s", got, want)
	}
}

func TestCORS(t *testing.T) {
	run := func(c *CORS, origin, method string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/", nil)
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		rec := httptest.NewRecorder()
		c.Wrap(okHandler()).ServeHTTP(rec, req)
		return rec
	}

	t.Run("star", func(t *testing.T) {
		c := NewCORS([]string{"*"})
		rec := run(c, "https://anything.example.com", http.MethodGet)
		if rec.Header().Get("Access-Control-Allow-Origin") != "https://anything.example.com" {
			t.Fatalf("allow-origin = %q", rec.Header().Get("Access-Control-Allow-Origin"))
		}
	})

	t.Run("allowed list", func(t *testing.T) {
		c := NewCORS([]string{"https://app.example.com"})
		rec := run(c, "https://app.example.com", http.MethodGet)
		if rec.Header().Get("Access-Control-Allow-Origin") == "" {
			t.Fatal("expected CORS headers")
		}
	})

	t.Run("case insensitive match", func(t *testing.T) {
		c := NewCORS([]string{"https://app.example.com"})
		rec := run(c, "HTTPS://APP.EXAMPLE.COM", http.MethodGet)
		if rec.Header().Get("Access-Control-Allow-Origin") == "" {
			t.Fatal("expected case-insensitive CORS match")
		}
	})

	t.Run("disallowed origin", func(t *testing.T) {
		c := NewCORS([]string{"https://app.example.com"})
		rec := run(c, "https://evil.example.com", http.MethodGet)
		if rec.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Fatal("expected no CORS headers")
		}
	})

	t.Run("no origin", func(t *testing.T) {
		c := NewCORS([]string{"https://app.example.com"})
		rec := run(c, "", http.MethodGet)
		if rec.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Fatal("expected no CORS headers")
		}
	})

	t.Run("options short-circuits", func(t *testing.T) {
		c := NewCORS([]string{"https://app.example.com"})
		rec := run(c, "https://app.example.com", http.MethodOptions)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d", rec.Code)
		}
	})
}

func TestDegradedWrap(t *testing.T) {
	t.Run("not degraded passes through", func(t *testing.T) {
		d := &DegradedDetector{}
		rec := httptest.NewRecorder()
		d.Wrap(okHandler()).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/x", nil))
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d", rec.Code)
		}
	})

	t.Run("degraded blocks and allows probes", func(t *testing.T) {
		d := &DegradedDetector{}
		d.degraded.Store(true)
		probePaths := []string{"/healthz", "/readyz", "/metrics", "/"}
		for _, p := range probePaths {
			rec := httptest.NewRecorder()
			d.Wrap(okHandler()).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
			if rec.Code != http.StatusNoContent {
				t.Fatalf("%s: status = %d", p, rec.Code)
			}
		}
		rec := httptest.NewRecorder()
		d.Wrap(okHandler()).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/x", nil))
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d", rec.Code)
		}
	})
}

func TestDegradedFlags(t *testing.T) {
	d := &DegradedDetector{}
	if d.IsDegraded() {
		t.Error("should start not degraded")
	}
	if d.IsHalted() {
		t.Error("should start not halted")
	}
	d.Halt()
	if !d.IsHalted() {
		t.Error("Halt should set halted")
	}
}

func TestLogging(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	rec := httptest.NewRecorder()
	Logging(next).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api", nil))
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestRateLimiterCoverage(t *testing.T) {
	t.Run("exempt paths", func(t *testing.T) {
		rl := NewRateLimiter(1, 1, nil)
		for _, p := range []string{"/healthz", "/readyz", "/metrics"} {
			rec := httptest.NewRecorder()
			rl.Wrap(okHandler()).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
			if rec.Code != http.StatusNoContent {
				t.Fatalf("%s: status = %d", p, rec.Code)
			}
		}
	})

	t.Run("rate limited", func(t *testing.T) {
		rl := NewRateLimiter(1, 1, nil)
		req := func() *httptest.ResponseRecorder {
			r := httptest.NewRequest(http.MethodGet, "/api", nil)
			r.RemoteAddr = "1.2.3.4:1234"
			rec := httptest.NewRecorder()
			rl.Wrap(okHandler()).ServeHTTP(rec, r)
			return rec
		}
		if rec := req(); rec.Code != http.StatusNoContent {
			t.Fatalf("first status = %d", rec.Code)
		}
		if rec := req(); rec.Code != http.StatusTooManyRequests {
			t.Fatalf("second status = %d", rec.Code)
		}
	})

	t.Run("invalid remote addr falls back to raw", func(t *testing.T) {
		rl := NewRateLimiter(50, 100, nil)
		r := httptest.NewRequest(http.MethodGet, "/api", nil)
		r.RemoteAddr = "not-an-addr"
		rec := httptest.NewRecorder()
		rl.Wrap(okHandler()).ServeHTTP(rec, r)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d", rec.Code)
		}
	})

	t.Run("clientIP untrusted peer", func(t *testing.T) {
		rl := NewRateLimiter(50, 100, nil)
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = "8.8.8.8:9999"
		if got := rl.clientIP(r); got != "8.8.8.8" {
			t.Errorf("clientIP = %q", got)
		}
	})

	t.Run("clientIP trusted proxy without xff", func(t *testing.T) {
		rl := NewRateLimiter(50, 100, []string{"10.0.0.0/8"})
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = "10.0.0.5:80"
		if got := rl.clientIP(r); got != "10.0.0.5" {
			t.Errorf("clientIP = %q", got)
		}
	})

	t.Run("clientIP xff all trusted falls back to peer", func(t *testing.T) {
		rl := NewRateLimiter(50, 100, []string{"10.0.0.0/8"})
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = "10.0.0.5:80"
		r.Header.Set("X-Forwarded-For", "10.0.0.6, 10.0.0.7")
		if got := rl.clientIP(r); got != "10.0.0.5" {
			t.Errorf("clientIP = %q", got)
		}
	})

	t.Run("clientIP xff invalid entries", func(t *testing.T) {
		rl := NewRateLimiter(50, 100, []string{"10.0.0.0/8"})
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = "10.0.0.5:80"
		r.Header.Set("X-Forwarded-For", "garbage,  ")
		if got := rl.clientIP(r); got != "10.0.0.5" {
			t.Errorf("clientIP = %q", got)
		}
	})

	t.Run("clientIP prefers rightmost untrusted xff", func(t *testing.T) {
		rl := NewRateLimiter(50, 100, []string{"10.0.0.0/8"})
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = "10.0.0.5:80"
		r.Header.Add("X-Forwarded-For", "10.0.0.6")
		r.Header.Add("X-Forwarded-For", "1.2.3.4")
		if got := rl.clientIP(r); got != "1.2.3.4" {
			t.Errorf("clientIP = %q", got)
		}
	})

	t.Run("parseClientAddr", func(t *testing.T) {
		rl := NewRateLimiter(50, 100, nil)
		if _, ok := parseClientAddr("1.2.3.4:99"); !ok {
			t.Error("expected host:port parse")
		}
		if a, ok := parseClientAddr("1.2.3.4"); !ok || a.String() != "1.2.3.4" {
			t.Errorf("bare addr = %v, %v", a, ok)
		}
		if _, ok := parseClientAddr(":"); ok {
			t.Error("expected parse failure")
		}
		_ = rl
	})

	t.Run("max visitors eviction", func(t *testing.T) {
		rl := NewRateLimiter(1000, 10, nil)
		for i := 0; i < maxVisitors+5; i++ {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.RemoteAddr = "10.0.0." + itoa(i%250) + ":1"
			rec := httptest.NewRecorder()
			rl.Wrap(okHandler()).ServeHTTP(rec, r)
		}
		rl.mu.Lock()
		n := len(rl.visitors)
		rl.mu.Unlock()
		if n > maxVisitors {
			t.Fatalf("visitors = %d, want <= %d", n, maxVisitors)
		}
	})
}

func itoa(n int) string {
	if n < 10 {
		return "0" + string(rune('0'+n))
	}
	return string(rune('0'+n/10)) + string(rune('0'+n%10))
}
