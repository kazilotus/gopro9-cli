package cli

import (
	"bytes"
	"context"
	"encoding/xml"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLabsQR(t *testing.T) {
	out, errOut, code := run(t, "labs", "qr", "mVr4p60!S!10E")
	require.Equal(t, 0, code, errOut)
	assert.Contains(t, out, "command")
	assert.Contains(t, out, "mVr4p60!S!10E")
	assert.Greater(t, strings.Count(out, "\n"), 10, "terminal QR printed")

	out, _, code = run(t, "labs", "qr", "mVr4p60!S!10E", "--no-qr", "--output", "plain")
	require.Equal(t, 0, code)
	assert.Equal(t, "mVr4p60!S!10E\n", out)

	dir := t.TempDir()
	pngPath, svgPath := filepath.Join(dir, "q.png"), filepath.Join(dir, "q.svg")
	out, _, code = run(t, "labs", "qr", "mP!S", "--png", pngPath, "--svg", svgPath, "--size", "128", "--output", "json")
	require.Equal(t, 0, code)
	assert.Contains(t, out, `"png"`)
	f, err := os.Open(pngPath)
	require.NoError(t, err)
	img, err := png.Decode(f)
	_ = f.Close()
	require.NoError(t, err)
	assert.Equal(t, 128, img.Bounds().Dx())
	svg, err := os.ReadFile(svgPath)
	require.NoError(t, err)
	require.NoError(t, xml.Unmarshal(svg, new(struct{})))

	// Warnings (unsupported extension) are printed but do not fail.
	_, errOut, code = run(t, "labs", "qr", "$BITR=150", "--no-qr")
	assert.Equal(t, 0, code)
	assert.Contains(t, errOut, "BITR")

	// Validation errors fail with the data exit code.
	_, errOut, code = run(t, "labs", "qr", "mP!S!E", "--no-qr")
	assert.Equal(t, 65, code)
	assert.Contains(t, errOut, "error:")

	// Syntax errors are usage errors.
	_, _, code = run(t, "labs", "qr", `"unterminated`)
	assert.Equal(t, 64, code)

	// Unwritable PNG path.
	_, _, code = run(t, "labs", "qr", "mP!S", "--png", filepath.Join(dir, "no", "dir", "q.png"))
	assert.Equal(t, 74, code)
}

func TestLabsTime(t *testing.T) {
	var out, errb bytes.Buffer
	app := NewApp(Stdio{In: strings.NewReader(""), Out: &out, Err: &errb}, WithClock(func() time.Time { return fixedNow }))
	code := app.Run(context.Background(), []string{"labs", "time", "--utc", "--no-qr", "--output", "plain"})
	require.Equal(t, 0, code, errb.String())
	assert.Equal(t, "oT261023084634.000\n", out.String())

	out.Reset()
	code = app.Run(context.Background(), []string{"labs", "time", "--utc", "--precision=false", "--no-qr", "--output", "plain"})
	require.Equal(t, 0, code)
	assert.Equal(t, "oT261023084634\n", out.String())

	out.Reset()
	code = app.Run(context.Background(), []string{"labs", "time", "--at", "2030-01-02T03:04:05", "--no-qr", "--output", "plain"})
	require.Equal(t, 0, code)
	assert.True(t, strings.HasPrefix(out.String(), "oT300102030405"), out.String())

	_, _, code = run(t, "labs", "time", "--at", "yesterday")
	assert.Equal(t, 64, code)
}

func TestLabsBuild(t *testing.T) {
	out, errOut, code := run(t, "labs", "build", "--mode", "video", "--res", "4K", "--fps", "60", "--lens", "linear", "--ev", "-0.5", "--start-in", "10s", "--duration", "30s", "--no-qr", "--output", "plain")
	require.Equal(t, 0, code, errOut)
	cmd := strings.TrimSpace(out)
	assert.Contains(t, cmd, "mV")
	assert.Contains(t, cmd, "r4")
	assert.Contains(t, cmd, "p60")
	assert.Contains(t, cmd, "fL")
	assert.Contains(t, cmd, "x-.5")
	assert.Contains(t, cmd, "!10S")
	assert.Contains(t, cmd, "!30E")

	out, _, code = run(t, "labs", "build", "--mode", "photo", "--start-at", "06:30", "--repeat", "10m", "--message", "Sunrise", "--gps", "on", "--color", "flat", "--hypersmooth", "high", "--wb", "5500K", "--iso", "100:1600", "--interval", "0.5", "--no-qr", "--output", "plain")
	require.Equal(t, 0, code)
	cmd = strings.TrimSpace(out)
	assert.Contains(t, cmd, "mP")
	assert.Contains(t, cmd, "!06:30S")
	assert.Contains(t, cmd, `"Sunrise"`)
	assert.Contains(t, cmd, "g1")
	assert.Contains(t, cmd, "cF")
	assert.Contains(t, cmd, "w55")

	out, errOut, code = run(t, "labs", "build", "--ext", "HIST=1", "--meta", "OWNR=Jane", "--raw", "oV1", "--no-qr", "--output", "plain")
	require.Equal(t, 0, code, errOut)
	assert.Contains(t, out, "HIST")
	assert.Contains(t, out, "OWNR")
	assert.Contains(t, out, "oV1")

	for _, args := range [][]string{
		{"--mode", "sideways"},
		{"--lens", "fisheye"},
		{"--color", "vivid"},
		{"--hypersmooth", "max"},
		{"--gps", "maybe"},
		{"--wb", "warm"},
		{"--iso", "100"},
		{"--ext", "HISTOGRAM=1"},
		{"--meta", "OW=1"},
		{},
	} {
		_, _, code := run(t, append([]string{"labs", "build", "--no-qr"}, args...)...)
		assert.Equal(t, 64, code, "%v", args)
	}
}

func TestLabsValidateExplainExtensions(t *testing.T) {
	out, _, code := run(t, "labs", "validate", "mVr4p60!S!10E")
	require.Equal(t, 0, code)
	assert.Contains(t, out, "no issues")

	out, _, code = run(t, "labs", "validate", "mP!S!E", "--output", "plain")
	assert.Equal(t, 65, code)
	assert.Equal(t, "invalid\n", out)

	_, _, code = run(t, "labs", "validate", `"oops`)
	assert.Equal(t, 64, code)

	out, _, code = run(t, "labs", "explain", "mVr4p60x-.5!10S!30E", "--output", "plain")
	require.Equal(t, 0, code)
	assert.Contains(t, out, "Video")
	assert.Contains(t, out, "4K")
	lines := strings.Split(strings.TrimSpace(out), "\n")
	assert.GreaterOrEqual(t, len(lines), 5)
	out, _, code = run(t, "labs", "explain", "mVr4")
	require.Equal(t, 0, code)
	assert.Contains(t, out, "mV")
	_, _, code = run(t, "labs", "explain", `"oops`)
	assert.Equal(t, 64, code)

	out, _, code = run(t, "labs", "extensions")
	require.Equal(t, 0, code)
	assert.Contains(t, out, "BITR")
	assert.Contains(t, out, "unsupported")
	out, _, code = run(t, "labs", "extensions", "--supported", "--output", "json")
	require.Equal(t, 0, code)
	assert.NotContains(t, out, `"BITR"`)
	assert.Contains(t, out, `"HIST"`)
}
