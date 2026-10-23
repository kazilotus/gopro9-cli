package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/kazilotus/gopro9-cli/internal/cli/render"
	"github.com/kazilotus/gopro9-cli/internal/exitcode"
	"github.com/kazilotus/gopro9-cli/pkg/gopro"
	"github.com/kazilotus/gopro9-cli/pkg/gopro/stream"
	"github.com/kazilotus/gopro9-cli/pkg/gopro/webcam"
)

// --- webcam ---------------------------------------------------------------

type webcamStateResult struct {
	webcam.State `yaml:",inline"`
}

func (r webcamStateResult) Headers() []string { return nil }
func (r webcamStateResult) Rows() [][]string {
	return render.KV{{"status", r.Status.String()}, {"error", r.Error.String()}}.Rows()
}
func (r webcamStateResult) Plain() string { return r.Status.String() }

type webcamVersionResult struct {
	webcam.Version `yaml:",inline"`
}

func (r webcamVersionResult) Headers() []string { return nil }
func (r webcamVersionResult) Rows() [][]string {
	return render.KV{
		{"version", strconv.Itoa(r.Version.Version)},
		{"usb 3.1 compatible", strconv.FormatBool(r.USB31Compatible)},
		{"max lens support", strconv.FormatBool(r.MaxLensSupport)},
	}.Rows()
}
func (r webcamVersionResult) Plain() string { return strconv.Itoa(r.Version.Version) }

func (a *App) newWebcamCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "webcam",
		Short: "Use the camera as a USB webcam (MPEG-TS over UDP)",
		Long: `Webcam mode streams MPEG-TS to this host on UDP port 8554. Start it, then open
the stream with "gopro9 stream play" or point ffplay/OBS at udp://@:8554.
"stop" pauses streaming but stays in webcam mode; "exit" returns the camera
to normal operation.`,
	}
	var (
		res  webcam.Resolution
		fov  webcam.FOV
		port int
	)
	start := &cobra.Command{
		Use:   "start",
		Short: "Start streaming at full power",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cam, err := a.connect(cmd.Context())
			if err != nil {
				return err
			}
			opts := webcam.StartOptions{Resolution: res, FOV: fov, HasFOV: cmd.Flags().Changed("fov"), Port: port}
			return a.webcamAction(cmd, cam, "webcam start", func(ctx context.Context) error { return cam.Webcam.Start(ctx, opts) })
		},
	}
	start.Flags().Var(&res, "res", "output resolution (default: camera's choice)")
	start.Flags().Var(&fov, "fov", "digital lens")
	start.Flags().IntVar(&port, "udp-port", 0, "UDP port on this host the camera should stream to (default 8554)")
	preview := &cobra.Command{
		Use:   "preview",
		Short: "Start the low-power preview stream",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cam, err := a.connect(cmd.Context())
			if err != nil {
				return err
			}
			return a.webcamAction(cmd, cam, "webcam preview", cam.Webcam.Preview)
		},
	}
	stop := &cobra.Command{
		Use:   "stop",
		Short: "Stop streaming (stay in webcam mode)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cam, err := a.connect(cmd.Context())
			if err != nil {
				return err
			}
			return a.webcamAction(cmd, cam, "webcam stop", cam.Webcam.Stop)
		},
	}
	exit := &cobra.Command{
		Use:   "exit",
		Short: "Leave webcam mode",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cam, err := a.connect(cmd.Context())
			if err != nil {
				return err
			}
			return a.webcamAction(cmd, cam, "webcam exit", cam.Webcam.Exit)
		},
	}
	status := &cobra.Command{
		Use:   "status",
		Short: "Show webcam state and last error",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cam, err := a.connect(cmd.Context())
			if err != nil {
				return err
			}
			st, err := cam.Webcam.Status(cmd.Context())
			if err != nil {
				return wrapCameraErr(err)
			}
			return a.print(cmd, webcamStateResult{State: st})
		},
	}
	version := &cobra.Command{
		Use:   "version",
		Short: "Show the webcam feature version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cam, err := a.connect(cmd.Context())
			if err != nil {
				return err
			}
			v, err := cam.Webcam.Version(cmd.Context())
			if err != nil {
				return wrapCameraErr(err)
			}
			return a.print(cmd, webcamVersionResult{Version: v})
		},
	}
	cmd.AddCommand(start, preview, stop, exit, status, version)
	return cmd
}

