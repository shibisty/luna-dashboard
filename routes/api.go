package routes

import (
	"fullstack"
	"fullstack/env"
	"fullstack/web"
	"luna-keys"

	"luna-dashboard/app/keystore"
	"luna-dashboard/docs"
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
	// The internal API's description; Swagger UI only with API_DOCS=true.
	docs.Register(app.Router, "/internal/openapi.yaml", "/internal/docs", "Luna Dashboard internal API", env.Bool("API_DOCS", false))
	return nil
}
