# Quest System Complete Implementation Flow

## Overview
The GoHYE Discord bot quest system is a K-pop themed achievement system with daily, weekly, and monthly quests. Users complete various actions to progress through quests and earn rewards (snowflakes, vials, XP).

## Architecture Components

### 1. Database Schema
- **quest_definitions**: Stores all 45 quest templates (15 daily, 15 weekly, 15 monthly)
- **user_quest_progress**: Tracks user progress with metadata for complex tracking
  - Added `Metadata map[string]interface{}` field for storing complex quest data
- **quest_chains**: Created but not utilized (kept simple)
- **quest_leaderboards**: Created but not utilized (kept simple)

### 2. Quest Types & Requirements
All quest types are now fully implemented:

#### Simple Action Types (✅ Implemented)
- `RequirementTypeCardClaim`: Track card claims
- `RequirementTypeCardLevelUp`: Track card level ups
- `RequirementTypeWorkCommand`: Track work command usage
- `RequirementTypeAuctionBid/Win/Create`: Track auction activities
- `RequirementTypeSnowflakesEarned`: Track & accumulate snowflakes from all sources
- `RequirementTypeCardDraw`: Track draw commands (pending command implementation)
- `RequirementTypeCardTrade`: Track trades (pending command implementation)
- `RequirementTypeAscend`: Track ascensions (pending command implementation)

#### Complex Tracking Types (✅ Implemented)
- `RequirementTypeCommandCount`: Tracks unique commands used
  - Stores command names in metadata["commands_used"]
  - Progress = count of unique commands
- `RequirementTypeWorkDays`: Tracks work on different days
  - Stores dates in metadata["work_days"]
  - Progress = count of unique days
- `RequirementTypeDailyComplete/WeeklyComplete`: Tracks quest completions
  - Triggers after other quests complete
  - Uses completion counting in UpdateProgress
- `RequirementTypeCombo`: Multi-action requirements
  - Stores progress per action in metadata["combo_progress"]
  - Completes when all requirements met

### 3. Core Services

#### QuestService (`services/quest_service.go`)
Main service handling quest logic:
- `AssignDailyQuests/WeeklyQuests/MonthlyQuests`: Auto-assigns 3 quests (1 per tier)
- `UpdateProgress`: Enhanced with special handling for each requirement type
- `ClaimRewards`: Claims rewards for completed quests
- `GetUserQuestStatus`: Returns active quests with progress

Key enhancements:
- `trackWorkDay`: Tracks unique days for work quests
- `trackUniqueCommand`: Tracks unique commands for command count quests
- `trackComboProgress`: Handles multi-requirement combo quests
- `updateCompletionQuests`: Updates completion-type quests after others complete

#### QuestTracker (`services/quest_tracker.go`)
Wrapper for easy integration:
- `TrackCommand`: Now tracks both command count and specific commands
- `TrackCardClaim/LevelUp/Work/etc`: Specific action trackers
- `TrackSnowflakesEarned`: Accumulates snowflakes from all sources

### 4. Command Integration

#### Command Tracking System
- Modified `WrapWithLogging` to track all command executions
- Added `WrapWithLoggingAndQuests` for quest-aware command wrapping
- Tracks commands in background to not slow responses
- Every successful command triggers quest progress updates

#### Quest Commands
- `/quests`: View active quests with K-pop themed UI
  - 3-page system (daily/weekly/monthly)
  - Progress bars with 25/50/75% milestones
  - Color-coded completion status
- `/questclaim`: Claim completed quest rewards
  - Shows summary of claimed quests by type
  - Updates user balance immediately

### 5. Integration Points

#### Existing Command Integration
- `claim.go`: Calls `TrackCardClaim` after successful claims
- `levelup.go`: Calls `TrackCardLevelUp` after level ups
- `work.go`: Calls `TrackWork` + `TrackSnowflakesEarned`
- `daily.go`: Calls `TrackSnowflakesEarned`
- `auction_commands.go`: Tracks bid/create actions
- `auction_confirmation.go`: Tracks auction creation
- `auction_lifecycle.go`: Tracks wins via callback

#### Background Processes
- Quest rotation process runs hourly
- Cleans expired quests automatically
- Auto-assigns quests when viewing if needed

### 6. UI/UX Features

#### K-pop Theming
- Tier names: "🎵 Trainee" → "🌟 Debut" → "✨ Idol"
- Engaging quest names fitting K-pop culture
- Progress bars with milestone celebrations
- Emoji indicators for visual appeal

#### Progress Display
- Percentage-based progress bars
- Milestone diamonds (♦) at 25%, 50%, 75%
- Green checkmarks (✓) for completed quests
- Clean pagination between quest types

### 7. Technical Implementation Details

#### Metadata Storage
Quest progress metadata structure examples:
```go
// Command count tracking
metadata["commands_used"] = map[string]bool{
    "claim": true,
    "work": true,
    "cards": true
}

// Work days tracking
metadata["work_days"] = map[string]bool{
    "2024-01-15": true,
    "2024-01-16": true
}

// Combo progress tracking
metadata["combo_progress"] = map[string]int{
    "claim": 5,      // out of 8 required
    "work": 3,       // out of 3 required (complete)
    "levelup": 7,    // out of 10 required
    "auction_create": 0  // out of 1 required
}
```

#### Quest Flow
1. User performs action → Command handler executes
2. `WrapWithLogging` tracks command execution → `QuestTracker.TrackCommand`
3. Command-specific tracking (e.g., `TrackCardClaim`)
4. `QuestService.UpdateProgress` called with action & metadata
5. Special handling based on requirement type
6. Progress updated, milestones checked
7. If completed, updates leaderboard & triggers completion quests
8. User can claim rewards with `/questclaim`

### 8. Quest Data Structure
45 total quests organized by:
- **Tier 1 (Trainee)**: Easy quests - basic actions
- **Tier 2 (Debut)**: Medium quests - more requirements
- **Tier 3 (Idol)**: Hard quests - challenging goals

Examples:
- Daily T1: "Claim any card" (1 claim)
- Daily T2: "Use 5 different commands" (command count)
- Daily T3: "Combo Player" (8 claims + 3 work + 10 levelup + 1 auction)
- Weekly T2: "Work on 5 different days" (work days)
- Monthly T3: "Complete all Weekly quests every week" (weekly complete)

### 9. Rewards System
Exact rewards by tier:
- **Daily**: T1 (250/20/25), T2 (400/30/35), T3 (650/50/50)
- **Weekly**: T1 (1000/40/50), T2 (1500/60/75), T3 (2500/100/100)
- **Monthly**: T1 (3000/80/100), T2 (5000/120/150), T3 (8000/200/200)
Format: (snowflakes/vials/XP)

### 10. Future Enhancements Ready
The system is prepared for:
- `/draw`, `/trade`, `/ascend` commands (quests defined, awaiting implementation)
- Quest chains (database ready, not implemented)
- Leaderboards (database ready, not implemented)
- Additional quest types (extensible design)