package database

import (
    "context"
    "database/sql"
    "encoding/json"
    "fmt"
    "net"
    "os"
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
    schemaVersion        = 1 // bump when schema/migrations change
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

func New(ctx context.Context, cfg DBConfig) (*DB, error) {
    // Add retry logic for initial connection
    var conn net.Conn
    var err error

    tryDial := func() (net.Conn, error) {
        addr := net.JoinHostPort(cfg.Host, fmt.Sprintf("%d", cfg.Port))
        force4 := os.Getenv("DB_DIAL_FORCE_IPV4") == "1"
        force6 := os.Getenv("DB_DIAL_FORCE_IPV6") == "1"

        if force4 {
            return net.DialTimeout("tcp4", addr, defaultConnTimeout)
        }
        if force6 {
            return net.DialTimeout("tcp6", addr, defaultConnTimeout)
        }

        // Prefer IPv4, then fall back to IPv6
        if c, e := net.DialTimeout("tcp4", addr, defaultConnTimeout); e == nil {
            return c, nil
        }
        return net.DialTimeout("tcp6", addr, defaultConnTimeout)
    }

    for i := 0; i < defaultMaxRetries; i++ {
        conn, err = tryDial()
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
    // Default to disabling SSL for Bun unless explicitly overridden by env
    sslMode := os.Getenv("PG_SSLMODE")
    if sslMode == "" {
        sslMode = "disable"
    }

    dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
        pool.Config().ConnConfig.User,
        pool.Config().ConnConfig.Password,
        pool.Config().ConnConfig.Host,
        pool.Config().ConnConfig.Port,
        pool.Config().ConnConfig.Database,
        sslMode,
    )

    sqldb := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(dsn)))
    return bun.NewDB(sqldb, pgdialect.New())
}

// ResetAppTables truncates application tables for a fresh start (PostgreSQL only)
func (db *DB) ResetAppTables(ctx context.Context) error {
    if db.bunDB == nil {
        return fmt.Errorf("bun DB not initialized")
    }

    // Candidate tables managed by this application
    candidates := []string{
        "auction_bids",
        "auctions",
        "trades",
        "user_quest_progress",
        "quest_leaderboards",
        "quest_definitions",
        "quest_chains",
        "user_effects",
        "effect_items",
        "user_inventory",
        "user_items",
        "items",
        "card_market_history",
        "collection_resets",
        "collection_progress",
        "claims",
        "claim_stats",
        "economy_stats",
        "user_cards",
        "user_quests",
        "user_slots",
        "user_stats",
        "wishlists",
        "users",
        "cards",
        "collections",
    }

    // Verify present tables to avoid failures on missing ones
    rows, err := db.pool.Query(ctx, `SELECT table_name FROM information_schema.tables WHERE table_schema = 'public'`)
    if err != nil {
        return fmt.Errorf("failed to list tables: %w", err)
    }
    defer rows.Close()

    present := map[string]bool{}
    for rows.Next() {
        var name string
        if err := rows.Scan(&name); err == nil {
            present[name] = true
        }
    }

    var toTruncate []string
    for _, t := range candidates {
        if present[t] {
            toTruncate = append(toTruncate, t)
        }
    }

    if len(toTruncate) == 0 {
        slog.Warn("No app tables found to reset")
        return nil
    }

    // Build TRUNCATE statement safely
    stmt := "TRUNCATE TABLE " + joinIdentifiers(toTruncate) + " RESTART IDENTITY CASCADE;"
    if _, err := db.ExecWithLog(ctx, stmt); err != nil {
        return fmt.Errorf("failed to truncate tables: %w", err)
    }

    slog.Info("App tables truncated successfully", "tables", toTruncate)
    return nil
}

