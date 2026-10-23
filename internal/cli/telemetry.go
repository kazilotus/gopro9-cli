package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/kazilotus/gopro9-cli/internal/exitcode"
	"github.com/kazilotus/gopro9-cli/pkg/gopro/gpmf"
)

type telemetryStreamRow struct {
	Key      string   `json:"key" yaml:"key"`
	Name     string   `json:"name" yaml:"name"`
	Units    []string `json:"units,omitempty" yaml:"units,omitempty"`
	Columns  int      `json:"columns" yaml:"columns"`
	Samples  int      `json:"samples" yaml:"samples"`
	Payloads int      `json:"payloads" yaml:"payloads"`
}

type telemetrySummary struct {
	Source   string               `json:"source" yaml:"source"`
	Device   gpmf.Device          `json:"device" yaml:"device"`
	Payloads int                  `json:"payloads" yaml:"payloads"`
	Duration float64              `json:"duration_s" yaml:"duration_s"`
	GPSFixes int                  `json:"gps_fixes" yaml:"gps_fixes"`
	Streams  []telemetryStreamRow `json:"streams" yaml:"streams"`
}

func (r telemetrySummary) Headers() []string {
	return []string{"STREAM", "NAME", "UNITS", "COLS", "SAMPLES", "PAYLOADS"}
}

func (r telemetrySummary) Rows() [][]string {
	rows := make([][]string, 0, len(r.Streams))
	for _, s := range r.Streams {
		rows = append(rows, []string{s.Key, s.Name, strings.Join(s.Units, ","), strconv.Itoa(s.Columns), strconv.Itoa(s.Samples), strconv.Itoa(s.Payloads)})
	}
	return rows
}

func (r telemetrySummary) Plain() string {
	keys := make([]string, 0, len(r.Streams))
	for _, s := range r.Streams {
		keys = append(keys, s.Key)
	}
	return strings.Join(keys, "\n")
}

// telemetryJSON is the JSON shape for --format json. Samples use pointers so
// that NaN (non-numeric columns) becomes null, which encoding/json cannot do
// for float64 directly.
type telemetryJSON struct {
	Device   gpmf.Device        `json:"device"`
	Payloads int                `json:"payloads"`
	Streams  []telemetryJSONStr `json:"streams"`
}

type telemetryJSONStr struct {
	Key      string       `json:"key"`
	Name     string       `json:"name"`
	Units    []string     `json:"units,omitempty"`
	Scale    []float64    `json:"scale,omitempty"`
	Columns  int          `json:"columns"`
	Payloads []int        `json:"payloads"`
	Samples  [][]*float64 `json:"samples"`
}

func jsonTelemetry(t *gpmf.Telemetry) telemetryJSON {
	out := telemetryJSON{Device: t.Device, Payloads: t.Payloads}
	for _, key := range t.Order {
		s := t.Streams[key]
		js := telemetryJSONStr{Key: key, Name: s.Name, Units: s.Units, Scale: s.Scale, Columns: s.Columns, Payloads: s.Payloads}
		js.Samples = make([][]*float64, len(s.Samples))
		for i, row := range s.Samples {
			js.Samples[i] = make([]*float64, len(row))
			for j, v := range row {
				if v == v { // not NaN
					val := v
					js.Samples[i][j] = &val
				}
			}
		}
		out.Streams = append(out.Streams, js)
	}
	return out
}

func summarizeTelemetry(source string, t *gpmf.Telemetry) telemetrySummary {
	res := telemetrySummary{Source: source, Device: t.Device, Payloads: t.Payloads, GPSFixes: len(t.GPS())}
	for p := 0; p < t.Payloads; p++ {
		res.Duration += t.PayloadDuration(p)
	}
	for _, key := range t.Order {
		s := t.Streams[key]
		res.Streams = append(res.Streams, telemetryStreamRow{Key: key, Name: s.Name, Units: s.Units, Columns: s.Columns, Samples: len(s.Samples), Payloads: len(s.Payloads)})
	}
	sort.SliceStable(res.Streams, func(i, j int) bool { return res.Streams[i].Key < res.Streams[j].Key })
	return res
}

