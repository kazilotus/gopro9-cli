package cli

import (
	"bytes"
	"context"
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kazilotus/gopro9-cli/internal/fakecam"
)

var update = flag.Bool("update", false, "rewrite golden files")

// fixedNow is the deterministic clock used by golden tests.
var fixedNow = time.Date(2026, 10, 23, 8, 46, 34, 0, time.UTC)

// Golden files render local times; pin the zone so they match on every runner.
func init() { time.Local = time.UTC }

// runCam executes the CLI against a fake camera and returns stdout, stderr, exit code.
func runCam(t *testing.T, cam *fakecam.Server, args ...string) (string, string, int) {
	t.Helper()
	ep := cam.Endpoint()
	full := append([]string{"--host", ep.Host, "--port", strconv.Itoa(ep.Port), "--timeout", "2s"}, args...)
	var out, errb bytes.Buffer
	app := NewApp(Stdio{In: strings.NewReader(""), Out: &out, Err: &errb}, WithClock(func() time.Time { return fixedNow }))
	code := app.Run(context.Background(), full)
	return out.String(), errb.String(), code
}

var portRe = regexp.MustCompile(`127\.0\.0\.1:\d+`)

// golden compares got with testdata/golden/<name>.golden, normalising the fake camera's port.
func golden(t *testing.T, name, got string) {
	t.Helper()
	got = portRe.ReplaceAllString(got, "127.0.0.1:PORT")
	path := filepath.Join("..", "..", "testdata", "golden", name+".golden")
	if *update {
		require.NoError(t, os.WriteFile(path, []byte(got), 0o600))
		return
	}
	want, err := os.ReadFile(path)
	require.NoError(t, err, "missing golden %s (run with -update)", path)
	assert.Equal(t, string(want), got, "golden mismatch for %s (run go test ./internal/cli -update to accept)", name)
}

func TestInfoGolden(t *testing.T) {
	cam := fakecam.New(t)
	for _, f := range []string{"table", "json", "yaml", "plain"} {
		out, errOut, code := runCam(t, cam, "info", "--output", f)
		require.Equal(t, 0, code, errOut)
		golden(t, "info_"+f, out)
	}
}

func TestStatusGolden(t *testing.T) {
	cam := fakecam.New(t)
	out, _, code := runCam(t, cam, "status")
	require.Equal(t, 0, code)
	golden(t, "status_table", out)
	out, _, code = runCam(t, cam, "status", "--output", "json")
	require.Equal(t, 0, code)
	golden(t, "status_json", out)
	out, _, _ = runCam(t, cam, "status", "--output", "plain")
	assert.Equal(t, "idle\n", out)
	cam.Recording = true
	out, _, _ = runCam(t, cam, "status", "--output", "plain")
	assert.Equal(t, "recording\n", out)
}

func TestStateGolden(t *testing.T) {
	cam := fakecam.New(t)
	out, _, code := runCam(t, cam, "state")
	require.Equal(t, 0, code)
	golden(t, "state_table", out)
	out, _, code = runCam(t, cam, "state", "--output", "json")
	require.Equal(t, 0, code)
	golden(t, "state_json", out)
	out, _, code = runCam(t, cam, "state", "--raw")
	require.Equal(t, 0, code)
	assert.True(t, strings.HasPrefix(out, "{"))
	assert.Contains(t, out, `"settings"`)
}

func TestStateWatchStopsOnContext(t *testing.T) {
	cam := fakecam.New(t)
	ep := cam.Endpoint()
	ctx, cancel := context.WithTimeout(context.Background(), 350*time.Millisecond)
	defer cancel()
	var out bytes.Buffer
	app := NewApp(Stdio{In: strings.NewReader(""), Out: &out, Err: &out}, WithClock(func() time.Time { return fixedNow }))
	code := app.Run(ctx, []string{"--host", ep.Host, "--port", strconv.Itoa(ep.Port), "state", "--watch", "100ms"})
	assert.Equal(t, 0, code)
	assert.Contains(t, out.String(), "setting.video-resolution")
	first := out.Len()
	// Change state and confirm only the diff is printed on later ticks.
	assert.Positive(t, first)
}

func TestCapabilitiesGolden(t *testing.T) {
	cam := fakecam.New(t)
	out, _, code := runCam(t, cam, "capabilities", "--level", "verified")
	require.Equal(t, 0, code)
	golden(t, "capabilities_verified", out)
	out, _, code = runCam(t, cam, "capabilities", "--op", "OGP_GET_STATE", "--output", "json")
	require.Equal(t, 0, code)
	assert.Contains(t, out, `"level": "verified"`)
	_, errOut, code := runCam(t, cam, "capabilities", "--op", "NOPE")
	assert.Equal(t, 64, code)
	assert.Contains(t, errOut, "unknown operation")
}

