# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**GoHYE** is a sophisticated Discord bot built in Go that implements a collectible K-pop card trading game with a complete economic system. The bot features card claiming, trading, auctions, forging, daily activities, and a dynamic pricing system.

## Development Commands

### Local Development
```bash
# Run with development config and sync commands
go run . --config=config.toml --sync-commands=true

# Run with price calculation on startup
go run . --config=config.toml --calculate-prices=true

# Standard development run
go run .
```

### Production Build
```bash
# Build with version info
go build -ldflags="-X 'main.version=${VERSION}' -X 'main.commit=${COMMIT}'" -o bot

# Docker build
docker build -t gohye .

# Docker compose
docker-compose up
```

### Testing
No formal test suite exists currently. Testing is done through Discord interactions.

## Architecture Overview

### Core Structure
```
bottemplate/
├── bot.go              # Main bot struct and initialization
├── config.go           # Configuration management (TOML)
├── commands/           # Discord slash commands (35+ commands)
├── database/           # Database layer
│   ├── models/         # Data models (User, Card, Collection, etc.)
│   └── repositories/   # Repository pattern for data access
├── economy/           # Economic systems
│   ├── auction/       # Auction mechanics and management
│   ├── claim/         # Card claiming system with cooldowns
│   ├── effects/       # Temporary user effects system
│   ├── forge/         # Card crafting/upgrading system
│   └── vials/         # Virtual currency/crafting materials
├── handlers/          # Event and command handlers with logging
├── services/          # External services (AWS S3/DigitalOcean Spaces)
└── utils/             # Shared utilities and formatters
```

### Database Architecture
- **Primary**: PostgreSQL with pgx driver for performance
- **ORM**: Bun ORM for complex queries
- **Pattern**: Repository pattern for clean separation
- **Schema**: 14+ tables covering users, cards, collections, economy, auctions
- **Connection**: Pool-based with configurable pool size

### Economic System
- **Dynamic Pricing**: Algorithm based on scarcity, activity, ownership distribution
- **Price Updates**: Automatic recalculation every 6 hours
- **Auction System**: Real-time bidding with automated management
- **Vials System**: Virtual currency for card upgrades and purchases
- **Economic Monitoring**: Statistics tracking and health monitoring

## Key Commands Structure

### Command Categories
- **Card Management**: `/claim`, `/cards`, `/forge`, `/summon`, `/levelup`
- **User Features**: `/balance`, `/daily`, `/inventory`, `/work`, `/shop`
- **Social Features**: `/has`, `/miss`, `/diff`, `/wish`
- **Economy**: `/liquefy`, `/auction`, `/price-stats`
- **Limited Cards**: `/limitedcards`, `/limitedstats`
- **Admin**: `/dbtest`, `/manage-images`, `/analyze-economy`

### Command Handler Pattern
All commands use consistent wrapper pattern:
```go
h.Command("/commandname", handlers.WrapWithLogging("commandname", commands.CommandHandler(b)))
h.Component("/componentpath/", handlers.WrapComponentWithLogging("component", commands.ComponentHandler(b)))
```

## Configuration

### Required Configuration (config.toml)
```toml
[log]
level = "info"
format = "text" 
add_source = true

[bot]
token = "your_discord_token"
dev_guilds = [] # Guild IDs for command testing

[db]
host = "localhost"
port = 5432
user = "username"
password = "password"
database = "database_name"
pool_size = 10

[spaces]
key = "spaces_access_key"
secret = "spaces_secret_key"
region = "region"
bucket = "bucket_name"
card_root = "cards/"
```

## Important Development Patterns

### Repository Pattern
All database access goes through repository interfaces:
```go
type UserRepository interface {
    GetByDiscordID(ctx context.Context, discordID string) (*models.User, error)
    Create(ctx context.Context, user *models.User) error
    Update(ctx context.Context, user *models.User) error
}
```

### Effect System
Temporary user effects with automatic expiration:
```go
type Effect struct {
    Name      string    `bun:"name,pk"`
    UserID    string    `bun:"user_id,pk"`
    ExpiresAt time.Time `bun:"expires_at"`
    Data      string    `bun:"data"`
}
```

### Component System
Interactive Discord components for pagination and user actions:
- Buttons for navigation (next/prev pages)
- Select menus for filtering
- Modal forms for user input

### Card System Mechanics
- **Levels**: Cards have 1-5 levels with different rarities
- **Animated**: Some cards have animated variants
- **Collections**: Organized by K-pop groups (girl groups, boy groups)
- **Tags**: Filtering system for searching cards
- **Promo Status**: Special promotional cards with different behavior

## Database Schema Key Points

### Core Tables
- `users`: User profiles with Discord integration
- `cards`: Master card data with collection references  
- `user_cards`: User ownership with level/animated status
- `collections`: Card collections (K-pop groups)
- `claims`: Claim history and cooldown management
- `auctions`: Auction system with bidding
- `effects`: Temporary user effects
- `wishlist`: User card wishlists

### Performance Considerations
- Indexed queries for card lookups by user
- Connection pooling for concurrent Discord interactions
- Cached collection data for promo filtering
- Price calculation caching with 15-minute expiration

## Cloud Integration

### DigitalOcean Spaces (S3-compatible)
- Card image storage and CDN delivery
- Configured through `services.SpacesService`
- URL generation for Discord embeds
- Image management commands for admin

## Logging and Monitoring

### Structured Logging
- Custom logger with service name "GoHYE"
- Contextual logging with component and status fields
- Error tracking with detailed error information
- Performance monitoring for database operations

### Command Monitoring  
All commands wrapped with logging for:
- Execution time tracking
- Error rate monitoring
- User interaction patterns
- Component interaction tracking

## Bot Initialization Flow

1. **Configuration Loading**: Parse TOML config file
2. **Database Connection**: Initialize PostgreSQL with schema
3. **Repository Setup**: Initialize all repository instances
4. **Collection Cache**: Load and cache collection data
5. **Price Calculator**: Initialize dynamic pricing system
6. **Managers**: Setup claim, auction, and effect managers
7. **Command Registration**: Register all slash commands and components
8. **Gateway Connection**: Connect to Discord and start bot

## Development Notes

- **Module Name**: `github.com/disgoorg/bot-template` (legacy from template)
- **Go Version**: 1.22+
- **Discord Framework**: disgo v0.18.7
- **No Formal Tests**: Testing done through Discord interactions
- **Price Updates**: Automatic background process every 6 hours
- **Command Sync**: Use `--sync-commands=true` flag for development