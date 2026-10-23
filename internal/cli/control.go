package cli

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/kazilotus/gopro9-cli/internal/cli/render"
	"github.com/kazilotus/gopro9-cli/internal/exitcode"
	"github.com/kazilotus/gopro9-cli/pkg/gopro"
	"github.com/kazilotus/gopro9-cli/pkg/gopro/capability"
	"github.com/kazilotus/gopro9-cli/pkg/gopro/ogp"
	"github.com/kazilotus/gopro9-cli/pkg/gopro/presets"
)

// okResult is the generic acknowledgement for fire-and-forget commands.
type okResult struct {
	Action string `json:"action" yaml:"action"`
	Detail string `json:"detail,omitempty" yaml:"detail,omitempty"`
}

func (r okResult) Headers() []string { return nil }
func (r okResult) Rows() [][]string {
	if r.Detail == "" {
		return [][]string{{r.Action, "ok"}}
	}
	return [][]string{{r.Action, r.Detail}}
}
func (r okResult) Plain() string { return "ok" }

// --- shutter / capture ----------------------------------------------------

func (a *App) newShutterCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "shutter start|stop|toggle",
		Short: "Press the shutter in the current mode",
		Long: `start begins capture (video records until stop; photo and time-lapse modes
stop on their own). toggle reads the encoding status and flips it.`,
		Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		ValidArgs: []string{"start", "stop", "toggle"},
		RunE: func(cmd *cobra.Command, args []string) error {
			cam, err := a.connect(cmd.Context())
			if err != nil {
				return err
			}
			action := ogp.ShutterAction(args[0])
			if args[0] == "toggle" {
				st, err := cam.OGP.State(cmd.Context())
				if err != nil {
					return wrapCameraErr(err)
				}
				action = ogp.ShutterStart
				if st.Status.Encoding {
					action = ogp.ShutterStop
				}
			}
			if err := cam.OGP.Shutter(cmd.Context(), action); err != nil {
				return wrapCameraErr(err)
			}
			return a.print(cmd, okResult{Action: "shutter", Detail: string(action)})
		},
	}
	return cmd
}

type captureResult struct {
	Mode     string        `json:"mode" yaml:"mode"`
	Duration time.Duration `json:"duration,omitempty" yaml:"duration,omitempty"`
	Started  time.Time     `json:"started" yaml:"started"`
}

func (r captureResult) Headers() []string { return nil }
func (r captureResult) Rows() [][]string {
	kv := render.KV{{"mode", r.Mode}, {"started", r.Started.Format(time.RFC3339)}}
	if r.Duration > 0 {
		kv = append(kv, [2]string{"duration", r.Duration.String()})
	}
	return kv.Rows()
}
func (r captureResult) Plain() string { return r.Mode }

func (a *App) newCaptureCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "capture",
		Short: "Switch mode and capture in one step",
	}
	var duration time.Duration
	video := &cobra.Command{
		Use:   "video",
		Short: "Record a video clip of a fixed duration",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if duration <= 0 {
				return newUsageError("--duration must be positive")
			}
			return a.timedCapture(cmd, presets.GroupVideo, duration)
		},
	}
	video.Flags().DurationVar(&duration, "duration", 10*time.Second, "how long to record")
	photo := &cobra.Command{
		Use:   "photo",
		Short: "Take a photo",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cam, err := a.connect(cmd.Context())
			if err != nil {
				return err
			}
			if err := a.switchGroup(cmd.Context(), cam, presets.GroupPhoto); err != nil {
				return err
			}
			if err := cam.OGP.Shutter(cmd.Context(), ogp.ShutterStart); err != nil {
				return wrapCameraErr(err)
			}
			return a.print(cmd, captureResult{Mode: "photo", Started: a.now()})
		},
	}
	var tlDuration time.Duration
	timelapse := &cobra.Command{
		Use:   "timelapse",
		Short: "Run the time-lapse preset for a fixed duration",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if tlDuration <= 0 {
				return newUsageError("--duration must be positive")
			}
			return a.timedCapture(cmd, presets.GroupTimelapse, tlDuration)
		},
	}
	timelapse.Flags().DurationVar(&tlDuration, "duration", time.Minute, "how long to run the time-lapse")
	cmd.AddCommand(video, photo, timelapse)
	return cmd
}

func (a *App) switchGroup(ctx context.Context, cam *gopro.Camera, g presets.Group) error {
	st, err := cam.OGP.State(ctx)
	if err != nil {
		return wrapCameraErr(err)
	}
	if st.Status.Encoding {
		return &cameraError{err: errors.New("camera is already recording; stop it first"), code: exitcode.TempFail}
	}
	gid, _ := g.ID()
	if int64(gid) == st.Status.PresetGroup {
		return nil
	}
	return wrapCameraErr(cam.OGP.SetPresetGroup(ctx, g))
}

