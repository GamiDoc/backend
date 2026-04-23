package cli

import (
	"fmt"

	"github.com/gamidoc/backend/internal/version"
	"github.com/spf13/cobra"
)

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "version: %s\n", version.Version)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "commit: %s\n", version.Commit)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "build_time: %s\n", version.BuildTime)
			return nil
		},
	}
}
