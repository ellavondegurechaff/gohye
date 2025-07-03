# GoHYE Project Overview

## Purpose
GoHYE is a sophisticated Discord bot built in Go that implements a collectible K-pop card trading game with a complete economic system. The bot features card claiming, trading, auctions, forging, daily activities, and a dynamic pricing system.

## Tech Stack
- **Language**: Go 1.22+
- **Discord Framework**: disgo v0.18.7
- **Database**: PostgreSQL with pgx driver
- **ORM**: Bun ORM
- **Storage**: DigitalOcean Spaces (S3-compatible) for card images
- **Architecture Pattern**: Repository pattern with service layer

## Project Structure
```
bottemplate/
├── bot.go              # Main bot struct and initialization
├── commands/           # Discord slash commands (35+ commands)
│   ├── admin/
│   ├── cards/
│   ├── economy/
│   ├── social/
│   └── system/
├── database/           # Database layer
│   ├── models/         # Data models
│   └── repositories/   # Repository pattern
├── economy/           # Economic systems
│   ├── auction/
│   ├── claim/
│   ├── effects/
│   ├── forge/
│   └── vials/
├── handlers/          # Event and command handlers
├── services/          # Business logic and external services
└── utils/             # Shared utilities
```

## Key Features
- Card collection system with levels and animations
- Dynamic pricing based on scarcity and activity
- Real-time auction system
- Daily/work economy activities
- Effects system for temporary buffs
- Collection progress tracking
- Wishlist system