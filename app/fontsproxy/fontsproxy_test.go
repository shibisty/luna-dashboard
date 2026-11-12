package fontsproxy

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type fake struct {
	calls  atomic.Int32
	fail   atomic.Bool
	answer func(url string) (int, string)
}

func (f *fake) RoundTrip(r *http.Request) (*http.Response, error) {
	f.calls.Add(1)
	if f.fail.Load() {
		return nil, errors.New("network down")
	}
	status, body := f.answer(r.URL.String())
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}, Request: r}, nil
}

func setup(t *testing.T) (*Proxy, *fake, *time.Time, http.Handler) {
	f := &fake{answer: func(url string) (int, string) {
		switch url {
		case "https://fonts.google.com/metadata/fonts":
			return 200, ")]}'\n{\"familyMetadataList\":[]}"
		case "https://api.fontshare.com/v2/fonts?offset=100&limit=100":
			return 200, `{"fonts":[],"has_more":false}`
		case "https://fonts.bunny.net/list":
			return 200, `<html>not json</html>`
		}
		return 500, "boom"
	}}
	p := New()
	p.Client = &http.Client{Transport: f}
	now := time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)
	p.Now = func() time.Time { return now }
	mux := http.NewServeMux()
	mux.Handle("/editor/fonts/{path...}", p)
	return p, f, &now, mux
}

func get(h http.Handler, url string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", url, nil))
	return w
}

func TestCatalogues(t *testing.T) {
	_, f, now, h := setup(t)
	w := get(h, "/editor/fonts/google/v1/fonts")
	if w.Code != 200 || w.Body.String() != `{"familyMetadataList":[]}` || w.Header().Get("X-Fonts-Proxy-Cache") != "MISS" {
		t.Fatal(w.Code, w.Body.String(), w.Header())
	}
	if w.Header().Get("Content-Type") != "application/json; charset=utf-8" || w.Header().Get("Cache-Control") != "public, max-age=86400" {
		t.Fatal(w.Header())
	}
	*now = now.Add(time.Hour)
	w = get(h, "/editor/fonts/google/v1/fonts")
	if w.Header().Get("X-Fonts-Proxy-Cache") != "HIT" || f.calls.Load() != 1 || w.Header().Get("Cache-Control") != "public, max-age=82800" {
		t.Fatal(w.Header(), f.calls.Load())
	}
	// A day later the upstream is down: the old copy, marked stale.
	*now = now.Add(24 * time.Hour)
	f.fail.Store(true)
	w = get(h, "/editor/fonts/google/v1/fonts")
	if w.Code != 200 || w.Header().Get("X-Fonts-Proxy-Cache") != "STALE" || w.Body.String() != `{"familyMetadataList":[]}` {
		t.Fatal(w.Code, w.Header())
	}
	f.fail.Store(false)
	if w := get(h, "/editor/fonts/google/v1/fonts"); w.Header().Get("X-Fonts-Proxy-Cache") != "MISS" {
		t.Fatal("refreshed")
	}

	if w := get(h, "/editor/fonts/fontshare/v1/fonts?offset=100&limit=100"); w.Code != 200 {
		t.Fatal(w.Code)
	}
	// Not JSON, an upstream error: 502, nothing cached.
	if w := get(h, "/editor/fonts/bunny/v1/list"); w.Code != 502 {
		t.Fatal(w.Code)
	}
	if w := get(h, "/editor/fonts/fontsource/v1/fonts"); w.Code != 502 || !strings.Contains(w.Body.String(), "could not be fetched") {
		t.Fatal(w.Code, w.Body.String())
	}
}

func TestOnlyTheCatalogues(t *testing.T) {
	_, f, _, h := setup(t)
	for _, u := range []string{
		"/editor/fonts/", "/editor/fonts/google/v1/fonts/x", "/editor/fonts/evil.example/x",
		"/editor/fonts/fontshare/v1/fonts?offset=50&limit=100", "/editor/fonts/fontshare/v1/fonts?offset=0&limit=5",
		"/editor/fonts/fontshare/v1/fonts?offset=3000&limit=100", "/editor/fonts/fontshare/v1/fonts?offset=-100&limit=100",
		"/editor/fonts/fontshare/v1/fonts?offset=0100&limit=100",
	} {
		if w := get(h, u); w.Code != 404 {
			t.Errorf("%s: %d", u, w.Code)
		}
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("POST", "/editor/fonts/google/v1/fonts", nil))
	if w.Code != 405 || f.calls.Load() != 0 {
		t.Fatal(w.Code, f.calls.Load())
	}
}

func TestTooLarge(t *testing.T) {
	p, _, _, h := setup(t)
	p.MaxBytes = 10
	if w := get(h, "/editor/fonts/google/v1/fonts"); w.Code != 502 {
		t.Fatal(w.Code)
	}
}
