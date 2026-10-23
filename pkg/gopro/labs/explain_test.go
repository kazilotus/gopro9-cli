package labs

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExplain(t *testing.T) {
	t.Parallel()
	lines := Explain(mustParse(t, `mVr4p60x-.5cFw55fL$BITR=150!10S!120E!R`))
	joined := strings.Join(lines, "\n")
	for _, want := range []string{
		"mV → Video mode",
		"r4 → resolution 4K",
		"p60 → frame rate 60 fps",
		"x-.5 → EV -.5",
		"cF → colour F",
		"w55 → white balance 5500 K",
		"fL → lens linear",
		"$BITR=150 → Labs extension BITR (HEVC bitrate in Mb/s) = 150 [not supported on HERO9]",
		"!10S → start capture after 10 s",
		"!120E → end capture after 120 s",
		"!R → repeat script",
	} {
		assert.Contains(t, joined, want)
	}
}

func TestExplainCoversEveryKind(t *testing.T) {
	t.Parallel()
	src := `mZZr27Tr4Xr53r8r5p.5wAwNg1q0v1hS1oV1oTZ-7oQ9!12:00S!00;05R!s-30N!Lbt!GLC!Z1!QQ>b50<t:W3==v12+~=A5*MAC="mV!S"^MAC"hi"*OWNR="jo"$ABCD=1!MHIST=3$PRXY=1@!SAVEx=mP!S`
	sc, err := Parse(src)
	require.NoError(t, err)
	lines := Explain(sc)
	require.Len(t, lines, len(sc.Commands))
	joined := strings.Join(lines, "\n")
	for _, want := range []string{
		"unknown mode ZZ", "resolution 2.7K 4:3", "resolution 4K 8:7", "resolution 5.3K", "resolution 8K", "resolution 5K",
		"interval .5 s", "white balance auto", "white balance native", "GPS on", "Quick Capture off", "voice control on",
		"HindSight 1", "beep volume = 1", "timezone offset = -7", "option oQ = 9",
		"start capture at 12:00", "repeat script aligned to every 00:05", "sleep / pause relative to sunset (-30)",
		"load saved script bt", "save script x = mP!S", "start live stream 1080p (save copy)", "ignore buttons 1", "action QQ",
		"if battery % is at least 50", "if weekday is less than 3", "if mode number is equal to 12", "also under the same condition", "else",
		"set variable A = 5", "define macro MAC", "run macro MAC", `show message "hi"`,
		"Labs extension OWNR (owner name) = jo, permanent", `temporary metadata ABCD = "1"`,
		"Labs extension HIST (histogram overlay (1-11, 0 off)) = 3, permanent", "[HERO9 support uncertain]", "unrecognised",
	} {
		assert.Contains(t, joined, want)
	}
	// time conditional and a mode-number conditional
	lines = Explain(mustParse(t, `>09:00<17:00==v12>Q7`))
	assert.Contains(t, lines[0], "time is at least 09:00")
	assert.Contains(t, lines[1], "time is less than 17:00")
	assert.Contains(t, lines[2], "mode number is equal to 12")
	assert.Contains(t, lines[3], "variable Q is at least 7")
}
