package cmd

import (
	"log/slog"

	"github.com/disgoorg/bot-template/bottemplate/migration"
	"github.com/disgoorg/bot-template/internal/gateways/database"
	"github.com/spf13/cobra"
)

var migrateCMD = &cobra.Command{
	Use:   "migrate",
	Short: "migrate old database to new one services",
	RunE: func(cmd *cobra.Command, args []string) error {

		ctx := cmd.Context()

		db, err := database.New(ctx, database.DBConfig{
			Host:     "localhost",
			Port:     5432,
			User:     "root",
			Password: "root",
			Database: "postgres",
			PoolSize: 10,
		})
		if err != nil {
			slog.Error("Failed to connect to database", "error", err)
			return err
		}
		defer db.Close()

		// Create migrator using the bunDB from our DB instance
		migrator := migration.NewMigrator(
			db.BunDB(), // Use BunDB() instead of GetBunDB()
			"../../data/users.bson",
			"../../data/usercards.bson",
		)

		// Run migration
		if err := migrator.MigrateAll(ctx); err != nil {
			slog.Error("Migration failed", "error", err)
			return err
		}

		slog.Info("Migration completed successfully!")

		return nil
	},
}

func init() {
	rootCmd.AddCommand(migrateCMD)
}

// Initialize PostgreSQL connection using existing DB package
