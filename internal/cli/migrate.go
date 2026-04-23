package cli

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/yifen9/gamidoc-backend/config"
	"github.com/yifen9/gamidoc-backend/internal/migrate"
	"github.com/yifen9/gamidoc-backend/internal/storage/postgres"
)

func newMigrateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Run database migrations",
	}

	cmd.AddCommand(newMigrateUpCommand())
	cmd.AddCommand(newMigrateStatusCommand())

	return cmd
}

func newMigrateUpCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "up",
		Short: "Apply pending migrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Load()

			db, err := postgres.New(cfg.PostgresDSN())
			if err != nil {
				return err
			}
			defer func() {
				_ = db.Close()
			}()

			ctx := context.Background()
			m := migrate.NewMigrator(db, cfg.MigrationsDir)

			statuses, err := m.Up(ctx)
			if err != nil {
				return err
			}

			printMigrationStatuses(statuses)
			return nil
		},
	}
}

func newMigrateStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show migration status",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Load()

			db, err := postgres.New(cfg.PostgresDSN())
			if err != nil {
				return err
			}
			defer func() {
				_ = db.Close()
			}()

			ctx := context.Background()
			m := migrate.NewMigrator(db, cfg.MigrationsDir)

			statuses, err := m.Status(ctx)
			if err != nil {
				return err
			}

			printMigrationStatuses(statuses)
			return nil
		},
	}
}

func printMigrationStatuses(statuses []migrate.StatusEntry) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "NAME\tAPPLIED")
	for _, item := range statuses {
		_, _ = fmt.Fprintf(w, "%s\t%t\n", item.Name, item.Applied)
	}
	_ = w.Flush()
}
