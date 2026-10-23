package cli

import (
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/kazilotus/gopro9-cli/pkg/gopro/discovery"
)

type discoverResult struct {
	Cameras []discovery.Candidate `json:"cameras" yaml:"cameras"`
}

func (r discoverResult) Headers() []string {
	return []string{"ADDRESS", "PORT", "INTERFACE", "API", "FIRMWARE", "SERIAL", "SOURCE"}
}

func (r discoverResult) Rows() [][]string {
	rows := make([][]string, 0, len(r.Cameras))
	for i := range r.Cameras {
		c := &r.Cameras[i]
		rows = append(rows, []string{c.Address.String(), strconv.Itoa(c.Port), c.Interface, c.APIVersion, c.Firmware, c.Serial, string(c.Source)})
	}
	return rows
}

func (r discoverResult) Plain() string {
	if len(r.Cameras) == 0 {
		return ""
	}
	return r.Cameras[0].Address.String()
}

func (a *App) newDiscoverCommand() *cobra.Command {
	var (
		mdnsWait time.Duration
		noMDNS   bool
	)
	cmd := &cobra.Command{
		Use:   "discover",
		Short: "Find cameras on the USB link",
		Long: `Probe every network interface for a HERO9 (the camera always takes host .51
of its USB subnet) and enrich matches with mDNS metadata (firmware, serial).
With --output plain, prints just the first camera's IP for use in scripts.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			opts := discovery.Options{Port: a.opts.Port, ProbeTimeout: a.opts.Timeout, MDNSTimeout: mdnsWait}
			if noMDNS {
				opts.MDNSTimeout = 0
			}
			found, err := discovery.Discover(cmd.Context(), opts)
			if err != nil {
				return wrapCameraErr(err)
			}
			return a.print(cmd, discoverResult{Cameras: found})
		},
	}
	cmd.Flags().DurationVar(&mdnsWait, "mdns-wait", 1500*time.Millisecond, "how long to wait for mDNS answers")
	cmd.Flags().BoolVar(&noMDNS, "no-mdns", false, "skip mDNS enrichment (faster)")
	return cmd
}
