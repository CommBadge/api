package settings

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"kronus.dev/commbadge_api/discord"
	"kronus.dev/commbadge_api/session"
)

func manageChannelsGuilds() []discord.Guild {
	return []discord.Guild{{ID: guildID, Name: "Mock Guild", Permissions: "16"}}
}

func testSession() *session.Session {
	return &session.Session{UserID: "user-1", DiscordAccessToken: "mock-access"}
}

func TestSettingsHandler_SetNotificationChannel(t *testing.T) {
	t.Run("sets channel", func(t *testing.T) {
		fx := testSettingsHandler(t)
		fx.sessions.On("Get", anyCtx(), "test-session").Return(testSession(), nil).Once()
		fx.discord.On("GetChannel", anyCtx(), "mock-bot-token", channelID).Return(&discord.Channel{ID: channelID, GuildID: guildID}, nil).Once()
		fx.discord.On("GetUserGuilds", anyCtx(), "mock-access").Return(manageChannelsGuilds(), nil).Once()
		fx.users.On("SetNotificationChannel", anyCtx(), "user-1", channelID).Return(nil).Once()
		rr := httptest.NewRecorder()
		fx.h.SetNotificationChannel(rr, authReq("POST", "/x", `{"channel_id":"`+channelID+`"}`, "user-1"))
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d: %s", rr.Code, rr.Body.String())
		}
		fx.users.AssertCalled(t, "SetNotificationChannel", anyCtx(), "user-1", channelID)
	})

	t.Run("missing channel id", func(t *testing.T) {
		fx := testSettingsHandler(t)
		rr := httptest.NewRecorder()
		fx.h.SetNotificationChannel(rr, authReq("POST", "/x", `{}`, "user-1"))
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d", rr.Code)
		}
	})

	t.Run("no discord session", func(t *testing.T) {
		fx := testSettingsHandler(t)
		fx.sessions.On("Get", anyCtx(), "test-session").Return(&session.Session{UserID: "user-1", DiscordAccessToken: ""}, nil).Once()
		req := authReq("POST", "/x", `{"channel_id":"`+channelID+`"}`, "user-1")
		req = req.WithContext(authContext(req.Context(), "user-1"))
		rr := httptest.NewRecorder()
		fx.h.SetNotificationChannel(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Fatalf("status = %d", rr.Code)
		}
		fx.discord.AssertNotCalled(t, "GetChannel", anyCtx(), anyCtx(), anyCtx())
	})

	t.Run("channel not accessible to bot", func(t *testing.T) {
		fx := testSettingsHandler(t)
		fx.sessions.On("Get", anyCtx(), "test-session").Return(testSession(), nil).Once()
		fx.discord.On("GetChannel", anyCtx(), "mock-bot-token", channelID).Return(nil, errH).Once()
		rr := httptest.NewRecorder()
		fx.h.SetNotificationChannel(rr, authReq("POST", "/x", `{"channel_id":"`+channelID+`"}`, "user-1"))
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d", rr.Code)
		}
	})

	t.Run("user lacks manage channels", func(t *testing.T) {
		fx := testSettingsHandler(t)
		fx.sessions.On("Get", anyCtx(), "test-session").Return(testSession(), nil).Once()
		fx.discord.On("GetChannel", anyCtx(), "mock-bot-token", channelID).Return(&discord.Channel{ID: channelID, GuildID: guildID}, nil).Once()
		fx.discord.On("GetUserGuilds", anyCtx(), "mock-access").Return([]discord.Guild{{ID: guildID, Permissions: "0"}}, nil).Once()
		rr := httptest.NewRecorder()
		fx.h.SetNotificationChannel(rr, authReq("POST", "/x", `{"channel_id":"`+channelID+`"}`, "user-1"))
		if rr.Code != http.StatusForbidden {
			t.Fatalf("status = %d", rr.Code)
		}
		fx.users.AssertNotCalled(t, "SetNotificationChannel", anyCtx(), anyCtx(), anyCtx())
	})

	t.Run("save failure", func(t *testing.T) {
		fx := testSettingsHandler(t)
		fx.sessions.On("Get", anyCtx(), "test-session").Return(testSession(), nil).Once()
		fx.discord.On("GetChannel", anyCtx(), "mock-bot-token", channelID).Return(&discord.Channel{ID: channelID, GuildID: guildID}, nil).Once()
		fx.discord.On("GetUserGuilds", anyCtx(), "mock-access").Return(manageChannelsGuilds(), nil).Once()
		fx.users.On("SetNotificationChannel", anyCtx(), "user-1", channelID).Return(errH).Once()
		rr := httptest.NewRecorder()
		fx.h.SetNotificationChannel(rr, authReq("POST", "/x", `{"channel_id":"`+channelID+`"}`, "user-1"))
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d", rr.Code)
		}
	})
}

