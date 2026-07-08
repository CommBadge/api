package testutil

import (
	"context"
	"sync"

	"kronus.dev/commbadge_api/twitch"
)

type MockTwitchClient struct {
	mu             sync.Mutex
	AuthURLResult  string
	ExchangeResult *twitch.TokenResponse
	ExchangeErr    error
	GetUserResult  *twitch.TwitchUser
	GetUserErr     error
	RevokeErr      error
}

func NewMockTwitchClient() *MockTwitchClient {
	return &MockTwitchClient{
		AuthURLResult: "https://id.twitch.tv/oauth2/authorize?mock=1",
		ExchangeResult: &twitch.TokenResponse{
			AccessToken:  "mock-access",
			RefreshToken: "mock-refresh",
		},
		GetUserResult: &twitch.TwitchUser{
			ID:          "user-1",
			Login:       "testuser",
			DisplayName: "TestUser",
			Email:       "test@example.com",
			AvatarURL:   "https://example.com/avatar.png",
		},
	}
}

func (m *MockTwitchClient) AuthURL(state string) string {
	return m.AuthURLResult + "&state=" + state
}

func (m *MockTwitchClient) Exchange(ctx context.Context, code string) (*twitch.TokenResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.ExchangeResult, m.ExchangeErr
}

func (m *MockTwitchClient) GetUser(ctx context.Context, accessToken string) (*twitch.TwitchUser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.GetUserResult, m.GetUserErr
}

func (m *MockTwitchClient) RevokeToken(ctx context.Context, accessToken string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.RevokeErr
}
