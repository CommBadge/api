package middleware

import (
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// maxVisitors bounds the per-IP limiter map so a flood of spoofed source IPs
// cannot grow memory without limit.
const maxVisitors = 10000

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type RateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	rate     rate.Limit
	burst    int

	// trusted is the set of reverse-proxy prefixes whose X-Forwarded-For
	// headers we trust. Client-supplied XFF headers from other sources are
	// ignored, so a remote attacker cannot rotate the header to bypass the
	// limiter.
	trusted []netip.Prefix
}

func NewRateLimiter(rps, burst int, trustedProxyCIDRs []string) *RateLimiter {
	rl := &RateLimiter{
		visitors: make(map[string]*visitor),
		rate:     rate.Limit(rps),
		burst:    burst,
	}
	for _, cidr := range trustedProxyCIDRs {
		if p, err := netip.ParsePrefix(cidr); err == nil {
			rl.trusted = append(rl.trusted, p)
		}
	}
	go rl.cleanup(5 * time.Minute)
	return rl
}

func (rl *RateLimiter) isTrusted(ip netip.Addr) bool {
	for _, p := range rl.trusted {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}

// clientIP determines the effective client IP. When the direct peer is a
// trusted proxy, the rightmost X-Forwarded-For entry that is not itself a
// trusted proxy is used. Otherwise the socket peer address is used and the
// XFF header is ignored entirely.
func (rl *RateLimiter) clientIP(r *http.Request) string {
	remote, ok := parseClientAddr(r.RemoteAddr)
	if !ok {
		return r.RemoteAddr
	}

	if !remote.IsValid() || len(rl.trusted) == 0 || !rl.isTrusted(remote) {
		return remote.String()
	}

	chain := r.Header.Values("X-Forwarded-For")
	if len(chain) == 0 {
		return remote.String()
	}
	// Walk the chain right-to-left: the rightmost entry was added by the
	// closest trusted proxy and is the closest to the real client.
	for i := len(chain) - 1; i >= 0; i-- {
		for _, part := range strings.Split(chain[i], ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			if a, aerr := netip.ParseAddr(part); aerr == nil {
				// Unmap so IPv4-mapped IPv6 (::ffff:1.2.3.4) keys the same
				// limiter bucket as the plain dotted-quad form (1.2.3.4).
				a = a.Unmap()
				if !rl.isTrusted(a) {
					return a.String()
				}
			}
		}
	}
	return remote.String()
}

// parseClientAddr parses an http.Server RemoteAddr value (host, host:port, or
// [v6]:port) and normalizes it, unmapping IPv4-mapped IPv6 addresses so they
// produce the same limiter key as their dotted-quad form.
func parseClientAddr(s string) (netip.Addr, bool) {
	if a, err := netip.ParseAddr(s); err == nil {
		return a.Unmap(), true
	}
	if a, err := netip.ParseAddrPort(s); err == nil {
		return a.Addr().Unmap(), true
	}
	return netip.Addr{}, false
}

func (rl *RateLimiter) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Health and metrics endpoints are exempt: throttling a probe that is
		// supposed to report availability would otherwise report a healthy
		// instance as degraded.
		switch r.URL.Path {
		case "/healthz", "/readyz", "/metrics":
			next.ServeHTTP(w, r)
			return
		}

		ip := rl.clientIP(r)

		rl.mu.Lock()
		v, ok := rl.visitors[ip]
		if !ok {
			if len(rl.visitors) >= maxVisitors {
				rl.evictOldestLocked()
			}
			v = &visitor{limiter: rate.NewLimiter(rl.rate, rl.burst)}
			rl.visitors[ip] = v
		}
		v.lastSeen = time.Now()
		rl.mu.Unlock()

		if !v.limiter.Allow() {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":"rate limit exceeded"}`))
			return
		}

		next.ServeHTTP(w, r)
	})
}

// evictOldestLocked removes the single least-recently-seen visitor. Callers
// must hold rl.mu.
func (rl *RateLimiter) evictOldestLocked() {
	var oldestKey string
	var oldest time.Time
	first := true
	for ip, v := range rl.visitors {
		if first || v.lastSeen.Before(oldest) {
			oldestKey = ip
			oldest = v.lastSeen
			first = false
		}
	}
	if oldestKey != "" {
		delete(rl.visitors, oldestKey)
	}
}

func (rl *RateLimiter) cleanup(maxAge time.Duration) {
	for {
		time.Sleep(time.Minute)
		rl.mu.Lock()
		for ip, v := range rl.visitors {
			if time.Since(v.lastSeen) > maxAge {
				delete(rl.visitors, ip)
			}
		}
		rl.mu.Unlock()
	}
}
