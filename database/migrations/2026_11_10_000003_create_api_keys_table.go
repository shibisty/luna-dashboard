package migrations

import (
	"context"

	ormstore "migration-orm"
	"orm/schema"
)

func init() {
	register(ormstore.Migration{
		Name: "2026_11_10_000003_create_api_keys_table",
		Up: func(ctx context.Context, db *ormstore.DB) error {
			return db.Schema.Create(ctx, "api_keys", func(t *schema.Blueprint) {
				t.ID()
				t.UUID("public_id").Unique()
				t.ForeignID("application_id").References("id").On("applications").OnDelete("cascade")
				t.String("name", 100)
				t.String("kind", 16)
				t.Char("hash", 64).Unique() // hex SHA-256 of the key
				t.String("hint", 32)
				t.JSONB("scopes")
				t.JSONB("origins")
				t.JSONB("ips")
				t.Timestamp("last_used_at").Nullable()
				t.Timestamp("revoked_at").Nullable()
				t.Timestamps()
				t.IndexCols("application_id")
			})
		},
		Down: func(ctx context.Context, db *ormstore.DB) error {
			return db.Schema.DropIfExists(ctx, "api_keys")
		},
	})
}