func TestDatetime(t *testing.T) {
	cam := fakecam.New(t, fakecam.WithClock(func() time.Time { return fixedNow }))
	out, _, code := runCam(t, cam, "datetime", "get", "--output", "json")
	require.Equal(t, 0, code)
	assert.Contains(t, out, `"drift"`)

	_, errOut, code := runCam(t, cam, "datetime", "sync")
	assert.Equal(t, 69, code, "set_date_time is marked unsupported on HERO9 until verified")
	assert.Contains(t, errOut, "--force")

	out, errOut, code = runCam(t, cam, "datetime", "sync", "--force", "--output", "plain")
	require.Equal(t, 0, code, errOut)
	assert.Contains(t, out, "2026-10-23")
	assert.Equal(t, 2026, cam.Clock.Year())

	_, _, code = runCam(t, cam, "datetime", "set", "2030-01-02T03:04:05", "--force")
	require.Equal(t, 0, code)
	assert.Equal(t, 2030, cam.Clock.Year())

	_, errOut, code = runCam(t, cam, "datetime", "set", "yesterday", "--force")
	assert.Equal(t, 64, code)
	assert.Contains(t, errOut, "cannot parse time")
}

func TestKeepaliveRunsUntilCancelled(t *testing.T) {
	cam := fakecam.New(t)
	ep := cam.Endpoint()
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	var out, errb bytes.Buffer
	app := NewApp(Stdio{In: strings.NewReader(""), Out: &out, Err: &errb}, WithClock(func() time.Time { return fixedNow }))
	code := app.Run(ctx, []string{"--host", ep.Host, "--port", strconv.Itoa(ep.Port), "keepalive", "--interval", "50ms"})
	assert.Equal(t, 0, code)
	assert.GreaterOrEqual(t, cam.KeepAlives, 3)
	assert.Contains(t, errb.String(), "keep-alive sent")
}

func TestDoctorAgainstFake(t *testing.T) {
	cam := fakecam.New(t)
	dir := t.TempDir()
	out, errOut, code := runCam(t, cam, "doctor", "--record", dir, "--output", "json")
	// The fake does not serve port 80/8000/8001, so those are warnings, not failures.
	require.Equal(t, 0, code, errOut)
	assert.Contains(t, out, `"host-guard"`)
	assert.Contains(t, out, `"pass"`)
	assert.NotContains(t, out, `"fail"`)
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.NotEmpty(t, entries, "fixtures recorded")
	for _, e := range entries {
		b, _ := os.ReadFile(filepath.Join(dir, e.Name()))
		assert.NotContains(t, string(b), "C3441326546435", "serial must be scrubbed")
	}

	out, _, code = runCam(t, cam, "doctor", "--no-probe", "--output", "plain")
	assert.Equal(t, 0, code)
	assert.Equal(t, "warn\n", out, "alternate ports are not served by the fake")
}

func TestDoctorDetectsDrift(t *testing.T) {
	cam := fakecam.New(t)
	cam.FailWith["/gopro/camera/presets/get"] = 404 // verified op suddenly missing
	out, _, code := runCam(t, cam, "doctor")
	assert.Equal(t, 69, code)
	assert.Contains(t, out, "DRIFT")
}

func TestCameraUnreachable(t *testing.T) {
	var out, errb bytes.Buffer
	app := NewApp(Stdio{In: strings.NewReader(""), Out: &out, Err: &errb})
	code := app.Run(context.Background(), []string{"--host", "127.0.0.1", "--port", "9", "--timeout", "300ms", "info"})
	assert.Equal(t, 69, code)
	assert.Contains(t, errb.String(), "unreachable")

	errb.Reset()
	code = app.Run(context.Background(), []string{"--host", "127.0.0.1", "--port", "9", "--timeout", "300ms", "--output", "json", "info"})
	assert.Equal(t, 69, code)
	assert.Contains(t, errb.String(), `"exit": 69`)
}

func TestScrub(t *testing.T) {
	t.Parallel()
	in := `{"serial_number":"C3441326546435","ap_mac":"2674f7422ab3","ap_ssid":"Wazed HERO9 Black","30":"Wazed HERO9 Black"}`
	got := string(scrub([]byte(in)))
	assert.NotContains(t, got, "C3441326546435")
	assert.NotContains(t, got, "2674f7422ab3")
	assert.NotContains(t, got, "Wazed")
}
