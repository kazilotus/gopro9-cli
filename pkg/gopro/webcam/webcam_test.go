package webcam_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kazilotus/gopro9-cli/internal/fakecam"
	"github.com/kazilotus/gopro9-cli/pkg/gopro/transport"
	"github.com/kazilotus/gopro9-cli/pkg/gopro/webcam"
)

func TestLifecycle(t *testing.T) {
	t.Parallel()
	cam := fakecam.New(t)
	cam.StreamPackets = 0 // no UDP side effects here
	c := webcam.New(transport.New(cam.Endpoint()))
	ctx := context.Background()

	st, err := c.Status(ctx)
	require.NoError(t, err)
	assert.Equal(t, webcam.StatusIdle, st.Status)
	assert.Equal(t, webcam.ErrNone, st.Error)

	v, err := c.Version(ctx)
	require.NoError(t, err)
	assert.Equal(t, 4, v.Version)
	assert.True(t, v.USB31Compatible)

	require.NoError(t, c.Start(ctx, webcam.StartOptions{Resolution: webcam.Res1080, FOV: webcam.FOVLinear, HasFOV: true, Port: 9000}))
	assert.Equal(t, 2, cam.WebcamStatus)
	reqs := strings.Join(cam.Snapshot(), "\n")
	assert.Contains(t, reqs, "/gopro/webcam/start?res=12&fov=4&port=9000")

	require.NoError(t, c.Preview(ctx))
	assert.Equal(t, 3, cam.WebcamStatus)
	require.NoError(t, c.Stop(ctx))
	assert.Equal(t, 1, cam.WebcamStatus)
	require.NoError(t, c.Exit(ctx))
	assert.Equal(t, 0, cam.WebcamStatus)
	st, err = c.Status(ctx)
	require.NoError(t, err)
	assert.Equal(t, webcam.StatusOff, st.Status)

	// Bare start sends no params.
	require.NoError(t, c.Start(ctx, webcam.StartOptions{}))
	all := cam.Snapshot()
	assert.Equal(t, "GET /gopro/webcam/start", all[len(all)-1])
}

func TestEnums(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "streaming", webcam.StatusHighPowerPreview.String())
	assert.Equal(t, "7", webcam.Status(7).String())
	b, err := webcam.StatusOff.MarshalText()
	require.NoError(t, err)
	assert.Equal(t, "off", string(b))
	assert.Equal(t, "invalid param", webcam.ErrInvalidParam.String())
	assert.Equal(t, "99", webcam.Error(99).String())
	b, err = webcam.ErrShutter.MarshalText()
	require.NoError(t, err)
	assert.Equal(t, "shutter", string(b))
	for _, e := range []webcam.Error{webcam.ErrNone, webcam.ErrSetPreset, webcam.ErrSetWindowSize, webcam.ErrExecStream, webcam.ErrComTimeout, webcam.ErrUnavailable, webcam.ErrExit} {
		assert.NotEmpty(t, e.String())
	}
	for _, s := range []webcam.Status{webcam.StatusOff, webcam.StatusIdle, webcam.StatusLowPowerPreview} {
		assert.NotEmpty(t, s.String())
	}

	r, err := webcam.ParseResolution("1080p")
	require.NoError(t, err)
	assert.Equal(t, webcam.Res1080, r)
	assert.Equal(t, "1080", r.String())
	assert.Equal(t, "480|720|1080", r.Type())
	assert.Equal(t, "99", webcam.Resolution(99).String())
	var rf webcam.Resolution
	require.NoError(t, rf.Set("480"))
	assert.Equal(t, webcam.Res480, rf)
	require.NoError(t, rf.Set("720"))
	require.Error(t, rf.Set("4k"))

	f, err := webcam.ParseFOV("Linear")
	require.NoError(t, err)
	assert.Equal(t, webcam.FOVLinear, f)
	for _, in := range []string{"wide", "n", "superview"} {
		_, err := webcam.ParseFOV(in)
		require.NoError(t, err, in)
	}
	assert.Equal(t, "superview", webcam.FOVSuperview.String())
	assert.Equal(t, "narrow", webcam.FOVNarrow.String())
	assert.Equal(t, "9", webcam.FOV(9).String())
	assert.Equal(t, "wide|narrow|superview|linear", f.Type())
	var ff webcam.FOV
	require.NoError(t, ff.Set("wide"))
	require.Error(t, ff.Set("fisheye"))
}
