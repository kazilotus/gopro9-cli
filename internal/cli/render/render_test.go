package render

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type sample struct {
	Name  string `json:"name" yaml:"name"`
	Count int    `json:"count" yaml:"count"`
}

func (s sample) Headers() []string { return []string{"NAME", "COUNT"} }
func (s sample) Rows() [][]string  { return [][]string{{s.Name, "42"}} }
func (s sample) Plain() string     { return s.Name }

type bare struct {
	X int `json:"x" yaml:"x"`
}

func TestParseFormat(t *testing.T) {
	t.Parallel()
	for _, s := range []string{"table", "JSON", "Yaml", "plain"} {
		_, err := ParseFormat(s)
		require.NoError(t, err, s)
	}
	_, err := ParseFormat("xml")
	require.Error(t, err)

	var f Format
	require.NoError(t, f.Set("json"))
	assert.Equal(t, FormatJSON, f)
	assert.Equal(t, "json", f.String())
	assert.Equal(t, "table|json|yaml|plain", f.Type())

	b, err := f.MarshalText()
	require.NoError(t, err)
	assert.Equal(t, "json", string(b))
	var g Format
	require.NoError(t, g.UnmarshalText([]byte("yaml")))
	assert.Equal(t, FormatYAML, g)
}

func TestPrintJSONEnvelope(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	require.NoError(t, Print(&buf, FormatJSON, sample{Name: "a", Count: 42}))
	var env Envelope[sample]
	require.NoError(t, json.Unmarshal(buf.Bytes(), &env))
	assert.True(t, env.OK)
	assert.Equal(t, sample{Name: "a", Count: 42}, env.Data)
}

func TestPrintYAMLEnvelope(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	require.NoError(t, Print(&buf, FormatYAML, sample{Name: "a", Count: 42}))
	assert.Equal(t, "ok: true\ndata:\n  name: a\n  count: 42\n", buf.String())
}

func TestPrintTable(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	require.NoError(t, Print(&buf, FormatTable, sample{Name: "a"}))
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	require.Len(t, lines, 2)
	assert.Contains(t, lines[0], "NAME")
	assert.Contains(t, lines[1], "42")
}

func TestPrintTableFallsBackToYAML(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	require.NoError(t, Print(&buf, FormatTable, bare{X: 7}))
	assert.Equal(t, "x: 7\n", buf.String())
}

func TestPrintPlain(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	require.NoError(t, Print(&buf, FormatPlain, sample{Name: "only"}))
	assert.Equal(t, "only\n", buf.String())

	buf.Reset()
	require.NoError(t, Print(&buf, FormatPlain, bare{X: 1}))
	assert.Equal(t, "{1}\n", buf.String())
}

func TestPrintUnknownFormat(t *testing.T) {
	t.Parallel()
	require.Error(t, Print(&bytes.Buffer{}, Format(99), bare{}))
}

func TestPrintError(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	require.NoError(t, PrintError(&buf, FormatTable, "unavailable", 69, "camera not found"))
	assert.Equal(t, "error: camera not found\n", buf.String())

	buf.Reset()
	require.NoError(t, PrintError(&buf, FormatJSON, "unavailable", 69, "camera not found"))
	var env ErrorEnvelope
	require.NoError(t, json.Unmarshal(buf.Bytes(), &env))
	assert.False(t, env.OK)
	assert.Equal(t, 69, env.Error.Exit)

	buf.Reset()
	require.NoError(t, PrintError(&buf, FormatYAML, "io", 74, "disk full"))
	assert.Contains(t, buf.String(), "exit: 74")
}

func TestKV(t *testing.T) {
	t.Parallel()
	kv := KV{{"a", "1"}, {"b", "2"}}
	assert.Nil(t, kv.Headers())
	assert.Equal(t, [][]string{{"a", "1"}, {"b", "2"}}, kv.Rows())
}
