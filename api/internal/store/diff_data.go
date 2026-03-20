package store

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// DiffData is a JSON payload for diff computation result.
// It is intentionally schema-flexible for alpha; concrete structure can evolve later.
type DiffData json.RawMessage

func EmptyDiffData() DiffData {
	return DiffData(json.RawMessage("[]"))
}

func (d DiffData) Value() (driver.Value, error) {
	if len(d) == 0 {
		return nil, nil
	}
	if !json.Valid(d) {
		return nil, fmt.Errorf("invalid diff_data json")
	}
	return []byte(d), nil
}

func (d *DiffData) Scan(src any) error {
	if src == nil {
		*d = nil
		return nil
	}
	switch v := src.(type) {
	case []byte:
		if len(v) == 0 {
			*d = nil
			return nil
		}
		if !json.Valid(v) {
			return fmt.Errorf("invalid diff_data json")
		}
		*d = append((*d)[:0], v...)
		return nil
	case string:
		if v == "" {
			*d = nil
			return nil
		}
		b := []byte(v)
		if !json.Valid(b) {
			return fmt.Errorf("invalid diff_data json")
		}
		*d = append((*d)[:0], b...)
		return nil
	default:
		return fmt.Errorf("unsupported diff_data type: %T", src)
	}
}
