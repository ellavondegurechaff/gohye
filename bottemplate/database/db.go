package database

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"time"

	"log/slog"

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
	// Create collections table
	_, err := db.ExecWithLog(ctx, `
		CREATE TABLE IF NOT EXISTS collections (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			origin TEXT,
			aliases JSONB,
			promo BOOLEAN NOT NULL DEFAULT false,
			compressed BOOLEAN NOT NULL DEFAULT false,
			tags JSONB,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL
		);
	`)
	if err != nil {
		return fmt.Errorf("failed to create collections table: %w", err)
	}

	// Create cards table
	_, err = db.ExecWithLog(ctx, `
		CREATE TABLE IF NOT EXISTS cards (
			id BIGSERIAL PRIMARY KEY,
			name TEXT NOT NULL,
			level INTEGER NOT NULL,
			animated BOOLEAN NOT NULL,
			col_id TEXT NOT NULL REFERENCES collections(id),
			tags JSONB,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL
		);
	`)
	if err != nil {
		return fmt.Errorf("failed to create cards table: %w", err)
	}

	// Create user_cards table with all fields
	_, err = db.ExecWithLog(ctx, `
		CREATE TABLE IF NOT EXISTS user_cards (
			id BIGSERIAL PRIMARY KEY,
			user_id TEXT NOT NULL,
			card_id BIGINT NOT NULL REFERENCES cards(id) ON DELETE CASCADE,
			favorite BOOLEAN NOT NULL DEFAULT false,
			locked BOOLEAN NOT NULL DEFAULT false,
			amount BIGINT NOT NULL DEFAULT 1,
			rating BIGINT NOT NULL DEFAULT 0,
			obtained TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			exp BIGINT NOT NULL DEFAULT 0,
			mark TEXT DEFAULT '',
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL,
		);
	`)
	if err != nil {
		return fmt.Errorf("failed to create user_cards table: %w", err)
	}

	// Create all required indexes
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_cards_col_id ON cards(col_id);",
		"CREATE INDEX IF NOT EXISTS idx_cards_name ON cards(name);",
		"CREATE INDEX IF NOT EXISTS idx_cards_level ON cards(level);",
		"CREATE INDEX IF NOT EXISTS idx_collections_name ON collections(name);",
		"CREATE INDEX IF NOT EXISTS idx_user_cards_user_id ON user_cards(user_id);",
		"CREATE INDEX IF NOT EXISTS idx_user_cards_card_id ON user_cards(card_id);",
		"CREATE INDEX IF NOT EXISTS idx_user_cards_user_card ON user_cards(user_id, card_id);",
	}

	for _, idx := range indexes {
		if _, err := db.ExecWithLog(ctx, idx); err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}
	}

	// Create user_quests table
	_, err = db.ExecWithLog(ctx, `
		CREATE TABLE IF NOT EXISTS user_quests (
			id BIGSERIAL PRIMARY KEY,
			user_id TEXT NOT NULL,
			quest_id TEXT NOT NULL,
			type TEXT NOT NULL,
			completed BOOLEAN NOT NULL DEFAULT false,
			created_at TIMESTAMP NOT NULL,
			expires_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL,
			UNIQUE(user_id, quest_id)
		);
		CREATE INDEX IF NOT EXISTS idx_user_quests_user_id ON user_quests(user_id);
		CREATE INDEX IF NOT EXISTS idx_user_quests_type ON user_quests(type);
	`)
	if err != nil {
		return fmt.Errorf("failed to create user_quests table: %w", err)
	}

	// Create user_slots table
	_, err = db.ExecWithLog(ctx, `
		CREATE TABLE IF NOT EXISTS user_slots (
			id BIGSERIAL PRIMARY KEY,
			discord_id TEXT NOT NULL,
			effect_name TEXT,
			slot_expires TIMESTAMP,
			cooldown TIMESTAMP,
			is_active BOOLEAN NOT NULL DEFAULT true,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL,
			UNIQUE(discord_id, effect_name)
		);
		CREATE INDEX IF NOT EXISTS idx_user_slots_discord_id ON user_slots(discord_id);
	`)
	if err != nil {
		return fmt.Errorf("failed to create user_slots table: %w", err)
	}

	// Create user_stats table
	_, err = db.ExecWithLog(ctx, `
		CREATE TABLE IF NOT EXISTS user_stats (
			id BIGSERIAL PRIMARY KEY,
			discord_id TEXT NOT NULL UNIQUE,
			username TEXT NOT NULL,
			last_daily TIMESTAMP,
			-- Core Stats
			claims BIGINT NOT NULL DEFAULT 0,
			promo_claims BIGINT NOT NULL DEFAULT 0,
			total_reg_claims BIGINT NOT NULL DEFAULT 0,
			train BIGINT NOT NULL DEFAULT 0,
			work BIGINT NOT NULL DEFAULT 0,
			-- Other fields omitted for brevity, add all fields from the struct
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_user_stats_discord_id ON user_stats(discord_id);
		CREATE INDEX IF NOT EXISTS idx_user_stats_last_daily ON user_stats(last_daily);
	`)
	if err != nil {
		return fmt.Errorf("failed to create user_stats table: %w", err)
	}

	// Create users table
	_, err = db.ExecWithLog(ctx, `
		CREATE TABLE IF NOT EXISTS users (
			id BIGSERIAL PRIMARY KEY,
			discord_id TEXT NOT NULL UNIQUE,
			username TEXT NOT NULL,
			exp BIGINT NOT NULL DEFAULT 0,
			promo_exp BIGINT NOT NULL DEFAULT 0,
			joined TIMESTAMP NOT NULL,
			last_queried_card JSONB,
			last_kofi_claim TIMESTAMP,

			-- Stats stored as JSONB
			daily_stats JSONB NOT NULL DEFAULT '{}',
			effect_stats JSONB NOT NULL DEFAULT '{}',
			user_stats JSONB NOT NULL DEFAULT '{}',

			-- Arrays stored as JSONB
			cards JSONB NOT NULL DEFAULT '[]',
			inventory JSONB NOT NULL DEFAULT '[]',
			completed_cols JSONB NOT NULL DEFAULT '[]',
			clouted_cols JSONB NOT NULL DEFAULT '[]',
			achievements JSONB NOT NULL DEFAULT '[]',
			effects JSONB NOT NULL DEFAULT '[]',
			wishlist JSONB NOT NULL DEFAULT '[]',

			-- Timestamps
			last_daily TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			last_train TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			last_work TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			last_vote TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			last_announce TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			last_msg TEXT,

			-- Hero System
			hero_slots JSONB NOT NULL DEFAULT '[]',
			hero_cooldown JSONB NOT NULL DEFAULT '[]',
			hero TEXT,
			hero_changed TIMESTAMP,
			hero_submits INTEGER NOT NULL DEFAULT 0,

			-- User Status
			roles JSONB NOT NULL DEFAULT '[]',
			ban JSONB NOT NULL DEFAULT '{}',

			-- Premium
			premium BOOLEAN NOT NULL DEFAULT false,
			premium_expires TIMESTAMP,

			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL,

			CONSTRAINT users_discord_id_unique UNIQUE (discord_id)
		);
		CREATE INDEX IF NOT EXISTS idx_users_discord_id ON users(discord_id);
		CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
		CREATE INDEX IF NOT EXISTS idx_users_exp ON users(exp);
		CREATE INDEX IF NOT EXISTS idx_users_premium ON users(premium);
	`)
	if err != nil {
		return fmt.Errorf("failed to create users table: %w", err)
	}

	// Create user_effects table
	_, err = db.ExecWithLog(ctx, `
		CREATE TABLE IF NOT EXISTS user_effects (
			id BIGSERIAL PRIMARY KEY,
			user_id TEXT NOT NULL,
			effect_id TEXT NOT NULL,
			uses INTEGER NOT NULL DEFAULT 0,
			cooldown_ends TIMESTAMP,
			expires TIMESTAMP,
			notified BOOLEAN NOT NULL DEFAULT true,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL,
			UNIQUE(user_id, effect_id)
		);
		CREATE INDEX IF NOT EXISTS idx_user_effects_user_id ON user_effects(user_id);
		CREATE INDEX IF NOT EXISTS idx_user_effects_effect_id ON user_effects(effect_id);
	`)
	if err != nil {
		return fmt.Errorf("failed to create user_effects table: %w", err)
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
