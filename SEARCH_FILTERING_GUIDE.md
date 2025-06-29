# GoHYE Search and Filtering Architecture Analysis

## Overview

The GoHYE Discord bot implements a sophisticated multi-layered search and filtering system for card collection management. The architecture consists of multiple components working together to provide fast, accurate, and flexible search capabilities.

## Core Architecture Components

### 1. Repository Layer (`bottemplate/database/repositories/`)

#### Card Repository (`card_repository.go`)
- **Primary Interface**: `CardRepository` interface with 20+ methods
- **Core Search Method**: `Search(ctx, filters, offset, limit) ([]*models.Card, int, error)`
- **Search Filters**: Uses `repositories.SearchFilters` struct
- **Caching**: Built-in sync.Map-based caching with expiration
- **Optimization Features**:
  - Query timeout handling (config.DefaultQueryTimeout)
  - Batch processing (maxBatchSize = 1000)
  - Advanced SQL query building with Bun ORM
  - Cache key generation and invalidation

#### Search Filters (`search_filters.go`)
```go
type SearchFilters struct {
    Name       string
    ID         int64
    Level      int
    Collection string
    Type       string
    Animated   bool
}
```

#### Enhanced Search Methods
- `GetByNameOrIDFuzzy()`: Optimized for exact matches (price stats)
- `SearchByNameFuzzy()`: Fuzzy search with limits (admin commands)
- `SearchAdminMode()`: Bypass promo/exclusion filters for admin

### 2. Service Layer (`bottemplate/services/`)

#### Search Service (`search_service.go`)
- **Purpose**: Unified search functionality across commands
- **Key Methods**:
  - `SearchUserCards()`: Search user's card collection
  - `SearchMissingCards()`: Find cards user doesn't own
  - `SearchCardsForDiff()`: Compare two users' collections
  - `SearchWishlistCards()`: Search wishlist (placeholder)
  - `GetSearchSuggestions()`: Auto-complete functionality

#### Unified Search Service (`unified_search_service.go`)
- **Purpose**: Advanced fuzzy search with third-party library
- **Library**: `github.com/sahilm/fuzzy` for improved matching
- **Key Features**:
  - Fuzzy matching with relevance scoring
  - Card name normalization (underscore/hyphen handling)
  - Basic filter application before fuzzy search
  - Admin mode bypassing all filters
  - Backward compatibility with existing WeightedSearch

#### Service Integration
- **CardOperationsService**: Card manipulation and search integration
- **CardDisplayService**: Formatting and presentation
- **CollectionService**: Collection-specific search operations

### 3. Utils Layer (`bottemplate/utils/`)

#### Search Utils (`search_utils.go`)
- **Advanced Filters**: 20+ filter types including enhanced filtering
- **Query Parser**: `ParseSearchQuery()` - converts text to structured filters
- **Weighted Search**: `WeightedSearch()` - relevance-based ranking
- **Collection Management**: Cached collection info with promo/exclusion logic

#### Enhanced Search Filters
```go
type SearchFilters struct {
    // Basic filters
    Query, Name string
    Levels []int
    Collections []string
    Animated, Favorites bool
    
    // Advanced filters
    AntiCollections []string     // !collection - exclude collections
    AntiLevels []int            // !level - exclude levels
    Tags, AntiTags []string     // #tag, !#tag
    AmountFilter AmountFilter   // >amount, <amount, =amount
    
    // User-specific filters
    ExcludeFavorites bool       // !fav
    LockedOnly bool            // -locked
    ExcludeLocked bool         // !locked
    SingleOnly bool            // !multi
    ExcludePromo bool          // !promo
    ExcludeAnimated bool       // !gif
    
    // Special filters
    LastCard bool              // . - last card filter
}
```

#### Query Syntax Support
- **Level Filters**: `3`, `level=3`, `-3`, `!3`
- **Collection Filters**: `twice`, `!twice`, `collection=exact_id`
- **Tag Filters**: `#girlgroups`, `!#boygroups`
- **Amount Filters**: `>amount=2`, `<amount=5`, `=amount=1`
- **Sort Options**: `>level`, `<name`, `>date`
- **Boolean Filters**: `gif`, `!gif`, `multi`, `!multi`, `promo`, `!promo`
- **User Filters**: `fav`, `!fav`, `locked`, `!locked`

