//go:build e2e

// Package e2e runs against a real HERO9 Black connected over USB.
//
//	GOPRO9_E2E=1 go test -tags e2e ./test/e2e/...
//
// Read-only by default. Set GOPRO9_E2E_MUTATING=1 to also run tests that
// change camera state (settings are snapshotted and restored). Nothing here
// ever deletes media or uploads firmware.
package e2e

import (
	"context"
	"io"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kazilotus/gopro9-cli/pkg/gopro"
	"github.com/kazilotus/gopro9-cli/pkg/gopro/discovery"
	"github.com/kazilotus/gopro9-cli/pkg/gopro/gpmf"
	"github.com/kazilotus/gopro9-cli/pkg/gopro/presets"
	"github.com/kazilotus/gopro9-cli/pkg/gopro/stream"
	"github.com/kazilotus/gopro9-cli/pkg/gopro/transport"
)

func connect(t *testing.T) *gopro.Camera {
	t.Helper()
	if os.Getenv("GOPRO9_E2E") == "" {
		t.Skip("GOPRO9_E2E not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if host := os.Getenv("GOPRO9_HOST"); host != "" {
		port := 8080
		if p, err := strconv.Atoi(os.Getenv("GOPRO9_PORT")); err == nil {
			port = p
		}
		return gopro.New(transport.Endpoint{Host: host, Port: port})
	}
	found, err := discovery.Discover(ctx, discovery.Options{MDNSTimeout: 1500 * time.Millisecond})
	require.NoError(t, err)
	require.NotEmpty(t, found)
	t.Logf("camera: %+v", found[0])
	return gopro.New(found[0].Endpoint())
}

func TestDiscoveryFindsCameraWithMDNS(t *testing.T) {
	if os.Getenv("GOPRO9_E2E") == "" {
		t.Skip("GOPRO9_E2E not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	found, err := discovery.Discover(ctx, discovery.Options{MDNSTimeout: 2 * time.Second})
	require.NoError(t, err)
	require.NotEmpty(t, found)
	c := found[0]
	assert.Equal(t, "2.0", c.APIVersion)
	assert.NotEmpty(t, c.Interface)
	assert.Contains(t, c.Firmware, "HD9.01.01", "mDNS TXT should carry firmware")
	assert.NotEmpty(t, c.Serial)
}

func TestReadOnlyQueries(t *testing.T) {
	cam := connect(t)
	ctx := context.Background()

	v, err := cam.OGP.Version(ctx)
	require.NoError(t, err)
	assert.Equal(t, "2.0", v.Version)

	st, err := cam.OGP.State(ctx)
	require.NoError(t, err)
	assert.True(t, st.Status.BatteryPresent)
	assert.Positive(t, st.Status.InternalBatteryPercentage)
	assert.True(t, st.Settings.VideoResolution.Valid(), "resolution %v", st.Settings.VideoResolution)
	assert.True(t, st.Settings.FramesPerSecond.Valid())

	info, err := cam.Legacy.Info(ctx)
	require.NoError(t, err)
	assert.Equal(t, "HERO9 Black", info.ModelName)
	t.Logf("firmware %s labs=%v", info.FirmwareVersion, info.IsLabs())

	ps, err := cam.OGP.Presets(ctx)
	require.NoError(t, err)
	assert.Len(t, ps.Groups, 3)

	dt, err := cam.OGP.GetDateTime(ctx)
	require.NoError(t, err)
	parsed, err := dt.Parse(time.Local)
	require.NoError(t, err)
	assert.WithinDuration(t, time.Now(), parsed, 10*time.Minute, "camera clock drift")

	require.NoError(t, cam.OGP.KeepAlive(ctx))
}

func TestMediaReadOnly(t *testing.T) {
	cam := connect(t)
	ctx := context.Background()
	list, err := cam.Media.List(ctx)
	require.NoError(t, err)
	if len(list.Files) == 0 {
		t.Skip("no media on card")
	}
	var video string
	for _, f := range list.Files {
		if f.Kind == "video" {
			video = f.Path()
			break
		}
	}
	require.NotEmpty(t, video, "need at least one video")

	size, err := cam.Media.Size(ctx, video)
	require.NoError(t, err)
	assert.Positive(t, size)

	thumb, err := cam.Media.Thumbnail(ctx, video)
	require.NoError(t, err)
	assert.Equal(t, []byte{0xFF, 0xD8}, thumb[:2], "JPEG")

	hdr, err := cam.Media.GPMF(ctx, video)
	require.NoError(t, err)
	assert.Equal(t, "DEVC", string(hdr[:4]))

	tele, err := cam.Media.Telemetry(ctx, video)
	require.NoError(t, err)
	parsed, err := gpmf.ParseTelemetry(tele)
	require.NoError(t, err)
	assert.Positive(t, parsed.Payloads)
	_, hasAccl := parsed.Stream("ACCL")
	assert.True(t, hasAccl)

	tree, err := cam.Media.Tree(ctx, "")
	require.NoError(t, err)
	assert.NotEmpty(t, tree)

	st, err := cam.Webcam.Status(ctx)
	require.NoError(t, err)
	t.Logf("webcam status %s error %s", st.Status, st.Error)
	v, err := cam.Webcam.Version(ctx)
	require.NoError(t, err)
	assert.Equal(t, 4, v.Version)
}

func TestMutatingStatePreserving(t *testing.T) {
	cam := connect(t)
	if os.Getenv("GOPRO9_E2E_MUTATING") == "" {
		t.Skip("GOPRO9_E2E_MUTATING not set")
	}
	ctx := context.Background()
	st, err := cam.OGP.State(ctx)
	require.NoError(t, err)
	require.False(t, st.Status.Encoding, "camera must be idle")

	// Re-assert the current resolution: exercises the setting path without changing state.
	require.NoError(t, cam.OGP.SetSetting(ctx, st.Settings.VideoResolution))

	// Turbo on/off leaves the camera as found.
	require.NoError(t, cam.OGP.TurboTransfer(ctx, true))
	require.NoError(t, cam.OGP.TurboTransfer(ctx, false))

	// Preset group round trip.
	cur := st.Status.PresetGroup
	for _, g := range presets.Groups() {
		if id, _ := g.ID(); int64(id) == cur {
			require.NoError(t, cam.OGP.SetPresetGroup(ctx, g))
		}
	}

	// Preview stream start/stop with a few packets received.
	conn, err := stream.Listen(stream.DefaultPort)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()
	require.NoError(t, cam.Stream.Start(ctx, 0))
	stats, err := stream.Receive(ctx, conn, io.Discard, stream.ReceiveOptions{MaxDuration: 2 * time.Second, IdleTimeout: 5 * time.Second})
	require.NoError(t, cam.Stream.Stop(ctx))
	require.NoError(t, err)
	assert.Positive(t, stats.Packets)
	t.Logf("preview stream: %d packets, %.2f Mbit/s", stats.Packets, stats.Bitrate()/1e6)
}
