package labs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type want struct {
	kind Kind
	key  string
	arg  string
	when string
}

func kinds(sc Script) []Kind {
	out := make([]Kind, 0, len(sc.Commands))
	for _, c := range sc.Commands {
		out = append(out, c.Kind)
	}
	return out
}

func TestParseDocumentedExamples(t *testing.T) {
	t.Parallel()
	cases := []struct {
		src  string
		want []want
	}{
		{`mVr4p60x-.5cFw55$BITR=150$LEVL=6`, []want{
			{ModeCmd, "mV", "V", ""},
			{SettingCmd, "r", "4", ""},
			{SettingCmd, "p", "60", ""},
			{SettingCmd, "x", "-.5", ""},
			{SettingCmd, "c", "F", ""},
			{SettingCmd, "w", "55", ""},
			{ExtensionCmd, "BITR", "150", ""},
			{ExtensionCmd, "LEVL", "6", ""},
		}},
		{`*BOOT="!Lbt"!SAVEbt=!12:00NmP!S!1R`, []want{
			{ExtensionCmd, "BOOT", "!Lbt", ""}, {Action, "SAVE", "bt=!12:00NmP!S!1R", ""},
		}},
		{`!17:00W!GLC`, []want{{Action, "W", "", "17:00"}, {Action, "GLC", "", ""}}},
		{`mV"Video Mode"!S!5E!4NmP"Photo Mode"!S!5R`, []want{
			{ModeCmd, "mV", "V", ""},
			{Message, `"`, "Video Mode", ""},
			{Action, "S", "", ""},
			{Action, "E", "", "5"},
			{Action, "N", "", "4"},
			{ModeCmd, "mP", "P", ""},
			{Message, `"`, "Photo Mode", ""},
			{Action, "S", "", ""},
			{Action, "R", "", "5"},
		}},
		{`=E6.25=Gi=G*0.01=E/G=E*s=E#2"Exposure value $E"!R`, []want{
			{Assignment, "E", "6.25", ""},
			{Assignment, "G", "i", ""},
			{Assignment, "G", "*0.01", ""},
			{Assignment, "E", "/G", ""},
			{Assignment, "E", "*s", ""},
			{Assignment, "E", "#2", ""},
			{Message, `"`, "Exposure value $E", ""},
			{Action, "R", "", ""},
		}},
		{`=Tt:W=T%6>T1>09:00<17:00mP+!S+!2N+!600RQ!09:00R`, []want{
			{Assignment, "T", "t:W", ""},
			{Assignment, "T", "%6", ""},
			{Conditional, ">", "T1", ""},
			{Conditional, ">", "09:00", ""},
			{Conditional, "<", "17:00", ""},
			{ModeCmd, "mP", "P", ""},
			{Bundle, "+", "", ""},
			{Action, "S", "", ""},
			{Bundle, "+", "", ""},
			{Action, "N", "", "2"},
			{Bundle, "+", "", ""},
			{Action, "RQ", "", "600"},
			{Action, "R", "", "09:00"},
		}},
		{`>06:00<18:00mP+!S+!2N+!1800RQ!06:00R`, []want{
			{Conditional, ">", "06:00", ""},
			{Conditional, "<", "18:00", ""},
			{ModeCmd, "mP", "P", ""},
			{Bundle, "+", "", ""},
			{Action, "S", "", ""},
			{Bundle, "+", "", ""},
			{Action, "N", "", "2"},
			{Bundle, "+", "", ""},
			{Action, "RQ", "", "1800"},
			{Action, "R", "", "06:00"},
		}},
		{`<08:45!S+"Hello World"+!60E~!08:44N!R`, []want{
			{Conditional, "<", "08:45", ""},
			{Action, "S", "", ""},
			{Bundle, "+", "", ""},
			{Message, `"`, "Hello World", ""},
			{Bundle, "+", "", ""},
			{Action, "E", "", "60"},
			{Else, "~", "", ""},
			{Action, "N", "", "08:44"},
			{Action, "R", "", ""},
		}},
		{`mP!S!2N!600R`, []want{{ModeCmd, "mP", "P", ""}, {Action, "S", "", ""}, {Action, "N", "", "2"}, {Action, "R", "", "600"}}},
		{`==v12!S`, []want{{Conditional, "==", "v12", ""}, {Action, "S", "", ""}}},
		{`*LLTZ=32.707,-117.1576,-7*OWNR="Joe Bloggs"`, []want{{ExtensionCmd, "LLTZ", "32.707,-117.1576,-7", ""}, {ExtensionCmd, "OWNR", "Joe Bloggs", ""}}},
		{`*SUBA="mVr4p60'60p'!S!2E!1N"^SUBA"hi $E"`, []want{{Macro, "SUBA", "mVr4p60'60p'!S!2E!1N", ""}, {MacroCall, "^", "SUBA", ""}, {Message, `"`, "hi $E", ""}}},
		{`!MTCAL=-20!00;05R!s-30S!rN!n15E`, []want{{ExtensionCmd, "TCAL", "-20", ""}, {Action, "R", "", "00;05"}, {Action, "S", "", "s-30"}, {Action, "N", "", "r"}, {Action, "E", "", "n15"}}},
		{`oV1B4D2oFUoTZ-7oTD1oT261023084634.123oMABCD="x"oMQWER=5`, []want{
			{OptionCmd, "oV", "1", ""},
			{OptionCmd, "oB", "4", ""},
			{OptionCmd, "oD", "2", ""},
			{OptionCmd, "oF", "U", ""},
			{OptionCmd, "oTZ", "-7", ""},
			{OptionCmd, "oTD", "1", ""},
			{OptionCmd, "oT", "261023084634.123", ""},
			{Metadata, "ABCD", "x", ""},
			{Metadata, "QWER", "5", ""},
		}},
		{`hS1eL2i16M2fNwAdLtEg0`, []want{
			{SettingCmd, "hS", "1", ""},
			{SettingCmd, "eL", "2", ""},
			{SettingCmd, "i", "16M2", ""},
			{SettingCmd, "f", "N", ""},
			{SettingCmd, "w", "A", ""},
			{SettingCmd, "d", "L", ""},
			{SettingCmd, "t", "E", ""},
			{SettingCmd, "g", "0", ""},
		}},
		{`!Lbt!Rx3!Z1!C10!U5!X!TR`, []want{
			{Action, "L", "bt", ""},
			{Action, "R", "", ""},
			{SettingCmd, "x", "3", ""},
			{Action, "Z", "1", ""},
			{Action, "C", "10", ""},
			{Action, "U", "5", ""},
			{Action, "X", "", ""},
			{Action, "TR", "", ""},
		}},
		{`$XYZW=1*ABCD="just some words"*ABCD=7`, []want{{Metadata, "XYZW", "1", ""}, {Metadata, "ABCD", "just some words", ""}, {Metadata, "ABCD", "7", ""}}},
		{`!RESET!1OR`, []want{{Action, "RESET", "", ""}, {Action, "OR", "", "1"}}},
		{`m !s oM"x"`, []want{{ModeCmd, "m", "", ""}, {Action, "s", "", ""}, {OptionCmd, "oM", "", ""}, {Message, `"`, "x", ""}}},
	}
	for _, tc := range cases {
		sc, err := Parse(tc.src)
		require.NoError(t, err, tc.src)
		assert.Equal(t, tc.src, sc.Source)
		require.Len(t, sc.Commands, len(tc.want), "%s → %+v", tc.src, sc.Commands)
		for i, w := range tc.want {
			c := sc.Commands[i]
			assert.Equal(t, w.kind, c.Kind, "%s cmd %d kind (%q)", tc.src, i, c.Raw)
			assert.Equal(t, w.key, c.Key, "%s cmd %d key (%q)", tc.src, i, c.Raw)
			assert.Equal(t, w.arg, c.Arg, "%s cmd %d arg (%q)", tc.src, i, c.Raw)
			assert.Equal(t, w.when, c.When, "%s cmd %d when (%q)", tc.src, i, c.Raw)
			assert.GreaterOrEqual(t, c.Pos, 0)
		}
	}
}

