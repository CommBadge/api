package testutil

import (
	"context"
	"sync"

	"kronus.dev/commbadge_api/discord"
)

type MockDiscordClient struct {
	mu             sync.Mutex
	AuthURLResult  string
	ExchangeResult *discord.TokenResponse
	ExchangeErr    error
	GetUserResult  *discord.User
	GetUserErr     error
}

func NewMockDiscordClient() *MockDiscordClient {
	return &MockDiscordClient{
		AuthURLResult: "https://discord.com/oauth2/authorize?mock=1",
		ExchangeResult: &discord.TokenResponse{
			AccessToken: "mock-access",
		},
		GetUserResult: &discord.User{
			ID:         "user-1",
			Username:   "testuser",
			GlobalName: "TestUser",
			Email:      "test@example.com",
			Avatar:     "avatarhash",
		},
	}
}

func (m *MockDiscordClient) AuthURL(state string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.AuthURLResult + "&state=" + state
}

func (m *MockDiscordClient) Exchange(ctx context.Context, code string) (*discord.TokenResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.ExchangeResult, m.ExchangeErr
}

func (m *MockDiscordClient) GetUser(ctx context.Context, accessToken string) (*discord.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.GetUserResult, m.GetUserErr
}
