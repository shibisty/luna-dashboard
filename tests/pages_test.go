package tests

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"

	apptest "fullstack-testing"

	"luna-dashboard/app/http/controllers"
)

const pagesToken = "pages-internal-token"

// fakePages is luna-pages' internal API over memory: what the dashboard
// forwards (recorded) and the content API the quick forms use.
type fakePages struct {
	mu          sync.Mutex
	calls       []fakeCall
	templates   map[string]map[string]any // id → template
	entries     map[string]map[string]any // id → entry
	publishFail map[string][]string       // next publish answers 422 with these errors
}

type fakeCall struct {
	Method, Path, Query, IfMatch, Cookie, CSRF, Body string
}

func newFakePages(t *testing.T) *fakePages {
	f := &fakePages{templates: map[string]map[string]any{}, entries: map[string]map[string]any{}}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	t.Setenv("LUNA_PAGES_URL", srv.URL)
	t.Setenv("LUNA_PAGES_TOKEN", pagesToken)
	return f
}

func (f *fakePages) last() fakeCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.calls) == 0 {
		return fakeCall{}
	}
	return f.calls[len(f.calls)-1]
}

func (f *fakePages) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func (f *fakePages) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	body, _ := io.ReadAll(r.Body)
	f.calls = append(f.calls, fakeCall{r.Method, r.URL.Path, r.URL.RawQuery, r.Header.Get("If-Match"), r.Header.Get("Cookie"), r.Header.Get("X-CSRF-Token"), string(body)})
	if r.Header.Get("Authorization") != "Bearer "+pagesToken {
		reply(w, 401, map[string]string{"error": "missing or invalid service token"})
		return
	}
	rest, ok := strings.CutPrefix(r.URL.Path, "/internal/v1/apps/")
	parts := strings.Split(rest, "/")
	if !ok || len(parts) < 2 {
		reply(w, 404, map[string]string{"error": "Not found."})
		return
	}
	path := strings.Join(parts[1:], "/")
	version := func(e map[string]any) bool {
		if m := r.Header.Get("If-Match"); m != "" && m != `"`+strconv.Itoa(e["version"].(int))+`"` {
			reply(w, 412, map[string]string{"error": "Changed since you loaded it."})
			return false
		}
		return true
	}
	switch {
	case strings.HasPrefix(path, "editor/"):
		w.Header().Set("ETag", `"3"`)
		reply(w, 200, map[string]any{"forwarded": path, "body": json.RawMessage(orNull(body))})
	case path == "content/templates" && r.Method == "GET":
		items := []map[string]any{}
		for _, t := range f.templates {
			items = append(items, t)
		}
		reply(w, 200, map[string]any{"items": items})
	case strings.HasPrefix(path, "content/templates/") && r.Method == "GET":
		if t, ok := f.templates[parts[3]]; ok {
			reply(w, 200, t)
		} else {
			reply(w, 404, map[string]string{"error": "Not found."})
		}
	case path == "content/entries" && r.Method == "GET":
		items := []map[string]any{}
		for _, e := range f.entries {
			items = append(items, map[string]any{"id": e["id"], "template_id": e["template_id"], "name": e["name"], "slug": e["slug"], "status": e["status"], "version": e["version"], "updated_at": "2026-11-11T10:00:00Z"})
		}
		reply(w, 200, map[string]any{"items": items})
	case len(parts) == 4 && parts[1] == "content" && parts[2] == "entries":
		id := parts[3]
		e, exists := f.entries[id]
		switch r.Method {
		case "GET":
			if !exists {
				reply(w, 404, map[string]string{"error": "Not found."})
				return
			}
			reply(w, 200, e)
		case "PUT":
			var in struct {
				TemplateID string                     `json:"template_id"`
				Name       string                     `json:"name"`
				Slug       *string                    `json:"slug"`
				Values     map[string]json.RawMessage `json:"values"`
			}
			_ = json.Unmarshal(body, &in)
			if in.Slug != nil && strings.Contains(*in.Slug, " ") {
				reply(w, 422, map[string]any{"error": "The given data was invalid.", "errors": map[string][]string{"slug": {"Use lower-case letters, digits and dashes."}}})
				return
			}
			if !exists {
				slug := "/" + strings.ToLower(strings.ReplaceAll(in.Name, " ", "-"))
				if in.Slug != nil {
					slug = *in.Slug
				}
				e = map[string]any{"id": id, "template_id": in.TemplateID, "slug": slug, "status": "draft", "version": 0, "values": map[string]json.RawMessage{}, "updated_at": "2026-11-11T10:00:00Z"}
				f.entries[id] = e
			} else if !version(e) {
				return
			}
			e["name"] = in.Name
			if in.Slug != nil {
				e["slug"] = *in.Slug
			}
			vals := e["values"].(map[string]json.RawMessage)
			for k, v := range in.Values {
				if string(v) == "null" {
					delete(vals, k)
				} else {
					vals[k] = v
				}
			}
			e["version"] = e["version"].(int) + 1
			e["status"] = "draft"
			reply(w, map[bool]int{true: 200, false: 201}[exists], e)
		case "DELETE":
			if exists && version(e) {
				delete(f.entries, id)
				w.WriteHeader(204)
			}
		}
	case len(parts) == 5 && parts[2] == "entries" && parts[4] == "publish" && r.Method == "POST":
		e := f.entries[parts[3]]
		if !version(e) {
			return
		}
		if f.publishFail != nil {
			reply(w, 422, map[string]any{"error": "The given data was invalid.", "errors": f.publishFail})
			f.publishFail = nil
			return
		}
		e["status"] = "published"
		reply(w, 200, e)
	case path == "render" && r.Method == "GET":
		id := r.URL.Query().Get("id")
		if e, ok := f.entries[id]; ok && e["status"] == "published" && r.URL.Query().Get("draft") == "1" {
			reply(w, 200, map[string]any{"id": id, "title": e["name"], "html": `<h1 data-field="title">Grinder</h1><script>window.x=1</script>`, "css": `h1{color:red}`})
			return
		}
		reply(w, 404, map[string]string{"error": "Not found."})
	default:
		reply(w, 404, map[string]string{"error": "Not found."})
	}
}

