// Package discord implements the Discord OAuth2 authorization-code flow used
// as the primary login provider for CommBadge.
package discord

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Config struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	BaseURL      string
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
}

type User struct {
	ID         string `json:"id"`
	Username   string `json:"username"`
	GlobalName string `json:"global_name"`
	Avatar     string `json:"avatar"`
	Email      string `json:"email"`
}

func (u *User) DisplayName() string {
	if u.GlobalName != "" {
		return u.GlobalName
	}
	return u.Username
}

func (u *User) AvatarURL() string {
	if u.Avatar == "" {
		return ""
	}
	ext := "png"
	if strings.HasPrefix(u.Avatar, "a_") {
		ext = "gif"
	}
	return fmt.Sprintf("https://cdn.discordapp.com/avatars/%s/%s.%s?size=256", u.ID, u.Avatar, ext)
}

// AdministratorPermission is the Discord permission bit that grants all
// permissions; it is accepted wherever a specific permission is required.
const AdministratorPermission int64 = 1 << 3

// ManageChannelsPermission is the Discord permission bit for "Manage Channels",
// required before a user may configure a channel as a notification target.
const ManageChannelsPermission int64 = 1 << 4

type Guild struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Permissions string `json:"permissions"`
}

type Channel struct {
	ID      string `json:"id"`
	GuildID string `json:"guild_id"`
	Name    string `json:"name"`
	Type    int    `json:"type"`
}

type Client struct {
	cfg     *Config
	httpCli *http.Client
}

func NewClient(cfg *Config) *Client {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://discord.com"
	}
	return &Client{cfg: cfg, httpCli: &http.Client{Timeout: 10 * time.Second}}
}

func (c *Client) AuthURL(state, codeChallenge string) string {
	v := url.Values{}
	v.Set("client_id", c.cfg.ClientID)
	v.Set("redirect_uri", c.cfg.RedirectURL)
	v.Set("response_type", "code")
	// The "guilds" scope is required so the API can later verify, via
	// GetUserGuilds, that the user manages the guild hosting a target channel.
	v.Set("scope", "identify email guilds")
	v.Set("state", state)
	// PKCE (RFC 7636): the S256 challenge is bound to the authorization
	// request; the verifier is sent with the token exchange.
	v.Set("code_challenge", codeChallenge)
	v.Set("code_challenge_method", "S256")
	return c.cfg.BaseURL + "/oauth2/authorize?" + v.Encode()
}

func (c *Client) Exchange(ctx context.Context, code, verifier string) (*TokenResponse, error) {
	v := url.Values{}
	v.Set("client_id", c.cfg.ClientID)
	v.Set("client_secret", c.cfg.ClientSecret)
	v.Set("grant_type", "authorization_code")
	v.Set("code", code)
	v.Set("code_verifier", verifier)
	v.Set("redirect_uri", c.cfg.RedirectURL)

	req, err := http.NewRequestWithContext(ctx, "POST", c.cfg.BaseURL+"/api/oauth2/token", strings.NewReader(v.Encode()))
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

func (c *Client) GetUser(ctx context.Context, accessToken string) (*User, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.cfg.BaseURL+"/api/users/@me", nil)
	if err != nil {
		return nil, fmt.Errorf("NewRequest: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

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
		return nil, fmt.Errorf("get user failed: %s", string(body))
	}

	var u User
	if err := json.Unmarshal(body, &u); err != nil {
		return nil, fmt.Errorf("json.Unmarshal: %w", err)
	}
	return &u, nil
}

// GetUserGuilds returns the guilds the user belongs to. The partial guild
// objects include the user's aggregate permissions bitmask in each guild, which
// is used to verify Manage Channels permission before saving a target channel.
func (c *Client) GetUserGuilds(ctx context.Context, accessToken string) ([]Guild, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.cfg.BaseURL+"/api/users/@me/guilds", nil)
	if err != nil {
		return nil, fmt.Errorf("NewRequest: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

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
		return nil, fmt.Errorf("get user guilds failed: %s", string(body))
	}

	var guilds []Guild
	if err := json.Unmarshal(body, &guilds); err != nil {
		return nil, fmt.Errorf("json.Unmarshal: %w", err)
	}
	return guilds, nil
}

// GetChannel fetches a channel using the bot token. This fails unless the bot
// is installed in the guild owning the channel, which proves guild
// installation.
func (c *Client) GetChannel(ctx context.Context, botToken, channelID string) (*Channel, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.cfg.BaseURL+"/api/channels/"+channelID, nil)
	if err != nil {
		return nil, fmt.Errorf("NewRequest: %w", err)
	}
	req.Header.Set("Authorization", "Bot "+botToken)

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
		return nil, fmt.Errorf("get channel failed: %s", string(body))
	}

	var ch Channel
	if err := json.Unmarshal(body, &ch); err != nil {
		return nil, fmt.Errorf("json.Unmarshal: %w", err)
	}
	return &ch, nil
}
