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
    "go.mongodb.org/mongo-driver/mongo"
    "go.mongodb.org/mongo-driver/mongo/options"
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
        mongoURI = flag.String("mongo-uri", "", "MongoDB connection URI (if set, migrates directly from Mongo)")
        mongoDB  = flag.String("mongo-db", "", "MongoDB database name (required if --mongo-uri is provided)")
        batchSize = flag.Int("batch-size", 1000, "Batch size for inserts (lower for poolers, e.g., 200)")
        resetOnError = flag.Bool("reset-on-error", false, "If set, truncates app tables on migration error (Postgres only)")
        resetBefore = flag.Bool("reset-before", false, "If set, truncates app tables before migration (Postgres only)")
        sleepMS = flag.Int("sleep-ms", 0, "Optional sleep in ms between batches/statements (helps poolers)")
        insertMode = flag.String("insert-mode", "batch", "Insert mode: batch (default) or single (pooler-friendly)")
        mongoCardsColl = flag.String("mongo-cards-coll", "", "Override Mongo cards collection name (default: cards)")
        mongoCollectionsColl = flag.String("mongo-collections-coll", "", "Override Mongo collections collection name (default: collections)")
        autoCreateMissing = flag.Bool("auto-create-missing-cards", false, "Auto-create placeholder cards for missing IDs referenced by usercards (default false; JSON backfill used instead)")
        useCopy = flag.Bool("use-copy", false, "Use pgx COPY for fastest bulk inserts (recommended for millions of rows)")
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

    // Optionally reset tables before starting
    if *resetBefore {
        slog.Warn("Resetting PostgreSQL app tables before migration")
        if err := db.ResetAppTables(ctx); err != nil {
            slog.Error("Failed to reset app tables", "error", err)
            os.Exit(1)
        }
    }

    // Initialize database schema first
    slog.Info("Initializing database schema")
    if err := db.InitializeSchema(ctx); err != nil {
        slog.Error("Failed to initialize database schema", "error", err)
        os.Exit(1)
    }
	slog.Info("Database schema initialized successfully")

    // Decide migration mode
    if *mongoURI != "" {
        // Direct-from-Mongo mode
        if *mongoDB == "" {
            fmt.Println("--mongo-db is required when --mongo-uri is provided")
            os.Exit(1)
        }

        slog.Info("Starting direct Mongo->Postgres migration")
        client, err := mongo.Connect(ctx, options.Client().ApplyURI(*mongoURI))
        if err != nil {
            slog.Error("Failed to connect to MongoDB", "error", err)
            os.Exit(1)
        }
        defer func() { _ = client.Disconnect(context.Background()) }()

        migrator := migration.NewMigrator(db.BunDB(), *dataDir)
        migrator.SetBatchSize(*batchSize)
        migrator.SetSleepBetween(*sleepMS)
        migrator.SetInsertMode(*insertMode)
        migrator.UseMongo(client, *mongoDB)
        migrator.UsePool(db.GetPool())
        if *mongoCardsColl != "" { migrator.SetMongoCollectionName("cards", *mongoCardsColl) }
        if *mongoCollectionsColl != "" { migrator.SetMongoCollectionName("collections", *mongoCollectionsColl) }
        migrator.SetAutoCreateMissingCards(*autoCreateMissing)
        migrator.SetUseCopy(*useCopy)

        if err := migrator.MigrateAllFromMongo(ctx); err != nil {
            slog.Error("Mongo migration failed", "error", err)
            if *resetOnError {
                slog.Warn("Reset-on-error enabled: truncating app tables")
                _ = db.ResetAppTables(ctx)
            }
            os.Exit(1)
        }
    } else {
        // Use comprehensive BSON migrator
        slog.Info("Starting comprehensive BSON migration")
        slog.Info("Data directory", "path", *dataDir)

        migrator := migration.NewMigrator(db.BunDB(), *dataDir)
        migrator.UsePool(db.GetPool())
        migrator.SetBatchSize(*batchSize)
        migrator.SetSleepBetween(*sleepMS)
        migrator.SetInsertMode(*insertMode)
        migrator.SetAutoCreateMissingCards(*autoCreateMissing)
        migrator.SetUseCopy(*useCopy)

        if err := migrator.MigrateAll(ctx); err != nil {
            slog.Error("BSON migration failed", "error", err)
            if *resetOnError {
                slog.Warn("Reset-on-error enabled: truncating app tables")
                _ = db.ResetAppTables(ctx)
            }
            os.Exit(1)
        }
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
