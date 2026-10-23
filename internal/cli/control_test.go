package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kazilotus/gopro9-cli/internal/fakecam"
)

func TestShutter(t *testing.T) {
	cam := fakecam.New(t)
	out, _, code := runCam(t, cam, "shutter", "start")
	require.Equal(t, 0, code)
	assert.Contains(t, out, "start")
	assert.True(t, cam.Recording)

	_, errOut, code := runCam(t, cam, "shutter", "start")
	assert.Equal(t, 75, code, "double start → camera busy")
	assert.Contains(t, errOut, "refused")

	_, _, code = runCam(t, cam, "shutter", "toggle")
	require.Equal(t, 0, code)
	assert.False(t, cam.Recording)
	_, _, code = runCam(t, cam, "shutter", "toggle")
	require.Equal(t, 0, code)
	assert.True(t, cam.Recording)
	out, _, code = runCam(t, cam, "shutter", "stop", "--output", "json")
	require.Equal(t, 0, code)
	assert.Contains(t, out, `"detail": "stop"`)

	_, _, code = runCam(t, cam, "shutter", "pause")
	assert.Equal(t, 64, code)
}

func TestCaptureVideo(t *testing.T) {
	cam := fakecam.New(t)
	cam.Status[96] = []byte("1001") // currently in photo group
	out, errOut, code := runCam(t, cam, "capture", "video", "--duration", "120ms", "--output", "json")
	require.Equal(t, 0, code, errOut)
	assert.Contains(t, out, `"mode": "video"`)
	assert.False(t, cam.Recording)
	assert.Equal(t, uint32(1000), cam.PresetGroup, "switched to video group first")
	reqs := strings.Join(cam.Snapshot(), "\n")
	assert.Contains(t, reqs, "/gopro/camera/shutter/start")
	assert.Contains(t, reqs, "/gopro/camera/shutter/stop")

	_, errOut, code = runCam(t, cam, "capture", "video", "--duration", "0")
	assert.Equal(t, 64, code)
	assert.Contains(t, errOut, "positive")
}

func TestCaptureStopsOnInterrupt(t *testing.T) {
	cam := fakecam.New(t)
	ep := cam.Endpoint()
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	var out, errb bytes.Buffer
	app := NewApp(Stdio{In: strings.NewReader(""), Out: &out, Err: &errb}, WithClock(func() time.Time { return fixedNow }))
	code := app.Run(ctx, []string{"--host", ep.Host, "--port", strconv.Itoa(ep.Port), "capture", "video", "--duration", "1h"})
	assert.Equal(t, 0, code, errb.String())
	assert.False(t, cam.Recording, "shutter stopped on interrupt")
}

func TestCapturePhotoAndTimelapse(t *testing.T) {
	cam := fakecam.New(t)
	out, _, code := runCam(t, cam, "capture", "photo", "--output", "plain")
	require.Equal(t, 0, code)
	assert.Equal(t, "photo\n", out)
	assert.Equal(t, uint32(1001), cam.PresetGroup)
	cam.Recording = false
	_, _, code = runCam(t, cam, "capture", "timelapse", "--duration", "50ms")
	require.Equal(t, 0, code)
	assert.Equal(t, uint32(1002), cam.PresetGroup)

	cam.Recording = true
	_, errOut, code := runCam(t, cam, "capture", "photo")
	assert.Equal(t, 75, code)
	assert.Contains(t, errOut, "already recording")
}

func TestPreset(t *testing.T) {
	cam := fakecam.New(t)
	out, _, code := runCam(t, cam, "preset", "list")
	require.Equal(t, 0, code)
	golden(t, "preset_list", out)

	out, _, code = runCam(t, cam, "preset", "load", "max photo")
	require.Equal(t, 0, code)
	assert.Contains(t, out, "262144")
	assert.Equal(t, uint32(262144), cam.PresetLoaded)

	_, errOut, code := runCam(t, cam, "preset", "load", "nope")
	assert.Equal(t, 64, code)
	assert.Contains(t, errOut, "no preset matches")

	_, _, code = runCam(t, cam, "preset", "group", "timelapse")
	require.Equal(t, 0, code)
	assert.Equal(t, uint32(1002), cam.PresetGroup)
	_, _, code = runCam(t, cam, "preset", "group", "burst")
	assert.Equal(t, 64, code)
}

