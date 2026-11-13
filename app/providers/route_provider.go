// Package providers holds the application's service providers.
package providers

import (
	"errors"
	"html/template"
	"net/http"
	"strings"

	"fullstack"
	auth "fullstack-passport"
	"fullstack/view"
	"fullstack/web"

	"luna-dashboard/app/http/controllers"
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
	v, err := view.New(views, view.Options{Dir: "views", Reload: reload, Funcs: template.FuncMap{"upper": strings.ToUpper, "join": strings.Join, "statusText": http.StatusText, "dict": dict, "size": controllers.SizeText}})
	if err != nil {
		return err
	}
	app.Router.SetRenderer(v)
	app.Router.OnError(controllers.Errors(app))
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

// dict builds a map for a template call with several values:
// {{template "x" (dict "In" . "L" $.L)}}.
func dict(kv ...any) (map[string]any, error) {
	if len(kv)%2 != 0 {
		return nil, errors.New("dict: odd number of arguments")
	}
	m := make(map[string]any, len(kv)/2)
	for i := 0; i < len(kv); i += 2 {
		k, ok := kv[i].(string)
		if !ok {
			return nil, errors.New("dict: keys are strings")
		}
		m[k] = kv[i+1]
	}
	return m, nil
}
