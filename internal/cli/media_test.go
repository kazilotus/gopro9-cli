package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kazilotus/gopro9-cli/internal/fakecam"
)

func TestMediaList(t *testing.T) {
	cam := fakecam.New(t)
	out, _, code := runCam(t, cam, "media", "list", "--type", "photo")
	require.Equal(t, 0, code)
	golden(t, "media_list_photo", out)

	out, _, code = runCam(t, cam, "media", "list", "--output", "plain", "--group", "0495")
	require.Equal(t, 0, code)
	assert.Len(t, strings.Split(strings.TrimSpace(out), "\n"), 4)

	out, _, code = runCam(t, cam, "media", "list", "--grouped")
	require.Equal(t, 0, code)
	assert.Contains(t, out, "100GOPRO/0495")
	assert.Contains(t, out, "CHAPTERS")

	out, _, code = runCam(t, cam, "media", "list", "--since", "2024-01-01", "--output", "json")
	require.Equal(t, 0, code)
	assert.Contains(t, out, "GH010516.MP4")
	assert.NotContains(t, out, "GH050365.MP4")

	_, errOut, code := runCam(t, cam, "media", "list", "--type", "gif")
	assert.Equal(t, 64, code)
	assert.Contains(t, errOut, "unknown media type")
	_, _, code = runCam(t, cam, "media", "list", "--since", "yesterday")
	assert.Equal(t, 64, code)
	_, _, code = runCam(t, cam, "media", "list", "--since", "48h")
	assert.Equal(t, 0, code)
}

func TestMediaTree(t *testing.T) {
	cam := fakecam.New(t)
	out, _, code := runCam(t, cam, "media", "tree")
	require.Equal(t, 0, code)
	assert.Contains(t, out, "100GOPRO")
	assert.Contains(t, out, "dir")
	out, _, code = runCam(t, cam, "media", "tree", "100GOPRO", "--output", "plain")
	require.Equal(t, 0, code)
	assert.Contains(t, out, "100GOPRO/GH010397.MP4")
}

func TestMediaDownload(t *testing.T) {
	cam := fakecam.New(t)
	dir := t.TempDir()
	out, errOut, code := runCam(t, cam, "media", "download", "100GOPRO/GH010397.MP4", "100GOPRO/GOPR0497.JPG", "--out", dir)
	require.Equal(t, 0, code, errOut)
	assert.Contains(t, out, "downloaded")
	want, _ := cam.FindMedia("100GOPRO/GH010397.MP4")
	got, err := os.ReadFile(filepath.Join(dir, "100GOPRO", "GH010397.MP4"))
	require.NoError(t, err)
	assert.Equal(t, want.Body, got)
	assert.Contains(t, errOut, "100%", "progress printed to stderr for table output")
	reqs := strings.Join(cam.Snapshot(), "\n")
	assert.Contains(t, reqs, "turbo_transfer?p=1")
	assert.Contains(t, reqs, "turbo_transfer?p=0")
	assert.False(t, cam.Turbo)

	// Second run skips complete files.
	out, _, code = runCam(t, cam, "media", "download", "100GOPRO/GH010397.MP4", "--out", dir, "--output", "json")
	require.Equal(t, 0, code)
	assert.Contains(t, out, `"skipped": 1`)

	// --all with filters, LRV/THM companions, quiet, no turbo.
	out, errOut, code = runCam(t, cam, "media", "download", "--all", "--type", "video", "--group", "0403", "--lrv", "--thm", "--out", dir, "--quiet", "--no-turbo", "--output", "plain")
	require.Equal(t, 0, code, errOut)
	assert.Contains(t, out, "GH010403.MP4")
	assert.Empty(t, errOut)
	reqs = strings.Join(cam.Snapshot(), "\n")
	assert.Contains(t, reqs, "GL010403.LRV")
	assert.Contains(t, reqs, "GH010403.THM")

	_, errOut, code = runCam(t, cam, "media", "download")
	assert.Equal(t, 64, code)
	assert.Contains(t, errOut, "--all")
	_, _, code = runCam(t, cam, "media", "download", "--all", "100GOPRO/X.MP4")
	assert.Equal(t, 64, code)
	_, _, code = runCam(t, cam, "media", "download", "nope", "--out", dir)
	assert.Equal(t, 64, code)
	_, errOut, code = runCam(t, cam, "media", "download", "100GOPRO/NOPE.MP4", "--out", dir)
	assert.Equal(t, 69, code)
	assert.Contains(t, errOut, "NOPE")

	// Nothing matched: empty result, exit 0.
	_, _, code = runCam(t, cam, "media", "download", "--all", "--group", "9999", "--out", dir)
	assert.Equal(t, 0, code)
}

