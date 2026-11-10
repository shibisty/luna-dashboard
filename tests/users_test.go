package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	apptest "fullstack-testing"
)

const authToken = "auth-internal-token"

// fakeAuth is luna-auth's internal API over memory: users per application
// and their data.
type fakeAuth struct {
	mu    sync.Mutex
	users map[string][]*fakeUser // app id → users
	next  int
	apps  []string // the application of every request, in order
}

type fakeUser struct {
	ID            string            `json:"id"`
	Username      *string           `json:"username"`
	Email         string            `json:"email"`
	EmailVerified bool              `json:"email_verified"`
	CreatedAt     string            `json:"created_at"`
	UpdatedAt     string            `json:"updated_at"`
	data          map[string]string `json:"-"`
}

func newFakeAuth(t *testing.T) *fakeAuth {
	f := &fakeAuth{users: map[string][]*fakeUser{}}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	t.Setenv("LUNA_AUTH_URL", srv.URL)
	t.Setenv("LUNA_AUTH_TOKEN", authToken)
	return f
}

func reply(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

func (f *fakeAuth) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.Header.Get("Authorization") != "Bearer "+authToken {
		reply(w, 401, map[string]string{"error": "missing or invalid service token"})
		return
	}
	rest, ok := strings.CutPrefix(r.URL.Path, "/internal/v1/apps/")
	parts := strings.Split(rest, "/")
	if !ok || len(parts) < 2 || parts[1] != "users" {
		reply(w, 404, map[string]string{"error": "not found"})
		return
	}
	app := parts[0]
	f.apps = append(f.apps, app)
	var body map[string]any
	_ = json.NewDecoder(r.Body).Decode(&body)
	find := func(id string) *fakeUser {
		for _, u := range f.users[app] {
			if u.ID == id {
				return u
			}
		}
		return nil
	}
	switch {
	case len(parts) == 2 && r.Method == "GET":
		q := r.URL.Query().Get("q")
		list := []*fakeUser{}
		for _, u := range f.users[app] {
			if q == "" || strings.Contains(u.Email, q) {
				list = append(list, u)
			}
		}
		reply(w, 200, map[string]any{"data": list, "page": 1, "per_page": 25, "total": len(list), "last_page": 1})
	case len(parts) == 2 && r.Method == "POST":
		email, _ := body["email"].(string)
		for _, u := range f.users[app] {
			if u.Email == email {
				reply(w, 422, map[string]any{"error": "The given data was invalid.", "errors": map[string][]string{"email": {"The email has already been taken."}}})
				return
			}
		}
		f.next++
		u := &fakeUser{ID: fmt.Sprintf("00000000-0000-4000-8000-%012d", f.next), Email: email, CreatedAt: "2026-11-11T10:00:00Z", UpdatedAt: "2026-11-11T10:00:00Z", data: map[string]string{}}
		if s, _ := body["username"].(string); s != "" {
			u.Username = &s
		}
		u.EmailVerified, _ = body["email_verified"].(bool)
		f.users[app] = append(f.users[app], u)
		reply(w, 201, u)
	default:
		u := find(parts[2])
		if u == nil {
			reply(w, 404, map[string]string{"error": "Not Found"})
			return
		}
		switch {
		case len(parts) == 3 && r.Method == "GET":
			reply(w, 200, u)
		case len(parts) == 3 && r.Method == "PATCH":
			if s, ok := body["username"].(string); ok {
				if s == "bad name" {
					reply(w, 422, map[string]any{"error": "invalid", "errors": map[string][]string{"username": {"The username may contain only letters."}}})
					return
				}
				u.Username = &s
			}
			if s, ok := body["email"].(string); ok {
				u.Email = s
			}
			if b, ok := body["email_verified"].(bool); ok {
				u.EmailVerified = b
			}
			reply(w, 200, u)
		case len(parts) == 3 && r.Method == "DELETE":
			list := f.users[app][:0]
			for _, x := range f.users[app] {
				if x != u {
					list = append(list, x)
				}
			}
			f.users[app] = list
			reply(w, 204, nil)
		case len(parts) == 4 && r.Method == "GET":
			reply(w, 200, map[string]any{"data": u.data})
		case len(parts) == 5 && r.Method == "PUT":
			if strings.Contains(parts[4], " ") {
				reply(w, 422, map[string]any{"error": "invalid", "errors": map[string][]string{"key": {"The key must be letters and digits."}}})
				return
			}
			u.data[parts[4]], _ = body["value"].(string)
			reply(w, 200, map[string]any{"key": parts[4], "value": u.data[parts[4]]})
		case len(parts) == 5 && r.Method == "DELETE":
			if _, ok := u.data[parts[4]]; !ok {
				reply(w, 404, map[string]string{"error": "no such key"})
				return
			}
			delete(u.data, parts[4])
			reply(w, 204, nil)
		default:
			reply(w, 405, map[string]string{"error": "method"})
		}
	}
}

