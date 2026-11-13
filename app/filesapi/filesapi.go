// Package filesapi is the dashboard's client of luna-files' internal API
// (/internal/v1/apps/{app}/…): the file browser's folders, files, uploads
// and links, and the page editor's assets. luna-files trusts the path's
// application, so callers check that the signed-in developer owns it first.
package filesapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"
	"time"

	"fullstack"
	"fullstack/env"
)

// Config is how the dashboard reaches luna-files.
type Config struct {
	// URL is luna-files' internal address, e.g. http://luna-files:8080.
	// Empty: the files screens say that luna-files is not connected.
	URL string `env:"LUNA_FILES_URL"`
	// Token is luna-files' LUNA_DASHBOARD_TOKEN (one of LUNA_SERVICE_TOKENS).
	Token string `env:"LUNA_FILES_TOKEN"`
	// Timeout covers one request, an upload included.
	Timeout time.Duration `env:"LUNA_FILES_TIMEOUT" default:"5m"`
	// MaxUpload is the largest file the dashboard sends through luna-files'
	// API (luna-files' FILES_MAX_UPLOAD); larger ones go straight to the
	// store from the browser.
	MaxUpload int64 `env:"LUNA_FILES_MAX_UPLOAD" default:"10485760"`
}

// ErrNotConfigured: LUNA_FILES_URL or LUNA_FILES_TOKEN is not set.
var ErrNotConfigured = errors.New("luna-files is not connected (LUNA_FILES_URL, LUNA_FILES_TOKEN)")

// ErrNotFound: no such folder or file.
var ErrNotFound = errors.New("not found")

// Error is an answer of luna-files other than success and 404.
type Error struct {
	Status  int
	Message string
	Fields  map[string][]string // 422
}

func (e *Error) Error() string { return fmt.Sprintf("luna-files: %d %s", e.Status, e.Message) }

