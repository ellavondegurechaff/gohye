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

// Promo Collection Emoji Constants
const (
	// Seasonal Events
	EmojiHalloween   = "🎃"
	EmojiChristmas   = "❄"
	EmojiValentine   = "🍫"
	EmojiBirthday    = "🍰"
	EmojiEaster      = "🐰"
	EmojiLunar       = "🧧"

	// Special Events  
	EmojiLimited     = "🔥"
	EmojiSpecial     = "✨"
	EmojiLiveAuction = "💎"
	EmojiLottery     = "🎁"
	EmojiFanart      = "🎨"
	EmojiJackpot     = "🎯"
	EmojiSigned      = "💫"

	// Anniversary & Celebration
	EmojiAnniversary = "🎉"
	EmojiSmiley      = "☀️"
	EmojiFlowerCrown = "🌺"

	// Music & Album
	EmojiAlbumGirl   = "📀"
	EmojiAlbumBoy    = "📀"

	// Collaboration Events
	EmojiCollabGeneral = "🧩"
	EmojiTwizone       = "🌹"
	EmojiOneTheStory   = "🌸"
	EmojiBlackVelvet   = "🍨"
	EmojiStrayBTS      = "🧬"
	EmojiGot7Teen      = "🧭"
	EmojiItzidle       = "💄"
	EmojiMonstaEXO     = "👾"
	EmojiMystical      = "🦋"
	EmojiPetsEvent     = "🐾"

	// Year-specific Variants
	EmojiHalloween18   = "🍬"
	EmojiChristmas18   = "🎄"
	EmojiValentine19   = "💗"
	EmojiHalloween19   = "👻"
	EmojiChristmas19   = "☃️"
	EmojiBirthday20    = "🎈"
	EmojiHalloween20   = "🎃"
	EmojiXmas20        = "🎀"
	EmojiHalloween21   = "🔮"
	EmojiXmas21        = "☃️"
	EmojiValentines22  = "❣️"
	EmojiHalloween23   = "👻"
	EmojiWinter23      = "⛸️"
	EmojiChuseok24     = "🌕"
	EmojiXmas24        = "🌧️"
	EmojiSummerEvent   = "🍹"
)

