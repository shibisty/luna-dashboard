// Package providers holds the application's service providers.
package providers

import (
	"fullstack"
	auth "fullstack-passport"
	"fullstack/view"
	"fullstack/web"

	"luna-dashboard/app/sessions"
	"luna-dashboard/resources"
	"luna-dashboard/routes"
)

// RouteProvider sets up the views and the routes.
type RouteProvider struct{}

// Register implements fullstack.Provider.
func (RouteProvider) Register(*fullstack.App) error { return nil }

// Boot loads the views (from disk and re-read on every request with
// APP_ENV=local, embedded otherwise) and
// registers the routes.
func (RouteProvider) Boot(app *fullstack.App) error {
	views, reload := resources.Views, false
	if app.IsLocal() && resources.OnDisk() {
		views, reload = resources.Disk(), true
	}
	v, err := view.New(views, view.Options{Dir: "views", Reload: reload})
	if err != nil {
		return err
	}
	app.Router.SetRenderer(v)
	app.Router.Use(web.SecureHeaders(false))
	au, err := fullstack.Make[*auth.Auth](app)
	if err != nil {
		return err
	}
	keeper, err := sessions.NewKeeper(app, au)
	if err != nil {
		return err
	}
	fullstack.Set(app, keeper)
	routes.Web(app, au, keeper)
	return routes.API(app)
}
