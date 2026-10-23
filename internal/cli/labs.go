package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/kazilotus/gopro9-cli/internal/cli/render"
	"github.com/kazilotus/gopro9-cli/internal/exitcode"
	"github.com/kazilotus/gopro9-cli/pkg/gopro/labs"
)

// qrFlags are shared by every command that renders a QR code.
type qrFlags struct {
	png, svg string
	size     int
	invert   bool
	noQR     bool
}

func (f *qrFlags) bind(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.png, "png", "", "write the QR code as PNG to this file")
	cmd.Flags().StringVar(&f.svg, "svg", "", "write the QR code as SVG to this file")
	cmd.Flags().IntVar(&f.size, "size", 512, "PNG size in pixels")
	cmd.Flags().BoolVar(&f.invert, "invert", false, "invert the terminal rendering for light backgrounds")
	cmd.Flags().BoolVar(&f.noQR, "no-qr", false, "print only the command string, no QR code")
}

type labsCommandResult struct {
	Command string       `json:"command" yaml:"command"`
	Issues  []labs.Issue `json:"issues,omitempty" yaml:"issues,omitempty"`
	PNG     string       `json:"png,omitempty" yaml:"png,omitempty"`
	SVG     string       `json:"svg,omitempty" yaml:"svg,omitempty"`
}

func (r labsCommandResult) Headers() []string { return nil }
func (r labsCommandResult) Rows() [][]string {
	kv := render.KV{{"command", r.Command}}
	if r.PNG != "" {
		kv = append(kv, [2]string{"png", r.PNG})
	}
	if r.SVG != "" {
		kv = append(kv, [2]string{"svg", r.SVG})
	}
	for _, i := range r.Issues {
		kv = append(kv, [2]string{i.Severity, i.Message})
	}
	return kv.Rows()
}
func (r labsCommandResult) Plain() string { return r.Command }

// renderLabs validates a command string, writes requested QR files, prints the
// terminal QR (table output only) and reports issues. Validation errors fail.
func (a *App) renderLabs(cmd *cobra.Command, command string, f qrFlags) error {
	sc, err := labs.Parse(command)
	if err != nil {
		return newUsageError("%v", err)
	}
	issues := labs.Validate(sc)
	res := labsCommandResult{Command: command, Issues: issues}
	if errs := labs.Errors(issues); len(errs) > 0 {
		for _, i := range errs {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "error: %s\n", i.Message)
		}
		return &cameraError{err: fmt.Errorf("labs command has %d error(s)", len(errs)), code: exitcode.DataErr}
	}
	if !a.opts.Quiet {
		for _, i := range issues {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "%s: %s\n", i.Severity, i.Message)
		}
	}
	if f.png != "" {
		png, err := labs.QRPNG(command, f.size)
		if err != nil {
			return &cameraError{err: err, code: exitcode.DataErr}
		}
		if err := os.WriteFile(f.png, png, 0o644); err != nil { //nolint:gosec // user-chosen output image
			return &cameraError{err: err, code: exitcode.IOErr}
		}
		res.PNG = f.png
	}
	if f.svg != "" {
		svg, err := labs.QRSVG(command)
		if err != nil {
			return &cameraError{err: err, code: exitcode.DataErr}
		}
		if err := os.WriteFile(f.svg, []byte(svg), 0o644); err != nil { //nolint:gosec // user-chosen output image
			return &cameraError{err: err, code: exitcode.IOErr}
		}
		res.SVG = f.svg
	}
	if a.opts.Output == render.FormatTable && !f.noQR {
		ansi, err := labs.QRANSI(command, f.invert)
		if err != nil {
			return &cameraError{err: err, code: exitcode.DataErr}
		}
		if _, err := fmt.Fprintln(cmd.OutOrStdout(), ansi); err != nil {
			return err
		}
	}
	return a.print(cmd, res)
}

func (a *App) newLabsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "labs",
		Short: "GoPro Labs command language: build, validate, explain and render QR codes",
		Long: `GoPro Labs firmware adds a scripting language entered by showing the camera a
QR code. These commands need no USB connection: they compose and validate
command strings and render them as QR codes (terminal, PNG or SVG) for the
camera to scan from your screen. The camera can read QR codes while cabled
and recording, so "gopro9 state --watch" can confirm the effect.`,
	}
	cmd.AddCommand(a.newLabsQRCommand(), a.newLabsTimeCommand(), a.newLabsBuildCommand(), a.newLabsValidateCommand(), a.newLabsExplainCommand(), a.newLabsExtensionsCommand())
	return cmd
}

