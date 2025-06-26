package config

import "time"

// Application-wide constants organized by domain

// UI and Display Constants
const (
	// Pagination
	CardsPerPage     = 7
	DefaultPageSize  = 10
	MaxPageSize      = 25

	// Colors
	ErrorColor   = 0xFF0000
	SuccessColor = 0x00FF00
	InfoColor    = 0x0099FF
	WarningColor = 0xFFAA00
	
	// Discord UI Colors
	BackgroundColor     = 0x2B2D31
	EmbedDefaultColor   = 0x2B2D31
	
	// Rarity Colors
	RarityCommonColor    = 0x808080  // Gray for Level 1
	RarityUncommonColor  = 0x00FF00  // Green for Level 2
	RarityRareColor      = 0x0000FF  // Blue for Level 3
	RarityEpicColor      = 0x800080  // Purple for Level 4
	RarityLegendaryColor = 0xFFD700  // Gold for Level 5

	// Status indicators
	ActiveMarketStatus   = "🟢 Active"
	InactiveMarketStatus = "🔴 Inactive"
)

// Database and Performance Constants
const (
	// Timeouts
	DefaultQueryTimeout   = 30 * time.Second
	SearchTimeout         = 10 * time.Second
	MarketQueryTimeout    = 30 * time.Second
	StatsQueryTimeout     = 10 * time.Second
	InitialPricingTimeout = 5 * time.Minute
	BatchQueryTimeout     = 30 * time.Second
	CommandExecutionTimeout = 10 * time.Second
	WorkHandlerTimeout    = 10 * time.Second
	NetworkDialTimeout    = 5 * time.Second
	NetworkKeepAlive      = 30 * time.Second

	// Cache settings
	CacheExpiration      = 5 * time.Minute
	PriceCacheExpiration = 15 * time.Minute
	ImageCacheExpiration = 24 * time.Hour
	ImageCacheCleanupInterval = 1 * time.Hour
	CacheSize            = 10000

	// Batch processing
	DefaultBatchSize     = 50
	MaxBatchSize         = 25
	MinQueryBatchSize    = 100
	NumWorkers           = 3
	WorkerPoolSize       = 10
	MaxConcurrentBatches = 5
	ParallelQueries      = 4
	MaxRetries           = 3
)

// Economy and Pricing Constants
const (
	// Price bounds
	MinPrice = 500
	MaxPrice = 1000000

	// Base pricing
	InitialBasePrice  = 1000
	LevelMultiplier   = 1.5
	ScarcityBaseValue = 100
	ActivityBaseValue = 50

	// Economy monitoring
	EconomyCheckInterval    = 15 * time.Minute
	EconomyCorrectionDelay  = 6 * time.Hour
	EconomyTrendPeriod      = 30 * 24 * time.Hour
	DefaultAnalysisPeriod   = 30 * 24 * time.Hour

	// Minimums for calculations
	MinimumActiveOwners = 1
	MinimumTotalCopies  = 1
)

// Game Mechanics Constants
const (
	// Daily system
	DailyVialReward = 50
	DailyStreakBonus = 10
	DailyPeriod     = 24 * time.Hour

	// Work system
	WorkBasePayout    = 25
	WorkVialReward    = 5
	WorkCooldown      = 6 * time.Hour
	WorkMinCooldown   = 10 * time.Second
	DailyCooldown     = 24 * time.Hour

	// Claim system
	ClaimCooldown     = 1 * time.Hour
	GuaranteedClaim   = 10
	PitySystemThreshold = 50

	// Auction system
	MinAuctionDuration = 1 * time.Hour
	MaxAuctionDuration = 24 * time.Hour
	MinAuctionTime     = 10 * time.Second
	AuctionCleanupInterval = 5 * time.Minute
	MinBidIncrement = 100

	// Forge system
	ForgeBaseCost = 100
	VialsCostMultiplier = 1.5
)

// File and Storage Constants
const (
	// Image processing
	MaxImageSize     = 10 * 1024 * 1024 // 10MB
	ImageQuality     = 85
	ThumbnailSize    = 256

	// File paths
	CardImageRoot    = "cards/"
	UserAvatarRoot   = "avatars/"
	TempUploadPath   = "/tmp/uploads/"
)

// API and Rate Limiting Constants
const (
	// Rate limiting
	GlobalRateLimit     = 50
	UserRateLimit       = 10
	RateLimitWindow     = 1 * time.Minute
	RateLimitCooldown   = 5 * time.Minute

	// Request limits
	MaxRequestSize      = 1024 * 1024 // 1MB
	RequestTimeout      = 30 * time.Second
	MaxConcurrentRequests = 100
)

// Search and Filter Constants
const (
	// Search parameters
	MaxSearchResults    = 100
	SearchScoreThreshold = 0.1
	WeightedSearchLimit = 50

	// Filter parameters
	MaxTagsPerCard      = 10
	MaxCollectionsPerUser = 1000
	MaxCardsPerCollection = 10000
)

// Logging and Monitoring Constants
const (
	// Log levels
	LogLevelDebug = "debug"
	LogLevelInfo  = "info"
	LogLevelWarn  = "warn"
	LogLevelError = "error"

	// Monitoring intervals
	MetricsInterval     = 1 * time.Minute
	HealthCheckInterval = 30 * time.Second
	CleanupInterval     = 1 * time.Hour
)

// Security Constants
const (
	// Session and token limits
	SessionTimeout      = 24 * time.Hour
	TokenExpiration     = 1 * time.Hour
	MaxLoginAttempts    = 5
	LoginCooldown       = 15 * time.Minute

	// Data limits
	MaxUsernameLength   = 32
	MaxBioLength        = 500
	MaxCommandLength    = 2000
)