func TestSettingsHandler_SetShoutoutTemplate(t *testing.T) {
	t.Run("valid template", func(t *testing.T) {
		fx := testSettingsHandler(t)
		fx.users.On("SetShoutoutTemplate", anyCtx(), "user-1", "Check out {twitch_url}!").Return(nil).Once()
		rr := httptest.NewRecorder()
		fx.h.SetShoutoutTemplate(rr, authReq("POST", "/x", `{"template":"Check out {twitch_url}!"}`, "user-1"))
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d: %s", rr.Code, rr.Body.String())
		}
		fx.users.AssertCalled(t, "SetShoutoutTemplate", anyCtx(), "user-1", "Check out {twitch_url}!")
	})

	t.Run("empty clears", func(t *testing.T) {
		fx := testSettingsHandler(t)
		fx.users.On("SetShoutoutTemplate", anyCtx(), "user-1", "").Return(nil).Once()
		rr := httptest.NewRecorder()
		fx.h.SetShoutoutTemplate(rr, authReq("POST", "/x", `{"template":""}`, "user-1"))
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d", rr.Code)
		}
		fx.users.AssertCalled(t, "SetShoutoutTemplate", anyCtx(), "user-1", "")
	})

	t.Run("missing placeholder", func(t *testing.T) {
		fx := testSettingsHandler(t)
		rr := httptest.NewRecorder()
		fx.h.SetShoutoutTemplate(rr, authReq("POST", "/x", `{"template":"no placeholder"}`, "user-1"))
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d", rr.Code)
		}
	})

	t.Run("too many placeholders", func(t *testing.T) {
		fx := testSettingsHandler(t)
		rr := httptest.NewRecorder()
		fx.h.SetShoutoutTemplate(rr, authReq("POST", "/x", `{"template":"{twitch_url} {twitch_url}"}`, "user-1"))
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d", rr.Code)
		}
	})

	t.Run("too long", func(t *testing.T) {
		fx := testSettingsHandler(t)
		long := strings.Repeat("a", MaxShoutoutTemplateLength+1)
		rr := httptest.NewRecorder()
		fx.h.SetShoutoutTemplate(rr, authReq("POST", "/x", `{"template":"`+long+`"}`, "user-1"))
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d", rr.Code)
		}
	})

	t.Run("control characters rejected", func(t *testing.T) {
		fx := testSettingsHandler(t)
		rr := httptest.NewRecorder()
		fx.h.SetShoutoutTemplate(rr, authReq("POST", "/x", `{"template":"line\n{twitch_url}"}`, "user-1"))
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d", rr.Code)
		}
	})

	t.Run("save failure", func(t *testing.T) {
		fx := testSettingsHandler(t)
		fx.users.On("SetShoutoutTemplate", anyCtx(), "user-1", "{twitch_url}").Return(errH).Once()
		rr := httptest.NewRecorder()
		fx.h.SetShoutoutTemplate(rr, authReq("POST", "/x", `{"template":"{twitch_url}"}`, "user-1"))
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d", rr.Code)
		}
	})
}

