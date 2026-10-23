package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/kazilotus/gopro9-cli/internal/cli/render"
	"github.com/kazilotus/gopro9-cli/internal/exitcode"
	"github.com/kazilotus/gopro9-cli/pkg/gopro"
	"github.com/kazilotus/gopro9-cli/pkg/gopro/media"
)

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return strconv.FormatInt(n, 10) + " B"
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

// --- list -----------------------------------------------------------------

type mediaListResult struct {
	Files []media.File `json:"files" yaml:"files"`
	Total int64        `json:"total_bytes" yaml:"total_bytes"`
}

func (r mediaListResult) Headers() []string {
	return []string{"PATH", "KIND", "SIZE", "MODIFIED", "GROUP", "LRV"}
}

func (r mediaListResult) Rows() [][]string {
	rows := make([][]string, 0, len(r.Files))
	for _, f := range r.Files {
		rows = append(rows, []string{f.Path(), string(f.Kind), humanBytes(f.Size), f.Modified.Local().Format("2006-01-02 15:04"), f.Group, strconv.FormatBool(f.HasLRV)})
	}
	return rows
}

func (r mediaListResult) Plain() string {
	lines := make([]string, 0, len(r.Files))
	for _, f := range r.Files {
		lines = append(lines, f.Path())
	}
	return strings.Join(lines, "\n")
}

type mediaGroupedResult struct {
	Groups [][]media.File `json:"groups" yaml:"groups"`
}

func (r mediaGroupedResult) Headers() []string { return []string{"GROUP", "CHAPTERS", "FIRST", "SIZE"} }

func (r mediaGroupedResult) Rows() [][]string {
	rows := make([][]string, 0, len(r.Groups))
	for _, g := range r.Groups {
		var total int64
		for _, f := range g {
			total += f.Size
		}
		rows = append(rows, []string{g[0].Directory + "/" + g[0].Group, strconv.Itoa(len(g)), g[0].Name, humanBytes(total)})
	}
	return rows
}

func parseKinds(s string) ([]media.Kind, error) {
	if s == "" {
		return nil, nil
	}
	var out []media.Kind
	for _, k := range strings.Split(s, ",") {
		switch strings.ToLower(strings.TrimSpace(k)) {
		case "video", "mp4":
			out = append(out, media.KindVideo)
		case "photo", "jpg":
			out = append(out, media.KindPhoto)
		case "raw", "gpr":
			out = append(out, media.KindRaw)
		case "lrv":
			out = append(out, media.KindLowRes)
		case "thm":
			out = append(out, media.KindThumbnail)
		default:
			return nil, newUsageError("unknown media type %q (want video, photo, raw, lrv or thm)", k)
		}
	}
	return out, nil
}

func parseSince(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	if d, err := time.ParseDuration(s); err == nil {
		return time.Now().Add(-d), nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02"} {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, newUsageError("cannot parse --since %q (want a duration like 24h or a date like 2026-01-31)", s)
}

func (a *App) newMediaCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "media", Short: "List, download, inspect and delete files on the SD card"}
	cmd.AddCommand(
		a.newMediaListCommand(),
		a.newMediaTreeCommand(),
		a.newMediaDownloadCommand(),
		a.newMediaImageCommand("thumbnail", "Save a file's JPEG thumbnail"),
		a.newMediaImageCommand("screennail", "Save a file's larger JPEG preview"),
		a.newMediaGPMFCommand(),
		a.newMediaTelemetryCommand(),
		a.newMediaHilightCommand(),
		a.newMediaDeleteCommand(),
	)
	return cmd
}

type listFlags struct {
	dir, types, since, group string
	grouped                  bool
}

func (f *listFlags) bind(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.dir, "dir", "", "only this DCIM directory (e.g. 100GOPRO)")
	cmd.Flags().StringVar(&f.types, "type", "", "comma-separated kinds: video, photo, raw")
	cmd.Flags().StringVar(&f.since, "since", "", "only files modified after a duration ago (24h) or a date (2026-01-31)")
	cmd.Flags().StringVar(&f.group, "group", "", "only chapters of this 4-digit capture group")
}

