package twitch

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

// EventSub webhook subscriptions are created and deleted with an app access
// token (client credentials), never a user access token. The user access
// token is only used during the one-time Twitch link to establish the grant
// and capture the broadcaster id.

type EventSubTransport struct {
	Method   string `json:"method"`
	Callback string `json:"callback"`
	Secret   string `json:"secret"`
}

type EventSubSubscription struct {
	ID        string            `json:"id"`
	Status    string            `json:"status"`
	Type      string            `json:"type"`
	Version   string            `json:"version"`
	Condition map[string]string `json:"condition"`
	CreatedAt string            `json:"created_at"`
	Cost      int               `json:"cost"`
}

type eventsubCreateRequest struct {
	Type      string            `json:"type"`
	Version   string            `json:"version"`
	Condition map[string]string `json:"condition"`
	Transport EventSubTransport `json:"transport"`
}

type eventsubResponse struct {
	Data  []EventSubSubscription `json:"data"`
	Total int                    `json:"total"`
}

func (c *Client) GetAppAccessToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.appToken != "" && time.Now().Before(c.appTokenExpires) {
		return c.appToken, nil
	}

	v := url.Values{}
	v.Set("client_id", c.cfg.ClientID)
	v.Set("client_secret", c.cfg.ClientSecret)
	v.Set("grant_type", "client_credentials")

	req, err := http.NewRequestWithContext(ctx, "POST", c.cfg.OAuthURL+"/oauth2/token", strings.NewReader(v.Encode()))
	if err != nil {
		return "", fmt.Errorf("NewRequest: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpCli.Do(req)
	if err != nil {
		return "", fmt.Errorf("Do: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("ReadAll: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("app token request failed: %s", string(body))
	}

	var tr TokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return "", fmt.Errorf("json.Unmarshal: %w", err)
	}
	if tr.AccessToken == "" {
		return "", fmt.Errorf("empty access token in response")
	}

	c.appToken = tr.AccessToken
	expiresIn := tr.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 3600
	}
	// Request a new token shortly before expiry to avoid mid-flight expiration.
	c.appTokenExpires = time.Now().Add(time.Duration(expiresIn)*time.Second - 5*time.Minute)
	return c.appToken, nil
}

func (c *Client) CreateSubscription(ctx context.Context, appToken, eventType, version string, condition map[string]string, callback, secret string) (*EventSubSubscription, error) {
	payload := eventsubCreateRequest{
		Type:      eventType,
		Version:   version,
		Condition: condition,
		Transport: EventSubTransport{
			Method:   "webhook",
			Callback: callback,
			Secret:   secret,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("json.Marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.cfg.HelixURL+"/helix/eventsub/subscriptions", strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("NewRequest: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+appToken)
	req.Header.Set("Client-Id", c.cfg.ClientID)

	resp, err := c.httpCli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Do: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ReadAll: %w", err)
	}
	if resp.StatusCode != http.StatusAccepted {
		return nil, fmt.Errorf("create subscription failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	var er eventsubResponse
	if err := json.Unmarshal(respBody, &er); err != nil {
		return nil, fmt.Errorf("json.Unmarshal: %w", err)
	}
	if len(er.Data) == 0 {
		return nil, fmt.Errorf("no subscription data returned")
	}
	return &er.Data[0], nil
}

func (c *Client) DeleteSubscription(ctx context.Context, appToken, id string) error {
	req, err := http.NewRequestWithContext(ctx, "DELETE", c.cfg.HelixURL+"/helix/eventsub/subscriptions?id="+url.QueryEscape(id), nil)
	if err != nil {
		return fmt.Errorf("NewRequest: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+appToken)
	req.Header.Set("Client-Id", c.cfg.ClientID)

	resp, err := c.httpCli.Do(req)
	if err != nil {
		return fmt.Errorf("Do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete subscription failed (status %d): %s", resp.StatusCode, string(body))
	}
	return nil
}
