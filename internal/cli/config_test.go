package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kazilotus/gopro9-cli/internal/config"
	"github.com/kazilotus/gopro9-cli/internal/fakecam"
)

func TestConfigFilePrecedence(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config.yaml")
	t.Setenv(config.EnvPath, cfg)

	out, _, code := run(t, "config", "path")
	require.Equal(t, 0, code)
	assert.Equal(t, cfg+"\n", out)

	out, _, code = run(t, "config", "show", "--output", "plain")
	require.Equal(t, 0, code)
	assert.Equal(t, cfg+"\n", out)

	// File sets output=plain → version prints plain.
	require.NoError(t, os.WriteFile(cfg, []byte("output: plain\n"), 0o600))
	out, _, code = run(t, "version")
	require.Equal(t, 0, code)
	assert.Equal(t, "dev\n", out)

	// Env beats file.
	t.Setenv("GOPRO9_OUTPUT", "json")
	out, _, code = run(t, "version")
	require.Equal(t, 0, code)
	assert.Contains(t, out, `"ok": true`)
	t.Setenv("GOPRO9_OUTPUT", "")
	os.Unsetenv("GOPRO9_OUTPUT")

	// Flag beats file.
	out, _, code = run(t, "version", "--output", "yaml")
	require.Equal(t, 0, code)
	assert.Contains(t, out, "ok: true")

	// Host from file reaches the camera.
	cam := fakecam.New(t)
	ep := cam.Endpoint()
	require.NoError(t, os.WriteFile(cfg, []byte("host: "+ep.Host+"\nport: "+itoa(ep.Port)+"\ntimeout: 2s\nquiet: true\n"), 0o600))
	out, _, code = run(t, "info", "--output", "plain")
	require.Equal(t, 0, code)
	assert.Contains(t, out, "HD9.01.01.72")

	// config set persists the current flags.
	_, _, code = run(t, "config", "set", "--host", "10.5.5.9", "--output", "json")
	require.Equal(t, 0, code)
	f, _, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, "10.5.5.9", f.Host)
	assert.Equal(t, "json", f.Output)
	assert.Equal(t, ep.Port, f.Port, "untouched keys survive")

	// Broken file → config exit code.
	require.NoError(t, os.WriteFile(cfg, []byte("host: [oops"), 0o600))
	_, errOut, code := run(t, "version")
	assert.Equal(t, 78, code)
	assert.Contains(t, errOut, "parse")
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
