# Code Style and Conventions

## Go Code Style
- Follow standard Go conventions (gofmt, golint)
- Use meaningful variable and function names
- Error handling: Always check and handle errors appropriately
- Use context for cancellation and timeouts
- Repository pattern for database access
- Service layer for business logic

## Naming Conventions
- **Files**: lowercase with underscores (e.g., `user_quest.go`)
- **Structs**: PascalCase (e.g., `UserQuest`)
- **Functions**: PascalCase for exported, camelCase for unexported
- **Variables**: camelCase
- **Constants**: PascalCase or UPPER_SNAKE_CASE
- **Database tables**: snake_case with plural names (e.g., `user_quests`)

## Command Structure Pattern
```go
// Command definition
var QuestCommand = discord.SlashCommandCreate{
    Name:        "quest",
    Description: "View your active quests",
}

// Handler function
func QuestHandler(b *bottemplate.Bot) handlers.CommandHandler {
    return func(e *events.ApplicationCommandInteractionCreate) error {
        // Implementation
    }
}
```

## Repository Pattern
```go
type QuestRepository interface {
    GetActiveQuests(ctx context.Context, userID string) ([]*models.Quest, error)
    CreateQuest(ctx context.Context, quest *models.Quest) error
    UpdateQuest(ctx context.Context, quest *models.Quest) error
}
```

## Error Handling
- Use wrapped errors with context
- Return structured error responses
- Log errors with appropriate levels
- Use utils.EH for Discord error responses

## Database Models
- Use Bun ORM struct tags
- Include timestamps (created_at, updated_at)
- Use appropriate indexes
- Follow PostgreSQL naming conventions