### 4. Command Layer (`bottemplate/commands/`)

#### Search Commands
- **`/searchcards`** (`cards/searchcards.go`): Primary search interface
- **`/cards`** (`cards/cards.go`): User collection with search
- **`/has`** (`social/has.go`): Single card lookup
- **`/miss`** (`social/miss.go`): Missing cards search
- **`/diff`** (`social/diff.go`): Collection comparison

#### Command-Specific Features
- **Caching**: Per-command caching with custom expiration
- **Pagination**: Advanced pagination with search preservation
- **Timeouts**: Graceful timeout handling for complex queries
- **Error Handling**: Comprehensive error messages and fallbacks

### 5. Data Models (`bottemplate/database/models/`)

#### Core Models
- **Card** (`card.go`): Base card data with tags, collection, level
- **UserCard** (`user_card.go`): User ownership with amount, favorites, locked status
- **Collection** (`collection.go`): Collection metadata with promo status

#### Model Relationships
- Cards belong to Collections (`Card.Collection`)
- UserCards reference Cards (`UserCard.CardID`)
- Tags stored as JSONB arrays for flexible querying

## Search Architecture Patterns

### 1. Multi-Layer Search Strategy

```
User Query → ParseSearchQuery() → SearchFilters → Repository Search → Service Enhancement → Cached Results
```

### 2. Search Performance Optimization

#### Caching Strategy
- **Repository Level**: Query result caching with generated keys
- **Command Level**: Paginated result caching
- **Collection Info**: Static collection metadata caching
- **Expiration**: Configurable cache timeouts (15min default)

#### Query Optimization
- **Exact Match Priority**: ID and exact name matches first
- **Fuzzy Fallback**: Progressive fuzzy matching for partial queries
- **Filter Pre-Application**: Basic filters applied before fuzzy search
- **Batch Processing**: Large dataset handling with batching

### 3. Filter Application Order

1. **Collection Filters**: Promo/exclusion logic
2. **Level Filters**: Level matching and exclusions
3. **Tag Filters**: Group type and custom tag filtering
4. **User Filters**: Amount, favorites, locked status
5. **Search Term Matching**: Name-based fuzzy search
6. **Relevance Scoring**: Weight-based result ranking

## Current Search Capabilities

### 1. Basic Search Features
- ✅ Card name search (exact and partial)
- ✅ Card ID lookup
- ✅ Level filtering (1-5 stars)
- ✅ Collection filtering
- ✅ Animated card filtering
- ✅ Tag-based filtering (boygroups, girlgroups)

### 2. Advanced Search Features
- ✅ Multi-term query parsing
- ✅ Exclusion filters (!collection, !level)
- ✅ Amount-based filtering (>, <, =)
- ✅ Promo collection handling
- ✅ User-specific filters (favorites, locked)
- ✅ Sorting options (level, name, collection, date)

### 3. Performance Features
- ✅ Query result caching
- ✅ Fuzzy search with relevance scoring
- ✅ Timeout handling
- ✅ Pagination support
- ✅ Admin mode (bypass filters)

### 4. Integration Features
- ✅ Cross-command search consistency
- ✅ Collection comparison (diff)
- ✅ Missing cards detection
- ✅ Search suggestions/autocomplete
- ✅ Backward compatibility

## Key File Locations

### Core Search Components
- `/bottemplate/commands/cards/searchcards.go` - Primary search command
- `/bottemplate/services/search_service.go` - Unified search service
- `/bottemplate/services/unified_search_service.go` - Fuzzy search implementation
- `/bottemplate/database/repositories/card_repository.go` - Repository layer
- `/bottemplate/database/repositories/search_filters.go` - Basic filter struct
- `/bottemplate/utils/search_utils.go` - Advanced parsing and filtering

### Search Integration Points
- `/bottemplate/commands/cards/cards.go` - User collection search
- `/bottemplate/commands/social/has.go` - Single card lookup
- `/bottemplate/commands/social/miss.go` - Missing cards search
- `/bottemplate/commands/social/diff.go` - Collection comparison
- `/bottemplate/services/card_operations.go` - Card manipulation with search
- `/bottemplate/services/card_display.go` - Search result formatting