func orNull(b []byte) []byte {
	if len(b) == 0 {
		return []byte("null")
	}
	return b
}

func TestEditorBundle(t *testing.T) {
	files, err := controllers.EditorBundle()
	if err != nil {
		t.Fatal(err) // a file and manifest.json disagree: see resources/editor/README.md
	}
	for _, name := range []string{"grapes.min.js", "grapes.min.css", "grapesjs-pages.js", "grapesjs-storage.js"} {
		if files[name] == nil {
			t.Fatalf("%s is not in the bundle", name)
		}
	}
	if v := files["grapes.min.js"].Version; !strings.HasPrefix(v, "0.23.") {
		t.Fatalf("GrapesJS %s, the dashboard pins 0.23.x", v)
	}

	c := apptest.New(t, newApp(t))
	f := files["grapesjs-pages.js"]
	r := c.Get(f.URL()).AssertOK().AssertHeader("Cache-Control", "public, max-age=31536000, immutable").AssertHeader("Content-Type", "text/javascript; charset=utf-8")
	sum := sha256.Sum256([]byte(r.Text()))
	if hex.EncodeToString(sum[:]) != f.SHA256 {
		t.Fatal("served bytes differ from the manifest")
	}
	c.Get("/assets/editor/grapesjs-pages.js").AssertOK().AssertHeader("Cache-Control", "public, max-age=300")
	c.WithHeader("If-None-Match", r.Header().Get("ETag")).Get(f.URL()).AssertStatus(http.StatusNotModified)
	c.Get("/assets/editor/grapes.min.css").AssertOK().AssertHeader("Content-Type", "text/css; charset=utf-8")
	c.Get("/assets/editor/manifest.json").AssertNotFound()
	c.Get("/assets/editor/README.md").AssertNotFound()
}

