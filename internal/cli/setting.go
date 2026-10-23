package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/kazilotus/gopro9-cli/internal/exitcode"
	"github.com/kazilotus/gopro9-cli/pkg/gopro/ogp"
	"github.com/kazilotus/gopro9-cli/pkg/gopro/settings"
	"github.com/kazilotus/gopro9-cli/pkg/gopro/transport"
)

type settingRow struct {
	ID      settings.SettingID `json:"id" yaml:"id"`
	Name    string             `json:"name" yaml:"name"`
	Title   string             `json:"title" yaml:"title"`
	Value   string             `json:"value" yaml:"value"`
	Raw     int64              `json:"raw" yaml:"raw"`
	Known   bool               `json:"known" yaml:"known"`
	Options []string           `json:"options,omitempty" yaml:"options,omitempty"`
}

type settingListResult struct {
	Settings []settingRow `json:"settings" yaml:"settings"`
}

func (r settingListResult) Headers() []string { return []string{"ID", "NAME", "VALUE", "OPTIONS"} }
func (r settingListResult) Rows() [][]string {
	rows := make([][]string, 0, len(r.Settings))
	for _, s := range r.Settings {
		rows = append(rows, []string{strconv.Itoa(int(s.ID)), s.Name, s.Value, strings.Join(s.Options, ", ")})
	}
	return rows
}

func (r settingRow) Headers() []string { return nil }
func (r settingRow) Rows() [][]string {
	return [][]string{{"id", strconv.Itoa(int(r.ID))}, {"name", r.Name}, {"title", r.Title}, {"value", r.Value}, {"raw", strconv.FormatInt(r.Raw, 10)}, {"options", strings.Join(r.Options, ", ")}}
}
func (r settingRow) Plain() string { return r.Value }

