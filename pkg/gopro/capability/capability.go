// Package capability is the single source of truth for what the HERO9 Black
// supports over USB. It is generated from the Open GoPro spec's
// x-supported-cameras annotations, corrected by spec/hero9_overrides.yaml
// where real hardware disagrees with the spec.
package capability

import (
	"errors"
	"fmt"
	"sort"
)

// Op identifies one camera operation by its upstream operationId.
type Op string

// Level is how confident we are that an operation works on the HERO9.
type Level uint8

// Support levels.
const (
	// Unsupported: absent for HERO9 in the spec, or observed to fail on hardware.
	Unsupported Level = iota
	// Spec: listed for HERO9 in the upstream spec; not yet exercised by this project.
	Spec
	// Verified: exercised on a real HERO9 over USB.
	Verified
)

// String implements fmt.Stringer.
func (l Level) String() string {
	switch l {
	case Verified:
		return "verified"
	case Spec:
		return "spec"
	default:
		return "unsupported"
	}
}

// MarshalText implements encoding.TextMarshaler.
func (l Level) MarshalText() ([]byte, error) { return []byte(l.String()), nil }

// Info describes one operation.
type Info struct {
	Op      Op       `json:"op" yaml:"op"`
	Method  string   `json:"method" yaml:"method"`
	Path    string   `json:"path" yaml:"path"`
	Summary string   `json:"summary" yaml:"summary"`
	Tags    []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	InSpec  bool     `json:"in_spec" yaml:"in_spec"`
	Legacy  bool     `json:"legacy" yaml:"legacy"`
	Level   Level    `json:"level" yaml:"level"`
	Note    string   `json:"note,omitempty" yaml:"note,omitempty"`
}

// ErrUnsupported is returned when an operation is known not to work on the HERO9.
var ErrUnsupported = errors.New("capability: operation unsupported on HERO9 Black")

// UnsupportedError carries the operation that was refused.
type UnsupportedError struct {
	Op   Op
	Note string
}

func (e *UnsupportedError) Error() string {
	if e.Note != "" {
		return fmt.Sprintf("%s is not supported on the HERO9 Black (%s)", e.Op, e.Note)
	}
	return fmt.Sprintf("%s is not supported on the HERO9 Black", e.Op)
}

// Unwrap exposes ErrUnsupported for errors.Is.
func (*UnsupportedError) Unwrap() error { return ErrUnsupported }

// Lookup returns the Info for op.
func Lookup(op Op) (Info, bool) {
	i, ok := matrix[op]
	return i, ok
}

// Support returns the support level for op; unknown operations are Unsupported.
func Support(op Op) Level {
	return matrix[op].Level
}

// Check returns an *UnsupportedError when op is known not to work.
func Check(op Op) error {
	i, ok := matrix[op]
	if !ok || i.Level == Unsupported {
		return &UnsupportedError{Op: op, Note: i.Note}
	}
	return nil
}

// All returns every known operation sorted by ID.
func All() []Info {
	out := make([]Info, 0, len(matrix))
	for _, i := range matrix {
		out = append(out, i)
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Op < out[b].Op })
	return out
}
