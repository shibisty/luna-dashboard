// Package keystore configures the introspection endpoint that answers the
// other Luna services about API keys (luna-keys' protocol).
package keystore

import (
	"context"
	"strings"

	"fullstack"
	"fullstack/env"
	"luna-keys"

	"luna-dashboard/app/apps"
)

// Config is the introspection endpoint's configuration.
type Config struct {
	// ServiceTokens are the tokens of the services allowed to ask
	// (luna-auth, luna-pages), separated by commas.
	ServiceTokens string `env:"LUNA_SERVICE_TOKENS"`
}

// Tokens are the configured service tokens.
func (c Config) Tokens() []string {
	var out []string
	for _, t := range strings.Split(c.ServiceTokens, ",") {
		if t = strings.TrimSpace(t); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// Load reads the configuration.
func Load() (Config, error) {
	var c Config
	err := env.Decode(&c)
	return c, err
}

// Lookup finds an active key by its hash (apps.Service.Lookup).
func Lookup(app *fullstack.App) keys.LookupFunc {
	return func(ctx context.Context, hash string) (*keys.Key, error) {
		svc, err := fullstack.Make[*apps.Service](app)
		if err != nil {
			return nil, err
		}
		return svc.Lookup(ctx, hash)
	}
}
