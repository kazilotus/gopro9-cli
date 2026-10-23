package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvPath, filepath.Join(dir, "cfg.yaml"))

	f, path, err := Load()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "cfg.yaml"), path)
	assert.Equal(t, File{}, f, "missing file is empty config")

	saved, err := Save(File{Host: "10.5.5.9", Port: 8080, Timeout: 3 * time.Second, Output: "json", Quiet: true})
	require.NoError(t, err)
	assert.Equal(t, path, saved)

	f, _, err = Load()
	require.NoError(t, err)
	assert.Equal(t, "10.5.5.9", f.Host)
	assert.Equal(t, 3*time.Second, f.Timeout)
	assert.True(t, f.Quiet)

	require.NoError(t, os.WriteFile(path, []byte("host: [oops"), 0o600))
	_, _, err = Load()
	require.Error(t, err)
}

func TestDefaultPath(t *testing.T) {
	t.Setenv(EnvPath, "")
	p, err := Path()
	require.NoError(t, err)
	assert.Contains(t, p, filepath.Join("gopro9", "config.yaml"))
}
