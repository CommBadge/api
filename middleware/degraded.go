package middleware

import (
	"context"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"kronus.dev/commbadge_api/session"
)

type DegradedDetector struct {
	pool      *pgxpool.Pool
	sessions  *session.Store
	maintFile string
	degraded  atomic.Bool
	halted    atomic.Bool
}

func NewDegradedDetector(pool *pgxpool.Pool, sessions *session.Store, maintFile string) *DegradedDetector {
	d := &DegradedDetector{
		pool:      pool,
		sessions:  sessions,
		maintFile: maintFile,
	}
	d.degraded.Store(false)
	go d.poll()
	return d
}

func (d *DegradedDetector) IsDegraded() bool {
	return d.degraded.Load()
}

func (d *DegradedDetector) IsHalted() bool {
	return d.halted.Load()
}

func (d *DegradedDetector) Halt() {
	d.halted.Store(true)
}

func (d *DegradedDetector) poll() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if d.halted.Load() {
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		dbOK := d.pool.Ping(ctx) == nil
		redisOK := d.sessions.Ping(ctx) == nil
		cancel()

		_, maintFound := os.Stat(d.maintFile)

		if !dbOK || !redisOK || maintFound == nil {
			d.degraded.Store(true)
		} else {
			d.degraded.Store(false)
		}
	}
}

func (d *DegradedDetector) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if d.degraded.Load() {
			path := r.URL.Path
			if path == "/healthz" || path == "/metrics" || path == "/" {
				next.ServeHTTP(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"error":"service unavailable"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}