func (f *listFlags) filter() (media.Filter, error) {
	kinds, err := parseKinds(f.types)
	if err != nil {
		return media.Filter{}, err
	}
	since, err := parseSince(f.since)
	if err != nil {
		return media.Filter{}, err
	}
	return media.Filter{Directory: f.dir, Kinds: kinds, Since: since, Group: f.group}, nil
}

func (a *App) newMediaListCommand() *cobra.Command {
	var lf listFlags
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List media files (videos, photos) with sizes and chapter groups",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			filter, err := lf.filter()
			if err != nil {
				return err
			}
			cam, err := a.connect(cmd.Context())
			if err != nil {
				return err
			}
			l, err := cam.Media.List(cmd.Context())
			if err != nil {
				return wrapCameraErr(err)
			}
			files := l.Apply(filter)
			if lf.grouped {
				return a.print(cmd, mediaGroupedResult{Groups: media.List{Files: files}.Groups()})
			}
			res := mediaListResult{Files: files}
			for _, f := range files {
				res.Total += f.Size
			}
			return a.print(cmd, res)
		},
	}
	lf.bind(cmd)
	cmd.Flags().BoolVar(&lf.grouped, "grouped", false, "collapse chapters into one row per recording")
	return cmd
}

type treeResult struct {
	Entries []media.Entry `json:"entries" yaml:"entries"`
}

func (r treeResult) Headers() []string { return []string{"PATH", "TYPE"} }
func (r treeResult) Rows() [][]string {
	rows := make([][]string, 0, len(r.Entries))
	for _, e := range r.Entries {
		kind := "file"
		if e.IsDir {
			kind = "dir"
		}
		rows = append(rows, []string{e.Path, kind})
	}
	return rows
}

func (r treeResult) Plain() string {
	lines := make([]string, 0, len(r.Entries))
	for _, e := range r.Entries {
		lines = append(lines, e.Path)
	}
	return strings.Join(lines, "\n")
}

func (a *App) newMediaTreeCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "tree [dir]",
		Short: "Walk the raw DCIM file server (shows LRV/THM proxies and camera databases)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := ""
			if len(args) == 1 {
				dir = args[0]
			}
			cam, err := a.connect(cmd.Context())
			if err != nil {
				return err
			}
			entries, err := cam.Media.Tree(cmd.Context(), dir)
			if err != nil {
				return wrapCameraErr(err)
			}
			return a.print(cmd, treeResult{Entries: entries})
		},
	}
}

// --- download -------------------------------------------------------------

type downloadResult struct {
	Downloaded []media.DownloadResult `json:"downloaded" yaml:"downloaded"`
	Bytes      int64                  `json:"bytes" yaml:"bytes"`
	Skipped    int                    `json:"skipped" yaml:"skipped"`
}

func (r downloadResult) Headers() []string { return []string{"SOURCE", "DEST", "SIZE", "STATUS"} }
func (r downloadResult) Rows() [][]string {
	rows := make([][]string, 0, len(r.Downloaded))
	for _, d := range r.Downloaded {
		status := "downloaded"
		switch {
		case d.Skipped:
			status = "skipped (complete)"
		case d.Resumed:
			status = "resumed"
		}
		rows = append(rows, []string{d.Source, d.Dest, humanBytes(d.Bytes), status})
	}
	return rows
}

func (r downloadResult) Plain() string {
	lines := make([]string, 0, len(r.Downloaded))
	for _, d := range r.Downloaded {
		lines = append(lines, d.Dest)
	}
	return strings.Join(lines, "\n")
}