// joinIdentifiers joins identifiers with proper quoting
func joinIdentifiers(names []string) string {
    if len(names) == 0 {
        return ""
    }
    out := ""
    for i, n := range names {
        if i > 0 {
            out += ", "
        }
        out += fmt.Sprintf("\"%s\"", n)
    }
    return out
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
    // Fast init path for development: skip when schema version matches
    fastInit := os.Getenv("DB_FAST_INIT") == "1"
    if fastInit {
        if err := db.ensureAppMeta(ctx); err == nil {
            if v, _ := db.getAppMeta(ctx, "schema_version"); v == fmt.Sprintf("%d", schemaVersion) {
                slog.Info("Fast DB init: schema up-to-date, skipping initialization",
                    slog.String("mode", "DB_FAST_INIT"),
                    slog.Int("schema_version", schemaVersion))
                return nil
            }
        }
    }
	// First, ensure the database is using UTF-8 encoding
	if err := db.ensureUTF8Encoding(ctx); err != nil {
		return fmt.Errorf("failed to ensure UTF-8 encoding: %w", err)
	}

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
		(*models.Auction)(nil),
		(*models.AuctionBid)(nil),
		(*models.Trade)(nil),
		(*models.CardMarketHistory)(nil),
		(*models.Item)(nil),
		(*models.UserItem)(nil),
		// Quest system tables
		(*models.QuestChain)(nil),
		(*models.QuestDefinition)(nil),
		(*models.UserQuestProgress)(nil),
		(*models.QuestLeaderboard)(nil),
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

	// Apply schema migrations for existing tables FIRST
	if err := db.MigrateSchema(ctx); err != nil {
		return fmt.Errorf("failed to migrate schema: %w", err)
	}

	// Create indexes AFTER schema migrations
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
		// Critical performance indexes
		"CREATE INDEX IF NOT EXISTS idx_user_cards_user_id_amount ON user_cards(user_id) WHERE amount > 0;",
		"CREATE INDEX IF NOT EXISTS idx_user_cards_user_level ON user_cards(user_id, level DESC, id ASC) WHERE amount > 0;",
		"CREATE INDEX IF NOT EXISTS idx_user_cards_compound_search ON user_cards(user_id, card_id, amount) WHERE amount > 0;",
		"CREATE INDEX IF NOT EXISTS idx_auctions_status_end_time ON auctions(status, end_time);",
		"CREATE INDEX IF NOT EXISTS idx_auctions_active ON auctions(end_time) WHERE status = 'active';",
		"CREATE INDEX IF NOT EXISTS idx_claims_user_claimed ON claims(user_id, claimed_at);",
		// Trade system indexes
		"CREATE INDEX IF NOT EXISTS idx_trades_offerer_id ON trades(offerer_id);",
		"CREATE INDEX IF NOT EXISTS idx_trades_target_id ON trades(target_id);",
		"CREATE INDEX IF NOT EXISTS idx_trades_status ON trades(status);",
		"CREATE INDEX IF NOT EXISTS idx_trades_pending ON trades(status, expires_at) WHERE status = 'pending';",
		"CREATE INDEX IF NOT EXISTS idx_trades_user_trades ON trades(offerer_id, target_id, status);",
		// Effect system indexes (created after columns are added)
		"CREATE INDEX IF NOT EXISTS idx_user_effects_user_id ON user_effects(user_id);",
		"CREATE INDEX IF NOT EXISTS idx_user_effects_active ON user_effects(user_id, active);",
		"CREATE INDEX IF NOT EXISTS idx_user_effects_expires ON user_effects(expires_at) WHERE expires_at IS NOT NULL;",
		"CREATE INDEX IF NOT EXISTS idx_user_recipes_user_id ON user_recipes(user_id);",
		// Item system indexes
		"CREATE INDEX IF NOT EXISTS idx_user_items_user_id ON user_items(user_id);",
		"CREATE INDEX IF NOT EXISTS idx_items_type ON items(type);",
		// Quest system indexes
		"CREATE INDEX IF NOT EXISTS idx_quest_definitions_type_tier ON quest_definitions(type, tier);",
		"CREATE INDEX IF NOT EXISTS idx_quest_definitions_quest_id ON quest_definitions(quest_id);",
		"CREATE INDEX IF NOT EXISTS idx_user_quest_progress_user_id ON user_quest_progress(user_id);",
		"CREATE INDEX IF NOT EXISTS idx_user_quest_progress_user_quest ON user_quest_progress(user_id, quest_id);",
		"CREATE INDEX IF NOT EXISTS idx_user_quest_progress_completed ON user_quest_progress(user_id, completed) WHERE completed = true;",
		"CREATE INDEX IF NOT EXISTS idx_user_quest_progress_expires ON user_quest_progress(expires_at);",
		"CREATE INDEX IF NOT EXISTS idx_quest_leaderboards_period ON quest_leaderboards(period_type, period_start);",
		"CREATE INDEX IF NOT EXISTS idx_quest_leaderboards_user ON quest_leaderboards(user_id, period_type, period_start);",
	}

    for _, idx := range indexes {
        if _, err := db.ExecWithLog(ctx, idx); err != nil {
            return fmt.Errorf("failed to create index: %w", err)
        }
    }

	// Initialize item data
	if err := db.InitializeItemData(ctx); err != nil {
		return fmt.Errorf("failed to initialize item data: %w", err)
	}

	// Initialize quest data
    if err := db.InitializeQuestData(ctx); err != nil {
        return fmt.Errorf("failed to initialize quest data: %w", err)
    }

    // Update schema version marker (safe upsert)
    if err := db.ensureAppMeta(ctx); err == nil {
        _ = db.setAppMeta(ctx, "schema_version", fmt.Sprintf("%d", schemaVersion))
    }

    return nil
}

