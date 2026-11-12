package controllers

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"fullstack"
	auth "fullstack-passport"
	"fullstack-validation"
	"fullstack/web"

	"luna-dashboard/app/models"
	"luna-dashboard/app/pagesapi"
)

// Pages serves an application's site, kept by luna-pages: the page editor
// (GrapesJS with grapesjs-pages and grapesjs-storage, talking to luna-pages
// through the dashboard), previews, and quick forms for entries.
type Pages struct{ App *fullstack.App }

func (h Pages) client() (*pagesapi.Client, error) { return fullstack.Make[*pagesapi.Client](h.App) }

// current is the developer's application of {app} (404 for anyone else's)
// and the luna-pages client.
func (h Pages) current(c *web.Ctx) (*pagesapi.Client, *models.Application, error) {
	_, a, err := Apps(h).current(c)
	if err != nil {
		return nil, nil, err
	}
	cl, err := h.client()
	return cl, a, err
}

func pagesURL(a *models.Application) string { return "/apps/" + a.PublicID + "/pages" }

func entryURL(a *models.Application, id string) string {
	return pagesURL(a) + "/entries/" + url.PathEscape(id)
}

// unavailable is the page shown when luna-pages cannot be asked.
func (h Pages) unavailable(c *web.Ctx, a *models.Application, err error) error {
	msg := "luna-pages cannot be reached right now. Try again in a moment."
	if errors.Is(err, pagesapi.ErrNotConfigured) {
		msg = "luna-pages is not connected to this dashboard (LUNA_PAGES_URL and LUNA_PAGES_TOKEN)."
	}
	h.App.Log.Error("pages: luna-pages", "error", err)
	return c.HTML(http.StatusBadGateway, "pages/apps/pages/unavailable", page(h.App, c, map[string]any{"Application": a, "Message": msg}))
}

/* ---------------- the editor ---------------- */

// editorScripts are the bundle's scripts in load order: GrapesJS, the
// official plugins, ours (the stand's set, C:\OSPanel6\home\grapesjs,
// without the plugins manager). grapesjs-pages comes before
// grapesjs-storage, which then leaves the templates' designs to it.
var editorScripts = []string{
	"grapes.min.js",
	"grapesjs-preset-webpage.js",
	"grapesjs-blocks-basic.js",
	"grapesjs-plugin-forms.js",
	"grapesjs-navbar.js",
	"grapesjs-tabs.js",
	"grapesjs-component-countdown.js",
	"grapesjs-custom-code.js",
	"grapesjs-tooltip.js",
	"grapesjs-typed.js",
	"grapesjs-parser-postcss.js",
	"grapesjs-style-gradient.js",
	"grapesjs-style-bg.js",
	"grapesjs-style-filter.js",
	"grapesjs-plugin-export.js",
	"grapesjs-tui-image-editor.js",
	"grapesjs-cloud-assets.js",
	"grapesjs-sidebar-tools.js",
	"grapesjs-context-menu.js",
	"grapesjs-export-jsx.js",
	"grapesjs-pages.js",
	"grapesjs-storage.js",
}

// The TOAST UI image editor, which grapesjs-tui-image-editor loads when it
// is first opened (from uicdn.toast.com unless told otherwise).
var (
	tuiScripts = []string{"tui-code-snippet.min.js", "tui-color-picker.min.js", "tui-image-editor.min.js"}
	tuiStyles  = []string{"tui-color-picker.min.css", "tui-image-editor.min.css"}
)

var editorStyles = []string{"grapes.min.css", "grapick.min.css"}

