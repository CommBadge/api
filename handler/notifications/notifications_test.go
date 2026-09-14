package notifications

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"kronus.dev/commbadge_api/store"
	"kronus.dev/commbadge_api/twitch"
)

// setupSubscribeSuccess configures the happy-path expectations for Subscribe.
func setupSubscribeSuccess(s *setup, eventType string) {
	s.users.On("GetTwitchID", mock.Anything, "user-1").Return("twitch-user-1", nil).Once()
	s.subs.On("HasActive", mock.Anything, "user-1", eventType).Return(false, nil).Once()
	s.esub.On("GetAppAccessToken", mock.Anything).Return("mock-app-token", nil).Once()
	s.esub.On("CreateSubscription", mock.Anything, "mock-app-token", eventType, "1", mock.Anything, "https://example.com/webhooks", "test-secret").
		Return(&twitch.EventSubSubscription{ID: "eventsub-1"}, nil).Once()
	s.subs.On("Create", mock.Anything, "user-1", eventType, mock.Anything, "eventsub-1").
		Return(&store.NotificationSubscription{
			ID:                   "sub-1",
			UserID:               "user-1",
			TwitchEventType:      eventType,
			TwitchSubscriptionID: "eventsub-1",
			Status:               store.SubscriptionEnabled,
		}, nil).Once()
}