func TestPageEditor(t *testing.T) {
	pages := newFakePages(t)
	app := newApp(t)
	c := developer(t, apptest.New(t, app), "ann@example.com")
	id := createApp(t, c, "Shop")
	base := "/apps/" + id + "/pages"

	c.Get("/apps/"+id).AssertOK().AssertSee(base, base+"/entries")
	r := c.Get(base).AssertOK().AssertHeader("Cache-Control", "no-store").
		AssertSee("/assets/editor/grapes.min.js?v=", "/assets/editor/grapesjs-pages.js?v=", "/assets/editor/grapesjs-storage.js?v=", `"api":"`+base+`/api"`)
	csrf := c.Cookie("csrf_token")
	if csrf == "" || !strings.Contains(r.Text(), `"csrf":"`+csrf+`"`) {
		t.Fatal("the editor page does not carry the CSRF token")
	}
	// grapesjs-pages loads before grapesjs-storage (which then leaves the designs to it).
	if strings.Index(r.Text(), "grapesjs-pages.js") > strings.Index(r.Text(), "grapesjs-storage.js") {
		t.Fatal("grapesjs-storage loads before grapesjs-pages")
	}

	// The proxy: path, query, body, If-Match and the service token go to
	// luna-pages; the developer's cookies and CSRF token do not.
	pid := "0b0f7a52-9a64-4c55-8a51-6c7a1f1e2d3c"
	c.WithHeader("If-Match", `"2"`).PutJSON(base+"/api/editor/pages/"+pid, map[string]any{"name": "Home"}).
		AssertOK().AssertHeader("ETag", `"3"`).AssertHeader("Cache-Control", "no-store").AssertJSONPath("forwarded", "editor/pages/"+pid).AssertJSONPath("body.name", "Home")
	got := pages.last()
	if got.Method != "PUT" || got.Path != "/internal/v1/apps/"+id+"/editor/pages/"+pid || got.IfMatch != `"2"` || got.Cookie != "" || got.CSRF != "" {
		t.Fatalf("forwarded %+v", got)
	}
	c.GetJSON(base + "/api/render?id=" + pid + "&draft=1").AssertNotFound()
	if got := pages.last(); got.Path != "/internal/v1/apps/"+id+"/render" || got.Query != "id="+pid+"&draft=1" {
		t.Fatalf("forwarded %+v", got)
	}

	// What the proxy refuses without asking luna-pages.
	n := pages.count()
	c.WithoutCSRF().PutJSON(base+"/api/editor/pages/"+pid, map[string]any{}).AssertForbidden()
	c.GetJSON(base + "/api/healthz").AssertNotFound()
	c.GetJSON(base + "/api/editor").AssertNotFound()
	c.PostJSON(base+"/api/render", map[string]any{}).AssertNotFound()
	c.GetJSON(base + "/api/pages/a/b").AssertNotFound()
	c.GetJSON(base + "/api/editor/%2e%2e/%2e%2e/keys").AssertNotFound()
	big := map[string]any{"data": strings.Repeat("x", 4<<20)}
	c.PutJSON(base+"/api/editor/pages/"+pid, big).AssertStatus(http.StatusRequestEntityTooLarge)
	if pages.count() != n {
		t.Fatalf("luna-pages was asked %d times for refused requests", pages.count()-n)
	}

	// Someone else's application, and no session.
	other := developer(t, apptest.New(t, app), "eve@example.com")
	other.Get(base).AssertNotFound()
	other.GetJSON(base + "/api/editor/pages").AssertNotFound()
	other.Get(base + "/entries").AssertNotFound()
	apptest.New(t, app).GetJSON(base + "/api/editor/pages").AssertUnauthorized()
	apptest.New(t, app).Get(base).AssertRedirect()
	if pages.count() != n {
		t.Fatal("luna-pages was asked about someone else's application")
	}
}

