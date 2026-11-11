// Package pagesapi is the dashboard's client of luna-pages' internal API
// (/internal/v1/apps/{app}/…): it forwards the page editor's requests and
// reads and writes entries for the quick forms. luna-pages trusts the
// path's application, so callers check that the signed-in developer owns it
// first.
package pagesapi

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

// Config is how the dashboard reaches luna-pages.
type Config struct {
	// URL is luna-pages' internal address, e.g. http://luna-pages:8080.
	// Empty: the pages screens say that luna-pages is not connected.
	URL string `env:"LUNA_PAGES_URL"`
	// Token is luna-pages' LUNA_DASHBOARD_TOKEN (one of LUNA_SERVICE_TOKENS).
	Token   string        `env:"LUNA_PAGES_TOKEN"`
	Timeout time.Duration `env:"LUNA_PAGES_TIMEOUT" default:"30s"`
}

// MaxBody is the largest editor request forwarded (a project with its HTML);
// luna-pages has the same limit.
const MaxBody = 4 << 20

// ErrNotConfigured: LUNA_PAGES_URL or LUNA_PAGES_TOKEN is not set.
var ErrNotConfigured = errors.New("luna-pages is not connected (LUNA_PAGES_URL, LUNA_PAGES_TOKEN)")

// ErrNotFound: no such record.
var ErrNotFound = errors.New("not found")

// ErrVersion: the record changed since it was loaded (412).
var ErrVersion = errors.New("changed since it was loaded")

// Error is an answer of luna-pages other than success, 404 and 412.
type Error struct {
	Status  int
	Message string
	Fields  map[string][]string // 422
}

func (e *Error) Error() string { return fmt.Sprintf("luna-pages: %d %s", e.Status, e.Message) }

// Field is a template's field (grapesjs-storage's model).
type Field struct {
	Name     string          `json:"name"`
	Type     string          `json:"type"`
	Label    string          `json:"label,omitempty"`
	Required bool            `json:"required,omitempty"`
	Default  json.RawMessage `json:"default,omitempty"`
	Options  []string        `json:"options,omitempty"`
	Format   string          `json:"format,omitempty"`
	Fields   []Field         `json:"fields,omitempty"`
}

// Title is the field's label, or its name.
func (f Field) Title() string {
	if f.Label != "" {
		return f.Label
	}
	return f.Name
}

