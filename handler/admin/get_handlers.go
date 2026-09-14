package admin

import (
	"net/http"

	"kronus.dev/commbadge_api/handler/contracts"
)

// Deps carries what GetHandlers needs to build an AdminHandler. DegradedDetector
// is optional; the production wiring passes nil to keep maintenance handling
// in the admin module from fighting the app-level detector.
type Deps struct {
	AdminStore       contracts.AdminRepository
	CommunityStore   contracts.CommunityRepository
	TicketStore      contracts.TicketRepository
	UserStore        contracts.UserRepository
	DegradedDetector DegradedHalter
}

// GetHandlers builds the AdminHandler from deps.
func GetHandlers(deps Deps) *AdminHandler {
	return &AdminHandler{
		AdminStore:       deps.AdminStore,
		CommunityStore:   deps.CommunityStore,
		TicketStore:      deps.TicketStore,
		UserStore:        deps.UserStore,
		DegradedDetector: deps.DegradedDetector,
	}
}

// Register registers the admin endpoints on adminMux, the mux that app mounts
// at /api/admin/ behind the admin-only middleware.
func (h *AdminHandler) Register(adminMux *http.ServeMux) {
	adminMux.HandleFunc("GET /admin/communities", h.ListCommunities)
	adminMux.HandleFunc("DELETE /admin/communities/{communityID}", h.DisbandCommunity)
	adminMux.HandleFunc("POST /admin/maintenance", h.SetMaintenance)
	adminMux.HandleFunc("GET /admin/tickets/count", h.TicketCount)
	adminMux.HandleFunc("GET /admin/tickets", h.ListTickets)
	adminMux.HandleFunc("GET /admin/tickets/{ticketID}", h.GetTicket)
	adminMux.HandleFunc("POST /admin/tickets/{ticketID}/reply", h.ReplyTicket)
	adminMux.HandleFunc("PATCH /admin/tickets/{ticketID}/status", h.UpdateTicketStatus)
	adminMux.HandleFunc("GET /admin/users", h.ListUsers)
	adminMux.HandleFunc("GET /admin/users/search", h.SearchUsers)
	adminMux.HandleFunc("GET /admin/users/{userID}/warnings", h.GetUserWarnings)
	adminMux.HandleFunc("POST /admin/users/{userID}/warn", h.WarnUser)
	adminMux.HandleFunc("POST /admin/users/{userID}/ban", h.BanUser)
	adminMux.HandleFunc("POST /admin/users/{userID}/unban", h.UnbanUser)
}