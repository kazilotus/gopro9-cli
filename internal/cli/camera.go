package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/kazilotus/gopro9-cli/internal/cli/render"
	"github.com/kazilotus/gopro9-cli/pkg/gopro/capability"
	"github.com/kazilotus/gopro9-cli/pkg/gopro/gpcontrol"
	"github.com/kazilotus/gopro9-cli/pkg/gopro/ogp"
	"github.com/kazilotus/gopro9-cli/pkg/gopro/presets"
)

// --- info -----------------------------------------------------------------

type infoResult struct {
	gpcontrol.Info `yaml:",inline"`
	APIVersion     string `json:"api_version" yaml:"api_version"`
	Labs           bool   `json:"labs" yaml:"labs"`
}

func (r infoResult) Headers() []string { return nil }
func (r infoResult) Rows() [][]string {
	return render.KV{
		{"model", r.ModelName},
		{"firmware", r.FirmwareVersion},
		{"labs", strconv.FormatBool(r.Labs)},
		{"serial", r.SerialNumber},
		{"board", r.BoardType},
		{"ap ssid", r.APSSID},
		{"ap mac", r.APMAC},
		{"open gopro api", r.APIVersion},
		{"update required", r.UpdateRequired},
	}.Rows()
}
func (r infoResult) Plain() string { return r.FirmwareVersion }

func (a *App) newInfoCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Show camera identity: model, firmware, serial, Open GoPro API version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cam, err := a.connect(cmd.Context())
			if err != nil {
				return err
			}
			info, err := cam.Legacy.Info(cmd.Context())
			if err != nil {
				return wrapCameraErr(err)
			}
			ver, err := cam.OGP.Version(cmd.Context())
			if err != nil {
				return wrapCameraErr(err)
			}
			return a.print(cmd, infoResult{Info: info, APIVersion: ver.Version, Labs: info.IsLabs()})
		},
	}
}

// --- status ---------------------------------------------------------------

type statusResult struct {
	Battery       int64   `json:"battery_percent" yaml:"battery_percent"`
	Charging      bool    `json:"charging" yaml:"charging"`
	Recording     bool    `json:"recording" yaml:"recording"`
	Busy          bool    `json:"busy" yaml:"busy"`
	Overheating   bool    `json:"overheating" yaml:"overheating"`
	SDFreeGB      float64 `json:"sd_free_gb" yaml:"sd_free_gb"`
	SDStatus      string  `json:"sd_status" yaml:"sd_status"`
	Group         string  `json:"preset_group" yaml:"preset_group"`
	Preset        uint32  `json:"preset" yaml:"preset"`
	Resolution    string  `json:"video_resolution" yaml:"video_resolution"`
	FPS           string  `json:"fps" yaml:"fps"`
	Lens          string  `json:"video_lens" yaml:"video_lens"`
	Name          string  `json:"name" yaml:"name"`
	VideosLeftMin int64   `json:"video_remaining_min" yaml:"video_remaining_min"`
	PhotosLeft    int64   `json:"photos_remaining" yaml:"photos_remaining"`
}

func (r statusResult) Headers() []string { return nil }
func (r statusResult) Rows() [][]string {
	rec := "idle"
	if r.Recording {
		rec = "RECORDING"
	}
	charge := ""
	if r.Charging {
		charge = " (charging)"
	}
	return render.KV{
		{"camera", r.Name},
		{"battery", fmt.Sprintf("%d%%%s", r.Battery, charge)},
		{"state", rec},
		{"busy", strconv.FormatBool(r.Busy)},
		{"overheating", strconv.FormatBool(r.Overheating)},
		{"sd card", fmt.Sprintf("%.1f GB free (%s)", r.SDFreeGB, r.SDStatus)},
		{"mode", r.Group},
		{"video", fmt.Sprintf("%s @ %s fps, %s lens", r.Resolution, r.FPS, r.Lens)},
		{"remaining", fmt.Sprintf("%d min video, %d photos", r.VideosLeftMin, r.PhotosLeft)},
	}.Rows()
}

func (r statusResult) Plain() string {
	if r.Recording {
		return "recording"
	}
	return "idle"
}

