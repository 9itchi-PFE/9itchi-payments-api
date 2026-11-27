// Package account is the HTTP client for 9itchi-account-api. Base URL comes from env, never hard-coded.
package account

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	BaseURL string
	APIKey  string
	HTTP    *http.Client
}

func New(baseURL, apiKey string) *Client {
	return &Client{
		BaseURL: baseURL,
		APIKey:  apiKey,
		HTTP:    &http.Client{Timeout: 50 * time.Second},
	}
}

// User is what payments-api reads about an account.
type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	// EmailVerified says whether a Google or LinkedIn sign-in vouched for the email.
	EmailVerified bool `json:"email_verified"`
}

// User calls GET /internal/users/:id.
func (c *Client) User(ctx context.Context, id string) (*User, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(c.BaseURL, "/")+"/internal/users/"+url.PathEscape(id), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", c.APIKey)
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("account-api: GET user: status %d", res.StatusCode)
	}
	var u User
	if err := json.NewDecoder(res.Body).Decode(&u); err != nil {
		return nil, err
	}
	return &u, nil
}
