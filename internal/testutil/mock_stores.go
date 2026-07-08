package testutil

import (
	"context"
	"fmt"
	"sync"
	"time"

	"kronus.dev/commbadge_api/store"
)

type MockUserStore struct {
	mu           sync.Mutex
	Users        map[string]*store.User
	AdminViews   map[string]*store.UserAdminView
	Warnings     map[string][]store.UserWarning
	Err          error
}

func NewMockUserStore() *MockUserStore {
	return &MockUserStore{
		Users:      make(map[string]*store.User),
		AdminViews: make(map[string]*store.UserAdminView),
		Warnings:   make(map[string][]store.UserWarning),
	}
}

func (m *MockUserStore) UpsertUser(ctx context.Context, user *store.User) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return m.Err
	}
	m.Users[user.ID] = user
	return nil
}

func (m *MockUserStore) GetUserByID(ctx context.Context, id string) (*store.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return nil, m.Err
	}
	return m.Users[id], nil
}

func (m *MockUserStore) IsBanned(ctx context.Context, userID string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return false, m.Err
	}
	v, ok := m.AdminViews[userID]
	if !ok {
		return false, nil
	}
	return v.Banned, nil
}

func (m *MockUserStore) ListUsers(ctx context.Context, limit, offset int) ([]store.UserAdminView, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return nil, m.Err
	}
	var result []store.UserAdminView
	for _, v := range m.AdminViews {
		result = append(result, *v)
	}
	return result, nil
}

func (m *MockUserStore) SearchUsers(ctx context.Context, query string, limit int) ([]store.UserAdminView, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return nil, m.Err
	}
	var result []store.UserAdminView
	for _, v := range m.AdminViews {
		if v.Login == query || v.DisplayName == query {
			result = append(result, *v)
		}
	}
	return result, nil
}

func (m *MockUserStore) GetWarningLog(ctx context.Context, userID string) ([]store.UserWarning, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return nil, m.Err
	}
	return m.Warnings[userID], nil
}

func (m *MockUserStore) AddWarning(ctx context.Context, userID, adminID, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return m.Err
	}
	m.Warnings[userID] = append(m.Warnings[userID], store.UserWarning{
		ID:        "warn-1",
		UserID:    userID,
		AdminID:   adminID,
		Reason:    reason,
		CreatedAt: time.Now(),
	})
	return nil
}

func (m *MockUserStore) BanUser(ctx context.Context, userID, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return m.Err
	}
	now := time.Now()
	v, ok := m.AdminViews[userID]
	if !ok {
		v = &store.UserAdminView{}
		m.AdminViews[userID] = v
	}
	v.Banned = true
	v.BanReason = reason
	v.BannedAt = &now
	return nil
}

func (m *MockUserStore) UnbanUser(ctx context.Context, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return m.Err
	}
	v, ok := m.AdminViews[userID]
	if !ok {
		return nil
	}
	v.Banned = false
	v.BanReason = ""
	v.BannedAt = nil
	return nil
}

type MockCommunityStore struct {
	mu           sync.Mutex
	Communities  map[string]*store.Community
	Members      map[string][]store.CommunityMember
	MemberRoles  map[string]string
	Err          error
}

func NewMockCommunityStore() *MockCommunityStore {
	return &MockCommunityStore{
		Communities: make(map[string]*store.Community),
		Members:     make(map[string][]store.CommunityMember),
		MemberRoles: make(map[string]string),
	}
}

var mockCommunityIdx int

func (m *MockCommunityStore) Create(ctx context.Context, name, description, ownerID string) (*store.Community, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return nil, m.Err
	}
	mockCommunityIdx++
	id := fmt.Sprintf("00000000-0000-4000-8000-%012d", mockCommunityIdx)
	c := &store.Community{
		ID:          id,
		Name:        name,
		Description: description,
		OwnerID:     ownerID,
		Moderators:  []string{},
		LogoURL:     "",
		JoinLinkID:  "joinlink1",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	m.Communities[c.ID] = c
	return c, nil
}

func (m *MockCommunityStore) GetByID(ctx context.Context, id string) (*store.Community, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return nil, m.Err
	}
	return m.Communities[id], nil
}

func (m *MockCommunityStore) ListByUserID(ctx context.Context, userID string) ([]store.CommunityWithRole, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return nil, m.Err
	}
	return nil, nil
}

func (m *MockCommunityStore) Update(ctx context.Context, id, name, description string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return m.Err
	}
	if c, ok := m.Communities[id]; ok {
		c.Name = name
		c.Description = description
	}
	return nil
}

func (m *MockCommunityStore) Delete(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return m.Err
	}
	delete(m.Communities, id)
	return nil
}

func (m *MockCommunityStore) AddMember(ctx context.Context, communityID, userID, role string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return m.Err
	}
	key := communityID + ":" + userID
	m.MemberRoles[key] = role
	return nil
}

func (m *MockCommunityStore) RemoveMember(ctx context.Context, communityID, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return m.Err
	}
	key := communityID + ":" + userID
	delete(m.MemberRoles, key)
	return nil
}