### Data Models
- `/bottemplate/database/models/card.go` - Core card structure
- `/bottemplate/database/models/user_card.go` - User ownership data
- `/bottemplate/database/models/collection.go` - Collection metadata

## Architecture Strengths

1. **Modularity**: Clear separation between repository, service, and command layers
2. **Performance**: Multi-level caching and query optimization
3. **Flexibility**: Advanced query syntax with 20+ filter types
4. **Consistency**: Unified search service across all commands
5. **Extensibility**: Easy to add new filter types and search methods
6. **Error Handling**: Comprehensive timeout and error recovery
7. **User Experience**: Fuzzy search with relevance scoring

## Potential Areas for Enhancement

1. **Search Analytics**: Query performance metrics and usage tracking
2. **Index Optimization**: Database index analysis for search queries
3. **Advanced Ranking**: Machine learning-based relevance scoring
4. **Real-time Search**: Live search suggestions and autocomplete
5. **Search History**: User search history and saved searches
6. **Export/Import**: Search result export and query sharing
7. **Visual Search**: Image-based card search capabilities

## Configuration and Constants

### Performance Settings
- `CardsPerPage`: 15 (pagination size)
- `CacheExpiration`: 15 minutes
- `SearchTimeout`: 30 seconds
- `maxBatchSize`: 1000 (repository batching)

### Weight Constants
- `WeightExactMatch`: 1000
- `WeightNameMatch`: 500
- `WeightCollectionMatch`: 200
- `WeightLevelMatch`: 100
- `WeightTypeMatch`: 50
- `WeightPrefixMatch`: 25
- `WeightPartialMatch`: 10

This architecture provides a robust, scalable, and user-friendly search system that efficiently handles the complex requirements of a card trading game while maintaining high performance and flexibility.

## Overview

GoHYE bot supports advanced search and filtering across multiple commands. This guide explains the unified search syntax that works across all card-related commands.

## Commands That Support Search

### Primary Commands
- `/cards query: <search>` - Search your owned cards
- `/searchcards query: <search>` - Search all cards in database
- `/miss card_query: <search>` - Search missing cards from your collection

### Secondary Commands  
- `/has user: @user card_query: <search>` - Check if user has specific card
- `/diff user: @user query: <search>` - Compare collections with search
- `/wish card_query: <search>` - Search your wishlist

## Search Syntax

### Basic Text Search
```
/cards query: winter
/cards query: red velvet
/cards query: cheer up
```
- Searches card names and collections
- Supports partial matching
- Case insensitive

### Level Filtering

#### Include Levels
```
/cards query: 1          # Show only level 1 cards
/cards query: 5          # Show only level 5 cards  
/cards query: 3 4 5      # Show levels 3, 4, and 5
/cards query: level=4    # Alternative syntax for level 4
```

#### Exclude Levels
```
/cards query: -1         # Exclude level 1 cards
/cards query: -1 -2      # Exclude levels 1 and 2
/cards query: 5 -3       # Show level 5, exclude level 3
```

### Collection Filtering

#### Include Collections

**Partial Matching (searches collection names):**
```
/cards query: twice      # Cards from collections containing "twice"
/cards query: aespa      # Cards from collections containing "aespa"
/cards query: itzy red   # Cards from collections containing "itzy" or "red"
```

**Exact Collection ID Matching:**
```
/cards query: collection=twice       # Only cards from exact "twice" collection
/cards query: collection=aespa       # Only cards from exact "aespa" collection  
/cards query: collection=red_velvet  # Only cards from exact "red_velvet" collection
```

#### Exclude Collections  
```
/cards query: !twice     # Exclude collections containing "twice"
/cards query: !promo     # Exclude promo collections
/cards query: winter !aespa  # Cards with "winter" but not from aespa collections
```

### Tag Filtering

#### Include Tags
```
/cards query: #girlgroups    # Show only girl group cards
/cards query: #boygroups     # Show only boy group cards
```

#### Exclude Tags
```
/cards query: !#girlgroups   # Exclude girl group cards
/cards query: !#boygroups    # Exclude boy group cards
```

