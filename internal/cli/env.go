package cli

import (
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// bindEnv applies GOPRO9_* environment variables to any persistent flag the
// user did not set explicitly. Flags always win over the environment.
func (a *App) bindEnv(cmd *cobra.Command) error {
	var err error
	cmd.Root().PersistentFlags().VisitAll(func(f *pflag.Flag) {
		if err != nil || f.Changed {
			return
		}
		key := envPrefix + strings.ToUpper(strings.ReplaceAll(f.Name, "-", "_"))
		if v, ok := os.LookupEnv(key); ok {
			err = f.Value.Set(v)
		}
	})
	return err
}
