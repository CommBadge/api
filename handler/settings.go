package handler

import (
	"fmt"
	"net/http"
	"strings"

	"kronus.dev/commbadge_api/middleware"
)

type SettingsHandler struct {
	Users       UserRepository
	Communities CommunityRepository
}

type discordNotificationSettings struct {
	ChannelID string `json:"channel_id"`
}

func (h *SettingsHandler) SetNotificationChannel(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var body discordNotificationSettings
	if err := decodeJSON(r, &body); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.ChannelID == "" {
		respondError(w, http.StatusBadRequest, "channel_id is required")
		return
	}

	if err := h.Users.SetNotificationChannel(r.Context(), userID, body.ChannelID); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to save notification settings")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message":    "notification channel updated",
		"channel_id": body.ChannelID,
	})
}

type communityDiscordSettings struct {
	GuildID       string `json:"guild_id"`
	LiveChannelID string `json:"live_channel_id"`
}

const shoutoutPlaceholder = "{twitch_url}"

// MaxShoutoutTemplateLength caps the shoutout template at 500 characters, the
// same limit Twitch chat places on a single message. The value is bound as a
// query parameter (never concatenated into SQL), so the freeform template
// cannot be used for SQL injection.
const MaxShoutoutTemplateLength = 500

type shoutoutTemplateSettings struct {
	Template string `json:"template"`
}

// SetShoutoutTemplate stores the broadcaster's shoutout message template. An
// empty template clears the setting (the default is used); a non-empty
// template must contain exactly one {twitch_url} placeholder and stay within
// MaxShoutoutTemplateLength characters.
func (h *SettingsHandler) SetShoutoutTemplate(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var body shoutoutTemplateSettings
	if err := decodeJSON(r, &body); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	body.Template = strings.TrimSpace(body.Template)
	if len(body.Template) > MaxShoutoutTemplateLength {
		respondError(w, http.StatusBadRequest,
			fmt.Sprintf("shoutout template must be at most %d characters", MaxShoutoutTemplateLength))
		return
	}
	if body.Template != "" && strings.Count(body.Template, shoutoutPlaceholder) != 1 {
		respondError(w, http.StatusBadRequest,
			"shoutout template must contain exactly one "+shoutoutPlaceholder+" placeholder")
		return
	}

	if err := h.Users.SetShoutoutTemplate(r.Context(), userID, body.Template); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to save shoutout template")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message":  "shoutout template updated",
		"template": body.Template,
	})
}

func (h *SettingsHandler) SetCommunityDiscord(w http.ResponseWriter, r *http.Request) {
	communityID, ok := validateCommunityID(w, r)
	if !ok {
		return
	}
	userID := middleware.GetUserID(r.Context())

	isMod, err := h.Communities.IsModeratorOrOwner(r.Context(), communityID, userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to check permissions")
		return
	}
	if !isMod {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}

	var body communityDiscordSettings
	if err := decodeJSON(r, &body); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.GuildID == "" {
		respondError(w, http.StatusBadRequest, "guild_id is required")
		return
	}
	if body.LiveChannelID == "" {
		respondError(w, http.StatusBadRequest, "live_channel_id is required")
		return
	}

	if err := h.Communities.SetDiscordConfig(r.Context(), communityID, body.GuildID, body.LiveChannelID); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to save community discord settings")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message":         "community discord settings updated",
		"guild_id":        body.GuildID,
		"live_channel_id": body.LiveChannelID,
	})
}