func (a *App) newLabsQRCommand() *cobra.Command {
	var f qrFlags
	cmd := &cobra.Command{
		Use:   "qr <command>",
		Short: "Render a Labs command string as a QR code",
		Example: `  gopro9 labs qr 'mVr4p60!S!10E'            # 4K60 video, record 10 s
  gopro9 labs qr '!MBOOT="!Lbt"' --png boot.png`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error { return a.renderLabs(cmd, args[0], f) },
	}
	f.bind(cmd)
	return cmd
}

func (a *App) newLabsTimeCommand() *cobra.Command {
	var (
		f         qrFlags
		utc       bool
		precision bool
		at        string
	)
	cmd := &cobra.Command{
		Use:   "time",
		Short: "Render the Labs time-sync QR code (sets the camera clock when scanned)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			t := a.now()
			if at != "" {
				parsed, err := parseLocalTime(at)
				if err != nil {
					return newUsageError("%v", err)
				}
				t = parsed
			}
			if utc {
				t = t.UTC()
			} else {
				t = t.Local()
			}
			return a.renderLabs(cmd, labs.TimeSyncCommand(t, precision), f)
		},
	}
	f.bind(cmd)
	cmd.Flags().BoolVar(&utc, "utc", false, "encode UTC instead of local wall time")
	cmd.Flags().BoolVar(&precision, "precision", true, "include milliseconds")
	cmd.Flags().StringVar(&at, "at", "", "encode this time instead of now (RFC3339 or YYYY-MM-DDTHH:MM:SS)")
	return cmd
}

