# Quest System Original Plan & Design Decisions

## Original Requirements (from quests.txt)
- 3 quests per period (1 of each tier: T1, T2, T3)
- Manual claiming required (no auto-complete)
- 5 quest variants per tier for rotation
- Exact rewards: T1 daily (250/20/25), T2 daily (400/30/35), T3 daily (650/50/50), etc.

## Design Decisions Made

### K-pop Theming
- Tier names: "Trainee" → "Debut" → "Idol" journey
- Engaging quest names fitting K-pop culture
- Visual progress with milestone celebrations
- Interactive UI with emoji indicators

### Architecture Decisions
- Repository pattern for data access
- Service layer for business logic  
- Quest tracker for easy command integration
- Background process for expiration handling
- No quest chains/storylines (avoid overengineering)
- No leaderboards (keep it simple)
- Each quest independent (no dependencies)

### UI/UX Decisions
- 3-page system (daily/weekly/monthly)
- Progress bars with 25/50/75% milestones
- Green checkmarks for completed quests
- Clean claim summary showing only relevant completions
- K-pop themed descriptions and formatting

## Implementation Strategy
1. Follow existing bot patterns exactly
2. Integrate seamlessly into existing commands
3. Don't modify core command logic
4. Use goroutines for non-blocking tracking
5. Maintain backward compatibility

## Quest Types by Requirement
- **Simple Actions**: claim, levelup, work, auction operations
- **Missing Commands**: draw, trade, ascend
- **Complex Tracking**: command count, different days, completions
- **Special Cases**: max level, snowflakes from specific sources, combos

## Original Conversation Context
User wanted engaging, K-pop themed quest system following the spec but making it more interactive. Emphasized:
- Don't break existing functionality
- Follow existing codebase patterns
- Make it fun and engaging
- Avoid overengineering