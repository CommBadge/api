// Package tickets implements the user-facing support ticket endpoints.
package tickets

import (
	"encoding/json"
	"net/http"

	"kronus.dev/commbadge_api/handler/common"
	"kronus.dev/commbadge_api/handler/contracts"
	"kronus.dev/commbadge_api/middleware"
)

type TicketHandler struct {
	Tickets contracts.TicketRepository
}

func (h *TicketHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var body struct {
		Subject string `json:"subject"`
		Body    string `json:"body"`
	}
	if err := common.DecodeJSON(r, &body); err != nil {
		common.RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Subject == "" || body.Body == "" {
		common.RespondError(w, http.StatusBadRequest, "subject and body are required")
		return
	}
	if len(body.Body) > 2000 {
		common.RespondError(w, http.StatusBadRequest, "body must be 2000 characters or less")
		return
	}

	ticket, err := h.Tickets.Create(r.Context(), userID, body.Subject, body.Body)
	if err != nil {
		common.RespondError(w, http.StatusInternalServerError, "failed to create ticket")
		return
	}
	common.RespondJSON(w, http.StatusCreated, ticket)
}

func (h *TicketHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	tickets, err := h.Tickets.ListByUser(r.Context(), userID)
	if err != nil {
		common.RespondError(w, http.StatusInternalServerError, "failed to list tickets")
		return
	}
	common.RespondJSON(w, http.StatusOK, tickets)
}

func (h *TicketHandler) Get(w http.ResponseWriter, r *http.Request) {
	ticketID := r.PathValue("ticketID")
	userID := middleware.GetUserID(r.Context())

	ticket, err := h.Tickets.GetByID(r.Context(), ticketID)
	if err != nil {
		common.RespondError(w, http.StatusInternalServerError, "failed to get ticket")
		return
	}
	if ticket == nil {
		common.RespondError(w, http.StatusNotFound, "ticket not found")
		return
	}
	if ticket.UserID != userID {
		common.RespondError(w, http.StatusForbidden, "forbidden")
		return
	}

	messages, err := h.Tickets.GetMessages(r.Context(), ticketID)
	if err != nil {
		common.RespondError(w, http.StatusInternalServerError, "failed to get messages")
		return
	}
	common.RespondJSON(w, http.StatusOK, map[string]interface{}{
		"ticket":   ticket,
		"messages": messages,
	})
}

func (h *TicketHandler) AddMessage(w http.ResponseWriter, r *http.Request) {
	ticketID := r.PathValue("ticketID")
	userID := middleware.GetUserID(r.Context())

	ticket, err := h.Tickets.GetByID(r.Context(), ticketID)
	if err != nil {
		common.RespondError(w, http.StatusInternalServerError, "failed to get ticket")
		return
	}
	if ticket == nil {
		common.RespondError(w, http.StatusNotFound, "ticket not found")
		return
	}
	if ticket.UserID != userID {
		common.RespondError(w, http.StatusForbidden, "forbidden")
		return
	}

	var body struct {
		Body string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Body == "" {
		common.RespondError(w, http.StatusBadRequest, "body is required")
		return
	}
	if len(body.Body) > 2000 {
		common.RespondError(w, http.StatusBadRequest, "body must be 2000 characters or less")
		return
	}

	msg, err := h.Tickets.AddMessage(r.Context(), ticketID, userID, body.Body, false)
	if err != nil {
		common.RespondError(w, http.StatusInternalServerError, "failed to add message")
		return
	}
	common.RespondJSON(w, http.StatusCreated, msg)
}