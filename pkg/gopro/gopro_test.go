package gopro_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kazilotus/gopro9-cli/internal/fakecam"
	"github.com/kazilotus/gopro9-cli/pkg/gopro"
)

func TestFacade(t *testing.T) {
	t.Parallel()
	cam := fakecam.New(t)
	c := gopro.New(cam.Endpoint(), gopro.WithTimeout(time.Second), gopro.WithRetries(0), gopro.WithLogger(slog.New(slog.DiscardHandler)), gopro.WithUserAgent("t"))
	v, err := c.OGP.Version(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "2.0", v.Version)
	info, err := c.Legacy.Info(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "HERO9 Black", info.ModelName)
	assert.Equal(t, cam.Endpoint(), c.Endpoint)
}
