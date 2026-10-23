package cli

import (
	"strconv"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/kazilotus/gopro9-cli/internal/cli/render"
	"github.com/kazilotus/gopro9-cli/internal/config"
	"github.com/kazilotus/gopro9-cli/internal/exitcode"
)

// applyConfigFile fills persistent flags from the config file when neither the
// flag nor its environment variable set them.
func (a *App) applyConfigFile(cmd *cobra.Command) error {
	f, _, err := config.Load()
	if err != nil {
		return &cameraError{err: err, code: exitcode.Config}
	}
	set := func(name, value string) error {
		fl := cmd.Root().PersistentFlags().Lookup(name)
		if fl == nil || fl.Changed || value == "" {
			return nil
		}
		return fl.Value.Set(value)
	}
	if err := set("host", f.Host); err != nil {
		return err
	}
	if f.Port != 0 {
		if err := set("port", strconv.Itoa(f.Port)); err != nil {
			return err
		}
	}
	if f.Timeout != 0 {
		if err := set("timeout", f.Timeout.String()); err != nil {
			return err
		}
	}
	if err := set("output", f.Output); err != nil {
		return err
	}
	if f.Quiet {
		if err := set("quiet", "true"); err != nil {
			return err
		}
	}
	return nil
}

type configResult struct {
	Path string      `json:"path" yaml:"path"`
	File config.File `json:"config" yaml:"config"`
}

func (r configResult) Headers() []string { return nil }
func (r configResult) Rows() [][]string {
	kv := render.KV{{"path", r.Path}, {"host", r.File.Host}, {"port", strconv.Itoa(r.File.Port)}, {"timeout", r.File.Timeout.String()}, {"output", r.File.Output}, {"quiet", strconv.FormatBool(r.File.Quiet)}}
	return kv.Rows()
}
func (r configResult) Plain() string { return r.Path }

func (a *App) newConfigCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Show or write the configuration file (flags > env > file > defaults)",
	}
	show := &cobra.Command{
		Use:   "show",
		Short: "Print the configuration file location and contents",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			f, path, err := config.Load()
			if err != nil {
				return &cameraError{err: err, code: exitcode.Config}
			}
			return a.print(cmd, configResult{Path: path, File: f})
		},
	}
	path := &cobra.Command{
		Use:   "path",
		Short: "Print the configuration file path",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := config.Path()
			if err != nil {
				return &cameraError{err: err, code: exitcode.Config}
			}
			_, err = cmd.OutOrStdout().Write([]byte(p + "\n"))
			return err
		},
	}
	set := &cobra.Command{
		Use:   "set",
		Short: "Write the current global flags (--host, --port, --timeout, --output, --quiet) to the config file",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			f, _, err := config.Load()
			if err != nil {
				return &cameraError{err: err, code: exitcode.Config}
			}
			// Parsing happened on the subcommand's merged flag set, so inspect each
			// flag's Changed bit rather than the root set's visit list.
			cmd.Root().PersistentFlags().VisitAll(func(fl *pflag.Flag) {
				if !fl.Changed {
					return
				}
				switch fl.Name {
				case "host":
					f.Host = a.opts.Host
				case "port":
					f.Port = a.opts.Port
				case "timeout":
					f.Timeout = a.opts.Timeout
				case "output":
					f.Output = a.opts.Output.String()
				case "quiet":
					f.Quiet = a.opts.Quiet
				}
			})
			p, err := config.Save(f)
			if err != nil {
				return &cameraError{err: err, code: exitcode.IOErr}
			}
			return a.print(cmd, configResult{Path: p, File: f})
		},
	}
	cmd.AddCommand(show, path, set)
	return cmd
}
