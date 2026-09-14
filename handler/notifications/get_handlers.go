package notifications

import (
	"net/http"

	"kronus.dev/commbadge_api/handler/contracts"
)

// Deps carries what GetHandlers needs to build a NotificationHandler.
type Deps struct {
	Users       contracts.UserRepository
	Subs        contracts.SubscriptionRepository
	EventSub    contracts.TwitchEventSubClient
	CallbackURL string
	Secret      string
}

// GetHandlers builds the NotificationHandler from deps.
func GetHandlers(deps Deps) *NotificationHandler {
	return &NotificationHandler{
		Users:       deps.Users,
		Subs:        deps.Subs,
		EventSub:    deps.EventSub,
		CallbackURL: deps.CallbackURL,
		Secret:      deps.Secret,
	}
}

// Register registers the subscription endpoints on notifMux, the mux that app
// mounts at /api/notifications/ with authentication, ban, and body-size
// middleware.
func (h *NotificationHandler) Register(notifMux *http.ServeMux) {
	notifMux.HandleFunc("POST /subscriptions", h.Subscribe)
	notifMux.HandleFunc("GET /subscriptions", h.List)
	notifMux.HandleFunc("DELETE /subscriptions/{id}", h.Delete)
}