// Editor is the page editor: a whole-window page outside the dashboard's
// layout.
func (h Pages) Editor(c *web.Ctx) error {
	cl, a, err := h.current(c)
	if err != nil {
		return err
	}
	if !cl.Configured() {
		return h.unavailable(c, a, pagesapi.ErrNotConfigured)
	}
	files, err := EditorBundle()
	if err != nil {
		return err
	}
	urls := func(names []string) []string {
		out := make([]string, len(names))
		for i, n := range names {
			out[i] = files[n].URL()
		}
		return out
	}
	cfg, err := json.Marshal(map[string]any{
		"app":      a.PublicID,
		"name":     a.Name,
		"api":      pagesURL(a) + "/api",
		"pages":    pagesURL(a),
		"back":     "/apps/" + a.PublicID,
		"csrf":     auth.CSRFToken(c),
		"grapesjs": files["grapes.min.js"].Version,
		// GrapesJS loads Font Awesome from a CDN unless told where it is.
		"icons": files["font-awesome.min.css"].URL(),
		"tui":   map[string][]string{"script": urls(tuiScripts), "style": urls(tuiStyles)},
		// grapesjs-cloud-assets' sign-in popups come back here.
		"callbacks": map[string]string{
			"dropbox":   "/assets/editor/dropbox-callback.html",
			"microsoft": "/assets/editor/microsoft-callback.html",
		},
	})
	if err != nil {
		return err
	}
	c.W.Header().Set("Cache-Control", "no-store")
	return c.HTML(http.StatusOK, "pages/apps/pages/editor", page(h.App, c, map[string]any{
		"Application": a,
		"Scripts":     urls(editorScripts),
		"Styles":      urls(editorStyles),
		// JSON in a <script type="application/json">: html/template keeps it
		// JSON-escaped (no "</script>" can close the element).
		"Config": template.JS(cfg),
	}))
}

// API forwards one request of the editor to luna-pages' internal API for
// the application: the editor contract (grapesjs-pages), the content API
// (grapesjs-storage) and the read API. The developer's session and CSRF
// token are checked here; luna-pages gets the service token.
func (h Pages) API(c *web.Ctx) error {
	cl, a, err := h.current(c)
	if err != nil {
		return err
	}
	path := c.Param("path")
	if !pagesapi.Allowed(c.R.Method, path) {
		return web.ErrNotFound
	}
	if err := cl.Forward(c.W, c.R, a.PublicID, path); err != nil {
		var tooBig *http.MaxBytesError
		switch {
		case errors.As(err, &tooBig):
			return web.Error(http.StatusRequestEntityTooLarge, "The request body is too large.")
		case c.Written():
			h.App.Log.Error("pages: forwarding", "error", err)
			return nil
		case errors.Is(err, pagesapi.ErrNotConfigured):
			return web.Error(http.StatusBadGateway, "luna-pages is not connected to this dashboard.")
		default:
			h.App.Log.Error("pages: forwarding", "error", err)
			return web.Error(http.StatusBadGateway, "luna-pages cannot be reached right now.")
		}
	}
	return nil
}

// Preview shows a page or an entry as the site would: the published design,
// an entry's draft values. It runs under a sandbox CSP, so a page's own
// scripts (sites that allow custom code) run in an opaque origin, away from
// the dashboard's cookies.
func (h Pages) Preview(c *web.Ctx) error {
	cl, a, err := h.current(c)
	if err != nil {
		return err
	}
	id := strings.TrimSpace(c.Query("id"))
	if id == "" {
		return web.ErrNotFound
	}
	r, err := cl.Render(c.Context(), a.PublicID, id, true)
	if errors.Is(err, pagesapi.ErrNotFound) {
		return c.HTML(http.StatusNotFound, "pages/apps/pages/unpublished", page(h.App, c, map[string]any{"Application": a}))
	}
	if err != nil {
		return h.unavailable(c, a, err)
	}
	hd := c.W.Header()
	hd.Set("Content-Security-Policy", "sandbox allow-scripts allow-popups allow-forms")
	hd.Set("Cache-Control", "no-store")
	return c.HTML(http.StatusOK, "pages/apps/pages/preview", map[string]any{
		"Title": r.Title,
		// luna-pages sanitized both when they were published.
		"HTML": template.HTML(r.HTML),
		"CSS":  template.CSS(r.CSS),
	})
}

/* ---------------- quick forms for entries ---------------- */

type templateEntries struct {
	Template pagesapi.Template
	Entries  []pagesapi.Entry
}

type newEntryForm struct {
	Template string `form:"template" validate:"required"`
	Name     string `form:"name" validate:"required,max=200"`
	Slug     string `form:"slug" validate:"max=255"`
}