func (a *App) newMediaDownloadCommand() *cobra.Command {
	var (
		lf        listFlags
		out       string
		all       bool
		withLRV   bool
		withTHM   bool
		parallel  int
		noResume  bool
		overwrite bool
		noVerify  bool
		noTurbo   bool
	)
	cmd := &cobra.Command{
		Use:   "download [DIR/NAME ...]",
		Short: "Download media files, resumable, with bounded parallelism",
		Long: `Download the given files (as "100GOPRO/GH010397.MP4") or, with --all, every
file matching the filters. Files are written under --out preserving the DCIM
directory. Partial files are resumed by default and sizes are verified
against the camera. Turbo transfer is enabled for the duration and restored.`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && !all {
				return newUsageError("give at least one DIR/NAME or --all")
			}
			if len(args) > 0 && all {
				return newUsageError("--all cannot be combined with explicit paths")
			}
			filter, err := lf.filter()
			if err != nil {
				return err
			}
			cam, err := a.connect(cmd.Context())
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			paths := args
			var list media.List
			if all || withLRV || withTHM {
				list, err = cam.Media.List(ctx)
				if err != nil {
					return wrapCameraErr(err)
				}
			}
			if all {
				paths = paths[:0]
				for _, f := range list.Apply(filter) {
					paths = append(paths, f.Path())
				}
			}
			paths = expandProxies(paths, list, withLRV, withTHM)
			if len(paths) == 0 {
				return a.print(cmd, downloadResult{})
			}
			if !noTurbo {
				if terr := cam.OGP.TurboTransfer(ctx, true); terr == nil {
					defer func() { _ = cam.OGP.TurboTransfer(cmd.Context(), false) }()
				}
			}
			opts := media.DownloadOptions{Resume: !noResume, Verify: !noVerify, Overwrite: overwrite}
			if !a.opts.Quiet && a.opts.Output == render.FormatTable {
				opts.Progress = a.progressPrinter(cmd)
			}
			results, err := cam.Media.DownloadMany(ctx, paths, out, parallel, opts)
			res := downloadResult{Downloaded: results}
			for _, r := range results {
				res.Bytes += r.Bytes
				if r.Skipped {
					res.Skipped++
				}
			}
			if err != nil {
				if perr := a.print(cmd, res); perr != nil {
					return perr
				}
				return downloadErr(err)
			}
			return a.print(cmd, res)
		},
	}
	lf.bind(cmd)
	cmd.Flags().StringVar(&out, "out", ".", "destination directory")
	cmd.Flags().BoolVar(&all, "all", false, "download every file matching the filters")
	cmd.Flags().BoolVar(&withLRV, "lrv", false, "also fetch the low-resolution .LRV proxy of each video")
	cmd.Flags().BoolVar(&withTHM, "thm", false, "also fetch the .THM thumbnail of each file")
	cmd.Flags().IntVar(&parallel, "parallel", 2, "concurrent transfers (the camera degrades above 2)")
	cmd.Flags().BoolVar(&noResume, "no-resume", false, "restart partial files instead of resuming")
	cmd.Flags().BoolVar(&overwrite, "overwrite", false, "re-download files that already exist complete")
	cmd.Flags().BoolVar(&noVerify, "no-verify", false, "skip the final size check")
	cmd.Flags().BoolVar(&noTurbo, "no-turbo", false, "do not toggle turbo transfer")
	return cmd
}

// expandProxies appends LRV/THM companions for the selected paths.
func expandProxies(paths []string, list media.List, lrv, thm bool) []string {
	if !lrv && !thm {
		return paths
	}
	out := make([]string, 0, len(paths)*2)
	for _, p := range paths {
		out = append(out, p)
		f, ok := list.Find(p)
		if !ok {
			continue
		}
		if lrv && f.HasLRV && f.LRVName() != "" {
			out = append(out, f.Directory+"/"+f.LRVName())
		}
		if thm && f.THMName() != "" {
			out = append(out, f.Directory+"/"+f.THMName())
		}
	}
	return out
}

func downloadErr(err error) error {
	var pe *os.PathError
	if errors.As(err, &pe) {
		return &cameraError{err: err, code: exitcode.IOErr}
	}
	if errors.Is(err, media.ErrSizeMismatch) {
		return &cameraError{err: err, code: exitcode.DataErr}
	}
	if errors.Is(err, media.ErrBadPath) {
		return newUsageError("%v", err)
	}
	return wrapCameraErr(err)
}

