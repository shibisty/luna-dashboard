package routes

import (
	"fullstack"
	"fullstack/web"
	"luna-keys"

	"luna-dashboard/app/keystore"
)

// API registers the machine endpoints: the health check and the API key
// introspection that luna-auth and luna-pages call on the internal network
// (nginx never routes /internal/).
func API(app *fullstack.App) error {
	cfg, err := keystore.Load()
	if err != nil {
		return err
	}
	app.Router.Get("/healthz", func(c *web.Ctx) error {
		return c.JSON(200, map[string]string{"status": "ok", "app": app.Config.Name})
	}).Name("healthz")
	app.Router.HandleHTTP("POST", keys.IntrospectPath, keys.Handler(keystore.Lookup(app), cfg.Tokens()...)).Name("keys.introspect")
	return nil
}
