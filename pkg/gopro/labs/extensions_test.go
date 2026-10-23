package labs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtensions(t *testing.T) {
	t.Parallel()
	all := Extensions()
	require.NotEmpty(t, all)
	seen := map[string]bool{}
	for i, e := range all {
		assert.Len(t, e.FourCC, 4, e.FourCC)
		assert.False(t, seen[e.FourCC], "duplicate %s", e.FourCC)
		seen[e.FourCC] = true
		assert.NotEmpty(t, e.Description)
		if i > 0 {
			assert.Less(t, all[i-1].FourCC, e.FourCC, "sorted")
		}
	}
	for _, f := range []string{"BOOT", "HIST", "GAIN", "RTMP", "SPED"} {
		e, ok := LookupExtension(f)
		require.True(t, ok, f)
		assert.Equal(t, Supported, e.Support, f)
	}
	for _, f := range []string{"BITR", "LEVL", "SHEL", "PRES", "TUSB", "STDL"} {
		e, ok := LookupExtension(f)
		require.True(t, ok, f)
		assert.Equal(t, Unsupported, e.Support, f)
	}
	e, ok := LookupExtension("prxy")
	require.True(t, ok)
	assert.Equal(t, Uncertain, e.Support)
	e, ok = LookupExtension("OWNZ")
	require.True(t, ok)
	assert.Equal(t, Supported, e.Support)
	assert.Equal(t, ValueString, e.Value)
	_, ok = LookupExtension("QQQQ")
	assert.False(t, ok)
	_, ok = LookupExtension("OWN")
	assert.False(t, ok)

	assert.Equal(t, "supported", Supported.String())
	assert.Equal(t, "uncertain", Uncertain.String())
	assert.Equal(t, "unsupported", Unsupported.String())
	assert.Equal(t, "int", ValueInt.String())
	assert.Equal(t, "float", ValueFloat.String())
	assert.Equal(t, "string", ValueString.String())
	assert.Equal(t, "list", ValueList.String())
	assert.Equal(t, "none", ValueNone.String())
}