func TestPagesWithoutLunaPages(t *testing.T) {
	t.Setenv("LUNA_PAGES_URL", "")
	c := developer(t, apptest.New(t, newApp(t)), "ann@example.com")
	id := createApp(t, c, "Shop")
	c.Get("/apps/" + id + "/pages").AssertStatus(http.StatusBadGateway).AssertSee("not connected")
	c.Get("/apps/" + id + "/pages/entries").AssertStatus(http.StatusBadGateway).AssertSee("not connected")
	c.GetJSON("/apps/" + id + "/pages/api/editor/pages").AssertStatus(http.StatusBadGateway)
}

var hiddenVersion = regexp.MustCompile(`name="version" value="(\d+)"`)

func TestEntryForms(t *testing.T) {
	pages := newFakePages(t)
	app := newApp(t)
	c := developer(t, apptest.New(t, app), "ann@example.com")
	id := createApp(t, c, "Shop")
	base := "/apps/" + id + "/pages/entries"

	c.Get(base).AssertOK().AssertSee("No templates yet")
	tpl := "7c1f15c0-0000-4000-8000-000000000001"
	pages.templates[tpl] = map[string]any{"id": tpl, "name": "Product", "version": 1, "updated_at": "2026-11-11T10:00:00Z", "fields": []map[string]any{
		{"name": "title", "type": "text", "required": true},
		{"name": "price", "type": "number", "format": "0.00"},
		{"name": "photo", "type": "image"},
		{"name": "buy", "type": "link"},
		{"name": "in_stock", "type": "boolean", "default": true},
		{"name": "color", "type": "select", "options": []string{"black", "white"}},
		{"name": "released", "type": "date"},
		{"name": "about", "type": "richtext"},
		{"name": "specs", "type": "list", "fields": []map[string]any{{"name": "name", "type": "text"}, {"name": "value", "type": "text"}}},
	}}
	c.Get(base).AssertOK().AssertSee("Product", "No entries yet", "New entry")

	// Create: the template's defaults, then the form.
	c.Post(base, url.Values{"template": {"nope"}, "name": {"X"}}).AssertUnprocessable().AssertSee("No such template")
	r := c.Post(base, url.Values{"template": {tpl}, "name": {"Coffee grinder"}, "slug": {"/products/grinder"}}).AssertRedirect()
	entry := r.Header().Get("Location")
	eid := strings.TrimPrefix(entry, base+"/")
	if len(eid) != 36 || pages.entries[eid] == nil {
		t.Fatalf("redirect to %q", entry)
	}
	if v := string(pages.entries[eid]["values"].(map[string]json.RawMessage)["in_stock"]); v != "true" {
		t.Fatalf("default in_stock = %s", v)
	}
	page := c.Get(entry).AssertOK().AssertSee("Coffee grinder", "/products/grinder", `name="v.title"`, `name="v.photo.url"`, `name="v.specs.0.name"`, "New item", `type="date"`, ">white</option>")
	ver := hiddenVersion.FindStringSubmatch(page.Text())[1]

	// Save: every type, a list with an item to remove and the new row.
	form := url.Values{
		"version": {ver}, "name": {"Coffee grinder"}, "slug": {"/products/grinder"},
		"v.title": {"  Grinder  "}, "v.price": {"49,5"}, "v.photo.url": {"https://cdn.example.com/g.jpg"}, "v.photo.alt": {"A grinder"},
		"v.buy.url": {""}, "v.buy.label": {""}, "v.color": {"black"}, "v.released": {"2026-11-01"}, "v.about": {"<p>Steel <b>burrs</b></p>"},
		"v.specs.0.name": {"Burrs"}, "v.specs.0.value": {"steel"},
		"v.specs.1.name": {"Weight"}, "v.specs.1.value": {"1 kg"}, "v.specs.1.remove": {"1"},
		"v.specs.2.name": {"Warranty"}, "v.specs.2.value": {""},
		"v.specs.3.name": {""}, "v.specs.3.value": {""},
	}
	c.Post(entry, form).AssertRedirect(entry + "?saved=draft")
	vals := pages.entries[eid]["values"].(map[string]json.RawMessage)
	want := map[string]string{
		"title": `"Grinder"`, "price": `49.5`, "photo": `{"alt":"A grinder","url":"https://cdn.example.com/g.jpg"}`,
		"in_stock": `false`, "color": `"black"`, "released": `"2026-11-01"`, "about": `"<p>Steel <b>burrs</b></p>"`,
		"specs": `[{"name":"Burrs","value":"steel"},{"name":"Warranty"}]`,
	}
	norm := func(raw string) string {
		var v any
		_ = json.Unmarshal([]byte(raw), &v)
		b, _ := json.Marshal(v)
		return string(b)
	}
	for k, w := range want {
		if norm(string(vals[k])) != norm(w) {
			t.Errorf("%s = %s, want %s", k, vals[k], w)
		}
	}
	if _, ok := vals["buy"]; ok {
		t.Error("an empty link was kept")
	}
	c.Get(entry).AssertOK().AssertSee(`value="Grinder"`, `value="49.5"`, "Item 2", `value="Warranty"`, "New item")

	// Mistakes caught here, and luna-pages' 422.
	bad := url.Values{"version": {"2"}, "name": {"Coffee grinder"}, "slug": {"/products/grinder"}, "v.price": {"cheap"}, "v.color": {"red"}, "v.specs.5.name": {"x"}, "v.specs.5.value": {""}}
	c.Post(entry, bad).AssertUnprocessable().AssertSee("Enter a number", "Choose one of the options", `value="cheap"`)
	c.Post(entry, url.Values{"version": {"2"}, "name": {""}}).AssertUnprocessable().AssertSee("Give the entry a name")
	c.Post(entry, url.Values{"version": {"2"}, "name": {"Coffee grinder"}, "slug": {"no spaces please"}}).AssertUnprocessable().AssertSee("lower-case letters")

	// A version from before someone else's save: 409, nothing lost silently.
	c.Post(entry, url.Values{"version": {"1"}, "name": {"Old"}, "slug": {"/products/grinder"}, "v.title": {"Old title"}}).
		AssertStatus(http.StatusConflict).AssertSee("Someone changed this entry", `value="Old title"`)
	if string(pages.entries[eid]["values"].(map[string]json.RawMessage)["title"]) != `"Grinder"` {
		t.Fatal("a stale form overwrote the entry")
	}

	// Publish: luna-pages refuses (a required field), then accepts.
	cur := pages.entries[eid]["version"].(int)
	pages.publishFail = map[string][]string{"field.specs.1.value": {"The value is required."}}
	form.Set("version", strconv.Itoa(cur))
	c.Post(entry+"/publish", form).AssertUnprocessable().AssertSee("The value is required")
	cur = pages.entries[eid]["version"].(int) // the save went through
	form.Set("version", strconv.Itoa(cur))
	c.Post(entry+"/publish", form).AssertRedirect(entry + "?saved=published")
	if pages.entries[eid]["status"] != "published" {
		t.Fatal("not published")
	}
	c.Get(base).AssertOK().AssertSee("Coffee grinder", "/products/grinder", "published")

	// Preview: the page, under a sandbox; unknown ids say why.
	p := c.Get("/apps/"+id+"/pages/preview?id="+eid).AssertOK().
		AssertHeader("Content-Security-Policy", "sandbox allow-scripts allow-popups allow-forms").AssertSee(`<h1 data-field="title">Grinder</h1>`, "h1{color:red}")
	if strings.Contains(p.Text(), "csrf") {
		t.Fatal("the preview carries the dashboard's CSRF token")
	}
	c.Get("/apps/" + id + "/pages/preview?id=" + tpl).AssertNotFound().AssertSee("Nothing to preview yet")

	// Delete.
	c.Post(entry+"/delete", nil).AssertRedirect(base)
	c.Get(entry).AssertNotFound()
	c.Get(base).AssertOK().AssertSee("No entries yet")
}
