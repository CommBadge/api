package twitch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type Config struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	OAuthURL     string
	HelixURL     string
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	IDToken      string `json:"id_token,omitempty"`
}

type TwitchUser struct {
	ID          string `json:"id"`
	Login       string `json:"login"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	AvatarURL   string `json:"profile_image_url"`
}

type helixResponse struct {
	Data []TwitchUser `json:"data"`
}

type Client struct {
	cfg     *Config
	httpCli *http.Client

	mu              sync.Mutex
	appToken        string
	appTokenExpires time.Time
}

func NewClient(cfg *Config) *Client {
	if cfg.OAuthURL == "" {
		cfg.OAuthURL = "https://id.twitch.tv"
	}
	if cfg.HelixURL == "" {
		cfg.HelixURL = "https://api.twitch.tv"
	}
	return &Client{cfg: cfg, httpCli: &http.Client{Timeout: 10 * time.Second}}
}

func (c *Client) AuthURL(state string) string {
	v := url.Values{}
	v.Set("client_id", c.cfg.ClientID)
	v.Set("redirect_uri", c.cfg.RedirectURL)
	v.Set("response_type", "code")
	v.Set("scope", "openid user:read:email")
	v.Set("state", state)
	v.Set("nonce", state)
	return c.cfg.OAuthURL + "/oauth2/authorize?" + v.Encode()
}

func (c *Client) Exchange(ctx context.Context, code string) (*TokenResponse, error) {
	v := url.Values{}
	v.Set("client_id", c.cfg.ClientID)
	v.Set("client_secret", c.cfg.ClientSecret)
	v.Set("code", code)
	v.Set("grant_type", "authorization_code")
	v.Set("redirect_uri", c.cfg.RedirectURL)

	req, err := http.NewRequestWithContext(ctx, "POST", c.cfg.OAuthURL+"/oauth2/token", strings.NewReader(v.Encode()))
	if err != nil {
		return nil, fmt.Errorf("NewRequest: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpCli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Do: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ReadAll: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange failed: %s", string(body))
	}

	var tr TokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return nil, fmt.Errorf("json.Unmarshal: %w", err)
	}
	return &tr, nil
}

func (c *Client) GetUser(ctx context.Context, accessToken string) (*TwitchUser, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.cfg.HelixURL+"/helix/users", nil)
	if err != nil {
		return nil, fmt.Errorf("NewRequest: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Client-ID", c.cfg.ClientID)

	resp, err := c.httpCli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Do: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ReadAll: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("helix users failed: %s", string(body))
	}

	var hr helixResponse
	if err := json.Unmarshal(body, &hr); err != nil {
		return nil, fmt.Errorf("json.Unmarshal: %w", err)
	}
	if len(hr.Data) == 0 {
		return nil, fmt.Errorf("no user data returned")
	}
	return &hr.Data[0], nil
}
