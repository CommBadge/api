package settings

import (
	"net/http"

	"kronus.dev/commbadge_api/handler/contracts"
	"kronus.dev/commbadge_api/middleware"
)

// Deps carries what GetHandlers needs to build a SettingsHandler.
type Deps struct {
	Users           contracts.UserRepository
	Communities     contracts.CommunityRepository
	Discord         DiscordVerifier
	DiscordBotToken string
	Sessions        middleware.SessionFinder
}

// GetHandlers builds the SettingsHandler from deps.
func GetHandlers(deps Deps) *SettingsHandler {
	return &SettingsHandler{
		Users:           deps.Users,
		Communities:     deps.Communities,
		Discord:         deps.Discord,
		DiscordBotToken: deps.DiscordBotToken,
		Sessions:        deps.Sessions,
	}
}

// RegisterAPI registers the settings endpoints on apiMux, the mux that app
// mounts at /api/ with authentication, ban, and body-size middleware.
func (h *SettingsHandler) RegisterAPI(apiMux *http.ServeMux) {
	apiMux.HandleFunc("PATCH /communities/{communityID}/discord", h.SetCommunityDiscord)
	apiMux.HandleFunc("PATCH /me/discord-notification", h.SetNotificationChannel)
	apiMux.HandleFunc("PATCH /me/shoutout-template", h.SetShoutoutTemplate)
}