func rowsFromValues(v settings.Values, includeUnknown bool) []settingRow {
	var rows []settingRow
	for _, d := range settings.All() {
		opt, ok := v.Get(d.ID)
		if !ok {
			continue
		}
		rows = append(rows, settingRow{ID: d.ID, Name: d.Name, Title: d.Title, Value: opt.String(), Raw: int64(opt.Value()), Known: true, Options: optionNames(d)})
	}
	if includeUnknown {
		for id, raw := range v.Unknown {
			rows = append(rows, settingRow{ID: id, Name: id.String(), Title: "(undocumented for HERO9)", Value: strconv.FormatInt(raw, 10), Raw: raw})
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	return rows
}

// optionNames lists the options the spec documents for the HERO9.
func optionNames(d settings.Descriptor) []string {
	if d.Numeric {
		if d.Max > 0 {
			return []string{fmt.Sprintf("%d-%d", d.Min, d.Max)}
		}
		return []string{"integer"}
	}
	out := make([]string, 0, len(d.Options))
	for _, o := range d.Options {
		if o.HERO9 {
			out = append(out, o.Label)
		}
	}
	return out
}

// warnIfNotHERO9 tells the user when an option is documented only for other cameras.
func (a *App) warnIfNotHERO9(cmd *cobra.Command, d settings.Descriptor, opt settings.Option) {
	if a.opts.Quiet || opt.SupportedOnHERO9() {
		return
	}
	_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s=%s is not listed for the HERO9 in the Open GoPro spec; the camera may reject it\n", d.Name, opt)
}

func (a *App) newSettingCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setting",
		Short: "Read and change camera settings with typed validation",
		Long: `Settings are addressed by name (video-resolution, frames-per-second, ...) or
numeric id. Values accept the human label ("4K", "2.7K 4:3", "Linear") or the
numeric option. Only options the HERO9 supports are accepted; see
"gopro9 setting describe <name>".`,
	}

	var all bool
	list := &cobra.Command{
		Use:   "list",
		Short: "Show current values of all documented settings",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cam, err := a.connect(cmd.Context())
			if err != nil {
				return err
			}
			st, err := cam.OGP.State(cmd.Context())
			if err != nil {
				return wrapCameraErr(err)
			}
			return a.print(cmd, settingListResult{Settings: rowsFromValues(st.Settings, all)})
		},
	}
	list.Flags().BoolVar(&all, "all", false, "include settings the spec does not document for HERO9")

	get := &cobra.Command{
		Use:   "get <name|id>",
		Short: "Show one setting's current value",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			d, ok := settings.Lookup(args[0])
			if !ok {
				return unknownSetting(args[0])
			}
			cam, err := a.connect(cmd.Context())
			if err != nil {
				return err
			}
			opt, err := cam.OGP.GetSetting(cmd.Context(), d.ID)
			if err != nil {
				return wrapCameraErr(err)
			}
			return a.print(cmd, settingRow{ID: d.ID, Name: d.Name, Title: d.Title, Value: opt.String(), Raw: int64(opt.Value()), Known: true, Options: optionNames(d)})
		},
	}

	set := &cobra.Command{
		Use:   "set <name|id> <value>",
		Short: "Change one setting (value may be a label or a number)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			d, ok := settings.Lookup(args[0])
			if !ok {
				return unknownSetting(args[0])
			}
			opt, err := d.Parse(args[1])
			if err != nil {
				return newUsageError("%v; valid: %s", err, strings.Join(optionNames(d), ", "))
			}
			a.warnIfNotHERO9(cmd, d, opt)
			cam, err := a.connect(cmd.Context())
			if err != nil {
				return err
			}
			if err := cam.OGP.SetSetting(cmd.Context(), opt); err != nil {
				return rejectionError(d, opt, err)
			}
			return a.print(cmd, settingRow{ID: d.ID, Name: d.Name, Title: d.Title, Value: opt.String(), Raw: int64(opt.Value()), Known: true})
		},
	}

	describe := &cobra.Command{
		Use:   "describe <name|id>",
		Short: "Show a setting's id, title and the options the HERO9 accepts",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			d, ok := settings.Lookup(args[0])
			if !ok {
				return unknownSetting(args[0])
			}
			return a.print(cmd, d)
		},
	}

	dump := &cobra.Command{
		Use:   "dump",
		Short: "Write all documented settings as YAML (name: value), suitable for `setting apply`",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cam, err := a.connect(cmd.Context())
			if err != nil {
				return err
			}
			st, err := cam.OGP.State(cmd.Context())
			if err != nil {
				return wrapCameraErr(err)
			}
			node := &yaml.Node{Kind: yaml.MappingNode}
			var skipped []string
			for _, d := range settings.All() {
				opt, ok := st.Settings.Get(d.ID)
				if !ok {
					continue
				}
				if !opt.Valid() {
					// The firmware reports a value the spec does not document; it
					// cannot be expressed portably, so it is recorded as a comment.
					skipped = append(skipped, fmt.Sprintf("%s: %d", d.Name, opt.Value()))
					continue
				}
				node.Content = append(node.Content,
					&yaml.Node{Kind: yaml.ScalarNode, Value: d.Name},
					&yaml.Node{Kind: yaml.ScalarNode, Value: opt.String(), Style: yaml.DoubleQuotedStyle})
			}
			if len(skipped) > 0 {
				node.HeadComment = "undocumented values reported by the camera (not applied): " + strings.Join(skipped, ", ")
			}
			enc := yaml.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent(2)
			if err := enc.Encode(node); err != nil {
				return err
			}
			return enc.Close()
		},
	}

	var strict bool
	apply := &cobra.Command{
		Use:   "apply <file.yaml|->",
		Short: "Apply settings from a YAML map of name: value (as produced by `setting dump`)",
		Long: `Applies every entry in order of setting id. Settings the camera reports as
not applicable in its current mode (for example time-lapse rates while in
video mode) are skipped and listed on stderr; pass --strict to fail instead.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var data []byte
			var err error
			if args[0] == "-" {
				data, err = readAll(cmd)
			} else {
				data, err = os.ReadFile(args[0])
			}
			if err != nil {
				return &cameraError{err: err, code: exitcode.IOErr}
			}
			var m map[string]string
			if err := yaml.Unmarshal(data, &m); err != nil {
				return &cameraError{err: fmt.Errorf("parse %s: %w", args[0], err), code: exitcode.DataErr}
			}
			// Validate everything before touching the camera.
			type pair struct {
				d   settings.Descriptor
				opt settings.Option
			}
			pairs := make([]pair, 0, len(m))
			for name, val := range m {
				d, ok := settings.Lookup(name)
				if !ok {
					return unknownSetting(name)
				}
				opt, err := d.Parse(val)
				if err != nil {
					return newUsageError("%s: %v", name, err)
				}
				pairs = append(pairs, pair{d, opt})
			}
			sort.Slice(pairs, func(i, j int) bool { return pairs[i].d.ID < pairs[j].d.ID })
			for _, p := range pairs {
				a.warnIfNotHERO9(cmd, p.d, p.opt)
			}
			cam, err := a.connect(cmd.Context())
			if err != nil {
				return err
			}
			applied := settingListResult{}
			var skipped []string
			for _, p := range pairs {
				err := cam.OGP.SetSetting(cmd.Context(), p.opt)
				var rej *ogp.SettingRejectedError
				switch {
				case err == nil:
					applied.Settings = append(applied.Settings, settingRow{ID: p.d.ID, Name: p.d.Name, Title: p.d.Title, Value: p.opt.String(), Raw: int64(p.opt.Value()), Known: true})
				case errors.As(err, &rej) && !rej.Applicable() && !strict:
					skipped = append(skipped, p.d.Name)
				default:
					return rejectionError(p.d, p.opt, err)
				}
			}
			if len(skipped) > 0 && !a.opts.Quiet {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "skipped (not applicable in the current mode): %s\n", strings.Join(skipped, ", "))
			}
			return a.print(cmd, applied)
		},
	}
	apply.Flags().BoolVar(&strict, "strict", false, "fail when the camera rejects any setting instead of skipping inapplicable ones")

	cmd.AddCommand(list, get, set, describe, dump, apply)
	return cmd
}

// rejectionError turns a camera refusal into an actionable message.
func rejectionError(d settings.Descriptor, opt settings.Option, err error) error {
	var rej *ogp.SettingRejectedError
	if !errors.As(err, &rej) {
		if errors.Is(err, transport.ErrForbidden) {
			return &cameraError{err: fmt.Errorf("camera rejected %s=%s", d.Name, opt), code: exitcode.TempFail}
		}
		return wrapCameraErr(err)
	}
	if !rej.Applicable() {
		return &cameraError{err: fmt.Errorf("camera rejected %s=%s: setting does not apply in the current mode or preset", d.Name, opt), code: exitcode.TempFail}
	}
	labels := make([]string, 0, len(rej.AvailableOptions))
	for _, v := range rej.AvailableOptions {
		if o, ok := d.FromValue(v); ok {
			labels = append(labels, o.String())
		} else {
			labels = append(labels, strconv.FormatUint(uint64(v), 10))
		}
	}
	return &cameraError{err: fmt.Errorf("camera rejected %s=%s; valid right now: %s", d.Name, opt, strings.Join(labels, ", ")), code: exitcode.TempFail}
}

func unknownSetting(name string) error {
	names := make([]string, 0, len(settings.All()))
	for _, d := range settings.All() {
		names = append(names, d.Name)
	}
	return newUsageError("unknown setting %q; known: %s", name, strings.Join(names, ", "))
}

func readAll(cmd *cobra.Command) ([]byte, error) { return io.ReadAll(cmd.InOrStdin()) }