func TestUsersAndData(t *testing.T) {
	auth := newFakeAuth(t)
	app := newApp(t)
	c := developer(t, apptest.New(t, app), "ann@example.com")
	id := createApp(t, c, "Shop")
	base := "/apps/" + id + "/users"

	c.Get("/apps/"+id).AssertOK().AssertSee(base)
	c.Get(base).AssertOK().AssertSee("No users yet", "Add a user")

	c.Post(base, url.Values{"email": {"not-an-email"}, "password": {"secret-password"}}).AssertUnprocessable()
	r := c.Post(base, url.Values{"email": {"bob@example.com"}, "username": {"bob"}, "password": {"secret-password"}, "email_verified": {"1"}}).AssertRedirect()
	user := strings.TrimPrefix(r.Header().Get("Location"), base+"/")
	if len(user) != 36 {
		t.Fatalf("redirect to %q", r.Header().Get("Location"))
	}
	c.Post(base, url.Values{"email": {"bob@example.com"}, "password": {"secret-password"}}).AssertUnprocessable().AssertSee("already been taken")

	c.Get(base).AssertOK().AssertSee("bob@example.com", "1 user")
	c.Get(base+"?q=nobody").AssertOK().AssertSee("No users match")

	page := base + "/" + user
	c.Get(page).AssertOK().AssertSee("bob@example.com", "No data yet", user)

	// Data: add, change, refuse a bad key, delete.
	c.Post(page+"/data", url.Values{"key": {"profile.city"}, "value": {"Kyiv"}}).AssertRedirect(page + "#data")
	c.Post(page+"/data", url.Values{"key": {"plan"}, "value": {"pro"}}).AssertRedirect()
	c.Post(page+"/data", url.Values{"key": {"plan"}, "value": {"team"}}).AssertRedirect()
	c.Get(page).AssertOK().AssertSee("profile.city", "Kyiv", "team").AssertDontSee("No data yet")
	c.Post(page+"/data", url.Values{"key": {"bad key"}, "value": {"x"}}).AssertUnprocessable().AssertSee("letters and digits")
	c.Post(page+"/data/plan/delete", nil).AssertRedirect(page + "#data")
	c.Get(page).AssertOK().AssertDontSee("team")

	// The account.
	c.Post(page, url.Values{"email": {"bob@example.com"}, "username": {"bad name"}}).AssertUnprocessable().AssertSee("only letters")
	c.Post(page, url.Values{"email": {"robert@example.com"}, "username": {"robert"}}).AssertRedirect(page)
	c.Get(page).AssertOK().AssertSee("robert@example.com")
	if u := auth.users[id][0]; u.EmailVerified { // the unchecked box sends false
		t.Fatal("email_verified stayed true")
	}

	// Another developer's application is a 404, and luna-auth is not asked.
	asked := len(auth.apps)
	other := developer(t, apptest.New(t, app), "eve@example.com")
	other.Get(base).AssertNotFound()
	other.Get(page).AssertNotFound()
	other.Post(page+"/delete", nil).AssertNotFound()
	if len(auth.apps) != asked {
		t.Fatal("luna-auth was asked for someone else's application")
	}
	for _, a := range auth.apps {
		if a != id {
			t.Fatalf("luna-auth asked about application %s", a)
		}
	}

	c.Post(page+"/delete", nil).AssertRedirect(base)
	c.Get(page).AssertNotFound()
	c.Get(base).AssertOK().AssertSee("No users yet")
}

func TestUsersWithoutLunaAuth(t *testing.T) {
	t.Setenv("LUNA_AUTH_URL", "")
	c := developer(t, apptest.New(t, newApp(t)), "ann@example.com")
	id := createApp(t, c, "Shop")
	c.Get("/apps/"+id+"/users").AssertStatus(http.StatusBadGateway).AssertSee("not connected")
}
