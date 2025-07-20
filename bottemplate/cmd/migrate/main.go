package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/disgoorg/bot-template/bottemplate/database"
	"github.com/disgoorg/bot-template/bottemplate/migration"
)

func main() {
	ctx := context.Background()

	// Command line flags
	var (
		dataDir = flag.String("data", ".", "Directory containing MongoDB BSON export files")
		dbHost  = flag.String("host", "localhost", "PostgreSQL host")
		dbPort  = flag.Int("port", 5432, "PostgreSQL port")
		dbUser  = flag.String("user", "root", "PostgreSQL user")
		dbPass  = flag.String("password", "root", "PostgreSQL password")
		dbName  = flag.String("database", "postgres", "PostgreSQL database name")
		logDir  = flag.String("logdir", ".", "Directory to write migration logs")
	)
	flag.Parse()

	// Immediate debug output to see if we get this far
	fmt.Println("=== MIGRATION STARTING ===")
	fmt.Printf("Data directory: %s\n", *dataDir)
	fmt.Printf("DB Host: %s:%d\n", *dbHost, *dbPort)

	// Setup file logging for migration tracking
	fmt.Println("=== SETTING UP LOGGING ===")
	timestamp := time.Now().Format("20060102_150405")
	logFile := filepath.Join(*logDir, fmt.Sprintf("migration_%s.log", timestamp))
	fmt.Printf("Log file: %s\n", logFile)

	if err := setupFileLogging(logFile); err != nil {
		fmt.Printf("Failed to setup file logging: %v\n", err)
		slog.Error("Failed to setup file logging", "error", err)
		os.Exit(1)
	}
	fmt.Println("=== LOGGING SETUP COMPLETE ===")

	slog.Info("Migration started",
		"timestamp", timestamp,
		"logFile", logFile,
		"dataDir", *dataDir)

	// Initialize PostgreSQL connection using existing DB package
	fmt.Println("=== CONNECTING TO DATABASE ===")
	db, err := database.New(ctx, database.DBConfig{
		Host:     *dbHost,
		Port:     *dbPort,
		User:     *dbUser,
		Password: *dbPass,
		Database: *dbName,
		PoolSize: 10,
	})
	if err != nil {
		fmt.Printf("Failed to connect to database: %v\n", err)
		slog.Error("Failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	fmt.Println("=== DATABASE CONNECTED ===")

	// Initialize database schema first
	slog.Info("Initializing database schema")
	if err := db.InitializeSchema(ctx); err != nil {
		slog.Error("Failed to initialize database schema", "error", err)
		os.Exit(1)
	}
	slog.Info("Database schema initialized successfully")

	// Use comprehensive BSON migrator
	slog.Info("Starting comprehensive BSON migration")
	slog.Info("Data directory", "path", *dataDir)

	migrator := migration.NewMigrator(db.BunDB(), *dataDir)

	if err := migrator.MigrateAll(ctx); err != nil {
		slog.Error("BSON migration failed", "error", err)
		os.Exit(1)
	}

	slog.Info("Migration completed successfully!")
}

// setupFileLogging configures slog to write to both console and file
func setupFileLogging(logFile string) error {
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	// Create a multi-writer that writes to both stdout and file
	multiHandler := slog.NewTextHandler(file, &slog.HandlerOptions{
		Level:     slog.LevelDebug,
		AddSource: true,
	})

	logger := slog.New(multiHandler)
	slog.SetDefault(logger)

	return nil
}
