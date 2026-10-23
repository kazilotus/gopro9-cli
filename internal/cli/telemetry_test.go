package cli

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kazilotus/gopro9-cli/internal/fakecam"
)

const realTelemetry = "../../testdata/gpmf/telemetry_GH050365.bin"

func TestTelemetryFromRealFile(t *testing.T) {
	if _, err := os.Stat(realTelemetry); err != nil {
		t.Skip("real telemetry fixture missing")
	}
	out, errOut, code := run(t, "media", "telemetry", "--from", realTelemetry)
	require.Equal(t, 0, code, errOut)
	assert.Contains(t, out, "GPS5")
	assert.Contains(t, out, "ACCL")
	assert.Contains(t, out, "GYRO")

	out, _, code = run(t, "media", "telemetry", "--from", realTelemetry, "--format", "gpx")
	require.Equal(t, 0, code)
	require.NoError(t, xml.Unmarshal([]byte(out), new(struct{})))
	assert.Contains(t, out, "<trkpt")

	dir := t.TempDir()
	gpx := filepath.Join(dir, "t.gpx")
	_, _, code = run(t, "media", "telemetry", "--from", realTelemetry, "--format", "gpx", "--out", gpx, "--output", "plain")
	require.Equal(t, 0, code)
	st, err := os.Stat(gpx)
	require.NoError(t, err)
	assert.Positive(t, st.Size())

	out, _, code = run(t, "media", "telemetry", "--from", realTelemetry, "--format", "csv", "--stream", "ACCL")
	require.Equal(t, 0, code)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	assert.Greater(t, len(lines), 100)
	assert.True(t, strings.HasPrefix(lines[0], "payload,"), lines[0])

	out, _, code = run(t, "media", "telemetry", "--from", realTelemetry, "--format", "json")
	require.Equal(t, 0, code)
	assert.Contains(t, out, `"GPS5"`)

	out, _, code = run(t, "media", "telemetry", "--from", realTelemetry, "--output", "json")
	require.Equal(t, 0, code)
	assert.Contains(t, out, `"gps_fixes"`)

	_, errOut, code = run(t, "media", "telemetry", "--from", realTelemetry, "--format", "csv", "--stream", "NOPE")
	assert.Equal(t, 64, code)
	assert.Contains(t, errOut, "available")
	_, _, code = run(t, "media", "telemetry", "--from", realTelemetry, "--format", "xml")
	assert.Equal(t, 64, code)
	_, _, code = run(t, "media", "telemetry")
	assert.Equal(t, 64, code)
	_, _, code = run(t, "media", "telemetry", "--from", filepath.Join(dir, "missing.bin"))
	assert.Equal(t, 74, code)
}

func TestTelemetryFromCamera(t *testing.T) {
	cam := fakecam.New(t)
	// The fake serves a minimal raw GPMF blob with no GPS: summary works, gpx fails cleanly.
	out, errOut, code := runCam(t, cam, "media", "telemetry", "100GOPRO/GH010397.MP4")
	require.Equal(t, 0, code, errOut)
	assert.Contains(t, out, "STREAM")
	_, errOut, code = runCam(t, cam, "media", "telemetry", "100GOPRO/GH010397.MP4", "--format", "gpx")
	assert.Equal(t, 65, code)
	assert.Contains(t, errOut, "no GPS fixes")
	dir := t.TempDir()
	raw := filepath.Join(dir, "raw.mp4")
	_, _, code = runCam(t, cam, "media", "telemetry", "100GOPRO/GH010397.MP4", "--format", "raw", "--out", raw)
	require.Equal(t, 0, code)
	_, err := os.Stat(raw)
	require.NoError(t, err)
	_, _, code = runCam(t, cam, "media", "telemetry", "100GOPRO/NOPE.MP4")
	assert.Equal(t, 69, code)

	garbage := filepath.Join(dir, "garbage.bin")
	require.NoError(t, os.WriteFile(garbage, []byte("not gpmf at all, definitely not"), 0o600))
	_, _, code = run(t, "media", "telemetry", "--from", garbage)
	assert.NotEqual(t, 0, code)
}