// webcamAction runs a webcam command and reports the resulting state, turning
// a camera-side webcam error into a non-zero exit.
func (a *App) webcamAction(cmd *cobra.Command, cam *gopro.Camera, action string, fn func(context.Context) error) error {
	if err := fn(cmd.Context()); err != nil {
		return wrapCameraErr(err)
	}
	st, err := cam.Webcam.Status(cmd.Context())
	if err != nil {
		return wrapCameraErr(err)
	}
	if st.Error != webcam.ErrNone {
		return &cameraError{err: fmt.Errorf("%s: camera reported webcam error %q", action, st.Error), code: exitcode.TempFail}
	}
	return a.print(cmd, webcamStateResult{State: st})
}

// --- stream ---------------------------------------------------------------

type streamStats struct {
	stream.Stats `yaml:",inline"`
	Dest         string  `json:"dest,omitempty" yaml:"dest,omitempty"`
	Mbps         float64 `json:"mbps" yaml:"mbps"`
}

func (r streamStats) Headers() []string { return nil }
func (r streamStats) Rows() [][]string {
	kv := render.KV{
		{"packets", strconv.FormatInt(r.Packets, 10)},
		{"bytes", humanBytes(r.Bytes)},
		{"duration", r.Duration.Round(time.Millisecond).String()},
		{"bitrate", fmt.Sprintf("%.2f Mbit/s", r.Mbps)},
	}
	if r.Dest != "" {
		kv = append(kv, [2]string{"dest", r.Dest})
	}
	return kv.Rows()
}
func (r streamStats) Plain() string { return r.Dest }