// progressPrinter writes a progress line per file to stderr. It is called
// concurrently from parallel downloads, hence the mutex.
func (a *App) progressPrinter(cmd *cobra.Command) media.Progress {
	var mu sync.Mutex
	last := map[string]int64{}
	return func(path string, done, total int64) {
		mu.Lock()
		defer mu.Unlock()
		if total > 0 && done < total && done-last[path] < 4<<20 {
			return // throttle to every 4 MiB
		}
		last[path] = done
		pct := int64(0)
		if total > 0 {
			pct = done * 100 / total
		}
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "%s %3d%% (%s / %s)\n", path, pct, humanBytes(done), humanBytes(total))
	}
}

// --- thumbnail / screennail / gpmf / telemetry ----------------------------

type savedResult struct {
	Source string `json:"source" yaml:"source"`
	Dest   string `json:"dest" yaml:"dest"`
	Bytes  int    `json:"bytes" yaml:"bytes"`
}

func (r savedResult) Headers() []string { return nil }
func (r savedResult) Rows() [][]string {
	return render.KV{{"source", r.Source}, {"dest", r.Dest}, {"bytes", strconv.Itoa(r.Bytes)}}.Rows()
}
func (r savedResult) Plain() string { return r.Dest }

// writeBlob writes data to --out, or to stdout when out is "-".
func (a *App) writeBlob(cmd *cobra.Command, source, out string, data []byte) error {
	if out == "-" {
		_, err := cmd.OutOrStdout().Write(data)
		return err
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o750); err != nil {
		return &cameraError{err: err, code: exitcode.IOErr}
	}
	if err := os.WriteFile(out, data, 0o644); err != nil { //nolint:gosec // media files are meant to be readable
		return &cameraError{err: err, code: exitcode.IOErr}
	}
	return a.print(cmd, savedResult{Source: source, Dest: out, Bytes: len(data)})
}

func defaultOut(p, suffix string) string {
	base := filepath.Base(filepath.FromSlash(p))
	return strings.TrimSuffix(base, filepath.Ext(base)) + suffix
}

func (a *App) newMediaImageCommand(name, short string) *cobra.Command {
	var out string
	cmd := &cobra.Command{
		Use:   name + " <DIR/NAME>",
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cam, err := a.connect(cmd.Context())
			if err != nil {
				return err
			}
			fetch := cam.Media.Thumbnail
			if name == "screennail" {
				fetch = cam.Media.Screennail
			}
			data, err := fetch(cmd.Context(), args[0])
			if err != nil {
				return downloadErr(err)
			}
			if out == "" {
				out = defaultOut(args[0], "."+name+".jpg")
			}
			return a.writeBlob(cmd, args[0], out, data)
		},
	}
	cmd.Flags().StringVar(&out, "out", "", "output file (default <name>."+name+".jpg; - for stdout)")
	return cmd
}

func (a *App) newMediaGPMFCommand() *cobra.Command {
	var out string
	cmd := &cobra.Command{
		Use:   "gpmf <DIR/NAME>",
		Short: "Save a video's raw GPMF metadata header",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cam, err := a.connect(cmd.Context())
			if err != nil {
				return err
			}
			data, err := cam.Media.GPMF(cmd.Context(), args[0])
			if err != nil {
				return downloadErr(err)
			}
			if out == "" {
				out = defaultOut(args[0], ".gpmf")
			}
			return a.writeBlob(cmd, args[0], out, data)
		},
	}
	cmd.Flags().StringVar(&out, "out", "", "output file (default <name>.gpmf; - for stdout)")
	return cmd
}

// --- hilight --------------------------------------------------------------

