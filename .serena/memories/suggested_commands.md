# Suggested Commands for GoHYE Development

## Development Commands
```bash
# Run with development config and sync commands
go run . --config=config.toml --sync-commands=true

# Run with price calculation on startup
go run . --config=config.toml --calculate-prices=true

# Standard development run
go run .
```

## Production Build
```bash
# Build with version info
go build -ldflags="-X 'main.version=${VERSION}' -X 'main.commit=${COMMIT}'" -o bot

# Docker build
docker build -t gohye .

# Docker compose
docker-compose up
```

## Code Quality Commands
```bash
# Run linter (golangci-lint)
golangci-lint run

# Format code
go fmt ./...

# Run go vet
go vet ./...
```

## Database Commands
```bash
# Run migrations (if available)
# Note: Currently no formal migration system

# Connect to PostgreSQL
psql -h localhost -U username -d database_name
```

## Testing
No formal test suite exists currently. Testing is done through Discord interactions.

## Git Commands
```bash
# Check status
git status

# Create feature branch
git checkout -b feature/quest-system

# Stage changes
git add .

# Commit with conventional message
git commit -m "feat: add quest system implementation"

# Push changes
git push origin feature/quest-system
```