// PromoCollectionEmojis maps collection IDs to their special emojis
// 
// To add a new promo collection:
// 1. Add the emoji constant above in the appropriate category
// 2. Add the mapping here: "collection_id": EmojiConstant,
// 3. The system will automatically use the emoji for all cards in that collection
//
// Note: Collection IDs should match exactly with the database col_id field
var PromoCollectionEmojis = map[string]string{
	// Main seasonal events
	"halloween":      EmojiHalloween,
	"christmas":      EmojiChristmas,
	"valentine":      EmojiValentine,
	"birthdays":      EmojiBirthday,
	"easter21":       EmojiEaster,
	"lunar":          EmojiLunar,

	// Special events
	"limited":        EmojiLimited,
	"special":        EmojiSpecial,
	"liveauction":    EmojiLiveAuction,
	"lottery":        EmojiLottery,
	"fanarts":        EmojiFanart,
	"jackpot":        EmojiJackpot,
	"signed":         EmojiSigned,

	// Anniversary & celebration
	"anniversary21":  EmojiAnniversary,
	"smileyevent":    EmojiSmiley,
	"flowercrown":    EmojiFlowerCrown,

	// Music & albums
	"ggalbums":       EmojiAlbumGirl,
	"bgalbums":       EmojiAlbumBoy,

	// Collaboration events (using general collab emoji)
	"izonesomeday":       EmojiCollabGeneral,
	"loonaoec":           EmojiCollabGeneral,
	"izonedaydream":      EmojiCollabGeneral,
	"izonepinkblusher":   EmojiCollabGeneral,
	"gugudansemina":      EmojiCollabGeneral,
	"loonaonethird":      EmojiCollabGeneral,
	"loonayyxy":          EmojiCollabGeneral,
	"pristinv":           EmojiCollabGeneral,
	"snsdohggsnsdtts":    EmojiCollabGeneral,
	"wjmk":               EmojiCollabGeneral,
	"wjsnchocome":        EmojiCollabGeneral,
	"exocbx":             EmojiCollabGeneral,
	"seventeenbss":       EmojiCollabGeneral,
	"day6evenofday":      EmojiCollabGeneral,
	"btobblue":           EmojiCollabGeneral,
	"snsdohgg":           EmojiCollabGeneral,
	"snsdtts":            EmojiCollabGeneral,
	"orangecaramel":      EmojiCollabGeneral,
	"btob4u":             EmojiCollabGeneral,
	"ninemusesa":         EmojiCollabGeneral,
	"rainbowpixie":       EmojiCollabGeneral,
	"rainbowblaxx":       EmojiCollabGeneral,
	"superjuniorkry":     EmojiCollabGeneral,
	"aoacream":           EmojiCollabGeneral,
	"pinkfantasyshadow":  EmojiCollabGeneral,
	"pinkfantasyshy":     EmojiCollabGeneral,
	"spicas":             EmojiCollabGeneral,
	"fanaticsflavor":     EmojiCollabGeneral,
	"tripleh":            EmojiCollabGeneral,
	"wowthing":           EmojiCollabGeneral,
	"berrygoodhh":        EmojiCollabGeneral,
	"honeybee":           EmojiCollabGeneral,
	"girlsnextdoor":      EmojiCollabGeneral,
	"nuestw":             EmojiCollabGeneral,
	"sunnygirls":         EmojiCollabGeneral,
	"teenteen":           EmojiCollabGeneral,
	"purplehashtag":      EmojiCollabGeneral,
	"notfriends":         EmojiCollabGeneral,
	"hairintheair":       EmojiCollabGeneral,
	"rgpbside":           EmojiCollabGeneral,
	"wjsntheblack":       EmojiCollabGeneral,
	"taran4":             EmojiCollabGeneral,
	"elastu":             EmojiCollabGeneral,

	// Special collaboration events (unique emojis)
	"twizonevent":    EmojiTwizone,
	"onethestory":    EmojiOneTheStory,
	"blackvelvet":    EmojiBlackVelvet,
	"straybts":       EmojiStrayBTS,
	"got7teen":       EmojiGot7Teen,
	"itzidle":        EmojiItzidle,
	"monstaexo":      EmojiMonstaEXO,
	"mystical":       EmojiMystical,
	"petsevent":      EmojiPetsEvent,

	// Year-specific variants
	"halloween18":    EmojiHalloween18,
	"christmas18":    EmojiChristmas18,
	"valentine19":    EmojiValentine19,
	"halloween19":    EmojiHalloween19,
	"christmas19":    EmojiChristmas19,
	"birthday20":     EmojiBirthday20,
	"halloween20":    EmojiHalloween20,
	"xmas20":         EmojiXmas20,
	"halloween21":    EmojiHalloween21,
	"xmas21":         EmojiXmas21,
	"valentines22":   EmojiValentines22,
	"halloween23":    EmojiHalloween23,
	"winterevent23":  EmojiWinter23,
	"chuseok24":      EmojiChuseok24,
	"xmas24":         EmojiXmas24,
	"summerevent":    EmojiSummerEvent,
}

// Helper functions for emoji management

// IsPromoCollection checks if a collection ID has a special promo emoji
func IsPromoCollection(colID string) bool {
	_, exists := PromoCollectionEmojis[colID]
	return exists
}

// GetPromoEmoji returns the emoji for a promo collection, or empty string if not promo
func GetPromoEmoji(colID string) string {
	return PromoCollectionEmojis[colID]
}

// AddPromoEmoji allows runtime addition of new promo emojis (for future extensibility)
func AddPromoEmoji(colID, emoji string) {
	PromoCollectionEmojis[colID] = emoji
}

// RemovePromoEmoji allows runtime removal of promo emojis (for future extensibility)
func RemovePromoEmoji(colID string) {
	delete(PromoCollectionEmojis, colID)
}

// GetAllPromoCollections returns all promo collection IDs
func GetAllPromoCollections() []string {
	collections := make([]string, 0, len(PromoCollectionEmojis))
	for colID := range PromoCollectionEmojis {
		collections = append(collections, colID)
	}
	return collections
}