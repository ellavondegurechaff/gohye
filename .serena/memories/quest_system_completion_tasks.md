# Quest System Completion Tasks

## Priority 1: Handle Missing Commands
Since /draw, /trade, and /ascend don't exist, we have 3 options:
1. **Remove these quests** from quest_data.go (simplest)
2. **Implement placeholder commands** that just track quest progress
3. **Replace with alternative quests** using existing commands

Recommendation: Option 1 - Remove unsupported quests and replace with variants of existing commands

## Priority 2: Fix Special Quest Types

### RequirementTypeWorkDays
- Add date tracking to work progress
- Store last work dates in metadata
- Check unique days in progress calculation

### RequirementTypeDailyComplete / WeeklyComplete
- Track quest completions by type
- Add completion counter in quest service
- Reset counters at period boundaries

### RequirementTypeCombo
- Already has metadata structure in quest_data.go
- Need to track multiple actions in UpdateProgress
- Check all requirements met before marking complete

### RequirementTypeCommandCount
- Add command execution wrapper
- Track unique commands used
- Store in user metadata or separate table

## Priority 3: Enhance Tracking

### Max Level Detection
- Check if resulting card level = max (5) after levelup
- Pass this info in metadata to quest tracker

### Source-Specific Snowflakes
- Track snowflakes by source (auction, work, daily, etc.)
- Use metadata["source"] field
- Already partially implemented for auction

### Quest Rotation Enhancement
- On period reset, auto-assign new quests
- Check at quest view time if period changed
- Clean old quests and assign fresh set

## Implementation Order
1. Remove/replace unsupported quest types
2. Fix RequirementTypeCommandCount (most quests need this)
3. Implement special tracking for days/completions
4. Add max level detection
5. Enhance rotation system

## Code Locations
- Quest definitions: `bottemplate/database/quest_data.go`
- Progress logic: `bottemplate/services/quest_service.go`
- Tracking integration: Various command files
- Background process: `main.go` (quest-rotation)