// ensureAppMeta creates the app_meta table if not exists
func (db *DB) ensureAppMeta(ctx context.Context) error {
    _, err := db.ExecWithLog(ctx, `CREATE TABLE IF NOT EXISTS app_meta (key TEXT PRIMARY KEY, value TEXT)`)
    return err
}

func (db *DB) getAppMeta(ctx context.Context, key string) (string, error) {
    row := db.pool.QueryRow(ctx, `SELECT value FROM app_meta WHERE key = $1`, key)
    var v string
    if err := row.Scan(&v); err != nil {
        return "", err
    }
    return v, nil
}

func (db *DB) setAppMeta(ctx context.Context, key, value string) error {
    sql := `INSERT INTO app_meta(key, value) VALUES($1, $2)
            ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`
    _, err := db.pool.Exec(ctx, sql, key, value)
    return err
}

// MigrateSchema applies necessary schema changes to existing tables
func (db *DB) MigrateSchema(ctx context.Context) error {
	// Add fragments column to collections table if it doesn't exist
	fragmentsColumnSQL := `
		ALTER TABLE collections 
		ADD COLUMN IF NOT EXISTS fragments BOOLEAN NOT NULL DEFAULT false;
	`

	if _, err := db.ExecWithLog(ctx, fragmentsColumnSQL); err != nil {
		return fmt.Errorf("failed to add fragments column: %w", err)
	}

	// Add missing columns to user_effects table if they don't exist
	userEffectsColumnsSQL := []string{
		`ALTER TABLE user_effects ADD COLUMN IF NOT EXISTS is_recipe BOOLEAN NOT NULL DEFAULT false;`,
		`ALTER TABLE user_effects ADD COLUMN IF NOT EXISTS recipe_cards JSONB;`,
		`ALTER TABLE user_effects ADD COLUMN IF NOT EXISTS active BOOLEAN NOT NULL DEFAULT false;`,
		`ALTER TABLE user_effects ADD COLUMN IF NOT EXISTS uses INTEGER NOT NULL DEFAULT 0;`,
		`ALTER TABLE user_effects ADD COLUMN IF NOT EXISTS expires_at TIMESTAMP;`,
		`ALTER TABLE user_effects ADD COLUMN IF NOT EXISTS cooldown_ends_at TIMESTAMP;`,
		`ALTER TABLE user_effects ADD COLUMN IF NOT EXISTS notified BOOLEAN NOT NULL DEFAULT true;`,
		`ALTER TABLE user_effects ADD COLUMN IF NOT EXISTS created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP;`,
		`ALTER TABLE user_effects ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP;`,
		`ALTER TABLE user_effects ADD COLUMN IF NOT EXISTS tier INTEGER NOT NULL DEFAULT 1;`,
		`ALTER TABLE user_effects ADD COLUMN IF NOT EXISTS progress INTEGER NOT NULL DEFAULT 0;`,
	}

	for _, sql := range userEffectsColumnsSQL {
		if _, err := db.ExecWithLog(ctx, sql); err != nil {
			return fmt.Errorf("failed to add user_effects column: %w", err)
		}
	}

	// Add metadata column to user_quest_progress table if it doesn't exist
	questMetadataSQL := `
		ALTER TABLE user_quest_progress 
		ADD COLUMN IF NOT EXISTS metadata JSONB;
	`

	if _, err := db.ExecWithLog(ctx, questMetadataSQL); err != nil {
		return fmt.Errorf("failed to add metadata column to user_quest_progress: %w", err)
	}

	// Add unique constraint to quest_leaderboards for upsert operations
	questLeaderboardConstraintSQL := `
		DO $$ 
		BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM pg_constraint 
				WHERE conname = 'quest_leaderboards_unique_user_period'
			) THEN
				ALTER TABLE quest_leaderboards 
				ADD CONSTRAINT quest_leaderboards_unique_user_period 
				UNIQUE (period_type, period_start, user_id);
			END IF;
		END $$;
	`

	if _, err := db.ExecWithLog(ctx, questLeaderboardConstraintSQL); err != nil {
		// Log but don't fail - constraint might already exist with different name
		slog.Warn("Failed to add unique constraint to quest_leaderboards (may already exist)",
			slog.Any("error", err))
	}

	// Fix JSONB fields in users table that might be stored as strings
	if err := db.MigrateUserJSONBFields(ctx); err != nil {
		return fmt.Errorf("failed to migrate user JSONB fields: %w", err)
	}

	return nil
}

