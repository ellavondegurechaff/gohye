package database

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"time"

	"log/slog"

	"github.com/disgoorg/bot-template/bottemplate/database/models"
	"github.com/uptrace/bun"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
)

const (
	defaultConnTimeout   = 5 * time.Second
	defaultMaxRetries    = 3
	defaultRetryInterval = time.Second
)

type DBConfig struct {
	Host         string `toml:"host"`
	Port         int    `toml:"port"`
	User         string `toml:"user"`
	Password     string `toml:"password"`
	Database     string `toml:"database"`
	PoolSize     int    `toml:"pool_size"`
	MaxIdleConns int    `toml:"max_idle_conns"`
	MaxLifetime  int    `toml:"max_lifetime"`
}

type DB struct {
	pool  *pgxpool.Pool
	bunDB *bun.DB
}

type QueryLogger struct {
	Operation string
	Query     string
	Args      []interface{}
	StartTime time.Time
}

func newQueryLogger(operation, query string, args ...interface{}) *QueryLogger {
	return &QueryLogger{
		Operation: operation,
		Query:     query,
		Args:      args,
		StartTime: time.Now(),
	}
}

func (l *QueryLogger) log(err error, rowsAffected int64) {
	duration := time.Since(l.StartTime)

	if err != nil {
		slog.Error("Query failed",
			slog.String("type", "db"),
			slog.String("operation", l.Operation),
			slog.String("query", l.Query),
			slog.Any("args", l.Args),
			slog.Duration("took", duration),
			slog.Any("error", err),
		)
		return
	}

	slog.Info("Query executed",
		slog.String("type", "db"),
		slog.String("operation", l.Operation),
		slog.String("query", l.Query),
		slog.Any("args", l.Args),
		slog.Duration("took", duration),
		slog.Int64("affected_rows", rowsAffected),
	)
}

func New(ctx context.Context, cfg DBConfig) (*DB, error) {
	// Add retry logic for initial connection
	var conn net.Conn
	var err error

	for i := 0; i < defaultMaxRetries; i++ {
		conn, err = net.DialTimeout("tcp", fmt.Sprintf("%s:%d", cfg.Host, cfg.Port), defaultConnTimeout)
		if err == nil {
			break
		}
		time.Sleep(defaultRetryInterval)
	}
	if err != nil {
		return nil, fmt.Errorf("database server unreachable after %d attempts: %w", defaultMaxRetries, err)
	}
	defer conn.Close()

	poolConfig, err := pgxpool.ParseConfig(buildConnString(cfg))
	if err != nil {
		return nil, fmt.Errorf("failed to parse connection string: %w", err)
	}

	// Configure pool settings
	if cfg.PoolSize > 0 {
		poolConfig.MaxConns = int32(cfg.PoolSize)
	}
	if cfg.MaxIdleConns > 0 {
		poolConfig.MinConns = int32(cfg.MaxIdleConns)
	}
	if cfg.MaxLifetime > 0 {
		poolConfig.MaxConnLifetime = time.Duration(cfg.MaxLifetime) * time.Second
	}

	return createDB(ctx, poolConfig)
}

// Helper function to build connection string
func buildConnString(cfg DBConfig) string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?connect_timeout=5",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Database,
	)
}

// Helper function to create DB instance
func createDB(ctx context.Context, poolConfig *pgxpool.Config) (*DB, error) {
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	bunDB := newBunDB(pool)
	return &DB{pool: pool, bunDB: bunDB}, nil
}

func (db *DB) GetPool() *pgxpool.Pool {
	return db.pool
}

// Add method to get the bun.DB instance
func (db *DB) BunDB() *bun.DB {
	return db.bunDB
}

func newBunDB(pool *pgxpool.Pool) *bun.DB {
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		pool.Config().ConnConfig.User,
		pool.Config().ConnConfig.Password,
		pool.Config().ConnConfig.Host,
		pool.Config().ConnConfig.Port,
		pool.Config().ConnConfig.Database,
	)

	sqldb := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(dsn)))
	return bun.NewDB(sqldb, pgdialect.New())
}

