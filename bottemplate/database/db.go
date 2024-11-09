package database

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
)

type DBConfig struct {
	Host     string `toml:"host"`
	Port     int    `toml:"port"`
	User     string `toml:"user"`
	Password string `toml:"password"`
	Database string `toml:"database"`
	PoolSize int    `toml:"pool_size"`
}

type DB struct {
	pool  *pgxpool.Pool
	bunDB *bun.DB
}

func New(ctx context.Context, cfg DBConfig) (*DB, error) {
	// First, try to establish a TCP connection to verify the host and port are reachable
	timeout := time.Second * 5
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", cfg.Host, cfg.Port), timeout)
	if err != nil {
		return nil, fmt.Errorf("database server unreachable: %w", err)
	}
	conn.Close()

	connString := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?connect_timeout=5",
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.Database,
	)

	poolConfig, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse connection string: %w", err)
	}

	if cfg.PoolSize > 0 {
		poolConfig.MaxConns = int32(cfg.PoolSize)
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Initialize bun.DB
	bunDB := newBunDB(pool)

	return &DB{
		pool:  pool,
		bunDB: bunDB,
	}, nil
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
		fmt.Printf("[DB Error] Query failed (%.3fms): %v\n", float64(duration.Microseconds())/1000.0, err)
		return result, err
	}

	fmt.Printf("[DB Success] Query executed (%.3fms): %s\n", float64(duration.Microseconds())/1000.0, sql)
	return result, nil
}

func (db *DB) QueryWithLog(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	start := time.Now()
	rows, err := db.pool.Query(ctx, sql, args...)
	duration := time.Since(start)

	if err != nil {
		fmt.Printf("[DB Error] Query failed (%.3fms): %v\n", float64(duration.Microseconds())/1000.0, err)
		return rows, err
	}

	fmt.Printf("[DB Success] Query executed (%.3fms): %s\n", float64(duration.Microseconds())/1000.0, sql)
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

	// Create user_cards table for tracking card ownership
	_, err = db.ExecWithLog(ctx, `
		CREATE TABLE IF NOT EXISTS user_cards (
			id BIGSERIAL PRIMARY KEY,
			user_id TEXT NOT NULL,
			card_id BIGINT NOT NULL REFERENCES cards(id) ON DELETE CASCADE,
			obtained_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, card_id)
		);
	`)
	if err != nil {
		return fmt.Errorf("failed to create user_cards table: %w", err)
	}

	// Create indexes
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_cards_col_id ON cards(col_id);",
		"CREATE INDEX IF NOT EXISTS idx_cards_name ON cards(name);",
		"CREATE INDEX IF NOT EXISTS idx_cards_level ON cards(level);",
		"CREATE INDEX IF NOT EXISTS idx_collections_name ON collections(name);",
		"CREATE INDEX IF NOT EXISTS idx_user_cards_user_id ON user_cards(user_id);",
		"CREATE INDEX IF NOT EXISTS idx_user_cards_card_id ON user_cards(card_id);",
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
