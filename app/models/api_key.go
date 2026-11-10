package models

import (
	"time"

	"orm"
)

// APIKey is a key of an application. The key itself is shown once; only
// its SHA-256 (Hash) and a Hint are kept.
type APIKey struct {
	orm.BaseModel
	PublicID      string     `db:"public_id"` // uuid, the key_id other services see
	ApplicationID int64      `db:"application_id"`
	Name          string     `db:"name"`
	Kind          string     `db:"kind"` // keys.Server or keys.Browser
	Hash          string     `db:"hash"`
	Hint          string     `db:"hint"` // sk_live_…Q9xF
	Scopes        StringList `db:"scopes"`
	Origins       StringList `db:"origins"` // allowed Origin values (browser keys)
	IPs           StringList `db:"ips"`     // allowed client networks; empty: any
	LastUsedAt    *time.Time `db:"last_used_at"`
	RevokedAt     *time.Time `db:"revoked_at"`
	orm.Timestamps
}

// TableName implements orm.Model.
func (APIKey) TableName() string { return "api_keys" }

// Active reports whether the key is not revoked.
func (k *APIKey) Active() bool { return k.RevokedAt == nil }
