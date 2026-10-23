package gpcontrol_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kazilotus/gopro9-cli/internal/fakecam"
	"github.com/kazilotus/gopro9-cli/pkg/gopro/gpcontrol"
	"github.com/kazilotus/gopro9-cli/pkg/gopro/transport"
)

func TestLegacy(t *testing.T) {
	t.Parallel()
	cam := fakecam.New(t)
	c := gpcontrol.New(transport.New(cam.Endpoint()))
	ctx := context.Background()

	info, err := c.Info(ctx)
	require.NoError(t, err)
	assert.Equal(t, "HERO9 Black", info.ModelName)
	assert.Equal(t, "HD9.01.01.72.00", info.FirmwareVersion)
	assert.False(t, info.IsLabs())
	assert.True(t, gpcontrol.Info{FirmwareVersion: "HD9.01.01.72.70"}.IsLabs())
	assert.False(t, gpcontrol.Info{FirmwareVersion: "x"}.IsLabs())
	assert.Equal(t, []string{"ble"}, info.RequiredNetworks)

	st, err := c.Status(ctx)
	require.NoError(t, err)
	assert.True(t, st.Status.BatteryPresent)

	require.NoError(t, c.Locate(ctx, true))
	assert.True(t, cam.Locate)
	require.NoError(t, c.Locate(ctx, false))
	assert.False(t, cam.Locate)
	require.NoError(t, c.Sleep(ctx))
	assert.True(t, cam.Sleeping)

	_, err = c.AnalyticsBlob(ctx)
	require.Error(t, err, "fakecam does not serve the analytics blob")
}