func (a *App) newLabsBuildCommand() *cobra.Command {
	var (
		f                                   qrFlags
		mode, res, lens, color, hs, wb, iso string
		fps                                 int
		interval                            float64
		ev                                  float64
		gps                                 string
		startIn, duration, repeat           time.Duration
		startAt, message                    string
		exts, metas, raws                   []string
	)
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Compose a Labs command from typed flags and render it",
		Example: `  gopro9 labs build --mode video --res 4K --fps 60 --lens linear --ev -0.5 --start-in 10s --duration 30s
  gopro9 labs build --mode photo --start-at 06:30 --repeat 10m --message "Sunrise"
  gopro9 labs build --ext HIST=1 --ext BURN=1 --meta OWNR="Jane Doe"`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			b := &labs.Builder{}
			if mode != "" {
				m, err := parseLabsMode(mode)
				if err != nil {
					return err
				}
				b.Mode(m)
			}
			if res != "" {
				b.Resolution(res)
			}
			if fps > 0 {
				b.FPS(fps)
			}
			if interval > 0 {
				b.Interval(interval)
			}
			if lens != "" {
				l, err := parseLabsLens(lens)
				if err != nil {
					return err
				}
				b.Lens(l)
			}
			if cmd.Flags().Changed("ev") {
				b.EV(ev)
			}
			if wb != "" {
				k, err := strconv.Atoi(strings.TrimSuffix(strings.ToLower(wb), "k"))
				if err != nil {
					return newUsageError("--wb must be kelvin (e.g. 5500): %v", err)
				}
				b.WhiteBalance(k)
			}
			if iso != "" {
				lo, hi, ok := strings.Cut(iso, ":")
				minV, err1 := strconv.Atoi(lo)
				maxV, err2 := strconv.Atoi(hi)
				if !ok || err1 != nil || err2 != nil {
					return newUsageError("--iso must be MIN:MAX in ISO units (e.g. 100:1600)")
				}
				b.ISO(minV/100, maxV/100)
			}
			if color != "" {
				c, err := parseLabsColor(color)
				if err != nil {
					return err
				}
				b.Color(c)
			}
			if hs != "" {
				h, err := parseLabsHyperSmooth(hs)
				if err != nil {
					return err
				}
				b.HyperSmooth(h)
			}
			if gps != "" {
				switch strings.ToLower(gps) {
				case "on", "true", "1":
					b.GPS(true)
				case "off", "false", "0":
					b.GPS(false)
				default:
					return newUsageError("--gps must be on or off")
				}
			}
			for _, e := range exts {
				k, v, _ := strings.Cut(e, "=")
				if len(k) != 4 {
					return newUsageError("--ext wants FOURCC=value, got %q", e)
				}
				b.Extension(strings.ToUpper(k), v)
			}
			for _, m := range metas {
				k, v, _ := strings.Cut(m, "=")
				if len(k) != 4 {
					return newUsageError("--meta wants FOURCC=value, got %q", m)
				}
				b.Metadata(strings.ToUpper(k), v, true)
			}
			for _, r := range raws {
				b.Raw(r)
			}
			switch {
			case startAt != "":
				b.StartAt(startAt)
			case startIn > 0:
				b.StartIn(startIn)
			case duration > 0 || repeat > 0:
				b.Start()
			}
			if duration > 0 {
				b.Duration(duration)
			}
			if repeat > 0 {
				b.Repeat(repeat)
			}
			if message != "" {
				b.Message(message)
			}
			command := b.String()
			if command == "" {
				return newUsageError("nothing to build: pass at least one flag (see --help)")
			}
			return a.renderLabs(cmd, command, f)
		},
	}
	f.bind(cmd)
	fl := cmd.Flags()
	fl.StringVar(&mode, "mode", "", "video, video-loop, photo, burst, night, timelapse, timelapse-photo, timewarp, nightlapse, nightlapse-photo, liveburst")
	fl.StringVar(&res, "res", "", "resolution token: 4K, 2.7K, 1440, 1080, 4K4:3, 5K ...")
	fl.IntVar(&fps, "fps", 0, "frames per second")
	fl.Float64Var(&interval, "interval", 0, "time-lapse interval in seconds (0.5, 2, 60 ...)")
	fl.StringVar(&lens, "lens", "", "wide, narrow, medium, linear, superview, max-superview, horizon, max")
	fl.Float64Var(&ev, "ev", 0, "exposure compensation (-2.0 .. 2.0)")
	fl.StringVar(&wb, "wb", "", "white balance in kelvin (2300..6500) or 'auto'/'native' via --raw wA/wN")
	fl.StringVar(&iso, "iso", "", "ISO range MIN:MAX, e.g. 100:1600")
	fl.StringVar(&color, "color", "", "gopro, natural, flat")
	fl.StringVar(&hs, "hypersmooth", "", "off, on, high, boost, auto")
	fl.StringVar(&gps, "gps", "", "on or off")
	fl.DurationVar(&startIn, "start-in", 0, "start capture after this delay")
	fl.StringVar(&startAt, "start-at", "", "start capture at this clock time (HH:MM)")
	fl.DurationVar(&duration, "duration", 0, "stop capture after this long")
	fl.DurationVar(&repeat, "repeat", 0, "repeat the whole script every interval")
	fl.StringVar(&message, "message", "", "text to show on the camera screen")
	fl.StringArrayVar(&exts, "ext", nil, "Labs extension FOURCC=value (repeatable), e.g. HIST=1")
	fl.StringArrayVar(&metas, "meta", nil, "permanent metadata FOURCC=value (repeatable), e.g. OWNR=Name")
	fl.StringArrayVar(&raws, "raw", nil, "raw token appended verbatim (repeatable)")
	return cmd
}

func parseLabsMode(s string) (labs.Mode, error) {
	switch strings.ToLower(s) {
	case "video", "v":
		return labs.ModeVideo, nil
	case "video-loop", "loop":
		return labs.ModeVideoLooping, nil
	case "photo", "p":
		return labs.ModePhoto, nil
	case "burst":
		return labs.ModePhotoBurst, nil
	case "night":
		return labs.ModePhotoNight, nil
	case "timelapse", "timelapse-video":
		return labs.ModeTimelapseVideo, nil
	case "timelapse-photo":
		return labs.ModeTimelapsePhoto, nil
	case "timewarp":
		return labs.ModeTimeWarp, nil
	case "nightlapse":
		return labs.ModeNightLapse, nil
	case "nightlapse-photo":
		return labs.ModeNightLapsePhot, nil
	case "liveburst":
		return labs.ModeLiveBurst, nil
	}
	return "", newUsageError("unknown --mode %q", s)
}

func parseLabsLens(s string) (labs.Lens, error) {
	switch strings.ToLower(s) {
	case "wide":
		return labs.LensWide, nil
	case "narrow":
		return labs.LensNarrow, nil
	case "medium":
		return labs.LensMedium, nil
	case "linear":
		return labs.LensLinear, nil
	case "superview":
		return labs.LensSuperView, nil
	case "max-superview":
		return labs.LensMaxSuperView, nil
	case "horizon", "horizon-leveling":
		return labs.LensHorizonLevel, nil
	case "max":
		return labs.LensMax, nil
	}
	return "", newUsageError("unknown --lens %q", s)
}

