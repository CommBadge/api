package tickets

import (
	"net/http"

	"kronus.dev/commbadge_api/handler/contracts"
)

// Deps carries what GetHandlers needs to build a TicketHandler.
type Deps struct {
	Tickets contracts.TicketRepository
}

// GetHandlers builds the TicketHandler from deps.
func GetHandlers(deps Deps) *TicketHandler {
	return &TicketHandler{
		Tickets: deps.Tickets,
	}
}

// Register registers the ticket endpoints on ticketMux, the mux that app
// mounts at /api/tickets and /api/tickets/ with authentication, ban, and
// body-size middleware.
func (h *TicketHandler) Register(ticketMux *http.ServeMux) {
	ticketMux.HandleFunc("POST /tickets", h.Create)
	ticketMux.HandleFunc("GET /tickets", h.List)
	ticketMux.HandleFunc("GET /tickets/{ticketID}", h.Get)
	ticketMux.HandleFunc("POST /tickets/{ticketID}/messages", h.AddMessage)
}