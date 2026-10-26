package cli

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

// docsCmd generates the reference docs from the command tree (hidden).
func (a *app) docsCmd() *cobra.Command {
	var dir, format string
	cmd := &cobra.Command{
		Use:    "docs",
		Short:  "Generate markdown or man page reference docs",
		Long:   "Write one file per command into --dir: markdown (the default) or man pages. Used to build docs/reference.",
		Hidden: true,
		Example: `  legion docs --dir docs/reference
  legion docs --dir man --format man`,
		Args: args(0, 0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if dir == "" {
				return usagef("legion docs --dir docs/reference", "--dir is required")
			}
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}
			root := cmd.Root()
			root.DisableAutoGenTag = true
			if format == "man" {
				return doc.GenManTree(root, &doc.GenManHeader{Title: "LEGION", Section: "1"}, dir)
			}
			return doc.GenMarkdownTree(root, dir)
		},
	}
	cmd.Flags().StringVar(&dir, "dir", "", "output directory")
	cmd.Flags().StringVar(&format, "format", "markdown", "markdown or man")
	return cmd
}
