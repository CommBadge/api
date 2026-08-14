package testutil

import (
	"context"
	"sync"

	"kronus.dev/commbadge_api/twitch"
)

type MockTwitchClient struct {
	mu                 sync.Mutex
	AuthURLResult      string
	ExchangeResult     *twitch.TokenResponse
	ExchangeErr        error
	GetUserResult      *twitch.TwitchUser
	GetUserErr         error
	AppToken           string
	AppTokenErr        error
	CreateSubResult    *twitch.EventSubSubscription
	CreateSubErr       error
	DeleteSubErr       error
	CreateSubCallCount int
	DeleteSubCallCount int
}

func NewMockTwitchClient() *MockTwitchClient {
	return &MockTwitchClient{
		AuthURLResult: "https://id.twitch.tv/oauth2/authorize?mock=1",
		ExchangeResult: &twitch.TokenResponse{
			AccessToken: "mock-access",
		},
		GetUserResult: &twitch.TwitchUser{
			ID:          "twitch-user-1",
			Login:       "teststreamer",
			DisplayName: "TestStreamer",
			Email:       "streamer@example.com",
			AvatarURL:   "https://example.com/avatar.png",
		},
		AppToken: "mock-app-token",
		CreateSubResult: &twitch.EventSubSubscription{
			ID:        "eventsub-1",
			Status:    "webhook_callback_verification_pending",
			Type:      "stream.online",
			Version:   "1",
			Condition: map[string]string{"broadcaster_user_id": "twitch-user-1"},
		},
	}
}

func (m *MockTwitchClient) AuthURL(state string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
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

func (m *MockTwitchClient) GetAppAccessToken(ctx context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.AppToken, m.AppTokenErr
}

func (m *MockTwitchClient) CreateSubscription(ctx context.Context, appToken, eventType, version string, condition map[string]string, callback, secret string) (*twitch.EventSubSubscription, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CreateSubCallCount++
	if m.CreateSubErr != nil {
		return nil, m.CreateSubErr
	}
	return m.CreateSubResult, nil
}

func (m *MockTwitchClient) DeleteSubscription(ctx context.Context, appToken, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.DeleteSubCallCount++
	return m.DeleteSubErr
}
