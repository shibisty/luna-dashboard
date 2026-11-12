package sessions

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"fullstack"
	database "fullstack-orm"
	auth "fullstack-passport"
	"fullstack/env"
	"fullstack/web"
	"passport"
	sqlstore "passport-session-sql"
	"passport/httpauth"
	"passport/refresh"
)

// A developer's login is a short session plus a refresh token that starts
// the next session when the last one ends, so whoever keeps working stays
// signed in:
//
//	session        AUTH_SESSION_TTL (30m): the session_id cookie
//	refresh token  luna_refresh, a browser-session cookie: the login lasts while
//	               the browser is open and the developer is not away for longer
//	               than DASHBOARD_REFRESH_TTL (2h)
//	"Remember me"  luna_remember, a cookie for DASHBOARD_REMEMBER_TTL (30 days),
//	               renewed by every new session: away for less than that, still
//	               signed in, after a browser restart too
//
// Refresh tokens rotate (each is good once; passport/refresh): a token used
// twice ends the whole login, as a stolen one would.

// Config sets the lifetimes.
type Config struct {
	RefreshTTL  time.Duration `env:"DASHBOARD_REFRESH_TTL" default:"2h"`
	RememberTTL time.Duration `env:"DASHBOARD_REMEMBER_TTL" default:"720h"`
}

// Cookie names.
const (
	RefreshCookie  = "luna_refresh"
	RememberCookie = "luna_remember"
)

// Keeper starts and renews logins.
type Keeper struct {
	App  *fullstack.App
	Auth *auth.Auth
	Cfg  Config
	// Grace is how long a rotated token still answers with the session it
	// started (a minute): an editor sends several requests at once, all
	// with the old token, and only one may rotate it. Within one process:
	// with several dashboard replicas, keep the sessions sticky or accept
	// that a rare race signs the developer out.
	Grace time.Duration

	mu     sync.Mutex
	recent map[string]rotated // by the hash of the spent token
}

type rotated struct {
	session string
	token   string
	at      time.Time
}

// NewKeeper reads the configuration.
func NewKeeper(app *fullstack.App, au *auth.Auth) (*Keeper, error) {
	var cfg Config
	if err := env.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("sessions: %w", err)
	}
	return &Keeper{App: app, Auth: au, Cfg: cfg, Grace: time.Minute, recent: map[string]rotated{}}, nil
}

func (k *Keeper) refresher(remember bool) (*refresh.Refresher, error) {
	exec, err := database.Exec(k.App)
	if err != nil {
		return nil, err
	}
	ttl := k.Cfg.RefreshTTL
	if remember {
		ttl = k.Cfg.RememberTTL
	}
	return refresh.New(sqlstore.RefreshTokens(exec, ""), ttl), nil
}

// sessionIssuer makes passport/refresh's "access token" a session.
type sessionIssuer struct{ au *auth.Auth }

func (s sessionIssuer) Issue(ctx context.Context, id *passport.Identity, ttl time.Duration) (string, error) {
	return s.au.StartSession(ctx, id, ttl)
}

// Login signs the developer in: a session and a refresh token (kept across
// browser restarts with remember).
func (k *Keeper) Login(c *web.Ctx, identity *passport.Identity, remember bool) error {
	if err := k.revoke(c); err != nil { // a login the browser had before
		return err
	}
	if old, err := c.R.Cookie(httpauth.CookieName); err == nil && old.Value != "" {
		_ = k.Auth.EndSession(c.Context(), old.Value)
	}
	r, err := k.refresher(remember)
	if err != nil {
		return err
	}
	pair, err := r.Issue(c.Context(), sessionIssuer{k.Auth}, identity, k.Auth.Config.SessionTTL)
	if err != nil {
		return err
	}
	k.setCookies(c.W, pair.Access, pair.Refresh, remember)
	return nil
}

// Logout ends the session and the refresh tokens of this login.
func (k *Keeper) Logout(c *web.Ctx) error {
	err := k.revoke(c)
	clear := func(name string) {
		http.SetCookie(c.W, &http.Cookie{Name: name, Value: "", Path: "/", HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode, MaxAge: -1})
	}
	clear(RefreshCookie)
	clear(RememberCookie)
	if lerr := k.Auth.Logout(c); err == nil {
		err = lerr
	}
	return err
}

