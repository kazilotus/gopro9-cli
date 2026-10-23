package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kazilotus/gopro9-cli/internal/exitcode"
)

func run(t *testing.T, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	var out, errb bytes.Buffer
	code = Main(context.Background(), args, Stdio{In: strings.NewReader(""), Out: &out, Err: &errb})
	return out.String(), errb.String(), code
}

func TestVersionTable(t *testing.T) {
	out, errOut, code := run(t, "version")
	assert.Equal(t, 0, code)
	assert.Empty(t, errOut)
	assert.Contains(t, out, "version")
	assert.Contains(t, out, "commit")
}

func TestVersionJSON(t *testing.T) {
	out, _, code := run(t, "version", "--output", "json")
	require.Equal(t, 0, code)
	var env struct {
		OK   bool `json:"ok"`
		Data struct {
			Version string `json:"version"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &env))
	assert.True(t, env.OK)
	assert.NotEmpty(t, env.Data.Version)
}

func TestVersionPlain(t *testing.T) {
	out, _, code := run(t, "version", "--output", "plain")
	assert.Equal(t, 0, code)
	assert.Equal(t, "dev\n", out)
}

func TestOutputFromEnv(t *testing.T) {
	t.Setenv("GOPRO9_OUTPUT", "plain")
	out, _, code := run(t, "version")
	assert.Equal(t, 0, code)
	assert.Equal(t, "dev\n", out)
}

func TestFlagBeatsEnv(t *testing.T) {
	t.Setenv("GOPRO9_OUTPUT", "plain")
	out, _, code := run(t, "version", "--output", "json")
	assert.Equal(t, 0, code)
	assert.True(t, strings.HasPrefix(out, "{"))
}

func TestBadEnvValueIsConfigError(t *testing.T) {
	t.Setenv("GOPRO9_OUTPUT", "xml")
	_, errOut, code := run(t, "version")
	assert.Equal(t, int(exitcode.Software), code)
	assert.Contains(t, errOut, "unknown output format")
}

func TestUnknownCommandIsUsageError(t *testing.T) {
	_, errOut, code := run(t, "nope")
	assert.Equal(t, int(exitcode.Usage), code)
	assert.Contains(t, errOut, "unknown command")
	_, _, code = run(t, "version", "extra-arg")
	assert.Equal(t, int(exitcode.Usage), code)
}

func TestBadFlagIsUsageError(t *testing.T) {
	_, errOut, code := run(t, "version", "--output", "xml")
	assert.Equal(t, int(exitcode.Usage), code)
	assert.Contains(t, errOut, "unknown output format")
	_, errOut, code = run(t, "version", "--bogus")
	assert.Equal(t, int(exitcode.Usage), code)
	assert.Contains(t, errOut, "unknown flag")
}

func TestCompletion(t *testing.T) {
	for _, sh := range []string{"bash", "zsh", "fish", "powershell"} {
		out, _, code := run(t, "completion", sh)
		assert.Equal(t, 0, code, sh)
		assert.NotEmpty(t, out, sh)
	}
	_, _, code := run(t, "completion", "tcsh")
	assert.NotEqual(t, 0, code)
}

func TestDocsMarkdown(t *testing.T) {
	dir := t.TempDir()
	_, _, code := run(t, "docs", "markdown", "--dir", dir)
	require.Equal(t, 0, code)
	out, _, code := run(t, "docs", "man", "--dir", dir)
	require.Equal(t, 0, code, out)
}

func TestExitCodeFor(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	assert.Equal(t, exitcode.Usage, exitCodeFor(ctx, newUsageError("bad %s", "arg")))
	assert.Equal(t, exitcode.Interrupted, exitCodeFor(ctx, context.Canceled))
	assert.Equal(t, exitcode.Unavailable, exitCodeFor(ctx, context.DeadlineExceeded))
	assert.Equal(t, exitcode.Software, exitCodeFor(ctx, errors.New("boom")))

	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	assert.Equal(t, exitcode.Interrupted, exitCodeFor(cancelled, errors.New("io closed")))
}
