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
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kazilotus/gopro9-cli/pkg/gopro"
	"github.com/kazilotus/gopro9-cli/pkg/gopro/discovery"
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
