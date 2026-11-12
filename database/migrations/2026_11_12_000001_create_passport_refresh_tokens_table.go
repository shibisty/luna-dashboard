package migrations

import (
	"context"

	ormstore "migration-orm"
	sqlstore "passport-session-sql"
)

// The developers' refresh tokens (sessions.Keeper): they start the next
// session when the last one ends.
func init() {
	register(ormstore.Migration{
		Name: "2026_11_12_000001_create_passport_refresh_tokens_table",
		Up: func(ctx context.Context, db *ormstore.DB) error {
			return db.Schema.Create(ctx, sqlstore.DefaultRefreshTable, sqlstore.RefreshTable)
		},
		Down: func(ctx context.Context, db *ormstore.DB) error {
			return db.Schema.DropIfExists(ctx, sqlstore.DefaultRefreshTable)
		},
	})
}
