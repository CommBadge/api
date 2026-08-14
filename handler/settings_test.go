package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"kronus.dev/commbadge_api/internal/testutil"
	"kronus.dev/commbadge_api/store"
)

func testSettingsHandler(t *testing.T) *SettingsHandler {
	t.Helper()
	return &SettingsHandler{
		Users:       testutil.NewMockUserStore(),
		Communities: testutil.NewMockCommunityStore(),
	}
}

func TestSettingsHandler_SetNotificationChannel(t *testing.T) {
	h := testSettingsHandler(t)
	body := `{"channel_id":"300000000000000001"}`
	req := httptest.NewRequest("PATCH", "/me/discord-notification", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := authContext(req.Context(), "user-1")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	h.SetNotificationChannel(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]string
	if err := decodeJSONBody(rr, &resp); err != nil {
		t.Fatal(err)
	}
	if resp["channel_id"] != "300000000000000001" {
		t.Fatalf("expected channel_id in response, got %v", resp)
	}
}

func TestSettingsHandler_SetNotificationChannel_MissingChannelID(t *testing.T) {
	h := testSettingsHandler(t)
	req := httptest.NewRequest("PATCH", "/me/discord-notification", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	ctx := authContext(req.Context(), "user-1")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	h.SetNotificationChannel(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestSettingsHandler_SetShoutoutTemplate(t *testing.T) {
	h := testSettingsHandler(t)
	body := `{"template":"Go check out {twitch_url} and say hi!"}`
	req := httptest.NewRequest("PATCH", "/me/shoutout-template", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := authContext(req.Context(), "user-1")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	h.SetShoutoutTemplate(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]string
	if err := decodeJSONBody(rr, &resp); err != nil {
		t.Fatal(err)
	}
	if resp["template"] != "Go check out {twitch_url} and say hi!" {
		t.Fatalf("unexpected template in response: %v", resp)
	}
}

func TestSettingsHandler_SetShoutoutTemplate_Clear(t *testing.T) {
	h := testSettingsHandler(t)
	req := httptest.NewRequest("PATCH", "/me/shoutout-template", strings.NewReader(`{"template":""}`))
	req.Header.Set("Content-Type", "application/json")
	ctx := authContext(req.Context(), "user-1")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	h.SetShoutoutTemplate(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestSettingsHandler_SetShoutoutTemplate_MissingPlaceholder(t *testing.T) {
	h := testSettingsHandler(t)
	req := httptest.NewRequest("PATCH", "/me/shoutout-template", strings.NewReader(`{"template":"no placeholder here"}`))
	req.Header.Set("Content-Type", "application/json")
	ctx := authContext(req.Context(), "user-1")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	h.SetShoutoutTemplate(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestSettingsHandler_SetShoutoutTemplate_MultiplePlaceholders(t *testing.T) {
	h := testSettingsHandler(t)
	req := httptest.NewRequest("PATCH", "/me/shoutout-template", strings.NewReader(`{"template":"{twitch_url} and {twitch_url}"}`))
	req.Header.Set("Content-Type", "application/json")
	ctx := authContext(req.Context(), "user-1")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	h.SetShoutoutTemplate(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestSettingsHandler_SetShoutoutTemplate_TooLong(t *testing.T) {
	h := testSettingsHandler(t)
	body := `{"template":"` + strings.Repeat("x", MaxShoutoutTemplateLength+1) + `"}`
	req := httptest.NewRequest("PATCH", "/me/shoutout-template", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := authContext(req.Context(), "user-1")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	h.SetShoutoutTemplate(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

// TestSettingsHandler_SetShoutoutTemplate_SQLInjectionTreatedAsData verifies
// that freeform template input containing SQL is stored as literal text
// rather than being interpreted. The store binds the value as a query
// parameter, so it can never be concatenated into a SQL statement.
func TestSettingsHandler_SetShoutoutTemplate_SQLInjectionTreatedAsData(t *testing.T) {
	h := testSettingsHandler(t)
	mock := h.Users.(*testutil.MockUserStore)
	mock.Users["user-1"] = &store.User{ID: "user-1", Username: "kronus", DisplayName: "Kronus"}
	payload := "Go check out {twitch_url}'); DROP TABLE users;--"
	body, _ := json.Marshal(map[string]string{"template": payload})
	req := httptest.NewRequest("PATCH", "/me/shoutout-template", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	ctx := authContext(req.Context(), "user-1")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	h.SetShoutoutTemplate(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	u, err := h.Users.GetUserByID(req.Context(), "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if u.ShoutoutTemplate != payload {
		t.Fatalf("expected template stored verbatim, got %q", u.ShoutoutTemplate)
	}
}

func TestSettingsHandler_SetCommunityDiscord(t *testing.T) {
	h := testSettingsHandler(t)
	com, _ := h.Communities.Create(nil, "Testers", "desc", "user-1")
	body := `{"guild_id":"200000000000000001","live_channel_id":"300000000000000006"}`
	req := httptest.NewRequest("PATCH", "/communities/"+com.ID+"/discord", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("communityID", com.ID)
	ctx := authContext(req.Context(), "user-1")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	h.SetCommunityDiscord(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]string
	if err := decodeJSONBody(rr, &resp); err != nil {
		t.Fatal(err)
	}
	if resp["guild_id"] != "200000000000000001" || resp["live_channel_id"] != "300000000000000006" {
		t.Fatalf("unexpected response: %v", resp)
	}
}

func TestSettingsHandler_SetCommunityDiscord_Forbidden(t *testing.T) {
	h := testSettingsHandler(t)
	com, _ := h.Communities.Create(nil, "Testers", "desc", "user-1")
	body := `{"guild_id":"200000000000000001","live_channel_id":"300000000000000006"}`
	req := httptest.NewRequest("PATCH", "/communities/"+com.ID+"/discord", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("communityID", com.ID)
	ctx := authContext(req.Context(), "other-user")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	h.SetCommunityDiscord(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestSettingsHandler_SetCommunityDiscord_MissingFields(t *testing.T) {
	h := testSettingsHandler(t)
	com, _ := h.Communities.Create(nil, "Testers", "desc", "user-1")
	req := httptest.NewRequest("PATCH", "/communities/"+com.ID+"/discord", strings.NewReader(`{"guild_id":"200000000000000001"}`))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("communityID", com.ID)
	ctx := authContext(req.Context(), "user-1")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	h.SetCommunityDiscord(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestSettingsHandler_SetCommunityDiscord_InvalidCommunityID(t *testing.T) {
	h := testSettingsHandler(t)
	req := httptest.NewRequest("PATCH", "/communities/not-a-uuid/discord", strings.NewReader(`{"guild_id":"g","live_channel_id":"c"}`))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("communityID", "not-a-uuid")
	ctx := authContext(req.Context(), "user-1")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	h.SetCommunityDiscord(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}
