package cli

import "github.com/spf13/cobra"

func Execute() error {
	return newRootCommand().Execute()
}

func newRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "gamidoc-backend",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.AddCommand(newServeCommand())
	cmd.AddCommand(newMigrateCommand())
	cmd.AddCommand(newDoctorCommand())
	cmd.AddCommand(newVersionCommand())

	return cmd
}