func (a *App) newStreamCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stream",
		Short: "Receive the preview or webcam stream: play it, record it, or just start/stop it",
		Long: `The camera pushes MPEG-TS over UDP to this host. "start"/"stop" only toggle the
low-resolution preview stream on the camera. "play" and "record" additionally
open the UDP port here, start the stream, and stop it again when done.
Use --webcam to drive the full-resolution webcam stream instead of the preview.`,
	}
	var port int
	cmd.PersistentFlags().IntVar(&port, "udp-port", stream.DefaultPort, "UDP port on this host that receives the stream")

	start := &cobra.Command{
		Use:   "start",
		Short: "Ask the camera to start the preview stream",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cam, err := a.connect(cmd.Context())
			if err != nil {
				return err
			}
			if err := cam.Stream.Start(cmd.Context(), portParam(port)); err != nil {
				return wrapCameraErr(err)
			}
			return a.print(cmd, okResult{Action: "stream start", Detail: stream.URL(port)})
		},
	}
	stop := &cobra.Command{
		Use:   "stop",
		Short: "Ask the camera to stop the preview stream",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cam, err := a.connect(cmd.Context())
			if err != nil {
				return err
			}
			if err := cam.Stream.Stop(cmd.Context()); err != nil {
				return wrapCameraErr(err)
			}
			return a.print(cmd, okResult{Action: "stream stop"})
		},
	}

	var (
		useWebcam bool
		res       webcam.Resolution
		player    string
		duration  time.Duration
		out       string
		idle      time.Duration
	)
	play := &cobra.Command{
		Use:   "play",
		Short: "Start the stream and open it in ffplay, mpv or vlc",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := stream.FindPlayer(player)
			if err != nil {
				return &cameraError{err: err, code: exitcode.Unavailable}
			}
			cam, err := a.connect(cmd.Context())
			if err != nil {
				return err
			}
			stopFn, err := a.startLive(cmd.Context(), cam, useWebcam, res, port)
			if err != nil {
				return err
			}
			defer stopFn()
			proc := p.Command(cmd.Context(), stream.URL(port))
			proc.Stdout, proc.Stderr = cmd.ErrOrStderr(), cmd.ErrOrStderr()
			if !a.opts.Quiet {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "opening %s with %s (close the player to stop)\n", stream.URL(port), p.Name)
			}
			if err := proc.Run(); err != nil && cmd.Context().Err() == nil {
				return &cameraError{err: fmt.Errorf("%s: %w", p.Name, err), code: exitcode.Software}
			}
			return nil
		},
	}
	play.Flags().StringVar(&player, "player", "", "player binary: ffplay, mpv or vlc (default: first found)")
	play.Flags().BoolVar(&useWebcam, "webcam", false, "use the full-resolution webcam stream instead of the preview")
	play.Flags().Var(&res, "res", "webcam resolution when --webcam is set")

	record := &cobra.Command{
		Use:   "record",
		Short: "Start the stream and save the raw MPEG-TS to a file",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if out == "" {
				return newUsageError("--out is required")
			}
			conn, err := stream.Listen(port)
			if err != nil {
				return &cameraError{err: err, code: exitcode.Unavailable}
			}
			defer func() { _ = conn.Close() }()
			f, err := os.Create(out)
			if err != nil {
				return &cameraError{err: err, code: exitcode.IOErr}
			}
			defer func() { _ = f.Close() }()
			cam, err := a.connect(cmd.Context())
			if err != nil {
				return err
			}
			stopFn, err := a.startLive(cmd.Context(), cam, useWebcam, res, port)
			if err != nil {
				return err
			}
			defer stopFn()
			stats, err := stream.Receive(cmd.Context(), conn, f, stream.ReceiveOptions{Port: port, IdleTimeout: idle, MaxDuration: duration})
			if err != nil && !errors.Is(err, context.Canceled) {
				if errors.Is(err, stream.ErrNoData) {
					return &cameraError{err: err, code: exitcode.TempFail}
				}
				return &cameraError{err: err, code: exitcode.IOErr}
			}
			return a.print(cmd, streamStats{Stats: stats, Dest: out, Mbps: stats.Bitrate() / 1e6})
		},
	}
	record.Flags().StringVar(&out, "out", "", "output .ts file")
	record.Flags().DurationVar(&duration, "duration", 0, "stop after this long (default: until Ctrl-C)")
	record.Flags().DurationVar(&idle, "idle-timeout", 5*time.Second, "give up when no packets arrive for this long")
	record.Flags().BoolVar(&useWebcam, "webcam", false, "use the full-resolution webcam stream instead of the preview")
	record.Flags().Var(&res, "res", "webcam resolution when --webcam is set")

	cmd.AddCommand(start, stop, play, record)
	return cmd
}

func portParam(port int) int {
	if port == stream.DefaultPort {
		return 0 // let the camera use its default; HERO9 may not accept the parameter
	}
	return port
}

// startLive starts either the preview stream or the webcam and returns a stop
// function that always runs with a short grace context.
func (a *App) startLive(ctx context.Context, cam *gopro.Camera, useWebcam bool, res webcam.Resolution, port int) (func(), error) {
	var startErr error
	if useWebcam {
		startErr = cam.Webcam.Start(ctx, webcam.StartOptions{Resolution: res, Port: portParam(port)})
	} else {
		startErr = cam.Stream.Start(ctx, portParam(port))
	}
	if startErr != nil {
		return nil, wrapCameraErr(startErr)
	}
	return func() {
		grace := 3 * time.Second
		if useWebcam {
			grace = webcam.ModeSwitchTimeout
		}
		sctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), grace)
		defer cancel()
		if useWebcam {
			_ = cam.Webcam.Stop(sctx)
			return
		}
		_ = cam.Stream.Stop(sctx)
	}, nil
}
