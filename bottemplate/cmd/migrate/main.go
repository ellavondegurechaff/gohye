package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/disgoorg/bot-template/bottemplate/database"
	"github.com/disgoorg/bot-template/bottemplate/migration"
)

func main() {
	ctx := context.Background()

	// Initialize PostgreSQL connection using existing DB package
	db, err := database.New(ctx, database.DBConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "postgres",
		Password: "root",
		Database: "postgres",
		PoolSize: 10,
	})
	if err != nil {
		slog.Error("Failed to connect to database", "error", err)
		os.Exit(1)
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
		os.Exit(1)
	}

	slog.Info("Migration completed successfully!")
}
