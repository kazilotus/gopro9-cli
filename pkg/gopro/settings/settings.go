// Package settings provides typed access to HERO9 camera settings.
//
// Every setting the Open GoPro spec documents for the HERO9 Black has a
// generated Go type whose constants are the options the camera accepts.
// The Registry maps CLI-friendly names to those types so a generic
// `setting set <name> <value>` can still be fully validated.
package settings

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

//go:generate go run ../../../internal/gen -root ../../..

// SettingID is the numeric setting identifier used on the wire.
type SettingID uint16

// String returns the CLI name of the setting, or "setting-N" for undocumented IDs.
func (id SettingID) String() string {
	if d, ok := byID[id]; ok {
		return d.Name
	}
	return "setting-" + strconv.Itoa(int(id))
}

// MarshalText implements encoding.TextMarshaler so maps keyed by SettingID stay readable.
func (id SettingID) MarshalText() ([]byte, error) { return []byte(id.String()), nil }

// Option is any typed setting value. Each generated setting type implements it.
type Option interface {
	ID() SettingID
	Value() uint32
	String() string
}

// OptionInfo describes one selectable option of an enumerated setting.
type OptionInfo struct {
	Value uint32 `json:"value" yaml:"value"`
	Name  string `json:"name" yaml:"name"`
	Label string `json:"label" yaml:"label"`
}

// Descriptor is the registry entry for one setting.
type Descriptor struct {
	ID      SettingID    `json:"id" yaml:"id"`
	Name    string       `json:"name" yaml:"name"`
	Title   string       `json:"title" yaml:"title"`
	Numeric bool         `json:"numeric" yaml:"numeric"`
	Min     uint32       `json:"min,omitempty" yaml:"min,omitempty"`
	Max     uint32       `json:"max,omitempty" yaml:"max,omitempty"`
	Options []OptionInfo `json:"options,omitempty" yaml:"options,omitempty"`

	parse     func(string) (Option, error)
	fromValue func(uint32) (Option, bool)
}

// Parse converts user input (label or number) into a validated Option.
func (d Descriptor) Parse(s string) (Option, error) { return d.parse(s) }

// FromValue converts a wire value into a typed Option, reporting validity.
func (d Descriptor) FromValue(v uint32) (Option, bool) { return d.fromValue(v) }

var (
	byID   = map[SettingID]Descriptor{}
	byName = map[string]Descriptor{}
)

func init() {
	for _, d := range descriptors {
		byID[d.ID] = d
		byName[normalize(d.Name)] = d
	}
}

// All returns every documented HERO9 setting, ordered by ID.
func All() []Descriptor {
	out := make([]Descriptor, 0, len(descriptors))
	out = append(out, descriptors...)
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Lookup finds a setting by CLI name ("video-resolution"), title, or numeric ID.
func Lookup(name string) (Descriptor, bool) {
	if n, err := strconv.ParseUint(name, 10, 16); err == nil {
		d, ok := byID[SettingID(n)]
		return d, ok
	}
	d, ok := byName[normalize(name)]
	return d, ok
}

// ByID finds a setting by its numeric identifier.
func ByID(id SettingID) (Descriptor, bool) {
	d, ok := byID[id]
	return d, ok
}

// normalize lowercases and strips everything but letters and digits so
// "2.7K 4:3", "2.7k-4-3" and "27k43" compare equal.
func normalize(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// UnmarshalJSON decodes the camera's `{"2": 9, "3": 10, ...}` settings block.
func (v *Values) UnmarshalJSON(data []byte) error {
	var raw map[string]json.Number
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("settings: decode: %w", err)
	}
	*v = Values{}
	for k, n := range raw {
		id, err := strconv.ParseUint(k, 10, 16)
		if err != nil {
			return fmt.Errorf("settings: bad setting id %q: %w", k, err)
		}
		val, err := n.Int64()
		if err != nil {
			return fmt.Errorf("settings: setting %s has non-integer value %s: %w", k, n, err)
		}
		v.apply(SettingID(id), val)
	}
	return nil
}
