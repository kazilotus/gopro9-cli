package media_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kazilotus/gopro9-cli/internal/fakecam"
	"github.com/kazilotus/gopro9-cli/pkg/gopro/media"
	"github.com/kazilotus/gopro9-cli/pkg/gopro/transport"
)

func newClient(t *testing.T) (*media.Client, *fakecam.Server) {
	t.Helper()
	cam := fakecam.New(t)
	return media.New(transport.New(cam.Endpoint(), transport.WithRetries(0, 0))), cam
}

func TestParseListFixture(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "fixtures", "hero9_HD9.01.01.72.70", "gopro_media_list.resp"))
	require.NoError(t, err)
	l, err := media.ParseList(raw)
	require.NoError(t, err)
	assert.Equal(t, "1902797806607713128", l.ID)
	require.Len(t, l.Files, 35)
	f := l.Files[0]
	assert.Equal(t, "100GOPRO", f.Directory)
	assert.Equal(t, "GH050365.MP4", f.Name)
	assert.Equal(t, "100GOPRO/GH050365.MP4", f.Path())
	assert.Equal(t, int64(1336359061), f.Size)
	assert.Equal(t, media.KindVideo, f.Kind)
	assert.Equal(t, "0365", f.Group)
	assert.Equal(t, "05", f.Chapter)
	assert.True(t, f.HasLRV)
	assert.Equal(t, "GL050365.LRV", f.LRVName())
	assert.Equal(t, "GH050365.THM", f.THMName())
	assert.Equal(t, 2022, f.Modified.Year())

	photos := l.Apply(media.Filter{Kinds: []media.Kind{media.KindPhoto}})
	require.Len(t, photos, 2)
	assert.Equal(t, media.KindPhoto, photos[0].Kind)
	assert.Empty(t, photos[0].LRVName())

	since := l.Apply(media.Filter{Since: time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)})
	assert.Len(t, since, 3)
	assert.Empty(t, l.Apply(media.Filter{Directory: "101GOPRO"}))
	g495 := l.Apply(media.Filter{Group: "0495"})
	assert.Len(t, g495, 4, "four chapters")

	groups := l.Groups()
	assert.Less(t, len(groups), len(l.Files), "chapters collapse into groups")
	for _, g := range groups {
		for i := 1; i < len(g); i++ {
			assert.LessOrEqual(t, g[i-1].Chapter, g[i].Chapter)
		}
	}

	got, ok := l.Find("100gopro/gh010516.mp4")
	require.True(t, ok)
	assert.Equal(t, "GH010516.MP4", got.Name)
	_, ok = l.Find("nope")
	assert.False(t, ok)
	_, ok = l.Find("100GOPRO/NOPE.MP4")
	assert.False(t, ok)

	_, err = media.ParseList([]byte("nope"))
	require.Error(t, err)
}

func TestKindsAndNames(t *testing.T) {
	t.Parallel()
	l, err := media.ParseList([]byte(`{"id":"1","media":[{"d":"100GOPRO","fs":[{"n":"GOPR0001.GPR","s":"1"},{"n":"GL010001.LRV","s":"1"},{"n":"GH010001.THM","s":"1"},{"n":"leinfo.sav","s":"1"}]}]}`))
	require.NoError(t, err)
	assert.Equal(t, media.KindRaw, l.Files[0].Kind)
	assert.Equal(t, media.KindLowRes, l.Files[1].Kind)
	assert.Equal(t, media.KindThumbnail, l.Files[2].Kind)
	assert.Equal(t, media.KindOther, l.Files[3].Kind)
	assert.Empty(t, l.Files[3].Group)
	assert.Empty(t, l.Files[3].THMName())
}

func TestListAndBlobs(t *testing.T) {
	t.Parallel()
	c, _ := newClient(t)
	ctx := context.Background()
	l, err := c.List(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, l.Files)
	p := l.Files[0].Path()

	th, err := c.Thumbnail(ctx, p)
	require.NoError(t, err)
	assert.Equal(t, []byte{0xFF, 0xD8}, th[:2])
	sn, err := c.Screennail(ctx, p)
	require.NoError(t, err)
	assert.NotEmpty(t, sn)
	g, err := c.GPMF(ctx, p)
	require.NoError(t, err)
	assert.Equal(t, "DEVC", string(g[:4]))
	tele, err := c.Telemetry(ctx, p)
	require.NoError(t, err)
	assert.Greater(t, len(tele), len(g))

	_, err = c.Thumbnail(ctx, "100GOPRO/NOPE.MP4")
	require.ErrorIs(t, err, transport.ErrNotFound)
	_, err = c.Thumbnail(ctx, "bad")
	require.ErrorIs(t, err, media.ErrBadPath)
	_, err = c.GPMF(ctx, "100GOPRO/GOPR0497.JPG")
	require.Error(t, err, "no GPMF for photos")
}

func TestDelete(t *testing.T) {
	t.Parallel()
	c, cam := newClient(t)
	ctx := context.Background()
	before := len(cam.Visible())
	require.NoError(t, c.DeleteFile(ctx, "100GOPRO/GH010397.MP4"))
	assert.Len(t, cam.Visible(), before-1)
	require.ErrorIs(t, c.DeleteFile(ctx, "100GOPRO/GH010397.MP4"), transport.ErrNotFound)
	require.ErrorIs(t, c.DeleteFile(ctx, "x"), media.ErrBadPath)

	require.NoError(t, c.DeleteGroup(ctx, "100GOPRO/GH010495.MP4"))
	l, err := c.List(ctx)
	require.NoError(t, err)
	assert.Empty(t, l.Apply(media.Filter{Group: "0495"}))
	require.ErrorIs(t, c.DeleteGroup(ctx, "x"), media.ErrBadPath)

	require.NoError(t, c.DeleteAll(ctx))
	l, err = c.List(ctx)
	require.NoError(t, err)
	assert.Empty(t, l.Files)
}

