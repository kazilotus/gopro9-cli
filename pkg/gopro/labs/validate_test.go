package labs

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustParse(t *testing.T, s string) Script {
	t.Helper()
	sc, err := Parse(s)
	require.NoError(t, err)
	return sc
}

func messages(issues []Issue) string {
	parts := make([]string, 0, len(issues))
	for _, i := range issues {
		parts = append(parts, i.String())
	}
	return strings.Join(parts, "\n")
}

func TestValidateClean(t *testing.T) {
	t.Parallel()
	for _, s := range []string{
		`mVr4p60x-.5cFw55$HIST=6!S!10E`, `mP!S!2N!600R`, `<08:45!S+"Hello World"+!60E~!08:44N!R`,
		`*BOOT="!Lbt"!SAVEbt=!12:00NmP!S!1R`, `!17:00W!GLC`, `*OWNR="Jo"$OWNa="x"`, `$WAKE=1`, `oV1B4D2`, `=A5=B+A^A"ok"`, `!O`,
	} {
		issues := Validate(mustParse(t, s))
		assert.Empty(t, issues, "%s: %s", s, messages(issues))
	}
}

func TestValidateRules(t *testing.T) {
	t.Parallel()
	cases := []struct {
		src      string
		severity string
		contains string
	}{
		{`!s`, SeverityError, "lowercase letter after '!'"},
		{`mP!S!5E`, SeverityError, "photo modes do not use !E"},
		{`mTP!S!5E`, SeverityError, "photo modes do not use !E"},
		{`mV!S!5E!O!R`, SeverityError, "never shut down inside a repeating script"},
		{`!OR!600RQ`, SeverityError, "never shut down"},
		{`!QQ`, SeverityError, "unknown action"},
		{`!`, SeverityError, "needs an action letter"},
		{`mPr4`, SeverityWarning, "video setting"},
		{`mPBp30`, SeverityWarning, "video setting"},
		{`$BITR=150`, SeverityWarning, "not supported on the HERO9"},
		{`$PRXY=1`, SeverityInfo, "inconsistent HERO9 support"},
		{`$DAUD=1`, SeverityInfo, "inconsistent HERO9 support"},
		{`$WAKE=2`, SeverityWarning, "only supports WAKE=1"},
		{`$ZZZZ=1`, SeverityWarning, "not a known Labs extension"},
		{`@`, SeverityError, "unrecognised command"},
		{`m`, SeverityError, "no mode letter"},
		{`r`, SeverityError, "has no value"},
	}
	for _, tc := range cases {
		issues := Validate(mustParse(t, tc.src))
		require.NotEmpty(t, issues, tc.src)
		found := false
		for _, i := range issues {
			if i.Severity == tc.severity && strings.Contains(i.Message, tc.contains) {
				found = true
			}
		}
		assert.True(t, found, "%s: want %s containing %q, got:\n%s", tc.src, tc.severity, tc.contains, messages(issues))
	}
}

func TestValidateExtensionUnknownFourCCViaStar(t *testing.T) {
	t.Parallel()
	// Metadata with an unknown FourCC is allowed but flagged so typos surface.
	issues := Validate(mustParse(t, `*QQQQ=1`))
	require.Len(t, issues, 1)
	assert.Equal(t, SeverityWarning, issues[0].Severity)
	assert.Contains(t, issues[0].Message, "plain metadata")
	// Owner tags are fine.
	assert.Empty(t, Validate(mustParse(t, `*OWNS="x"`)))
}

func TestErrorsFilter(t *testing.T) {
	t.Parallel()
	issues := Validate(mustParse(t, `mPr4!s$BITR=1`))
	errs := Errors(issues)
	assert.Len(t, errs, 1)
	assert.Greater(t, len(issues), len(errs))
	assert.Contains(t, issues[0].String(), "warning at")
}
