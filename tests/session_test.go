package tests

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	"fullstack"
	apptest "fullstack-testing"
	"passport/httpauth"

	"luna-dashboard/app/sessions"
)

func cookieOf(r *apptest.Response, name string) *http.Cookie {
	for _, c := range r.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// login signs in through the form, with or without "Remember me".
func login(t *testing.T, c *apptest.Client, email string, remember bool) *apptest.Response {
	t.Helper()
	c.Get("/login")
	form := url.Values{"email": {email}, "password": {"secret-password"}}
	if remember {
		form.Set("remember", "1")
	}
	return c.Post("/login", form).AssertRedirect("/dashboard")
}

// expire drops the session cookie, as the browser does when it runs out.
func expire(c *apptest.Client) { c.SetCookie(httpauth.CookieName, "") }

func TestSessionsAndRefreshTokens(t *testing.T) {
	t.Setenv("AUTH_SESSION_TTL", "30m")
	app := newApp(t)
	keeper, err := fullstack.Make[*sessions.Keeper](app)
	if err != nil {
		t.Fatal(err)
	}
	developer(t, apptest.New(t, app), "ann@example.com").Post("/logout", nil)

	t.Run("a login is a short session and a browser-session refresh token", func(t *testing.T) {
		c := apptest.New(t, app)
		c.Get("/login").AssertSee(`name="remember"`, "Remember me")
		r := login(t, c, "ann@example.com", false)
		if s := cookieOf(r, httpauth.CookieName); s == nil || s.MaxAge != 29*60 {
			t.Fatalf("session cookie %+v", s)
		}
		rf := cookieOf(r, sessions.RefreshCookie)
		if rf == nil || rf.MaxAge != 0 || !rf.HttpOnly || !rf.Secure || rf.Path != "/" {
			t.Fatalf("refresh cookie %+v", rf)
		}
		if cookieOf(r, sessions.RememberCookie) != nil {
			t.Fatal("remember cookie without Remember me")
		}

		// The session runs out while the developer works: the next request
		// gets a new session and a new refresh token, as if nothing happened.
		expire(c)
		old := c.Cookie(sessions.RefreshCookie)
		r = c.Get("/apps").AssertOK()
		if cookieOf(r, httpauth.CookieName) == nil || c.Cookie(sessions.RefreshCookie) == old {
			t.Fatal("not renewed")
		}
		c.Get("/apps").AssertOK() // the new session works

		// The old token is spent: presented again (a copy, after the grace
		// minute) it ends the whole login.
		keeper.Grace = 0
		defer func() { keeper.Grace = time.Minute }()
		thief := apptest.New(t, app)
		thief.SetCookie(sessions.RefreshCookie, old)
		thief.Get("/apps").AssertRedirect("/login?redirect=%2Fapps")
		expire(c)
		c.Get("/apps").AssertRedirect("/login?redirect=%2Fapps") // the family is revoked
	})

	t.Run("requests at once with the same token share one renewal", func(t *testing.T) {
		c := apptest.New(t, app)
		login(t, c, "ann@example.com", false)
		expire(c)
		token := c.Cookie(sessions.RefreshCookie)
		other := apptest.New(t, app) // a second tab's request, sent before the first came back
		other.SetCookie(sessions.RefreshCookie, token)
		c.Get("/apps").AssertOK()
		other.Get("/apps").AssertOK()
		if c.Cookie(httpauth.CookieName) != other.Cookie(httpauth.CookieName) {
			t.Fatal("two sessions for one renewal")
		}
	})

	t.Run("Remember me keeps the login for 30 days, across browser restarts", func(t *testing.T) {
		c := apptest.New(t, app)
		r := login(t, c, "ann@example.com", true)
		rm := cookieOf(r, sessions.RememberCookie)
		if rm == nil || rm.MaxAge != 30*24*3600 || cookieOf(r, sessions.RefreshCookie) != nil {
			t.Fatalf("remember cookie %+v", rm)
		}
		// A restarted browser: only the remember cookie is left.
		next := apptest.New(t, app)
		next.SetCookie(sessions.RememberCookie, c.Cookie(sessions.RememberCookie))
		r = next.Get("/apps").AssertOK()
		if rm := cookieOf(r, sessions.RememberCookie); rm == nil || rm.MaxAge != 30*24*3600 {
			t.Fatalf("renewed remember cookie %+v", rm)
		}
	})

	t.Run("logging out ends the refresh token too", func(t *testing.T) {
		c := apptest.New(t, app)
		login(t, c, "ann@example.com", true)
		token := c.Cookie(sessions.RememberCookie)
		r := c.Post("/logout", nil).AssertRedirect("/")
		if rm := cookieOf(r, sessions.RememberCookie); rm == nil || rm.MaxAge >= 0 {
			t.Fatal("remember cookie not cleared")
		}
		again := apptest.New(t, app)
		again.SetCookie(sessions.RememberCookie, token)
		again.Get("/apps").AssertRedirect("/login?redirect=%2Fapps")
	})

	t.Run("a garbage token is dropped", func(t *testing.T) {
		c := apptest.New(t, app)
		c.SetCookie(sessions.RefreshCookie, "nope")
		r := c.Get("/apps").AssertRedirect("/login?redirect=%2Fapps")
		if rf := cookieOf(r, sessions.RefreshCookie); rf == nil || rf.MaxAge >= 0 {
			t.Fatal("bad refresh cookie kept")
		}
	})
}

func TestRefreshTokenRunsOutWhenAway(t *testing.T) {
	t.Setenv("DASHBOARD_REFRESH_TTL", "1s")
	app := newApp(t)
	c := developer(t, apptest.New(t, app), "ann@example.com")
	expire(c)
	time.Sleep(1100 * time.Millisecond)
	c.Get("/apps").AssertRedirect("/login?redirect=%2Fapps")
}
