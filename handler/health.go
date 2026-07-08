package handler

import (
	"context"
	"net/http"
	"time"
)

type DBPinger interface {
	Ping(ctx context.Context) error
}

type SessionPinger interface {
	Ping(ctx context.Context) error
}

type HealthHandler struct {
	DB        DBPinger
	Sessions  SessionPinger
	S3        S3Repository
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

func (h *HealthHandler) Healthz(w http.ResponseWriter, r *http.Request) {
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
		respondJSON(w, http.StatusOK, status)
	} else {
		status.Status = "degraded"
		respondJSON(w, http.StatusServiceUnavailable, status)
	}
}
