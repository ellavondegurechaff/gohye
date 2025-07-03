-- Quest System Bug Fixes
-- Run this SQL to update existing quests in the database

-- Update Level Addict quest to track days instead of max level
UPDATE quest_definitions
SET 
    description = 'Use /levelup on 3 different days',
    requirement_metadata = '{"track_days": true}'::jsonb
WHERE quest_id = 'monthly_level_addict' 
   OR (name = 'Level Addict' AND requirement_type = 'card_levelup');

-- Ensure Balanced Routine exists and tracks days
INSERT INTO quest_definitions (
    quest_id, name, description, tier, type, category, 
    requirement_type, requirement_count, requirement_metadata,
    reward_snowflakes, reward_vials, reward_xp, 
    created_at, updated_at
) VALUES (
    'weekly_balanced_routine',
    'Balanced Routine',
    'Level up cards on 5 different days',
    2,
    'weekly',
    'debut',
    'card_levelup',
    5,
    '{"track_days": true}'::jsonb,
    1500,
    60,
    75,
    NOW(),
    NOW()
) ON CONFLICT (quest_id) DO UPDATE
SET 
    description = EXCLUDED.description,
    requirement_metadata = EXCLUDED.requirement_metadata,
    updated_at = NOW();

-- Ensure Flake Farmer exists
INSERT INTO quest_definitions (
    quest_id, name, description, tier, type, category,
    requirement_type, requirement_count,
    reward_snowflakes, reward_vials, reward_xp,
    created_at, updated_at
) VALUES (
    'daily_flake_farmer',
    'Flake Farmer',
    'Earn 3000 snowflakes from any source',
    2,
    'daily',
    'debut',
    'snowflakes_earned',
    3000,
    400,
    30,
    35,
    NOW(),
    NOW()
) ON CONFLICT (quest_id) DO NOTHING;

-- Reset progress for affected quests to allow proper tracking
-- This will reset Level Addict progress for all users
UPDATE user_quest_progress
SET 
    current_progress = 0,
    metadata = '{"levelup_days": {}}'::jsonb,
    completed = false,
    claimed = false
WHERE quest_id IN (
    SELECT quest_id 
    FROM quest_definitions 
    WHERE (quest_id = 'monthly_level_addict' OR name = 'Level Addict')
    AND requirement_type = 'card_levelup'
);

-- Reset Balanced Routine progress if it exists
UPDATE user_quest_progress
SET 
    current_progress = 0,
    metadata = '{"levelup_days": {}}'::jsonb,
    completed = false,
    claimed = false
WHERE quest_id = 'weekly_balanced_routine';