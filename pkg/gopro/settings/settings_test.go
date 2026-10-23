package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistryIsConsistent(t *testing.T) {
	t.Parallel()
	all := All()
	require.NotEmpty(t, all)
	seenID, seenName := map[SettingID]bool{}, map[string]bool{}
	for _, d := range all {
		assert.False(t, seenID[d.ID], "duplicate id %d", d.ID)
		assert.False(t, seenName[d.Name], "duplicate name %s", d.Name)
		seenID[d.ID], seenName[d.Name] = true, true
		assert.NotEmpty(t, d.Title)
		if d.Numeric {
			assert.Empty(t, d.Options)
		} else {
			assert.NotEmpty(t, d.Options, "%s has no options", d.Name)
		}
		// Every documented option must round-trip through Parse/FromValue/String.
		for _, o := range d.Options {
			opt, ok := d.FromValue(o.Value)
			require.True(t, ok, "%s value %d", d.Name, o.Value)
			assert.Equal(t, o.Value, opt.Value())
			assert.Equal(t, d.ID, opt.ID())
			assert.Equal(t, o.Label, opt.String())
			byLabel, err := d.Parse(o.Label)
			require.NoError(t, err)
			assert.Equal(t, opt, byLabel)
			byName, err := d.Parse(o.Name)
			require.NoError(t, err)
			assert.Equal(t, opt, byName)
			byNum, err := d.Parse(json.Number(itoa(o.Value)).String())
			require.NoError(t, err)
			assert.Equal(t, opt, byNum)
		}
	}
}

func itoa(v uint32) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func TestLookup(t *testing.T) {
	t.Parallel()
	for _, key := range []string{"video-resolution", "Video Resolution", "VIDEO_RESOLUTION", "2"} {
		d, ok := Lookup(key)
		require.True(t, ok, key)
		assert.Equal(t, SettingVideoResolution, d.ID)
	}
	_, ok := Lookup("does-not-exist")
	assert.False(t, ok)
	_, ok = Lookup("9999")
	assert.False(t, ok)
	d, ok := ByID(SettingFramesPerSecond)
	require.True(t, ok)
	assert.Equal(t, "frames-per-second", d.Name)
}

func TestVideoResolutionTyped(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "4K", VideoResolution4K.String())
	assert.Equal(t, SettingVideoResolution, VideoResolution4K.ID())
	assert.True(t, VideoResolution4K.Valid())
	assert.False(t, VideoResolution(250).Valid())
	assert.Equal(t, "250", VideoResolution(250).String())

	for _, in := range []string{"4k", "4K", " 4K ", "1", " 1"} {
		v, err := ParseVideoResolution(in)
		require.NoError(t, err, in)
		assert.Equal(t, VideoResolution4K, v)
	}
	v, err := ParseVideoResolution("2.7k 4:3")
	require.NoError(t, err)
	assert.Equal(t, VideoResolution2_7K4x3, v)
	v, err = ParseVideoResolution("2.7k-4-3")
	require.NoError(t, err)
	assert.Equal(t, VideoResolution2_7K4x3, v)
	// Numeric-looking labels resolve as labels, not wire values: "1080" is the
	// 1080p label (wire value 9), not option 1080.
	v, err = ParseVideoResolution("1080")
	require.NoError(t, err)
	assert.Equal(t, VideoResolution1080, v)
	assert.Equal(t, uint32(9), v.Value())

	_, err = ParseVideoResolution("8k") // HERO13-only option is rejected for HERO9
	require.Error(t, err)
	_, err = ParseVideoResolution("31")
	require.Error(t, err)

	var flag VideoResolution
	require.NoError(t, flag.Set("1080"))
	assert.Equal(t, VideoResolution1080, flag)
	assert.Equal(t, "video-resolution", flag.Type())
	txt, err := flag.MarshalText()
	require.NoError(t, err)
	assert.Equal(t, "1080", string(txt))
	var back VideoResolution
	require.NoError(t, back.UnmarshalText(txt))
	assert.Equal(t, flag, back)
}

func TestNumericSetting(t *testing.T) {
	t.Parallel()
	v, err := ParseLCDBrightness("50")
	require.NoError(t, err)
	assert.Equal(t, LCDBrightness(50), v)
	assert.True(t, v.Valid())
	_, err = ParseLCDBrightness("5")
	require.Error(t, err, "below documented minimum")
	_, err = ParseLCDBrightness("abc")
	require.Error(t, err)
	assert.Equal(t, "50", v.String())
	assert.Equal(t, uint32(50), v.Value())
	assert.Equal(t, SettingLCDBrightness, v.ID())
}

func TestSettingIDString(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "video-resolution", SettingVideoResolution.String())
	assert.Equal(t, "setting-9999", SettingID(9999).String())
	b, err := SettingID(3).MarshalText()
	require.NoError(t, err)
	assert.Equal(t, "frames-per-second", string(b))
}

func TestValuesFromFixture(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "fixtures", "hero9_HD9.01.01.72.70", "gopro_camera_state.resp"))
	require.NoError(t, err)
	var state struct {
		Settings Values `json:"settings"`
	}
	require.NoError(t, json.Unmarshal(raw, &state))
	s := state.Settings
	// Recorded camera state: 2.7K video, 60 fps, GPS off, LCD brightness 100.
	assert.Equal(t, VideoResolution2_7K, s.VideoResolution)
	assert.Equal(t, FramesPerSecond24, s.FramesPerSecond)
	assert.Equal(t, LCDBrightness(100), s.LCDBrightness)
	assert.True(t, s.VideoResolution.Valid())
	assert.NotEmpty(t, s.Unknown, "firmware exposes settings the spec does not document for HERO9")
	_, known := s.Unknown[SettingVideoResolution]
	assert.False(t, known)

	opt, ok := s.Get(SettingVideoResolution)
	require.True(t, ok)
	assert.Equal(t, "2.7K", opt.String())
	_, ok = s.Get(SettingID(9999))
	assert.False(t, ok)

	// JSON round trip keeps typed labels readable.
	out, err := json.Marshal(s)
	require.NoError(t, err)
	assert.Contains(t, string(out), `"video-resolution":"2.7K"`)
}

func TestValuesUnmarshalErrors(t *testing.T) {
	t.Parallel()
	var v Values
	require.Error(t, v.UnmarshalJSON([]byte(`[]`)))
	require.Error(t, v.UnmarshalJSON([]byte(`{"x":1}`)))
	require.Error(t, v.UnmarshalJSON([]byte(`{"2":1.5}`)))
	require.NoError(t, v.UnmarshalJSON([]byte(`{"2":9}`)))
	assert.Equal(t, VideoResolution1080, v.VideoResolution)
}
