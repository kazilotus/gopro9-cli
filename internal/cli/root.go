package cli

import (
	"time"

	"github.com/spf13/cobra"
)

const (
	defaultPort    = 8080
	defaultTimeout = 5 * time.Second
	envPrefix      = "GOPRO9_"
)

func (a *App) newRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "gopro9",
		Short: "Control a GoPro HERO9 Black over USB-C",
		Long: `gopro9 drives a HERO9 Black through the Open GoPro HTTP API exposed over the
USB-C cable (GoPro Connect mode). It covers camera control, settings, presets,
media transfer, telemetry, webcam and preview streams, and GoPro Labs tooling.

Output is human-readable by default; pass --output json for scripting.
Exit codes follow sysexits(3): 0 ok, 64 usage, 65 bad data, 69 camera
unavailable or unsupported, 70 internal, 74 local I/O, 75 camera busy.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true, // we ship our own `completion` command
		},
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			return a.bindEnv(cmd)
		},
	}
	root.SetVersionTemplate("{{.Version}}\n")
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error { return newUsageError("%v", err) })

	pf := root.PersistentFlags()
	pf.StringVar(&a.opts.Host, "host", "", "camera IP or hostname (default: auto-discover on the USB link)")
	pf.IntVar(&a.opts.Port, "port", defaultPort, "camera HTTP port")
	pf.DurationVar(&a.opts.Timeout, "timeout", defaultTimeout, "per-request timeout")
	pf.Var(&a.opts.Output, "output", "output format")
	pf.BoolVarP(&a.opts.Verbose, "verbose", "v", false, "log HTTP requests to stderr")
	pf.BoolVarP(&a.opts.Quiet, "quiet", "q", false, "suppress progress and informational output")
	pf.BoolVarP(&a.opts.Yes, "yes", "y", false, "assume yes for confirmations")

	root.AddCommand(
		a.newVersionCommand(),
		a.newCompletionCommand(),
		a.newDocsCommand(root),
		a.newDiscoverCommand(),
		a.newDoctorCommand(),
		a.newCapabilitiesCommand(),
		a.newInfoCommand(),
		a.newStatusCommand(),
		a.newStateCommand(),
		a.newKeepaliveCommand(),
		a.newDatetimeCommand(),
		a.newShutterCommand(),
		a.newCaptureCommand(),
		a.newPresetCommand(),
		a.newSettingCommand(),
		a.newZoomCommand(),
		a.newPowerCommand(),
		a.newTurboCommand(),
	)
	return root
}
