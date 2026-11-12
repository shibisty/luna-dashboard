// Package controllers holds the HTTP handlers.
package controllers

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"fullstack"
	auth "fullstack-passport"
	"fullstack-validation"
	"fullstack/web"

	"luna-dashboard/app/locale"
)

// page is the data every page gets: the app name, the developer (User, and
// Me, which a page's own User cannot hide), the CSRF field, the language
// (L.T translates) and the theme. Errors, DataErrors, Message and Notice
// come from the code in English and are translated here.
func page(app *fullstack.App, c *web.Ctx, extra map[string]any) map[string]any {
	l := locale.Detect(c.R)
	data := map[string]any{
		"App": app.Config.Name, "CSRFField": auth.CSRFField(c),
		"L": l, "Lang": l, "Langs": locale.All, "Theme": locale.Theme(c.R),
		"Back": backURL(c.R), "InApps": strings.HasPrefix(c.R.URL.Path, "/apps"),
	}
	if u, ok := auth.User(c); ok {
		data["User"] = u
		data["Me"] = u
	}
	for k, v := range extra {
		switch v := v.(type) {
		case validation.Errors:
			data[k] = l.Errors(v)
		case string:
			if k == "Message" || k == "Notice" {
				data[k] = l.Msg(v)
			} else {
				data[k] = v
			}
		default:
			data[k] = v
		}
	}
	return data
}

// backURL is where /prefs returns to: this page, or for a form that was
// posted, the page the form was on.
func backURL(r *http.Request) string {
	if r.Method == http.MethodGet {
		return r.URL.RequestURI()
	}
	if ref, err := url.Parse(r.Referer()); err == nil && ref.Host == r.Host && localPath(ref.RequestURI()) {
		return ref.RequestURI()
	}
	return "/"
}

// localPath reports whether p is a path on this site ("/apps"), not
// another one ("//evil.example", "/\\evil.example", "https://…").
func localPath(p string) bool {
	return strings.HasPrefix(p, "/") && !strings.HasPrefix(p, "//") && !strings.HasPrefix(p, "/\\") && !strings.ContainsAny(p, "\r\n")
}

// Prefs keeps the dashboard's language (?lang=en|ru) and theme
// (?theme=light|dark|system) in cookies for a year and goes back to the
// page (?back=, a local path).
func Prefs(c *web.Ctx) error {
	set := func(name, value string) {
		http.SetCookie(c.W, &http.Cookie{Name: name, Value: value, Path: "/", MaxAge: int(locale.CookieAge.Seconds()),
			HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode})
	}
	if v := c.Query("lang"); locale.Supported(v) {
		set(locale.LangCookie, v)
	}
	if v := c.Query("theme"); locale.ValidTheme(v) {
		set(locale.ThemeCookie, v)
	}
	back := c.Query("back")
	if !localPath(back) {
		back = "/"
	}
	return c.Redirect(http.StatusSeeOther, back)
}

// Home serves the public pages.
type Home struct{ App *fullstack.App }

// Index is the home page.
func (h Home) Index(c *web.Ctx) error { return c.HTML(200, "pages/home", page(h.App, c, nil)) }

// Dashboard is where logged-in developers land: their applications.
func (h Home) Dashboard(c *web.Ctx) error { return c.Redirect(0, "/apps") }

// Errors answers a browser's page request that failed with an error page
// in the dashboard's look and language; JSON clients (the editor's API
// calls) and the rest get web.DefaultErrorHandler.
func Errors(app *fullstack.App) web.ErrorHandler {
	return func(c *web.Ctx, err error) {
		if c.Written() || c.WantsJSON() || !strings.Contains(c.R.Header.Get("Accept"), "text/html") {
			web.DefaultErrorHandler(c, err)
			return
		}
		status := web.StatusOf(err)
		msg := http.StatusText(status)
		var he *web.HTTPError
		if errors.As(err, &he) && he.Message != "" && status < 500 {
			msg = he.Message
		}
		if status >= 500 {
			app.Log.Error("request failed", "status", status, "path", c.R.URL.Path, "error", err)
		}
		if c.HTML(status, "pages/error", page(app, c, map[string]any{"Status": status, "Message": msg})) != nil {
			web.DefaultErrorHandler(c, err)
		}
	}
}
