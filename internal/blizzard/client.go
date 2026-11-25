package blizzard

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/StormChild/Wow-Token-Tracker/internal/tracker"
)

// ErrBadCredentials means the OAuth endpoint refused the client ID or secret.
var ErrBadCredentials = errors.New("Blizzard rejected the client ID or secret")

// errUnauthorized marks a 401 from the price endpoint (expired or revoked token).
var errUnauthorized = errors.New("unauthorized")

// Client fetches the WoW Token price. Safe for concurrent use.
type Client struct {
	HTTP         *http.Client
	OAuthURL     string
	APIBaseFmt   string // fmt pattern taking the region, e.g. "https://%s.api.blizzard.com"
	ClientID     string
	ClientSecret string

	mu      sync.Mutex
	token   string
	expires time.Time
}

func New(clientID, clientSecret string) *Client {
	return &Client{
		HTTP:         &http.Client{Timeout: 15 * time.Second},
		OAuthURL:     "https://oauth.battle.net/token",
		APIBaseFmt:   "https://%s.api.blizzard.com",
		ClientID:     clientID,
		ClientSecret: clientSecret,
	}
}

// TokenPrice returns the current token price for region (us, eu, kr, tw).
func (c *Client) TokenPrice(ctx context.Context, region string) (tracker.Observation, error) {
	obs, err := c.fetchPrice(ctx, region, false)
	if errors.Is(err, errUnauthorized) {
		obs, err = c.fetchPrice(ctx, region, true)
	}
	if errors.Is(err, errUnauthorized) {
		return obs, ErrBadCredentials
	}
	return obs, err
}

func (c *Client) fetchPrice(ctx context.Context, region string, forceRefresh bool) (tracker.Observation, error) {
	tok, err := c.accessToken(ctx, forceRefresh)
	if err != nil {
		return tracker.Observation{}, err
	}
	q := url.Values{"namespace": {"dynamic-" + region}, "locale": {"en_US"}}
	u := fmt.Sprintf(c.APIBaseFmt, region) + "/data/wow/token/index?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return tracker.Observation{}, err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return tracker.Observation{}, fmt.Errorf("can't reach Blizzard: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return tracker.Observation{}, errUnauthorized
	}
	if resp.StatusCode != http.StatusOK {
		return tracker.Observation{}, fmt.Errorf("Blizzard price API returned %d", resp.StatusCode)
	}
	var body struct {
		Price       int64 `json:"price"`
		LastUpdated int64 `json:"last_updated_timestamp"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&body); err != nil {
		return tracker.Observation{}, fmt.Errorf("unexpected Blizzard response: %w", err)
	}
	if body.Price <= 0 {
		return tracker.Observation{}, errors.New("Blizzard response had no price")
	}
	return tracker.Observation{PriceCopper: body.Price, UpdatedAt: time.UnixMilli(body.LastUpdated)}, nil
}

func (c *Client) accessToken(ctx context.Context, forceRefresh bool) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !forceRefresh && c.token != "" && time.Until(c.expires) > time.Minute {
		return c.token, nil
	}
	form := url.Values{"grant_type": {"client_credentials"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.OAuthURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(c.ClientID, c.ClientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", fmt.Errorf("can't reach Blizzard: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return "", ErrBadCredentials
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Blizzard login returned %d", resp.StatusCode)
	}
	var body struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&body); err != nil || body.AccessToken == "" {
		return "", errors.New("unexpected Blizzard login response")
	}
	c.token = body.AccessToken
	c.expires = time.Now().Add(time.Duration(body.ExpiresIn) * time.Second)
	return c.token, nil
}