// Template is a template of the content API.
type Template struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Fields    []Field   `json:"fields"`
	Version   int64     `json:"version"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Entry is an entry of the content API: values by plain field name.
type Entry struct {
	ID         string                     `json:"id"`
	TemplateID string                     `json:"template_id"`
	Name       string                     `json:"name"`
	Slug       string                     `json:"slug"`
	Status     string                     `json:"status"`
	Values     map[string]json.RawMessage `json:"values,omitempty"`
	Version    int64                      `json:"version"`
	UpdatedAt  time.Time                  `json:"updated_at"`
}

// EntryWrite creates or replaces an entry; values merge into the draft
// (JSON null removes one).
type EntryWrite struct {
	TemplateID string                     `json:"template_id,omitempty"`
	Name       string                     `json:"name"`
	Slug       *string                    `json:"slug,omitempty"`
	Values     map[string]json.RawMessage `json:"values"`
}

// Rendered is a page from /render.
type Rendered struct {
	ID    string `json:"id"`
	Slug  string `json:"slug"`
	Title string `json:"title"`
	HTML  string `json:"html"`
	CSS   string `json:"css"`
	Draft bool   `json:"draft"`
}

// Client calls luna-pages.
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
		return fmt.Errorf("pagesapi: %w", err)
	}
	fullstack.Set(app, New(cfg))
	return nil
}

// Configured reports whether luna-pages can be called.
func (c *Client) Configured() bool { return c.cfg.URL != "" && c.cfg.Token != "" }

func base(app string) string { return "/internal/v1/apps/" + url.PathEscape(app) }

/* ---------------- the editor's requests ---------------- */

// Allowed reports whether the editor may reach a path of the internal API
// through the dashboard: the editor contract, the content API and the read
// API (for previews). Anything else luna-pages may grow stays closed until
// the dashboard is taught about it.
func Allowed(method, path string) bool {
	seg := strings.Split(path, "/")
	for _, s := range seg {
		if s == "" || s == "." || s == ".." {
			return false
		}
	}
	switch seg[0] {
	case "editor", "content":
		return len(seg) >= 2
	case "render", "site", "pages":
		return method == http.MethodGet && len(seg) <= 2
	}
	return false
}

// request headers forwarded to luna-pages, and response headers back.
var (
	forwardIn  = []string{"Accept", "Content-Type", "If-Match", "If-None-Match"}
	forwardOut = []string{"Content-Type", "ETag", "Cache-Control", "Location"}
)

// Forward sends one editor request to luna-pages, as the internal API of
// the application app, and copies the answer back. path is relative to the
// application (editor/pages/…, content/entries, render). The developer's
// cookies and CSRF token stay here.
func (c *Client) Forward(w http.ResponseWriter, r *http.Request, app, path string) error {
	if !c.Configured() {
		return ErrNotConfigured
	}
	target := c.cfg.URL + base(app) + "/" + path
	if r.URL.RawQuery != "" {
		target += "?" + r.URL.RawQuery
	}
	var body io.Reader
	if r.Body != nil && r.Method != http.MethodGet && r.Method != http.MethodHead {
		b, err := io.ReadAll(http.MaxBytesReader(w, r.Body, MaxBody))
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(r.Context(), r.Method, target, body)
	if err != nil {
		return err
	}
	for _, h := range forwardIn {
		if v := r.Header.Get(h); v != "" {
			req.Header.Set(h, v)
		}
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.Token)
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	for _, h := range forwardOut {
		if v := res.Header.Get(h); v != "" {
			w.Header().Set(h, v)
		}
	}
	if w.Header().Get("Cache-Control") == "" {
		w.Header().Set("Cache-Control", "no-store")
	}
	w.WriteHeader(res.StatusCode)
	_, err = io.Copy(w, res.Body)
	return err
}

/* ---------------- entries, for the quick forms ---------------- */

// Templates lists the templates with their fields.
func (c *Client) Templates(ctx context.Context, app string) ([]Template, error) {
	var out struct {
		Items []Template `json:"items"`
	}
	_, err := c.do(ctx, http.MethodGet, base(app)+"/content/templates", nil, 0, &out)
	return out.Items, err
}

// Template gets one template.
func (c *Client) Template(ctx context.Context, app, id string) (Template, error) {
	var t Template
	_, err := c.do(ctx, http.MethodGet, base(app)+"/content/templates/"+url.PathEscape(id), nil, 0, &t)
	return t, err
}

// Entries lists the entries (of one template, when given), without values.
func (c *Client) Entries(ctx context.Context, app, template string) ([]Entry, error) {
	q := ""
	if template != "" {
		q = "?" + url.Values{"template": {template}}.Encode()
	}
	var out struct {
		Items []Entry `json:"items"`
	}
	_, err := c.do(ctx, http.MethodGet, base(app)+"/content/entries"+q, nil, 0, &out)
	return out.Items, err
}

// Entry gets an entry with its draft values.
func (c *Client) Entry(ctx context.Context, app, id string) (Entry, error) {
	var e Entry
	_, err := c.do(ctx, http.MethodGet, base(app)+"/content/entries/"+url.PathEscape(id), nil, 0, &e)
	return e, err
}

// PutEntry creates (version 0) or replaces (If-Match: version) an entry.
func (c *Client) PutEntry(ctx context.Context, app, id string, in EntryWrite, version int64) (Entry, error) {
	var e Entry
	_, err := c.do(ctx, http.MethodPut, base(app)+"/content/entries/"+url.PathEscape(id), in, version, &e)
	return e, err
}

// PublishEntry publishes an entry's draft values.
func (c *Client) PublishEntry(ctx context.Context, app, id string, version int64) (Entry, error) {
	var e Entry
	_, err := c.do(ctx, http.MethodPost, base(app)+"/content/entries/"+url.PathEscape(id)+"/publish", nil, version, &e)
	return e, err
}

// DeleteEntry deletes an entry.
func (c *Client) DeleteEntry(ctx context.Context, app, id string, version int64) error {
	_, err := c.do(ctx, http.MethodDelete, base(app)+"/content/entries/"+url.PathEscape(id), nil, version, nil)
	return err
}

// Render renders a page or an entry by id; draft fills an entry with its
// draft values (the designs are always the published ones).
func (c *Client) Render(ctx context.Context, app, id string, draft bool) (Rendered, error) {
	v := url.Values{"id": {id}}
	if draft {
		v.Set("draft", "1")
	}
	var r Rendered
	_, err := c.do(ctx, http.MethodGet, base(app)+"/render?"+v.Encode(), nil, 0, &r)
	return r, err
}

func (c *Client) do(ctx context.Context, method, path string, in any, version int64, out any) (int, error) {
	if !c.Configured() {
		return 0, ErrNotConfigured
	}
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return 0, err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.cfg.URL+path, body)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.Token)
	req.Header.Set("Accept", "application/json")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if version > 0 {
		req.Header.Set("If-Match", `"`+strconv.FormatInt(version, 10)+`"`)
	}
	res, err := c.http.Do(req)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, 16<<20))
	if err != nil {
		return res.StatusCode, err
	}
	switch {
	case res.StatusCode == http.StatusNotFound:
		return res.StatusCode, ErrNotFound
	case res.StatusCode == http.StatusPreconditionFailed:
		return res.StatusCode, ErrVersion
	case res.StatusCode >= 300:
		var e struct {
			Error  string              `json:"error"`
			Errors map[string][]string `json:"errors"`
		}
		_ = json.Unmarshal(data, &e)
		if e.Error == "" {
			e.Error = http.StatusText(res.StatusCode)
		}
		return res.StatusCode, &Error{Status: res.StatusCode, Message: e.Error, Fields: e.Errors}
	}
	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return res.StatusCode, fmt.Errorf("luna-pages: %w", err)
		}
	}
	return res.StatusCode, nil
}
