# Effect System Complete Implementation Guide

## Overview
The effect system has been transformed from static purchases to a tiered progression system where effects level up through usage (Tiers 1-5).

## Database Schema

### Migrations Added to `bottemplate/database/db.go`
```go
`ALTER TABLE user_effects ADD COLUMN IF NOT EXISTS tier INTEGER NOT NULL DEFAULT 1;`,
`ALTER TABLE user_effects ADD COLUMN IF NOT EXISTS progress INTEGER NOT NULL DEFAULT 0;`,
```

### Updated Models in `bottemplate/database/models/effect.go`
```go
type UserEffect struct {
    // ... existing fields ...
    Tier           int        `bun:"tier,notnull,default:1"`     // Current tier level (1-5)
    Progress       int        `bun:"progress,notnull,default:0"` // Progress towards next tier
}

type EffectTierData struct {
    Values     []int `json:"values"`     // Values per tier (e.g., bonus amounts)
    Thresholds []int `json:"thresholds"` // Requirements to reach next tier
}
```

## Effect Definitions with Tier Data

### Updated `bottemplate/economy/effects/effect_items.go`

1. **Cake Day** (cakeday)
   - Values: [10, 25, 45, 70, 100] flakes per claim
   - Thresholds: [100, 300, 700, 1500] claims
   - Progress tracked in: `claim.go`

2. **Holy Grail** (holygrail)
   - Values: [5, 10, 20, 40, 70] vials per liquify
   - Thresholds: [30, 80, 180, 350] liquefies
   - Progress tracked in: `liquefy.go`

3. **Wolf of Hyejoo** (skyfriend)
   - Values: [2, 4, 6, 8, 10] % cashback
   - Thresholds: [20000, 60000, 150000, 350000] flakes spent
   - Progress tracked in: auction wins (TODO)

4. **Lamb of Hyejoo** (lambhyejoo)
   - Values: [2, 4, 6, 8, 10] % sale bonus
   - Thresholds: [20000, 60000, 150000, 350000] flakes earned
   - Progress tracked in: auction sales (TODO)

5. **Cherry Blossom** (cherrybloss)
   - Values: [20, 30, 40, 50, 60] % discount
   - Thresholds: [10, 30, 70, 150] forges/ascends
   - Progress tracked in: `forge.go`

6. **Ruler Jeanne** (rulerjeanne)
   - Values: [1170, 1140, 1110, 1080, 1020] minutes (19.5h, 19h, 18.5h, 18h, 17h)
   - Thresholds: [10, 25, 50, 100] dailies
   - Progress tracked in: `daily.go`

7. **Youth Youth By Young** (youthyouth)
   - Values: [10, 20, 30, 40, 50] % work bonus
   - Thresholds: [50, 150, 350, 700] works
   - Progress tracked in: `work.go`

8. **Kiss Later** (kisslater)
   - Values: [5, 10, 15, 20, 30] % XP bonus
   - Thresholds: [30, 80, 180, 400] levelups
   - Progress tracked in: `levelup.go`

## Repository Methods

### Added to `bottemplate/database/repositories/effect_repository.go` interface:
```go
UpdateEffectProgress(ctx context.Context, userID string, effectID string, increment int) error
UpgradeEffectTier(ctx context.Context, userID string, effectID string) error
GetUserEffectsByTier(ctx context.Context, userID string) ([]*models.UserEffect, error)
```

## Manager Methods

### Already exist in `bottemplate/economy/effects/manager.go`:
```go
GetEffectTierValue(ctx context.Context, userID string, effectID string) (int, error)
UpdateEffectProgress(ctx context.Context, userID string, effectID string, increment int) error
CheckEffectUpgrade(ctx context.Context, userID string, effectID string) (bool, int, int, error)
UpgradeEffectTier(ctx context.Context, userID string, effectID string) error
GetUserEffectsSorted(ctx context.Context, userID string) ([]*models.UserEffect, error)
```

## New Commands

### 1. `/effects` - `bottemplate/commands/system/effects.go`
- Shows all user effects with tier stars and progress bars
- Visual format: `🍰 Cake Day ⭐⭐⭐ [████░░] 420/700`

### 2. `/effect info <effect>` - `bottemplate/commands/system/effect_info.go`
- Subcommands: `info` and `upgrade`
- Shows detailed tier progression and requirements
- Displays how to progress each effect

### 3. `/effect upgrade <effect>` - `bottemplate/commands/system/effect_upgrade.go`
- Handles tier upgrades with confirmation UI
- Shows before/after values

## Progress Tracking Integration

### Hook Locations:
1. **claim.go** (line 370): `go h.bot.EffectManager.UpdateEffectProgress(context.Background(), userID, "cakeday", count)`
2. **daily.go** (line 103): `go b.EffectManager.UpdateEffectProgress(ctx, user.DiscordID, "rulerjeanne", 1)`
3. **work.go** (line 665): `go h.bot.EffectManager.UpdateEffectProgress(context.Background(), user.DiscordID, "youthyouth", 1)`
4. **liquefy.go** (line 242): `go h.bot.EffectManager.UpdateEffectProgress(context.Background(), e.User().ID.String(), "holygrail", 1)`
5. **forge.go** (line 184): `go h.bot.EffectManager.UpdateEffectProgress(context.Background(), userID, "cherrybloss", 1)`
6. **levelup.go** (line 107): `go c.bot.EffectManager.UpdateEffectProgress(context.Background(), event.User().ID.String(), "kisslater", 1)`

## New Effect Handlers

### Added to `bottemplate/economy/effects/handlers/passive.go`:

1. **LambhyejooHandler**
   - Applies auction sale bonus based on tier
   - Integrates with auction system for progress

2. **YouthyouthHandler**
   - Applies work reward bonus based on tier
   - Already integrated with work command

3. **KisslaterHandler**
   - Applies levelup XP bonus based on tier
   - Already integrated with levelup command

### Registration in `main.go`:
```go
passiveEffects := []effects.EffectHandler{
    // ... existing handlers ...
    effectsHandlers.NewLambhyejooHandler(deps),
    effectsHandlers.NewYouthyouthHandler(deps),
    effectsHandlers.NewKisslaterHandler(deps),
}
```

## Implementation Details

### Effect ID Corrections:
- Fixed: `wolfofhyejoo` → `skyfriend` (multiple occurrences)
- Fixed: `lambofhyejoo` → `lambhyejoo` (multiple occurrences)
- Fixed: `youthyouthbyyoung` → `youthyouth` (multiple occurrences)

### Calculation Fixes:
- Ruler Jeanne: Changed from `value / 100.0` to `value / 60.0` for proper hours calculation

### Visual Elements:
- Tier stars: ⭐ (1-4), 🌟 (5/max)
- Progress bars: `[████████░░]` format
- Percentage display: `(60%)`

## Testing Checklist

1. Purchase effect from shop
2. Check `/effects` shows tier 1 with 0 progress
3. Perform action (e.g., claim for Cake Day)
4. Check progress updates
5. Reach threshold and use `/effect upgrade`
6. Verify new tier values apply correctly

## Future Considerations

1. **Auction Integration**: Wolf/Lamb effects need auction hooks
2. **Visual Feedback**: Consider adding progress notifications
3. **Milestone Rewards**: Could add rewards at 25%, 50%, 75%
4. **Effect Focus System**: Removed from plan but could be revisited

## Code Patterns Used

- Repository pattern for database access
- Async progress updates with goroutines
- Consistent error handling with slog
- Discord embed builders for UI
- Component handlers for interactions
- Effect handler interface pattern