func (k *Keeper) revoke(c *web.Ctx) error {
	for _, name := range []string{RefreshCookie, RememberCookie} {
		ck, err := c.R.Cookie(name)
		if err != nil || ck.Value == "" {
			continue
		}
		r, err := k.refresher(name == RememberCookie)
		if err != nil {
			return err
		}
		if err := r.Revoke(c.Context(), ck.Value); err != nil {
			return err
		}
	}
	return nil
}

// setCookies writes the session cookie, a minute shorter than the session
// (the browser stops sending it before the server would refuse it, and
// Renew starts the next one), and the refresh cookie.
func (k *Keeper) setCookies(w http.ResponseWriter, session, token string, remember bool) {
	age := int((k.Auth.Config.SessionTTL - time.Minute) / time.Second)
	if age < 60 {
		age = int(k.Auth.Config.SessionTTL / time.Second)
	}
	httpauth.SetSessionCookie(w, session, age)
	c := &http.Cookie{Name: RefreshCookie, Value: token, Path: "/", HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode}
	if remember {
		c.Name = RememberCookie
		c.MaxAge = int(k.Cfg.RememberTTL / time.Second)
	}
	http.SetCookie(w, c)
}

// Renew goes before auth.Web: a request without a session but with a
// refresh token gets a new session (and token), as if it had come with
// them. A token that is unknown, expired or reused is dropped and the
// request goes on signed out.
func (k *Keeper) Renew() web.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if ck, err := r.Cookie(httpauth.CookieName); err == nil && ck.Value != "" {
				next.ServeHTTP(w, r)
				return
			}
			name, token := "", ""
			for _, n := range []string{RememberCookie, RefreshCookie} {
				if ck, err := r.Cookie(n); err == nil && ck.Value != "" {
					name, token = n, ck.Value
					break
				}
			}
			if token == "" {
				next.ServeHTTP(w, r)
				return
			}
			remember := name == RememberCookie
			session, fresh, err := k.rotate(r.Context(), token, remember)
			switch {
			case err == nil:
				k.setCookies(w, session, fresh, remember)
				r = withSession(r, session)
			case errors.Is(err, refresh.ErrInvalid) || errors.Is(err, refresh.ErrReused):
				http.SetCookie(w, &http.Cookie{Name: name, Value: "", Path: "/", HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode, MaxAge: -1})
			default:
				k.App.Log.Error("sessions: renewing a login", "error", err)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// rotate spends token for a new session and token; the same token within
// the grace minute gets the same answer.
func (k *Keeper) rotate(ctx context.Context, token string, remember bool) (string, string, error) {
	key := hash(token)
	k.mu.Lock()
	defer k.mu.Unlock()
	now := time.Now()
	for h, r := range k.recent {
		if now.Sub(r.at) > k.Grace {
			delete(k.recent, h)
		}
	}
	if r, ok := k.recent[key]; ok && now.Sub(r.at) <= k.Grace {
		return r.session, r.token, nil
	}
	rf, err := k.refresher(remember)
	if err != nil {
		return "", "", err
	}
	pair, err := rf.Rotate(ctx, sessionIssuer{k.Auth}, token, k.Auth.Config.SessionTTL)
	if err != nil {
		return "", "", err
	}
	k.recent[key] = rotated{session: pair.Access, token: pair.Refresh, at: now}
	return pair.Access, pair.Refresh, nil
}

// withSession puts the new session cookie into the request, for auth.Web.
func withSession(r *http.Request, session string) *http.Request {
	r = r.Clone(r.Context())
	var kept []string
	for _, c := range r.Cookies() {
		if c.Name != httpauth.CookieName {
			kept = append(kept, c.Name+"="+c.Value)
		}
	}
	kept = append(kept, httpauth.CookieName+"="+session)
	r.Header.Set("Cookie", strings.Join(kept, "; "))
	return r
}

func hash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// SweepRefresh deletes expired refresh tokens.
func (s Store) SweepRefresh(ctx context.Context) (int64, error) {
	exec, err := database.Exec(s.App)
	if err != nil {
		return 0, err
	}
	return sqlstore.RefreshTokens(exec, "").Sweep(ctx)
}
