package web

import (
	"encoding/json"

	"github.com/jeffmvr/mangle-vpn/internal/uuid"
)

// uuidField is a UUID in a request body.
//
// A malformed value is recorded rather than returned as an error, so that
// the handler can answer with a message against the field the frontend can
// show, instead of failing the whole decode with nothing to display.
type uuidField struct {
	Value   uuid.UUID
	Present bool
	Valid   bool
}

// UnmarshalJSON implements [json.Unmarshaler].
func (f *uuidField) UnmarshalJSON(data []byte) error {
	f.Present = true

	var raw string
	if err := json.Unmarshal(data, &raw); err != nil || raw == "" {
		return nil
	}

	value, err := uuid.Parse(raw)
	if err != nil {
		return nil
	}

	f.Value, f.Valid = value, true
	return nil
}

// MarshalJSON implements [json.Marshaler].
func (f uuidField) MarshalJSON() ([]byte, error) {
	if !f.Valid {
		return []byte("null"), nil
	}
	return json.Marshal(f.Value)
}

// resolve reads the field into target, recording a message against it when
// the value was given but unusable. It reports whether the field is set,
// either from this request or from what target already held.
func (f uuidField) resolve(errs fieldErrors, name string, target *uuid.UUID, exists func(uuid.UUID) bool) {
	if !f.Present {
		return
	}
	if !f.Valid {
		errs.add(name, "Must be a valid UUID.")
		return
	}
	if !exists(f.Value) {
		errs.add(name, "The selected group does not exist.")
		return
	}
	*target = f.Value
}

// optionalBoolField is a nullable boolean in a request body that also
// records whether it was sent at all, so that leaving it out can mean
// "unchanged" while null still means "clear it".
type optionalBoolField struct {
	Value   *bool
	Present bool
}

// UnmarshalJSON implements [json.Unmarshaler].
func (f *optionalBoolField) UnmarshalJSON(data []byte) error {
	f.Present = true
	return json.Unmarshal(data, &f.Value)
}
