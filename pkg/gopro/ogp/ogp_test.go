package ogp_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kazilotus/gopro9-cli/internal/fakecam"
	"github.com/kazilotus/gopro9-cli/pkg/gopro/ogp"
	"github.com/kazilotus/gopro9-cli/pkg/gopro/presets"
	"github.com/kazilotus/gopro9-cli/pkg/gopro/settings"
	"github.com/kazilotus/gopro9-cli/pkg/gopro/transport"
)

func newClient(t *testing.T) (*ogp.Client, *fakecam.Server) {
	t.Helper()
	cam := fakecam.New(t)
	return ogp.New(transport.New(cam.Endpoint(), transport.WithRetries(0, 0))), cam
}

func TestVersionAndState(t *testing.T) {
	t.Parallel()
	c, _ := newClient(t)
	ctx := context.Background()
	v, err := c.Version(ctx)
	require.NoError(t, err)
	assert.Equal(t, "2.0", v.Version)

	st, err := c.State(ctx)
	require.NoError(t, err)
	assert.True(t, st.Status.BatteryPresent)
	assert.False(t, st.Status.Encoding)
	assert.Equal(t, settings.VideoResolution2_7K, st.Settings.VideoResolution)
}

func TestKeepAliveAndAnalytics(t *testing.T) {
	t.Parallel()
	c, cam := newClient(t)
	ctx := context.Background()
	require.NoError(t, c.KeepAlive(ctx))
	require.NoError(t, c.KeepAlive(ctx))
	require.NoError(t, c.SetAnalyticsClientInfo(ctx))
	assert.Equal(t, 2, cam.KeepAlives)
}

func TestShutter(t *testing.T) {
	t.Parallel()
	c, cam := newClient(t)
	ctx := context.Background()
	require.NoError(t, c.Shutter(ctx, ogp.ShutterStart))
	assert.True(t, cam.Recording)
	st, err := c.State(ctx)
	require.NoError(t, err)
	assert.True(t, st.Status.Encoding)
	err = c.Shutter(ctx, ogp.ShutterStart)
	require.ErrorIs(t, err, transport.ErrServer, "double start is rejected by the camera")
	require.NoError(t, c.Shutter(ctx, ogp.ShutterStop))
	assert.False(t, cam.Recording)
	require.Error(t, c.Shutter(ctx, ogp.ShutterAction("pause")))
}

func TestSettings(t *testing.T) {
	t.Parallel()
	c, cam := newClient(t)
	ctx := context.Background()
	require.NoError(t, c.SetSetting(ctx, settings.VideoResolution1080))
	assert.Equal(t, int64(9), cam.Settings[2])

	opt, err := c.GetSetting(ctx, settings.SettingVideoResolution)
	require.NoError(t, err)
	assert.Equal(t, "1080", opt.String())

	err = c.SetSetting(ctx, settings.VideoResolution(250))
	require.ErrorIs(t, err, transport.ErrForbidden)

	// Undocumented setting still surfaces its raw value.
	raw, err := c.GetSetting(ctx, settings.SettingID(169))
	require.NoError(t, err)
	assert.Equal(t, settings.SettingID(169), raw.ID())
	assert.Equal(t, "1", raw.String())
	assert.Equal(t, uint32(1), raw.Value())

	_, err = c.GetSetting(ctx, settings.SettingID(9999))
	require.Error(t, err)
}

func TestPresets(t *testing.T) {
	t.Parallel()
	c, cam := newClient(t)
	ctx := context.Background()
	st, err := c.Presets(ctx)
	require.NoError(t, err)
	require.Len(t, st.Groups, 3)
	require.NoError(t, c.LoadPreset(ctx, st.Groups[1].Presets[0].ID))
	assert.Equal(t, uint32(262144), cam.PresetLoaded)
	require.ErrorIs(t, c.LoadPreset(ctx, 42), transport.ErrForbidden)
	require.NoError(t, c.SetPresetGroup(ctx, presets.GroupTimelapse))
	assert.Equal(t, uint32(1002), cam.PresetGroup)
	require.Error(t, c.SetPresetGroup(ctx, presets.Group("bogus")))
}

func TestZoomTurbo(t *testing.T) {
	t.Parallel()
	c, cam := newClient(t)
	ctx := context.Background()
	require.NoError(t, c.DigitalZoom(ctx, 40))
	assert.Equal(t, 40, cam.Zoom)
	require.Error(t, c.DigitalZoom(ctx, 101))
	require.NoError(t, c.TurboTransfer(ctx, true))
	assert.True(t, cam.Turbo)
	require.NoError(t, c.TurboTransfer(ctx, false))
	assert.False(t, cam.Turbo)
}

func TestDateTime(t *testing.T) {
	t.Parallel()
	fixed := time.Date(2026, 10, 23, 8, 46, 34, 0, time.UTC)
	cam := fakecam.New(t, fakecam.WithClock(func() time.Time { return fixed }))
	c := ogp.New(transport.New(cam.Endpoint()))
	ctx := context.Background()
	dt, err := c.GetDateTime(ctx)
	require.NoError(t, err)
	assert.Equal(t, "2026_10_23", dt.Date)
	parsed, err := dt.Parse(time.UTC)
	require.NoError(t, err)
	assert.True(t, parsed.Equal(fixed))

	want := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	require.NoError(t, c.SetDateTime(ctx, want))
	dt, err = c.GetDateTime(ctx)
	require.NoError(t, err)
	assert.Equal(t, "2030_01_02", dt.Date)
	assert.Equal(t, "03_04_05", dt.Time)
	_, err = ogp.DateTime{Date: "x", Time: "y"}.Parse(time.UTC)
	require.Error(t, err)
}

func TestHilights(t *testing.T) {
	t.Parallel()
	c, cam := newClient(t)
	ctx := context.Background()
	const f = "100GOPRO/GH010397.MP4"
	require.NoError(t, c.HilightFile(ctx, f, 12500*time.Millisecond))
	assert.Equal(t, []int64{12500}, cam.Hilights[f])
	require.NoError(t, c.HilightRemove(ctx, f, 12500*time.Millisecond))
	assert.Empty(t, cam.Hilights[f])
	require.Error(t, c.HilightFile(ctx, "", 0))
	require.ErrorIs(t, c.HilightFile(ctx, "100GOPRO/NOPE.MP4", 0), transport.ErrNotFound)
	require.ErrorIs(t, c.HilightMoment(ctx), transport.ErrNotFound, "not available on HERO9")
}

func TestHostHeaderGuard(t *testing.T) {
	t.Parallel()
	cam := fakecam.New(t)
	ep := cam.Endpoint()
	// Point the transport at the server by a hostname the camera does not recognise.
	bad := transport.New(transport.Endpoint{Host: "evil.example", Port: ep.Port}, transport.WithHTTPClient(cam.Client()), transport.WithRetries(0, 0))
	_, err := ogp.New(bad).Version(context.Background())
	require.Error(t, err)
}