func (db *DB) ExecWithLog(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
	start := time.Now()
	result, err := db.pool.Exec(ctx, sql, args...)
	duration := time.Since(start)

	if err != nil {
		slog.Error("Query failed",
			slog.String("type", "db"),
			slog.String("operation", "exec"),
			slog.String("query", sql),
			slog.Any("args", args),
			slog.Duration("took", duration),
			slog.Any("error", err),
		)
		return result, err
	}

	slog.Info("Query executed",
		slog.String("type", "db"),
		slog.String("operation", "exec"),
		slog.String("query", sql),
		slog.Any("args", args),
		slog.Duration("took", duration),
		slog.Int64("affected_rows", result.RowsAffected()),
	)
	return result, nil
}

func (db *DB) QueryWithLog(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	start := time.Now()
	rows, err := db.pool.Query(ctx, sql, args...)
	duration := time.Since(start)

	if err != nil {
		slog.Error("Query failed",
			slog.String("type", "db"),
			slog.String("operation", "query"),
			slog.String("query", sql),
			slog.Any("args", args),
			slog.Duration("took", duration),
			slog.Any("error", err),
		)
		return rows, err
	}

	slog.Info("Query executed",
		slog.String("type", "db"),
		slog.String("operation", "query"),
		slog.String("query", sql),
		slog.Any("args", args),
		slog.Duration("took", duration),
	)
	return rows, nil
}

func (db *DB) Close() {
	if db.pool != nil {
		db.pool.Close()
	}
	if db.bunDB != nil {
		db.bunDB.Close()
	}
}

// InitializeSchema creates all required database tables and indexes
func (db *DB) InitializeSchema(ctx context.Context) error {
	// Create tables in the correct order to handle foreign key constraints
	tables := []interface{}{
		(*models.Collection)(nil),
		(*models.Card)(nil),
		(*models.User)(nil),
		(*models.UserCard)(nil),
		(*models.UserQuest)(nil),
		(*models.UserSlot)(nil),
		(*models.UserStats)(nil),
		(*models.UserEffect)(nil),
		(*models.Claim)(nil),
		(*models.ClaimStats)(nil),
		(*models.EconomyStats)(nil),
		(*models.Wishlist)(nil),
		(*models.UserInventory)(nil),
		(*models.UserRecipe)(nil),
	}

	// Create tables using Bun
	for _, model := range tables {
		query := db.bunDB.NewCreateTable().
			Model(model).
			IfNotExists()

		_, err := query.Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to create table: %w", err)
		}
	}

	// Create indexes
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_cards_col_id ON cards(col_id);",
		"CREATE INDEX IF NOT EXISTS idx_cards_name ON cards(name);",
		"CREATE INDEX IF NOT EXISTS idx_cards_level ON cards(level);",
		"CREATE INDEX IF NOT EXISTS idx_collections_name ON collections(name);",
		"CREATE INDEX IF NOT EXISTS idx_user_cards_user_id ON user_cards(user_id);",
		"CREATE INDEX IF NOT EXISTS idx_user_cards_card_id ON user_cards(card_id);",
		"CREATE INDEX IF NOT EXISTS idx_user_cards_user_card ON user_cards(user_id, card_id);",
		"CREATE INDEX IF NOT EXISTS idx_economy_stats_timestamp ON economy_stats(timestamp);",
		"CREATE INDEX IF NOT EXISTS idx_economy_stats_economic_health ON economy_stats(economic_health);",
		"CREATE INDEX IF NOT EXISTS idx_claim_stats_user_id ON claim_stats(user_id);",
		"CREATE INDEX IF NOT EXISTS idx_claim_stats_last_claim ON claim_stats(last_claim_at);",
	}

	for _, idx := range indexes {
		if _, err := db.ExecWithLog(ctx, idx); err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}
	}

	return nil
}

// Ping verifies both database connections are working
func (db *DB) Ping(ctx context.Context) error {
	// Check pgxpool connection
	if err := db.pool.Ping(ctx); err != nil {
		return fmt.Errorf("pgxpool ping failed: %w", err)
	}

	// Check bun connection
	if err := db.bunDB.PingContext(ctx); err != nil {
		return fmt.Errorf("bun ping failed: %w", err)
	}

	return nil
}
