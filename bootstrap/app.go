// Package bootstrap builds the application: its configuration and providers.
package bootstrap

import (
	"fullstack"
	database "fullstack-orm"
	auth "fullstack-passport"
	"orm"
	postgres "orm-postgres"
	"passport"
	"passport-local"

	"luna-dashboard/app/apps"
	"luna-dashboard/app/providers"
	"luna-dashboard/app/sessions"
	"luna-dashboard/app/users"
	"luna-dashboard/database/migrations"
	"luna-dashboard/database/seeders"
)

// NewApp creates the application with every provider registered. Tests pass
// Options to choose the .env files and the log output.
func NewApp(opts ...fullstack.Options) (*fullstack.App, error) {
	app, err := fullstack.New(opts...)
	if err != nil {
		return nil, err
	}
	err = app.Register(
		&database.Provider{
			Drivers:    map[string]func(string) orm.Driver{"postgres": postgres.Driver},
			Migrations: migrations.All,
			Seeders: []database.Seeder{
				{Name: "Users", Run: seeders.Users},
			},
		},
		&users.Provider{},
		&apps.Provider{},
		&auth.Provider{
			// The local strategy checks emails and passwords against the users table.
			Strategies: []passport.Strategy{local.New(users.Verify(app))},
			// Sessions live in the database (passport_sessions), not in memory.
			Sessions: sessions.Store{App: app},
		},
		&providers.RouteProvider{},
	)
	return app, err
}
