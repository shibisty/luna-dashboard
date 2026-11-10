// Package keystore answers the other Luna services about API keys
// (luna-keys' introspection protocol). Applications and keys arrive in
// stage 4 of the plan; until then every key is unknown.
package keystore

import (
	"context"
	"strings"

	"fullstack"
	"fullstack/env"
	"luna-keys"
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

// Lookup finds a key by its hash.
func Lookup(app *fullstack.App) keys.LookupFunc {
	return func(ctx context.Context, hash string) (*keys.Key, error) {
		return nil, keys.ErrUnknownKey // the api_keys table comes in stage 4
	}
}
