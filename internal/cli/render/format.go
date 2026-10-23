// Package render turns typed results into terminal or machine-readable output.
package render

import (
	"fmt"
	"strings"
)

// Format selects an output encoding.
type Format uint8

// Supported output formats.
const (
	FormatTable Format = iota
	FormatJSON
	FormatYAML
	FormatPlain
)

var formatNames = map[Format]string{
	FormatTable: "table",
	FormatJSON:  "json",
	FormatYAML:  "yaml",
	FormatPlain: "plain",
}

// String implements fmt.Stringer and pflag.Value.
func (f Format) String() string { return formatNames[f] }

// Type implements pflag.Value.
func (Format) Type() string { return "table|json|yaml|plain" }

// Set implements pflag.Value.
func (f *Format) Set(s string) error {
	v, err := ParseFormat(s)
	if err != nil {
		return err
	}
	*f = v
	return nil
}

// ParseFormat converts a user string into a Format.
func ParseFormat(s string) (Format, error) {
	for k, v := range formatNames {
		if strings.EqualFold(v, s) {
			return k, nil
		}
	}
	return FormatTable, fmt.Errorf("render: unknown output format %q (want table, json, yaml or plain)", s)
}

// MarshalText implements encoding.TextMarshaler.
func (f Format) MarshalText() ([]byte, error) { return []byte(f.String()), nil }

// UnmarshalText implements encoding.TextUnmarshaler.
func (f *Format) UnmarshalText(b []byte) error { return f.Set(string(b)) }