// Entries lists the templates and their entries.
func (h Pages) Entries(c *web.Ctx) error { return h.entries(c, http.StatusOK, nil, newEntryForm{}) }

func (h Pages) entries(c *web.Ctx, status int, errs validation.Errors, in newEntryForm) error {
	cl, a, err := h.current(c)
	if err != nil {
		return err
	}
	tpls, err := cl.Templates(c.Context(), a.PublicID)
	if err != nil {
		return h.unavailable(c, a, err)
	}
	all, err := cl.Entries(c.Context(), a.PublicID, "")
	if err != nil {
		return h.unavailable(c, a, err)
	}
	by := map[string][]pagesapi.Entry{}
	for _, e := range all {
		by[e.TemplateID] = append(by[e.TemplateID], e)
	}
	sort.SliceStable(tpls, func(i, j int) bool { return strings.ToLower(tpls[i].Name) < strings.ToLower(tpls[j].Name) })
	groups := make([]templateEntries, 0, len(tpls))
	for _, t := range tpls {
		groups = append(groups, templateEntries{Template: t, Entries: by[t.ID]})
	}
	return c.HTML(status, "pages/apps/pages/entries", page(h.App, c, map[string]any{
		"Application": a, "Groups": groups, "Errors": errs, "Form": in, "URL": pagesURL(a),
	}))
}

// CreateEntry makes an entry of a template and opens its form.
func (h Pages) CreateEntry(c *web.Ctx) error {
	cl, a, err := h.current(c)
	if err != nil {
		return err
	}
	var in newEntryForm
	if err := validation.Bind(c, &in); err != nil {
		if errs, ok := validation.ErrorsOf(err); ok {
			return h.entries(c, http.StatusUnprocessableEntity, errs, in)
		}
		return err
	}
	t, err := cl.Template(c.Context(), a.PublicID, in.Template)
	if errors.Is(err, pagesapi.ErrNotFound) {
		return h.entries(c, http.StatusUnprocessableEntity, validation.Errors{"template": {"No such template."}}, in)
	}
	if err != nil {
		return h.unavailable(c, a, err)
	}
	values := map[string]json.RawMessage{}
	for _, f := range t.Fields {
		if len(f.Default) > 0 {
			values[f.Name] = f.Default
		}
	}
	w := pagesapi.EntryWrite{TemplateID: t.ID, Name: strings.TrimSpace(in.Name), Values: values}
	if s := strings.TrimSpace(in.Slug); s != "" {
		w.Slug = &s
	}
	e, err := cl.PutEntry(c.Context(), a.PublicID, newID(), w, 0)
	if errs, ok := pagesErrors(err); ok {
		return h.entries(c, http.StatusUnprocessableEntity, errs, in)
	}
	if err != nil {
		return h.unavailable(c, a, err)
	}
	return c.Redirect(http.StatusSeeOther, entryURL(a, e.ID))
}

// Entry is an entry's form.
func (h Pages) Entry(c *web.Ctx) error {
	cl, a, err := h.current(c)
	if err != nil {
		return err
	}
	e, t, err := h.load(c, cl, a)
	if err != nil {
		return err
	}
	return h.entry(c, a, http.StatusOK, t, e, e.Values, nil, "")
}

func (h Pages) load(c *web.Ctx, cl *pagesapi.Client, a *models.Application) (pagesapi.Entry, pagesapi.Template, error) {
	e, err := cl.Entry(c.Context(), a.PublicID, c.Param("entry"))
	if errors.Is(err, pagesapi.ErrNotFound) {
		return e, pagesapi.Template{}, web.ErrNotFound
	}
	if err != nil {
		return e, pagesapi.Template{}, h.unavailable(c, a, err)
	}
	t, err := cl.Template(c.Context(), a.PublicID, e.TemplateID)
	if err != nil && !errors.Is(err, pagesapi.ErrNotFound) {
		return e, t, h.unavailable(c, a, err)
	}
	return e, t, nil
}

