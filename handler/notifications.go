package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"kronus.dev/commbadge_api/middleware"
	"kronus.dev/commbadge_api/store"
	"kronus.dev/commbadge_api/twitch"
)

const (
	eventStreamOnline  = "stream.online"
	eventStreamOffline = "stream.offline"
	eventSubVersion    = "1"
)

type SubscriptionRepository interface {
	Create(ctx context.Context, userID, eventType string, condition map[string]string, twitchSubscriptionID string) (*store.NotificationSubscription, error)
	GetByID(ctx context.Context, id string) (*store.NotificationSubscription, error)
	HasActive(ctx context.Context, userID, eventType string) (bool, error)
	ListActiveByUser(ctx context.Context, userID string) ([]store.NotificationSubscription, error)
	MarkRevoked(ctx context.Context, id string) error
	RevokeAllByUser(ctx context.Context, userID string) ([]store.NotificationSubscription, error)
}

type TwitchEventSubClient interface {
	GetAppAccessToken(ctx context.Context) (string, error)
	CreateSubscription(ctx context.Context, appToken, eventType, version string, condition map[string]string, callback, secret string) (*twitch.EventSubSubscription, error)
	DeleteSubscription(ctx context.Context, appToken, id string) error
}

type NotificationHandler struct {
	Users       UserRepository
	Subs        SubscriptionRepository
	EventSub    TwitchEventSubClient
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
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req subscribeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if !validEventType(req.EventType) {
		respondError(w, http.StatusBadRequest, "unsupported event type")
		return
	}

	twitchID, err := h.Users.GetTwitchID(r.Context(), userID)
	if err != nil {
		slog.Error("get twitch id error", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to load twitch link")
		return
	}
	if twitchID == "" {
		respondError(w, http.StatusConflict, "twitch account required")
		return
	}

	exists, err := h.Subs.HasActive(r.Context(), userID, req.EventType)
	if err != nil {
		slog.Error("check subscription error", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to check subscriptions")
		return
	}
	if exists {
		respondError(w, http.StatusConflict, "already subscribed to this event")
		return
	}

	appToken, err := h.EventSub.GetAppAccessToken(r.Context())
	if err != nil {
		slog.Error("app token error", "error", err)
		respondError(w, http.StatusBadGateway, "failed to reach twitch")
		return
	}

	condition := map[string]string{"broadcaster_user_id": twitchID}
	sub, err := h.EventSub.CreateSubscription(r.Context(), appToken, req.EventType, eventSubVersion, condition, h.CallbackURL, h.Secret)
	if err != nil {
		slog.Error("create subscription error", "error", err)
		respondError(w, http.StatusBadGateway, "failed to create subscription")
		return
	}

	record, err := h.Subs.Create(r.Context(), userID, req.EventType, condition, sub.ID)
	if err != nil {
		slog.Error("persist subscription error", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to save subscription")
		return
	}

	respondJSON(w, http.StatusCreated, record)
}

func (h *NotificationHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	subs, err := h.Subs.ListActiveByUser(r.Context(), userID)
	if err != nil {
		slog.Error("list subscriptions error", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to list subscriptions")
		return
	}
	if subs == nil {
		subs = []store.NotificationSubscription{}
	}
	respondJSON(w, http.StatusOK, subs)
}

func (h *NotificationHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id := r.PathValue("id")
	sub, err := h.Subs.GetByID(r.Context(), id)
	if err != nil {
		slog.Error("get subscription error", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to load subscription")
		return
	}
	if sub == nil || sub.Status != store.SubscriptionEnabled {
		respondError(w, http.StatusNotFound, "subscription not found")
		return
	}
	if sub.UserID != userID {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}

	appToken, err := h.EventSub.GetAppAccessToken(r.Context())
	if err != nil {
		slog.Error("app token error", "error", err)
		respondError(w, http.StatusBadGateway, "failed to reach twitch")
		return
	}

	if err := h.EventSub.DeleteSubscription(r.Context(), appToken, sub.TwitchSubscriptionID); err != nil {
		slog.Error("delete subscription error", "error", err)
		respondError(w, http.StatusBadGateway, "failed to revoke subscription")
		return
	}

	if err := h.Subs.MarkRevoked(r.Context(), sub.ID); err != nil {
		slog.Error("mark revoked error", "error", err)
		respondError(w, http.StatusInternalServerError, "failed to update subscription")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "subscription revoked"})
}