func parseLabsColor(s string) (labs.Color, error) {
	switch strings.ToLower(s) {
	case "gopro":
		return labs.ColorGoPro, nil
	case "natural":
		return labs.ColorNatural, nil
	case "flat":
		return labs.ColorFlat, nil
	}
	return "", newUsageError("unknown --color %q", s)
}

func parseLabsHyperSmooth(s string) (labs.HyperSmooth, error) {
	switch strings.ToLower(s) {
	case "off":
		return labs.HyperSmoothOff, nil
	case "on":
		return labs.HyperSmoothOn, nil
	case "high":
		return labs.HyperSmoothHigh, nil
	case "boost":
		return labs.HyperSmoothBoost, nil
	case "auto":
		return labs.HyperSmoothAuto, nil
	}
	return "", newUsageError("unknown --hypersmooth %q", s)
}

type labsIssuesResult struct {
	Command string       `json:"command" yaml:"command"`
	Issues  []labs.Issue `json:"issues" yaml:"issues"`
	OK      bool         `json:"ok" yaml:"ok"`
}

func (r labsIssuesResult) Headers() []string { return []string{"SEVERITY", "POS", "MESSAGE"} }
func (r labsIssuesResult) Rows() [][]string {
	if len(r.Issues) == 0 {
		return [][]string{{"ok", "", "no issues"}}
	}
	rows := make([][]string, 0, len(r.Issues))
	for _, i := range r.Issues {
		rows = append(rows, []string{i.Severity, strconv.Itoa(i.Pos), i.Message})
	}
	return rows
}

func (r labsIssuesResult) Plain() string {
	if r.OK {
		return "ok"
	}
	return "invalid"
}

func (a *App) newLabsValidateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "validate <command>",
		Short: "Check a Labs command for syntax errors and HERO9 support",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sc, err := labs.Parse(args[0])
			if err != nil {
				return newUsageError("%v", err)
			}
			issues := labs.Validate(sc)
			res := labsIssuesResult{Command: args[0], Issues: issues, OK: len(labs.Errors(issues)) == 0}
			if perr := a.print(cmd, res); perr != nil {
				return perr
			}
			if !res.OK {
				return &cameraError{err: fmt.Errorf("labs command has %d error(s)", len(labs.Errors(issues))), code: exitcode.DataErr}
			}
			return nil
		},
	}
}

type labsExplainResult struct {
	Command string   `json:"command" yaml:"command"`
	Lines   []string `json:"lines" yaml:"lines"`
}

func (r labsExplainResult) Headers() []string { return nil }
func (r labsExplainResult) Rows() [][]string {
	rows := make([][]string, 0, len(r.Lines))
	for _, l := range r.Lines {
		tok, desc, _ := strings.Cut(l, " → ")
		rows = append(rows, []string{tok, desc})
	}
	return rows
}
func (r labsExplainResult) Plain() string { return strings.Join(r.Lines, "\n") }

func (a *App) newLabsExplainCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "explain <command>",
		Short: "Translate a Labs command into plain English, one line per token",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sc, err := labs.Parse(args[0])
			if err != nil {
				return newUsageError("%v", err)
			}
			return a.print(cmd, labsExplainResult{Command: args[0], Lines: labs.Explain(sc)})
		},
	}
}

type labsExtensionsResult struct {
	Extensions []labs.Extension `json:"extensions" yaml:"extensions"`
}

func (r labsExtensionsResult) Headers() []string {
	return []string{"FOURCC", "HERO9", "VALUE", "DESCRIPTION"}
}

func (r labsExtensionsResult) Rows() [][]string {
	rows := make([][]string, 0, len(r.Extensions))
	for _, e := range r.Extensions {
		rows = append(rows, []string{e.FourCC, e.Support.String(), e.Value.String(), e.Description})
	}
	return rows
}

func (a *App) newLabsExtensionsCommand() *cobra.Command {
	var supported bool
	cmd := &cobra.Command{
		Use:   "extensions",
		Short: "List Labs extension FourCCs and their HERO9 support",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			res := labsExtensionsResult{}
			for _, e := range labs.Extensions() {
				if supported && e.Support != labs.Supported {
					continue
				}
				res.Extensions = append(res.Extensions, e)
			}
			return a.print(cmd, res)
		},
	}
	cmd.Flags().BoolVar(&supported, "supported", false, "only extensions the HERO9 Labs firmware supports")
	return cmd
}
