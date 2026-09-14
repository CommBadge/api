// Package settings implements the user and community Discord/notification
// preference endpoints.
package settings

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"kronus.dev/commbadge_api/discord"
	"kronus.dev/commbadge_api/handler/common"
	"kronus.dev/commbadge_api/handler/contracts"
	"kronus.dev/commbadge_api/middleware"
)

// DiscordVerifier performs the Discord API lookups needed to prove that a
// notification target channel is actually accessible and authorized.
type DiscordVerifier interface {
	GetUserGuilds(ctx context.Context, accessToken string) ([]discord.Guild, error)
	GetChannel(ctx context.Context, botToken, channelID string) (*discord.Channel, error)
}

type SettingsHandler struct {
	Users           contracts.UserRepository
	Communities     contracts.CommunityRepository
	Discord         DiscordVerifier
	DiscordBotToken string
	Sessions        middleware.SessionFinder
}

type discordNotificationSettings struct {
	ChannelID string `json:"channel_id"`
}

// verifyDiscordAccess proves two things before a channel is accepted as a
// notification target: (1) the CommBadge bot is installed in the guild that
// owns the channel, and (2) the requesting user has Manage Channels in that
// guild. This prevents users from pointing notifications at channels in
// arbitrary Discord servers.
func (h *SettingsHandler) verifyDiscordAccess(ctx context.Context, accessToken, guildID string) error {
	guilds, err := h.Discord.GetUserGuilds(ctx, accessToken)
	if err != nil {
		return err
	}
	for _, g := range guilds {
		if g.ID != guildID {
			continue
		}
		perms, err := strconv.ParseInt(g.Permissions, 10, 64)
		if err == nil && (perms&discord.ManageChannelsPermission != 0 || perms&discord.AdministratorPermission != 0) {
			return nil
		}
	}
	return fmt.Errorf("user lacks Manage Channels permission in guild %s", guildID)
}

func (h *SettingsHandler) SetNotificationChannel(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var body discordNotificationSettings
	if err := common.DecodeJSON(r, &body); err != nil {
		common.RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.ChannelID == "" {
		common.RespondError(w, http.StatusBadRequest, "channel_id is required")
		return
	}

	ses, err := h.Sessions.Get(r.Context(), middleware.GetSessionID(r.Context()))
	if err != nil || ses == nil || ses.DiscordAccessToken == "" {
		common.RespondError(w, http.StatusForbidden, "re-login required to configure discord notifications")
		return
	}

	channel, err := h.Discord.GetChannel(r.Context(), h.DiscordBotToken, body.ChannelID)
	if err != nil {
		common.RespondError(w, http.StatusBadRequest, "channel is not accessible to the commbadge bot")
		return
	}
	if err := h.verifyDiscordAccess(r.Context(), ses.DiscordAccessToken, channel.GuildID); err != nil {
		common.RespondError(w, http.StatusForbidden, "you need Manage Channels permission in that server")
		return
	}

	if err := h.Users.SetNotificationChannel(r.Context(), userID, body.ChannelID); err != nil {
		common.RespondError(w, http.StatusInternalServerError, "failed to save notification settings")
		return
	}

	common.RespondJSON(w, http.StatusOK, map[string]string{
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
// MaxShoutoutTemplateLength characters. Control characters (carriage returns,
// line feeds, and NUL) are rejected because the rendered message is sent over
// the IRC wire format, where they could otherwise be used to inject IRC
// commands.
func (h *SettingsHandler) SetShoutoutTemplate(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var body shoutoutTemplateSettings
	if err := common.DecodeJSON(r, &body); err != nil {
		common.RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	body.Template = strings.TrimSpace(body.Template)
	if len(body.Template) > MaxShoutoutTemplateLength {
		common.RespondError(w, http.StatusBadRequest,
			fmt.Sprintf("shoutout template must be at most %d characters", MaxShoutoutTemplateLength))
		return
	}
	if strings.ContainsAny(body.Template, "\r\n\x00") {
		common.RespondError(w, http.StatusBadRequest, "shoutout template must not contain control characters")
		return
	}
	if body.Template != "" && strings.Count(body.Template, shoutoutPlaceholder) != 1 {
		common.RespondError(w, http.StatusBadRequest,
			"shoutout template must contain exactly one "+shoutoutPlaceholder+" placeholder")
		return
	}

	if err := h.Users.SetShoutoutTemplate(r.Context(), userID, body.Template); err != nil {
		common.RespondError(w, http.StatusInternalServerError, "failed to save shoutout template")
		return
	}

	common.RespondJSON(w, http.StatusOK, map[string]string{
		"message":  "shoutout template updated",
		"template": body.Template,
	})
}

func (h *SettingsHandler) SetCommunityDiscord(w http.ResponseWriter, r *http.Request) {
	communityID, ok := common.ValidateCommunityID(w, r)
	if !ok {
		return
	}
	userID := middleware.GetUserID(r.Context())

	isMod, err := h.Communities.IsModeratorOrOwner(r.Context(), communityID, userID)
	if err != nil {
		common.RespondError(w, http.StatusInternalServerError, "failed to check permissions")
		return
	}
	if !isMod {
		common.RespondError(w, http.StatusForbidden, "forbidden")
		return
	}

	var body communityDiscordSettings
	if err := common.DecodeJSON(r, &body); err != nil {
		common.RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.GuildID == "" {
		common.RespondError(w, http.StatusBadRequest, "guild_id is required")
		return
	}
	if body.LiveChannelID == "" {
		common.RespondError(w, http.StatusBadRequest, "live_channel_id is required")
		return
	}

	ses, err := h.Sessions.Get(r.Context(), middleware.GetSessionID(r.Context()))
	if err != nil || ses == nil || ses.DiscordAccessToken == "" {
		common.RespondError(w, http.StatusForbidden, "re-login required to configure discord settings")
		return
	}

	channel, err := h.Discord.GetChannel(r.Context(), h.DiscordBotToken, body.LiveChannelID)
	if err != nil {
		common.RespondError(w, http.StatusBadRequest, "live channel is not accessible to the commbadge bot")
		return
	}
	if channel.GuildID != body.GuildID {
		common.RespondError(w, http.StatusBadRequest, "live_channel_id is not in the given guild")
		return
	}
	if err := h.verifyDiscordAccess(r.Context(), ses.DiscordAccessToken, body.GuildID); err != nil {
		common.RespondError(w, http.StatusForbidden, "you need Manage Channels permission in that server")
		return
	}

	if err := h.Communities.SetDiscordConfig(r.Context(), communityID, body.GuildID, body.LiveChannelID); err != nil {
		common.RespondError(w, http.StatusInternalServerError, "failed to save community discord settings")
		return
	}

	common.RespondJSON(w, http.StatusOK, map[string]string{
		"message":         "community discord settings updated",
		"guild_id":        body.GuildID,
		"live_channel_id": body.LiveChannelID,
	})
}