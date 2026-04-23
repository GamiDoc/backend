package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/yifen9/gamidoc-backend/internal/version"
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