func TestNotificationHandler_Subscribe(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		s := testNotificationHandler(t)
		setupSubscribeSuccess(s, "stream.online")
		rr := httptest.NewRecorder()
		s.h.Subscribe(rr, authReq("POST", "/x", `{"event_type":"stream.online"}`, "user-1"))
		require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	})

	t.Run("unauthorized", func(t *testing.T) {
		s := testNotificationHandler(t)
		req := httptest.NewRequest("POST", "/x", nil)
		rr := httptest.NewRecorder()
		s.h.Subscribe(rr, req)
		require.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("bad body", func(t *testing.T) {
		s := testNotificationHandler(t)
		rr := httptest.NewRecorder()
		s.h.Subscribe(rr, authReq("POST", "/x", `{oops`, "user-1"))
		require.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("unsupported event type", func(t *testing.T) {
		s := testNotificationHandler(t)
		rr := httptest.NewRecorder()
		s.h.Subscribe(rr, authReq("POST", "/x", `{"event_type":"channel.follow"}`, "user-1"))
		require.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("no twitch link", func(t *testing.T) {
		s := testNotificationHandler(t)
		s.users.On("GetTwitchID", mock.Anything, "user-1").Return("", nil).Once()
		rr := httptest.NewRecorder()
		s.h.Subscribe(rr, authReq("POST", "/x", `{"event_type":"stream.online"}`, "user-1"))
		require.Equal(t, http.StatusConflict, rr.Code)
	})

	t.Run("already subscribed", func(t *testing.T) {
		s := testNotificationHandler(t)
		s.users.On("GetTwitchID", mock.Anything, "user-1").Return("twitch-user-1", nil).Once()
		s.subs.On("HasActive", mock.Anything, "user-1", "stream.online").Return(true, nil).Once()
		rr := httptest.NewRecorder()
		s.h.Subscribe(rr, authReq("POST", "/x", `{"event_type":"stream.online"}`, "user-1"))
		require.Equal(t, http.StatusConflict, rr.Code)
	})

	t.Run("get twitch id error", func(t *testing.T) {
		s := testNotificationHandler(t)
		s.users.On("GetTwitchID", mock.Anything, "user-1").Return("", errH).Once()
		rr := httptest.NewRecorder()
		s.h.Subscribe(rr, authReq("POST", "/x", `{"event_type":"stream.online"}`, "user-1"))
		require.Equal(t, http.StatusInternalServerError, rr.Code)
	})

	t.Run("has active check error", func(t *testing.T) {
		s := testNotificationHandler(t)
		s.users.On("GetTwitchID", mock.Anything, "user-1").Return("twitch-user-1", nil).Once()
		s.subs.On("HasActive", mock.Anything, "user-1", "stream.online").Return(false, errH).Once()
		rr := httptest.NewRecorder()
		s.h.Subscribe(rr, authReq("POST", "/x", `{"event_type":"stream.online"}`, "user-1"))
		require.Equal(t, http.StatusInternalServerError, rr.Code)
	})

	t.Run("app token error", func(t *testing.T) {
		s := testNotificationHandler(t)
		s.users.On("GetTwitchID", mock.Anything, "user-1").Return("twitch-user-1", nil).Once()
		s.subs.On("HasActive", mock.Anything, "user-1", "stream.online").Return(false, nil).Once()
		s.esub.On("GetAppAccessToken", mock.Anything).Return("", errH).Once()
		rr := httptest.NewRecorder()
		s.h.Subscribe(rr, authReq("POST", "/x", `{"event_type":"stream.online"}`, "user-1"))
		require.Equal(t, http.StatusBadGateway, rr.Code)
	})

	t.Run("create subscription error", func(t *testing.T) {
		s := testNotificationHandler(t)
		s.users.On("GetTwitchID", mock.Anything, "user-1").Return("twitch-user-1", nil).Once()
		s.subs.On("HasActive", mock.Anything, "user-1", "stream.online").Return(false, nil).Once()
		s.esub.On("GetAppAccessToken", mock.Anything).Return("mock-app-token", nil).Once()
		s.esub.On("CreateSubscription", mock.Anything, "mock-app-token", "stream.online", "1", mock.Anything, "https://example.com/webhooks", "test-secret").
			Return(nil, errH).Once()
		rr := httptest.NewRecorder()
		s.h.Subscribe(rr, authReq("POST", "/x", `{"event_type":"stream.online"}`, "user-1"))
		require.Equal(t, http.StatusBadGateway, rr.Code)
	})

	t.Run("persist error", func(t *testing.T) {
		s := testNotificationHandler(t)
		s.users.On("GetTwitchID", mock.Anything, "user-1").Return("twitch-user-1", nil).Once()
		s.subs.On("HasActive", mock.Anything, "user-1", "stream.offline").Return(false, nil).Once()
		s.esub.On("GetAppAccessToken", mock.Anything).Return("mock-app-token", nil).Once()
		s.esub.On("CreateSubscription", mock.Anything, "mock-app-token", "stream.offline", "1", mock.Anything, "https://example.com/webhooks", "test-secret").
			Return(&twitch.EventSubSubscription{ID: "eventsub-1"}, nil).Once()
		s.subs.On("Create", mock.Anything, "user-1", "stream.offline", mock.Anything, "eventsub-1").
			Return(nil, errH).Once()
		rr := httptest.NewRecorder()
		s.h.Subscribe(rr, authReq("POST", "/x", `{"event_type":"stream.offline"}`, "user-1"))
		require.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

func TestNotificationHandler_List(t *testing.T) {
	t.Run("success with empty list", func(t *testing.T) {
		s := testNotificationHandler(t)
		s.subs.On("ListActiveByUser", mock.Anything, "user-1").Return(nil, nil).Once()
		rr := httptest.NewRecorder()
		s.h.List(rr, authReq("GET", "/x", "", "user-1"))
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		require.Equal(t, "[]", strings.TrimSpace(rr.Body.String()))
	})

	t.Run("success", func(t *testing.T) {
		s := testNotificationHandler(t)
		s.subs.On("ListActiveByUser", mock.Anything, "user-1").
			Return([]store.NotificationSubscription{{
				ID:              "sub-1",
				UserID:          "user-1",
				TwitchEventType: "stream.online",
				Status:          store.SubscriptionEnabled,
			}}, nil).Once()
		rr := httptest.NewRecorder()
		s.h.List(rr, authReq("GET", "/x", "", "user-1"))
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	})

	t.Run("unauthorized", func(t *testing.T) {
		s := testNotificationHandler(t)
		rr := httptest.NewRecorder()
		s.h.List(rr, httptest.NewRequest("GET", "/x", nil))
		require.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("store error", func(t *testing.T) {
		s := testNotificationHandler(t)
		s.subs.On("ListActiveByUser", mock.Anything, "user-1").Return(nil, errH).Once()
		rr := httptest.NewRecorder()
		s.h.List(rr, authReq("GET", "/x", "", "user-1"))
		require.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

func TestNotificationHandler_Delete(t *testing.T) {
	sub := &store.NotificationSubscription{
		ID:                   "sub-1",
		UserID:               "user-1",
		TwitchEventType:      "stream.online",
		TwitchSubscriptionID: "eventsub-1",
		Status:               store.SubscriptionEnabled,
	}

	t.Run("success", func(t *testing.T) {
		s := testNotificationHandler(t)
		s.subs.On("GetByID", mock.Anything, "sub-1").Return(sub, nil).Once()
		s.esub.On("GetAppAccessToken", mock.Anything).Return("mock-app-token", nil).Once()
		s.esub.On("DeleteSubscription", mock.Anything, "mock-app-token", "eventsub-1").Return(nil).Once()
		s.subs.On("MarkRevoked", mock.Anything, "sub-1").Return(nil).Once()

		req := authReq("DELETE", "/x", "", "user-1")
		req.SetPathValue("id", "sub-1")
		rr := httptest.NewRecorder()
		s.h.Delete(rr, req)
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		s.esub.AssertCalled(t, "DeleteSubscription", mock.Anything, "mock-app-token", "eventsub-1")
	})

	t.Run("not owner", func(t *testing.T) {
		s := testNotificationHandler(t)
		s.subs.On("GetByID", mock.Anything, "sub-1").Return(sub, nil).Once()
		req := authReq("DELETE", "/x", "", "user-2")
		req.SetPathValue("id", "sub-1")
		rr := httptest.NewRecorder()
		s.h.Delete(rr, req)
		require.Equal(t, http.StatusForbidden, rr.Code)
	})

	t.Run("not found", func(t *testing.T) {
		s := testNotificationHandler(t)
		s.subs.On("GetByID", mock.Anything, "absent").Return(nil, nil).Once()
		req := authReq("DELETE", "/x", "", "user-1")
		req.SetPathValue("id", "absent")
		rr := httptest.NewRecorder()
		s.h.Delete(rr, req)
		require.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("unauthorized", func(t *testing.T) {
		s := testNotificationHandler(t)
		rr := httptest.NewRecorder()
		s.h.Delete(rr, httptest.NewRequest("DELETE", "/x", nil))
		require.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("get error", func(t *testing.T) {
		s := testNotificationHandler(t)
		s.subs.On("GetByID", mock.Anything, "absent").Return(nil, errH).Once()
		req := authReq("DELETE", "/x", "", "user-1")
		req.SetPathValue("id", "absent")
		rr := httptest.NewRecorder()
		s.h.Delete(rr, req)
		require.Equal(t, http.StatusInternalServerError, rr.Code)
	})

	t.Run("app token error", func(t *testing.T) {
		s := testNotificationHandler(t)
		s.subs.On("GetByID", mock.Anything, "sub-1").Return(sub, nil).Once()
		s.esub.On("GetAppAccessToken", mock.Anything).Return("", errH).Once()
		req := authReq("DELETE", "/x", "", "user-1")
		req.SetPathValue("id", "sub-1")
		rr := httptest.NewRecorder()
		s.h.Delete(rr, req)
		require.Equal(t, http.StatusBadGateway, rr.Code)
	})

	t.Run("delete subscription error", func(t *testing.T) {
		s := testNotificationHandler(t)
		s.subs.On("GetByID", mock.Anything, "sub-1").Return(sub, nil).Once()
		s.esub.On("GetAppAccessToken", mock.Anything).Return("mock-app-token", nil).Once()
		s.esub.On("DeleteSubscription", mock.Anything, "mock-app-token", "eventsub-1").Return(errH).Once()
		req := authReq("DELETE", "/x", "", "user-1")
		req.SetPathValue("id", "sub-1")
		rr := httptest.NewRecorder()
		s.h.Delete(rr, req)
		require.Equal(t, http.StatusBadGateway, rr.Code)
	})

	t.Run("mark revoked error", func(t *testing.T) {
		s := testNotificationHandler(t)
		s.subs.On("GetByID", mock.Anything, "sub-1").Return(sub, nil).Once()
		s.esub.On("GetAppAccessToken", mock.Anything).Return("mock-app-token", nil).Once()
		s.esub.On("DeleteSubscription", mock.Anything, "mock-app-token", "eventsub-1").Return(nil).Once()
		s.subs.On("MarkRevoked", mock.Anything, "sub-1").Return(errH).Once()
		req := authReq("DELETE", "/x", "", "user-1")
		req.SetPathValue("id", "sub-1")
		rr := httptest.NewRecorder()
		s.h.Delete(rr, req)
		require.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}