func summarize(st ogp.State) statusResult {
	s := st.Status
	return statusResult{
		Battery:       s.InternalBatteryPercentage,
		Charging:      s.InternalBatteryBars == 4,
		Recording:     s.Encoding,
		Busy:          s.Busy,
		Overheating:   s.Overheating,
		SDFreeGB:      float64(s.SDCardRemaining) / 1024 / 1024,
		SDStatus:      s.PrimaryStorage.String(),
		Group:         groupName(s.PresetGroup),
		Preset:        uint32(s.Preset),
		Resolution:    st.Settings.VideoResolution.String(),
		FPS:           st.Settings.FramesPerSecond.String(),
		Lens:          st.Settings.VideoLens.String(),
		Name:          s.AccessPointSSID,
		VideosLeftMin: s.RemainingVideoTime / 60,
		PhotosLeft:    s.RemainingPhotos,
	}
}

func groupName(id int64) string {
	for _, g := range presets.Groups() {
		if gid, ok := g.ID(); ok && int64(gid) == id {
			return g.String()
		}
	}
	return strconv.FormatInt(id, 10)
}

func (a *App) newStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "One-screen camera summary: battery, SD card, mode, recording state",
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
			return a.print(cmd, summarize(st))
		},
	}
}

// --- state ----------------------------------------------------------------

type stateResult struct{ ogp.State }

func (r stateResult) Headers() []string { return []string{"KEY", "VALUE"} }
func (r stateResult) Rows() [][]string {
	rows := [][]string{}
	for _, d := range allSettingDescriptors() {
		if opt, ok := r.Settings.Get(d.ID); ok {
			rows = append(rows, []string{"setting." + d.Name, opt.String()})
		}
	}
	for id, v := range r.Settings.Unknown {
		rows = append(rows, []string{"setting." + id.String(), strconv.FormatInt(v, 10)})
	}
	rows = append(rows, statusRows(r.State)...)
	sortRows(rows)
	return rows
}

