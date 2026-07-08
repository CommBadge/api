package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"kronus.dev/commbadge_api/middleware"
)

type AdminRepository interface {
	IsAdmin(ctx context.Context, userID string) (bool, error)
}

type AdminHandler struct {
	AdminStore       AdminRepository
	CommunityStore   CommunityRepository
	TicketStore      TicketRepository
	UserStore        UserRepository
	DegradedDetector DegradedHalter
}

type DegradedHalter interface {
	Halt()
}

func paginate(r *http.Request) (limit, offset int) {
	limit = 50
	offset = 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	return
}

func (h *AdminHandler) ListCommunities(w http.ResponseWriter, r *http.Request) {
	limit, offset := paginate(r)
	communities, err := h.CommunityStore.AdminListAll(r.Context(), limit, offset)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list communities")
		return
	}
	respondJSON(w, http.StatusOK, communities)
}

func (h *AdminHandler) DisbandCommunity(w http.ResponseWriter, r *http.Request) {
	communityID, ok := validateCommunityID(w, r)
	if !ok {
		return
	}
	if err := h.CommunityStore.Delete(r.Context(), communityID); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to disband community")
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"message": "disbanded"})
}

func (h *AdminHandler) SetMaintenance(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := decodeJSON(r, &body); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if body.Enabled {
		if h.DegradedDetector != nil {
			h.DegradedDetector.Halt()
		}
	}
	respondJSON(w, http.StatusOK, map[string]bool{"maintenance": body.Enabled})
}

func (h *AdminHandler) ListTickets(w http.ResponseWriter, r *http.Request) {
	limit, offset := paginate(r)
	tickets, err := h.TicketStore.AdminListAll(r.Context(), limit, offset)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list tickets")
		return
	}
	respondJSON(w, http.StatusOK, tickets)
}

func (h *AdminHandler) GetTicket(w http.ResponseWriter, r *http.Request) {
	ticketID := r.PathValue("ticketID")
	ticket, err := h.TicketStore.GetByID(r.Context(), ticketID)
	if err != nil {
		respondError(w, http.StatusNotFound, "ticket not found")
		return
	}
	messages, err := h.TicketStore.GetMessages(r.Context(), ticketID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get messages")
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"ticket":   ticket,
		"messages": messages,
	})
}

func (h *AdminHandler) ReplyTicket(w http.ResponseWriter, r *http.Request) {
	ticketID := r.PathValue("ticketID")
	userID := middleware.GetUserID(r.Context())

	var body struct {
		Body string `json:"body"`
	}
	if err := decodeJSON(r, &body); err != nil || body.Body == "" {
		respondError(w, http.StatusBadRequest, "body is required")
		return
	}

	msg, err := h.TicketStore.AddMessage(r.Context(), ticketID, userID, body.Body, true)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to add message")
		return
	}
	respondJSON(w, http.StatusCreated, msg)
}

func (h *AdminHandler) TicketCount(w http.ResponseWriter, r *http.Request) {
	count, err := h.TicketStore.AdminTicketCount(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to count tickets")
		return
	}
	respondJSON(w, http.StatusOK, map[string]int{"count": count})
}

func (h *AdminHandler) UpdateTicketStatus(w http.ResponseWriter, r *http.Request) {
	ticketID := r.PathValue("ticketID")

	var body struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Status == "" {
		respondError(w, http.StatusBadRequest, "status is required")
		return
	}

	if err := h.TicketStore.AdminUpdateStatus(r.Context(), ticketID, body.Status); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to update status")
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"message": "status updated"})
}

// ── User management ──────────────────────────────────

func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	limit, offset := paginate(r)
	users, err := h.UserStore.ListUsers(r.Context(), limit, offset)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list users")
		return
	}
	respondJSON(w, http.StatusOK, users)
}

func (h *AdminHandler) SearchUsers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		respondError(w, http.StatusBadRequest, "query parameter q is required")
		return
	}

	users, err := h.UserStore.SearchUsers(r.Context(), q, 50)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to search users")
		return
	}
	respondJSON(w, http.StatusOK, users)
}

func (h *AdminHandler) GetUserWarnings(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("userID")
	warnings, err := h.UserStore.GetWarningLog(r.Context(), userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get warnings")
		return
	}
	respondJSON(w, http.StatusOK, warnings)
}

func (h *AdminHandler) WarnUser(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("userID")
	adminID := middleware.GetUserID(r.Context())

	var body struct {
		Reason string `json:"reason"`
	}
	if err := decodeJSON(r, &body); err != nil || body.Reason == "" {
		respondError(w, http.StatusBadRequest, "reason is required")
		return
	}

	if err := h.UserStore.AddWarning(r.Context(), userID, adminID, body.Reason); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to add warning")
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"message": "warning added"})
}

func (h *AdminHandler) BanUser(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("userID")

	var body struct {
		Reason string `json:"reason"`
	}
	if err := decodeJSON(r, &body); err != nil || body.Reason == "" {
		respondError(w, http.StatusBadRequest, "reason is required")
		return
	}

	if err := h.UserStore.BanUser(r.Context(), userID, body.Reason); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to ban user")
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"message": "user banned"})
}

func (h *AdminHandler) UnbanUser(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("userID")

	if err := h.UserStore.UnbanUser(r.Context(), userID); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to unban user")
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"message": "user unbanned"})
}
