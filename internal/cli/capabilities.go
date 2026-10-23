package cli

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/kazilotus/gopro9-cli/pkg/gopro/capability"
)

type capabilitiesResult struct {
	SpecVersion    string            `json:"spec_version" yaml:"spec_version"`
	FirmwareTested []string          `json:"firmware_tested" yaml:"firmware_tested"`
	Operations     []capability.Info `json:"operations" yaml:"operations"`
}

func (r capabilitiesResult) Headers() []string {
	return []string{"OPERATION", "LEVEL", "METHOD", "PATH", "NOTE"}
}

func (r capabilitiesResult) Rows() [][]string {
	rows := make([][]string, 0, len(r.Operations))
	for _, o := range r.Operations {
		rows = append(rows, []string{string(o.Op), o.Level.String(), o.Method, o.Path, o.Note})
	}
	return rows
}

func (a *App) newCapabilitiesCommand() *cobra.Command {
	var (
		level string
		op    string
	)
	cmd := &cobra.Command{
		Use:   "capabilities",
		Short: "Show what the HERO9 supports over USB (verified / spec / unsupported)",
		Long: `The capability matrix is generated from the Open GoPro spec and corrected by
real-hardware probing. "verified" operations were exercised on a HERO9 over USB;
"spec" ones are listed for HERO9 upstream but not yet exercised; "unsupported"
ones are absent for HERO9 or known to fail on this firmware.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			res := capabilitiesResult{SpecVersion: capability.SpecVersion, FirmwareTested: capability.FirmwareTested}
			for _, info := range capability.All() {
				if op != "" && !strings.EqualFold(string(info.Op), op) {
					continue
				}
				if level != "" && !strings.EqualFold(info.Level.String(), level) {
					continue
				}
				res.Operations = append(res.Operations, info)
			}
			if op != "" && len(res.Operations) == 0 {
				return newUsageError("unknown operation %q", op)
			}
			return a.print(cmd, res)
		},
	}
	cmd.Flags().StringVar(&level, "level", "", "filter by level: verified, spec or unsupported")
	cmd.Flags().StringVar(&op, "op", "", "show a single operation by ID")
	return cmd
}