func (a *App) newStateCommand() *cobra.Command {
	var (
		watch time.Duration
		raw   bool
	)
	cmd := &cobra.Command{
		Use:   "state",
		Short: "Dump the full typed camera state (every status and setting)",
		Long: `Print every status and setting the camera reports, decoded into typed
values where the Open GoPro spec documents them. --watch repeats the query
and prints only the keys whose values changed.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cam, err := a.connect(cmd.Context())
			if err != nil {
				return err
			}
			if raw {
				resp, err := cam.Transport.Get(cmd.Context(), rawStateRequest())
				if err != nil {
					return wrapCameraErr(err)
				}
				_, err = cmd.OutOrStdout().Write(append(resp.Body, '\n'))
				return err
			}
			if watch <= 0 {
				st, err := cam.OGP.State(cmd.Context())
				if err != nil {
					return wrapCameraErr(err)
				}
				return a.print(cmd, stateResult{State: st})
			}
			return a.watchState(cmd.Context(), cam.OGP, watch, func(rows [][]string) error {
				return render.Table(cmd.OutOrStdout(), render.KV(toKV(rows)))
			})
		},
	}
	cmd.Flags().DurationVar(&watch, "watch", 0, "re-query at this interval and print changed keys (0 = once)")
	cmd.Flags().BoolVar(&raw, "raw", false, "print the camera's raw JSON instead of the typed view")
	return cmd
}

func (a *App) watchState(ctx context.Context, c *ogp.Client, every time.Duration, emit func([][]string) error) error {
	prev := map[string]string{}
	first := true
	tick := time.NewTicker(every)
	defer tick.Stop()
	for {
		st, err := c.State(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return wrapCameraErr(err)
		}
		var changed [][]string
		for _, row := range (stateResult{State: st}).Rows() {
			if first || prev[row[0]] != row[1] {
				changed = append(changed, []string{a.now().Format("15:04:05"), row[0], row[1]})
			}
			prev[row[0]] = row[1]
		}
		first = false
		if len(changed) > 0 {
			if err := emit(changed); err != nil {
				return err
			}
		}
		select {
		case <-ctx.Done():
			return nil
		case <-tick.C:
		}
	}
}

func toKV(rows [][]string) [][2]string {
	out := make([][2]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, [2]string{strings.Join(r[:len(r)-1], " "), r[len(r)-1]})
	}
	return out
}

// --- keepalive ------------------------------------------------------------

func (a *App) newKeepaliveCommand() *cobra.Command {
	var interval time.Duration
	cmd := &cobra.Command{
		Use:   "keepalive",
		Short: "Send heartbeats so the camera does not sleep (Ctrl-C to stop)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cam, err := a.connect(cmd.Context())
			if err != nil {
				return err
			}
			tick := time.NewTicker(interval)
			defer tick.Stop()
			for {
				if err := cam.OGP.KeepAlive(cmd.Context()); err != nil {
					if cmd.Context().Err() != nil {
						return nil // interrupted by the user
					}
					return wrapCameraErr(err)
				}
				if !a.opts.Quiet {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "%s keep-alive sent\n", a.now().Format("15:04:05"))
				}
				select {
				case <-cmd.Context().Done():
					return nil
				case <-tick.C:
				}
			}
		},
	}
	cmd.Flags().DurationVar(&interval, "interval", 3*time.Second, "time between heartbeats")
	return cmd
}

// --- datetime -------------------------------------------------------------

type datetimeResult struct {
	Camera time.Time     `json:"camera" yaml:"camera"`
	Host   time.Time     `json:"host" yaml:"host"`
	Drift  time.Duration `json:"drift" yaml:"drift"`
}

func (r datetimeResult) Headers() []string { return nil }
func (r datetimeResult) Rows() [][]string {
	return render.KV{
		{"camera", r.Camera.Format(time.RFC3339)},
		{"host", r.Host.Format(time.RFC3339)},
		{"drift", r.Drift.Round(time.Second).String()},
	}.Rows()
}
func (r datetimeResult) Plain() string { return r.Camera.Format(time.RFC3339) }

func (a *App) newDatetimeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "datetime",
		Short: "Read or set the camera clock",
	}
	get := &cobra.Command{
		Use:   "get",
		Short: "Show the camera clock and its drift from this host",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cam, err := a.connect(cmd.Context())
			if err != nil {
				return err
			}
			dt, err := cam.OGP.GetDateTime(cmd.Context())
			if err != nil {
				return wrapCameraErr(err)
			}
			camT, err := dt.Parse(time.Local)
			if err != nil {
				return &cameraError{err: fmt.Errorf("unparseable camera time %q %q: %w", dt.Date, dt.Time, err), code: 65}
			}
			now := a.now()
			return a.print(cmd, datetimeResult{Camera: camT, Host: now, Drift: camT.Sub(now)})
		},
	}
	set := &cobra.Command{
		Use:   "set <RFC3339|YYYY-MM-DDTHH:MM:SS>",
		Short: "Set the camera clock to an explicit local time",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			t, err := parseLocalTime(args[0])
			if err != nil {
				return newUsageError("%v", err)
			}
			return a.setClock(cmd, t)
		},
	}
	sync := &cobra.Command{
		Use:   "sync",
		Short: "Set the camera clock to this host's current local time",
		Args:  cobra.NoArgs,
		RunE:  func(cmd *cobra.Command, _ []string) error { return a.setClock(cmd, a.now()) },
	}
	addForceFlag(set)
	addForceFlag(sync)
	cmd.AddCommand(get, set, sync)
	return cmd
}

func (a *App) setClock(cmd *cobra.Command, t time.Time) error {
	if err := a.requireCapability(cmd, capability.OpOGPSetDateAndTimeDst); err != nil {
		return err
	}
	cam, err := a.connect(cmd.Context())
	if err != nil {
		return err
	}
	if err := cam.OGP.SetDateTime(cmd.Context(), t); err != nil {
		return wrapCameraErr(err)
	}
	return a.print(cmd, datetimeResult{Camera: t, Host: a.now(), Drift: 0})
}

func parseLocalTime(s string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02 15:04:05"} {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse time %q (want RFC3339 or YYYY-MM-DDTHH:MM:SS)", s)
}
