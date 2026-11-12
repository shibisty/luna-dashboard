// Package fontsproxy serves the font catalogues the editor's grapesjs-fonts
// plugin browses. The catalogue APIs of Google Fonts, Bunny Fonts,
// Fontsource and Fontshare send no CORS headers, so the browser asks the
// dashboard (same origin) and the dashboard asks them.
//
// Routes are those of the unifont proxy, which the plugin speaks:
//
//	/google/v1/fonts                       https://fonts.google.com/metadata/fonts
//	/bunny/v1/list                         https://fonts.bunny.net/list
//	/fontsource/v1/fonts                   https://api.fontsource.org/v1/fonts
//	/fontshare/v1/fonts?offset=N&limit=100 https://api.fontshare.com/v2/fonts
//
// Nothing else can be requested, Fontshare's pages are limited to the ones
// the plugin reads, and every answer is kept in memory for a day (a stale
// copy is served when the upstream fails), so the dashboard asks each
// upstream at most a few dozen times a day whoever calls it. The fonts
// themselves (stylesheets, files) are loaded by the browser from the CDNs.
package fontsproxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// Proxy fetches and caches the catalogues. The zero value is not ready:
// use New.
type Proxy struct {
	// Client makes the upstream requests.
	Client *http.Client
	// TTL is how long an answer is served from memory.
	TTL time.Duration
	// MaxBytes is the largest catalogue kept (Google's is a few MB).
	MaxBytes int64
	// Now is the clock (tests).
	Now func() time.Time

	mu      sync.Mutex
	entries map[string]*entry
}

type entry struct {
	mu      sync.Mutex // one upstream request per URL at a time
	body    []byte
	fetched time.Time
}

// New returns a proxy with a 25 s timeout, a day's cache and a 32 MiB limit.
func New() *Proxy {
	return &Proxy{
		Client:   &http.Client{Timeout: 25 * time.Second},
		TTL:      24 * time.Hour,
		MaxBytes: 32 << 20,
		Now:      time.Now,
		entries:  map[string]*entry{},
	}
}

// ErrUnknown is a path or query the proxy does not serve.
var ErrUnknown = errors.New("fontsproxy: unknown endpoint")

// Upstream is the catalogue URL for a proxy path ("google/v1/fonts") and
// its query, or ErrUnknown.
func Upstream(path string, offset, limit string) (string, error) {
	switch path {
	case "google/v1/fonts":
		return "https://fonts.google.com/metadata/fonts", nil
	case "bunny/v1/list":
		return "https://fonts.bunny.net/list", nil
	case "fontsource/v1/fonts":
		return "https://api.fontsource.org/v1/fonts", nil
	case "fontshare/v1/fonts":
		// The plugin reads pages of 100, at most 30 of them.
		if limit == "" {
			limit = "100"
		}
		if offset == "" {
			offset = "0"
		}
		o, err := strconv.Atoi(offset)
		if err != nil || limit != "100" || o < 0 || o%100 != 0 || o >= 3000 || strconv.Itoa(o) != offset {
			return "", ErrUnknown
		}
		return "https://api.fontshare.com/v2/fonts?offset=" + offset + "&limit=100", nil
	}
	return "", ErrUnknown
}

// Result is a catalogue and where it came from.
type Result struct {
	Body []byte
	// Cache is HIT, MISS or STALE (the upstream failed, an older copy).
	Cache string
	// Age is how old the copy is.
	Age time.Duration
}

// Get returns the catalogue at url, from memory while it is fresh.
func (p *Proxy) Get(ctx context.Context, url string) (Result, error) {
	p.mu.Lock()
	e := p.entries[url]
	if e == nil {
		e = &entry{}
		p.entries[url] = e
	}
	p.mu.Unlock()

	e.mu.Lock()
	defer e.mu.Unlock()
	now := p.Now()
	if e.body != nil && now.Sub(e.fetched) < p.TTL {
		return Result{e.body, "HIT", now.Sub(e.fetched)}, nil
	}
	body, err := p.fetch(ctx, url)
	if err != nil {
		if e.body != nil {
			return Result{e.body, "STALE", now.Sub(e.fetched)}, nil
		}
		return Result{}, err
	}
	e.body, e.fetched = body, now
	return Result{body, "MISS", 0}, nil
}

func (p *Proxy) fetch(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "luna-dashboard-fonts-proxy/1.0")
	res, err := p.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil, fmt.Errorf("fontsproxy: %s answered %d", url, res.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, p.MaxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > p.MaxBytes {
		return nil, fmt.Errorf("fontsproxy: %s is larger than %d bytes", url, p.MaxBytes)
	}
	// Google prefixes its JSON with an XSSI guard.
	body = bytes.TrimSpace(bytes.TrimPrefix(bytes.TrimSpace(body), []byte(")]}'")))
	if !json.Valid(body) {
		return nil, fmt.Errorf("fontsproxy: %s did not answer JSON", url)
	}
	return body, nil
}

// ServeHTTP answers GET /<route> (the router strips its prefix and passes
// the rest as the request's "path" value).
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	q := r.URL.Query()
	url, err := Upstream(r.PathValue("path"), q.Get("offset"), q.Get("limit"))
	if err != nil {
		jsonError(w, http.StatusNotFound, "unknown endpoint")
		return
	}
	res, err := p.Get(r.Context(), url)
	if err != nil {
		jsonError(w, http.StatusBadGateway, "the font catalogue could not be fetched")
		return
	}
	h := w.Header()
	h.Set("Content-Type", "application/json; charset=utf-8")
	h.Set("X-Fonts-Proxy-Cache", res.Cache)
	if res.Cache == "STALE" {
		h.Set("Cache-Control", "no-cache")
	} else {
		h.Set("Cache-Control", "public, max-age="+strconv.Itoa(int((p.TTL-res.Age).Seconds())))
	}
	h.Set("Content-Length", strconv.Itoa(len(res.Body)))
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_, _ = w.Write(res.Body)
	}
}

func jsonError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
