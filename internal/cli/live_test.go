package cli

import (
	"bytes"
	"context"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kazilotus/gopro9-cli/internal/fakecam"
	"github.com/kazilotus/gopro9-cli/pkg/gopro/stream"
)

func TestWebcamCommands(t *testing.T) {
	cam := fakecam.New(t)
	cam.StreamPackets = 0
	out, _, code := runCam(t, cam, "webcam", "status")
	require.Equal(t, 0, code)
	assert.Contains(t, out, "idle")
	out, _, code = runCam(t, cam, "webcam", "version", "--output", "plain")
	require.Equal(t, 0, code)
	assert.Equal(t, "4\n", out)

	out, errOut, code := runCam(t, cam, "webcam", "start", "--res", "1080", "--fov", "linear", "--output", "plain")
	require.Equal(t, 0, code, errOut)
	assert.Equal(t, "streaming\n", out)
	reqs := strings.Join(cam.Snapshot(), "\n")
	assert.Contains(t, reqs, "/gopro/webcam/start?res=12&fov=4")
	// The lens only sticks when set while webcam-idle: start → stop → set 43 → start.
	assert.Contains(t, reqs, "/gopro/webcam/stop\nGET /gopro/camera/setting?setting=43&option=4\nGET /gopro/webcam/start?res=12&fov=4")
	assert.Equal(t, int64(4), cam.Settings[43], "linear applied")
	assert.Equal(t, 2, cam.WebcamStatus)

	// Lens already matching: no restart.
	cam.WebcamStatus = 0
	before := len(cam.Snapshot())
	_, _, code = runCam(t, cam, "webcam", "start", "--fov", "superview")
	require.Equal(t, 0, code)
	after := cam.Snapshot()[before:]
	assert.Len(t, after, 3, "start, state read, status — no stop/set/restart: %v", after)

	_, _, code = runCam(t, cam, "webcam", "preview")
	require.Equal(t, 0, code)
	assert.Equal(t, 3, cam.WebcamStatus)
	_, _, code = runCam(t, cam, "webcam", "stop")
	require.Equal(t, 0, code)
	assert.Equal(t, 1, cam.WebcamStatus)
	out, _, code = runCam(t, cam, "webcam", "exit", "--output", "json")
	require.Equal(t, 0, code)
	assert.Contains(t, out, `"status": "off"`)

	_, _, code = runCam(t, cam, "webcam", "start", "--res", "4k")
	assert.Equal(t, 64, code)
	_, errOut, code = runCam(t, cam, "webcam", "start", "--fov", "fisheye")
	assert.Equal(t, 64, code)
	assert.Contains(t, errOut, "field of view")
}

func TestStreamStartStop(t *testing.T) {
	cam := fakecam.New(t)
	cam.StreamPackets = 0
	out, _, code := runCam(t, cam, "stream", "start")
	require.Equal(t, 0, code)
	assert.True(t, cam.Streaming)
	assert.Contains(t, out, "udp://@:8554")
	reqs := cam.Snapshot()
	assert.Equal(t, "GET /gopro/camera/stream/start", reqs[len(reqs)-1], "default port is left to the camera")
	_, _, code = runCam(t, cam, "stream", "stop", "--output", "plain")
	require.Equal(t, 0, code)
	assert.False(t, cam.Streaming)
	_, _, code = runCam(t, cam, "stream", "start", "--udp-port", "9001")
	require.Equal(t, 0, code)
	assert.Contains(t, strings.Join(cam.Snapshot(), "\n"), "stream/start?port=9001")
}

func freePort(t *testing.T) int {
	t.Helper()
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	require.NoError(t, err)
	port := conn.LocalAddr().(*net.UDPAddr).Port
	require.NoError(t, conn.Close())
	return port
}

