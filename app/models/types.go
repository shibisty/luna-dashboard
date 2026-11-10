package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// StringList is a list of strings kept in a JSONB column ("[]" when
// empty). orm-postgres cannot read Go slices back from TEXT[] yet, and JSON
// keeps the model portable.
type StringList []string

// Value implements driver.Valuer.
func (l StringList) Value() (driver.Value, error) {
	if l == nil {
		l = StringList{}
	}
	b, err := json.Marshal([]string(l))
	return string(b), err
}

// Scan implements sql.Scanner.
func (l *StringList) Scan(src any) error {
	var b []byte
	switch v := src.(type) {
	case nil:
		*l = nil
		return nil
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		return fmt.Errorf("models: StringList from %T", src)
	}
	return json.Unmarshal(b, (*[]string)(l))
}
