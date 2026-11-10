package migrations

import (
	"context"

	ormstore "migration-orm"
	"orm/schema"
)

func init() {
	register(ormstore.Migration{
		Name: "2026_11_10_000002_create_applications_table",
		Up: func(ctx context.Context, db *ormstore.DB) error {
			return db.Schema.Create(ctx, "applications", func(t *schema.Blueprint) {
				t.ID()
				t.UUID("public_id").Unique()
				t.ForeignID("user_id").References("id").On("users").OnDelete("cascade")
				t.String("name", 100)
				t.String("password_reset_url", 2048).Default("")
				t.Timestamps()
				t.IndexCols("user_id")
			})
		},
		Down: func(ctx context.Context, db *ormstore.DB) error {
			return db.Schema.DropIfExists(ctx, "applications")
		},
	})
}
