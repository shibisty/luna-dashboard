package controllers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"path"
	"sort"
	"strings"
	"sync"

	"fullstack/web"

	"luna-dashboard/resources"
)

// EditorFile is one file of the page editor's bundle (resources/editor).
type EditorFile struct {
	File    string `json:"file"`
	Package string `json:"package"`
	Version string `json:"version"`
	Source  string `json:"source"`
	SHA256  string `json:"sha256"`
	data    []byte
}

// URL is where the dashboard serves the file; the hash in the query makes
// a new version a new URL, so the browser may keep the old one forever.
func (f EditorFile) URL() string { return "/assets/editor/" + f.File + "?v=" + f.SHA256[:12] }

var bundle struct {
	once  sync.Once
	files map[string]*EditorFile
	err   error
}

// EditorBundle reads the bundle and checks every file against
// manifest.json: a file without its entry, or with another hash, is an
// error (the test suite and the editor page both report it).
func EditorBundle() (map[string]*EditorFile, error) {
	bundle.once.Do(func() { bundle.files, bundle.err = loadBundle(resources.Editor) })
	return bundle.files, bundle.err
}

func loadBundle(fsys fs.FS) (map[string]*EditorFile, error) {
	raw, err := fs.ReadFile(fsys, "manifest.json")
	if err != nil {
		return nil, err
	}
	var m struct {
		Files []*EditorFile `json:"files"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("editor manifest: %w", err)
	}
	out := map[string]*EditorFile{}
	for _, f := range m.Files {
		data, err := fs.ReadFile(fsys, f.File)
		if err != nil {
			return nil, fmt.Errorf("editor bundle: %w", err)
		}
		sum := sha256.Sum256(data)
		if got := hex.EncodeToString(sum[:]); got != f.SHA256 {
			return nil, fmt.Errorf("editor bundle: %s has sha256 %s, manifest.json says %s", f.File, got, f.SHA256)
		}
		f.data = data
		out[f.File] = f
	}
	names, _ := fs.Glob(fsys, "*.*")
	var extra []string
	for _, n := range names {
		if _, ok := out[n]; !ok && n != "manifest.json" && n != "README.md" {
			extra = append(extra, n)
		}
	}
	if len(extra) > 0 {
		sort.Strings(extra)
		return nil, fmt.Errorf("editor bundle: not in manifest.json: %s", strings.Join(extra, ", "))
	}
	return out, nil
}

// EditorAsset serves a file of the editor's bundle: immutable for a year
// when asked for by its hash (?v=), five minutes otherwise.
func EditorAsset(c *web.Ctx) error {
	files, err := EditorBundle()
	if err != nil {
		return err
	}
	f, ok := files[c.Param("file")]
	if !ok {
		return web.ErrNotFound
	}
	h := c.W.Header()
	tag := `"` + f.SHA256[:32] + `"`
	h.Set("ETag", tag)
	if c.Query("v") == f.SHA256[:12] {
		h.Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		h.Set("Cache-Control", "public, max-age=300")
	}
	switch path.Ext(f.File) {
	case ".js":
		h.Set("Content-Type", "text/javascript; charset=utf-8")
	case ".css":
		h.Set("Content-Type", "text/css; charset=utf-8")
	case ".woff2":
		h.Set("Content-Type", "font/woff2")
	case ".woff":
		h.Set("Content-Type", "font/woff")
	}
	if c.R.Header.Get("If-None-Match") == tag {
		c.W.WriteHeader(http.StatusNotModified)
		return nil
	}
	c.W.WriteHeader(http.StatusOK)
	if c.R.Method != http.MethodHead {
		_, err = c.W.Write(f.data)
	}
	return err
}
