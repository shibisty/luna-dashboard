// Package routes registers the application's routes.
package routes

import (
	"time"

	"fullstack"
	auth "fullstack-passport"
	"fullstack/web"

	"luna-dashboard/app/fontsproxy"
	"luna-dashboard/app/http/controllers"
	"luna-dashboard/app/sessions"
)

// Web registers the pages: the session cookie identifies the user (a
// refresh token starts the next session, see sessions.Keeper) and forms
// carry the CSRF token.
func Web(app *fullstack.App, au *auth.Auth, keeper *sessions.Keeper) {
	home := controllers.Home{App: app}
	login := controllers.Auth{App: app, Auth: au, Keeper: keeper}
	ap := controllers.Apps{App: app}
	us := controllers.Users{App: app}
	pg := controllers.Pages{App: app}
	fl := controllers.Files{App: app}
	// The page editor's bundle: public, cached by hash.
	app.Router.Get("/assets/editor/{file}", controllers.EditorAsset).Name("assets.editor")
	// The font catalogues for the editor's grapesjs-fonts (their APIs send
	// no CORS headers); public: the plugin fetches without cookies, and
	// the proxy only serves four fixed catalogues from a day's cache.
	fonts := fontsproxy.New()
	app.Router.Get("/editor/fonts/{path...}", func(c *web.Ctx) error { fonts.ServeHTTP(c.W, c.R); return nil }).Name("editor.fonts")
	app.Router.Group("/", func(r *web.Router) {
		r.Use(keeper.Renew(), au.Web(), au.CSRF())
		r.Get("/", home.Index).Name("home")
		r.Get("/dashboard", home.Dashboard, au.Require()).Name("dashboard")
		r.Get("/prefs", controllers.Prefs).Name("prefs")

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

			// The application's users and its data about them (luna-auth).
			r.Get("/{app}/users", us.Index).Name("apps.users")
			r.Post("/{app}/users", us.Create)
			r.Get("/{app}/users/{user}", us.Show).Name("apps.users.show")
			r.Post("/{app}/users/{user}", us.Update)
			r.Post("/{app}/users/{user}/delete", us.Delete)
			r.Post("/{app}/users/{user}/data", us.SetData)
			r.Post("/{app}/users/{user}/data/{key}/delete", us.DeleteData)

			// The application's site (luna-pages): the editor, which talks
			// to luna-pages through api/ (session + CSRF header), previews,
			// and quick forms for entries.
			r.Get("/{app}/pages", pg.Editor).Name("apps.pages")
			r.Any("/{app}/pages/api/{path...}", pg.API)
			r.Get("/{app}/pages/preview", pg.Preview).Name("apps.pages.preview")
			r.Get("/{app}/pages/entries", pg.Entries).Name("apps.pages.entries")
			r.Post("/{app}/pages/entries", pg.CreateEntry)
			r.Get("/{app}/pages/entries/{entry}", pg.Entry).Name("apps.pages.entry")
			r.Post("/{app}/pages/entries/{entry}", pg.SaveEntry)
			r.Post("/{app}/pages/entries/{entry}/publish", pg.PublishEntry)
			r.Post("/{app}/pages/entries/{entry}/delete", pg.DeleteEntry)

			// The application's files (luna-files): the browser, and api/
			// for the uploads of its script and the page editor's assets
			// (session + CSRF header).
			r.Get("/{app}/files", fl.Index).Name("apps.files")
			r.Post("/{app}/files/folders", fl.CreateFolder)
			r.Post("/{app}/files/folders/{folder}", fl.UpdateFolder)
			r.Post("/{app}/files/folders/{folder}/delete", fl.DeleteFolder)
			r.Post("/{app}/files/upload", fl.Upload)
			r.Get("/{app}/files/file/{file}", fl.Show).Name("apps.files.show")
			r.Post("/{app}/files/file/{file}", fl.Update)
			r.Post("/{app}/files/file/{file}/delete", fl.Delete)
			r.Get("/{app}/files/file/{file}/open", fl.Open).Name("apps.files.open")
			r.Post("/{app}/files/file/{file}/replace", fl.Replace)
			r.Get("/{app}/files/file/{file}/versions/{n}/open", fl.OpenVersion)
			r.Post("/{app}/files/file/{file}/versions/{n}/restore", fl.RestoreVersion)
			r.Post("/{app}/files/file/{file}/versions/{n}/delete", fl.DeleteVersion)
			r.Any("/{app}/files/api/{path...}", fl.API)
		}, au.Require())
	})
}
