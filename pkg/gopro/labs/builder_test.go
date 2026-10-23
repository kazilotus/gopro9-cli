package labs

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuilderString(t *testing.T) {
	t.Parallel()
	b := (&Builder{}).Mode(ModeVideo).Resolution("4").FPS(60).Lens(LensWide).EV(-0.5).WhiteBalance(5500).
		ISO(2, 16).Color(ColorFlat).HyperSmooth(HyperSmoothHigh).GPS(true).Setting("sL", "").
		Extension("HIST", "6").Metadata("OWNR", "Jo Bloggs", true).Metadata("LLTZ", "32.7,-117.1,-7", true).
		Message(`say "hi"`).StartIn(10 * time.Second).Duration(2 * time.Minute).Repeat(10 * time.Minute).Raw("!X")
	assert.Equal(t, `mVr4p60fWx-.5w55i16M2cFe2g1sL$HIST=6!MOWNR="Jo Bloggs"!MLLTZ=32.7,-117.1,-7"say 'hi'"!10S!120E!600R!X`, b.String())

	sc, err := b.Script()
	require.NoError(t, err)
	assert.Equal(t, []Kind{
		ModeCmd, SettingCmd, SettingCmd, SettingCmd, SettingCmd, SettingCmd, SettingCmd, SettingCmd, SettingCmd, SettingCmd, SettingCmd,
		ExtensionCmd, ExtensionCmd, ExtensionCmd, Message, Action, Action, Action, Action,
	}, kinds(sc))
}

func TestBuilderVariants(t *testing.T) {
	t.Parallel()
	assert.Equal(t, `mP!S`, (&Builder{}).Mode(ModePhoto).Start().String())
	assert.Equal(t, `mTWp2!12:00S`, (&Builder{}).Mode(ModeTimeWarp).Interval(2).StartAt("12:00").String())
	assert.Equal(t, `!S!30E!R`, (&Builder{}).Duration(30*time.Second).Repeat(0).String(), "Duration implies Start")
	assert.Equal(t, `wAi4x1.5`, (&Builder{}).WhiteBalance(0).ISO(0, 4).EV(1.5).String())
	assert.Equal(t, `$WAKE=1`, (&Builder{}).Extension("wake", "1").String())
	assert.Empty(t, (&Builder{}).String())

	_, err := (&Builder{}).Script()
	require.ErrorIs(t, err, ErrSyntax, "empty script")
	_, err = (&Builder{}).Mode(ModePhoto).Duration(5 * time.Second).Script()
	require.ErrorIs(t, err, ErrSyntax, "photo mode with !E is a validation error")
}

func TestBuilderRoundTrip(t *testing.T) {
	t.Parallel()
	b := (&Builder{}).Mode(ModeVideo).Resolution("1080").FPS(30).StartIn(5 * time.Second).Duration(10 * time.Second)
	sc, err := Parse(b.String())
	require.NoError(t, err)
	assert.Empty(t, Validate(sc))
	assert.Equal(t, []Kind{ModeCmd, SettingCmd, SettingCmd, Action, Action}, kinds(sc))
	assert.Equal(t, "5", sc.Commands[3].When)
	assert.Equal(t, "10", sc.Commands[4].When)
}

func TestHelpers(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "-.5", trimFloat(-0.5))
	assert.Equal(t, ".25", trimFloat(0.25))
	assert.Equal(t, "2", trimFloat(2))
	assert.Equal(t, "-2", trimFloat(-2))
	assert.Equal(t, "90", seconds(90*time.Second))
	assert.True(t, isNumberList("1,2.5,-3"))
	assert.False(t, isNumberList("1,x"))
	assert.False(t, isNumberList(""))
}
