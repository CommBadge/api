package whoami

import (
	"net/http"

	"kronus.dev/commbadge_api/handler/common"
	"kronus.dev/commbadge_api/handler/contracts"
)

// Deps carries what GetHandlers needs to build a WhoamiHandler.
type Deps struct {
	Users       contracts.UserRepository
	Communities contracts.CommunityRepository
}

// GetHandlers builds the WhoamiHandler from deps.
func GetHandlers(deps Deps) *WhoamiHandler {
	return &WhoamiHandler{
		Users:       deps.Users,
		Communities: deps.Communities,
	}
}

// Routes registers the whoami endpoint. The route is only served to
// authenticated, non-banned users, so the caller supplies the composed auth
// middleware.
func (h *WhoamiHandler) Routes(mux *http.ServeMux, auth common.Middleware) {
	mux.Handle("GET /auth/whoami", auth(http.HandlerFunc(h.Whoami)))
}