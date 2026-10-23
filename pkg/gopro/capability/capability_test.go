package capability

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMatrix(t *testing.T) {
	t.Parallel()
	all := All()
	require.NotEmpty(t, all)
	for i := 1; i < len(all); i++ {
		assert.Less(t, all[i-1].Op, all[i].Op, "All must be sorted")
	}
	info, ok := Lookup(OpOGPGetState)
	require.True(t, ok)
	assert.Equal(t, "/gopro/camera/state", info.Path)
	assert.Equal(t, Verified, info.Level)
	assert.True(t, info.InSpec)

	// Hardware-verified override of a spec omission.
	assert.Equal(t, Verified, Support(OpOGPShutter))
	// Spec lists it, hardware returns 404 → override to unsupported.
	assert.Equal(t, Unsupported, Support(OpOGPMediaInfo))
	// HERO13-only feature.
	assert.Equal(t, Unsupported, Support(OpOGPLabsQrCode))
	// Legacy catalogue entries exist.
	legacy, ok := Lookup(OpGPInfo)
	require.True(t, ok)
	assert.True(t, legacy.Legacy)
	assert.Equal(t, "/gp/gpControl/info", legacy.Path)
	// Unknown op.
	assert.Equal(t, Unsupported, Support(Op("NOPE")))
	_, ok = Lookup(Op("NOPE"))
	assert.False(t, ok)
}

func TestCheck(t *testing.T) {
	t.Parallel()
	require.NoError(t, Check(OpOGPGetState))
	err := Check(OpOGPMediaInfo)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrUnsupported)
	var ue *UnsupportedError
	require.ErrorAs(t, err, &ue)
	assert.Equal(t, OpOGPMediaInfo, ue.Op)
	assert.Contains(t, err.Error(), "404")
	assert.Contains(t, (&UnsupportedError{Op: "X"}).Error(), "X is not supported")
}

func TestLevelString(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "verified", Verified.String())
	assert.Equal(t, "spec", Spec.String())
	assert.Equal(t, "unsupported", Unsupported.String())
	b, err := Spec.MarshalText()
	require.NoError(t, err)
	assert.Equal(t, "spec", string(b))
	assert.NotEmpty(t, SpecVersion)
	assert.NotEmpty(t, FirmwareTested)
}
