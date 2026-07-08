package testutil

import (
	"context"
	"sync"
	"time"

	"kronus.dev/commbadge_api/session"
)

type MockSessionStore struct {
	mu       sync.Mutex
	Sessions map[string]*session.Session
	States   map[string]time.Time
	Err      error
}

func NewMockSessionStore() *MockSessionStore {
	return &MockSessionStore{
		Sessions: make(map[string]*session.Session),
		States:   make(map[string]time.Time),
	}
}

func (m *MockSessionStore) SetErr(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Err = err
}

func (m *MockSessionStore) Create(ctx context.Context, s *session.Session) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return "", m.Err
	}
	id := "session-" + s.UserID
	m.Sessions[id] = s
	return id, nil
}

func (m *MockSessionStore) Get(ctx context.Context, id string) (*session.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return nil, m.Err
	}
	s, ok := m.Sessions[id]
	if !ok {
		return nil, nil
	}
	return s, nil
}

func (m *MockSessionStore) Delete(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return m.Err
	}
	delete(m.Sessions, id)
	return nil
}

func (m *MockSessionStore) SetState(ctx context.Context, state string, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return m.Err
	}
	m.States[state] = time.Now().Add(ttl)
	return nil
}

func (m *MockSessionStore) VerifyState(ctx context.Context, state string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return false, m.Err
	}
	expiry, ok := m.States[state]
	if !ok {
		return false, nil
	}
	if time.Now().After(expiry) {
		delete(m.States, state)
		return false, nil
	}
	delete(m.States, state)
	return true, nil
}

func (m *MockSessionStore) Ping(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.Err
}

func (m *MockSessionStore) Close() error { return nil }
