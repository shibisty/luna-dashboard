package models

import "orm"

// Application is a developer's client application (a site, a service, an
// app): what its API keys belong to.
type Application struct {
	orm.BaseModel
	PublicID         string `db:"public_id"` // uuid, the app_id other services see
	UserID           int64  `db:"user_id"`   // the owner
	Name             string `db:"name"`
	PasswordResetURL string `db:"password_reset_url"` // where luna-auth's reset emails link to
	orm.Timestamps
}

// TableName implements orm.Model.
func (Application) TableName() string { return "applications" }