// MigrateUserJSONBFields fixes JSONB fields that might be stored as strings
func (db *DB) MigrateUserJSONBFields(ctx context.Context) error {
	// Fix completed_cols field: convert string arrays to proper JSONB objects
	migrateCompletedColsSQL := `
		UPDATE users 
		SET completed_cols = CASE 
			WHEN completed_cols::text LIKE '["%' OR completed_cols::text = '[]' THEN 
				(
					SELECT COALESCE(
						jsonb_agg(
							jsonb_build_object('id', elem, 'amount', 0)
						), 
						'[]'::jsonb
					)
					FROM jsonb_array_elements_text(completed_cols) elem
				)
			ELSE completed_cols
		END
		WHERE completed_cols IS NOT NULL;
	`

	if _, err := db.ExecWithLog(ctx, migrateCompletedColsSQL); err != nil {
		return fmt.Errorf("failed to migrate completed_cols field: %w", err)
	}

	// Fix clouted_cols field: convert string arrays to proper JSONB objects
	migrateCloutedColsSQL := `
		UPDATE users 
		SET clouted_cols = CASE 
			WHEN clouted_cols::text LIKE '["%' OR clouted_cols::text = '[]' THEN 
				(
					SELECT COALESCE(
						jsonb_agg(
							jsonb_build_object('id', elem, 'amount', 1)
						), 
						'[]'::jsonb
					)
					FROM jsonb_array_elements_text(clouted_cols) elem
				)
			ELSE clouted_cols
		END
		WHERE clouted_cols IS NOT NULL;
	`

	if _, err := db.ExecWithLog(ctx, migrateCloutedColsSQL); err != nil {
		return fmt.Errorf("failed to migrate clouted_cols field: %w", err)
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

// ensureUTF8Encoding checks and ensures the database is using UTF-8 encoding
func (db *DB) ensureUTF8Encoding(ctx context.Context) error {
	// Check current database encoding
	var encoding string
	err := db.pool.QueryRow(ctx, "SHOW server_encoding;").Scan(&encoding)
	if err != nil {
		return fmt.Errorf("failed to check database encoding: %w", err)
	}

	slog.Info("Database encoding", "encoding", encoding)

	// If not UTF-8, log a warning but continue (changing encoding requires superuser)
	if encoding != "UTF8" {
		slog.Warn("Database is not using UTF-8 encoding, this may cause character encoding issues",
			"current_encoding", encoding,
			"recommended", "UTF8")
	}

	// Set client encoding to UTF-8 for this session
	_, err = db.pool.Exec(ctx, "SET client_encoding TO 'UTF8';")
	if err != nil {
		return fmt.Errorf("failed to set client encoding to UTF-8: %w", err)
	}

	slog.Info("Client encoding set to UTF-8")
	return nil
}

// InitializeItemData inserts the default items into the items table
func (db *DB) InitializeItemData(ctx context.Context) error {
	// Check if items already exist
	var itemCount int
	err := db.pool.QueryRow(ctx, "SELECT COUNT(*) FROM items WHERE id IN ('broken_disc', 'microphone', 'forgotten_song')").Scan(&itemCount)
	if err == nil && itemCount >= 3 {
		slog.Info("Item data already initialized, skipping")
		return nil
	}

	// Check database encoding to determine if we should use emojis
	var encoding string
	useEmojis := true
	err = db.pool.QueryRow(ctx, "SHOW server_encoding;").Scan(&encoding)
	if err == nil && encoding != "UTF8" {
		useEmojis = false
		slog.Info("Database encoding is not UTF8, using text representations instead of emojis", "encoding", encoding)
	}

	// Insert items one by one to handle encoding issues
	items := []struct {
		ID            string
		Name          string
		Description   string
		Emoji         string
		FallbackEmoji string
		Type          string
		Rarity        int
		MaxStack      int
	}{
		{
			ID:            "broken_disc",
			Name:          "Broken Disc",
			Description:   "A scratched and broken album disc. Part of a greater whole.",
			Emoji:         "💿",
			FallbackEmoji: "CD",
			Type:          "material",
			Rarity:        3,
			MaxStack:      999,
		},
		{
			ID:            "microphone",
			Name:          "Microphone",
			Description:   "A vintage microphone used by idols. Still has some magic in it.",
			Emoji:         "🎤",
			FallbackEmoji: "MIC",
			Type:          "material",
			Rarity:        3,
			MaxStack:      999,
		},
		{
			ID:            "forgotten_song",
			Name:          "Forgotten Song",
			Description:   "Sheet music for a song lost to time. The notes still resonate.",
			Emoji:         "📜",
			FallbackEmoji: "SONG",
			Type:          "material",
			Rarity:        3,
			MaxStack:      999,
		},
	}

	for _, item := range items {
		// Use parameterized query to handle encoding properly
		insertSQL := `
			INSERT INTO items (id, name, description, emoji, type, rarity, max_stack, created_at, updated_at) 
			VALUES ($1, $2, $3, $4, $5, $6, $7, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
			ON CONFLICT (id) DO UPDATE SET updated_at = CURRENT_TIMESTAMP;
		`

		// Use fallback emoji if database doesn't support UTF8
		emoji := item.Emoji
		if !useEmojis {
			emoji = item.FallbackEmoji
		}

		_, err := db.ExecWithLog(ctx, insertSQL,
			item.ID, item.Name, item.Description, emoji,
			item.Type, item.Rarity, item.MaxStack)
		if err != nil {
			// If emoji still fails, use fallback
			if useEmojis {
				slog.Warn("Failed to insert item with emoji, trying with fallback",
					slog.String("item", item.ID),
					slog.String("error", err.Error()))

				_, err = db.ExecWithLog(ctx, insertSQL,
					item.ID, item.Name, item.Description, item.FallbackEmoji,
					item.Type, item.Rarity, item.MaxStack)
				if err != nil {
					return fmt.Errorf("failed to insert item %s: %w", item.ID, err)
				}
			} else {
				return fmt.Errorf("failed to insert item %s: %w", item.ID, err)
			}
		}
	}

	slog.Info("Initial item data initialized successfully")
	return nil
}

// InitializeQuestData inserts or updates the default quest definitions
func (db *DB) InitializeQuestData(ctx context.Context) error {
    type questDef struct {
        ID                 string
        Name               string
        Description        string
        Tier               int
        Type               string
        Category           string
        RequirementType    string
        RequirementTarget  string
        RequirementCount   int
        RequirementMeta    map[string]interface{}
        RewardSnowflakes   int64
        RewardVials        int
        RewardXP           int
    }

    trainee := "trainee"
    debut := "debut"
    idol := "idol"

    quests := []questDef{
        // Daily Tier 1
        {"daily_t1_workaholic", "Workaholic", "Use /work 3 times", 1, "daily", trainee, "work_command", "", 3, nil, 250, 20, 25},
        {"daily_t1_gacha_beginner", "Gacha Beginner", "Claim 5 cards", 1, "daily", trainee, "card_claim", "", 5, nil, 250, 20, 25},
        {"daily_t1_starter_auctioneer", "Starter Auctioneer", "Bid on 1 auction", 1, "daily", trainee, "auction_bid", "", 1, nil, 250, 20, 25},
        {"daily_t1_levels_on_levels", "Levels on levels", "Level up any card 5 times", 1, "daily", trainee, "card_levelup", "", 5, nil, 250, 20, 25},
        {"daily_t1_tap_the_market", "Tap the Market", "Auction 1 card", 1, "daily", trainee, "auction_create", "", 1, nil, 250, 20, 25},

        // Daily Tier 2
        {"daily_t2_pull_party", "Pull Party", "Claim 10 cards", 2, "daily", debut, "card_claim", "", 10, nil, 400, 30, 35},
        {"daily_t2_fast_worker", "Fast Worker", "Use /work 10 times", 2, "daily", debut, "work_command", "", 10, nil, 400, 30, 35},
        {"daily_t2_level_grinder", "Level Grinder", "Level up any card 15 times", 2, "daily", debut, "card_levelup", "", 15, nil, 400, 30, 35},
        {"daily_t2_market_moves", "Market Moves", "Bid on 3 auctions", 2, "daily", debut, "auction_bid", "", 3, nil, 400, 30, 35},
        {"daily_t2_rising_collector", "Rising Collector", "Draw any card", 2, "daily", debut, "card_draw", "", 1, nil, 400, 30, 35},
        {"daily_flake_farmer", "Flake Farmer", "Earn 3000 snowflakes from any source", 2, "daily", debut, "snowflakes_earned", "", 3000, nil, 400, 30, 35},

        // Daily Tier 3
        {"daily_t3_community_engager", "Community Engager", "Trade 1 card with another player", 3, "daily", idol, "card_trade", "", 1, nil, 650, 50, 50},
        {"daily_t3_auction_hunter", "Auction Hunter", "Win 1 auction", 3, "daily", idol, "auction_win", "", 1, nil, 650, 50, 50},
        {"daily_t3_full_routine", "Full Routine", "Complete 8 different commands today", 3, "daily", idol, "command_count", "", 8, nil, 650, 50, 50},
        {"daily_t3_combo_player", "Combo Player", "Claim 8 cards, use /work 3 times, level up 10 times and auction 1 card", 3, "daily", idol, "combo", "", 4, map[string]interface{}{"claim": 8, "work": 3, "levelup": 10, "auction_create": 1}, 650, 50, 50},

        // Weekly Tier 1
        {"weekly_t1_week_starter", "Week Starter", "Use /work on 4 separate days", 1, "weekly", trainee, "work_days", "", 4, nil, 800, 70, 75},
        {"weekly_t1_lucky_hands", "Lucky Hands", "Claim 30 cards", 1, "weekly", trainee, "card_claim", "", 30, nil, 800, 70, 75},
        {"weekly_t1_light_upgrades", "Light Upgrades", "Combine onto another card 3 times", 1, "weekly", trainee, "card_levelup", "", 3, map[string]interface{}{"only_combine": true}, 800, 70, 75},
        {"weekly_t1_lowkey_trader", "Lowkey Trader", "Trade 3 cards with other players", 1, "weekly", trainee, "card_trade", "", 3, nil, 800, 70, 75},
        {"weekly_t1_collection_helper", "Collection Helper", "Draw 4 different cards", 1, "weekly", trainee, "card_draw", "", 4, nil, 800, 70, 75},

        // Weekly Tier 2
        {"weekly_t2_middle_manager", "Middle Manager", "Use /work 40 times total", 2, "weekly", debut, "work_command", "", 40, nil, 1200, 80, 90},
        {"weekly_t2_regular_puller", "Regular Puller", "Claim 50 cards", 2, "weekly", debut, "card_claim", "", 50, nil, 1200, 80, 90},
        {"weekly_t2_experienced_upgrader", "Experienced Upgrader", "Level up any card 80 times", 2, "weekly", debut, "card_levelup", "", 80, nil, 1200, 80, 90},
        {"weekly_t2_flipper", "Flipper", "Auction 5 cards", 2, "weekly", debut, "auction_create", "", 5, nil, 1200, 80, 90},
        {"weekly_balanced_routine", "Balanced Routine", "Level up cards on 5 different days", 2, "weekly", debut, "card_levelup", "", 5, map[string]interface{}{"track_days": true}, 1200, 80, 90},

        // Weekly Tier 3
        {"weekly_t3_weekly_champion", "Weekly Champion", "Complete all 3 daily quests on 6 separate days", 3, "weekly", idol, "daily_complete", "", 18, nil, 1500, 100, 110},
        {"weekly_t3_auction_veteran", "Auction Veteran", "Win 5 auctions", 3, "weekly", idol, "auction_win", "", 5, nil, 1500, 100, 110},
        {"weekly_t3_flake_farmer", "Flake Farmer", "Earn a total of 8,000 snowflakes this week", 3, "weekly", idol, "snowflakes_earned", "", 8000, nil, 1500, 100, 110},
        {"weekly_t3_mega_leveler", "Mega Leveler", "Level up a card to the max level", 3, "weekly", idol, "card_levelup", "", 1, map[string]interface{}{"max_level_only": true}, 1500, 100, 110},
        {"weekly_t3_grind_hero", "Grind Hero", "Use commands 150 times total this week", 3, "weekly", idol, "command_usage", "", 150, nil, 1500, 100, 110},

        // Monthly Tier 1
        {"monthly_t1_monthly_gacha_fan", "Monthly Gacha Fan", "Claim 150 cards", 1, "monthly", trainee, "card_claim", "", 150, nil, 2000, 150, 125},
        {"monthly_t1_advanced_collector", "Advanced Collector", "Draw 15 different cards", 1, "monthly", trainee, "card_draw", "", 15, nil, 2000, 150, 125},
        {"monthly_t1_consistent_worker", "Consistent Worker", "Use /work 300 times", 1, "monthly", trainee, "work_command", "", 300, nil, 2000, 150, 125},
        {"monthly_t1_light_auctioneer", "Light Auctioneer", "Auction 15 cards", 1, "monthly", trainee, "auction_create", "", 15, nil, 2000, 150, 125},

        // Monthly Tier 2
        {"monthly_t2_claim_machine", "Claim Machine", "Claim 250 cards", 2, "monthly", debut, "card_claim", "", 250, nil, 3000, 200, 175},
        {"monthly_t2_level_enthusiast", "Level Enthusiast", "Use /levelup 200 times", 2, "monthly", debut, "card_levelup", "", 200, nil, 3000, 200, 175},
        {"monthly_t2_weekly_finisher", "Weekly Finisher", "Complete all 3 Weekly quests in 2 separate weeks", 2, "monthly", debut, "weekly_complete", "", 6, nil, 3000, 200, 175},
        {"monthly_t2_rising_trader", "Rising Trader", "Earn 8,000 snowflakes from auctions", 2, "monthly", debut, "snowflakes_from_source", "auction", 8000, nil, 3000, 200, 175},
        {"monthly_level_addict", "Level Addict", "Use /levelup on 3 different days", 2, "monthly", debut, "card_levelup", "", 3, map[string]interface{}{"track_days": true}, 3000, 200, 175},

        // Monthly Tier 3
        {"monthly_t3_gacha_god", "Gacha God", "Claim 300 cards", 3, "monthly", idol, "card_claim", "", 300, nil, 5000, 250, 200},
        {"monthly_t3_evolutionist", "Evolutionist", "Ascend 4 times", 3, "monthly", idol, "ascend", "", 4, nil, 5000, 250, 200},
        {"monthly_t3_ultimate_flipper", "Ultimate Flipper", "Earn 20,000 snowflakes through auctions", 3, "monthly", idol, "snowflakes_from_source", "auction", 20000, nil, 5000, 250, 200},
        {"monthly_t3_all_star_player", "All-Star Player", "Complete all Daily quests on 20 different days", 3, "monthly", idol, "daily_complete", "", 60, nil, 5000, 250, 200},
        {"monthly_t3_monthly_conqueror", "Monthly Conqueror", "Complete all Weekly quests every week this month", 3, "monthly", idol, "weekly_complete", "", 12, nil, 5000, 250, 200},
    }

    insertSQL := `
        INSERT INTO quest_definitions (
            quest_id, name, description, tier, type, category,
            requirement_type, requirement_target, requirement_count, requirement_metadata,
            reward_snowflakes, reward_vials, reward_xp,
            created_at, updated_at
        ) VALUES (
            $1, $2, $3, $4, $5, $6,
            $7, $8, $9, $10::jsonb,
            $11, $12, $13,
            CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
        ) ON CONFLICT (quest_id) DO UPDATE SET
            name = EXCLUDED.name,
            description = EXCLUDED.description,
            tier = EXCLUDED.tier,
            type = EXCLUDED.type,
            category = EXCLUDED.category,
            requirement_type = EXCLUDED.requirement_type,
            requirement_target = EXCLUDED.requirement_target,
            requirement_count = EXCLUDED.requirement_count,
            requirement_metadata = EXCLUDED.requirement_metadata,
            reward_snowflakes = EXCLUDED.reward_snowflakes,
            reward_vials = EXCLUDED.reward_vials,
            reward_xp = EXCLUDED.reward_xp,
            updated_at = CURRENT_TIMESTAMP;
    `

    for _, q := range quests {
        meta := q.RequirementMeta
        if meta == nil {
            meta = map[string]interface{}{}
        }
        metaBytes, err := json.Marshal(meta)
        if err != nil {
            return fmt.Errorf("failed to marshal quest metadata for %s: %w", q.ID, err)
        }

        if _, err := db.ExecWithLog(ctx, insertSQL,
            q.ID, q.Name, q.Description, q.Tier, q.Type, q.Category,
            q.RequirementType, q.RequirementTarget, q.RequirementCount, string(metaBytes),
            q.RewardSnowflakes, q.RewardVials, q.RewardXP,
        ); err != nil {
            return fmt.Errorf("failed to upsert quest %s: %w", q.ID, err)
        }
    }

    slog.Info("Quest definitions initialized/updated successfully", slog.Int("count", len(quests)))
    return nil
}
