package cli

import (
	"github.com/spf13/cobra"
)

// newMediaTelemetryCommand saves the raw telemetry track. Decoded formats
// (json, gpx, csv) are wired in once the gpmf package lands.
func (a *App) newMediaTelemetryCommand() *cobra.Command {
	var (
		out    string
		format string
	)
	cmd := &cobra.Command{
		Use:   "telemetry <DIR/NAME>",
		Short: "Extract a video's telemetry track (GPS, accelerometer, gyro) without downloading the video",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if format != "raw" {
				return newUsageError("format %q not available yet (raw only)", format)
			}
			cam, err := a.connect(cmd.Context())
			if err != nil {
				return err
			}
			data, err := cam.Media.Telemetry(cmd.Context(), args[0])
			if err != nil {
				return downloadErr(err)
			}
			if out == "" {
				out = defaultOut(args[0], ".telemetry.mp4")
			}
			return a.writeBlob(cmd, args[0], out, data)
		},
	}
	cmd.Flags().StringVar(&out, "out", "", "output file (default <name>.telemetry.mp4; - for stdout)")
	cmd.Flags().StringVar(&format, "format", "raw", "raw | json | gpx | csv")
	return cmd
}