func (a *App) newMediaTelemetryCommand() *cobra.Command {
	var out, format, streamKey, from string
	cmd := &cobra.Command{
		Use:   "telemetry <DIR/NAME>",
		Short: "Extract a video's telemetry (GPS, accelerometer, gyro) without downloading the video",
		Long: `Fetches the camera's telemetry track for a video and decodes the GPMF streams.

Formats:
  summary  one row per stream (default)
  gpx      GPS track as GPX 1.1 (from GPS5 + GPSU)
  csv      one stream as CSV (--stream, default GPS5)
  json     every stream with scaled samples
  raw      the camera's bytes untouched (an MP4 holding the gpmd track)

--from FILE decodes a previously saved raw telemetry file instead of asking
the camera, so no connection is needed.`,
		Example: `  gopro9 media telemetry 100GOPRO/GH010516.MP4 --format gpx --out ride.gpx
  gopro9 media telemetry 100GOPRO/GH010516.MP4 --format csv --stream ACCL --out accl.csv
  gopro9 media telemetry --from GH010516.telemetry.mp4 --format summary`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if from == "" && len(args) == 0 {
				return newUsageError("give a DIR/NAME or --from FILE")
			}
			var (
				data   []byte
				source string
				err    error
			)
			if from != "" {
				source = from
				data, err = os.ReadFile(from)
				if err != nil {
					return &cameraError{err: err, code: exitcode.IOErr}
				}
			} else {
				source = args[0]
				cam, cerr := a.connect(cmd.Context())
				if cerr != nil {
					return cerr
				}
				data, err = cam.Media.Telemetry(cmd.Context(), args[0])
				if err != nil {
					return downloadErr(err)
				}
			}
			if format == "raw" {
				if out == "" {
					out = defaultOut(source, ".telemetry.mp4")
				}
				return a.writeBlob(cmd, source, out, data)
			}
			t, err := gpmf.ParseTelemetry(data)
			if t == nil || t.Payloads == 0 {
				if err == nil {
					err = fmt.Errorf("no GPMF payloads found in %s", source)
				}
				return &cameraError{err: fmt.Errorf("decode telemetry: %w", err), code: exitcode.DataErr}
			}
			if err != nil && !a.opts.Quiet {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning: telemetry partially decoded: %v\n", err)
			}
			var buf bytes.Buffer
			switch format {
			case "summary":
				return a.print(cmd, summarizeTelemetry(source, t))
			case "gpx":
				fixes := t.GPS()
				if len(fixes) == 0 {
					return &cameraError{err: fmt.Errorf("no GPS fixes in %s (GPS off or no lock)", source), code: exitcode.DataErr}
				}
				if err := gpmf.WriteGPX(&buf, fixes, source); err != nil {
					return err
				}
			case "csv":
				s, ok := t.Stream(streamKey)
				if !ok {
					return newUsageError("stream %q not present; available: %s", streamKey, strings.Join(t.Order, ", "))
				}
				if err := gpmf.WriteCSV(&buf, s); err != nil {
					return err
				}
			case "json":
				enc := json.NewEncoder(&buf)
				enc.SetIndent("", "  ")
				if err := enc.Encode(jsonTelemetry(t)); err != nil {
					return err
				}
			default:
				return newUsageError("unknown --format %q (want summary, gpx, csv, json or raw)", format)
			}
			if out == "" || out == "-" {
				_, err := cmd.OutOrStdout().Write(buf.Bytes())
				return err
			}
			return a.writeBlob(cmd, source, out, buf.Bytes())
		},
	}
	cmd.Flags().StringVar(&out, "out", "", "output file (default: stdout for text formats, <name>.telemetry.mp4 for raw)")
	cmd.Flags().StringVar(&format, "format", "summary", "summary | gpx | csv | json | raw")
	cmd.Flags().StringVar(&streamKey, "stream", "GPS5", "stream FourCC for --format csv (GPS5, ACCL, GYRO, SHUT, ...)")
	cmd.Flags().StringVar(&from, "from", "", "decode this saved telemetry file instead of fetching from the camera")
	return cmd
}
