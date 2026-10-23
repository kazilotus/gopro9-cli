package labs

import (
	"bytes"
	"encoding/xml"
	"image/png"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQRPNG(t *testing.T) {
	t.Parallel()
	data, err := QRPNG("mVr4p60!S", 256)
	require.NoError(t, err)
	img, err := png.Decode(bytes.NewReader(data))
	require.NoError(t, err)
	assert.Equal(t, 256, img.Bounds().Dx())
	assert.Equal(t, img.Bounds().Dx(), img.Bounds().Dy())

	data, err = QRPNG("mV", 0)
	require.NoError(t, err)
	img, err = png.Decode(bytes.NewReader(data))
	require.NoError(t, err)
	assert.Equal(t, 512, img.Bounds().Dx(), "default size")

	_, err = QRPNG(strings.Repeat("x", 5000), 100)
	require.Error(t, err, "exceeds QR capacity")
}

func TestQRSVG(t *testing.T) {
	t.Parallel()
	svg, err := QRSVG("mP!S")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(svg, "<svg"))
	assert.Contains(t, svg, `fill="#000"`)
	var doc struct {
		XMLName xml.Name
		ViewBox string `xml:"viewBox,attr"`
	}
	require.NoError(t, xml.Unmarshal([]byte(svg), &doc))
	assert.Equal(t, "svg", doc.XMLName.Local)
	assert.NotEmpty(t, doc.ViewBox)
	_, err = QRSVG(strings.Repeat("x", 5000))
	require.Error(t, err)
}

func TestQRANSI(t *testing.T) {
	t.Parallel()
	bm, err := bitmap("mP!S")
	require.NoError(t, err)
	n := len(bm)
	out, err := QRANSI("mP!S", false)
	require.NoError(t, err)
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	assert.Len(t, lines, (n+1)/2, "two modules per line")
	for _, l := range lines {
		assert.Len(t, []rune(l), n)
	}
	// quiet zone: the first line is entirely light
	assert.Equal(t, strings.Repeat(" ", n), lines[0])
	inv, err := QRANSI("mP!S", true)
	require.NoError(t, err)
	invLines := strings.Split(strings.TrimRight(inv, "\n"), "\n")
	assert.Equal(t, strings.Repeat("█", n), invLines[0], "inverted quiet zone is dark")
	assert.NotEqual(t, out, inv)
	_, err = QRANSI(strings.Repeat("x", 5000), false)
	require.Error(t, err)
}

func TestTimeSyncCommand(t *testing.T) {
	t.Parallel()
	ts := time.Date(2026, 10, 23, 8, 46, 34, 123_000_000, time.UTC)
	assert.Equal(t, "oT261023084634", TimeSyncCommand(ts, false))
	assert.Equal(t, "oT261023084634.123", TimeSyncCommand(ts, true))
	sc, err := Parse(TimeSyncCommand(ts, true))
	require.NoError(t, err)
	require.Len(t, sc.Commands, 1)
	assert.Equal(t, OptionCmd, sc.Commands[0].Kind)
	assert.Equal(t, "oT", sc.Commands[0].Key)
	assert.Equal(t, "261023084634.123", sc.Commands[0].Arg)
	assert.Empty(t, Validate(sc))
}