func TestParsePermanentFlagAndRaw(t *testing.T) {
	t.Parallel()
	sc, err := Parse(`*HIST=5$HIST=0!MHIST=3oMHIST=1`)
	require.NoError(t, err)
	require.Len(t, sc.Commands, 4)
	assert.True(t, sc.Commands[0].Permanent)
	assert.False(t, sc.Commands[1].Permanent)
	assert.True(t, sc.Commands[2].Permanent)
	assert.False(t, sc.Commands[3].Permanent)
	assert.Equal(t, "oMHIST=1", sc.Commands[3].Raw)
	assert.Equal(t, "!MHIST=3", sc.Commands[2].Raw)
	assert.Equal(t, 0, sc.Commands[0].Pos)
	assert.Equal(t, 7, sc.Commands[1].Pos)
}

func TestParseErrors(t *testing.T) {
	t.Parallel()
	for _, src := range []string{``, `   `, `mV"unterminated`, `*OWNR="x`, `$HIST="x`, `!MHIST="x`, `oMHIST="x`, `!SAVEname`} {
		_, err := Parse(src)
		require.ErrorIs(t, err, ErrSyntax, src)
	}
}

func TestParseUnknownAndEdgeCases(t *testing.T) {
	t.Parallel()
	sc, err := Parse(`@mV$$*=5=5*X^!`)
	require.NoError(t, err)
	assert.Equal(t, []Kind{Unknown, ModeCmd, Unknown, Unknown, Unknown, Unknown, Unknown, MacroCall, Action}, kinds(sc))
	assert.Equal(t, "*=5", sc.Commands[4].Raw, "* with no name swallows its value")
	assert.Equal(t, "=5", sc.Commands[5].Raw, "assignment to a digit")
	assert.Equal(t, "*X", sc.Commands[6].Raw, "* without =")
	assert.Empty(t, sc.Commands[8].Key, "! with nothing after")

	sc, err = Parse("mV\n r4\t p30")
	require.NoError(t, err)
	assert.Equal(t, []Kind{ModeCmd, SettingCmd, SettingCmd}, kinds(sc), "whitespace between commands is tolerated")

	// *FOUR="text with spaces" is metadata, *FOUR="script" with a known extension is an extension,
	// *NAME="script" is a macro, *NAME=5 is metadata.
	sc, err = Parse(`*ABCD="mV!S"*ABCD="hello there"*ABC="x"*ABC=1`)
	require.NoError(t, err)
	assert.Equal(t, []Kind{Macro, Metadata, Macro, Metadata}, kinds(sc))
}

