// Package tests holds the application's HTTP tests. They run on Postgres:
// set TEST_DATABASE_DSN (a database the tests may migrate); each test runs
// in a transaction that is rolled back. Without it the tests are skipped.
package tests

import (
	"io"
	"net/url"
	"os"
	"strings"
	"testing"

	"fullstack"
	database "fullstack-orm"
	apptest "fullstack-testing"
	"luna-keys"

	"luna-dashboard/bootstrap"
)

const serviceToken = "test-service-token"

func newApp(t *testing.T) *fullstack.App {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN is not set (a Postgres database for the tests)")
	}
	for k, v := range map[string]string{
		"APP_NAME": "luna-dashboard", "APP_ENV": "testing",
		"DB_DRIVER": "postgres", "DATABASE_DSN": dsn, "DB_MIGRATE": "true",
		"LUNA_SERVICE_TOKENS": serviceToken,
	} {
		t.Setenv(k, v)
	}
	app, err := bootstrap.NewApp(fullstack.Options{EnvFiles: []string{}, LogOutput: io.Discard})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.Handler(); err != nil { // boot: migrations run before the transaction starts
		t.Fatal(err)
	}
	database.UseTransaction(t, app)
	return app
}

func TestHome(t *testing.T) {
	c := apptest.New(t, newApp(t))
	c.Get("/").AssertOK().AssertSee("Welcome to luna-dashboard", "Log in")
	c.Get("/dashboard").AssertRedirect("/login?redirect=%2Fdashboard")
	c.GetJSON("/healthz").AssertOK().AssertJSONPath("status", "ok")
}

func TestRegisterAndLogin(t *testing.T) {
	c := apptest.New(t, newApp(t))
	c.Get("/register").AssertOK()
	c.Post("/register", url.Values{"name": {"Ann"}, "email": {"ann@example.com"}, "password": {"short"}, "password_confirmation": {"short"}}).
		AssertUnprocessable().AssertSee("at least 8 characters")
	form := url.Values{"name": {"Ann"}, "email": {"Ann@Example.com"}, "password": {"secret-password"}, "password_confirmation": {"secret-password"}}
	c.Post("/register", form).AssertRedirect("/dashboard")
	c.Get("/dashboard").AssertOK().AssertSee("Hello, Ann (ann@example.com)")
	c.Post("/logout", nil).AssertRedirect("/")
	c.Get("/dashboard").AssertRedirect()

	c.Post("/register", form).AssertUnprocessable().AssertSee("already been taken")
	c.Post("/login", url.Values{"email": {"ann@example.com"}, "password": {"wrong-password"}}).AssertUnprocessable().AssertSee("do not match")
	c.Post("/login?redirect=/dashboard", url.Values{"email": {"ann@example.com"}, "password": {"secret-password"}}).AssertRedirect("/dashboard")
	c.Get("/login").AssertRedirect("/") // guests only
}

func TestIntrospection(t *testing.T) {
	c := apptest.New(t, newApp(t))
	body := map[string]any{"hash": keys.Hash("sk_live_" + strings.Repeat("a", 43))}
	c.PostJSON(keys.IntrospectPath, body).AssertUnauthorized()
	c.WithToken("wrong").PostJSON(keys.IntrospectPath, body).AssertUnauthorized()
	c.WithToken(serviceToken).PostJSON(keys.IntrospectPath, body).AssertOK().AssertJSONPath("active", false)
}