func TestSettingCommands(t *testing.T) {
	cam := fakecam.New(t)
	out, _, code := runCam(t, cam, "setting", "list")
	require.Equal(t, 0, code)
	golden(t, "setting_list", out)

	out, _, code = runCam(t, cam, "setting", "list", "--all", "--output", "json")
	require.Equal(t, 0, code)
	assert.Contains(t, out, "undocumented")

	out, _, code = runCam(t, cam, "setting", "get", "video-resolution", "--output", "plain")
	require.Equal(t, 0, code)
	assert.Equal(t, "2.7K\n", out)

	out, _, code = runCam(t, cam, "setting", "set", "video-resolution", "1080")
	require.Equal(t, 0, code)
	assert.Contains(t, out, "1080")
	assert.Equal(t, int64(9), cam.Settings[2])

	_, _, code = runCam(t, cam, "setting", "set", "2", "4K")
	require.Equal(t, 0, code)
	assert.Equal(t, int64(1), cam.Settings[2])

	_, errOut, code := runCam(t, cam, "setting", "set", "video-resolution", "8K")
	assert.Equal(t, 0, code, "documented for other cameras: allowed with a warning")
	assert.Contains(t, errOut, "not listed for the HERO9")
	_, errOut, code = runCam(t, cam, "setting", "set", "video-resolution", "999")
	assert.Equal(t, 64, code)
	assert.Contains(t, errOut, "valid: 4K")

	_, errOut, code = runCam(t, cam, "setting", "set", "bogus", "1")
	assert.Equal(t, 64, code)
	assert.Contains(t, errOut, "unknown setting")

	cam.FailWith["/gopro/camera/setting"] = 403
	_, _, code = runCam(t, cam, "setting", "set", "fps", "60")
	assert.Equal(t, 64, code, "fps is not a known name")
	_, errOut, code = runCam(t, cam, "setting", "set", "frames-per-second", "60")
	assert.Equal(t, 75, code)
	assert.Contains(t, errOut, "not valid in the current mode")
	delete(cam.FailWith, "/gopro/camera/setting")

	out, _, code = runCam(t, cam, "setting", "describe", "video-lens", "--output", "json")
	require.Equal(t, 0, code)
	assert.Contains(t, out, `"label": "Linear"`)
	out, _, code = runCam(t, cam, "setting", "describe", "lcd-brightness")
	require.Equal(t, 0, code)
	assert.Contains(t, out, "numeric: true")
}

func TestSettingDumpApply(t *testing.T) {
	cam := fakecam.New(t)
	dumped, _, code := runCam(t, cam, "setting", "dump")
	require.Equal(t, 0, code)
	golden(t, "setting_dump", dumped)

	dir := t.TempDir()
	file := filepath.Join(dir, "s.yaml")
	require.NoError(t, os.WriteFile(file, []byte("video-resolution: \"1080\"\nframes-per-second: \"60\"\n"), 0o600))
	out, errOut, code := runCam(t, cam, "setting", "apply", file)
	require.Equal(t, 0, code, errOut)
	assert.Equal(t, int64(9), cam.Settings[2])
	assert.Equal(t, int64(5), cam.Settings[3])
	assert.Contains(t, out, "frames-per-second")

	// Round trip: dump → apply is a no-op that succeeds.
	require.NoError(t, os.WriteFile(file, []byte(dumped), 0o600))
	_, errOut, code = runCam(t, cam, "setting", "apply", file)
	assert.Equal(t, 0, code, "round-trip apply: %s\ninput:\n%s", errOut, dumped)

	require.NoError(t, os.WriteFile(file, []byte("video-resolution: \"999\"\n"), 0o600))
	_, errOut, code = runCam(t, cam, "setting", "apply", file)
	assert.Equal(t, 64, code)
	assert.Contains(t, errOut, "video-resolution")

	require.NoError(t, os.WriteFile(file, []byte("nope: 1\n"), 0o600))
	_, _, code = runCam(t, cam, "setting", "apply", file)
	assert.Equal(t, 64, code)

	require.NoError(t, os.WriteFile(file, []byte("- not\n- a map\n"), 0o600))
	_, errOut, code = runCam(t, cam, "setting", "apply", file)
	assert.Equal(t, 65, code, errOut)

	_, _, code = runCam(t, cam, "setting", "apply", filepath.Join(dir, "missing.yaml"))
	assert.Equal(t, 74, code)

	// stdin
	ep := cam.Endpoint()
	var o, e bytes.Buffer
	app := NewApp(Stdio{In: strings.NewReader("video-lens: linear\n"), Out: &o, Err: &e})
	code = app.Run(context.Background(), []string{"--host", ep.Host, "--port", strconv.Itoa(ep.Port), "setting", "apply", "-"})
	assert.Equal(t, 0, code, e.String())
	assert.Equal(t, int64(4), cam.Settings[121])
}

func TestZoomPowerTurbo(t *testing.T) {
	cam := fakecam.New(t)
	_, _, code := runCam(t, cam, "zoom", "35")
	require.Equal(t, 0, code)
	assert.Equal(t, 35, cam.Zoom)
	_, _, code = runCam(t, cam, "zoom", "250")
	assert.Equal(t, 64, code)
	_, _, code = runCam(t, cam, "zoom", "x")
	assert.Equal(t, 64, code)

	_, _, code = runCam(t, cam, "power", "locate", "on")
	require.Equal(t, 0, code)
	assert.True(t, cam.Locate)
	_, _, code = runCam(t, cam, "power", "locate", "off")
	require.Equal(t, 0, code)
	assert.False(t, cam.Locate)

	_, _, code = runCam(t, cam, "turbo", "on")
	require.Equal(t, 0, code)
	assert.True(t, cam.Turbo)
	out, _, code := runCam(t, cam, "turbo", "off", "--output", "plain")
	require.Equal(t, 0, code)
	assert.Equal(t, "ok\n", out)
	assert.False(t, cam.Turbo)

	_, _, code = runCam(t, cam, "power", "sleep")
	require.Equal(t, 0, code)
	assert.True(t, cam.Sleeping)
}
