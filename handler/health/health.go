// Package health implements the liveness and readiness probes.
package health

import (
	"context"
	"net/http"
	"time"

	"kronus.dev/commbadge_api/handler/common"
	"kronus.dev/commbadge_api/handler/contracts"
)

type HealthHandler struct {
	DB        contracts.DBPinger
	Sessions  contracts.SessionPinger
	S3        contracts.S3Repository
	MaintFile string
}

type healthStatus struct {
	Status    string         `json:"status"`
	Databases databaseHealth `json:"databases"`
	Storage   storageHealth  `json:"storage,omitempty"`
	Mode      string         `json:"mode"`
}

type databaseHealth struct {
	Postgres string `json:"postgres"`
	Redis    string `json:"redis"`
}

type storageHealth struct {
	S3 string `json:"s3"`
}

// Healthz is a cheap liveness probe: it always returns 200 without touching
// any dependency, so orchestration can tell the process is alive even while
// databases are down (and the load balancer / degraded detector can keep
// routing to the pods). Use /readyz for dependency readiness.
func (h *HealthHandler) Healthz(w http.ResponseWriter, r *http.Request) {
	common.RespondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Readyz reports dependency readiness. Unlike the old combined probe, this
// endpoint is not rate-limited by the shared limiter (it is registered outside
// of it), so a healthy instance is never mistakenly reported as degraded
// because the probe itself got throttled.
func (h *HealthHandler) Readyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	status := healthStatus{
		Databases: databaseHealth{},
		Mode:      "normal",
	}

	dbOK := h.DB.Ping(ctx) == nil
	redisOK := h.Sessions.Ping(ctx) == nil

	if dbOK {
		status.Databases.Postgres = "connected"
	} else {
		status.Databases.Postgres = "disconnected"
	}

	if redisOK {
		status.Databases.Redis = "connected"
	} else {
		status.Databases.Redis = "disconnected"
	}

	if h.S3 != nil {
		if err := h.S3.BucketExists(ctx); err != nil {
			status.Storage.S3 = "unavailable"
		} else {
			status.Storage.S3 = "available"
		}
	}

	allOk := dbOK && redisOK
	if allOk && h.S3 != nil {
		allOk = status.Storage.S3 == "available"
	}

	if allOk {
		status.Status = "healthy"
		common.RespondJSON(w, http.StatusOK, status)
	} else {
		status.Status = "degraded"
		common.RespondJSON(w, http.StatusServiceUnavailable, status)
	}
}