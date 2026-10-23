package render

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"gopkg.in/yaml.v3"
)

// Tabular is implemented by results that know how to lay themselves out as a table.
type Tabular interface {
	Headers() []string
	Rows() [][]string
}

// Plainer is implemented by results that have a one-line or free-text form.
type Plainer interface {
	Plain() string
}

// Envelope is the stable machine-readable wrapper for every command result.
type Envelope[T any] struct {
	OK   bool `json:"ok" yaml:"ok"`
	Data T    `json:"data" yaml:"data"`
}

// ErrorBody is the machine-readable error payload.
type ErrorBody struct {
	Code    string `json:"code" yaml:"code"`
	Exit    int    `json:"exit" yaml:"exit"`
	Message string `json:"message" yaml:"message"`
}

// ErrorEnvelope is the wrapper emitted on failure when a structured format is selected.
type ErrorEnvelope struct {
	OK    bool      `json:"ok" yaml:"ok"`
	Error ErrorBody `json:"error" yaml:"error"`
}

// Print writes v to w in the requested format.
//
// Table output requires v to implement Tabular; plain output requires Plainer.
// When the value lacks the interface, Print falls back to YAML for table and to
// a fmt rendering for plain, so no result is ever silently dropped.
func Print[T any](w io.Writer, f Format, v T) error {
	switch f {
	case FormatJSON:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(Envelope[T]{OK: true, Data: v})
	case FormatYAML:
		enc := yaml.NewEncoder(w)
		enc.SetIndent(2)
		if err := enc.Encode(Envelope[T]{OK: true, Data: v}); err != nil {
			return err
		}
		return enc.Close()
	case FormatPlain:
		if p, ok := any(v).(Plainer); ok {
			_, err := io.WriteString(w, strings.TrimRight(p.Plain(), "\n")+"\n")
			return err
		}
		_, err := fmt.Fprintln(w, v)
		return err
	case FormatTable:
		if t, ok := any(v).(Tabular); ok {
			return Table(w, t)
		}
		enc := yaml.NewEncoder(w)
		enc.SetIndent(2)
		if err := enc.Encode(v); err != nil {
			return err
		}
		return enc.Close()
	default:
		return fmt.Errorf("render: unsupported format %d", f)
	}
}

// Table writes a Tabular value as aligned columns.
func Table(w io.Writer, t Tabular) error {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	if h := t.Headers(); len(h) > 0 {
		if _, err := fmt.Fprintln(tw, strings.Join(h, "\t")); err != nil {
			return err
		}
	}
	for _, r := range t.Rows() {
		if _, err := fmt.Fprintln(tw, strings.Join(r, "\t")); err != nil {
			return err
		}
	}
	return tw.Flush()
}

// PrintError writes a failure in the requested format. Table and plain formats
// write a human line; JSON and YAML write an ErrorEnvelope.
func PrintError(w io.Writer, f Format, code string, exit int, msg string) error {
	switch f {
	case FormatJSON:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(ErrorEnvelope{Error: ErrorBody{Code: code, Exit: exit, Message: msg}})
	case FormatYAML:
		enc := yaml.NewEncoder(w)
		enc.SetIndent(2)
		if err := enc.Encode(ErrorEnvelope{Error: ErrorBody{Code: code, Exit: exit, Message: msg}}); err != nil {
			return err
		}
		return enc.Close()
	default:
		_, err := fmt.Fprintf(w, "error: %s\n", msg)
		return err
	}
}

// KV is a simple two-column key/value table.
type KV [][2]string

// Headers implements Tabular.
func (KV) Headers() []string { return nil }

// Rows implements Tabular.
func (kv KV) Rows() [][]string {
	rows := make([][]string, 0, len(kv))
	for _, p := range kv {
		rows = append(rows, []string{p[0], p[1]})
	}
	return rows
}