func TestMediaBlobs(t *testing.T) {
	cam := fakecam.New(t)
	dir := t.TempDir()
	for _, sub := range []string{"thumbnail", "screennail", "gpmf", "telemetry"} {
		out := filepath.Join(dir, sub+".bin")
		_, errOut, code := runCam(t, cam, "media", sub, "100GOPRO/GH010397.MP4", "--out", out)
		require.Equal(t, 0, code, "%s: %s", sub, errOut)
		st, err := os.Stat(out)
		require.NoError(t, err)
		assert.Positive(t, st.Size())
	}
	// stdout streaming
	out, _, code := runCam(t, cam, "media", "thumbnail", "100GOPRO/GH010397.MP4", "--out", "-")
	require.Equal(t, 0, code)
	assert.True(t, strings.HasPrefix(out, "\xff\xd8"))
	// default names
	cwd, _ := os.Getwd()
	t.Chdir(dir)
	_, _, code = runCam(t, cam, "media", "gpmf", "100GOPRO/GH010397.MP4")
	require.Equal(t, 0, code)
	_, err := os.Stat(filepath.Join(dir, "GH010397.gpmf"))
	require.NoError(t, err)
	t.Chdir(cwd)

	_, _, code = runCam(t, cam, "media", "thumbnail", "100GOPRO/NOPE.MP4", "--out", "-")
	assert.Equal(t, 69, code)
	_, errOut, code := runCam(t, cam, "media", "telemetry", "100GOPRO/GH010397.MP4", "--format", "gpx", "--out", "-")
	assert.Equal(t, 64, code)
	assert.Contains(t, errOut, "raw only")
	blocked := filepath.Join(dir, "blocked")
	require.NoError(t, os.WriteFile(blocked, nil, 0o600)) // a file where a directory is needed
	_, _, code = runCam(t, cam, "media", "thumbnail", "100GOPRO/GH010397.MP4", "--out", filepath.Join(blocked, "x.jpg"))
	assert.Equal(t, 74, code)
}

func TestMediaHilight(t *testing.T) {
	cam := fakecam.New(t)
	out, _, code := runCam(t, cam, "media", "hilight", "add", "100GOPRO/GH010397.MP4", "--at", "12.5s")
	require.Equal(t, 0, code)
	assert.Contains(t, out, "12.5s")
	assert.Equal(t, []int64{12500}, cam.Hilights["100GOPRO/GH010397.MP4"])
	_, _, code = runCam(t, cam, "media", "hilight", "remove", "100GOPRO/GH010397.MP4", "--at", "12.5s")
	require.Equal(t, 0, code)
	assert.Empty(t, cam.Hilights["100GOPRO/GH010397.MP4"])
	_, errOut, code := runCam(t, cam, "media", "hilight", "moment")
	assert.Equal(t, 69, code)
	assert.Contains(t, errOut, "not supported")
	_, _, code = runCam(t, cam, "media", "hilight", "moment", "--force")
	assert.Equal(t, 69, code, "fake answers 404 like the real camera")
}

func TestMediaDelete(t *testing.T) {
	cam := fakecam.New(t)
	before := len(cam.Visible())
	_, errOut, code := runCam(t, cam, "media", "delete", "file", "100GOPRO/GH010397.MP4")
	assert.Equal(t, 64, code)
	assert.Contains(t, errOut, "--yes")
	assert.Len(t, cam.Visible(), before)

	_, _, code = runCam(t, cam, "media", "delete", "file", "100GOPRO/GH010397.MP4", "100GOPRO/GH010403.MP4", "--yes")
	require.Equal(t, 0, code)
	assert.Len(t, cam.Visible(), before-2)
	_, _, code = runCam(t, cam, "media", "delete", "file", "100GOPRO/NOPE.MP4", "--yes")
	assert.Equal(t, 69, code)

	_, _, code = runCam(t, cam, "media", "delete", "group", "100GOPRO/GH010495.MP4", "-y")
	require.Equal(t, 0, code)
	assert.Len(t, cam.Visible(), before-6)
	_, _, code = runCam(t, cam, "media", "delete", "group", "100GOPRO/GH010495.MP4")
	assert.Equal(t, 64, code)

	_, _, code = runCam(t, cam, "media", "delete", "all")
	assert.Equal(t, 64, code)
	_, _, code = runCam(t, cam, "media", "delete", "all", "--yes", "--output", "plain")
	require.Equal(t, 0, code)
	assert.Empty(t, cam.Visible())
}

func TestHumanBytes(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "512 B", humanBytes(512))
	assert.Equal(t, "1.0 KiB", humanBytes(1024))
	assert.Equal(t, "1.2 GiB", humanBytes(1336359061))
}