### Animation Filtering
```
/cards query: gif           # Show only animated cards
/cards query: !gif          # Exclude animated cards
/cards query: animated      # Alternative syntax for animated
/cards query: !animated     # Exclude animated cards
```

### Promo Filtering
```
/cards query: promo         # Show only promo cards
/cards query: !promo        # Exclude promo cards
```

### User-Specific Filters (Cards Command Only)

#### Favorites
```
/cards query: fav           # Show only favorited cards
/cards query: !fav          # Show only non-favorited cards
```

#### Multi Cards
```
/cards query: multi         # Show only cards with amount > 1
/cards query: !multi        # Show only single cards (amount = 1)
```

#### Amount Filtering
```
/cards query: >amount=5     # Cards with more than 5 copies
/cards query: <amount=3     # Cards with less than 3 copies  
/cards query: =amount=1     # Cards with exactly 1 copy
```

### Advanced Combinations

#### Complex Queries
```
/cards query: 5 gif winter !aespa
# Show: Level 5, animated cards with "winter" in name, excluding aespa collections

/cards query: collection=twice 4 5 fav
# Show: Exact TWICE collection, levels 4-5, that are favorited

/cards query: collection=aespa !gif -1 -2
# Show: Exact aespa collection, non-animated, excluding levels 1 and 2

/cards query: >amount=3 5 !promo
# Show: Cards with 3+ copies, level 5, excluding promos

/cards query: collection=red_velvet gif
# Show: Exact Red Velvet collection, animated cards only
```

#### Sorting
```
/cards query: >level       # Sort by level descending (default)
/cards query: <level       # Sort by level ascending
/cards query: >name        # Sort by name descending  
/cards query: <name        # Sort by name ascending
/cards query: >date        # Sort by date obtained descending
```

## Command-Specific Behavior

### /cards (Your Collection)
- Searches only cards you own
- Supports all user-specific filters (fav, multi, amount)
- Shows card amounts and user-specific metadata
- Default sort: Level desc, then name asc

### /searchcards (All Cards)
- Searches entire card database
- Does not show ownership information
- Supports basic filters (level, collection, tags, animation)
- Shows all available cards regardless of ownership

### /miss (Missing Cards)  
- Searches cards you DON'T own
- Uses same syntax as /searchcards
- Helpful for finding cards to collect
- Limited to 3 items per page to prevent Discord errors

## Tips & Best Practices

### Performance Tips
1. **Be Specific**: Use level filters to reduce result sets
2. **Combine Filters**: Use multiple filters to narrow searches
3. **Use Exclusions**: Use `!` filters to remove unwanted results

### Common Patterns
```bash
# Finding specific member cards
/cards query: winter 4 5 gif

# Collection completion  
/miss card_query: twice 5

# Trading preparation
/cards query: multi !fav  # Find duplicate non-favorites

# High value cards
/cards query: 5 gif fav   # Level 5 animated favorites
```

### Troubleshooting

#### "No cards found"
- Check spelling of card names and collections
- Try broader searches (remove some filters)
- Use `/searchcards` to verify card exists in database

#### Level filtering not working
- Ensure you're using numbers 1-5 only
- Try `level=X` syntax instead of just `X`
- Check if you actually own cards of that level

#### Too many results
- Add more specific filters
- Use exclusion filters with `!`
- Add level restrictions

## Examples by Use Case

### New Player
```bash
/cards query: 1 2           # See your starter cards
/miss card_query: 5         # See high-level cards to aim for
/searchcards query: twice   # Browse TWICE collection
```

### Advanced Player  
```bash
/cards query: 5 multi !fav      # Find level 5 duplicates for trading
/cards query: fav gif           # Show your animated favorites
/miss card_query: 5 #girlgroups # Missing level 5 girl group cards
```

### Traders
```bash
/cards query: multi >amount=5   # Cards with 5+ copies
/diff user: @friend query: 5    # Level 5 cards they need
/has user: @friend card_query: winter 5  # Check specific card
```

## Technical Notes

- Search is case-insensitive
- Partial matching supported for names and collections
- Filters are applied in order: user filters → card filters → search matching
- Results are cached for 5 minutes for performance
- Special characters in card names are normalized for searching

---

*This guide covers the unified search system. All syntax works across commands unless noted.*