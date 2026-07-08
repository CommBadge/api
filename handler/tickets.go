package handler

import (
	"encoding/json"
	"net/http"

	"kronus.dev/commbadge_api/middleware"
)

type TicketHandler struct {
	Tickets TicketRepository
}

func (h *TicketHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var body struct {
		Subject string `json:"subject"`
		Body    string `json:"body"`
	}
	if err := decodeJSON(r, &body); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Subject == "" || body.Body == "" {
		respondError(w, http.StatusBadRequest, "subject and body are required")
		return
	}
	if len(body.Body) > 2000 {
		respondError(w, http.StatusBadRequest, "body must be 2000 characters or less")
		return
	}

	ticket, err := h.Tickets.Create(r.Context(), userID, body.Subject, body.Body)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create ticket")
		return
	}
	respondJSON(w, http.StatusCreated, ticket)
}

func (h *TicketHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	tickets, err := h.Tickets.ListByUser(r.Context(), userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list tickets")
		return
	}
	respondJSON(w, http.StatusOK, tickets)
}

func (h *TicketHandler) Get(w http.ResponseWriter, r *http.Request) {
	ticketID := r.PathValue("ticketID")
	userID := middleware.GetUserID(r.Context())

	ticket, err := h.Tickets.GetByID(r.Context(), ticketID)
	if err != nil {
		respondError(w, http.StatusNotFound, "ticket not found")
		return
	}
	if ticket.UserID != userID {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}

	messages, err := h.Tickets.GetMessages(r.Context(), ticketID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get messages")
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"ticket":   ticket,
		"messages": messages,
	})
}

func (h *TicketHandler) AddMessage(w http.ResponseWriter, r *http.Request) {
	ticketID := r.PathValue("ticketID")
	userID := middleware.GetUserID(r.Context())

	ticket, err := h.Tickets.GetByID(r.Context(), ticketID)
	if err != nil {
		respondError(w, http.StatusNotFound, "ticket not found")
		return
	}
	if ticket.UserID != userID {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}

	var body struct {
		Body string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Body == "" {
		respondError(w, http.StatusBadRequest, "body is required")
		return
	}
	if len(body.Body) > 2000 {
		respondError(w, http.StatusBadRequest, "body must be 2000 characters or less")
		return
	}

	msg, err := h.Tickets.AddMessage(r.Context(), ticketID, userID, body.Body, false)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to add message")
		return
	}
	respondJSON(w, http.StatusCreated, msg)
}