func (a *App) newMediaHilightCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "hilight", Short: "Add or remove HiLight tags"}
	var at time.Duration
	add := &cobra.Command{
		Use:   "add <DIR/NAME>",
		Short: "Add a HiLight tag to an existing video at --at",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.hilight(cmd, args[0], at, true)
		},
	}
	add.Flags().DurationVar(&at, "at", 0, "offset into the video (e.g. 12.5s)")
	var rmAt time.Duration
	remove := &cobra.Command{
		Use:   "remove <DIR/NAME>",
		Short: "Remove a HiLight tag from an existing video at --at",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.hilight(cmd, args[0], rmAt, false)
		},
	}
	remove.Flags().DurationVar(&rmAt, "at", 0, "offset of the tag to remove")
	moment := &cobra.Command{
		Use:   "moment",
		Short: "Tag the current moment while recording (not available on HERO9 firmware 1.72)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := a.requireCapability(cmd, "OGP_TAG_MOMENT"); err != nil {
				return err
			}
			cam, err := a.connect(cmd.Context())
			if err != nil {
				return err
			}
			if err := cam.OGP.HilightMoment(cmd.Context()); err != nil {
				return wrapCameraErr(err)
			}
			return a.print(cmd, okResult{Action: "hilight moment"})
		},
	}
	addForceFlag(moment)
	cmd.AddCommand(add, remove, moment)
	return cmd
}

func (a *App) hilight(cmd *cobra.Command, p string, at time.Duration, add bool) error {
	cam, err := a.connect(cmd.Context())
	if err != nil {
		return err
	}
	fn := cam.OGP.HilightFile
	action := "hilight add"
	if !add {
		fn = cam.OGP.HilightRemove
		action = "hilight remove"
	}
	if err := fn(cmd.Context(), p, at); err != nil {
		return downloadErr(err)
	}
	return a.print(cmd, okResult{Action: action, Detail: fmt.Sprintf("%s @ %s", p, at)})
}

// --- delete ---------------------------------------------------------------

// confirm refuses destructive operations unless --yes was given. There is
// deliberately no interactive prompt: the CLI is meant to be scriptable and
// the camera offers no undo.
func (a *App) confirm(what string) error {
	if a.opts.Yes {
		return nil
	}
	return &cameraError{err: fmt.Errorf("refusing to %s without --yes", what), code: exitcode.Usage}
}

func (a *App) newMediaDeleteCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "delete", Short: "Delete files from the SD card (requires --yes)"}
	file := &cobra.Command{
		Use:   "file <DIR/NAME ...>",
		Short: "Delete one or more files",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.confirm(fmt.Sprintf("delete %d file(s)", len(args))); err != nil {
				return err
			}
			cam, err := a.connect(cmd.Context())
			if err != nil {
				return err
			}
			for _, p := range args {
				if err := cam.Media.DeleteFile(cmd.Context(), p); err != nil {
					return downloadErr(fmt.Errorf("%s: %w", p, err))
				}
			}
			return a.print(cmd, okResult{Action: "delete", Detail: strconv.Itoa(len(args)) + " file(s)"})
		},
	}
	group := &cobra.Command{
		Use:   "group <DIR/NAME>",
		Short: "Delete every chapter of the recording that contains this file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.confirm("delete a recording group"); err != nil {
				return err
			}
			cam, err := a.connect(cmd.Context())
			if err != nil {
				return err
			}
			if err := cam.Media.DeleteGroup(cmd.Context(), args[0]); err != nil {
				return downloadErr(err)
			}
			return a.print(cmd, okResult{Action: "delete group", Detail: args[0]})
		},
	}
	all := &cobra.Command{
		Use:   "all",
		Short: "Erase every file on the SD card",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := a.confirm("erase all media"); err != nil {
				return err
			}
			cam, err := a.connect(cmd.Context())
			if err != nil {
				return err
			}
			if err := cam.Media.DeleteAll(cmd.Context()); err != nil {
				return wrapCameraErr(err)
			}
			return a.print(cmd, okResult{Action: "delete all"})
		},
	}
	cmd.AddCommand(file, group, all)
	return cmd
}

var _ = gopro.New
