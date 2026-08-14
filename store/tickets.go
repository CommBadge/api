package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SupportTicket struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Subject   string    `json:"subject"`
	Body      string    `json:"body"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type TicketMessage struct {
	ID        string    `json:"id"`
	TicketID  string    `json:"ticket_id"`
	UserID    string    `json:"user_id"`
	Body      string    `json:"body"`
	IsAdmin   bool      `json:"is_admin"`
	CreatedAt time.Time `json:"created_at"`
}

type TicketStore struct {
	pool *pgxpool.Pool
}

func NewTicketStore(pool *pgxpool.Pool) *TicketStore {
	return &TicketStore{pool: pool}
}

func (s *TicketStore) Create(ctx context.Context, userID, subject, body string) (*SupportTicket, error) {
	row := s.pool.QueryRow(ctx, `
		INSERT INTO support_tickets (user_id, subject, body)
		VALUES ($1, $2, $3)
		RETURNING id, user_id, subject, body, status, created_at, updated_at
	`, userID, subject, body)
	t := &SupportTicket{}
	err := row.Scan(&t.ID, &t.UserID, &t.Subject, &t.Body, &t.Status, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (s *TicketStore) ListByUser(ctx context.Context, userID string) ([]SupportTicket, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, subject, body, status, created_at, updated_at
		FROM support_tickets
		WHERE user_id = $1
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tickets []SupportTicket
	for rows.Next() {
		var t SupportTicket
		if err := rows.Scan(&t.ID, &t.UserID, &t.Subject, &t.Body, &t.Status, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		tickets = append(tickets, t)
	}
	return tickets, nil
}

func (s *TicketStore) GetByID(ctx context.Context, id string) (*SupportTicket, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, user_id, subject, body, status, created_at, updated_at
		FROM support_tickets WHERE id = $1
	`, id)
	t := &SupportTicket{}
	err := row.Scan(&t.ID, &t.UserID, &t.Subject, &t.Body, &t.Status, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return t, nil
}

func (s *TicketStore) AddMessage(ctx context.Context, ticketID, userID, body string, isAdmin bool) (*TicketMessage, error) {
	row := s.pool.QueryRow(ctx, `
		INSERT INTO ticket_messages (ticket_id, user_id, body, is_admin)
		VALUES ($1, $2, $3, $4)
		RETURNING id, ticket_id, user_id, body, is_admin, created_at
	`, ticketID, userID, body, isAdmin)
	m := &TicketMessage{}
	err := row.Scan(&m.ID, &m.TicketID, &m.UserID, &m.Body, &m.IsAdmin, &m.CreatedAt)
	if err != nil {
		return nil, err
	}

	s.pool.Exec(ctx, `UPDATE support_tickets SET updated_at = now() WHERE id = $1`, ticketID)
	return m, nil
}

func (s *TicketStore) GetMessages(ctx context.Context, ticketID string) ([]TicketMessage, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, ticket_id, user_id, body, is_admin, created_at
		FROM ticket_messages
		WHERE ticket_id = $1
		ORDER BY created_at
	`, ticketID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []TicketMessage
	for rows.Next() {
		var m TicketMessage
		if err := rows.Scan(&m.ID, &m.TicketID, &m.UserID, &m.Body, &m.IsAdmin, &m.CreatedAt); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, nil
}

func (s *TicketStore) AdminListAll(ctx context.Context, limit, offset int) ([]SupportTicket, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, subject, body, status, created_at, updated_at
		FROM support_tickets
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tickets []SupportTicket
	for rows.Next() {
		var t SupportTicket
		if err := rows.Scan(&t.ID, &t.UserID, &t.Subject, &t.Body, &t.Status, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		tickets = append(tickets, t)
	}
	return tickets, nil
}

func (s *TicketStore) AdminTicketCount(ctx context.Context) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM support_tickets`).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (s *TicketStore) AdminUpdateStatus(ctx context.Context, id, status string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE support_tickets SET status = $1, updated_at = now() WHERE id = $2
	`, status, id)
	return err
}
