// Package authapi is the dashboard's client of luna-auth's internal API
// (/internal/v1/apps/{app}/…): the users of a developer's application and
// the application's data about them. luna-auth trusts the path's
// application, so callers check that the signed-in developer owns it first.
package authapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"fullstack"
	"fullstack/env"
)

// Config is how the dashboard reaches luna-auth.
type Config struct {
	// URL is luna-auth's internal address, e.g. http://luna-auth:8080.
	// Empty: the users pages say that luna-auth is not connected.
	URL string `env:"LUNA_AUTH_URL"`
	// Token is luna-auth's LUNA_DASHBOARD_TOKEN (one of LUNA_SERVICE_TOKENS).
	Token   string        `env:"LUNA_AUTH_TOKEN"`
	Timeout time.Duration `env:"LUNA_AUTH_TIMEOUT" default:"10s"`
}

// ErrNotConfigured: LUNA_AUTH_URL or LUNA_AUTH_TOKEN is not set.
var ErrNotConfigured = errors.New("luna-auth is not connected (LUNA_AUTH_URL, LUNA_AUTH_TOKEN)")

// ErrNotFound: no such user or key.
var ErrNotFound = errors.New("not found")

// Error is an answer of luna-auth other than success and 404.
type Error struct {
	Status  int
	Message string
	Fields  map[string][]string // 422
}

func (e *Error) Error() string { return fmt.Sprintf("luna-auth: %d %s", e.Status, e.Message) }

// User is an end user of an application.
type User struct {
	ID            string    `json:"id"`
	Username      *string   `json:"username"`
	Email         string    `json:"email"`
	EmailVerified bool      `json:"email_verified"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// Name is the username, or "" without one.
func (u User) Name() string {
	if u.Username == nil {
		return ""
	}
	return *u.Username
}

// Page is a page of users.
type Page struct {
	Data     []User `json:"data"`
	Page     int    `json:"page"`
	PerPage  int    `json:"per_page"`
	Total    int    `json:"total"`
	LastPage int    `json:"last_page"`
}

// NewUser is a user the developer creates.
type NewUser struct {
	Username      string `json:"username,omitempty"`
	Email         string `json:"email"`
	Password      string `json:"password"`
	EmailVerified bool   `json:"email_verified"`
}

// Changes are the fields of a user to change; nil fields stay.
type Changes struct {
	Username      *string `json:"username,omitempty"` // "" removes it
	Email         *string `json:"email,omitempty"`
	EmailVerified *bool   `json:"email_verified,omitempty"`
	Password      *string `json:"password,omitempty"` // ends the user's logins
}

// Client calls luna-auth.
type Client struct {
	cfg  Config
	http *http.Client
}

// New makes a client.
func New(cfg Config) *Client {
	cfg.URL = strings.TrimRight(cfg.URL, "/")
	return &Client{cfg: cfg, http: &http.Client{Timeout: cfg.Timeout}}
}

// Provider puts a *Client into the container.
type Provider struct{}

// Register implements fullstack.Provider.
func (Provider) Register(app *fullstack.App) error {
	var cfg Config
	if err := env.Decode(&cfg); err != nil {
		return fmt.Errorf("authapi: %w", err)
	}
	fullstack.Set(app, New(cfg))
	return nil
}

// Configured reports whether luna-auth can be called.
func (c *Client) Configured() bool { return c.cfg.URL != "" && c.cfg.Token != "" }

func base(app string) string { return "/internal/v1/apps/" + url.PathEscape(app) + "/users" }

// Users lists an application's users, newest first; q searches emails and
// usernames.
func (c *Client) Users(ctx context.Context, app, q string, page, perPage int) (Page, error) {
	v := url.Values{"page": {strconv.Itoa(page)}, "per_page": {strconv.Itoa(perPage)}}
	if q != "" {
		v.Set("q", q)
	}
	var p Page
	err := c.do(ctx, http.MethodGet, base(app)+"?"+v.Encode(), nil, &p)
	return p, err
}

// CreateUser adds a user.
func (c *Client) CreateUser(ctx context.Context, app string, in NewUser) (User, error) {
	var u User
	err := c.do(ctx, http.MethodPost, base(app), in, &u)
	return u, err
}

// User gets one user.
func (c *Client) User(ctx context.Context, app, id string) (User, error) {
	var u User
	err := c.do(ctx, http.MethodGet, base(app)+"/"+url.PathEscape(id), nil, &u)
	return u, err
}

// UpdateUser changes a user.
func (c *Client) UpdateUser(ctx context.Context, app, id string, ch Changes) (User, error) {
	var u User
	err := c.do(ctx, http.MethodPatch, base(app)+"/"+url.PathEscape(id), ch, &u)
	return u, err
}

// DeleteUser deletes a user, their data and their logins.
func (c *Client) DeleteUser(ctx context.Context, app, id string) error {
	return c.do(ctx, http.MethodDelete, base(app)+"/"+url.PathEscape(id), nil, nil)
}

// Data is every value of a user, by key.
func (c *Client) Data(ctx context.Context, app, id string) (map[string]string, error) {
	var out struct {
		Data map[string]string `json:"data"`
	}
	err := c.do(ctx, http.MethodGet, base(app)+"/"+url.PathEscape(id)+"/data", nil, &out)
	return out.Data, err
}

// SetData sets one value.
func (c *Client) SetData(ctx context.Context, app, id, key, value string) error {
	return c.do(ctx, http.MethodPut, base(app)+"/"+url.PathEscape(id)+"/data/"+url.PathEscape(key), map[string]string{"value": value}, nil)
}

// DeleteData deletes one value.
func (c *Client) DeleteData(ctx context.Context, app, id, key string) error {
	return c.do(ctx, http.MethodDelete, base(app)+"/"+url.PathEscape(id)+"/data/"+url.PathEscape(key), nil, nil)
}

func (c *Client) do(ctx context.Context, method, path string, in, out any) error {
	if !c.Configured() {
		return ErrNotConfigured
	}
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.cfg.URL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.Token)
	req.Header.Set("Accept", "application/json")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("luna-auth: %w", err)
	}
	defer res.Body.Close()
	b, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return fmt.Errorf("luna-auth: %w", err)
	}
	switch {
	case res.StatusCode == http.StatusNotFound:
		return ErrNotFound
	case res.StatusCode >= 300:
		var e struct {
			Error  string              `json:"error"`
			Errors map[string][]string `json:"errors"`
		}
		_ = json.Unmarshal(b, &e)
		if e.Error == "" {
			e.Error = http.StatusText(res.StatusCode)
		}
		return &Error{Status: res.StatusCode, Message: e.Error, Fields: e.Errors}
	}
	if out != nil && len(b) > 0 {
		if err := json.Unmarshal(b, out); err != nil {
			return fmt.Errorf("luna-auth: %w", err)
		}
	}
	return nil
}
