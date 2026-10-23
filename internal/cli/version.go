package cli

import (
	"strconv"

	"github.com/spf13/cobra"

	"github.com/kazilotus/gopro9-cli/internal/buildinfo"
)

type versionResult struct {
	buildinfo.Info `yaml:",inline"`
}

func (v versionResult) Plain() string { return v.Version }

func (v versionResult) Headers() []string { return nil }

func (v versionResult) Rows() [][]string {
	return [][]string{
		{"version", v.Version},
		{"commit", v.Commit},
		{"built", v.Date},
		{"go", v.GoVersion},
		{"modified", strconv.FormatBool(v.Modified)},
	}
}

func (a *App) newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print gopro9 build information",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.print(cmd, versionResult{Info: buildinfo.Get()})
		},
	}
}