func TestSettingsHandler_SetCommunityDiscord(t *testing.T) {
	const comID = "00000000-0000-0000-0000-000000000001"
	valid := `{"guild_id":"200000000000000001","live_channel_id":"300000000000000006"}`

	t.Run("sets config", func(t *testing.T) {
		fx := testSettingsHandler(t)
		fx.coms.On("IsModeratorOrOwner", anyCtx(), comID, "user-1").Return(true, nil).Once()
		fx.sessions.On("Get", anyCtx(), "test-session").Return(testSession(), nil).Once()
		fx.discord.On("GetChannel", anyCtx(), "mock-bot-token", channelID).Return(&discord.Channel{ID: channelID, GuildID: guildID}, nil).Once()
		fx.discord.On("GetUserGuilds", anyCtx(), "mock-access").Return(manageChannelsGuilds(), nil).Once()
		fx.coms.On("SetDiscordConfig", anyCtx(), comID, guildID, channelID).Return(nil).Once()
		req := authReq("POST", "/x", valid, "user-1")
		req.SetPathValue("communityID", comID)
		rr := httptest.NewRecorder()
		fx.h.SetCommunityDiscord(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d: %s", rr.Code, rr.Body.String())
		}
		var resp map[string]string
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatal(err)
		}
		if resp["guild_id"] != guildID || resp["live_channel_id"] != channelID {
			t.Fatalf("resp = %v", resp)
		}
		fx.coms.AssertCalled(t, "SetDiscordConfig", anyCtx(), comID, guildID, channelID)
	})

	t.Run("missing guild", func(t *testing.T) {
		fx := testSettingsHandler(t)
		fx.coms.On("IsModeratorOrOwner", anyCtx(), comID, "user-1").Return(true, nil).Once()
		req := authReq("POST", "/x", `{"live_channel_id":"300000000000000006"}`, "user-1")
		req.SetPathValue("communityID", comID)
		rr := httptest.NewRecorder()
		fx.h.SetCommunityDiscord(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d", rr.Code)
		}
	})

	t.Run("missing live channel", func(t *testing.T) {
		fx := testSettingsHandler(t)
		fx.coms.On("IsModeratorOrOwner", anyCtx(), comID, "user-1").Return(true, nil).Once()
		req := authReq("POST", "/x", `{"guild_id":"200000000000000001"}`, "user-1")
		req.SetPathValue("communityID", comID)
		rr := httptest.NewRecorder()
		fx.h.SetCommunityDiscord(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d", rr.Code)
		}
	})

	t.Run("not a moderator", func(t *testing.T) {
		fx := testSettingsHandler(t)
		fx.coms.On("IsModeratorOrOwner", anyCtx(), comID, "user-1").Return(false, nil).Once()
		req := authReq("POST", "/x", valid, "user-1")
		req.SetPathValue("communityID", comID)
		rr := httptest.NewRecorder()
		fx.h.SetCommunityDiscord(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Fatalf("status = %d", rr.Code)
		}
	})

	t.Run("channel not in guild", func(t *testing.T) {
		fx := testSettingsHandler(t)
		fx.coms.On("IsModeratorOrOwner", anyCtx(), comID, "user-1").Return(true, nil).Once()
		fx.sessions.On("Get", anyCtx(), "test-session").Return(testSession(), nil).Once()
		fx.discord.On("GetChannel", anyCtx(), "mock-bot-token", channelID).Return(&discord.Channel{ID: channelID, GuildID: "999999999999999999"}, nil).Once()
		req := authReq("POST", "/x", valid, "user-1")
		req.SetPathValue("communityID", comID)
		rr := httptest.NewRecorder()
		fx.h.SetCommunityDiscord(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d", rr.Code)
		}
		fx.coms.AssertNotCalled(t, "SetDiscordConfig", anyCtx(), anyCtx(), anyCtx(), anyCtx())
	})

	t.Run("user lacks manage channels", func(t *testing.T) {
		fx := testSettingsHandler(t)
		fx.coms.On("IsModeratorOrOwner", anyCtx(), comID, "user-1").Return(true, nil).Once()
		fx.sessions.On("Get", anyCtx(), "test-session").Return(testSession(), nil).Once()
		fx.discord.On("GetChannel", anyCtx(), "mock-bot-token", channelID).Return(&discord.Channel{ID: channelID, GuildID: guildID}, nil).Once()
		fx.discord.On("GetUserGuilds", anyCtx(), "mock-access").Return([]discord.Guild{{ID: guildID, Permissions: "0"}}, nil).Once()
		req := authReq("POST", "/x", valid, "user-1")
		req.SetPathValue("communityID", comID)
		rr := httptest.NewRecorder()
		fx.h.SetCommunityDiscord(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Fatalf("status = %d", rr.Code)
		}
		fx.coms.AssertNotCalled(t, "SetDiscordConfig", anyCtx(), anyCtx(), anyCtx(), anyCtx())
	})

	t.Run("save failure", func(t *testing.T) {
		fx := testSettingsHandler(t)
		fx.coms.On("IsModeratorOrOwner", anyCtx(), comID, "user-1").Return(true, nil).Once()
		fx.sessions.On("Get", anyCtx(), "test-session").Return(testSession(), nil).Once()
		fx.discord.On("GetChannel", anyCtx(), "mock-bot-token", channelID).Return(&discord.Channel{ID: channelID, GuildID: guildID}, nil).Once()
		fx.discord.On("GetUserGuilds", anyCtx(), "mock-access").Return(manageChannelsGuilds(), nil).Once()
		fx.coms.On("SetDiscordConfig", anyCtx(), comID, guildID, channelID).Return(errH).Once()
		req := authReq("POST", "/x", valid, "user-1")
		req.SetPathValue("communityID", comID)
		rr := httptest.NewRecorder()
		fx.h.SetCommunityDiscord(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d", rr.Code)
		}
	})

	t.Run("SetDiscordConfig fails", func(t *testing.T) {
		fx := testSettingsHandler(t)
		fx.coms.On("IsModeratorOrOwner", anyCtx(), comID, "user-1").Return(true, nil).Once()
		fx.sessions.On("Get", anyCtx(), "test-session").Return(testSession(), nil).Once()
		fx.discord.On("GetChannel", anyCtx(), "mock-bot-token", channelID).Return(&discord.Channel{ID: channelID, GuildID: guildID}, nil).Once()
		fx.discord.On("GetUserGuilds", anyCtx(), "mock-access").Return(manageChannelsGuilds(), nil).Once()
		fx.coms.On("SetDiscordConfig", anyCtx(), comID, guildID, channelID).Return(errH).Once()
		req := authReq("POST", "/x", valid, "user-1")
		req.SetPathValue("communityID", comID)
		rr := httptest.NewRecorder()
		fx.h.SetCommunityDiscord(rr, req)
		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d", rr.Code)
		}
	})
}

func TestSettingsHandler_SetCommunityDiscord_PermissionCheckError(t *testing.T) {
	const comID = "00000000-0000-0000-0000-000000000001"
	fx := testSettingsHandler(t)
	fx.coms.On("IsModeratorOrOwner", anyCtx(), comID, "user-1").Return(false, errH).Once()
	req := authReq("POST", "/x", `{"guild_id":"1","live_channel_id":"2"}`, "user-1")
	req.SetPathValue("communityID", comID)
	rr := httptest.NewRecorder()
	fx.h.SetCommunityDiscord(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rr.Code)
	}
}