func TestStreamRecord(t *testing.T) {
	cam := fakecam.New(t)
	port := freePort(t)
	cam.StreamPort = port
	cam.StreamPackets = 15
	out := filepath.Join(t.TempDir(), "preview.ts")

	res, errOut, code := runCam(t, cam, "stream", "record", "--out", out, "--udp-port", strconv.Itoa(port), "--idle-timeout", "500ms")
	require.Equal(t, 0, code, errOut)
	assert.Contains(t, res, "packets")
	assert.Contains(t, res, "15")
	st, err := os.Stat(out)
	require.NoError(t, err)
	assert.Equal(t, int64(15*188*7), st.Size())
	assert.False(t, cam.Streaming, "stream stopped afterwards")

	// Webcam flavour.
	cam.StreamPackets = 5
	res, errOut, code = runCam(t, cam, "stream", "record", "--out", out, "--udp-port", strconv.Itoa(port), "--webcam", "--res", "720", "--idle-timeout", "500ms", "--output", "plain")
	require.Equal(t, 0, code, errOut)
	assert.Equal(t, out+"\n", res)
	assert.Equal(t, 1, cam.WebcamStatus, "webcam stopped (idle) afterwards")
	assert.Contains(t, strings.Join(cam.Snapshot(), "\n"), "/gopro/webcam/start?res=7&port=")

	// No data → temp failure.
	cam.StreamPackets = 0
	_, errOut, code = runCam(t, cam, "stream", "record", "--out", out, "--udp-port", strconv.Itoa(port), "--idle-timeout", "200ms")
	assert.Equal(t, 75, code)
	assert.Contains(t, errOut, "no data")

	_, _, code = runCam(t, cam, "stream", "record")
	assert.Equal(t, 64, code)

	// Port already taken → unavailable.
	taken, err := stream.Listen(port)
	require.NoError(t, err)
	defer func() { _ = taken.Close() }()
	_, _, code = runCam(t, cam, "stream", "record", "--out", out, "--udp-port", strconv.Itoa(port))
	assert.Equal(t, 69, code)
}

func TestStreamRecordInterrupted(t *testing.T) {
	cam := fakecam.New(t)
	port := freePort(t)
	cam.StreamPort = port
	cam.StreamPackets = 1000
	out := filepath.Join(t.TempDir(), "p.ts")
	ep := cam.Endpoint()
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	var o, e bytes.Buffer
	app := NewApp(Stdio{In: strings.NewReader(""), Out: &o, Err: &e}, WithClock(func() time.Time { return fixedNow }))
	code := app.Run(ctx, []string{"--host", ep.Host, "--port", strconv.Itoa(ep.Port), "stream", "record", "--out", out, "--udp-port", strconv.Itoa(port)})
	assert.Equal(t, 0, code, e.String())
	assert.Contains(t, o.String(), "packets")
	assert.False(t, cam.Streaming)
}

func TestStreamPlayNoPlayer(t *testing.T) {
	cam := fakecam.New(t)
	_, errOut, code := runCam(t, cam, "stream", "play", "--player", "no-such-player")
	assert.Equal(t, 69, code)
	assert.Contains(t, errOut, "not found")
	assert.False(t, cam.Streaming)
}

func TestStreamPlayWithFakePlayer(t *testing.T) {
	if _, err := os.Stat("/bin/sh"); err != nil {
		t.Skip("needs /bin/sh")
	}
	cam := fakecam.New(t)
	cam.StreamPackets = 0
	// A "player" that exits immediately: ffplay resolved from a temp PATH.
	dir := t.TempDir()
	script := filepath.Join(dir, "ffplay")
	require.NoError(t, os.WriteFile(script, []byte("#!/bin/sh\necho played \"$@\" >&2\n"), 0o755))
	t.Setenv("PATH", dir)
	_, errOut, code := runCam(t, cam, "stream", "play", "--player", "ffplay", "--udp-port", "9123")
	require.Equal(t, 0, code, errOut)
	assert.Contains(t, errOut, "udp://@:9123")
	assert.False(t, cam.Streaming, "stream stopped when the player exits")
}
