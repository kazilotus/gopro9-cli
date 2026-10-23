package cli

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

func (a *App) newDocsCommand(root *cobra.Command) *cobra.Command {
	var dir string
	cmd := &cobra.Command{
		Use:       "docs markdown|man",
		Short:     "Generate command reference documentation",
		Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		ValidArgs: []string{"markdown", "man"},
		RunE: func(_ *cobra.Command, args []string) error {
			if err := os.MkdirAll(dir, 0o750); err != nil {
				return err
			}
			root.DisableAutoGenTag = true
			switch args[0] {
			case "man":
				return doc.GenManTree(root, &doc.GenManHeader{Title: "GOPRO9", Section: "1"}, dir)
			default:
				return doc.GenMarkdownTree(root, dir)
			}
		},
	}
	cmd.Flags().StringVar(&dir, "dir", "docs/commands", "output directory")
	return cmd
}
