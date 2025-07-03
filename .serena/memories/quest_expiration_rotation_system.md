# Quest Expiration and Rotation System

## Overview
The quest system implements automatic expiration and rotation to keep quests fresh and aligned with their time periods (daily, weekly, monthly).

## Expiration Timing

### Reset Schedule
The `getNextReset` function calculates when each quest type expires:

1. **Daily Quests**
   - Reset at midnight (00:00) every day
   - Example: Quest assigned at 3 PM expires at midnight same day (9 hours)
   - Next day starts with fresh daily quests

2. **Weekly Quests**
   - Reset at midnight on Monday each week
   - Example: Quest assigned Wednesday expires next Monday at midnight
   - Weekly cycle follows calendar weeks (Monday-Sunday)

3. **Monthly Quests**
   - Reset at midnight on the 1st of each month
   - Example: Quest assigned Jan 15th expires Feb 1st at midnight
   - Monthly cycle follows calendar months

## Rotation Mechanism

### 1. Background Process
- Runs every hour via `quest-rotation` background process
- Calls `DeleteExpiredQuests` to remove expired, unclaimed quests
- Only deletes quests where:
  - `expires_at < current_time`
  - `claimed = false`
- Claimed quests are preserved for historical records

### 2. Automatic Assignment
When user runs `/quests`:
1. System checks if user has 3 quests of each type
2. For missing tiers, assigns random quest from that tier
3. Each user always has exactly:
   - 3 daily quests (1 Trainee, 1 Debut, 1 Idol)
   - 3 weekly quests (1 of each tier)
   - 3 monthly quests (1 of each tier)

### 3. Assignment Logic
```go
// For each tier (1, 2, 3):
// 1. Check if user already has quest of this tier & type
// 2. If not, get random quest from pool
// 3. Assign with appropriate expiration time
```

## User Experience Flow

### Daily Reset Example
- **Day 1, 2 PM**: User completes daily quests, claims rewards
- **Day 1, 11:59 PM**: Uncompleted/unclaimed quests about to expire
- **Day 2, 12:00 AM**: Old quests expire, deleted by background process
- **Day 2, 9 AM**: User runs `/quests`, gets 3 new daily quests

### Weekly Reset Example
- **Wednesday**: User has 3 weekly quests, completes 2
- **Sunday Night**: 1 uncompleted quest still active
- **Monday, 12:00 AM**: All uncompleted weekly quests expire
- **Monday Morning**: User runs `/quests`, gets 3 fresh weekly quests

### Monthly Reset Example
- **January 15th**: User gets monthly quests
- **January 31st, 11:59 PM**: Any uncompleted quests about to expire
- **February 1st, 12:00 AM**: Monthly reset, old quests deleted
- **February 1st**: User runs `/quests`, gets new monthly quests

## Key Features

### No Manual Refresh Needed
- Users don't need to manually refresh quests
- Expired quests automatically removed
- New quests assigned on demand when viewing

### Grace Period
- Completed but unclaimed quests persist past expiration
- Users can still claim rewards for completed quests
- Only unclaimed AND uncompleted quests are deleted

### Consistent Quest Count
- System ensures users always have 9 active quests (3 per type)
- Missing quests auto-assigned when viewing quest list
- Random selection ensures variety across periods

### Time Zone Handling
- Uses server's local time zone for all calculations
- Consistent reset times for all users
- Midnight resets provide clear daily/weekly/monthly boundaries

## Technical Implementation

### Database Queries
- `GetActiveQuests`: Only returns non-expired, unclaimed quests
- `DeleteExpiredQuests`: Bulk deletes expired, unclaimed quests
- `AssignQuests`: Checks existing before assigning new

### Performance Optimizations
- Hourly cleanup prevents database bloat
- Indexed on `expires_at` for efficient queries
- Bulk operations for deletion
- Quest assignment only when needed (lazy loading)

## Edge Cases Handled

1. **Multiple Claims Prevention**
   - Can't claim same quest multiple times
   - Claimed quests marked and excluded from active list

2. **Period Boundary Handling**
   - Quests assigned near period end get full duration
   - No partial periods or shortened quests

3. **Missing Quest Recovery**
   - If user somehow has < 3 quests per type
   - System auto-assigns missing ones on view

4. **Completion After Expiration**
   - Completed quests remain claimable past expiration
   - Only deleted if uncompleted AND unclaimed