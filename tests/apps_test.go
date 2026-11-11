package tests

import (
	"net/url"
	"regexp"
	"strings"
	"testing"

	apptest "fullstack-testing"
	"luna-keys"
)

var secretRe = regexp.MustCompile(`(sk|pk)_live_[A-Za-z0-9_-]{43}`)

// developer registers a developer and returns a logged-in client.
func developer(t *testing.T, c *apptest.Client, email string) *apptest.Client {
	t.Helper()
	c.Get("/register")
	c.Post("/register", url.Values{"name": {"Dev"}, "email": {email}, "password": {"secret-password"}, "password_confirmation": {"secret-password"}}).
		AssertRedirect("/dashboard")
	return c
}

// createApp creates an application and returns its public id.
func createApp(t *testing.T, c *apptest.Client, name string) string {
	t.Helper()
	r := c.Post("/apps", url.Values{"name": {name}}).AssertRedirect()
	loc := r.Header().Get("Location")
	id, ok := strings.CutPrefix(loc, "/apps/")
	if !ok || len(id) != 36 {
		t.Fatalf("redirect to %q", loc)
	}
	return id
}

// createKey creates a key and returns its secret, read from the page.
func createKey(t *testing.T, c *apptest.Client, app string, form url.Values) string {
	t.Helper()
	r := c.Post("/apps/"+app+"/keys", form).AssertOK().AssertHeader("Cache-Control", "no-store")
	secret := secretRe.FindString(r.Text())
	if secret == "" {
		t.Fatalf("no key on the page: %s", r.Text())
	}
	return secret
}

// introspect asks the dashboard about a key, as luna-auth does.
func introspect(c *apptest.Client, key string) *apptest.Response {
	return c.WithToken(serviceToken).PostJSON(keys.IntrospectPath, map[string]any{"hash": keys.Hash(key)})
}

func TestApplicationsAndKeys(t *testing.T) {
	app := newApp(t)
	c := developer(t, apptest.New(t, app), "ann@example.com")

	c.Post("/apps", url.Values{"name": {""}}).AssertUnprocessable().AssertSee("name field is required")
	c.Post("/apps", url.Values{"name": {"Shop"}, "password_reset_url": {"ftp://x"}}).AssertUnprocessable()
	id := createApp(t, c, "Shop")
	c.Get("/apps").AssertOK().AssertSee("Shop", id)
	c.Get("/apps/"+id).AssertOK().AssertSee("Shop", "No keys yet")
	c.Get("/apps/"+id+"/keys/new").AssertOK().AssertSee("auth:login", "pages:write")

	// a server key
	server := createKey(t, c, id, url.Values{"name": {"backend"}, "kind": {"server"},
		"scopes": {"auth:login", "auth:users.read"}, "ips": {"203.0.113.0/24\n2001:db8::1"}})
	if !strings.HasPrefix(server, "sk_live_") {
		t.Fatalf("server key %q", server)
	}
	introspect(c, server).AssertOK().
		AssertJSONPath("active", true).AssertJSONPath("app_id", id).AssertJSONPath("app_name", "Shop").
		AssertJSONPath("kind", "server").AssertJSONPath("scopes", []string{"auth:login", "auth:users.read"}).
		AssertJSONPath("ips", []string{"203.0.113.0/24", "2001:db8::1/128"})
	c.Get("/apps/"+id).AssertOK().AssertSee("backend", keys.Hint(server)).AssertDontSee(server)

	// browser keys: origins required, auth:login and pages:read only
	c.Post("/apps/"+id+"/keys", url.Values{"kind": {"browser"}, "scopes": {"auth:login"}}).
		AssertUnprocessable().AssertSee("at least one allowed origin")
	c.Post("/apps/"+id+"/keys", url.Values{"kind": {"browser"}, "scopes": {"auth:users.read"}, "origins": {"https://shop.example"}}).
		AssertUnprocessable().AssertSee("only hold auth:login")
	c.Post("/apps/"+id+"/keys", url.Values{"kind": {"browser"}, "scopes": {"auth:login"}, "origins": {"https://shop.example/cart"}}).
		AssertUnprocessable().AssertSee("is not an origin")
	c.Post("/apps/"+id+"/keys", url.Values{"kind": {"server"}}).AssertUnprocessable().AssertSee("at least one scope")
	browser := createKey(t, c, id, url.Values{"kind": {"browser"}, "scopes": {"auth:login"}, "origins": {"https://Shop.example/\nhttp://localhost:3000"}})
	introspect(c, browser).AssertOK().AssertJSONPath("kind", "browser").
		AssertJSONPath("origins", []string{"https://shop.example", "http://localhost:3000"})

	// another developer sees none of it
	other := developer(t, apptest.New(t, app), "bob@example.com")
	other.Get("/apps/" + id).AssertNotFound()
	other.Get("/apps").AssertOK().AssertDontSee("Shop")
	other.Post("/apps/"+id+"/delete", nil).AssertNotFound()

	// a new secret replaces the old one
	keyID := keyIDOf(t, c, id, keys.Hint(server))
	r := c.Post("/apps/"+id+"/keys/"+keyID+"/regenerate", nil).AssertOK()
	renewed := secretRe.FindString(r.Text())
	if renewed == "" || renewed == server {
		t.Fatalf("regenerated key %q", renewed)
	}
	introspect(c, server).AssertOK().AssertJSONPath("active", false)
	introspect(c, renewed).AssertOK().AssertJSONPath("active", true)

	// revoking stops it
	c.Post("/apps/"+id+"/keys/"+keyID+"/revoke", nil).AssertRedirect("/apps/" + id)
	introspect(c, renewed).AssertOK().AssertJSONPath("active", false)
	c.Get("/apps/" + id).AssertOK().AssertSee("revoked")
	c.Post("/apps/"+id+"/keys/"+keyID+"/regenerate", nil).AssertNotFound()

	// settings and deletion
	c.Post("/apps/"+id, url.Values{"name": {"Shop 2"}, "password_reset_url": {"https://shop.example/reset"}}).AssertRedirect()
	c.Get("/apps/"+id).AssertOK().AssertSee("Shop 2", "https://shop.example/reset")
	c.Post("/apps/"+id+"/delete", nil).AssertRedirect("/apps")
	introspect(c, browser).AssertOK().AssertJSONPath("active", false)
	c.Get("/apps/" + id).AssertNotFound()
}

func TestAppsNeedLogin(t *testing.T) {
	c := apptest.New(t, newApp(t))
	c.Get("/apps").AssertRedirect("/login?redirect=%2Fapps")
	c.Get("/apps/not-a-uuid").AssertRedirect()
}

// keyIDOf finds a key's public id in the application page by its hint.
func keyIDOf(t *testing.T, c *apptest.Client, app, hint string) string {
	t.Helper()
	page := c.Get("/apps/" + app).AssertOK().Text()
	re := regexp.MustCompile(`/apps/` + app + `/keys/([0-9a-f-]{36})/revoke`)
	i := strings.Index(page, hint)
	if i < 0 {
		t.Fatalf("hint %q not on the page", hint)
	}
	m := re.FindStringSubmatch(page[i:])
	if m == nil {
		t.Fatal("no revoke form after the key")
	}
	return m[1]
}