func (h Pages) entry(c *web.Ctx, a *models.Application, status int, t pagesapi.Template, e pagesapi.Entry,
	values map[string]json.RawMessage, errs validation.Errors, notice string) error {
	return c.HTML(status, "pages/apps/pages/entry", page(h.App, c, map[string]any{
		"Application": a,
		"Template":    t,
		"Entry":       e,
		"Inputs":      fieldInputs(t.Fields, values, errs, "v."),
		"Errors":      errs,
		"Notice":      notice,
		"URL":         entryURL(a, e.ID),
		"PagesURL":    pagesURL(a),
	}))
}

// SaveEntry saves the form as the entry's draft.
func (h Pages) SaveEntry(c *web.Ctx) error { return h.saveEntry(c, false) }

// PublishEntry saves the form and publishes the entry.
func (h Pages) PublishEntry(c *web.Ctx) error { return h.saveEntry(c, true) }

func (h Pages) saveEntry(c *web.Ctx, publish bool) error {
	cl, a, err := h.current(c)
	if err != nil {
		return err
	}
	e, t, err := h.load(c, cl, a)
	if err != nil {
		return err
	}
	if err := c.R.ParseForm(); err != nil {
		return web.Error(http.StatusBadRequest, "The form could not be read.")
	}
	form := c.R.PostForm
	values, errs := formValues(t.Fields, form, "v.")
	name := strings.TrimSpace(form.Get("name"))
	slug := strings.TrimSpace(form.Get("slug"))
	if name == "" {
		errs = add(errs, "name", "Give the entry a name.")
	}
	version, _ := strconv.ParseInt(form.Get("version"), 10, 64)
	shown := e
	shown.Name, shown.Slug = name, slug
	if len(errs) > 0 {
		return h.entry(c, a, http.StatusUnprocessableEntity, t, shown, values, errs, "")
	}
	saved, err := cl.PutEntry(c.Context(), a.PublicID, e.ID, pagesapi.EntryWrite{Name: name, Slug: &slug, Values: values}, version)
	if err == nil && publish {
		_, err = cl.PublishEntry(c.Context(), a.PublicID, e.ID, saved.Version)
	}
	switch {
	case errors.Is(err, pagesapi.ErrVersion):
		return h.entry(c, a, http.StatusConflict, t, shown, values, nil,
			"Someone changed this entry since you opened it (in the editor or another tab). Reload the page to see their version; your changes below are not saved.")
	case err != nil:
		if errs, ok := pagesErrors(err); ok {
			if saved.ID != "" { // saved, but not published
				shown.Version, shown.Status = saved.Version, saved.Status
			}
			return h.entry(c, a, http.StatusUnprocessableEntity, t, shown, values, errs, "")
		}
		return h.unavailable(c, a, err)
	}
	return c.Redirect(http.StatusSeeOther, entryURL(a, e.ID)+"?saved="+map[bool]string{false: "draft", true: "published"}[publish])
}

// DeleteEntry deletes an entry.
func (h Pages) DeleteEntry(c *web.Ctx) error {
	cl, a, err := h.current(c)
	if err != nil {
		return err
	}
	e, _, err := h.load(c, cl, a)
	if err != nil {
		return err
	}
	if err := cl.DeleteEntry(c.Context(), a.PublicID, e.ID, e.Version); err != nil && !errors.Is(err, pagesapi.ErrNotFound) {
		return h.unavailable(c, a, err)
	}
	return c.Redirect(http.StatusSeeOther, pagesURL(a)+"/entries")
}

// pagesErrors turns luna-pages' 422 into form errors: "field.price" is the
// field "price", "field.specs.0.name" the list item's field.
func pagesErrors(err error) (validation.Errors, bool) {
	var e *pagesapi.Error
	if !errors.As(err, &e) || e.Status != http.StatusUnprocessableEntity {
		return nil, false
	}
	out := validation.Errors{}
	for k, msgs := range e.Fields {
		out[strings.TrimPrefix(k, "field.")] = msgs
	}
	if len(out) == 0 {
		out["name"] = []string{e.Message}
	}
	return out, true
}

func add(errs validation.Errors, key, msg string) validation.Errors {
	if errs == nil {
		errs = validation.Errors{}
	}
	errs[key] = append(errs[key], msg)
	return errs
}

func newID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
