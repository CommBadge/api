package health

import (
	"net/http"

	"kronus.dev/commbadge_api/handler/contracts"
)

// Deps carries what GetHandlers needs to build a HealthHandler. S3 is optional
// (may be nil when object storage is not configured).
type Deps struct {
	DB        contracts.DBPinger
	Sessions  contracts.SessionPinger
	S3        contracts.S3Repository
	MaintFile string
}

// GetHandlers builds the HealthHandler from deps.
func GetHandlers(deps Deps) *HealthHandler {
	return &HealthHandler{
		DB:        deps.DB,
		Sessions:  deps.Sessions,
		S3:        deps.S3,
		MaintFile: deps.MaintFile,
	}
}

// Register registers the health probe endpoints on mux.
func (h *HealthHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", h.Healthz)
	mux.HandleFunc("GET /readyz", h.Readyz)
}