func (m *MockCommunityStore) GetMemberRole(ctx context.Context, communityID, userID string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return "", m.Err
	}
	key := communityID + ":" + userID
	return m.MemberRoles[key], nil
}

func (m *MockCommunityStore) GetMembers(ctx context.Context, communityID string) ([]store.CommunityMember, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return nil, m.Err
	}
	return m.Members[communityID], nil
}

func (m *MockCommunityStore) IsModeratorOrOwner(ctx context.Context, communityID, userID string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return false, m.Err
	}
	c, ok := m.Communities[communityID]
	if !ok {
		return false, nil
	}
	if c.OwnerID == userID {
		return true, nil
	}
	for _, mod := range c.Moderators {
		if mod == userID {
			return true, nil
		}
	}
	return false, nil
}

func (m *MockCommunityStore) IsOwner(ctx context.Context, communityID, userID string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return false, m.Err
	}
	c, ok := m.Communities[communityID]
	if !ok {
		return false, nil
	}
	return c.OwnerID == userID, nil
}

func (m *MockCommunityStore) TransferOwnership(ctx context.Context, communityID, newOwnerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return m.Err
	}
	if c, ok := m.Communities[communityID]; ok {
		c.OwnerID = newOwnerID
	}
	return nil
}

func (m *MockCommunityStore) RegenerateJoinLink(ctx context.Context, id string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return "", m.Err
	}
	return "newjoinlink", nil
}

func (m *MockCommunityStore) UpdateLogoURL(ctx context.Context, id, url string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return m.Err
	}
	if c, ok := m.Communities[id]; ok {
		c.LogoURL = url
	}
	return nil
}

func (m *MockCommunityStore) AdminListAll(ctx context.Context, limit, offset int) ([]store.Community, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return nil, m.Err
	}
	var result []store.Community
	for _, c := range m.Communities {
		result = append(result, *c)
	}
	return result, nil
}

type MockTicketStore struct {
	mu       sync.Mutex
	Tickets  map[string]*store.SupportTicket
	Messages map[string][]store.TicketMessage
	Err      error
}

func NewMockTicketStore() *MockTicketStore {
	return &MockTicketStore{
		Tickets:  make(map[string]*store.SupportTicket),
		Messages: make(map[string][]store.TicketMessage),
	}
}

var mockTicketIdx int

func (m *MockTicketStore) Create(ctx context.Context, userID, subject, body string) (*store.SupportTicket, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return nil, m.Err
	}
	mockTicketIdx++
	id := fmt.Sprintf("ticket-%d", mockTicketIdx)
	t := &store.SupportTicket{
		ID:        id,
		UserID:    userID,
		Subject:   subject,
		Body:      body,
		Status:    "open",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	m.Tickets[t.ID] = t
	return t, nil
}

func (m *MockTicketStore) ListByUser(ctx context.Context, userID string) ([]store.SupportTicket, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return nil, m.Err
	}
	var result []store.SupportTicket
	for _, t := range m.Tickets {
		if t.UserID == userID {
			result = append(result, *t)
		}
	}
	return result, nil
}

func (m *MockTicketStore) GetByID(ctx context.Context, id string) (*store.SupportTicket, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return nil, m.Err
	}
	return m.Tickets[id], nil
}

func (m *MockTicketStore) AddMessage(ctx context.Context, ticketID, userID, body string, isAdmin bool) (*store.TicketMessage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return nil, m.Err
	}
	msg := &store.TicketMessage{
		ID:        "msg-1",
		TicketID:  ticketID,
		UserID:    userID,
		Body:      body,
		IsAdmin:   isAdmin,
		CreatedAt: time.Now(),
	}
	m.Messages[ticketID] = append(m.Messages[ticketID], *msg)
	return msg, nil
}

func (m *MockTicketStore) GetMessages(ctx context.Context, ticketID string) ([]store.TicketMessage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return nil, m.Err
	}
	return m.Messages[ticketID], nil
}

func (m *MockTicketStore) AdminListAll(ctx context.Context, limit, offset int) ([]store.SupportTicket, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return nil, m.Err
	}
	var result []store.SupportTicket
	for _, t := range m.Tickets {
		result = append(result, *t)
	}
	return result, nil
}

func (m *MockTicketStore) AdminTicketCount(ctx context.Context) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return 0, m.Err
	}
	return len(m.Tickets), nil
}

func (m *MockTicketStore) AdminUpdateStatus(ctx context.Context, id, status string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return m.Err
	}
	if t, ok := m.Tickets[id]; ok {
		t.Status = status
	}
	return nil
}

type MockAdminStore struct {
	mu      sync.Mutex
	Admins  map[string]bool
	Err     error
}

func NewMockAdminStore() *MockAdminStore {
	return &MockAdminStore{Admins: make(map[string]bool)}
}

func (m *MockAdminStore) IsAdmin(ctx context.Context, userID string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return false, m.Err
	}
	return m.Admins[userID], nil
}

func (m *MockAdminStore) SeedAdmins(ctx context.Context, adminIDs []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return m.Err
	}
	for _, id := range adminIDs {
		m.Admins[id] = true
	}
	return nil
}

type MockDBPinger struct {
	Err error
}

func (m *MockDBPinger) Ping(ctx context.Context) error { return m.Err }
