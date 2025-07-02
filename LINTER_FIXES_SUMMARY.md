# Linter Fixes Summary

## Fixed Issues:

### 1. work.go
- **Removed unused import**: Removed `strconv` import that was not being used
- **Fixed return type mismatch**: Changed `createScenarioComponents` to return `[]discord.ContainerComponent` instead of trying to return multiple components
- **Updated component usage**: Fixed the usage in `HandleWork` to pass components array directly

### 2. fuse.go
- **Fixed UserCard struct fields**:
  - Removed `Animated` field (doesn't exist in UserCard model)
  - Changed `ObtainedAt` to `Obtained` (correct field name)
- **Fixed GetCardImageURL call**:
  - Changed from non-existent `card.URL` property
  - Now correctly calls with required parameters: `(cardName, colID, level, groupType)`
  - Added logic to determine collection type based on album type

## Model Reference:
- UserCard fields: ID, UserID, CardID, Level, Exp, Amount, Favorite, Locked, Rating, Obtained, Mark, CreatedAt, UpdatedAt
- Card fields: ID, Name, Level, Animated, ColID, Tags, CreatedAt, UpdatedAt

All linter errors should now be resolved.