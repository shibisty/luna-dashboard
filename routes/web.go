// Package routes registers the application's routes.
package routes

import (
	"time"

	"fullstack"
	auth "fullstack-passport"
	"fullstack/web"

	"luna-dashboard/app/http/controllers"
)

// Web registers the pages: the session cookie identifies the user and
// forms carry the CSRF token.
func Web(app *fullstack.App, au *auth.Auth) {
	home := controllers.Home{App: app}
	login := controllers.Auth{App: app, Auth: au}
	ap := controllers.Apps{App: app}
	app.Router.Group("/", func(r *web.Router) {
		r.Use(au.Web(), au.CSRF())
		r.Get("/", home.Index).Name("home")
		r.Get("/dashboard", home.Dashboard, au.Require()).Name("dashboard")

		r.Get("/login", login.ShowLogin, au.Guest()).Name("login")
		r.Post("/login", login.Login, au.Guest(), au.Throttle(5, time.Minute))
		r.Get("/register", login.ShowRegister, au.Guest()).Name("register")
		r.Post("/register", login.Register, au.Guest())
		r.Post("/logout", login.Logout, au.Require()).Name("logout")

		// Applications and their API keys; another developer's are 404s.
		// HTML forms only send GET and POST, so changes are POSTs.
		r.Group("/apps", func(r *web.Router) {
			r.Get("", ap.Index).Name("apps")
			r.Post("", ap.Create)
			r.Get("/{app}", ap.Show).Name("apps.show")
			r.Post("/{app}", ap.Update)
			r.Post("/{app}/delete", ap.Delete)
			r.Get("/{app}/keys/new", ap.NewKey).Name("apps.keys.new")
			r.Post("/{app}/keys", ap.CreateKey)
			r.Post("/{app}/keys/{key}/regenerate", ap.Regenerate)
			r.Post("/{app}/keys/{key}/revoke", ap.Revoke)
		}, au.Require())
	})
}