func TestTree(t *testing.T) {
	t.Parallel()
	c, _ := newClient(t)
	ctx := context.Background()
	root, err := c.Tree(ctx, "")
	require.NoError(t, err)
	require.Len(t, root, 1)
	assert.True(t, root[0].IsDir)
	assert.Equal(t, "100GOPRO", root[0].Name)
	files, err := c.Tree(ctx, "100GOPRO")
	require.NoError(t, err)
	assert.Greater(t, len(files), 35, "tree shows companions the media list hides")
	assert.False(t, files[0].IsDir)
	assert.Equal(t, "100GOPRO/GH010397.MP4", files[0].Path)
	names := make([]string, 0, len(files))
	for _, f := range files {
		names = append(names, f.Name)
	}
	assert.Contains(t, names, "GL010397.LRV")
	assert.Contains(t, names, "GH010397.THM")
}

func TestParseDCIMRealFixture(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "fixtures", "hero9_HD9.01.01.72.70", "videos_DCIM_100GOPRO_.resp"))
	require.NoError(t, err)
	entries := media.ParseDCIM(raw, "100GOPRO")
	require.NotEmpty(t, entries)
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name)
		assert.False(t, e.IsDir)
	}
	assert.Contains(t, names, "GH010397.MP4")
	assert.Contains(t, names, "GL010397.LRV", "DCIM tree shows proxies the media list hides")
	assert.NotContains(t, names, "..")

	rootRaw, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "fixtures", "hero9_HD9.01.01.72.70", "videos_DCIM_.resp"))
	require.NoError(t, err)
	root := media.ParseDCIM(rootRaw, "")
	require.Len(t, root, 2)
	assert.Equal(t, "100GOPRO", root[0].Name)
	assert.True(t, root[0].IsDir)
	assert.Equal(t, "leinfo.sav", root[1].Name)
}

func TestSizeAndDownload(t *testing.T) {
	t.Parallel()
	c, cam := newClient(t)
	ctx := context.Background()
	const p = "100GOPRO/GH010397.MP4"
	want, ok := cam.FindMedia(p)
	require.True(t, ok)

	size, err := c.Size(ctx, p)
	require.NoError(t, err)
	assert.Equal(t, want.Size, size)
	_, err = c.Size(ctx, "bad")
	require.ErrorIs(t, err, media.ErrBadPath)
	_, err = c.Size(ctx, "100GOPRO/NOPE.MP4")
	require.Error(t, err)

	dir := t.TempDir()
	dest := filepath.Join(dir, "a.mp4")
	var calls atomic.Int32
	res, err := c.Download(ctx, p, dest, media.DownloadOptions{Verify: true, Progress: func(_ string, done, total int64) {
		calls.Add(1)
		assert.LessOrEqual(t, done, total)
	}})
	require.NoError(t, err)
	assert.Equal(t, want.Size, res.Bytes)
	assert.False(t, res.Resumed)
	assert.Positive(t, calls.Load())
	got, err := os.ReadFile(dest)
	require.NoError(t, err)
	assert.Equal(t, want.Body, got)

	// Complete file is skipped.
	res, err = c.Download(ctx, p, dest, media.DownloadOptions{})
	require.NoError(t, err)
	assert.True(t, res.Skipped)

	// Partial file resumes with a Range request and ends byte-identical.
	require.NoError(t, os.WriteFile(dest, want.Body[:1000], 0o600))
	res, err = c.Download(ctx, p, dest, media.DownloadOptions{Resume: true, Verify: true})
	require.NoError(t, err)
	assert.True(t, res.Resumed)
	got, err = os.ReadFile(dest)
	require.NoError(t, err)
	assert.Equal(t, want.Body, got)
	reqs := strings.Join(cam.Snapshot(), "\n")
	assert.Contains(t, reqs, "/videos/DCIM/"+p)

	// Partial file without resume is overwritten from scratch.
	require.NoError(t, os.WriteFile(dest, want.Body[:10], 0o600))
	res, err = c.Download(ctx, p, dest, media.DownloadOptions{})
	require.NoError(t, err)
	assert.False(t, res.Resumed)
	assert.Equal(t, want.Size, res.Bytes)

	// Overwrite forces a re-download of a complete file.
	res, err = c.Download(ctx, p, dest, media.DownloadOptions{Overwrite: true})
	require.NoError(t, err)
	assert.False(t, res.Skipped)

	_, err = c.Download(ctx, "bad", dest, media.DownloadOptions{})
	require.ErrorIs(t, err, media.ErrBadPath)
	_, err = c.Download(ctx, "100GOPRO/NOPE.MP4", dest, media.DownloadOptions{})
	require.Error(t, err)
}

func TestDownloadMany(t *testing.T) {
	t.Parallel()
	c, _ := newClient(t)
	ctx := context.Background()
	dir := t.TempDir()
	paths := []string{"100GOPRO/GH010397.MP4", "100GOPRO/GH010403.MP4", "100GOPRO/GOPR0497.JPG"}
	results, err := c.DownloadMany(ctx, paths, dir, 2, media.DownloadOptions{Verify: true})
	require.NoError(t, err)
	assert.Len(t, results, 3)
	for _, p := range paths {
		_, err := os.Stat(filepath.Join(dir, filepath.FromSlash(p)))
		require.NoError(t, err)
	}
	_, err = c.DownloadMany(ctx, []string{"100GOPRO/GH010397.MP4", "100GOPRO/NOPE.MP4"}, dir, 0, media.DownloadOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "NOPE")
}