func (a *App) timedCapture(cmd *cobra.Command, g presets.Group, d time.Duration) error {
	ctx := cmd.Context()
	cam, err := a.connect(ctx)
	if err != nil {
		return err
	}
	if err := a.switchGroup(ctx, cam, g); err != nil {
		return err
	}
	if err := cam.OGP.Shutter(ctx, ogp.ShutterStart); err != nil {
		return wrapCameraErr(err)
	}
	started := a.now()
	// Always try to stop, even when interrupted, with a short grace context.
	stop := func() error {
		sctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		defer cancel()
		return wrapCameraErr(cam.OGP.Shutter(sctx, ogp.ShutterStop))
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	keep := time.NewTicker(3 * time.Second)
	defer keep.Stop()
	for {
		select {
		case <-ctx.Done():
			return stop()
		case <-keep.C:
			_ = cam.OGP.KeepAlive(ctx)
		case <-timer.C:
			if err := stop(); err != nil {
				return err
			}
			return a.print(cmd, captureResult{Mode: g.String(), Duration: d, Started: started})
		}
	}
}

// --- preset ---------------------------------------------------------------

type presetListResult struct {
	presets.Status `yaml:",inline"`
}

func (r presetListResult) Headers() []string {
	return []string{"GROUP", "ID", "TITLE", "MODE", "CUSTOM"}
}

func (r presetListResult) Rows() [][]string {
	rows := make([][]string, 0, 8)
	for _, g := range r.Groups {
		for _, p := range g.Presets {
			rows = append(rows, []string{g.ID.String(), strconv.FormatUint(uint64(p.ID), 10), p.Title(), p.Mode, strconv.FormatBool(p.UserDefined)})
		}
	}
	return rows
}

func (a *App) newPresetCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "preset", Short: "List and activate presets and preset groups"}
	list := &cobra.Command{
		Use:   "list",
		Short: "List all presets grouped by mode",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cam, err := a.connect(cmd.Context())
			if err != nil {
				return err
			}
			st, err := cam.OGP.Presets(cmd.Context())
			if err != nil {
				return wrapCameraErr(err)
			}
			return a.print(cmd, presetListResult{Status: st})
		},
	}
	load := &cobra.Command{
		Use:   "load <id|title>",
		Short: "Activate a preset by numeric id or title (e.g. \"Max Video\")",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cam, err := a.connect(cmd.Context())
			if err != nil {
				return err
			}
			st, err := cam.OGP.Presets(cmd.Context())
			if err != nil {
				return wrapCameraErr(err)
			}
			p, g, ok := st.Find(args[0])
			if !ok {
				return newUsageError("no preset matches %q (see `gopro9 preset list`)", args[0])
			}
			if err := cam.OGP.LoadPreset(cmd.Context(), p.ID); err != nil {
				return wrapCameraErr(err)
			}
			return a.print(cmd, okResult{Action: "preset", Detail: fmt.Sprintf("%s (%d, %s)", p.Title(), p.ID, g)})
		},
	}
	var group presets.Group
	groupCmd := &cobra.Command{
		Use:       "group video|photo|timelapse",
		Short:     "Switch the active preset group (camera mode)",
		Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		ValidArgs: []string{"video", "photo", "timelapse"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := group.Set(args[0]); err != nil {
				return newUsageError("%v", err)
			}
			cam, err := a.connect(cmd.Context())
			if err != nil {
				return err
			}
			if err := cam.OGP.SetPresetGroup(cmd.Context(), group); err != nil {
				return wrapCameraErr(err)
			}
			return a.print(cmd, okResult{Action: "preset group", Detail: group.String()})
		},
	}
	cmd.AddCommand(list, load, groupCmd)
	return cmd
}

// --- zoom / power / turbo -------------------------------------------------

func (a *App) newZoomCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "zoom <0-100>",
		Short: "Set digital zoom as a percentage",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			n, err := strconv.Atoi(args[0])
			if err != nil || n < 0 || n > 100 {
				return newUsageError("zoom must be an integer 0-100, got %q", args[0])
			}
			cam, err := a.connect(cmd.Context())
			if err != nil {
				return err
			}
			st, err := cam.OGP.State(cmd.Context())
			if err != nil {
				return wrapCameraErr(err)
			}
			if !st.Status.ZoomAvailable {
				return &cameraError{err: errors.New("digital zoom is not available in the current mode (disabled with Max Lens Mod and in some presets)"), code: exitcode.TempFail}
			}
			if err := cam.OGP.DigitalZoom(cmd.Context(), uint8(n)); err != nil {
				return wrapCameraErr(err)
			}
			return a.print(cmd, okResult{Action: "zoom", Detail: strconv.Itoa(n) + "%"})
		},
	}
}

func (a *App) newPowerCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "power", Short: "Sleep the camera or trigger the locate beep"}
	sleep := &cobra.Command{
		Use:   "sleep",
		Short: "Put the camera to sleep (wake it with the mode button or power cycle)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := a.requireCapability(cmd, capability.OpGPCAMERAPowerId); err != nil {
				return err
			}
			cam, err := a.connect(cmd.Context())
			if err != nil {
				return err
			}
			if err := cam.Legacy.Sleep(cmd.Context()); err != nil {
				return wrapCameraErr(err)
			}
			return a.print(cmd, okResult{Action: "sleep"})
		},
	}
	addForceFlag(sleep)
	locate := &cobra.Command{
		Use:       "locate on|off",
		Short:     "Start or stop the locate beep",
		Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		ValidArgs: []string{"on", "off"},
		RunE: func(cmd *cobra.Command, args []string) error {
			cam, err := a.connect(cmd.Context())
			if err != nil {
				return err
			}
			if err := cam.Legacy.Locate(cmd.Context(), args[0] == "on"); err != nil {
				return wrapCameraErr(err)
			}
			return a.print(cmd, okResult{Action: "locate", Detail: args[0]})
		},
	}
	cmd.AddCommand(sleep, locate)
	return cmd
}

func (a *App) newTurboCommand() *cobra.Command {
	return &cobra.Command{
		Use:       "turbo on|off",
		Short:     "Toggle turbo transfer (prioritises USB bandwidth for downloads)",
		Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		ValidArgs: []string{"on", "off"},
		RunE: func(cmd *cobra.Command, args []string) error {
			cam, err := a.connect(cmd.Context())
			if err != nil {
				return err
			}
			if err := cam.OGP.TurboTransfer(cmd.Context(), args[0] == "on"); err != nil {
				return wrapCameraErr(err)
			}
			return a.print(cmd, okResult{Action: "turbo", Detail: args[0]})
		},
	}
}
