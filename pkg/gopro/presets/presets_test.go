package presets

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGroup(t *testing.T) {
	t.Parallel()
	id, ok := GroupVideo.ID()
	require.True(t, ok)
	assert.Equal(t, uint32(1000), id)
	_, ok = Group("x").ID()
	assert.False(t, ok)
	assert.Equal(t, "photo", GroupPhoto.String())
	assert.Equal(t, "x", Group("x").String())
	for _, in := range []string{"video", "VIDEO", "PRESET_GROUP_ID_VIDEO", "1000"} {
		g, err := ParseGroup(in)
		require.NoError(t, err, in)
		assert.Equal(t, GroupVideo, g)
	}
	_, err := ParseGroup("burst")
	require.Error(t, err)
	var g Group
	require.NoError(t, g.Set("timelapse"))
	assert.Equal(t, GroupTimelapse, g)
	assert.Equal(t, "video|photo|timelapse", g.Type())
	b, err := g.MarshalText()
	require.NoError(t, err)
	assert.Equal(t, "timelapse", string(b))
	require.Error(t, g.UnmarshalText([]byte("nope")))
	assert.Len(t, Groups(), 3)
}

func TestStatusFixture(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "fixtures", "hero9_HD9.01.01.72.70", "gopro_camera_presets_get.resp"))
	require.NoError(t, err)
	var st Status
	require.NoError(t, json.Unmarshal(raw, &st))
	require.Len(t, st.Groups, 3)
	assert.Equal(t, GroupVideo, st.Groups[0].ID)
	p := st.Groups[0].Presets[0]
	assert.Equal(t, uint32(196608), p.ID)
	assert.Equal(t, "Max Video", p.Title())
	assert.NotEmpty(t, p.Settings)

	for _, key := range []string{"196608", "PRESET_TITLE_MAX_VIDEO", "max video", "Max-Video"} {
		found, g, ok := st.Find(key)
		require.True(t, ok, key)
		assert.Equal(t, p.ID, found.ID)
		assert.Equal(t, GroupVideo, g)
	}
	_, _, ok := st.Find("nope")
	assert.False(t, ok)
}