// Folder is a folder.
type Folder struct {
	ID        string    `json:"id"`
	ParentID  *string   `json:"parent_id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// File is a file's metadata.
type File struct {
	ID         string    `json:"id"`
	FolderID   *string   `json:"folder_id"`
	Name       string    `json:"name"`
	Mime       string    `json:"mime"`
	Size       int64     `json:"size"`
	SHA256     string    `json:"sha256,omitempty"`
	Width      *int      `json:"width,omitempty"`
	Height     *int      `json:"height,omitempty"`
	Visibility string    `json:"visibility"`
	Status     string    `json:"status"`
	URL        string    `json:"url,omitempty"`
	Version    int       `json:"version"`
	ContentAt  time.Time `json:"content_at"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// Version is an old content of a file.
type Version struct {
	Number     int       `json:"number"`
	Mime       string    `json:"mime"`
	Size       int64     `json:"size"`
	SHA256     string    `json:"sha256,omitempty"`
	Width      *int      `json:"width,omitempty"`
	Height     *int      `json:"height,omitempty"`
	ContentAt  time.Time `json:"content_at"`
	ArchivedAt time.Time `json:"archived_at"`
}

// IsImage reports whether the file is an image a browser shows.
func (f File) IsImage() bool {
	switch f.Mime {
	case "image/png", "image/jpeg", "image/gif", "image/webp", "image/avif", "image/svg+xml":
		return true
	}
	return false
}

// Public reports whether anyone may read the file at its URL.
func (f File) Public() bool { return f.Visibility == "public" }

// Page is a page of files.
type Page struct {
	Data   []File `json:"data"`
	Total  int    `json:"total"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
}

// Query selects files.
type Query struct {
	Folder string // "" the root
	Search string // in every folder when Folder is ""
	Images bool
	Limit  int
	Offset int
}

// Usage is an application's storage.
type Usage struct {
	Used     int64 `json:"used"`
	Limit    int64 `json:"limit"`
	Files    int   `json:"files"`
	Versions int   `json:"versions"`
	History  int64 `json:"history"`
	Stored   int64 `json:"stored"`
}

// Saved is what the history's deduplication saves in the store.
func (u Usage) Saved() int64 {
	if d := u.Used - u.Stored; d > 0 {
		return d
	}
	return 0
}

// Percent is how full the storage is, 0–100.
func (u Usage) Percent() int {
	if u.Limit <= 0 {
		return 0
	}
	p := int(u.Used * 100 / u.Limit)
	if p > 100 {
		p = 100
	}
	if p == 0 && u.Used > 0 {
		p = 1
	}
	return p
}

// Link is a URL to read a file; ExpiresAt for signed ones.
type Link struct {
	URL       string     `json:"url"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// Upload describes a file sent through the API.
type Upload struct {
	Folder     string // "" the root
	Name       string
	Mime       string
	Visibility string // public (default) or private
	Rename     bool   // a free name instead of a conflict
	Replace    string // a file whose content this replaces (a new version)
}

// FilePatch changes a file: nil fields stay.
type FilePatch struct {
	Name       *string `json:"name,omitempty"`
	FolderID   *string `json:"-"` // "" the root
	Visibility *string `json:"visibility,omitempty"`
}

// FolderPatch changes a folder: nil fields stay.
type FolderPatch struct {
	Name     *string `json:"name,omitempty"`
	ParentID *string `json:"-"` // "" the root
}

// Client calls luna-files.
type Client struct {
	cfg  Config
	http *http.Client
}

// New makes a client.
func New(cfg Config) *Client {
	cfg.URL = strings.TrimRight(cfg.URL, "/")
	if cfg.MaxUpload <= 0 {
		cfg.MaxUpload = 10 << 20
	}
	return &Client{cfg: cfg, http: &http.Client{Timeout: cfg.Timeout}}
}

// Provider puts a *Client into the container.
type Provider struct{}

// Register implements fullstack.Provider.
func (Provider) Register(app *fullstack.App) error {
	var cfg Config
	if err := env.Decode(&cfg); err != nil {
		return fmt.Errorf("filesapi: %w", err)
	}
	fullstack.Set(app, New(cfg))
	return nil
}

// Configured reports whether luna-files can be called.
func (c *Client) Configured() bool { return c.cfg.URL != "" && c.cfg.Token != "" }

// MaxUpload is the largest file sent through the API.
func (c *Client) MaxUpload() int64 { return c.cfg.MaxUpload }

func base(app string) string { return "/internal/v1/apps/" + url.PathEscape(app) }

/* ---------------- the browser's requests ---------------- */

// Allowed reports whether the browser (the file browser's uploads, the page
// editor's asset manager) may reach a path of the internal API through the
// dashboard: listing and reading files, uploads, completing them. Quotas
// and the rest stay closed.
func Allowed(method, path string) bool {
	seg := strings.Split(path, "/")
	for _, s := range seg {
		if s == "" || s == "." || s == ".." {
			return false
		}
	}
	switch {
	case len(seg) == 1 && seg[0] == "files":
		return method == http.MethodGet || method == http.MethodPost
	case len(seg) == 1 && (seg[0] == "folders" || seg[0] == "usage"):
		return method == http.MethodGet
	case len(seg) == 1 && seg[0] == "uploads":
		return method == http.MethodPost
	case len(seg) == 2 && seg[0] == "files":
		return method == http.MethodGet
	case len(seg) == 3 && seg[0] == "files" && seg[2] == "complete":
		return method == http.MethodPost
	case len(seg) == 3 && seg[0] == "files" && seg[2] == "url":
		return method == http.MethodGet
	}
	return false
}

var (
	forwardIn  = []string{"Accept", "Content-Type", "Content-Length"}
	forwardOut = []string{"Content-Type", "Cache-Control"}
)

// Forward sends one browser request to luna-files' internal API for the
// application and copies the answer back, streaming the body (a multipart
// upload up to the limit and a little more for its fields).
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
		body = http.MaxBytesReader(w, r.Body, c.cfg.MaxUpload+1<<20)
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
	if req.Body != nil && r.ContentLength > 0 {
		req.ContentLength = r.ContentLength
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
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(res.StatusCode)
	_, err = io.Copy(w, res.Body)
	return err
}

/* ---------------- folders ---------------- */

// Folders lists the folders in parent ("" the root).
func (c *Client) Folders(ctx context.Context, app, parent string) ([]Folder, error) {
	q := ""
	if parent != "" {
		q = "?" + url.Values{"parent_id": {parent}}.Encode()
	}
	var out struct {
		Data []Folder `json:"data"`
	}
	err := c.do(ctx, http.MethodGet, base(app)+"/folders"+q, nil, &out)
	return out.Data, err
}

// AllFolders lists every folder of the application.
func (c *Client) AllFolders(ctx context.Context, app string) ([]Folder, error) {
	var out struct {
		Data []Folder `json:"data"`
	}
	err := c.do(ctx, http.MethodGet, base(app)+"/folders?all=1", nil, &out)
	return out.Data, err
}

// Path is the folders from the root down to id.
func (c *Client) Path(ctx context.Context, app, id string) ([]Folder, error) {
	var out struct {
		Path []Folder `json:"path"`
	}
	err := c.do(ctx, http.MethodGet, base(app)+"/folders/"+url.PathEscape(id), nil, &out)
	return out.Path, err
}

func nullable(id string) any {
	if id == "" {
		return nil
	}
	return id
}

// CreateFolder makes a folder in parent ("" the root).
func (c *Client) CreateFolder(ctx context.Context, app, parent, name string) (Folder, error) {
	var f Folder
	err := c.do(ctx, http.MethodPost, base(app)+"/folders", map[string]any{"name": name, "parent_id": nullable(parent)}, &f)
	return f, err
}

// UpdateFolder renames and/or moves a folder.
func (c *Client) UpdateFolder(ctx context.Context, app, id string, p FolderPatch) (Folder, error) {
	in := map[string]any{}
	if p.Name != nil {
		in["name"] = *p.Name
	}
	if p.ParentID != nil {
		in["parent_id"] = nullable(*p.ParentID)
	}
	var f Folder
	err := c.do(ctx, http.MethodPatch, base(app)+"/folders/"+url.PathEscape(id), in, &f)
	return f, err
}

// DeleteFolder deletes a folder; recursive also everything in it. It
// returns the number of files deleted.
func (c *Client) DeleteFolder(ctx context.Context, app, id string, recursive bool) (int, error) {
	q := ""
	if recursive {
		q = "?recursive=1"
	}
	var out struct {
		Deleted int `json:"deleted_files"`
	}
	err := c.do(ctx, http.MethodDelete, base(app)+"/folders/"+url.PathEscape(id)+q, nil, &out)
	return out.Deleted, err
}

/* ---------------- files ---------------- */

// Files lists files.
func (c *Client) Files(ctx context.Context, app string, q Query) (Page, error) {
	v := url.Values{}
	if q.Folder != "" {
		v.Set("folder_id", q.Folder)
	}
	if q.Search != "" {
		v.Set("q", q.Search)
	}
	if q.Images {
		v.Set("images", "1")
	}
	if q.Limit > 0 {
		v.Set("limit", strconv.Itoa(q.Limit))
	}
	if q.Offset > 0 {
		v.Set("offset", strconv.Itoa(q.Offset))
	}
	path := base(app) + "/files"
	if len(v) > 0 {
		path += "?" + v.Encode()
	}
	var p Page
	err := c.do(ctx, http.MethodGet, path, nil, &p)
	return p, err
}

// File is one file.
func (c *Client) File(ctx context.Context, app, id string) (File, error) {
	var f File
	err := c.do(ctx, http.MethodGet, base(app)+"/files/"+url.PathEscape(id), nil, &f)
	return f, err
}

// URL is a link to read a file; download makes it an attachment.
func (c *Client) URL(ctx context.Context, app, id string, download bool) (Link, error) {
	q := ""
	if download {
		q = "?download=1"
	}
	var l Link
	err := c.do(ctx, http.MethodGet, base(app)+"/files/"+url.PathEscape(id)+"/url"+q, nil, &l)
	return l, err
}

// UpdateFile renames, moves and/or changes a file's visibility.
func (c *Client) UpdateFile(ctx context.Context, app, id string, p FilePatch) (File, error) {
	in := map[string]any{}
	if p.Name != nil {
		in["name"] = *p.Name
	}
	if p.FolderID != nil {
		in["folder_id"] = nullable(*p.FolderID)
	}
	if p.Visibility != nil {
		in["visibility"] = *p.Visibility
	}
	var f File
	err := c.do(ctx, http.MethodPatch, base(app)+"/files/"+url.PathEscape(id), in, &f)
	return f, err
}

// DeleteFile deletes a file and its bytes.
func (c *Client) DeleteFile(ctx context.Context, app, id string) error {
	return c.do(ctx, http.MethodDelete, base(app)+"/files/"+url.PathEscape(id), nil, nil)
}

// Versions is a file's history, newest first.
func (c *Client) Versions(ctx context.Context, app, id string) ([]Version, error) {
	var out struct {
		Data []Version `json:"data"`
	}
	err := c.do(ctx, http.MethodGet, base(app)+"/files/"+url.PathEscape(id)+"/versions", nil, &out)
	return out.Data, err
}

func versionPath(app, id string, n int) string {
	return base(app) + "/files/" + url.PathEscape(id) + "/versions/" + strconv.Itoa(n)
}

// VersionURL is a signed link to an old version.
func (c *Client) VersionURL(ctx context.Context, app, id string, n int, download bool) (Link, error) {
	q := ""
	if download {
		q = "?download=1"
	}
	var l Link
	err := c.do(ctx, http.MethodGet, versionPath(app, id, n)+"/url"+q, nil, &l)
	return l, err
}

// Restore makes an old version the file's content again.
func (c *Client) Restore(ctx context.Context, app, id string, n int) (File, error) {
	var f File
	err := c.do(ctx, http.MethodPost, versionPath(app, id, n)+"/restore", nil, &f)
	return f, err
}

// DeleteVersion removes an old version.
func (c *Client) DeleteVersion(ctx context.Context, app, id string, n int) error {
	return c.do(ctx, http.MethodDelete, versionPath(app, id, n), nil, nil)
}

// Usage is the application's storage.
func (c *Client) Usage(ctx context.Context, app string) (Usage, error) {
	var u Usage
	err := c.do(ctx, http.MethodGet, base(app)+"/usage", nil, &u)
	return u, err
}

// Upload sends a file through the API as multipart, streaming body.
func (c *Client) Upload(ctx context.Context, app string, u Upload, body io.Reader) (File, error) {
	if !c.Configured() {
		return File{}, ErrNotConfigured
	}
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	go func() {
		err := func() error {
			fields := [][2]string{{"folder_id", u.Folder}, {"name", u.Name}, {"visibility", u.Visibility}, {"replace_id", u.Replace}}
			if u.Rename {
				fields = append(fields, [2]string{"rename", "1"})
			}
			for _, f := range fields {
				if f[1] == "" {
					continue
				}
				if err := mw.WriteField(f[0], f[1]); err != nil {
					return err
				}
			}
			h := textproto.MIMEHeader{}
			h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, quote(u.Name)))
			ct := u.Mime
			if ct == "" {
				ct = "application/octet-stream"
			}
			h.Set("Content-Type", ct)
			part, err := mw.CreatePart(h)
			if err != nil {
				return err
			}
			if _, err := io.Copy(part, body); err != nil {
				return err
			}
			return mw.Close()
		}()
		pw.CloseWithError(err)
	}()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.URL+base(app)+"/files", pr)
	if err != nil {
		pr.Close()
		return File{}, err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	var f File
	err = c.send(req, &f)
	pr.Close()
	return f, err
}

var quoter = strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\r", "", "\n", "")

func quote(s string) string { return quoter.Replace(s) }

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
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.send(req, out)
}

func (c *Client) send(req *http.Request, out any) error {
	req.Header.Set("Authorization", "Bearer "+c.cfg.Token)
	req.Header.Set("Accept", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, 16<<20))
	if err != nil {
		return err
	}
	switch {
	case res.StatusCode == http.StatusNotFound:
		return ErrNotFound
	case res.StatusCode >= 300:
		var e struct {
			Error  string              `json:"error"`
			Errors map[string][]string `json:"errors"`
		}
		_ = json.Unmarshal(data, &e)
		if e.Error == "" {
			e.Error = http.StatusText(res.StatusCode)
		}
		return &Error{Status: res.StatusCode, Message: e.Error, Fields: e.Errors}
	}
	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("luna-files: %w", err)
		}
	}
	return nil
}
