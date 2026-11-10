package migrations

import (
	"context"

	ormstore "migration-orm"
	sqlstore "passport-session-sql"
)

func init() {
	register(ormstore.Migration{
		Name: "2026_11_10_000001_create_passport_sessions_table",
		Up: func(ctx context.Context, db *ormstore.DB) error {
			return db.Schema.Create(ctx, sqlstore.DefaultSessionsTable, sqlstore.SessionsTable)
		},
		Down: func(ctx context.Context, db *ormstore.DB) error {
			return db.Schema.DropIfExists(ctx, sqlstore.DefaultSessionsTable)
		},
	})
}
