package community

import (
	"net/http"

	"kronus.dev/commbadge_api/handler/contracts"
)

// Deps carries what GetHandlers needs to build a CommunityHandler.
type Deps struct {
	Communities contracts.CommunityRepository
	S3          contracts.S3Repository
}

// GetHandlers builds the CommunityHandler from deps.
func GetHandlers(deps Deps) *CommunityHandler {
	return &CommunityHandler{
		Communities: deps.Communities,
		S3:          deps.S3,
	}
}

// RegisterAPI registers the JSON community and settings-adjacent endpoints on
// apiMux, the mux that app mounts at /api/ with authentication, ban, and
// body-size middleware. Join-by-code lives under /join/ so its {code} wildcard
// does not collide with /communities/{communityID}/join.
func (h *CommunityHandler) RegisterAPI(apiMux *http.ServeMux) {
	apiMux.HandleFunc("GET /communities", h.ListUserCommunities)
	apiMux.HandleFunc("POST /communities", h.CreateCommunity)
	apiMux.HandleFunc("GET /communities/{communityID}", h.GetCommunity)
	apiMux.HandleFunc("PUT /communities/{communityID}", h.UpdateCommunity)
	apiMux.HandleFunc("DELETE /communities/{communityID}", h.DeleteCommunity)
	apiMux.HandleFunc("POST /communities/{communityID}/join", h.JoinCommunity)
	apiMux.HandleFunc("POST /communities/{communityID}/moderate", h.ModerateCommunity)
	apiMux.HandleFunc("POST /communities/{communityID}/leave", h.LeaveCommunity)
	apiMux.HandleFunc("PUT /communities/{communityID}/join-link", h.RegenerateJoinLink)
	apiMux.HandleFunc("PATCH /communities/{communityID}/join-link-settings", h.UpdateJoinLinkSettings)
	apiMux.HandleFunc("GET /join/{code}", h.JoinByCode)
	apiMux.HandleFunc("POST /join/{code}", h.JoinByCode)
}

// RegisterUpload registers the multipart logo upload endpoint on uploadMux,
// the mux that app mounts at /api/upload/ with the larger body-size
// middleware.
func (h *CommunityHandler) RegisterUpload(uploadMux *http.ServeMux) {
	uploadMux.HandleFunc("POST /logo/{communityID}", h.UploadLogo)
}