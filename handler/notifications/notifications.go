// Package notifications implements the Twitch EventSub subscription lifecycle
// that powers stream live/offline notifications.
package notifications

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"kronus.dev/commbadge_api/handler/common"
	"kronus.dev/commbadge_api/handler/contracts"
	"kronus.dev/commbadge_api/middleware"
	"kronus.dev/commbadge_api/store"
)

const (
	eventStreamOnline  = "stream.online"
	eventStreamOffline = "stream.offline"
	eventSubVersion    = "1"
)

type NotificationHandler struct {
	Users       contracts.UserRepository
	Subs        contracts.SubscriptionRepository
	EventSub    contracts.TwitchEventSubClient
	CallbackURL string
	Secret      string
}

type subscribeRequest struct {
	EventType string `json:"event_type"`
}

func validEventType(t string) bool {
	return t == eventStreamOnline || t == eventStreamOffline
}

func (h *NotificationHandler) Subscribe(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		common.RespondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req subscribeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if !validEventType(req.EventType) {
		common.RespondError(w, http.StatusBadRequest, "unsupported event type")
		return
	}

	twitchID, err := h.Users.GetTwitchID(r.Context(), userID)
	if err != nil {
		slog.Error("get twitch id error", "error", err)
		common.RespondError(w, http.StatusInternalServerError, "failed to load twitch link")
		return
	}
	if twitchID == "" {
		common.RespondError(w, http.StatusConflict, "twitch account required")
		return
	}

	exists, err := h.Subs.HasActive(r.Context(), userID, req.EventType)
	if err != nil {
		slog.Error("check subscription error", "error", err)
		common.RespondError(w, http.StatusInternalServerError, "failed to check subscriptions")
		return
	}
	if exists {
		common.RespondError(w, http.StatusConflict, "already subscribed to this event")
		return
	}

	appToken, err := h.EventSub.GetAppAccessToken(r.Context())
	if err != nil {
		slog.Error("app token error", "error", err)
		common.RespondError(w, http.StatusBadGateway, "failed to reach twitch")
		return
	}

	condition := map[string]string{"broadcaster_user_id": twitchID}
	sub, err := h.EventSub.CreateSubscription(r.Context(), appToken, req.EventType, eventSubVersion, condition, h.CallbackURL, h.Secret)
	if err != nil {
		slog.Error("create subscription error", "error", err)
		common.RespondError(w, http.StatusBadGateway, "failed to create subscription")
		return
	}

	record, err := h.Subs.Create(r.Context(), userID, req.EventType, condition, sub.ID)
	if err != nil {
		slog.Error("persist subscription error", "error", err)
		common.RespondError(w, http.StatusInternalServerError, "failed to save subscription")
		return
	}

	common.RespondJSON(w, http.StatusCreated, record)
}

func (h *NotificationHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		common.RespondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	subs, err := h.Subs.ListActiveByUser(r.Context(), userID)
	if err != nil {
		slog.Error("list subscriptions error", "error", err)
		common.RespondError(w, http.StatusInternalServerError, "failed to list subscriptions")
		return
	}
	if subs == nil {
		subs = []store.NotificationSubscription{}
	}
	common.RespondJSON(w, http.StatusOK, subs)
}

func (h *NotificationHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		common.RespondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id := r.PathValue("id")
	sub, err := h.Subs.GetByID(r.Context(), id)
	if err != nil {
		slog.Error("get subscription error", "error", err)
		common.RespondError(w, http.StatusInternalServerError, "failed to load subscription")
		return
	}
	if sub == nil || sub.Status != store.SubscriptionEnabled {
		common.RespondError(w, http.StatusNotFound, "subscription not found")
		return
	}
	if sub.UserID != userID {
		common.RespondError(w, http.StatusForbidden, "forbidden")
		return
	}

	appToken, err := h.EventSub.GetAppAccessToken(r.Context())
	if err != nil {
		slog.Error("app token error", "error", err)
		common.RespondError(w, http.StatusBadGateway, "failed to reach twitch")
		return
	}

	if err := h.EventSub.DeleteSubscription(r.Context(), appToken, sub.TwitchSubscriptionID); err != nil {
		slog.Error("delete subscription error", "error", err)
		common.RespondError(w, http.StatusBadGateway, "failed to revoke subscription")
		return
	}

	if err := h.Subs.MarkRevoked(r.Context(), sub.ID); err != nil {
		slog.Error("mark revoked error", "error", err)
		common.RespondError(w, http.StatusInternalServerError, "failed to update subscription")
		return
	}

	common.RespondJSON(w, http.StatusOK, map[string]string{"message": "subscription revoked"})
}