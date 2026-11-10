// Package sessions keeps the developers' login sessions in the database
// (passport-session-sql, table passport_sessions).
package sessions

import (
	"context"
	"time"

	"fullstack"
	database "fullstack-orm"
	"passport"
	sqlstore "passport-session-sql"
)

// Store is a passport.SessionStore over the application's database. It
// resolves the executor on every call, so it can be created before the
// database provider boots and follows database.UseTransaction in tests.
type Store struct {
	App   *fullstack.App
	Table string // "" is sqlstore.DefaultSessionsTable
}

var _ passport.SessionStore = Store{}

func (s Store) store() (*sqlstore.SessionStore, error) {
	exec, err := database.Exec(s.App)
	if err != nil {
		return nil, err
	}
	return sqlstore.Sessions(exec, s.Table), nil
}

// Create implements passport.SessionStore.
func (s Store) Create(ctx context.Context, id *passport.Identity, ttl time.Duration) (string, error) {
	st, err := s.store()
	if err != nil {
		return "", err
	}
	return st.Create(ctx, id, ttl)
}

// Get implements passport.SessionStore.
func (s Store) Get(ctx context.Context, sessionID string) (*passport.Identity, error) {
	st, err := s.store()
	if err != nil {
		return nil, err
	}
	return st.Get(ctx, sessionID)
}

// Destroy implements passport.SessionStore.
func (s Store) Destroy(ctx context.Context, sessionID string) error {
	st, err := s.store()
	if err != nil {
		return err
	}
	return st.Destroy(ctx, sessionID)
}

// Sweep deletes expired sessions.
func (s Store) Sweep(ctx context.Context) (int64, error) {
	st, err := s.store()
	if err != nil {
		return 0, err
	}
	return st.Sweep(ctx)
}