func TestKindString(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "mode", ModeCmd.String())
	assert.Equal(t, "extension", ExtensionCmd.String())
	assert.Equal(t, "unknown", Kind(99).String())
	assert.Equal(t, "unknown", Kind(-1).String())
	b, err := Action.MarshalText()
	require.NoError(t, err)
	assert.Equal(t, "action", string(b))
}

func FuzzParse(f *testing.F) {
	for _, s := range []string{
		`mVr4p60x-.5cFw55$BITR=150$LEVL=6`, `*BOOT="!Lbt"!SAVEbt=!12:00NmP!S!1R`, `!17:00W!GLC`,
		`<08:45!S+"Hello World"+!60E~!08:44N!R`, `=Tt:W=T%6>T1>09:00<17:00mP+!S+!2N+!600RQ!09:00R`,
		`oV1B4D2oFUoTZ-7oMABCD="x"`, `*SUBA="mVr4p60'60p'!S!2E!1N"^SUBA`, `!MTCAL=-20!00;05R!s-30S`, `"`, `!`, `$`, `*`, `=`, `o`, `m`,
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		sc, err := Parse(s)
		if err != nil {
			return
		}
		// Every command must cover a non-empty, in-bounds slice of the source and positions must be ascending.
		last := -1
		for _, c := range sc.Commands {
			if c.Pos < 0 || c.Pos > len(s) {
				t.Fatalf("pos %d out of range for %q", c.Pos, s)
			}
			if c.Pos < last {
				t.Fatalf("positions not ascending in %q", s)
			}
			last = c.Pos
			if c.Raw == "" {
				t.Fatalf("empty raw for %+v in %q", c, s)
			}
		}
		_ = Validate(sc)
		_ = Explain(sc)
	})
}
