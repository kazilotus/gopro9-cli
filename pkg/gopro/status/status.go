// Package status provides the typed decoding of the camera's status block.
package status

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// StatusID is the numeric status identifier used on the wire.
type StatusID uint16

// String returns the spec's status name, or "status-N" when undocumented.
func (id StatusID) String() string {
	if n, ok := statusNames[id]; ok {
		return n
	}
	return "status-" + strconv.Itoa(int(id))
}

// MarshalText implements encoding.TextMarshaler so maps keyed by StatusID stay readable.
func (id StatusID) MarshalText() ([]byte, error) { return []byte(id.String()), nil }

// SupportedOnHERO9 reports whether the spec lists the status for the HERO9 Black.
func (id StatusID) SupportedOnHERO9() bool { return hero9Statuses[id] }

// rawValue is one wire value, which may be a number or a string.
type rawValue struct{ raw json.RawMessage }

func (r rawValue) Int() int64 {
	var n json.Number
	if err := json.Unmarshal(r.raw, &n); err != nil {
		return 0
	}
	i, err := n.Int64()
	if err != nil {
		f, ferr := n.Float64()
		if ferr != nil {
			return 0
		}
		return int64(f)
	}
	return i
}

func (r rawValue) Str() string {
	var s string
	if err := json.Unmarshal(r.raw, &s); err == nil {
		return s
	}
	return string(r.raw)
}

// UnmarshalJSON decodes the camera's `{"1": 1, "30": "name", ...}` status block.
func (v *Values) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("status: decode: %w", err)
	}
	*v = Values{}
	for k, msg := range raw {
		id, err := strconv.ParseUint(k, 10, 16)
		if err != nil {
			return fmt.Errorf("status: bad status id %q: %w", k, err)
		}
		v.apply(StatusID(id), rawValue{raw: msg})
	}
	return nil
}
